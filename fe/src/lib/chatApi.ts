import { apiUrl } from './apiBase';
import type { ChatFile, ChatMessage, ChatResponse } from '../types/chat';

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

async function postMultipart<T>(path: string, body: FormData): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {};
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  // No Content-Type on purpose: the browser appends the multipart boundary.
  const response = await fetch(apiUrl(path), {
    method: 'POST',
    headers,
    body,
  });

  if (!response.ok) {
    const payload = (await response
      .json()
      .catch(() => ({}))) as ErrorBody;
    throw new Error(payload.error || `Request failed: ${response.status}`);
  }

  return (await response.json()) as T;
}

interface ChatRequestBody {
  messages: ChatMessage[];
  file_id?: string;
  sheet?: string;
}

function chatBody(
  messages: ChatMessage[],
  fileId?: string,
  sheet?: string
): ChatRequestBody {
  const body: ChatRequestBody = { messages };
  if (fileId) body.file_id = fileId;
  if (sheet) body.sheet = sheet;
  return body;
}

export const chatApi = {
  send: (messages: ChatMessage[], fileId?: string, sheet?: string) =>
    post<ChatResponse>('/chat', chatBody(messages, fileId, sheet)),
  uploadFile: (file: File) => {
    const form = new FormData();
    form.append('file', file);
    return postMultipart<ChatFile>('/chat/files', form);
  },
  confirm: (id: string) =>
    post<ChatResponse>(`/chat/actions/${encodeURIComponent(id)}/confirm`),
  cancel: (id: string) =>
    post<ChatResponse>(`/chat/actions/${encodeURIComponent(id)}/cancel`),
};
