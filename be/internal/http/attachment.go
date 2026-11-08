package httpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"

	"github.com/google/uuid"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

// maxAttachmentUploadBytes bounds the whole multipart body, leaving room for
// the form overhead around a 5 MB file.
const maxAttachmentUploadBytes = 6 << 20

// AttachmentService is the attachment use case surface the HTTP layer needs.
type AttachmentService interface {
	List(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID) ([]model.Attachment, error)
	Upload(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID, filename string, declaredType string, r io.Reader, size int64) (*model.Attachment, error)
	Open(ctx context.Context, userID uuid.UUID, attachmentID uuid.UUID) (model.Attachment, io.ReadCloser, error)
	Delete(ctx context.Context, userID uuid.UUID, attachmentID uuid.UUID) error
	StorageKeysForTransaction(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID) ([]string, error)
	StorageKeysForAccount(ctx context.Context, userID uuid.UUID, accountID uuid.UUID) ([]string, error)
	RemoveFiles(keys []string)
}

// ListTransactionAttachments godoc
// @Summary List attachments for a transaction
// @Tags attachments
// @Produce json
// @Security BearerAuth
// @Param id path string true "Transaction ID"
// @Success 200 {array} model.Attachment
// @Failure 401 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Router /api/transactions/{id}/attachments [get]
func handleListTransactionAttachments(attachments AttachmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		transactionID, ok := parseUUIDPath(w, r, "id")
		if !ok {
			return
		}

		items, err := attachments.List(r.Context(), userID, transactionID)
		if err != nil {
			respondAttachmentError(w, err)
			return
		}
		if items == nil {
			items = []model.Attachment{}
		}

		respondJSON(w, http.StatusOK, items)
	}
}

// UploadTransactionAttachment godoc
// @Summary Upload an attachment for a transaction
// @Tags attachments
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path string true "Transaction ID"
// @Param file formData file true "Receipt file"
// @Success 201 {object} model.Attachment
// @Failure 400 {object} model.ErrorResponse
// @Failure 401 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 413 {object} model.ErrorResponse
// @Failure 415 {object} model.ErrorResponse
// @Router /api/transactions/{id}/attachments [post]
func handleUploadTransactionAttachment(attachments AttachmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		transactionID, ok := parseUUIDPath(w, r, "id")
		if !ok {
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxAttachmentUploadBytes)
		if err := r.ParseMultipartForm(maxAttachmentUploadBytes); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				respondError(w, http.StatusRequestEntityTooLarge, service.ErrAttachmentTooLarge.Error())
				return
			}
			respondError(w, http.StatusBadRequest, "invalid multipart form")
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			respondError(w, http.StatusBadRequest, "file is required")
			return
		}
		defer func() {
			_ = file.Close()
		}()

		created, err := attachments.Upload(
			r.Context(),
			userID,
			transactionID,
			header.Filename,
			header.Header.Get("Content-Type"),
			file,
			header.Size,
		)
		if err != nil {
			respondAttachmentError(w, err)
			return
		}

		respondJSON(w, http.StatusCreated, created)
	}
}

// DownloadAttachment godoc
// @Summary Download an attachment
// @Tags attachments
// @Produce octet-stream
// @Security BearerAuth
// @Param id path string true "Attachment ID"
// @Success 200 {file} binary
// @Failure 401 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Router /api/attachments/{id} [get]
func handleDownloadAttachment(attachments AttachmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		attachmentID, ok := parseUUIDPath(w, r, "id")
		if !ok {
			return
		}

		item, reader, err := attachments.Open(r.Context(), userID, attachmentID)
		if err != nil {
			respondAttachmentError(w, err)
			return
		}
		defer func() {
			_ = reader.Close()
		}()

		disposition := mime.FormatMediaType("inline", map[string]string{"filename": item.Filename})
		if disposition == "" {
			disposition = "inline"
		}
		w.Header().Set("Content-Type", item.ContentType)
		w.Header().Set("Content-Disposition", disposition)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)

		if _, err := io.Copy(w, reader); err != nil {
			slog.ErrorContext(r.Context(), "stream attachment", "attachment_id", attachmentID.String(), "error", err)
		}
	}
}

// DeleteAttachment godoc
// @Summary Delete an attachment
// @Tags attachments
// @Security BearerAuth
// @Param id path string true "Attachment ID"
// @Success 204
// @Failure 401 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Router /api/attachments/{id} [delete]
func handleDeleteAttachment(attachments AttachmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		attachmentID, ok := parseUUIDPath(w, r, "id")
		if !ok {
			return
		}

		if err := attachments.Delete(r.Context(), userID, attachmentID); err != nil {
			respondAttachmentError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func respondAttachmentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrTransactionNotFound),
		errors.Is(err, service.ErrAttachmentNotFound):
		respondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrAttachmentTooLarge):
		respondError(w, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, service.ErrUnsupportedAttachmentType):
		respondError(w, http.StatusUnsupportedMediaType, err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "internal server error")
	}
}
