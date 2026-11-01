package model

import "github.com/google/uuid"

type CashflowPoint struct {
	Period  string  `json:"period"`
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
	Net     float64 `json:"net"`
}

type CategoryTotal struct {
	CategoryID   uuid.UUID `json:"category_id"`
	CategoryName string    `json:"category_name"`
	Type         string    `json:"type"`
	Total        float64   `json:"total"`
	Share        float64   `json:"share"`
}

type MerchantTotal struct {
	Description string  `json:"description"`
	Total       float64 `json:"total"`
}
