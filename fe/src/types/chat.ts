export type ChatRole = 'user' | 'assistant';

export interface ChatToolCall {
  name: string;
  summary: string;
}

export interface ChatPendingAction {
  id: string;
  summary: string;
}

export interface ChatTokenUsage {
  used: number;
  limit: number;
  remaining: number;
  limit_reached: boolean;
}

export interface ChatResponse {
  reply: string;
  tool_calls: ChatToolCall[];
  pending_action?: ChatPendingAction;
  token_usage?: ChatTokenUsage;
}

export interface ChatMessage {
  role: ChatRole;
  content: string;
}

export interface ChatHistoryMessage {
  id: string;
  role: ChatRole;
  content: string;
  created_at: string;
}

export interface ChatHistoryResponse {
  messages: ChatHistoryMessage[];
  token_usage: ChatTokenUsage;
}

export interface ChatFile {
  id: string;
  name: string;
  sheet: string;
  sheets: string[];
  sheet_rows: Record<string, number>;
  rows: number;
  columns: string[];
  preview: string[][];
}

export interface ChatEntry extends ChatMessage {
  id: string;
  toolCalls: ChatToolCall[];
  pendingAction?: ChatPendingAction;
}
