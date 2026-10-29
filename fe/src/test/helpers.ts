import { vi } from 'vitest';
import type { User } from '../types';

export const testUser: User = {
  id: 'user-1',
  email: 'john.doe@example.com',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

export function jsonResponse(body: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: status === 204 ? 'No Content' : '',
    json: async () => body,
  } as unknown as Response;
}

export function mockFetchOnce(body: unknown, status = 200) {
  const fetchMock = vi.fn().mockResolvedValueOnce(jsonResponse(body, status));
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

export function mockFetchSequence(
  ...responses: Array<{ body: unknown; status?: number }>
) {
  const fetchMock = vi.fn();

  for (const response of responses) {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(response.body, response.status ?? 200)
    );
  }

  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

export function mockApiFetch(
  handlers: Record<string, { body: unknown; status?: number }>
) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : input.toString();
    const match = Object.keys(handlers).find((path) => url.includes(path));

    if (!match) {
      return jsonResponse({ error: 'not found' }, 404);
    }

    const handler = handlers[match];
    return jsonResponse(handler.body, handler.status ?? 200);
  });

  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}
