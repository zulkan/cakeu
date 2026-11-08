package httpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/google/uuid"

	httpserver "goskeleton/internal/http"
	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

type fakeAttachmentService struct {
	items        []model.Attachment
	item         *model.Attachment
	content      []byte
	uploaded     *model.Attachment
	listErr      error
	uploadErr    error
	openErr      error
	deleteErr    error
	keys         []string
	keysErr      error
	removedKeys  []string
	lastUserID   uuid.UUID
	lastID       uuid.UUID
	lastFilename string
	lastType     string
	lastSize     int64
	lastBytes    []byte
}

func (f *fakeAttachmentService) List(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID) ([]model.Attachment, error) {
	f.lastUserID = userID
	f.lastID = transactionID
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.items, nil
}

func (f *fakeAttachmentService) Upload(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID, filename string, declaredType string, r io.Reader, size int64) (*model.Attachment, error) {
	f.lastUserID = userID
	f.lastID = transactionID
	f.lastFilename = filename
	f.lastType = declaredType
	f.lastSize = size
	data, _ := io.ReadAll(r)
	f.lastBytes = data
	if f.uploadErr != nil {
		return nil, f.uploadErr
	}
	if f.uploaded != nil {
		return f.uploaded, nil
	}
	return &model.Attachment{ID: uuid.New(), TransactionID: transactionID, Filename: filename, ContentType: declaredType, SizeBytes: size}, nil
}

func (f *fakeAttachmentService) Open(ctx context.Context, userID uuid.UUID, attachmentID uuid.UUID) (model.Attachment, io.ReadCloser, error) {
	f.lastUserID = userID
	f.lastID = attachmentID
	if f.openErr != nil {
		return model.Attachment{}, nil, f.openErr
	}
	item := model.Attachment{ID: attachmentID, ContentType: "image/png", Filename: "receipt.png"}
	if f.item != nil {
		item = *f.item
	}
	return item, io.NopCloser(bytes.NewReader(f.content)), nil
}

func (f *fakeAttachmentService) Delete(ctx context.Context, userID uuid.UUID, attachmentID uuid.UUID) error {
	f.lastUserID = userID
	f.lastID = attachmentID
	return f.deleteErr
}

func (f *fakeAttachmentService) StorageKeysForTransaction(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID) ([]string, error) {
	f.lastUserID = userID
	f.lastID = transactionID
	return f.keys, f.keysErr
}

func (f *fakeAttachmentService) StorageKeysForAccount(ctx context.Context, userID uuid.UUID, accountID uuid.UUID) ([]string, error) {
	f.lastUserID = userID
	f.lastID = accountID
	return f.keys, f.keysErr
}

func (f *fakeAttachmentService) RemoveFiles(keys []string) {
	f.removedKeys = append(f.removedKeys, keys...)
}

// pngUpload starts with the PNG signature so the service sniffs image/png.
var pngUpload = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0x01}, 16)...)

func newAttachmentTestHandler(attachments httpserver.AttachmentService) http.Handler {
	return httpserver.NewHandler(
		service.NewHealth(nil), nil, &fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{},
		nil, service.ImportModeAI, nil, nil, nil, nil, attachments, testJWTSecret, nil,
	)
}

func newAttachmentCascadeHandler(attachments httpserver.AttachmentService, transactions httpserver.TransactionService, accounts httpserver.AccountService) http.Handler {
	return httpserver.NewHandler(
		service.NewHealth(nil), nil, accounts, &fakeCategoryService{}, transactions,
		nil, service.ImportModeAI, nil, nil, nil, nil, attachments, testJWTSecret, nil,
	)
}

func multipartUpload(t *testing.T, field, filename, contentType string, content []byte) (*bytes.Buffer, string) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, filename))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("create multipart part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	return &body, writer.FormDataContentType()
}

func doAttachmentRequest(t *testing.T, handler http.Handler, method, path, token string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, body)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

func TestAttachmentRoutesRequireAuthentication(t *testing.T) {
	t.Parallel()

	handler := newAttachmentTestHandler(&fakeAttachmentService{})
	id := uuid.New().String()

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/transactions/" + id + "/attachments"},
		{http.MethodPost, "/api/transactions/" + id + "/attachments"},
		{http.MethodGet, "/api/attachments/" + id},
		{http.MethodDelete, "/api/attachments/" + id},
	}

	for _, testCase := range cases {
		recorder := doAttachmentRequest(t, handler, testCase.method, testCase.path, "", nil, "")
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: expected 401, got %d", testCase.method, testCase.path, recorder.Code)
		}
	}
}

