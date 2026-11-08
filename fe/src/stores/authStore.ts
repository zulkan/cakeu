import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { AuthState, LoginResponse } from '../types';
import { apiUrl } from '../lib/apiBase';

interface AuthStore extends AuthState {
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string) => Promise<void>;
  forgotPassword: (email: string) => Promise<string>;
  resetPassword: (token: string, password: string) => Promise<string>;
  changePassword: (
    currentPassword: string,
    newPassword: string
  ) => Promise<string>;
  logout: () => void;
  clearError: () => void;
}

interface ErrorBody {
  error?: string;
}

interface MessageBody {
  message?: string;
}

async function readError(res: Response, fallback: string): Promise<string> {
  const body = (await res.json().catch(() => ({}))) as ErrorBody;
  return body.error || fallback;
}

async function postJson(path: string, body: unknown): Promise<Response> {
  try {
    return await fetch(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  } catch {
    throw new Error('Unable to reach the server. Please try again.');
  }
}

export const useAuthStore = create<AuthStore>()(
  persist(
    (set, get) => ({
      user: null,
      token: null,
      isAuthenticated: false,
      isLoading: false,
      error: null,

      login: async (email, password) => {
        set({ isLoading: true, error: null });

        try {
          const res = await postJson(apiUrl('/auth/login'), { email, password });

          if (!res.ok) {
            const message =
              res.status === 401
                ? 'Invalid email or password'
                : await readError(res, 'Unable to sign in. Please try again.');
            set({
              user: null,
              token: null,
              isAuthenticated: false,
              isLoading: false,
              error: message,
            });
            return;
          }

          const data = (await res.json()) as LoginResponse;
          set({
            user: data.user,
            token: data.token,
            isAuthenticated: true,
            isLoading: false,
            error: null,
          });
        } catch {
          set({
            user: null,
            token: null,
            isAuthenticated: false,
            isLoading: false,
            error: 'Unable to reach the server. Please try again.',
          });
        }
      },

      register: async (email, password) => {
        set({ isLoading: true, error: null });

        try {
          const res = await postJson(apiUrl('/auth/register'), {
            email,
            password,
          });

          if (!res.ok) {
            const message =
              res.status === 409
                ? 'An account with this email already exists'
                : await readError(res, 'Unable to create your account.');
            set({ isLoading: false, error: message });
            return;
          }

          await get().login(email, password);
        } catch {
          set({
            isLoading: false,
            error: 'Unable to reach the server. Please try again.',
          });
        }
      },

      forgotPassword: async (email) => {
        const res = await postJson(apiUrl('/auth/forgot-password'), { email });

        if (!res.ok) {
          throw new Error(
            await readError(
              res,
              'Unable to send the reset link. Please try again.'
            )
          );
        }

        const data = (await res.json().catch(() => ({}))) as MessageBody;
        return (
          data.message || 'If that email exists, a reset link is on its way'
        );
      },

      resetPassword: async (token, password) => {
        const res = await postJson(apiUrl('/auth/reset-password'), {
          token,
          password,
        });

        if (!res.ok) {
          throw new Error(
            await readError(res, 'Unable to reset your password.')
          );
        }

        const data = (await res.json().catch(() => ({}))) as MessageBody;
        return data.message || 'Password reset successful';
      },

      changePassword: async (currentPassword, newPassword) => {
        const { token } = get();

        let res: Response;
        try {
          res = await fetch(apiUrl('/auth/change-password'), {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              ...(token ? { Authorization: `Bearer ${token}` } : {}),
            },
            body: JSON.stringify({
              current_password: currentPassword,
              new_password: newPassword,
            }),
          });
        } catch {
          throw new Error('Unable to reach the server. Please try again.');
        }

        // A wrong current password returns 401. Do not treat that as an expired
        // session: this request bypasses `lib/api.ts` so no user is logged out.
        if (!res.ok) {
          throw new Error(
            await readError(res, 'Unable to change your password.')
          );
        }

        const data = (await res.json().catch(() => ({}))) as MessageBody;
        return data.message || 'Password changed successfully';
      },

      logout: () => {
        set({
          user: null,
          token: null,
          isAuthenticated: false,
          isLoading: false,
          error: null,
        });
      },

      clearError: () => set({ error: null }),
    }),
    {
      name: 'auth-storage',
      partialize: (state) => ({
        user: state.user,
        token: state.token,
        isAuthenticated: state.isAuthenticated,
      }),
    }
  )
);
