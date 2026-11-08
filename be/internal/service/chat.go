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
	ErrChatTokenLimitReached = errors.New("chat token limit reached; clear the chat to continue")
	ErrChatMessageRequired   = errors.New("content is required")
)

const (
	chatTokenLimit           = 50_000
	maxChatIterations        = 5
	pendingActionTTL         = 5 * time.Minute
	maxToolResultRows        = 100
	chatDateLayout           = "2006-01-02"
	pendingDeleteTransaction = "delete_transaction"
	pendingDeleteAccount     = "delete_account"
	pendingDeleteCategory    = "delete_category"
	pendingImportFile        = "import_file"
	chatSystemPromptText     = `You are Catatku's finance assistant. You can only act through the provided tools.
Rules:
- Never invent amounts, dates or balances. If you need data, call a tool.
- Refer to accounts and categories by their exact names; the server resolves names to records.
- An account's opening balance is a locked ledger entry. To correct a balance, use adjust_account_balance; never try to overwrite it.
- Before deleting anything, call the delete tool; the user must confirm before it executes.
- If a tool returns an error, explain it plainly instead of retrying blindly.
- Answer briefly and in the user's language.`
)

type pendingAction struct {
	userID    uuid.UUID
	kind      string
	targetID  uuid.UUID
	fileID    string
	sheet     string
	summary   string
	expiresAt time.Time
}

type ChatHistoryRepository interface {
	ListMessages(ctx context.Context, userID uuid.UUID) ([]model.ChatHistoryMessage, error)
	AppendMessages(ctx context.Context, userID uuid.UUID, messages []model.NewChatMessage) error
	Clear(ctx context.Context, userID uuid.UUID) error
	TokenTotal(ctx context.Context, userID uuid.UUID) (int, error)
}

type ChatService struct {
	accounts     AccountService
	categories   CategoryService
	transactions TransactionService
	budgets      BudgetService
	recurring    RecurringService
	reports      ReportService
	history      ChatHistoryRepository
	client       llm.Client
	logger       *slog.Logger

	mu      sync.Mutex
	pending map[string]pendingAction
	files   map[string]storedChatFile
}

func NewChatService(accounts AccountService, categories CategoryService, transactions TransactionService, budgets BudgetService, recurring RecurringService, reports ReportService, history ChatHistoryRepository, client llm.Client, logger *slog.Logger) *ChatService {
	if logger == nil {
		logger = slog.Default()
	}

	return &ChatService{
		accounts:     accounts,
		categories:   categories,
		transactions: transactions,
		budgets:      budgets,
		recurring:    recurring,
		reports:      reports,
		history:      history,
		client:       client,
		logger:       logger,
		pending:      make(map[string]pendingAction),
		files:        make(map[string]storedChatFile),
	}
}

func (s *ChatService) Handle(ctx context.Context, userID uuid.UUID, req model.ChatRequest) (*model.ChatResponse, error) {
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, ErrChatMessageRequired
	}

	used, err := s.history.TokenTotal(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("read chat token total: %w", err)
	}
	if used >= chatTokenLimit {
		return nil, ErrChatTokenLimitReached
	}

	stored, err := s.history.ListMessages(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("read chat history: %w", err)
	}

	messages := []llm.Message{{Role: "system", Content: chatSystemPromptText}}

	fileID := strings.TrimSpace(req.FileID)
	sheet := strings.TrimSpace(req.Sheet)
	if fileID != "" {
		file, ok := s.lookupFile(userID, fileID)
		if !ok {
			return nil, fmt.Errorf("%w; ask the user to upload it again", ErrAttachmentNotFound)
		}

		context, contextErr := s.fileContext(file, sheet)
		if contextErr != nil {
			return nil, contextErr
		}
		messages = append(messages, llm.Message{Role: "system", Content: context})
	}

	for _, message := range stored {
		if message.Role != "user" && message.Role != "assistant" {
			continue
		}
		if strings.TrimSpace(message.Content) == "" {
			continue
		}
		messages = append(messages, llm.Message{Role: message.Role, Content: message.Content})
	}

	messages = append(messages, llm.Message{Role: "user", Content: content})

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
			return s.finishTurn(ctx, userID, content, used, response)
		}

		for _, call := range reply.ToolCalls {
			content, summary, pending, err := s.executeTool(ctx, userID, fileID, sheet, call)
			if err != nil {
				response.ToolCalls = append(response.ToolCalls, model.ChatToolCall{Name: call.Function.Name, Summary: "failed"})
				messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, Content: "error: " + err.Error()})
				continue
			}

			if pending != nil {
				response.PendingAction = pending
				response.ToolCalls = append(response.ToolCalls, model.ChatToolCall{Name: call.Function.Name, Summary: summary})
				response.Reply = summary
				return s.finishTurn(ctx, userID, content, used, response)
			}

			response.ToolCalls = append(response.ToolCalls, model.ChatToolCall{Name: call.Function.Name, Summary: summary})
			messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, Content: content})
		}
	}

	return nil, ErrChatTooManySteps
}

