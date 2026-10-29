import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import ResetPassword from '../ResetPassword';
import { useAuthStore } from '../../stores/authStore';
import { mockFetchOnce } from '../../test/helpers';

function renderResetPassword(entry = '/reset-password?token=token-1') {
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <Routes>
        <Route path="/reset-password" element={<ResetPassword />} />
        <Route path="/login" element={<div>Login page</div>} />
        <Route
          path="/forgot-password"
          element={<div>Forgot password page</div>}
        />
      </Routes>
    </MemoryRouter>
  );
}

describe('ResetPassword component', () => {
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

  it('shows an error panel when the token is missing', () => {
    renderResetPassword('/reset-password');

    expect(screen.getByRole('alert')).toHaveTextContent(
      /missing its token/i
    );
    expect(
      screen.getByRole('link', { name: /request a new link/i })
    ).toHaveAttribute('href', '/forgot-password');
    expect(screen.queryByLabelText('New password')).not.toBeInTheDocument();
  });

  it('rejects a password shorter than 8 characters', async () => {
    const user = userEvent.setup();
    renderResetPassword();

    await user.type(screen.getByLabelText('New password'), 'short');
    await user.type(screen.getByLabelText('Confirm new password'), 'short');
    await user.click(screen.getByRole('button', { name: /reset password/i }));

    expect(
      await screen.findByText('Password must be at least 8 characters')
    ).toBeInTheDocument();
  });

  it('rejects mismatched passwords', async () => {
    const user = userEvent.setup();
    renderResetPassword();

    await user.type(screen.getByLabelText('New password'), 'password123');
    await user.type(
      screen.getByLabelText('Confirm new password'),
      'password124'
    );
    await user.click(screen.getByRole('button', { name: /reset password/i }));

    expect(
      await screen.findByText('Passwords do not match')
    ).toBeInTheDocument();
  });

  it('resets the password and navigates to login', async () => {
    const user = userEvent.setup();
    const fetchMock = mockFetchOnce({ message: 'password reset successful' });
    renderResetPassword();

    await user.type(screen.getByLabelText('New password'), 'password123');
    await user.type(
      screen.getByLabelText('Confirm new password'),
      'password123'
    );
    await user.click(screen.getByRole('button', { name: /reset password/i }));

    expect(await screen.findByText('Login page')).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/auth/reset-password',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ token: 'token-1', password: 'password123' }),
      })
    );
  });

  it('shows an error panel for an invalid or expired token', async () => {
    const user = userEvent.setup();
    mockFetchOnce({ error: 'invalid or expired token' }, 400);
    renderResetPassword();

    await user.type(screen.getByLabelText('New password'), 'password123');
    await user.type(
      screen.getByLabelText('Confirm new password'),
      'password123'
    );
    await user.click(screen.getByRole('button', { name: /reset password/i }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'invalid or expired token'
    );
    expect(
      screen.getByRole('link', { name: /request a new link/i })
    ).toHaveAttribute('href', '/forgot-password');
  });
});
