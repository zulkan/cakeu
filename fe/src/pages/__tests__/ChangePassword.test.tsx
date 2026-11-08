import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import ChangePassword from '../ChangePassword';
import { useAuthStore } from '../../stores/authStore';
import { useToastStore } from '../../stores/toastStore';
import { mockFetchOnce, testUser } from '../../test/helpers';

function renderChangePassword() {
  return render(
    <MemoryRouter>
      <ChangePassword />
    </MemoryRouter>
  );
}

async function fillForm(
  currentPassword: string,
  newPassword: string,
  confirmPassword: string
) {
  const user = userEvent.setup();
  await user.type(
    screen.getByLabelText('Current password'),
    currentPassword
  );
  await user.type(screen.getByLabelText('New password'), newPassword);
  await user.type(
    screen.getByLabelText('Confirm new password'),
    confirmPassword
  );
  return user;
}

describe('ChangePassword component', () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
    useAuthStore.setState({
      user: testUser,
      token: 'jwt-token',
      isAuthenticated: true,
      isLoading: false,
      error: null,
    });
    useToastStore.setState({ toasts: [] });
  });

  it('renders the password fields and back link', () => {
    renderChangePassword();

    expect(screen.getByText('Change password')).toBeInTheDocument();
    expect(screen.getByLabelText('Current password')).toBeInTheDocument();
    expect(screen.getByLabelText('New password')).toBeInTheDocument();
    expect(screen.getByLabelText('Confirm new password')).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: /back to dashboard/i })
    ).toHaveAttribute('href', '/dashboard');
  });

  it('rejects a new password shorter than 8 characters', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    renderChangePassword();

    const user = await fillForm('oldpassword', 'short', 'short');
    await user.click(screen.getByRole('button', { name: /update password/i }));

    expect(
      await screen.findByText('Password must be at least 8 characters')
    ).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('rejects a mismatched confirmation', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    renderChangePassword();

    const user = await fillForm(
      'oldpassword',
      'password123',
      'password124'
    );
    await user.click(screen.getByRole('button', { name: /update password/i }));

    expect(
      await screen.findByText('Passwords do not match')
    ).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('rejects a password longer than 72 UTF-8 bytes', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    renderChangePassword();

    const oversized = 'é'.repeat(40);
    const user = await fillForm('oldpassword', oversized, oversized);
    await user.click(screen.getByRole('button', { name: /update password/i }));

    expect(
      await screen.findByText('Password must be at most 72 bytes')
    ).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('changes the password, shows a success toast and resets the form', async () => {
    const fetchMock = mockFetchOnce({
      message: 'Password changed successfully',
    });
    renderChangePassword();

    const user = await fillForm(
      'oldpassword',
      'newpassword123',
      'newpassword123'
    );
    await user.click(screen.getByRole('button', { name: /update password/i }));

    await waitFor(() => {
      expect(useToastStore.getState().toasts).toHaveLength(1);
    });

    expect(useToastStore.getState().toasts[0]).toMatchObject({
      title: 'Password changed',
      description: 'Password changed successfully',
      tone: 'success',
    });
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/auth/change-password',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({
          Authorization: 'Bearer jwt-token',
        }),
        body: JSON.stringify({
          current_password: 'oldpassword',
          new_password: 'newpassword123',
        }),
      })
    );
    expect(screen.getByLabelText('Current password')).toHaveValue('');
    expect(screen.getByLabelText('New password')).toHaveValue('');
    expect(screen.getByLabelText('Confirm new password')).toHaveValue('');
    expect(useAuthStore.getState().isAuthenticated).toBe(true);
  });

  it('shows the server error inline without logging the user out', async () => {
    mockFetchOnce({ error: 'current password is incorrect' }, 401);
    renderChangePassword();

    const user = await fillForm(
      'wrongpassword',
      'newpassword123',
      'newpassword123'
    );
    await user.click(screen.getByRole('button', { name: /update password/i }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'current password is incorrect'
    );
    expect(useAuthStore.getState().isAuthenticated).toBe(true);
    expect(useAuthStore.getState().token).toBe('jwt-token');
  });
});
