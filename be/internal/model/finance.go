package model

import (
	"time"

	"github.com/google/uuid"
)

type Account struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	Name           string    `json:"name"`
	Balance        float64   `json:"balance"`
	OpeningBalance float64   `json:"opening_balance"`
	EntryCount     int       `json:"entry_count"`
	Currency       string    `json:"currency"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Category struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

type Transaction struct {
	ID              uuid.UUID `json:"id"`
	UserID          uuid.UUID `json:"user_id"`
	AccountID       uuid.UUID `json:"account_id"`
	CategoryID      uuid.UUID `json:"category_id"`
	TransferGroupID uuid.UUID `json:"transfer_group_id"`
	Type            string    `json:"type"`
	Amount          float64   `json:"amount"`
	Description     string    `json:"description"`
	Date            time.Time `json:"date"`
	CreatedAt       time.Time `json:"created_at"`
	AccountName     string    `json:"account_name"`
	CategoryName    string    `json:"category_name"`
	Kind            string    `json:"kind"`
	Currency        string    `json:"currency"`
}

type TransactionExportRow struct {
	Date         time.Time
	Type         string
	Amount       float64
	Description  string
	AccountName  string
	CategoryName string
}

type ImportRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

type ImportPreviewRow struct {
	Row         int     `json:"row"`
	Date        string  `json:"date"`
	Type        string  `json:"type"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Account     string  `json:"account"`
	Category    string  `json:"category"`
	Duplicate   bool    `json:"duplicate,omitempty"`
}

// TransactionFingerprint is the ledger side of deterministic import dedupe.
// The service hashes the same fields from a parsed import row and looks for a
// match in this set.
type TransactionFingerprint struct {
	Date        time.Time
	Type        string
	Amount      float64
	Description string
	AccountName string
}

type ImportFieldRule struct {
	Column       int               `json:"column,omitempty"`
	Constant     string            `json:"constant,omitempty"`
	CarryForward bool              `json:"carry_forward,omitempty"`
	DateFormat   string            `json:"date_format,omitempty"`
	AmountFormat string            `json:"amount_format,omitempty"`
	AmountMode   string            `json:"amount_mode,omitempty"`
	ValueMap     map[string]string `json:"value_map,omitempty"`
}

type ImportTypeRule struct {
	Column   int               `json:"column,omitempty"`
	Constant string            `json:"constant,omitempty"`
	ValueMap map[string]string `json:"value_map,omitempty"`
}

type ImportFields struct {
	Date        ImportFieldRule `json:"date"`
	Amount      ImportFieldRule `json:"amount"`
	Description ImportFieldRule `json:"description"`
	Account     ImportFieldRule `json:"account"`
	Category    ImportFieldRule `json:"category"`
}

type ImportMapping struct {
	Sheet      string         `json:"sheet"`
	HeaderRow  int            `json:"header_row"`
	DataStart  int            `json:"data_start"`
	DataEnd    int            `json:"data_end"`
	SkipRows   []int          `json:"skip_rows,omitempty"`
	Fields     ImportFields   `json:"fields"`
	Type       ImportTypeRule `json:"type"`
	Confidence float64        `json:"confidence,omitempty"`
	Notes      string         `json:"notes,omitempty"`
}

type ImportResult struct {
	Total          int                `json:"total"`
	Valid          int                `json:"valid"`
	Invalid        int                `json:"invalid"`
	Skipped        int                `json:"skipped"`
	Imported       int                `json:"imported"`
	DuplicateCount int                `json:"duplicate_count"`
	Sheet          string             `json:"sheet"`
	Sheets         []string           `json:"sheets"`
	Mode           string             `json:"mode,omitempty"`
	Mapping        *ImportMapping     `json:"mapping,omitempty"`
	Errors         []ImportRowError   `json:"errors"`
	Duplicates     []ImportRowError   `json:"duplicates"`
	Preview        []ImportPreviewRow `json:"preview"`
}

type CreateAccountRequest struct {
	Name           string  `json:"name" example:"Checking Account"`
	OpeningBalance float64 `json:"opening_balance" example:"1000.00"`
	Currency       string  `json:"currency" example:"USD"`
}

type UpdateAccountRequest struct {
	Name     *string `json:"name,omitempty" example:"Savings Account"`
	Currency *string `json:"currency,omitempty" example:"EUR"`
	// Balance is kept only to detect and reject callers that still try to
	// overwrite the derived balance. Corrections use the adjustment endpoint.
	Balance *float64 `json:"balance,omitempty" example:"2500.00"`
}

