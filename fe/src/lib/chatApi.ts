import { apiUrl } from './apiBase';
import { getToken } from './api';
import { CHAT_TIMEOUT_MS, abortableScope } from './http';
import type { ChatFile, ChatHistoryResponse, ChatResponse } from '../types/chat';

interface ErrorBody {
  error?: string;
}

export class ChatApiError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = 'ChatApiError';
    this.status = status;
  }
}

type HttpMethod = 'GET' | 'POST' | 'DELETE';

function chatHeaders(json: boolean): Record<string, string> {
  const headers: Record<string, string> = {};
  if (json) headers['Content-Type'] = 'application/json';
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  return headers;
}

async function readError(response: Response): Promise<ChatApiError> {
  const payload = (await response.json().catch(() => ({}))) as ErrorBody;
  return new ChatApiError(
    payload.error || `Request failed: ${response.status}`,
    response.status
  );
}

async function request<T>(
  method: HttpMethod,
  path: string,
  body?: unknown,
  signal?: AbortSignal
): Promise<T> {
  const scope = abortableScope(CHAT_TIMEOUT_MS, signal);
  const hasBody = body !== undefined;

  try {
    const response = await fetch(apiUrl(path), {
      method,
      headers: chatHeaders(hasBody),
      body: hasBody ? JSON.stringify(body) : undefined,
      signal: scope.signal,
    });

    if (!response.ok) {
      throw await readError(response);
    }

    if (response.status === 204) {
      return undefined as T;
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
  content: string;
  file_id?: string;
  sheet?: string;
}

function chatBody(
  content: string,
  fileId?: string,
  sheet?: string
): ChatRequestBody {
  const body: ChatRequestBody = { content };
  if (fileId) body.file_id = fileId;
  if (sheet) body.sheet = sheet;
  return body;
}

export const chatApi = {
  send: (content: string, fileId?: string, sheet?: string) =>
    request<ChatResponse>('POST', '/chat', chatBody(content, fileId, sheet)),
  history: () => request<ChatHistoryResponse>('GET', '/chat/history'),
  clear: () => request<{ message: string }>('DELETE', '/chat/history'),
  uploadFile: (file: File) => {
    const form = new FormData();
    form.append('file', file);
    return postMultipart<ChatFile>('/chat/files', form);
  },
  confirm: (id: string) =>
    request<ChatResponse>(
      'POST',
      `/chat/actions/${encodeURIComponent(id)}/confirm`
    ),
  cancel: (id: string) =>
    request<ChatResponse>(
      'POST',
      `/chat/actions/${encodeURIComponent(id)}/cancel`
    ),
};