func TestUploadAttachmentReturnsCreatedMetadata(t *testing.T) {
	t.Parallel()

	serviceFake := &fakeAttachmentService{uploaded: &model.Attachment{
		ID:            uuid.New(),
		TransactionID: uuid.New(),
		Filename:      "receipt.png",
		ContentType:   "image/png",
		SizeBytes:     int64(len(pngUpload)),
	}}
	handler := newAttachmentTestHandler(serviceFake)
	transactionID := serviceFake.uploaded.TransactionID
	token := signToken(t, testJWTSecret, uuid.New())

	body, contentType := multipartUpload(t, "file", "receipt.png", "image/png", pngUpload)
	recorder := doAttachmentRequest(t, handler, http.MethodPost, "/api/transactions/"+transactionID.String()+"/attachments", token, body, contentType)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if serviceFake.lastFilename != "receipt.png" || serviceFake.lastType != "image/png" {
		t.Fatalf("unexpected upload metadata: filename=%q type=%q", serviceFake.lastFilename, serviceFake.lastType)
	}
	if !bytes.Equal(serviceFake.lastBytes, pngUpload) {
		t.Fatal("expected handler to forward the uploaded bytes")
	}

	var response model.Attachment
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected JSON metadata, got error: %v", err)
	}
	if response.ID != serviceFake.uploaded.ID {
		t.Fatalf("expected attachment %s, got %s", serviceFake.uploaded.ID, response.ID)
	}
	if strings.Contains(recorder.Body.String(), "storage_key") {
		t.Fatal("storage_key must never be exposed")
	}
	if strings.Contains(recorder.Body.String(), "user_id") {
		t.Fatal("user_id must never be exposed")
	}
}

