package model

import "github.com/google/uuid"

type CashflowPoint struct {
	Currency string  `json:"currency"`
	Period   string  `json:"period"`
	Income   float64 `json:"income"`
	Expense  float64 `json:"expense"`
	Net      float64 `json:"net"`
}

type CategoryTotal struct {
	Currency     string    `json:"currency"`
	CategoryID   uuid.UUID `json:"category_id"`
	CategoryName string    `json:"category_name"`
	Type         string    `json:"type"`
	Total        float64   `json:"total"`
	Share        float64   `json:"share"`
}

type MerchantTotal struct {
	Currency    string  `json:"currency"`
	Description string  `json:"description"`
	Total       float64 `json:"total"`
}
