package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
	"goskeleton/internal/spreadsheet"
)

const maxChatBodyBytes = 64 << 10

type ChatService interface {
	Handle(ctx context.Context, userID uuid.UUID, req model.ChatRequest) (*model.ChatResponse, error)
	Confirm(ctx context.Context, userID uuid.UUID, id string) (string, error)
	Cancel(userID uuid.UUID, id string) error
	AttachFile(ctx context.Context, userID uuid.UUID, filename string, r io.Reader) (*model.ChatFile, error)
}

func handleChatFileUpload(chat ChatService, limiter *rateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		if limiter != nil && !limiter.allow("chat-file|"+userID.String()) {
			respondError(w, http.StatusTooManyRequests, "too many requests")
			return
		}

		file, filename, ok := parseUploadedFile(w, r, maxImportBytes)
		if !ok {
			return
		}
		defer func() {
			_ = file.Close()
		}()

		result, err := chat.AttachFile(r.Context(), userID, filename, file)
		if err != nil {
			respondSpreadsheetError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, result)
	}
}

func respondSpreadsheetError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, spreadsheet.ErrUnsupportedFormat),
		errors.Is(err, spreadsheet.ErrEmptyWorkbook),
		errors.Is(err, spreadsheet.ErrTooManyRows):
		respondError(w, http.StatusBadRequest, err.Error())
	default:
		respondError(w, http.StatusBadRequest, "unable to read the uploaded file")
	}
}

func handleChat(chat ChatService, limiter *rateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		if limiter != nil && !limiter.allow("chat|"+userID.String()) {
			respondError(w, http.StatusTooManyRequests, "too many requests")
			return
		}

		var req model.ChatRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxChatBodyBytes)).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		response, err := chat.Handle(r.Context(), userID, req)
		if err != nil {
			respondChatError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, response)
	}
}

func handleChatConfirm(chat ChatService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		summary, err := chat.Confirm(r.Context(), userID, r.PathValue("id"))
		if err != nil {
			respondChatError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, model.ChatResponse{Reply: summary, ToolCalls: []model.ChatToolCall{}})
	}
}

func handleChatCancel(chat ChatService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		if err := chat.Cancel(userID, r.PathValue("id")); err != nil {
			respondChatError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, model.ChatResponse{Reply: "Cancelled.", ToolCalls: []model.ChatToolCall{}})
	}
}

func respondChatError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrLLMNotConfigured):
		respondError(w, http.StatusServiceUnavailable, "chat is not configured")
	case errors.Is(err, service.ErrPendingActionNotFound):
		respondError(w, http.StatusNotFound, "pending action not found")
	case errors.Is(err, service.ErrChatTooManySteps):
		respondError(w, http.StatusUnprocessableEntity, "the assistant used too many steps; try a narrower request")
	case errors.Is(err, service.ErrAttachmentNotFound), errors.Is(err, service.ErrUnknownSheet):
		respondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrTransactionNotFound):
		respondError(w, http.StatusNotFound, err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "chat failed")
	}
}
