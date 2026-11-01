import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import App from '../App';
import { useAuthStore } from '../stores/authStore';
import { useAccountStore } from '../stores/accountStore';
import { useCategoryStore } from '../stores/categoryStore';
import { useDashboardStore } from '../stores/dashboardStore';
import { mockApiFetch, testUser } from '../test/helpers';
import type { Account, Category, DashboardSummary } from '../types';

const account: Account = {
  id: 'account-1',
  name: 'Everyday checking',
  balance: 1250,
  currency: 'USD',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

const category: Category = {
  id: 'category-1',
  name: 'Groceries',
  type: 'expense',
  color: '#FB923C',
  created_at: '2026-01-01T00:00:00Z',
};

const summary: DashboardSummary = {
  total_balance: 1250,
  monthly_income: 3000,
  monthly_expense: 1750,
  monthly_net: 1250,
  transaction_count: 4,
  recent_transactions: [],
};

function mockDashboardApi() {
  return mockApiFetch({
    '/api/dashboard/summary': { body: summary },
    '/api/accounts': { body: [account] },
    '/api/categories': { body: [category] },
    '/api/budgets': { body: [] },
  });
}

function renderApp(initialEntry: string) {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <App />
    </MemoryRouter>
  );
}

describe('App router', () => {
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
    useAccountStore.setState({ accounts: [], isLoading: false, error: null });
    useCategoryStore.setState({
      categories: [],
      isLoading: false,
      error: null,
    });
    useDashboardStore.setState({
      summary: null,
      isLoading: false,
      error: null,
    });
  });

  it('redirects to /login from / when unauthenticated', () => {
    renderApp('/');

    expect(screen.getByText('Welcome back')).toBeInTheDocument();
    expect(screen.getAllByText('Catatku').length).toBeGreaterThan(0);
  });

  it('redirects /dashboard to /login when unauthenticated', () => {
    renderApp('/dashboard');

    expect(screen.getByText('Welcome back')).toBeInTheDocument();
  });

  it('renders the register page when unauthenticated', () => {
    renderApp('/register');

    expect(screen.getByText('Create your account')).toBeInTheDocument();
  });

  it('redirects /login to /dashboard when authenticated', async () => {
    useAuthStore.setState({
      user: testUser,
      token: 'jwt-token',
      isAuthenticated: true,
    });
    mockDashboardApi();

    renderApp('/login');

    expect(await screen.findByText('Total balance')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Dashboard' })).toBeInTheDocument();
  });

  it('redirects the legacy /home route to /dashboard', async () => {
    useAuthStore.setState({
      user: testUser,
      token: 'jwt-token',
      isAuthenticated: true,
    });
    mockDashboardApi();

    renderApp('/home');

    expect(await screen.findByText('Total balance')).toBeInTheDocument();
  });

  it('renders the app shell navigation for authenticated users', async () => {
    useAuthStore.setState({
      user: testUser,
      token: 'jwt-token',
      isAuthenticated: true,
    });
    mockDashboardApi();

    renderApp('/dashboard');

    expect(
      await screen.findByRole('link', { name: 'Transactions' })
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Accounts' })).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: 'Categories' })
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Recurring' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Reports' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /logout/i })).toBeInTheDocument();
  });
});
