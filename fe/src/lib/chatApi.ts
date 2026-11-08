import { apiUrl } from './apiBase';
import { getToken } from './api';
import { CHAT_TIMEOUT_MS, abortableScope } from './http';
import type { ChatFile, ChatMessage, ChatResponse } from '../types/chat';

interface ErrorBody {
  error?: string;
}

function chatHeaders(json: boolean): Record<string, string> {
  const headers: Record<string, string> = {};
  if (json) headers['Content-Type'] = 'application/json';
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  return headers;
}

async function readError(response: Response): Promise<Error> {
  const payload = (await response.json().catch(() => ({}))) as ErrorBody;
  return new Error(payload.error || `Request failed: ${response.status}`);
}

async function post<T>(
  path: string,
  body?: unknown,
  signal?: AbortSignal
): Promise<T> {
  const scope = abortableScope(CHAT_TIMEOUT_MS, signal);

  try {
    const response = await fetch(apiUrl(path), {
      method: 'POST',
      headers: chatHeaders(true),
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: scope.signal,
    });

    if (!response.ok) {
      throw await readError(response);
    }

    return (await response.json()) as T;
  } finally {
    scope.dispose();
  }
}

async function postMultipart<T>(
  path: string,
  body: FormData,
  signal?: AbortSignal
): Promise<T> {
  const scope = abortableScope(CHAT_TIMEOUT_MS, signal);

  // No Content-Type on purpose: the browser appends the multipart boundary.
  try {
    const response = await fetch(apiUrl(path), {
      method: 'POST',
      headers: chatHeaders(false),
      body,
      signal: scope.signal,
    });

    if (!response.ok) {
      throw await readError(response);
    }

    return (await response.json()) as T;
  } finally {
    scope.dispose();
  }
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
