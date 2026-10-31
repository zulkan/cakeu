package model

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Messages []ChatMessage `json:"messages"`
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
