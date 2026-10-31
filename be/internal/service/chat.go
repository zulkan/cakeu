package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/llm"
	"goskeleton/internal/model"
)

var (
	ErrLLMNotConfigured      = errors.New("chat is not configured")
	ErrChatTooManySteps      = errors.New("chat exceeded the tool iteration limit")
	ErrPendingActionNotFound = errors.New("pending action not found")
)

const (
	maxChatIterations        = 5
	pendingActionTTL         = 5 * time.Minute
	maxToolResultRows        = 100
	chatDateLayout           = "2006-01-02"
	pendingDeleteTransaction = "delete_transaction"
	pendingDeleteAccount     = "delete_account"
	pendingDeleteCategory    = "delete_category"
	chatSystemPromptText     = `You are Catatku's finance assistant. You can only act through the provided tools.
Rules:
- Never invent amounts, dates or balances. If you need data, call a tool.
- Refer to accounts and categories by their exact names; the server resolves names to records.
- Before deleting anything, call the delete tool; the user must confirm before it executes.
- If a tool returns an error, explain it plainly instead of retrying blindly.
- Answer briefly and in the user's language.`
)

type pendingAction struct {
	userID    uuid.UUID
	kind      string
	targetID  uuid.UUID
	summary   string
	expiresAt time.Time
}

type ChatService struct {
	accounts     AccountService
	categories   CategoryService
	transactions TransactionService
	client       llm.Client
	logger       *slog.Logger

	mu      sync.Mutex
	pending map[string]pendingAction
}

func NewChatService(accounts AccountService, categories CategoryService, transactions TransactionService, client llm.Client, logger *slog.Logger) *ChatService {
	if logger == nil {
		logger = slog.Default()
	}

	return &ChatService{
		accounts:     accounts,
		categories:   categories,
		transactions: transactions,
		client:       client,
		logger:       logger,
		pending:      make(map[string]pendingAction),
	}
}

func (s *ChatService) Handle(ctx context.Context, userID uuid.UUID, req model.ChatRequest) (*model.ChatResponse, error) {
	messages := []llm.Message{{Role: "system", Content: chatSystemPromptText}}
	for _, message := range req.Messages {
		if message.Role != "user" && message.Role != "assistant" {
			continue
		}
		if strings.TrimSpace(message.Content) == "" {
			continue
		}
		messages = append(messages, llm.Message{Role: message.Role, Content: message.Content})
	}

	if len(messages) == 1 {
		return nil, errors.New("messages are required")
	}

	response := &model.ChatResponse{ToolCalls: []model.ChatToolCall{}}

	for i := 0; i < maxChatIterations; i++ {
		reply, err := s.client.Complete(ctx, messages, chatTools)
		if err != nil {
			if errors.Is(err, llm.ErrNotConfigured) {
				return nil, ErrLLMNotConfigured
			}
			return nil, fmt.Errorf("chat completion: %w", err)
		}

		messages = append(messages, *reply)

		if len(reply.ToolCalls) == 0 {
			response.Reply = strings.TrimSpace(reply.Content)
			if response.Reply == "" {
				response.Reply = "I could not produce an answer."
			}
			return response, nil
		}

		for _, call := range reply.ToolCalls {
			content, summary, pending, err := s.executeTool(ctx, userID, call)
			if err != nil {
				response.ToolCalls = append(response.ToolCalls, model.ChatToolCall{Name: call.Function.Name, Summary: "failed"})
				messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, Content: "error: " + err.Error()})
				continue
			}

			if pending != nil {
				response.PendingAction = pending
				response.ToolCalls = append(response.ToolCalls, model.ChatToolCall{Name: call.Function.Name, Summary: summary})
				response.Reply = summary
				return response, nil
			}

			response.ToolCalls = append(response.ToolCalls, model.ChatToolCall{Name: call.Function.Name, Summary: summary})
			messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, Content: content})
		}
	}

	return nil, ErrChatTooManySteps
}

func (s *ChatService) Confirm(ctx context.Context, userID uuid.UUID, id string) (string, error) {
	action, ok := s.takePending(userID, id)
	if !ok {
		return "", ErrPendingActionNotFound
	}

	var err error
	switch action.kind {
	case pendingDeleteTransaction:
		err = s.transactions.DeleteTransaction(ctx, userID, action.targetID)
	case pendingDeleteAccount:
		err = s.accounts.DeleteAccount(ctx, userID, action.targetID)
	case pendingDeleteCategory:
		err = s.categories.DeleteCategory(ctx, userID, action.targetID)
	default:
		return "", ErrPendingActionNotFound
	}

	if err != nil {
		return "", fmt.Errorf("confirm delete: %w", err)
	}

	return action.summary, nil
}

func (s *ChatService) Cancel(userID uuid.UUID, id string) error {
	if _, ok := s.takePending(userID, id); !ok {
		return ErrPendingActionNotFound
	}
	return nil
}

func (s *ChatService) takePending(userID uuid.UUID, id string) (pendingAction, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked()

	action, ok := s.pending[id]
	if !ok || action.userID != userID || time.Now().After(action.expiresAt) {
		return pendingAction{}, false
	}

	delete(s.pending, id)
	return action, true
}

func (s *ChatService) storePending(userID uuid.UUID, kind string, targetID uuid.UUID, summary string) model.ChatPendingAction {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked()

	id := randomToken()
	s.pending[id] = pendingAction{
		userID:    userID,
		kind:      kind,
		targetID:  targetID,
		summary:   summary,
		expiresAt: time.Now().Add(pendingActionTTL),
	}

	return model.ChatPendingAction{ID: id, Summary: summary}
}

func (s *ChatService) pruneLocked() {
	now := time.Now()
	for id, action := range s.pending {
		if now.After(action.expiresAt) {
			delete(s.pending, id)
		}
	}
}

func randomToken() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return uuid.NewString()
	}
	return hex.EncodeToString(buffer)
}

func (s *ChatService) executeTool(ctx context.Context, userID uuid.UUID, call llm.ToolCall) (string, string, *model.ChatPendingAction, error) {
	switch call.Function.Name {
	case "list_accounts":
		return s.listAccounts(ctx, userID)
	case "list_categories":
		return s.listCategories(ctx, userID)
	case "list_transactions":
		return s.listTransactions(ctx, userID, call.Function.Arguments)
	case "get_dashboard_summary":
		return s.dashboardSummary(ctx, userID)
	case "create_transaction":
		return s.createTransaction(ctx, userID, call.Function.Arguments)
	case "delete_transaction":
		return s.requestDeleteTransaction(ctx, userID, call.Function.Arguments)
	case "create_account":
		return s.createAccount(ctx, userID, call.Function.Arguments)
	case "create_category":
		return s.createCategory(ctx, userID, call.Function.Arguments)
	case "update_account":
		return s.updateAccount(ctx, userID, call.Function.Arguments)
	case "delete_account":
		return s.requestDeleteAccount(ctx, userID, call.Function.Arguments)
	case "delete_category":
		return s.requestDeleteCategory(ctx, userID, call.Function.Arguments)
	default:
		return "", "", nil, fmt.Errorf("unknown tool %q", call.Function.Name)
	}
}
