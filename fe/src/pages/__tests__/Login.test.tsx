import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import Login from '../Login';
import { useAuthStore } from '../../stores/authStore';
import { mockFetchOnce, testUser } from '../../test/helpers';

function renderLogin() {
  return render(
    <MemoryRouter>
      <Login />
    </MemoryRouter>
  );
}

describe('Login component', () => {
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

  it('renders the Catatku login form', () => {
    renderLogin();

    expect(screen.getAllByText('Catatku').length).toBeGreaterThan(0);
    expect(screen.getByText('Welcome back')).toBeInTheDocument();
    expect(screen.getByLabelText('Email')).toBeInTheDocument();
    expect(screen.getByLabelText('Password')).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /sign in/i })
    ).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: /create an account/i })
    ).toHaveAttribute('href', '/register');
    expect(
      screen.getByRole('link', { name: /forgot password/i })
    ).toHaveAttribute('href', '/forgot-password');
  });

  it('shows validation errors on empty submit', async () => {
    const user = userEvent.setup();
    renderLogin();

    await user.click(screen.getByRole('button', { name: /sign in/i }));

    expect(await screen.findByText('Email is required')).toBeInTheDocument();
    expect(screen.getByText('Password is required')).toBeInTheDocument();
  });

  it('shows an email format error', async () => {
    const user = userEvent.setup();
    renderLogin();

    await user.type(screen.getByLabelText('Email'), 'invalid');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    expect(await screen.findByText('Invalid email format')).toBeInTheDocument();
  });

  it('displays the auth error from the store', () => {
    useAuthStore.setState({ error: 'Invalid email or password' });
    renderLogin();

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Invalid email or password'
    );
  });

  it('logs in against the backend and stores the token', async () => {
    const user = userEvent.setup();
    const fetchMock = mockFetchOnce({
      token: 'jwt-token',
      user: testUser,
    });
    renderLogin();

    await user.type(screen.getByLabelText('Email'), 'john.doe@example.com');
    await user.type(screen.getByLabelText('Password'), 'password123');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(useAuthStore.getState().isAuthenticated).toBe(true);
    });

    expect(useAuthStore.getState().token).toBe('jwt-token');
    expect(useAuthStore.getState().user?.email).toBe('john.doe@example.com');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/auth/login',
      expect.objectContaining({ method: 'POST' })
    );
  });

  it('shows a success notice after a password reset', () => {
    render(
      <MemoryRouter
        initialEntries={[
          { pathname: '/login', state: { resetSuccess: true } },
        ]}
      >
        <Login />
      </MemoryRouter>
    );

    expect(screen.getByRole('status')).toHaveTextContent(
      /your password has been reset/i
    );
  });

  it('shows the backend error when credentials are rejected', async () => {
    const user = userEvent.setup();
    mockFetchOnce({ error: 'invalid credentials' }, 401);
    renderLogin();

    await user.type(screen.getByLabelText('Email'), 'john.doe@example.com');
    await user.type(screen.getByLabelText('Password'), 'wrongpass');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Invalid email or password'
    );
    expect(useAuthStore.getState().isAuthenticated).toBe(false);
  });
});
