package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/llm"
	"goskeleton/internal/model"
)

var chatTools = []llm.Tool{
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "list_accounts",
			Description: "List the user's accounts with their current balances.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "list_categories",
			Description: "List the user's transaction categories with their type (income or expense).",
			Parameters:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "list_transactions",
			Description: "List the user's transactions, newest first, optionally filtered by date range and type.",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{
					"date_from":{"type":"string","description":"Inclusive start date, YYYY-MM-DD"},
					"date_to":{"type":"string","description":"Inclusive end date, YYYY-MM-DD"},
					"type":{"type":"string","enum":["income","expense"]},
					"limit":{"type":"integer","minimum":1,"maximum":100}
				},
				"additionalProperties":false
			}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "get_dashboard_summary",
			Description: "Get total balance plus this month's income, expense, net and the most recent transactions.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "create_transaction",
			Description: "Record an income or expense transaction. Account and category are resolved by exact name.",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{
					"account":{"type":"string","description":"Exact account name"},
					"category":{"type":"string","description":"Exact category name"},
					"type":{"type":"string","enum":["income","expense"]},
					"amount":{"type":"number","exclusiveMinimum":0},
					"description":{"type":"string"},
					"date":{"type":"string","description":"YYYY-MM-DD, defaults to today"}
				},
				"required":["account","category","type","amount"],
				"additionalProperties":false
			}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "delete_transaction",
			Description: "Request deletion of one of the user's transactions. The user must confirm before it executes.",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{"id":{"type":"string","description":"Transaction id from list_transactions"}},
				"required":["id"],
				"additionalProperties":false
			}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "create_account",
			Description: "Create a new account. The opening balance becomes a locked opening entry.",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{
					"name":{"type":"string"},
					"opening_balance":{"type":"number","description":"Amount already in the account; may be negative for credit cards"},
					"currency":{"type":"string","description":"Three-letter code, defaults to USD"}
				},
				"required":["name"],
				"additionalProperties":false
			}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "create_category",
			Description: "Create a new transaction category.",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{
					"name":{"type":"string"},
					"type":{"type":"string","enum":["income","expense"]},
					"color":{"type":"string","description":"Hex colour, optional"}
				},
				"required":["name","type"],
				"additionalProperties":false
			}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "update_account",
			Description: "Rename an account or change its currency before the first entry.",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{
					"account":{"type":"string","description":"Current account name"},
					"name":{"type":"string","description":"New name"},
					"currency":{"type":"string"}
				},
				"required":["account"],
				"additionalProperties":false
			}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "adjust_account_balance",
			Description: "Correct an account to a real-world balance by recording a locked adjustment entry.",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{
					"account":{"type":"string","description":"Exact account name"},
					"target_balance":{"type":"number","description":"The balance the account should have now"},
					"description":{"type":"string","description":"Optional reason for the adjustment"}
				},
				"required":["account","target_balance"],
				"additionalProperties":false
			}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "delete_account",
			Description: "Request deletion of an account. The user must confirm before it executes.",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{"account":{"type":"string","description":"Exact account name"}},
				"required":["account"],
				"additionalProperties":false
			}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "delete_category",
			Description: "Request deletion of a category. The user must confirm before it executes.",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{
					"category":{"type":"string","description":"Exact category name"},
					"type":{"type":"string","enum":["income","expense"],"description":"Disambiguates categories sharing a name"}
				},
				"required":["category"],
				"additionalProperties":false
			}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "import_attached_file",
			Description: "Import the spreadsheet the user attached to this conversation as transactions. Requires user confirmation.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "set_budget",
			Description: "Set or replace the monthly budget for one category.",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{
					"category":{"type":"string","description":"Exact category name"},
					"type":{"type":"string","enum":["income","expense"],"description":"Disambiguates categories sharing a name"},
					"month":{"type":"string","description":"YYYY-MM, defaults to the current month"},
					"amount":{"type":"number","exclusiveMinimum":0}
				},
				"required":["category","amount"],
				"additionalProperties":false
			}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "list_budgets",
			Description: "List budgets with spent, remaining and percent for one month.",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{"month":{"type":"string","description":"YYYY-MM, defaults to the current month"}},
				"additionalProperties":false
			}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "create_recurring",
			Description: "Create a recurring transaction rule.",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{
					"account":{"type":"string","description":"Exact account name"},
					"category":{"type":"string","description":"Exact category name"},
					"type":{"type":"string","enum":["income","expense"]},
					"amount":{"type":"number","exclusiveMinimum":0},
					"description":{"type":"string"},
					"frequency":{"type":"string","enum":["daily","weekly","monthly"]},
					"interval":{"type":"integer","minimum":1,"maximum":12},
					"next_run":{"type":"string","description":"YYYY-MM-DD, defaults to today"}
				},
				"required":["account","category","type","amount","frequency"],
				"additionalProperties":false
			}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "list_recurring",
			Description: "List the user's recurring transaction rules.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "pause_recurring",
			Description: "Pause or resume a recurring rule. Pass active=true to resume.",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{
					"id":{"type":"string","description":"Rule id from list_recurring"},
					"active":{"type":"boolean","description":"true resumes, false or omitted pauses"}
				},
				"required":["id"],
				"additionalProperties":false
			}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "get_cashflow_report",
			Description: "Income, expense and net per month for a date range.",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{
					"from":{"type":"string","description":"YYYY-MM-DD"},
					"to":{"type":"string","description":"YYYY-MM-DD"}
				},
				"required":["from","to"],
				"additionalProperties":false
			}`),
		},
	},
	{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "get_category_report",
			Description: "Totals and share per category for a date range.",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{
					"from":{"type":"string","description":"YYYY-MM-DD"},
					"to":{"type":"string","description":"YYYY-MM-DD"}
				},
				"required":["from","to"],
				"additionalProperties":false
			}`),
		},
	},
}

