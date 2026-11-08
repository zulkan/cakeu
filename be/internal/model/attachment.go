package model

import (
	"time"

	"github.com/google/uuid"
)

// Attachment is a receipt or supporting file attached to a transaction. The
// bytes live on disk under StorageKey; this row is the source of truth.
type Attachment struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"-"`
	TransactionID uuid.UUID `json:"transaction_id"`
	Filename      string    `json:"filename"`
	ContentType   string    `json:"content_type"`
	SizeBytes     int64     `json:"size_bytes"`
	StorageKey    string    `json:"-"`
	CreatedAt     time.Time `json:"created_at"`
}
