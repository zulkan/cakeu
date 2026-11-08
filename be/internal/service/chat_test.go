package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/llm"
	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

type fakeLLMClient struct {
	responses []llm.Message
	err       error
	calls     int
}

func (f *fakeLLMClient) Complete(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Message, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if len(f.responses) == 0 {
		return &llm.Message{Role: "assistant", Content: "done"}, nil
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return &response, nil
}

type loopingLLMClient struct {
	calls int
}

func (l *loopingLLMClient) Complete(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Message, error) {
	l.calls++
	return &llm.Message{
		Role: "assistant",
		ToolCalls: []llm.ToolCall{{
			ID:       "call_loop",
			Type:     "function",
			Function: llm.FunctionCall{Name: "list_accounts", Arguments: "{}"},
		}},
	}, nil
}

type fakeChatHistoryRepo struct {
	messages []model.ChatHistoryMessage
	total    int
	appended []model.NewChatMessage
	cleared  bool
	err      error
}

func (f *fakeChatHistoryRepo) ListMessages(ctx context.Context, userID uuid.UUID) ([]model.ChatHistoryMessage, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.messages, nil
}

func (f *fakeChatHistoryRepo) AppendMessages(ctx context.Context, userID uuid.UUID, messages []model.NewChatMessage) error {
	if f.err != nil {
		return f.err
	}
	f.appended = append(f.appended, messages...)
	for _, message := range messages {
		f.total += message.TokenCount
	}
	return nil
}

func (f *fakeChatHistoryRepo) Clear(ctx context.Context, userID uuid.UUID) error {
	if f.err != nil {
		return f.err
	}
	f.messages = nil
	f.total = 0
	f.cleared = true
	return nil
}

func (f *fakeChatHistoryRepo) TokenTotal(ctx context.Context, userID uuid.UUID) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.total, nil
}

func newChatService(userID uuid.UUID, client llm.Client, transactions *fakeTransactionRepo) *service.ChatService {
	return newChatServiceWithHistory(userID, client, transactions, &fakeChatHistoryRepo{})
}

func newChatServiceWithHistory(userID uuid.UUID, client llm.Client, transactions *fakeTransactionRepo, history *fakeChatHistoryRepo) *service.ChatService {
	accounts := service.NewAccountService(&fakeAccountRepo{accounts: []model.Account{
		{ID: uuid.New(), UserID: userID, Name: "Cash", Balance: 100, Currency: "USD"},
	}})
	categories := service.NewCategoryService(&fakeCategoryRepo{categories: []model.Category{
		{ID: uuid.New(), UserID: userID, Name: "Food", Type: "expense"},
	}})
	transactionsService := service.NewTransactionService(transactions, &fakeAccountRepo{}, &fakeCategoryRepo{})

	return service.NewChatService(accounts, categories, transactionsService, nil, nil, nil, history, client, nil)
}