func (s *ChatService) listAccounts(ctx context.Context, userID uuid.UUID) (string, string, *model.ChatPendingAction, error) {
	accounts, err := s.accounts.ListAccounts(ctx, userID)
	if err != nil {
		return "", "", nil, err
	}

	type row struct {
		Name     string  `json:"name"`
		Balance  float64 `json:"balance"`
		Currency string  `json:"currency"`
	}

	rows := make([]row, 0, len(accounts))
	for _, account := range accounts {
		rows = append(rows, row{Name: account.Name, Balance: account.Balance, Currency: account.Currency})
	}

	encoded, err := json.Marshal(rows)
	if err != nil {
		return "", "", nil, fmt.Errorf("encode accounts: %w", err)
	}

	return string(encoded), fmt.Sprintf("Listed %d accounts", len(rows)), nil, nil
}

func (s *ChatService) listCategories(ctx context.Context, userID uuid.UUID) (string, string, *model.ChatPendingAction, error) {
	categories, err := s.categories.ListCategories(ctx, userID)
	if err != nil {
		return "", "", nil, err
	}

	type row struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}

	rows := make([]row, 0, len(categories))
	for _, category := range categories {
		rows = append(rows, row{Name: category.Name, Type: category.Type})
	}

	encoded, err := json.Marshal(rows)
	if err != nil {
		return "", "", nil, fmt.Errorf("encode categories: %w", err)
	}

	return string(encoded), fmt.Sprintf("Listed %d categories", len(rows)), nil, nil
}

func (s *ChatService) listTransactions(ctx context.Context, userID uuid.UUID, rawArgs string) (string, string, *model.ChatPendingAction, error) {
	var args struct {
		DateFrom string `json:"date_from"`
		DateTo   string `json:"date_to"`
		Type     string `json:"type"`
		Limit    int    `json:"limit"`
	}
	if err := decodeArgs(rawArgs, &args); err != nil {
		return "", "", nil, err
	}

	filters := model.TransactionFilters{Page: 1, PageSize: args.Limit}
	if args.DateFrom != "" {
		date, err := time.Parse(chatDateLayout, args.DateFrom)
		if err != nil {
			return "", "", nil, fmt.Errorf("invalid date_from %q", args.DateFrom)
		}
		filters.DateFrom = &date
	}
	if args.DateTo != "" {
		date, err := time.Parse(chatDateLayout, args.DateTo)
		if err != nil {
			return "", "", nil, fmt.Errorf("invalid date_to %q", args.DateTo)
		}
		filters.DateTo = &date
	}
	if args.Type != "" {
		filters.Type = args.Type
	}

	result, err := s.transactions.ListTransactions(ctx, userID, filters)
	if err != nil {
		return "", "", nil, err
	}

	type row struct {
		ID          string  `json:"id"`
		Date        string  `json:"date"`
		Type        string  `json:"type"`
		Amount      float64 `json:"amount"`
		Description string  `json:"description"`
		Account     string  `json:"account"`
		Category    string  `json:"category"`
	}

	rows := make([]row, 0, len(result.Data))
	for _, transaction := range result.Data {
		if len(rows) >= maxToolResultRows {
			break
		}
		rows = append(rows, row{
			ID:          transaction.ID.String(),
			Date:        transaction.Date.Format(chatDateLayout),
			Type:        transaction.Type,
			Amount:      transaction.Amount,
			Description: transaction.Description,
			Account:     transaction.AccountName,
			Category:    transaction.CategoryName,
		})
	}

	encoded, err := json.Marshal(rows)
	if err != nil {
		return "", "", nil, fmt.Errorf("encode transactions: %w", err)
	}

	return string(encoded), fmt.Sprintf("Looked up %d transactions", len(rows)), nil, nil
}

