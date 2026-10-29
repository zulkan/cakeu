package model

import (
	"time"

	"github.com/google/uuid"
)

type Account struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	Balance   float64   `json:"balance"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	AccountID    uuid.UUID `json:"account_id"`
	CategoryID   uuid.UUID `json:"category_id"`
	Type         string    `json:"type"`
	Amount       float64   `json:"amount"`
	Description  string    `json:"description"`
	Date         time.Time `json:"date"`
	CreatedAt    time.Time `json:"created_at"`
	AccountName  string    `json:"account_name"`
	CategoryName string    `json:"category_name"`
}

type CreateAccountRequest struct {
	Name     string  `json:"name" example:"Checking Account"`
	Balance  float64 `json:"balance" example:"1000.00"`
	Currency string  `json:"currency" example:"USD"`
}

type UpdateAccountRequest struct {
	Name     *string  `json:"name,omitempty" example:"Savings Account"`
	Balance  *float64 `json:"balance,omitempty" example:"2500.00"`
	Currency *string  `json:"currency,omitempty" example:"EUR"`
}

type CreateCategoryRequest struct {
	Name  string `json:"name" example:"Groceries"`
	Type  string `json:"type" example:"expense"`
	Color string `json:"color" example:"#FB923C"`
}

type CreateTransactionRequest struct {
	AccountID   uuid.UUID `json:"account_id" example:"00000000-0000-0000-0000-000000000000"`
	CategoryID  uuid.UUID `json:"category_id" example:"00000000-0000-0000-0000-000000000000"`
	Type        string    `json:"type" example:"expense"`
	Amount      float64   `json:"amount" example:"42.50"`
	Description string    `json:"description" example:"Weekly groceries"`
	Date        time.Time `json:"date" example:"2026-10-25T00:00:00Z"`
}

// TransactionFilters narrows a transaction listing. UserID is populated by the
// service layer from the authenticated user and is never read from the client.
type TransactionFilters struct {
	UserID     *uuid.UUID `json:"-"`
	AccountID  *uuid.UUID `json:"account_id,omitempty"`
	CategoryID *uuid.UUID `json:"category_id,omitempty"`
	Type       string     `json:"type,omitempty"`
	DateFrom   *time.Time `json:"date_from,omitempty"`
	DateTo     *time.Time `json:"date_to,omitempty"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
}

type DashboardSummary struct {
	TotalBalance       float64       `json:"total_balance"`
	MonthlyIncome      float64       `json:"monthly_income"`
	MonthlyExpense     float64       `json:"monthly_expense"`
	MonthlyNet         float64       `json:"monthly_net"`
	TransactionCount   int           `json:"transaction_count"`
	RecentTransactions []Transaction `json:"recent_transactions"`
}

type PaginatedTransactions struct {
	Data     []Transaction `json:"data"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}
