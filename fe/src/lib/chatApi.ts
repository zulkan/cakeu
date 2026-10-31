import { apiUrl } from './apiBase';
import type { ChatMessage, ChatResponse } from '../types/chat';

interface ErrorBody {
  error?: string;
}

function getToken(): string | null {
  try {
    const stored = localStorage.getItem('auth-storage');
    if (!stored) return null;
    const parsed = JSON.parse(stored) as {
      state?: { token?: string | null };
    };
    return parsed?.state?.token ?? null;
  } catch {
    return null;
  }
}

async function post<T>(path: string, body?: unknown): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  const response = await fetch(apiUrl(path), {
    method: 'POST',
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  if (!response.ok) {
    const payload = (await response
      .json()
      .catch(() => ({}))) as ErrorBody;
    throw new Error(payload.error || `Request failed: ${response.status}`);
  }

  return (await response.json()) as T;
}

export const chatApi = {
  send: (messages: ChatMessage[]) => post<ChatResponse>('/chat', { messages }),
  confirm: (id: string) =>
    post<ChatResponse>(`/chat/actions/${encodeURIComponent(id)}/confirm`),
  cancel: (id: string) =>
    post<ChatResponse>(`/chat/actions/${encodeURIComponent(id)}/cancel`),
};
