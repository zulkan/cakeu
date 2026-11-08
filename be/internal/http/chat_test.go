package httpserver_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"

	httpserver "goskeleton/internal/http"
	"goskeleton/internal/model"
	"goskeleton/internal/service"
	"goskeleton/internal/spreadsheet"
)

type fakeChatService struct {
	response   *model.ChatResponse
	history    *model.ChatHistoryResponse
	file       *model.ChatFile
	err        error
	cleared    bool
	lastUserID uuid.UUID
	lastID     string
	lastBody   model.ChatRequest
}

func (f *fakeChatService) Handle(ctx context.Context, userID uuid.UUID, req model.ChatRequest) (*model.ChatResponse, error) {
	f.lastUserID = userID
	f.lastBody = req
	if f.err != nil {
		return nil, f.err
	}
	if f.response != nil {
		return f.response, nil
	}
	return &model.ChatResponse{Reply: "ok", ToolCalls: []model.ChatToolCall{}}, nil
}

func (f *fakeChatService) Confirm(ctx context.Context, userID uuid.UUID, id string) (*model.ChatResponse, error) {
	f.lastUserID = userID
	f.lastID = id
	if f.err != nil {
		return nil, f.err
	}
	if f.response != nil {
		return f.response, nil
	}
	return &model.ChatResponse{Reply: "Deleted.", ToolCalls: []model.ChatToolCall{}}, nil
}

func (f *fakeChatService) Cancel(userID uuid.UUID, id string) error {
	f.lastUserID = userID
	f.lastID = id
	return f.err
}

func (f *fakeChatService) AttachFile(ctx context.Context, userID uuid.UUID, filename string, r io.Reader) (*model.ChatFile, error) {
	f.lastUserID = userID
	if f.err != nil {
		return nil, f.err
	}
	if f.file != nil {
		return f.file, nil
	}
	return &model.ChatFile{ID: "file-1", Name: filename}, nil
}

func (f *fakeChatService) History(ctx context.Context, userID uuid.UUID) (*model.ChatHistoryResponse, error) {
	f.lastUserID = userID
	if f.err != nil {
		return nil, f.err
	}
	if f.history != nil {
		return f.history, nil
	}
	return &model.ChatHistoryResponse{
		Messages:   []model.ChatHistoryMessage{},
		TokenUsage: model.ChatTokenUsage{Limit: 50_000, Remaining: 50_000},
	}, nil
}

func (f *fakeChatService) Clear(ctx context.Context, userID uuid.UUID) error {
	f.lastUserID = userID
	if f.err != nil {
		return f.err
	}
	f.cleared = true
	return nil
}

func newChatTestHandler(chat httpserver.ChatService) http.Handler {
	return httpserver.NewHandler(service.NewHealth(nil), nil, nil, nil, nil, nil, service.ImportModeAI, nil, nil, nil, chat, nil, testJWTSecret, nil)
}

