package model

import (
	"time"

	"github.com/google/uuid"
)

type Budget struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	CategoryID   uuid.UUID `json:"category_id"`
	CategoryName string    `json:"category_name"`
	Month        time.Time `json:"month"`
	Amount       float64   `json:"amount"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type BudgetStatus struct {
	CategoryID   uuid.UUID `json:"category_id"`
	CategoryName string    `json:"category_name"`
	Budget       float64   `json:"budget"`
	Spent        float64   `json:"spent"`
	Remaining    float64   `json:"remaining"`
	Percent      float64   `json:"percent"`
}

type UpsertBudgetRequest struct {
	CategoryID uuid.UUID `json:"category_id"`
	Month      string    `json:"month"`
	Amount     float64   `json:"amount"`
}