func (s *ChatService) finishTurn(ctx context.Context, userID uuid.UUID, content string, used int, response *model.ChatResponse) (*model.ChatResponse, error) {
	userTokens := estimateChatTokens(content)
	assistantTokens := estimateChatTokens(response.Reply)

	if err := s.history.AppendMessages(ctx, userID, []model.NewChatMessage{
		{Role: "user", Content: content, TokenCount: userTokens},
		{Role: "assistant", Content: response.Reply, TokenCount: assistantTokens},
	}); err != nil {
		return nil, fmt.Errorf("save chat turn: %w", err)
	}

	usage := chatTokenUsage(used + userTokens + assistantTokens)
	response.TokenUsage = &usage

	return response, nil
}

func (s *ChatService) History(ctx context.Context, userID uuid.UUID) (*model.ChatHistoryResponse, error) {
	messages, err := s.history.ListMessages(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("read chat history: %w", err)
	}
	if messages == nil {
		messages = []model.ChatHistoryMessage{}
	}

	used, err := s.history.TokenTotal(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("read chat token total: %w", err)
	}

	return &model.ChatHistoryResponse{
		Messages:   messages,
		TokenUsage: chatTokenUsage(used),
	}, nil
}

func (s *ChatService) Clear(ctx context.Context, userID uuid.UUID) error {
	if err := s.history.Clear(ctx, userID); err != nil {
		return fmt.Errorf("clear chat history: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for id, action := range s.pending {
		if action.userID == userID {
			delete(s.pending, id)
		}
	}
	for id, file := range s.files {
		if file.userID == userID {
			delete(s.files, id)
		}
	}

	return nil
}

func (s *ChatService) Confirm(ctx context.Context, userID uuid.UUID, id string) (*model.ChatResponse, error) {
	action, ok := s.takePending(userID, id)
	if !ok {
		return nil, ErrPendingActionNotFound
	}

	var err error
	switch action.kind {
	case pendingDeleteTransaction:
		err = s.transactions.DeleteTransaction(ctx, userID, action.targetID)
	case pendingDeleteAccount:
		err = s.accounts.DeleteAccount(ctx, userID, action.targetID)
	case pendingDeleteCategory:
		err = s.categories.DeleteCategory(ctx, userID, action.targetID)
	case pendingImportFile:
		file, ok := s.lookupFile(userID, action.fileID)
		if !ok {
			return nil, ErrPendingActionNotFound
		}
		selected, ok := file.pick(action.sheet)
		if !ok {
			return nil, ErrPendingActionNotFound
		}
		_, err = s.transactions.ImportTransactions(ctx, userID, strings.NewReader(selected.csv), false)
	default:
		return nil, ErrPendingActionNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("confirm action: %w", err)
	}

	used, err := s.history.TokenTotal(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("read chat token total: %w", err)
	}

	usage := chatTokenUsage(used)
	return &model.ChatResponse{
		Reply:      action.summary,
		ToolCalls:  []model.ChatToolCall{},
		TokenUsage: &usage,
	}, nil
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

func (s *ChatService) executeTool(ctx context.Context, userID uuid.UUID, fileID, sheet string, call llm.ToolCall) (string, string, *model.ChatPendingAction, error) {
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
	case "adjust_account_balance":
		return s.adjustAccountBalance(ctx, userID, call.Function.Arguments)
	case "delete_account":
		return s.requestDeleteAccount(ctx, userID, call.Function.Arguments)
	case "delete_category":
		return s.requestDeleteCategory(ctx, userID, call.Function.Arguments)
	case "import_attached_file":
		return s.requestImportAttachedFile(ctx, userID, call.Function.Arguments, fileID, sheet)
	case "set_budget":
		return s.setBudget(ctx, userID, call.Function.Arguments)
	case "list_budgets":
		return s.listBudgets(ctx, userID, call.Function.Arguments)
	case "create_recurring":
		return s.createRecurring(ctx, userID, call.Function.Arguments)
	case "list_recurring":
		return s.listRecurring(ctx, userID, call.Function.Arguments)
	case "pause_recurring":
		return s.pauseRecurring(ctx, userID, call.Function.Arguments)
	case "get_cashflow_report":
		return s.cashflowReport(ctx, userID, call.Function.Arguments)
	case "get_category_report":
		return s.categoryReport(ctx, userID, call.Function.Arguments)
	default:
		return "", "", nil, fmt.Errorf("unknown tool %q", call.Function.Name)
	}
}
