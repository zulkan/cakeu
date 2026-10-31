package service_test

import (
	"context"
	"encoding/json"
	"errors"
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

func newChatService(userID uuid.UUID, client llm.Client, transactions *fakeTransactionRepo) *service.ChatService {
	accounts := service.NewAccountService(&fakeAccountRepo{accounts: []model.Account{
		{ID: uuid.New(), UserID: userID, Name: "Cash", Balance: 100, Currency: "USD"},
	}})
	categories := service.NewCategoryService(&fakeCategoryRepo{categories: []model.Category{
		{ID: uuid.New(), UserID: userID, Name: "Food", Type: "expense"},
	}})
	transactionsService := service.NewTransactionService(transactions, &fakeAccountRepo{}, &fakeCategoryRepo{})

	return service.NewChatService(accounts, categories, transactionsService, client, nil)
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
		Messages: []model.ChatMessage{{Role: "user", Content: "Which accounts do I have?"}},
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
	chat := newChatService(userID, client, repo)

	response, err := chat.Handle(context.Background(), userID, model.ChatRequest{
		Messages: []model.ChatMessage{{Role: "user", Content: "Delete my coffee expense"}},
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

	if _, err := chat.Confirm(context.Background(), userID, response.PendingAction.ID); err != nil {
		t.Fatalf("expected confirm to succeed, got %v", err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != transactionID {
		t.Fatalf("expected transaction %s deleted once, got %+v", transactionID, repo.deleted)
	}

	if _, err := chat.Confirm(context.Background(), userID, response.PendingAction.ID); !errors.Is(err, service.ErrPendingActionNotFound) {
		t.Fatalf("expected pending action to be single-use, got %v", err)
	}

	otherResponse, err := chat.Handle(context.Background(), userID, model.ChatRequest{
		Messages: []model.ChatMessage{{Role: "user", Content: "Delete it again"}},
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
				Function: llm.FunctionCall{Name: "create_account", Arguments: `{"name":"Savings","balance":250,"currency":"USD"}`},
			}},
		},
		{Role: "assistant", Content: "Created the Savings account."},
	}}
	chat := service.NewChatService(accounts, categories, transactions, client, nil)

	response, err := chat.Handle(context.Background(), userID, model.ChatRequest{
		Messages: []model.ChatMessage{{Role: "user", Content: "Create a savings account with 250 USD"}},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if accountRepo.lastCreate.name != "Savings" || accountRepo.lastCreate.balance != 250 || accountRepo.lastCreate.currency != "USD" {
		t.Fatalf("unexpected created account: %+v", accountRepo.lastCreate)
	}
	if accountRepo.lastCreate.userID != userID {
		t.Fatalf("expected account scoped to caller %s, got %s", userID, accountRepo.lastCreate.userID)
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Summary != `Created account "Savings"` {
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
	chat := service.NewChatService(accounts, categories, transactions, client, nil)

	response, err := chat.Handle(context.Background(), userID, model.ChatRequest{
		Messages: []model.ChatMessage{{Role: "user", Content: "Delete my Cash account"}},
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
	chat := service.NewChatService(accounts, categories, transactions, client, nil)

	response, err := chat.Handle(context.Background(), userID, model.ChatRequest{
		Messages: []model.ChatMessage{{Role: "user", Content: "Delete the Food category"}},
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

func TestChatServiceReportsNotConfigured(t *testing.T) {
	t.Parallel()

	client := &fakeLLMClient{err: llm.ErrNotConfigured}
	chat := newChatService(uuid.New(), client, &fakeTransactionRepo{})

	_, err := chat.Handle(context.Background(), uuid.New(), model.ChatRequest{
		Messages: []model.ChatMessage{{Role: "user", Content: "hello"}},
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
		Messages: []model.ChatMessage{{Role: "user", Content: "loop forever"}},
	})
	if !errors.Is(err, service.ErrChatTooManySteps) {
		t.Fatalf("expected ErrChatTooManySteps, got %v", err)
	}
	if client.calls != 5 {
		t.Fatalf("expected the loop to stop at 5 llm calls, got %d", client.calls)
	}
}
