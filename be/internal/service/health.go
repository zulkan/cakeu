package service

import "context"

type Pinger interface {
	Ping(context.Context) error
}

type Health struct {
	db Pinger
}

type Status struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

func NewHealth(db Pinger) *Health {
	return &Health{db: db}
}

func (h *Health) Check(ctx context.Context) Status {
	if h.db == nil {
		return Status{Status: "ok", Database: "up"}
	}

	if err := h.db.Ping(ctx); err != nil {
		return Status{Status: "degraded", Database: "down"}
	}

	return Status{Status: "ok", Database: "up"}
}
