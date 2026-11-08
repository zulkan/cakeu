package service_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

type fakeAttachmentRepo struct {
	created      *model.Attachment
	items        []model.Attachment
	item         *model.Attachment
	createErr    error
	listErr      error
	getErr       error
	deleteErr    error
	keysErr      error
	keys         []string
	lastUserID   uuid.UUID
	lastCreate   model.Attachment
	lastDeleteID uuid.UUID
	deleted      []uuid.UUID
}

func (f *fakeAttachmentRepo) Create(ctx context.Context, userID uuid.UUID, item model.Attachment) (*model.Attachment, error) {
	f.lastUserID = userID
	f.lastCreate = item
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.created != nil {
		return f.created, nil
	}
	created := item
	created.ID = uuid.New()
	created.UserID = userID
	return &created, nil
}

func (f *fakeAttachmentRepo) ListByTransaction(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID) ([]model.Attachment, error) {
	f.lastUserID = userID
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.items, nil
}

func (f *fakeAttachmentRepo) GetByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*model.Attachment, error) {
	f.lastUserID = userID
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.item != nil {
		return f.item, nil
	}
	for _, item := range f.items {
		if item.ID == id {
			found := item
			return &found, nil
		}
	}
	return nil, service.ErrAttachmentNotFound
}

func (f *fakeAttachmentRepo) Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	f.lastUserID = userID
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.lastDeleteID = id
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeAttachmentRepo) StorageKeysByTransaction(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID) ([]string, error) {
	f.lastUserID = userID
	return f.keys, f.keysErr
}

func (f *fakeAttachmentRepo) StorageKeysByAccount(ctx context.Context, userID uuid.UUID, accountID uuid.UUID) ([]string, error) {
	f.lastUserID = userID
	return f.keys, f.keysErr
}

type fakeAttachmentStore struct {
	saved      map[string][]byte
	saveErr    error
	openErr    error
	removeErr  error
	savedKeys  []string
	removed    []string
	stillThere map[string]bool
}

func (f *fakeAttachmentStore) Save(key string, r io.Reader) (int64, error) {
	if f.saveErr != nil {
		return 0, f.saveErr
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	if f.saved == nil {
		f.saved = map[string][]byte{}
	}
	f.saved[key] = data
	f.savedKeys = append(f.savedKeys, key)

	return int64(len(data)), nil
}

func (f *fakeAttachmentStore) Open(key string) (io.ReadCloser, error) {
	if f.openErr != nil {
		return nil, f.openErr
	}
	data, ok := f.saved[key]
	if !ok {
		return nil, os.ErrNotExist
	}

	return io.NopCloser(bytes.NewReader(data)), nil
}

func (f *fakeAttachmentStore) Remove(key string) error {
	if f.removeErr != nil {
		return f.removeErr
	}
	f.removed = append(f.removed, key)
	delete(f.saved, key)

	return nil
}

type fakeAttachmentTransactions struct {
	transaction *model.Transaction
	err         error
	lastID      uuid.UUID
}

func (f *fakeAttachmentTransactions) GetTransaction(ctx context.Context, id uuid.UUID) (*model.Transaction, error) {
	f.lastID = id
	if f.err != nil {
		return nil, f.err
	}
	if f.transaction != nil {
		return f.transaction, nil
	}

	return nil, service.ErrTransactionNotFound
}

// pngPayload starts with the PNG signature so http.DetectContentType reports
// image/png, exactly like a real receipt would.
var pngPayload = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0x00}, 32)...)

func newOwnedAttachmentFixture(t *testing.T) (uuid.UUID, uuid.UUID, *fakeAttachmentRepo, *fakeAttachmentStore, *fakeAttachmentTransactions, service.AttachmentService) {
	t.Helper()

	userID := uuid.New()
	transactionID := uuid.New()
	repo := &fakeAttachmentRepo{}
	store := &fakeAttachmentStore{}
	transactions := &fakeAttachmentTransactions{
		transaction: &model.Transaction{ID: transactionID, UserID: userID},
	}

	return userID, transactionID, repo, store, transactions, service.NewAttachmentService(repo, transactions, store)
}