func TestChatServiceRunsToolThenReplies(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	client := &fakeLLMClient{responses: []llm.Message{
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{{
				ID:       "call_1",
				Type:     "function",
				Function: llm.FunctionCall{Name: "list_accounts", Arguments: "{}"},
			}},
		},
		{Role: "assistant", Content: "You have one account: Cash."},
	}}
	chat := newChatService(userID, client, &fakeTransactionRepo{})

	response, err := chat.Handle(context.Background(), userID, model.ChatRequest{
		Content: "Which accounts do I have?",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if response.Reply != "You have one account: Cash." {
		t.Fatalf("unexpected reply %q", response.Reply)
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "list_accounts" {
		t.Fatalf("unexpected tool calls %+v", response.ToolCalls)
	}
	if response.ToolCalls[0].Summary != "Listed 1 accounts" {
		t.Fatalf("unexpected tool summary %q", response.ToolCalls[0].Summary)
	}
	if client.calls != 2 {
		t.Fatalf("expected 2 llm calls, got %d", client.calls)
	}
	if response.TokenUsage == nil || response.TokenUsage.Used <= 0 {
		t.Fatalf("expected token usage on the response, got %+v", response.TokenUsage)
	}
}

func TestChatServiceDeleteRequiresConfirmation(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	transactionID := uuid.New()
	repo := &fakeTransactionRepo{
		list: []model.Transaction{{
			ID:           transactionID,
			UserID:       userID,
			Type:         "expense",
			Amount:       12.5,
			Description:  "Coffee",
			Date:         time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			AccountName:  "Cash",
			CategoryName: "Food",
		}},
		total: 1,
	}

	args, err := json.Marshal(map[string]string{"id": transactionID.String()})
	if err != nil {
		t.Fatalf("encode args: %v", err)
	}

	deleteCall := llm.Message{
		Role: "assistant",
		ToolCalls: []llm.ToolCall{{
			ID:       "call_delete",
			Type:     "function",
			Function: llm.FunctionCall{Name: "delete_transaction", Arguments: string(args)},
		}},
	}
	client := &fakeLLMClient{responses: []llm.Message{deleteCall, deleteCall}}
	history := &fakeChatHistoryRepo{}
	chat := newChatServiceWithHistory(userID, client, repo, history)

	response, err := chat.Handle(context.Background(), userID, model.ChatRequest{
		Content: "Delete my coffee expense",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if response.PendingAction == nil {
		t.Fatalf("expected a pending action, got %+v", response)
	}
	if len(repo.deleted) != 0 {
		t.Fatalf("transaction must not be deleted before confirmation")
	}
	if len(history.appended) != 2 {
		t.Fatalf("expected the assistant summary to be persisted once, got %d messages", len(history.appended))
	}

	confirmed, err := chat.Confirm(context.Background(), userID, response.PendingAction.ID)
	if err != nil {
		t.Fatalf("expected confirm to succeed, got %v", err)
	}
	if confirmed.Reply != response.Reply {
		t.Fatalf("expected confirm summary %q, got %q", response.Reply, confirmed.Reply)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != transactionID {
		t.Fatalf("expected transaction %s deleted once, got %+v", transactionID, repo.deleted)
	}

	if _, err := chat.Confirm(context.Background(), userID, response.PendingAction.ID); !errors.Is(err, service.ErrPendingActionNotFound) {
		t.Fatalf("expected pending action to be single-use, got %v", err)
	}

	otherResponse, err := chat.Handle(context.Background(), userID, model.ChatRequest{
		Content: "Delete it again",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if otherResponse.PendingAction == nil {
		t.Fatalf("expected a second pending action")
	}

	if _, err := chat.Confirm(context.Background(), uuid.New(), otherResponse.PendingAction.ID); !errors.Is(err, service.ErrPendingActionNotFound) {
		t.Fatalf("another user must not confirm the action, got %v", err)
	}
}

func TestChatServiceCreatesAccount(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountRepo := &fakeAccountRepo{}
	accounts := service.NewAccountService(accountRepo)
	categories := service.NewCategoryService(&fakeCategoryRepo{})
	transactions := service.NewTransactionService(&fakeTransactionRepo{}, &fakeAccountRepo{}, &fakeCategoryRepo{})

	client := &fakeLLMClient{responses: []llm.Message{
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{{
				ID:       "call_create_account",
				Type:     "function",
				Function: llm.FunctionCall{Name: "create_account", Arguments: `{"name":"Savings","opening_balance":250,"currency":"USD"}`},
			}},
		},
		{Role: "assistant", Content: "Created the Savings account."},
	}}
	chat := service.NewChatService(accounts, categories, transactions, nil, nil, nil, &fakeChatHistoryRepo{}, client, nil)

	response, err := chat.Handle(context.Background(), userID, model.ChatRequest{
		Content: "Create a savings account with 250 USD",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if accountRepo.lastCreate.name != "Savings" || accountRepo.lastCreate.openingBalance != 250 || accountRepo.lastCreate.currency != "USD" {
		t.Fatalf("unexpected created account: %+v", accountRepo.lastCreate)
	}
	if accountRepo.lastCreate.userID != userID {
		t.Fatalf("expected account scoped to caller %s, got %s", userID, accountRepo.lastCreate.userID)
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Summary != `Created account "Savings"` {
		t.Fatalf("unexpected tool calls: %+v", response.ToolCalls)
	}
}

func TestChatServiceAdjustsAccountBalance(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	accountRepo := &fakeAccountRepo{accounts: []model.Account{
		{ID: accountID, UserID: userID, Name: "Cash", Balance: 100, Currency: "USD"},
	}}
	accounts := service.NewAccountService(accountRepo)
	categories := service.NewCategoryService(&fakeCategoryRepo{})
	transactions := service.NewTransactionService(&fakeTransactionRepo{}, &fakeAccountRepo{}, &fakeCategoryRepo{})

	client := &fakeLLMClient{responses: []llm.Message{
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{{
				ID:       "call_adjust_account",
				Type:     "function",
				Function: llm.FunctionCall{Name: "adjust_account_balance", Arguments: `{"account":"Cash","target_balance":250}`},
			}},
		},
		{Role: "assistant", Content: "Adjusted the Cash account."},
	}}
	chat := service.NewChatService(accounts, categories, transactions, nil, nil, nil, &fakeChatHistoryRepo{}, client, nil)

	response, err := chat.Handle(context.Background(), userID, model.ChatRequest{
		Content: "Set Cash to 250",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if accountRepo.lastAdjust.TargetBalance != 250 {
		t.Fatalf("expected target 250, got %v", accountRepo.lastAdjust.TargetBalance)
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "adjust_account_balance" {
		t.Fatalf("unexpected tool calls: %+v", response.ToolCalls)
	}
}

func TestChatServiceAccountDeleteRequiresConfirmation(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	accountRepo := &fakeAccountRepo{accounts: []model.Account{
		{ID: accountID, UserID: userID, Name: "Cash", Balance: 100, Currency: "USD"},
	}}
	accounts := service.NewAccountService(accountRepo)
	categories := service.NewCategoryService(&fakeCategoryRepo{})
	transactions := service.NewTransactionService(&fakeTransactionRepo{}, &fakeAccountRepo{}, &fakeCategoryRepo{})

	client := &fakeLLMClient{responses: []llm.Message{{
		Role: "assistant",
		ToolCalls: []llm.ToolCall{{
			ID:       "call_delete_account",
			Type:     "function",
			Function: llm.FunctionCall{Name: "delete_account", Arguments: `{"account":"Cash"}`},
		}},
	}}}
	chat := service.NewChatService(accounts, categories, transactions, nil, nil, nil, &fakeChatHistoryRepo{}, client, nil)

	response, err := chat.Handle(context.Background(), userID, model.ChatRequest{
		Content: "Delete my Cash account",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if response.PendingAction == nil {
		t.Fatalf("expected a pending action, got %+v", response)
	}
	if len(accountRepo.deleted) != 0 {
		t.Fatalf("account must not be deleted before confirmation")
	}

	if _, err := chat.Confirm(context.Background(), userID, response.PendingAction.ID); err != nil {
		t.Fatalf("expected confirm to succeed, got %v", err)
	}
	if len(accountRepo.deleted) != 1 || accountRepo.deleted[0] != accountID {
		t.Fatalf("expected account %s deleted once, got %+v", accountID, accountRepo.deleted)
	}
}

func TestChatServiceCategoryDeleteRequiresConfirmation(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	categoryID := uuid.New()
	categoryRepo := &fakeCategoryRepo{categories: []model.Category{
		{ID: categoryID, UserID: userID, Name: "Food", Type: "expense"},
	}}
	accounts := service.NewAccountService(&fakeAccountRepo{})
	categories := service.NewCategoryService(categoryRepo)
	transactions := service.NewTransactionService(&fakeTransactionRepo{}, &fakeAccountRepo{}, &fakeCategoryRepo{})

	client := &fakeLLMClient{responses: []llm.Message{{
		Role: "assistant",
		ToolCalls: []llm.ToolCall{{
			ID:       "call_delete_category",
			Type:     "function",
			Function: llm.FunctionCall{Name: "delete_category", Arguments: `{"category":"Food","type":"expense"}`},
		}},
	}}}
	chat := service.NewChatService(accounts, categories, transactions, nil, nil, nil, &fakeChatHistoryRepo{}, client, nil)

	response, err := chat.Handle(context.Background(), userID, model.ChatRequest{
		Content: "Delete the Food category",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if response.PendingAction == nil {
		t.Fatalf("expected a pending action, got %+v", response)
	}
	if len(categoryRepo.deleted) != 0 {
		t.Fatalf("category must not be deleted before confirmation")
	}

	if _, err := chat.Confirm(context.Background(), userID, response.PendingAction.ID); err != nil {
		t.Fatalf("expected confirm to succeed, got %v", err)
	}
	if len(categoryRepo.deleted) != 1 || categoryRepo.deleted[0] != categoryID {
		t.Fatalf("expected category %s deleted once, got %+v", categoryID, categoryRepo.deleted)
	}
}

func TestChatServiceAttachesFileAndImports(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	categoryID := uuid.New()
	accountRepo := &fakeAccountRepo{accounts: []model.Account{{ID: accountID, UserID: userID, Name: "Cash"}}}
	categoryRepo := &fakeCategoryRepo{categories: []model.Category{{ID: categoryID, UserID: userID, Name: "Food", Type: "expense"}}}
	repo := &fakeTransactionRepo{}

	accounts := service.NewAccountService(accountRepo)
	categories := service.NewCategoryService(categoryRepo)
	transactions := service.NewTransactionService(repo, accountRepo, categoryRepo)

	client := &fakeLLMClient{responses: []llm.Message{{
		Role: "assistant",
		ToolCalls: []llm.ToolCall{{
			ID:       "call_import",
			Type:     "function",
			Function: llm.FunctionCall{Name: "import_attached_file", Arguments: "{}"},
		}},
	}}}
	chat := service.NewChatService(accounts, categories, transactions, nil, nil, nil, &fakeChatHistoryRepo{}, client, nil)

	csvBody := "date,type,amount,description,account,category\n2026-10-01,expense,12.50,Coffee,Cash,Food\n"
	file, err := chat.AttachFile(context.Background(), userID, "rows.csv", strings.NewReader(csvBody))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if file.Rows != 1 || len(file.Preview) != 2 {
		t.Fatalf("unexpected file metadata: %+v", file)
	}

	response, err := chat.Handle(context.Background(), userID, model.ChatRequest{
		Content: "Import this file",
		FileID:  file.ID,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if response.PendingAction == nil {
		t.Fatalf("expected a pending import action, got %+v", response)
	}
	if len(repo.lastBatch) != 0 {
		t.Fatalf("rows must not be imported before confirmation")
	}

	if _, err := chat.Confirm(context.Background(), userID, response.PendingAction.ID); err != nil {
		t.Fatalf("expected confirm to succeed, got %v", err)
	}
	if len(repo.lastBatch) != 1 {
		t.Fatalf("expected 1 imported transaction, got %d", len(repo.lastBatch))
	}
}

func TestChatServiceReportsNotConfigured(t *testing.T) {
	t.Parallel()

	client := &fakeLLMClient{err: llm.ErrNotConfigured}
	chat := newChatService(uuid.New(), client, &fakeTransactionRepo{})

	_, err := chat.Handle(context.Background(), uuid.New(), model.ChatRequest{
		Content: "hello",
	})
	if !errors.Is(err, service.ErrLLMNotConfigured) {
		t.Fatalf("expected ErrLLMNotConfigured, got %v", err)
	}
}

func TestChatServiceStopsAfterIterationLimit(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	client := &loopingLLMClient{}
	chat := newChatService(userID, client, &fakeTransactionRepo{})

	_, err := chat.Handle(context.Background(), userID, model.ChatRequest{
		Content: "loop forever",
	})
	if !errors.Is(err, service.ErrChatTooManySteps) {
		t.Fatalf("expected ErrChatTooManySteps, got %v", err)
	}
	if client.calls != 5 {
		t.Fatalf("expected the loop to stop at 5 llm calls, got %d", client.calls)
	}
}

type fakeBudgetService struct {
	statuses []model.BudgetStatus
}

func (f *fakeBudgetService) List(ctx context.Context, userID uuid.UUID, month time.Time) ([]model.BudgetStatus, error) {
	return f.statuses, nil
}

func (f *fakeBudgetService) Upsert(ctx context.Context, userID, categoryID uuid.UUID, month time.Time, currency string, amount float64) (*model.Budget, error) {
	return &model.Budget{CategoryID: categoryID, Month: month, Currency: currency, Amount: amount}, nil
}

func (f *fakeBudgetService) Delete(ctx context.Context, userID, categoryID uuid.UUID, month time.Time, currency string) error {
	return nil
}

func TestChatServiceListsBudgets(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	client := &fakeLLMClient{responses: []llm.Message{
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{{
				ID:       "call_budgets",
				Type:     "function",
				Function: llm.FunctionCall{Name: "list_budgets", Arguments: "{}"},
			}},
		},
		{Role: "assistant", Content: "You are halfway through your Food budget."},
	}}
	budgets := &fakeBudgetService{statuses: []model.BudgetStatus{{CategoryName: "Food", Budget: 100, Spent: 50, Percent: 50}}}

	chat := service.NewChatService(nil, nil, nil, budgets, nil, nil, &fakeChatHistoryRepo{}, client, nil)

	response, err := chat.Handle(context.Background(), userID, model.ChatRequest{
		Content: "How are my budgets?",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "list_budgets" {
		t.Fatalf("unexpected tool calls: %+v", response.ToolCalls)
	}
	if response.Reply != "You are halfway through your Food budget." {
		t.Fatalf("unexpected reply %q", response.Reply)
	}
}

func TestChatServiceRequiresContent(t *testing.T) {
	t.Parallel()

	history := &fakeChatHistoryRepo{}
	client := &fakeLLMClient{}
	chat := newChatServiceWithHistory(uuid.New(), client, &fakeTransactionRepo{}, history)

	_, err := chat.Handle(context.Background(), uuid.New(), model.ChatRequest{Content: "   "})
	if !errors.Is(err, service.ErrChatMessageRequired) {
		t.Fatalf("expected ErrChatMessageRequired, got %v", err)
	}
	if client.calls != 0 {
		t.Fatalf("expected no llm call, got %d", client.calls)
	}
}

func TestChatServiceBlocksWhenTokenLimitReached(t *testing.T) {
	t.Parallel()

	history := &fakeChatHistoryRepo{total: 50_000}
	client := &fakeLLMClient{}
	chat := newChatServiceWithHistory(uuid.New(), client, &fakeTransactionRepo{}, history)

	_, err := chat.Handle(context.Background(), uuid.New(), model.ChatRequest{Content: "hello"})
	if !errors.Is(err, service.ErrChatTokenLimitReached) {
		t.Fatalf("expected ErrChatTokenLimitReached, got %v", err)
	}
	if client.calls != 0 {
		t.Fatalf("expected no llm call, got %d", client.calls)
	}
	if len(history.appended) != 0 {
		t.Fatalf("expected nothing persisted, got %d messages", len(history.appended))
	}
}

func TestChatServiceAllowsTurnThatOvershootsLimit(t *testing.T) {
	t.Parallel()

	history := &fakeChatHistoryRepo{total: 49_999}
	client := &fakeLLMClient{responses: []llm.Message{
		{Role: "assistant", Content: strings.Repeat("a", 40)},
	}}
	chat := newChatServiceWithHistory(uuid.New(), client, &fakeTransactionRepo{}, history)

	response, err := chat.Handle(context.Background(), uuid.New(), model.ChatRequest{Content: "hello"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if response.TokenUsage == nil {
		t.Fatalf("expected token usage, got %+v", response.TokenUsage)
	}
	if response.TokenUsage.Used != 50_011 {
		t.Fatalf("expected used 50011, got %d", response.TokenUsage.Used)
	}
	if response.TokenUsage.Remaining != 0 {
		t.Fatalf("expected remaining to clamp at 0, got %d", response.TokenUsage.Remaining)
	}
	if !response.TokenUsage.LimitReached {
		t.Fatalf("expected the overshooting turn to report limit_reached")
	}
	if len(history.appended) != 2 {
		t.Fatalf("expected the user/assistant pair persisted, got %d messages", len(history.appended))
	}
	if history.appended[0].Role != "user" || history.appended[1].Role != "assistant" {
		t.Fatalf("unexpected persisted roles: %+v", history.appended)
	}
}

func TestChatServiceHistoryReturnsMessagesAndUsage(t *testing.T) {
	t.Parallel()

	messageID := uuid.New()
	createdAt := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	history := &fakeChatHistoryRepo{
		messages: []model.ChatHistoryMessage{{
			ID:        messageID,
			Role:      "user",
			Content:   "hi",
			CreatedAt: createdAt,
		}},
		total: 1_234,
	}
	chat := newChatServiceWithHistory(uuid.New(), &fakeLLMClient{}, &fakeTransactionRepo{}, history)

	result, err := chat.History(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Messages) != 1 || result.Messages[0].ID != messageID {
		t.Fatalf("unexpected messages: %+v", result.Messages)
	}
	if result.TokenUsage.Used != 1_234 || result.TokenUsage.Limit != 50_000 {
		t.Fatalf("unexpected token usage: %+v", result.TokenUsage)
	}
	if result.TokenUsage.Remaining != 48_766 || result.TokenUsage.LimitReached {
		t.Fatalf("unexpected token usage: %+v", result.TokenUsage)
	}
}

func TestChatServiceHistoryReturnsEmptyMessages(t *testing.T) {
	t.Parallel()

	chat := newChatServiceWithHistory(uuid.New(), &fakeLLMClient{}, &fakeTransactionRepo{}, &fakeChatHistoryRepo{})

	result, err := chat.History(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Messages == nil {
		t.Fatalf("expected an empty slice, got nil")
	}
	if len(result.Messages) != 0 {
		t.Fatalf("expected no messages, got %d", len(result.Messages))
	}
}

func TestChatServiceClearRemovesHistoryPendingAndFiles(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	transactionID := uuid.New()
	repo := &fakeTransactionRepo{
		list: []model.Transaction{{
			ID:          transactionID,
			UserID:      userID,
			Type:        "expense",
			Amount:      5,
			Description: "Tea",
			Date:        time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		}},
		total: 1,
	}

	args, err := json.Marshal(map[string]string{"id": transactionID.String()})
	if err != nil {
		t.Fatalf("encode args: %v", err)
	}
	client := &fakeLLMClient{responses: []llm.Message{{
		Role: "assistant",
		ToolCalls: []llm.ToolCall{{
			ID:       "call_delete",
			Type:     "function",
			Function: llm.FunctionCall{Name: "delete_transaction", Arguments: string(args)},
		}},
	}}}
	history := &fakeChatHistoryRepo{}
	chat := newChatServiceWithHistory(userID, client, repo, history)

	response, err := chat.Handle(context.Background(), userID, model.ChatRequest{Content: "delete the tea expense"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if response.PendingAction == nil {
		t.Fatalf("expected a pending action")
	}
	pendingID := response.PendingAction.ID

	file, err := chat.AttachFile(context.Background(), userID, "rows.csv", strings.NewReader(
		"date,type,amount,description,account,category\n2026-10-01,expense,1.00,Tea,Cash,Food\n",
	))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := chat.Clear(context.Background(), userID); err != nil {
		t.Fatalf("expected clear to succeed, got %v", err)
	}
	if !history.cleared || len(history.messages) != 0 || history.total != 0 {
		t.Fatalf("expected history cleared, got %+v", history)
	}

	if _, err := chat.Confirm(context.Background(), userID, pendingID); !errors.Is(err, service.ErrPendingActionNotFound) {
		t.Fatalf("expected pending action dropped, got %v", err)
	}

	_, err = chat.Handle(context.Background(), userID, model.ChatRequest{Content: "retry", FileID: file.ID})
	if !errors.Is(err, service.ErrAttachmentNotFound) {
		t.Fatalf("expected attached file dropped, got %v", err)
	}
}

func TestChatServiceConfirmReturnsUsageWithoutPersisting(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	transactionID := uuid.New()
	repo := &fakeTransactionRepo{
		list: []model.Transaction{{
			ID:          transactionID,
			UserID:      userID,
			Type:        "expense",
			Amount:      3,
			Description: "Snack",
			Date:        time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
		}},
		total: 1,
	}

	args, err := json.Marshal(map[string]string{"id": transactionID.String()})
	if err != nil {
		t.Fatalf("encode args: %v", err)
	}
	client := &fakeLLMClient{responses: []llm.Message{{
		Role: "assistant",
		ToolCalls: []llm.ToolCall{{
			ID:       "call_delete",
			Type:     "function",
			Function: llm.FunctionCall{Name: "delete_transaction", Arguments: string(args)},
		}},
	}}}
	history := &fakeChatHistoryRepo{}
	chat := newChatServiceWithHistory(userID, client, repo, history)

	response, err := chat.Handle(context.Background(), userID, model.ChatRequest{Content: "delete the snack"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if response.PendingAction == nil {
		t.Fatalf("expected a pending action")
	}

	persistedBefore := len(history.appended)
	usedBefore := history.total

	confirmed, err := chat.Confirm(context.Background(), userID, response.PendingAction.ID)
	if err != nil {
		t.Fatalf("expected confirm to succeed, got %v", err)
	}
	if confirmed.TokenUsage == nil || confirmed.TokenUsage.Used != usedBefore {
		t.Fatalf("expected current usage %d, got %+v", usedBefore, confirmed.TokenUsage)
	}
	if len(history.appended) != persistedBefore {
		t.Fatalf("confirm must not persist messages, got %d before and %d after", persistedBefore, len(history.appended))
	}
}
