import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import ForgotPassword from '../ForgotPassword';
import { useAuthStore } from '../../stores/authStore';
import { mockFetchOnce } from '../../test/helpers';

function renderForgotPassword() {
  return render(
    <MemoryRouter>
      <ForgotPassword />
    </MemoryRouter>
  );
}

describe('ForgotPassword component', () => {
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

  it('renders the email form and a link back to login', () => {
    renderForgotPassword();

    expect(screen.getByText('Forgot your password?')).toBeInTheDocument();
    expect(screen.getByLabelText('Email')).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /send reset link/i })
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /sign in/i })).toHaveAttribute(
      'href',
      '/login'
    );
  });

  it('shows validation errors on empty submit', async () => {
    const user = userEvent.setup();
    renderForgotPassword();

    await user.click(screen.getByRole('button', { name: /send reset link/i }));

    expect(await screen.findByText('Email is required')).toBeInTheDocument();
  });

  it('shows an email format error', async () => {
    const user = userEvent.setup();
    renderForgotPassword();

    await user.type(screen.getByLabelText('Email'), 'invalid');
    await user.click(screen.getByRole('button', { name: /send reset link/i }));

    expect(await screen.findByText('Invalid email format')).toBeInTheDocument();
  });

  it('shows the neutral success panel for any email', async () => {
    const user = userEvent.setup();
    const fetchMock = mockFetchOnce({
      message: 'if the email exists, a reset link will be sent',
    });
    renderForgotPassword();

    await user.type(screen.getByLabelText('Email'), 'unknown@example.com');
    await user.click(screen.getByRole('button', { name: /send reset link/i }));

    expect(
      await screen.findByText(
        /if that email exists, a reset link is on its way/i
      )
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /back to sign in/i })).toHaveAttribute(
      'href',
      '/login'
    );
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/auth/forgot-password',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ email: 'unknown@example.com' }),
      })
    );
  });

  it('shows the server error when the request fails', async () => {
    const user = userEvent.setup();
    mockFetchOnce({ error: 'too many requests' }, 429);
    renderForgotPassword();

    await user.type(screen.getByLabelText('Email'), 'john.doe@example.com');
    await user.click(screen.getByRole('button', { name: /send reset link/i }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'too many requests'
    );
  });
});