func TestChatReturnsReply(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	chat := &fakeChatService{response: &model.ChatResponse{
		Reply:     "You have one account.",
		ToolCalls: []model.ChatToolCall{{Name: "list_accounts", Summary: "Listed 1 accounts"}},
	}}
	handler := newChatTestHandler(chat)

	recorder := doRequest(t, handler, http.MethodPost, "/api/chat", signToken(t, testJWTSecret, userID),
		`{"content":"which accounts do I have?"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}

	var response model.ChatResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Reply != "You have one account." || len(response.ToolCalls) != 1 {
		t.Fatalf("unexpected response: %+v", response)
	}
	if chat.lastUserID != userID {
		t.Fatalf("expected caller %s forwarded, got %s", userID, chat.lastUserID)
	}
	if chat.lastBody.Content != "which accounts do I have?" {
		t.Fatalf("expected the content forwarded, got %q", chat.lastBody.Content)
	}
}

func TestChatReturnsServiceUnavailableWhenNotConfigured(t *testing.T) {
	t.Parallel()

	handler := newChatTestHandler(&fakeChatService{err: service.ErrLLMNotConfigured})

	recorder := doRequest(t, handler, http.MethodPost, "/api/chat", signToken(t, testJWTSecret, uuid.New()),
		`{"content":"hello"}`)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestChatTokenLimitReturnsTooManyRequests(t *testing.T) {
	t.Parallel()

	handler := newChatTestHandler(&fakeChatService{err: service.ErrChatTokenLimitReached})

	recorder := doRequest(t, handler, http.MethodPost, "/api/chat", signToken(t, testJWTSecret, uuid.New()),
		`{"content":"hello"}`)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestChatEmptyContentReturnsBadRequest(t *testing.T) {
	t.Parallel()

	handler := newChatTestHandler(&fakeChatService{err: service.ErrChatMessageRequired})

	recorder := doRequest(t, handler, http.MethodPost, "/api/chat", signToken(t, testJWTSecret, uuid.New()),
		`{"content":""}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestChatHistoryReturnsMessages(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	messageID := uuid.New()
	chat := &fakeChatService{history: &model.ChatHistoryResponse{
		Messages: []model.ChatHistoryMessage{{
			ID:      messageID,
			Role:    "user",
			Content: "hi",
		}},
		TokenUsage: model.ChatTokenUsage{Used: 1, Limit: 50_000, Remaining: 49_999},
	}}
	handler := newChatTestHandler(chat)

	recorder := doRequest(t, handler, http.MethodGet, "/api/chat/history", signToken(t, testJWTSecret, userID), "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}

	var history model.ChatHistoryResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &history); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(history.Messages) != 1 || history.Messages[0].ID != messageID {
		t.Fatalf("unexpected messages: %+v", history.Messages)
	}
	if history.TokenUsage.Used != 1 || history.TokenUsage.Remaining != 49_999 {
		t.Fatalf("unexpected token usage: %+v", history.TokenUsage)
	}
	if chat.lastUserID != userID {
		t.Fatalf("expected caller %s forwarded, got %s", userID, chat.lastUserID)
	}
}

func TestChatClearReturnsMessage(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	chat := &fakeChatService{}
	handler := newChatTestHandler(chat)

	recorder := doRequest(t, handler, http.MethodDelete, "/api/chat/history", signToken(t, testJWTSecret, userID), "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}

	var response model.MessageResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Message != "chat cleared" {
		t.Fatalf("unexpected message %q", response.Message)
	}
	if !chat.cleared {
		t.Fatalf("expected the service to clear the chat")
	}
	if chat.lastUserID != userID {
		t.Fatalf("expected caller %s forwarded, got %s", userID, chat.lastUserID)
	}
}

func TestChatConfirmUnknownActionReturnsNotFound(t *testing.T) {
	t.Parallel()

	handler := newChatTestHandler(&fakeChatService{err: service.ErrPendingActionNotFound})

	recorder := doRequest(t, handler, http.MethodPost, "/api/chat/actions/abc/confirm", signToken(t, testJWTSecret, uuid.New()), "")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestChatRequiresAuthentication(t *testing.T) {
	t.Parallel()

	handler := newChatTestHandler(&fakeChatService{})

	recorder := doRequest(t, handler, http.MethodPost, "/api/chat", "",
		`{"content":"hello"}`)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Code)
	}
}

func TestChatFileUpload(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	chat := &fakeChatService{file: &model.ChatFile{
		ID:      "file-1",
		Name:    "rows.csv",
		Sheet:   "csv",
		Rows:    1,
		Columns: []string{"date", "type"},
		Preview: [][]string{{"date", "type"}, {"2026-10-01", "expense"}},
	}}
	handler := newChatTestHandler(chat)

	recorder := doMultipartRequest(t, handler, "/api/chat/files", signToken(t, testJWTSecret, userID),
		"date,type\n2026-10-01,expense\n", false)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}

	var file model.ChatFile
	if err := json.Unmarshal(recorder.Body.Bytes(), &file); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if file.ID != "file-1" || file.Rows != 1 {
		t.Fatalf("unexpected file payload: %+v", file)
	}
	if chat.lastUserID != userID {
		t.Fatalf("expected caller %s forwarded, got %s", userID, chat.lastUserID)
	}
}

func TestChatFileUploadRejectsUnsupportedFormat(t *testing.T) {
	t.Parallel()

	handler := newChatTestHandler(&fakeChatService{err: spreadsheet.ErrUnsupportedFormat})

	recorder := doMultipartRequest(t, handler, "/api/chat/files", signToken(t, testJWTSecret, uuid.New()),
		"not a spreadsheet", false)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestChatStaleAttachmentReturnsBadRequest(t *testing.T) {
	t.Parallel()

	handler := newChatTestHandler(&fakeChatService{err: service.ErrAttachmentNotFound})

	recorder := doRequest(t, handler, http.MethodPost, "/api/chat", signToken(t, testJWTSecret, uuid.New()),
		`{"content":"hi","file_id":"gone"}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

var _ httpserver.ChatService = (*fakeChatService)(nil)
