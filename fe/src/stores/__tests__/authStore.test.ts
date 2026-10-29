import { describe, it, expect, beforeEach, vi } from 'vitest';
import { useAuthStore } from '../authStore';
import {
  jsonResponse,
  mockFetchOnce,
  mockFetchSequence,
  testUser,
} from '../../test/helpers';

const loginResponse = { token: 'jwt-token', user: testUser };

describe('auth store', () => {
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

  it('authenticates with valid credentials against the real login endpoint', async () => {
    const fetchMock = mockFetchOnce(loginResponse);

    await useAuthStore.getState().login('john.doe@example.com', 'password123');

    const state = useAuthStore.getState();
    expect(state.isAuthenticated).toBe(true);
    expect(state.token).toBe('jwt-token');
    expect(state.user?.email).toBe('john.doe@example.com');
    expect(state.error).toBeNull();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/auth/login',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          email: 'john.doe@example.com',
          password: 'password123',
        }),
      })
    );
  });

  it('rejects invalid credentials with a friendly error', async () => {
    mockFetchOnce({ error: 'invalid credentials' }, 401);

    await useAuthStore.getState().login('wrong@example.com', 'wrongpass');

    const state = useAuthStore.getState();
    expect(state.isAuthenticated).toBe(false);
    expect(state.token).toBeNull();
    expect(state.error).toBe('Invalid email or password');
  });

  it('reports a network failure without crashing', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));

    await useAuthStore.getState().login('john.doe@example.com', 'password123');

    expect(useAuthStore.getState().error).toBe(
      'Unable to reach the server. Please try again.'
    );
  });

  it('registers then logs in automatically', async () => {
    const fetchMock = mockFetchSequence(
      { body: testUser, status: 201 },
      { body: loginResponse }
    );

    await useAuthStore.getState().register('john.doe@example.com', 'password123');

    const state = useAuthStore.getState();
    expect(state.isAuthenticated).toBe(true);
    expect(state.token).toBe('jwt-token');
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls[0][0]).toBe('/api/auth/register');
    expect(fetchMock.mock.calls[1][0]).toBe('/api/auth/login');
  });

  it('surfaces a duplicate email error on register', async () => {
    mockFetchOnce({ error: 'email already registered' }, 409);

    await useAuthStore.getState().register('john.doe@example.com', 'password123');

    const state = useAuthStore.getState();
    expect(state.isAuthenticated).toBe(false);
    expect(state.error).toBe('An account with this email already exists');
  });

  it('clears auth state on logout', async () => {
    mockFetchOnce(loginResponse);
    await useAuthStore.getState().login('john.doe@example.com', 'password123');

    useAuthStore.getState().logout();

    const state = useAuthStore.getState();
    expect(state.isAuthenticated).toBe(false);
    expect(state.user).toBeNull();
    expect(state.token).toBeNull();
  });

  it('sets loading state while the login request is in flight', async () => {
    let resolveFetch: (value: Response) => void = () => undefined;
    const pending = new Promise<Response>((resolve) => {
      resolveFetch = resolve;
    });
    vi.stubGlobal('fetch', vi.fn().mockReturnValue(pending));

    const request = useAuthStore
      .getState()
      .login('john.doe@example.com', 'password123');

    expect(useAuthStore.getState().isLoading).toBe(true);

    resolveFetch(jsonResponse(loginResponse));
    await request;

    expect(useAuthStore.getState().isLoading).toBe(false);
  });

  it('persists auth state to localStorage and rehydrates on re-init', async () => {
    mockFetchOnce(loginResponse);
    await useAuthStore.getState().login('john.doe@example.com', 'password123');

    const persistedRaw = localStorage.getItem('auth-storage');
    expect(persistedRaw).not.toBeNull();
    const persisted = JSON.parse(persistedRaw as string);
    expect(persisted.state.user.email).toBe('john.doe@example.com');
    expect(persisted.state.token).toBe('jwt-token');
    expect(persisted.state.isAuthenticated).toBe(true);

    const savedBlob = localStorage.getItem('auth-storage');
    expect(savedBlob).not.toBeNull();

    useAuthStore.setState({
      user: null,
      token: null,
      isAuthenticated: false,
      isLoading: false,
      error: null,
    });
    localStorage.setItem('auth-storage', savedBlob as string);

    expect(useAuthStore.getState().isAuthenticated).toBe(false);

    await useAuthStore.persist.rehydrate();

    const rehydrated = useAuthStore.getState();
    expect(rehydrated.isAuthenticated).toBe(true);
    expect(rehydrated.user?.email).toBe('john.doe@example.com');
    expect(rehydrated.token).toBe('jwt-token');
  });

  it('requests a password reset link and returns the server message', async () => {
    const fetchMock = mockFetchOnce({
      message: 'if the email exists, a reset link will be sent',
    });

    const message = await useAuthStore
      .getState()
      .forgotPassword('john.doe@example.com');

    expect(message).toBe('if the email exists, a reset link will be sent');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/auth/forgot-password',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ email: 'john.doe@example.com' }),
      })
    );
  });

  it('throws the server error when the reset request fails', async () => {
    mockFetchOnce({ error: 'too many requests' }, 429);

    await expect(
      useAuthStore.getState().forgotPassword('john.doe@example.com')
    ).rejects.toThrow('too many requests');
  });

  it('resets the password and returns the server message', async () => {
    const fetchMock = mockFetchOnce({ message: 'password reset successful' });

    const message = await useAuthStore
      .getState()
      .resetPassword('token-1', 'newpassword');

    expect(message).toBe('password reset successful');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/auth/reset-password',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ token: 'token-1', password: 'newpassword' }),
      })
    );
  });

  it('throws the server error for an invalid reset token', async () => {
    mockFetchOnce({ error: 'invalid or expired token' }, 400);

    await expect(
      useAuthStore.getState().resetPassword('bad-token', 'newpassword')
    ).rejects.toThrow('invalid or expired token');
  });
});
