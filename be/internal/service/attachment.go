package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"goskeleton/internal/attachment"
	"goskeleton/internal/model"
)

var (
	ErrAttachmentTooLarge        = errors.New("attachment must be between 1 byte and 5 MB")
	ErrUnsupportedAttachmentType = errors.New("unsupported attachment type")
)

const (
	// MaxAttachmentBytes is the upload ceiling for a single attachment.
	MaxAttachmentBytes = 5 << 20
	// attachmentSniffBytes is what http.DetectContentType inspects.
	attachmentSniffBytes = 512
	// fallbackAttachmentFilename is stored when the client sends no filename.
	fallbackAttachmentFilename = "attachment"
)

// allowedAttachmentTypes maps an accepted content type to the on-disk
// extension. The extension never comes from the uploaded filename.
var allowedAttachmentTypes = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/webp":      ".webp",
	"image/heic":      ".heic",
	"application/pdf": ".pdf",
}

// dangerousAttachmentTypes are never served inline, whatever the client
// declares, because a browser would execute them as markup.
var dangerousAttachmentTypes = map[string]struct{}{
	"image/svg+xml":         {},
	"text/html":             {},
	"application/xhtml+xml": {},
}

// AttachmentRepository owns attachment metadata. Every method is scoped by
// user_id so one caller can never read or delete another caller's rows.
type AttachmentRepository interface {
	Create(ctx context.Context, userID uuid.UUID, attachment model.Attachment) (*model.Attachment, error)
	ListByTransaction(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID) ([]model.Attachment, error)
	GetByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*model.Attachment, error)
	Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
	StorageKeysByTransaction(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID) ([]string, error)
	StorageKeysByAccount(ctx context.Context, userID uuid.UUID, accountID uuid.UUID) ([]string, error)
}

// AttachmentTransactionLookup is the minimal transaction read the attachment
// service needs to prove the target transaction belongs to the caller.
type AttachmentTransactionLookup interface {
	GetTransaction(ctx context.Context, id uuid.UUID) (*model.Transaction, error)
}

// AttachmentService is the attachment use case surface.
type AttachmentService interface {
	List(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID) ([]model.Attachment, error)
	Upload(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID, filename string, declaredType string, r io.Reader, size int64) (*model.Attachment, error)
	Open(ctx context.Context, userID uuid.UUID, attachmentID uuid.UUID) (model.Attachment, io.ReadCloser, error)
	Delete(ctx context.Context, userID uuid.UUID, attachmentID uuid.UUID) error
	StorageKeysForTransaction(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID) ([]string, error)
	StorageKeysForAccount(ctx context.Context, userID uuid.UUID, accountID uuid.UUID) ([]string, error)
	RemoveFiles(keys []string)
}

type attachmentService struct {
	repo         AttachmentRepository
	transactions AttachmentTransactionLookup
	store        attachment.Store
}

func NewAttachmentService(repo AttachmentRepository, transactions AttachmentTransactionLookup, store attachment.Store) AttachmentService {
	return &attachmentService{repo: repo, transactions: transactions, store: store}
}

func (s *attachmentService) List(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID) ([]model.Attachment, error) {
	if err := s.requireTransaction(ctx, userID, transactionID); err != nil {
		return nil, err
	}

	items, err := s.repo.ListByTransaction(ctx, userID, transactionID)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	if items == nil {
		items = []model.Attachment{}
	}

	return items, nil
}

func (s *attachmentService) Upload(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID, filename string, declaredType string, r io.Reader, size int64) (*model.Attachment, error) {
	if size <= 0 || size > MaxAttachmentBytes {
		return nil, ErrAttachmentTooLarge
	}

	head := make([]byte, attachmentSniffBytes)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("read attachment: %w", err)
	}
	head = head[:n]
	if len(head) == 0 {
		return nil, ErrAttachmentTooLarge
	}

	contentType, extension, ok := resolveAttachmentType(declaredType, http.DetectContentType(head))
	if !ok {
		return nil, ErrUnsupportedAttachmentType
	}

	if err := s.requireTransaction(ctx, userID, transactionID); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(filename)
	if name == "" {
		name = fallbackAttachmentFilename
	}

	storageKey := uuid.New().String() + extension
	written, err := s.store.Save(storageKey, io.MultiReader(bytes.NewReader(head), r))
	if err != nil {
		return nil, fmt.Errorf("store attachment: %w", err)
	}
	if written <= 0 || written > MaxAttachmentBytes {
		s.removeFile(ctx, storageKey)
		return nil, ErrAttachmentTooLarge
	}

	created, err := s.repo.Create(ctx, userID, model.Attachment{
		TransactionID: transactionID,
		Filename:      name,
		ContentType:   contentType,
		SizeBytes:     written,
		StorageKey:    storageKey,
	})
	if err != nil {
		s.removeFile(ctx, storageKey)
		return nil, fmt.Errorf("create attachment metadata: %w", err)
	}

	return created, nil
}

