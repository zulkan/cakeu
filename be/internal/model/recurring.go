package model

import (
	"time"

	"github.com/google/uuid"
)

type RecurringRule struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	AccountID   uuid.UUID `json:"account_id"`
	CategoryID  uuid.UUID `json:"category_id"`
	Type        string    `json:"type"`
	Amount      float64   `json:"amount"`
	Description string    `json:"description"`
	Frequency   string    `json:"frequency"`
	Interval    int       `json:"interval"`
	NextRun     time.Time `json:"next_run"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateRecurringRequest struct {
	AccountID   uuid.UUID `json:"account_id"`
	CategoryID  uuid.UUID `json:"category_id"`
	Type        string    `json:"type"`
	Amount      float64   `json:"amount"`
	Description string    `json:"description"`
	Frequency   string    `json:"frequency"`
	Interval    int       `json:"interval"`
	NextRun     string    `json:"next_run"`
}

type UpdateRecurringRequest struct {
	Amount      *float64 `json:"amount,omitempty"`
	Description *string  `json:"description,omitempty"`
	Frequency   *string  `json:"frequency,omitempty"`
	Interval    *int     `json:"interval,omitempty"`
	NextRun     *string  `json:"next_run,omitempty"`
	Active      *bool    `json:"active,omitempty"`
}
