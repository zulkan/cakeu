package model

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Messages []ChatMessage `json:"messages"`
	FileID   string        `json:"file_id,omitempty"`
	Sheet    string        `json:"sheet,omitempty"`
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

type ChatResponse struct {
	Reply         string             `json:"reply"`
	ToolCalls     []ChatToolCall     `json:"tool_calls"`
	PendingAction *ChatPendingAction `json:"pending_action,omitempty"`
}