func TestUploadRejectsOversizedAndEmptyFiles(t *testing.T) {
	t.Parallel()

	for _, size := range []int64{0, -1, service.MaxAttachmentBytes + 1} {
		t.Run(fmt.Sprintf("size %d", size), func(t *testing.T) {
			userID, transactionID, _, _, _, attachments := newOwnedAttachmentFixture(t)

			_, err := attachments.Upload(context.Background(), userID, transactionID, "receipt.png", "image/png", bytes.NewReader(pngPayload), size)
			if !errors.Is(err, service.ErrAttachmentTooLarge) {
				t.Fatalf("expected ErrAttachmentTooLarge for size %d, got %v", size, err)
			}
		})
	}
}

func TestUploadRejectsUnsupportedTypes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		declaredType string
		payload      []byte
	}{
		{name: "plain text", declaredType: "text/plain", payload: []byte("just text")},
		{name: "svg", declaredType: "image/svg+xml", payload: []byte("<svg></svg>")},
		{name: "html declared as png", declaredType: "image/png", payload: []byte("<!DOCTYPE html><html></html>")},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			userID, transactionID, _, store, _, attachments := newOwnedAttachmentFixture(t)

			_, err := attachments.Upload(context.Background(), userID, transactionID, "file", testCase.declaredType, bytes.NewReader(testCase.payload), int64(len(testCase.payload)))
			if !errors.Is(err, service.ErrUnsupportedAttachmentType) {
				t.Fatalf("expected ErrUnsupportedAttachmentType, got %v", err)
			}
			if len(store.saved) != 0 {
				t.Fatalf("expected no bytes to be stored, got %d files", len(store.saved))
			}
		})
	}
}

func TestUploadReturnsTransactionNotFoundWhenTransactionIsMissing(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	transactionID := uuid.New()
	repo := &fakeAttachmentRepo{}
	store := &fakeAttachmentStore{}
	transactions := &fakeAttachmentTransactions{err: service.ErrTransactionNotFound}
	attachments := service.NewAttachmentService(repo, transactions, store)

	_, err := attachments.Upload(context.Background(), userID, transactionID, "receipt.png", "image/png", bytes.NewReader(pngPayload), int64(len(pngPayload)))
	if !errors.Is(err, service.ErrTransactionNotFound) {
		t.Fatalf("expected ErrTransactionNotFound, got %v", err)
	}
}

func TestUploadReturnsTransactionNotFoundWhenTransactionBelongsToAnotherUser(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	transactionID := uuid.New()
	repo := &fakeAttachmentRepo{}
	store := &fakeAttachmentStore{}
	transactions := &fakeAttachmentTransactions{
		transaction: &model.Transaction{ID: transactionID, UserID: uuid.New()},
	}
	attachments := service.NewAttachmentService(repo, transactions, store)

	_, err := attachments.Upload(context.Background(), userID, transactionID, "receipt.png", "image/png", bytes.NewReader(pngPayload), int64(len(pngPayload)))
	if !errors.Is(err, service.ErrTransactionNotFound) {
		t.Fatalf("expected ErrTransactionNotFound, got %v", err)
	}
}

func TestUploadWritesBytesAndMetadata(t *testing.T) {
	t.Parallel()

	userID, transactionID, repo, store, _, attachments := newOwnedAttachmentFixture(t)

	created, err := attachments.Upload(context.Background(), userID, transactionID, "  receipt.png  ", "image/png", bytes.NewReader(pngPayload), int64(len(pngPayload)))
	if err != nil {
		t.Fatalf("expected upload to succeed, got error: %v", err)
	}
	if created == nil {
		t.Fatal("expected created attachment metadata")
	}

	if len(store.savedKeys) != 1 {
		t.Fatalf("expected exactly one stored file, got %d", len(store.savedKeys))
	}
	key := store.savedKeys[0]
	if !strings.HasSuffix(key, ".png") {
		t.Fatalf("expected .png storage key, got %q", key)
	}
	if strings.Contains(key, "receipt") {
		t.Fatalf("expected storage key to ignore the uploaded filename, got %q", key)
	}
	if !bytes.Equal(store.saved[key], pngPayload) {
		t.Fatal("expected stored bytes to match the upload")
	}

	if repo.lastCreate.Filename != "receipt.png" {
		t.Fatalf("expected trimmed filename %q, got %q", "receipt.png", repo.lastCreate.Filename)
	}
	if repo.lastCreate.ContentType != "image/png" {
		t.Fatalf("expected image/png, got %q", repo.lastCreate.ContentType)
	}
	if repo.lastCreate.SizeBytes != int64(len(pngPayload)) {
		t.Fatalf("expected size %d, got %d", len(pngPayload), repo.lastCreate.SizeBytes)
	}
	if repo.lastCreate.TransactionID != transactionID {
		t.Fatalf("expected transaction %s, got %s", transactionID, repo.lastCreate.TransactionID)
	}
	if repo.lastUserID != userID {
		t.Fatalf("expected user %s, got %s", userID, repo.lastUserID)
	}
}