func (s *attachmentService) Open(ctx context.Context, userID uuid.UUID, attachmentID uuid.UUID) (model.Attachment, io.ReadCloser, error) {
	item, err := s.repo.GetByID(ctx, userID, attachmentID)
	if err != nil {
		if errors.Is(err, ErrAttachmentNotFound) {
			return model.Attachment{}, nil, ErrAttachmentNotFound
		}
		return model.Attachment{}, nil, fmt.Errorf("get attachment: %w", err)
	}

	file, err := s.store.Open(item.StorageKey)
	if err != nil {
		return model.Attachment{}, nil, fmt.Errorf("open attachment file: %w", err)
	}

	return *item, file, nil
}

func (s *attachmentService) Delete(ctx context.Context, userID uuid.UUID, attachmentID uuid.UUID) error {
	item, err := s.repo.GetByID(ctx, userID, attachmentID)
	if err != nil {
		if errors.Is(err, ErrAttachmentNotFound) {
			return ErrAttachmentNotFound
		}
		return fmt.Errorf("get attachment: %w", err)
	}

	if err := s.repo.Delete(ctx, userID, attachmentID); err != nil {
		if errors.Is(err, ErrAttachmentNotFound) {
			return ErrAttachmentNotFound
		}
		return fmt.Errorf("delete attachment: %w", err)
	}

	s.removeFile(ctx, item.StorageKey)

	return nil
}

func (s *attachmentService) StorageKeysForTransaction(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID) ([]string, error) {
	keys, err := s.repo.StorageKeysByTransaction(ctx, userID, transactionID)
	if err != nil {
		return nil, fmt.Errorf("list transaction attachment keys: %w", err)
	}

	return keys, nil
}

func (s *attachmentService) StorageKeysForAccount(ctx context.Context, userID uuid.UUID, accountID uuid.UUID) ([]string, error) {
	keys, err := s.repo.StorageKeysByAccount(ctx, userID, accountID)
	if err != nil {
		return nil, fmt.Errorf("list account attachment keys: %w", err)
	}

	return keys, nil
}

// RemoveFiles deletes stored bytes after the metadata rows are gone. A failed
// removal leaves an orphan file, which is harmless, so it is logged only.
func (s *attachmentService) RemoveFiles(keys []string) {
	for _, key := range keys {
		s.removeFile(context.Background(), key)
	}
}

func (s *attachmentService) removeFile(ctx context.Context, key string) {
	if err := s.store.Remove(key); err != nil {
		slog.ErrorContext(ctx, "remove attachment file", "storage_key", key, "error", err)
	}
}

func (s *attachmentService) requireTransaction(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID) error {
	transaction, err := s.transactions.GetTransaction(ctx, transactionID)
	if err != nil {
		if errors.Is(err, ErrTransactionNotFound) {
			return ErrTransactionNotFound
		}
		return fmt.Errorf("get transaction: %w", err)
	}
	if transaction.UserID != userID {
		return ErrTransactionNotFound
	}

	return nil
}

// resolveAttachmentType accepts a payload only when the sniffed or the declared
// type is allowlisted, and rejects browser-executable markup outright.
func resolveAttachmentType(declared string, sniffed string) (string, string, bool) {
	sniffed = normalizeContentType(sniffed)
	declared = normalizeContentType(declared)

	if _, dangerous := dangerousAttachmentTypes[sniffed]; dangerous {
		return "", "", false
	}
	if extension, ok := allowedAttachmentTypes[sniffed]; ok {
		return sniffed, extension, true
	}
	if extension, ok := allowedAttachmentTypes[declared]; ok {
		return declared, extension, true
	}

	return "", "", false
}

func normalizeContentType(value string) string {
	if index := strings.IndexByte(value, ';'); index >= 0 {
		value = value[:index]
	}

	return strings.ToLower(strings.TrimSpace(value))
}