// AdjustAccountRequest corrects an account to a real-world balance. The server
// records the difference as a locked adjustment entry.
type AdjustAccountRequest struct {
	TargetBalance float64 `json:"target_balance" example:"2500.00"`
	Description   string  `json:"description,omitempty" example:"Matched bank statement"`
}

type CreateCategoryRequest struct {
	Name  string `json:"name" example:"Groceries"`
	Type  string `json:"type" example:"expense"`
	Color string `json:"color" example:"#FB923C"`
}

// UpdateCategoryRequest is a partial rename/recolor. Type is decoded only so
// the handler can reject a category type change with 400; it is never applied.
type UpdateCategoryRequest struct {
	Name  *string `json:"name,omitempty" example:"Groceries"`
	Color *string `json:"color,omitempty" example:"#FB923C"`
	Type  *string `json:"type,omitempty" example:"expense"`
}

type CreateTransactionRequest struct {
	AccountID   uuid.UUID `json:"account_id" example:"00000000-0000-0000-0000-000000000000"`
	CategoryID  uuid.UUID `json:"category_id" example:"00000000-0000-0000-0000-000000000000"`
	Type        string    `json:"type" example:"expense"`
	Amount      float64   `json:"amount" example:"42.50"`
	Description string    `json:"description" example:"Weekly groceries"`
	Date        time.Time `json:"date" example:"2026-10-25T00:00:00Z"`
}

// CreateTransferRequest moves money between two same-currency accounts of the
// caller. Transfers are not created through the generic transaction endpoint.
type CreateTransferRequest struct {
	FromAccountID uuid.UUID `json:"from_account_id" example:"00000000-0000-0000-0000-000000000000"`
	ToAccountID   uuid.UUID `json:"to_account_id" example:"00000000-0000-0000-0000-000000000000"`
	Amount        float64   `json:"amount" example:"100.00"`
	Date          string    `json:"date" example:"2026-11-08"`
	Description   string    `json:"description,omitempty" example:"Move to savings"`
}

// TransferResult is the pair of ledger legs created by one transfer.
type TransferResult struct {
	TransferGroupID uuid.UUID   `json:"transfer_group_id"`
	From            Transaction `json:"from"`
	To              Transaction `json:"to"`
}

// UpdateTransactionRequest carries a partial transaction update. Omitted
// fields keep their current value; supplied fields are validated against the
// transaction's user, account and category before anything is written.
type UpdateTransactionRequest struct {
	AccountID   *uuid.UUID `json:"account_id,omitempty" example:"00000000-0000-0000-0000-000000000000"`
	CategoryID  *uuid.UUID `json:"category_id,omitempty" example:"00000000-0000-0000-0000-000000000000"`
	Type        *string    `json:"type,omitempty" example:"expense"`
	Amount      *float64   `json:"amount,omitempty" example:"42.50"`
	Description *string    `json:"description,omitempty" example:"Weekly groceries"`
	Date        *time.Time `json:"date,omitempty" example:"2026-10-25T00:00:00Z"`
}

// TransactionFilters narrows a transaction listing. UserID is populated by the
// service layer from the authenticated user and is never read from the client.
type TransactionFilters struct {
	UserID     *uuid.UUID `json:"-"`
	AccountID  *uuid.UUID `json:"account_id,omitempty"`
	CategoryID *uuid.UUID `json:"category_id,omitempty"`
	Type       string     `json:"type,omitempty"`
	Kind       string     `json:"kind,omitempty"`
	DateFrom   *time.Time `json:"date_from,omitempty"`
	DateTo     *time.Time `json:"date_to,omitempty"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
}

// CurrencyTotal is one currency's balance. Currencies are never added together.
type CurrencyTotal struct {
	Currency string  `json:"currency"`
	Total    float64 `json:"total"`
}

// MonthlyCurrencyTotal is one currency's income and expense for a month.
type MonthlyCurrencyTotal struct {
	Currency string  `json:"currency"`
	Income   float64 `json:"income"`
	Expense  float64 `json:"expense"`
	Net      float64 `json:"net"`
}

type DashboardSummary struct {
	Balances           []CurrencyTotal        `json:"balances"`
	Monthly            []MonthlyCurrencyTotal `json:"monthly"`
	TransactionCount   int                    `json:"transaction_count"`
	RecentTransactions []Transaction          `json:"recent_transactions"`
}

type PaginatedTransactions struct {
	Data     []Transaction `json:"data"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}
