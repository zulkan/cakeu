package model

import (
	"time"

	"github.com/google/uuid"
)

type ChatRequest struct {
	Content string `json:"content"`
	FileID  string `json:"file_id,omitempty"`
	Sheet   string `json:"sheet,omitempty"`
}

type ChatFile struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Sheet     string         `json:"sheet"`
	Sheets    []string       `json:"sheets"`
	SheetRows map[string]int `json:"sheet_rows"`
	Rows      int            `json:"rows"`
	Columns   []string       `json:"columns"`
	Preview   [][]string     `json:"preview"`
}

type ChatToolCall struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

type ChatPendingAction struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
}

type ChatTokenUsage struct {
	Used         int  `json:"used"`
	Limit        int  `json:"limit"`
	Remaining    int  `json:"remaining"`
	LimitReached bool `json:"limit_reached"`
}

type ChatHistoryMessage struct {
	ID        uuid.UUID `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type NewChatMessage struct {
	Role       string
	Content    string
	TokenCount int
}

type ChatHistoryResponse struct {
	Messages   []ChatHistoryMessage `json:"messages"`
	TokenUsage ChatTokenUsage       `json:"token_usage"`
}

type ChatResponse struct {
	Reply         string             `json:"reply"`
	ToolCalls     []ChatToolCall     `json:"tool_calls"`
	PendingAction *ChatPendingAction `json:"pending_action,omitempty"`
	TokenUsage    *ChatTokenUsage    `json:"token_usage,omitempty"`
}
