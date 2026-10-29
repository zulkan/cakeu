import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import Register from '../Register';
import { useAuthStore } from '../../stores/authStore';
import {
  mockFetchOnce,
  mockFetchSequence,
  testUser,
} from '../../test/helpers';

function renderRegister() {
  return render(
    <MemoryRouter>
      <Register />
    </MemoryRouter>
  );
}

describe('Register component', () => {
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

  it('renders the registration form and a link back to login', () => {
    renderRegister();

    expect(screen.getByText('Create your account')).toBeInTheDocument();
    expect(screen.getByLabelText('Email')).toBeInTheDocument();
    expect(screen.getByLabelText('Password')).toBeInTheDocument();
    expect(screen.getByLabelText('Confirm password')).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: /sign in/i })
    ).toHaveAttribute('href', '/login');
  });

  it('validates required fields and password length', async () => {
    const user = userEvent.setup();
    renderRegister();

    await user.click(screen.getByRole('button', { name: /create account/i }));

    expect(await screen.findByText('Email is required')).toBeInTheDocument();
    expect(screen.getByText('Password is required')).toBeInTheDocument();
    expect(
      screen.getByText('Please confirm your password')
    ).toBeInTheDocument();
  });

  it('rejects mismatched passwords', async () => {
    const user = userEvent.setup();
    renderRegister();

    await user.type(screen.getByLabelText('Email'), 'john.doe@example.com');
    await user.type(screen.getByLabelText('Password'), 'password123');
    await user.type(screen.getByLabelText('Confirm password'), 'password124');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    expect(await screen.findByText('Passwords do not match')).toBeInTheDocument();
  });

  it('registers and logs in automatically', async () => {
    const user = userEvent.setup();
    const fetchMock = mockFetchSequence(
      { body: testUser, status: 201 },
      { body: { token: 'jwt-token', user: testUser } }
    );
    renderRegister();

    await user.type(screen.getByLabelText('Email'), 'john.doe@example.com');
    await user.type(screen.getByLabelText('Password'), 'password123');
    await user.type(screen.getByLabelText('Confirm password'), 'password123');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    await waitFor(() => {
      expect(useAuthStore.getState().isAuthenticated).toBe(true);
    });

    expect(useAuthStore.getState().token).toBe('jwt-token');
    expect(fetchMock.mock.calls[0][0]).toBe('/api/auth/register');
    expect(fetchMock.mock.calls[1][0]).toBe('/api/auth/login');
  });

  it('shows a duplicate email error', async () => {
    const user = userEvent.setup();
    mockFetchOnce({ error: 'email already registered' }, 409);
    renderRegister();

    await user.type(screen.getByLabelText('Email'), 'john.doe@example.com');
    await user.type(screen.getByLabelText('Password'), 'password123');
    await user.type(screen.getByLabelText('Confirm password'), 'password123');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'An account with this email already exists'
    );
  });
});