func TestUploadFallsBackToDefaultFilename(t *testing.T) {
	t.Parallel()

	userID, transactionID, repo, _, _, attachments := newOwnedAttachmentFixture(t)

	if _, err := attachments.Upload(context.Background(), userID, transactionID, "   ", "image/png", bytes.NewReader(pngPayload), int64(len(pngPayload))); err != nil {
		t.Fatalf("expected upload to succeed, got error: %v", err)
	}
	if repo.lastCreate.Filename != "attachment" {
		t.Fatalf("expected fallback filename, got %q", repo.lastCreate.Filename)
	}
}

func TestUploadAcceptsDeclaredHeicWhenSniffedAsOctetStream(t *testing.T) {
	t.Parallel()

	userID, transactionID, repo, store, _, attachments := newOwnedAttachmentFixture(t)
	payload := []byte("heic-bytes")

	if _, err := attachments.Upload(context.Background(), userID, transactionID, "photo.heic", "image/heic", bytes.NewReader(payload), int64(len(payload))); err != nil {
		t.Fatalf("expected HEIC upload to succeed, got error: %v", err)
	}
	if repo.lastCreate.ContentType != "image/heic" {
		t.Fatalf("expected image/heic, got %q", repo.lastCreate.ContentType)
	}
	if len(store.savedKeys) != 1 || !strings.HasSuffix(store.savedKeys[0], ".heic") {
		t.Fatalf("expected .heic storage key, got %v", store.savedKeys)
	}
}

func TestUploadRemovesStoredFileWhenMetadataInsertFails(t *testing.T) {
	t.Parallel()

	userID, transactionID, repo, store, _, attachments := newOwnedAttachmentFixture(t)
	repo.createErr = errors.New("insert failed")

	if _, err := attachments.Upload(context.Background(), userID, transactionID, "receipt.png", "image/png", bytes.NewReader(pngPayload), int64(len(pngPayload))); err == nil {
		t.Fatal("expected metadata failure to surface")
	}
	if len(store.savedKeys) != 1 {
		t.Fatalf("expected one save attempt, got %d", len(store.savedKeys))
	}
	if len(store.removed) != 1 || store.removed[0] != store.savedKeys[0] {
		t.Fatalf("expected the stored file to be removed, removed=%v saved=%v", store.removed, store.savedKeys)
	}
}

func TestListReturnsRepositoryAttachments(t *testing.T) {
	t.Parallel()

	userID, transactionID, repo, _, _, attachments := newOwnedAttachmentFixture(t)
	repo.items = []model.Attachment{{ID: uuid.New(), TransactionID: transactionID, Filename: "a.pdf"}}

	items, err := attachments.List(context.Background(), userID, transactionID)
	if err != nil {
		t.Fatalf("expected list to succeed, got error: %v", err)
	}
	if len(items) != 1 || items[0].Filename != "a.pdf" {
		t.Fatalf("expected one attachment, got %v", items)
	}
}

func TestListReturnsEmptySliceWhenRepositoryReturnsNil(t *testing.T) {
	t.Parallel()

	userID, transactionID, repo, _, _, attachments := newOwnedAttachmentFixture(t)
	repo.items = nil

	items, err := attachments.List(context.Background(), userID, transactionID)
	if err != nil {
		t.Fatalf("expected list to succeed, got error: %v", err)
	}
	if items == nil {
		t.Fatal("expected a non-nil empty slice so JSON encodes []")
	}
	if len(items) != 0 {
		t.Fatalf("expected no attachments, got %d", len(items))
	}
}

