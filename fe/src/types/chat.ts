export type ChatRole = 'user' | 'assistant';

export interface ChatToolCall {
  name: string;
  summary: string;
}

export interface ChatPendingAction {
  id: string;
  summary: string;
}

export interface ChatResponse {
  reply: string;
  tool_calls: ChatToolCall[];
  pending_action?: ChatPendingAction;
}

export interface ChatMessage {
  role: ChatRole;
  content: string;
}

export interface ChatEntry extends ChatMessage {
  id: string;
  toolCalls: ChatToolCall[];
  pendingAction?: ChatPendingAction;
}