func (s *ChatService) dashboardSummary(ctx context.Context, userID uuid.UUID) (string, string, *model.ChatPendingAction, error) {
	summary, err := s.transactions.GetDashboardSummary(ctx, userID)
	if err != nil {
		return "", "", nil, err
	}

	encoded, err := json.Marshal(summary)
	if err != nil {
		return "", "", nil, fmt.Errorf("encode summary: %w", err)
	}

	return string(encoded), "Read the dashboard summary", nil, nil
}

func (s *ChatService) createTransaction(ctx context.Context, userID uuid.UUID, rawArgs string) (string, string, *model.ChatPendingAction, error) {
	var args struct {
		Account     string  `json:"account"`
		Category    string  `json:"category"`
		Type        string  `json:"type"`
		Amount      float64 `json:"amount"`
		Description string  `json:"description"`
		Date        string  `json:"date"`
	}
	if err := decodeArgs(rawArgs, &args); err != nil {
		return "", "", nil, err
	}

	accountID, err := s.resolveAccount(ctx, userID, args.Account)
	if err != nil {
		return "", "", nil, err
	}

	categoryID, err := s.resolveCategory(ctx, userID, args.Category, args.Type)
	if err != nil {
		return "", "", nil, err
	}

	date := time.Now().UTC().Truncate(24 * time.Hour)
	if args.Date != "" {
		parsed, err := time.Parse(chatDateLayout, args.Date)
		if err != nil {
			return "", "", nil, fmt.Errorf("invalid date %q", args.Date)
		}
		date = parsed
	}

	created, err := s.transactions.CreateTransaction(ctx, userID, model.CreateTransactionRequest{
		AccountID:   accountID,
		CategoryID:  categoryID,
		Type:        args.Type,
		Amount:      args.Amount,
		Description: args.Description,
		Date:        date,
	})
	if err != nil {
		return "", "", nil, err
	}

	encoded, err := json.Marshal(map[string]any{
		"id":          created.ID.String(),
		"date":        created.Date.Format(chatDateLayout),
		"type":        created.Type,
		"amount":      created.Amount,
		"description": created.Description,
		"account":     created.AccountName,
		"category":    created.CategoryName,
	})
	if err != nil {
		return "", "", nil, fmt.Errorf("encode transaction: %w", err)
	}

	return string(encoded), fmt.Sprintf("Recorded %s %.2f", created.Type, created.Amount), nil, nil
}

func (s *ChatService) requestDeleteTransaction(ctx context.Context, userID uuid.UUID, rawArgs string) (string, string, *model.ChatPendingAction, error) {
	var args struct {
		ID string `json:"id"`
	}
	if err := decodeArgs(rawArgs, &args); err != nil {
		return "", "", nil, err
	}

	transactionID, err := uuid.Parse(strings.TrimSpace(args.ID))
	if err != nil {
		return "", "", nil, fmt.Errorf("invalid transaction id")
	}

	result, err := s.transactions.ListTransactions(ctx, userID, model.TransactionFilters{Page: 1, PageSize: MaxPageSize})
	if err != nil {
		return "", "", nil, err
	}

	var target *model.Transaction
	for i := range result.Data {
		if result.Data[i].ID == transactionID {
			target = &result.Data[i]
			break
		}
	}
	if target == nil {
		return "", "", nil, fmt.Errorf("transaction not found")
	}

	label := strings.TrimSpace(target.Description)
	if label == "" {
		label = target.CategoryName
	}

	summary := fmt.Sprintf("Delete %q %.2f on %s?", label, target.Amount, target.Date.Format(chatDateLayout))
	pending := s.storePending(userID, pendingDeleteTransaction, transactionID, summary)

	return "", summary, &pending, nil
}

func (s *ChatService) resolveAccount(ctx context.Context, userID uuid.UUID, name string) (uuid.UUID, error) {
	account, err := s.findAccount(ctx, userID, name)
	if err != nil {
		return uuid.Nil, err
	}

	return account.ID, nil
}

func (s *ChatService) resolveCategory(ctx context.Context, userID uuid.UUID, name, type_ string) (uuid.UUID, error) {
	category, err := s.findCategory(ctx, userID, name, type_)
	if err != nil {
		return uuid.Nil, err
	}

	return category.ID, nil
}

func decodeArgs(raw string, target any) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		return fmt.Errorf("invalid tool arguments: %w", err)
	}
	return nil
}