func TestUploadAttachmentRequiresFile(t *testing.T) {
	t.Parallel()

	handler := newAttachmentTestHandler(&fakeAttachmentService{})
	token := signToken(t, testJWTSecret, uuid.New())

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("note", "no file here"); err != nil {
		t.Fatalf("write form field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	recorder := doAttachmentRequest(t, handler, http.MethodPost, "/api/transactions/"+uuid.New().String()+"/attachments", token, &body, writer.FormDataContentType())
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestListAttachmentsReturnsArray(t *testing.T) {
	t.Parallel()

	transactionID := uuid.New()
	serviceFake := &fakeAttachmentService{items: []model.Attachment{{
		ID:            uuid.New(),
		TransactionID: transactionID,
		Filename:      "receipt.pdf",
		ContentType:   "application/pdf",
		SizeBytes:     10,
	}}}
	handler := newAttachmentTestHandler(serviceFake)
	token := signToken(t, testJWTSecret, uuid.New())

	recorder := doAttachmentRequest(t, handler, http.MethodGet, "/api/transactions/"+transactionID.String()+"/attachments", token, nil, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}

	var items []model.Attachment
	if err := json.Unmarshal(recorder.Body.Bytes(), &items); err != nil {
		t.Fatalf("expected JSON array, got error: %v", err)
	}
	if len(items) != 1 || items[0].Filename != "receipt.pdf" {
		t.Fatalf("unexpected attachments: %v", items)
	}
}

func TestListAttachmentsEncodesEmptyArray(t *testing.T) {
	t.Parallel()

	handler := newAttachmentTestHandler(&fakeAttachmentService{items: []model.Attachment{}})
	token := signToken(t, testJWTSecret, uuid.New())

	recorder := doAttachmentRequest(t, handler, http.MethodGet, "/api/transactions/"+uuid.New().String()+"/attachments", token, nil, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if strings.TrimSpace(recorder.Body.String()) != "[]" {
		t.Fatalf("expected [] for an empty list, got %q", recorder.Body.String())
	}
}

func TestDeleteAttachmentReturnsNoContent(t *testing.T) {
	t.Parallel()

	attachmentID := uuid.New()
	serviceFake := &fakeAttachmentService{}
	handler := newAttachmentTestHandler(serviceFake)
	token := signToken(t, testJWTSecret, uuid.New())

	recorder := doAttachmentRequest(t, handler, http.MethodDelete, "/api/attachments/"+attachmentID.String(), token, nil, "")
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if serviceFake.lastID != attachmentID {
		t.Fatalf("expected attachment %s passed to the service, got %s", attachmentID, serviceFake.lastID)
	}
}

func TestDownloadAttachmentSetsHeadersAndStreamsBytes(t *testing.T) {
	t.Parallel()

	attachmentID := uuid.New()
	serviceFake := &fakeAttachmentService{
		item: &model.Attachment{
			ID:          attachmentID,
			Filename:    "receipt\"\r\nX-Injected: yes.png",
			ContentType: "image/png",
		},
		content: pngUpload,
	}
	handler := newAttachmentTestHandler(serviceFake)
	token := signToken(t, testJWTSecret, uuid.New())

	recorder := doAttachmentRequest(t, handler, http.MethodGet, "/api/attachments/"+attachmentID.String(), token, nil, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("expected stored content type, got %q", got)
	}
	disposition := recorder.Header().Get("Content-Disposition")
	if !strings.HasPrefix(disposition, "inline") {
		t.Fatalf("expected inline disposition, got %q", disposition)
	}
	if strings.ContainsAny(disposition, "\r\n\"") {
		t.Fatalf("expected CR/LF/quotes stripped from the disposition, got %q", disposition)
	}
	if !bytes.Equal(recorder.Body.Bytes(), pngUpload) {
		t.Fatal("expected streamed attachment bytes")
	}
}

func TestDownloadAttachmentReturnsNotFound(t *testing.T) {
	t.Parallel()

	handler := newAttachmentTestHandler(&fakeAttachmentService{openErr: service.ErrAttachmentNotFound})
	token := signToken(t, testJWTSecret, uuid.New())

	recorder := doAttachmentRequest(t, handler, http.MethodGet, "/api/attachments/"+uuid.New().String(), token, nil, "")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestUploadAttachmentMapsServiceErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		err    error
		status int
	}{
		{name: "too large", err: service.ErrAttachmentTooLarge, status: http.StatusRequestEntityTooLarge},
		{name: "unsupported", err: service.ErrUnsupportedAttachmentType, status: http.StatusUnsupportedMediaType},
		{name: "unknown transaction", err: service.ErrTransactionNotFound, status: http.StatusNotFound},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			handler := newAttachmentTestHandler(&fakeAttachmentService{uploadErr: testCase.err})
			token := signToken(t, testJWTSecret, uuid.New())
			body, contentType := multipartUpload(t, "file", "receipt.png", "image/png", pngUpload)

			recorder := doAttachmentRequest(t, handler, http.MethodPost, "/api/transactions/"+uuid.New().String()+"/attachments", token, body, contentType)
			if recorder.Code != testCase.status {
				t.Fatalf("expected %d, got %d with body %q", testCase.status, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestUploadAttachmentRejectsBodyOverLimit(t *testing.T) {
	t.Parallel()

	handler := newAttachmentTestHandler(&fakeAttachmentService{})
	token := signToken(t, testJWTSecret, uuid.New())
	oversized := bytes.Repeat([]byte{0x01}, 7<<20)

	body, contentType := multipartUpload(t, "file", "big.png", "image/png", oversized)
	recorder := doAttachmentRequest(t, handler, http.MethodPost, "/api/transactions/"+uuid.New().String()+"/attachments", token, body, contentType)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestDeleteTransactionRemovesAttachmentFiles(t *testing.T) {
	t.Parallel()

	attachmentFake := &fakeAttachmentService{keys: []string{"a.png", "b.pdf"}}
	handler := newAttachmentCascadeHandler(attachmentFake, &fakeTransactionService{}, &fakeAccountService{})
	token := signToken(t, testJWTSecret, uuid.New())

	recorder := doAttachmentRequest(t, handler, http.MethodDelete, "/api/transactions/"+uuid.New().String(), token, nil, "")
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if len(attachmentFake.removedKeys) != 2 || attachmentFake.removedKeys[0] != "a.png" {
		t.Fatalf("expected collected keys removed, got %v", attachmentFake.removedKeys)
	}
}

func TestDeleteTransactionStillSucceedsWhenKeyCollectionFails(t *testing.T) {
	t.Parallel()

	attachmentFake := &fakeAttachmentService{keysErr: fmt.Errorf("db down")}
	handler := newAttachmentCascadeHandler(attachmentFake, &fakeTransactionService{}, &fakeAccountService{})
	token := signToken(t, testJWTSecret, uuid.New())

	recorder := doAttachmentRequest(t, handler, http.MethodDelete, "/api/transactions/"+uuid.New().String(), token, nil, "")
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestDeleteAccountRemovesAttachmentFiles(t *testing.T) {
	t.Parallel()

	attachmentFake := &fakeAttachmentService{keys: []string{"a.png"}}
	handler := newAttachmentCascadeHandler(attachmentFake, &fakeTransactionService{}, &fakeAccountService{})
	token := signToken(t, testJWTSecret, uuid.New())

	recorder := doAttachmentRequest(t, handler, http.MethodDelete, "/api/accounts/"+uuid.New().String(), token, nil, "")
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if len(attachmentFake.removedKeys) != 1 || attachmentFake.removedKeys[0] != "a.png" {
		t.Fatalf("expected collected keys removed, got %v", attachmentFake.removedKeys)
	}
}
