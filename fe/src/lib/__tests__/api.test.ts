import { describe, it, expect, beforeEach, vi } from 'vitest';
import { api } from '../api';
import { useAuthStore } from '../../stores/authStore';
import { jsonResponse, mockFetchOnce, testUser } from '../../test/helpers';

function storedToken(token: string) {
  localStorage.setItem(
    'auth-storage',
    JSON.stringify({ state: { token }, version: 0 })
  );
}

describe('api client', () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
    useAuthStore.setState({
      user: null,
      token: null,
      isAuthenticated: false,
      isLoading: false,
      error: null,
    });
  });

  it('attaches the persisted JWT as a Bearer token', async () => {
    storedToken('abc123');
    const fetchMock = mockFetchOnce([]);

    await api.listAccounts();

    const options = fetchMock.mock.calls[0][1] as RequestInit;
    const headers = options.headers as Record<string, string>;
    expect(headers.Authorization).toBe('Bearer abc123');
  });

  it('omits the Authorization header when no token is stored', async () => {
    const fetchMock = mockFetchOnce([]);

    await api.listAccounts();

    const options = fetchMock.mock.calls[0][1] as RequestInit;
    const headers = options.headers as Record<string, string>;
    expect(headers.Authorization).toBeUndefined();
  });

  it('logs out and rejects when the API returns 401', async () => {
    storedToken('expired');
    useAuthStore.setState({
      user: testUser,
      token: 'expired',
      isAuthenticated: true,
    });
    mockFetchOnce({ error: 'invalid or expired token' }, 401);

    await expect(api.listAccounts()).rejects.toThrow('invalid or expired token');

    expect(useAuthStore.getState().isAuthenticated).toBe(false);
    expect(useAuthStore.getState().token).toBeNull();
  });

  it('returns undefined for a 204 response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(null, 204)));

    await expect(api.deleteAccount('account-1')).resolves.toBeUndefined();
  });

  it('surfaces the backend error message', async () => {
    mockFetchOnce({ error: 'account not found' }, 404);

    await expect(api.updateAccount('missing', { name: 'x' })).rejects.toThrow(
      'account not found'
    );
  });
});