func TestDeleteRemovesRowAndFile(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	attachmentID := uuid.New()
	repo := &fakeAttachmentRepo{item: &model.Attachment{ID: attachmentID, UserID: userID, StorageKey: "stored.png"}}
	store := &fakeAttachmentStore{}
	attachments := service.NewAttachmentService(repo, &fakeAttachmentTransactions{}, store)

	if err := attachments.Delete(context.Background(), userID, attachmentID); err != nil {
		t.Fatalf("expected delete to succeed, got error: %v", err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != attachmentID {
		t.Fatalf("expected row %s deleted, got %v", attachmentID, repo.deleted)
	}
	if len(store.removed) != 1 || store.removed[0] != "stored.png" {
		t.Fatalf("expected file removal, got %v", store.removed)
	}
}

func TestDeleteToleratesFailedFileRemoval(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	attachmentID := uuid.New()
	repo := &fakeAttachmentRepo{item: &model.Attachment{ID: attachmentID, UserID: userID, StorageKey: "stored.png"}}
	store := &fakeAttachmentStore{removeErr: errors.New("disk offline")}
	attachments := service.NewAttachmentService(repo, &fakeAttachmentTransactions{}, store)

	if err := attachments.Delete(context.Background(), userID, attachmentID); err != nil {
		t.Fatalf("expected delete to succeed despite file removal failure, got %v", err)
	}
	if len(repo.deleted) != 1 {
		t.Fatalf("expected metadata row deleted, got %v", repo.deleted)
	}
}

func TestDeleteReturnsNotFoundForUnknownAttachment(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := &fakeAttachmentRepo{getErr: service.ErrAttachmentNotFound}
	store := &fakeAttachmentStore{}
	attachments := service.NewAttachmentService(repo, &fakeAttachmentTransactions{}, store)

	if err := attachments.Delete(context.Background(), userID, uuid.New()); !errors.Is(err, service.ErrAttachmentNotFound) {
		t.Fatalf("expected ErrAttachmentNotFound, got %v", err)
	}
}

func TestStorageKeysForTransactionAndAccount(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := &fakeAttachmentRepo{keys: []string{"a.png", "b.pdf"}}
	attachments := service.NewAttachmentService(repo, &fakeAttachmentTransactions{}, &fakeAttachmentStore{})

	transactionKeys, err := attachments.StorageKeysForTransaction(context.Background(), userID, uuid.New())
	if err != nil {
		t.Fatalf("expected transaction keys, got error: %v", err)
	}
	if len(transactionKeys) != 2 || transactionKeys[0] != "a.png" || transactionKeys[1] != "b.pdf" {
		t.Fatalf("unexpected transaction keys: %v", transactionKeys)
	}

	accountKeys, err := attachments.StorageKeysForAccount(context.Background(), userID, uuid.New())
	if err != nil {
		t.Fatalf("expected account keys, got error: %v", err)
	}
	if len(accountKeys) != 2 {
		t.Fatalf("unexpected account keys: %v", accountKeys)
	}
}

func TestRemoveFilesDeletesEachKey(t *testing.T) {
	t.Parallel()

	store := &fakeAttachmentStore{}
	attachments := service.NewAttachmentService(&fakeAttachmentRepo{}, &fakeAttachmentTransactions{}, store)

	attachments.RemoveFiles([]string{"a.png", "b.pdf"})

	if len(store.removed) != 2 || store.removed[0] != "a.png" || store.removed[1] != "b.pdf" {
		t.Fatalf("expected both files removed, got %v", store.removed)
	}
}

func TestOpenReturnsMetadataAndFile(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	attachmentID := uuid.New()
	repo := &fakeAttachmentRepo{item: &model.Attachment{
		ID:          attachmentID,
		UserID:      userID,
		Filename:    "receipt.png",
		ContentType: "image/png",
		StorageKey:  "key.png",
	}}
	store := &fakeAttachmentStore{saved: map[string][]byte{"key.png": pngPayload}}
	attachments := service.NewAttachmentService(repo, &fakeAttachmentTransactions{}, store)

	item, reader, err := attachments.Open(context.Background(), userID, attachmentID)
	if err != nil {
		t.Fatalf("expected open to succeed, got error: %v", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	if item.Filename != "receipt.png" {
		t.Fatalf("unexpected metadata: %+v", item)
	}
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("expected read to succeed, got error: %v", err)
	}
	if !bytes.Equal(got, pngPayload) {
		t.Fatal("expected stored bytes")
	}
}
