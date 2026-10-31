package httpserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	httpserver "goskeleton/internal/http"
	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

type fakeChatService struct {
	response   *model.ChatResponse
	err        error
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

func (f *fakeChatService) Confirm(ctx context.Context, userID uuid.UUID, id string) (string, error) {
	f.lastUserID = userID
	f.lastID = id
	if f.err != nil {
		return "", f.err
	}
	return "Deleted.", nil
}

func (f *fakeChatService) Cancel(userID uuid.UUID, id string) error {
	f.lastUserID = userID
	f.lastID = id
	return f.err
}

func newChatTestHandler(chat httpserver.ChatService) http.Handler {
	return httpserver.NewHandler(service.NewHealth(nil), nil, nil, nil, nil, chat, testJWTSecret, nil)
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
		`{"messages":[{"role":"user","content":"which accounts do I have?"}]}`)

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
	if len(chat.lastBody.Messages) != 1 {
		t.Fatalf("expected one message forwarded, got %d", len(chat.lastBody.Messages))
	}
}

func TestChatReturnsServiceUnavailableWhenNotConfigured(t *testing.T) {
	t.Parallel()

	handler := newChatTestHandler(&fakeChatService{err: service.ErrLLMNotConfigured})

	recorder := doRequest(t, handler, http.MethodPost, "/api/chat", signToken(t, testJWTSecret, uuid.New()),
		`{"messages":[{"role":"user","content":"hello"}]}`)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d with body %q", recorder.Code, recorder.Body.String())
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
		`{"messages":[{"role":"user","content":"hello"}]}`)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Code)
	}
}

var _ httpserver.ChatService = (*fakeChatService)(nil)
