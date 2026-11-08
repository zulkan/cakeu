import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Dashboard from '../Dashboard';
import { useAuthStore } from '../../stores/authStore';
import { useAccountStore } from '../../stores/accountStore';
import { useCategoryStore } from '../../stores/categoryStore';
import { useDashboardStore } from '../../stores/dashboardStore';
import { useBudgetStore } from '../../stores/budgetStore';
import { testUser } from '../../test/helpers';
import type {
  Account,
  Category,
  DashboardSummary,
  Transaction,
} from '../../types';

const account: Account = {
  id: 'account-1',
  name: 'Everyday checking',
  balance: 1250,
  opening_balance: 1250,
  entry_count: 1,
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

const transaction: Transaction = {
  id: 'transaction-1',
  account_id: account.id,
  category_id: category.id,
  type: 'expense',
  amount: 42.5,
  description: 'Weekly groceries',
  date: '2026-10-20T00:00:00Z',
  created_at: '2026-10-20T00:00:00Z',
  account_name: account.name,
  category_name: category.name,
  kind: 'normal',
  currency: 'USD',
  transfer_group_id: '00000000-0000-0000-0000-000000000000',
};

const summary: DashboardSummary = {
  balances: [{ currency: 'USD', total: 1250 }],
  monthly: [{ currency: 'USD', income: 3000, expense: 1750, net: 1250 }],
  transaction_count: 4,
  recent_transactions: [transaction],
};

function renderDashboard() {
  return render(
    <MemoryRouter>
      <Dashboard />
    </MemoryRouter>
  );
}

describe('Dashboard page', () => {
  beforeEach(() => {
    useAuthStore.setState({
      user: testUser,
      token: 'jwt-token',
      isAuthenticated: true,
      isLoading: false,
      error: null,
    });
    useDashboardStore.setState({
      summary: null,
      isLoading: false,
      error: null,
      fetchSummary: vi.fn(async () => undefined),
    });
    useAccountStore.setState({
      accounts: [],
      isLoading: false,
      error: null,
      fetchAccounts: vi.fn(async () => undefined),
    });
    useCategoryStore.setState({
      categories: [],
      isLoading: false,
      error: null,
      fetchCategories: vi.fn(async () => undefined),
    });
    useBudgetStore.setState({
      month: '2026-11',
      budgets: [],
      isLoading: false,
      error: null,
      isSaving: false,
      saveError: null,
      fetchBudgets: vi.fn(async () => undefined),
      upsertBudget: vi.fn(async () => undefined),
      deleteBudget: vi.fn(async () => undefined),
    });
  });

  it('renders the balance hero, monthly stats, and recent transactions', () => {
    useDashboardStore.setState({ summary });
    useAccountStore.setState({ accounts: [account] });
    useCategoryStore.setState({ categories: [category] });

    renderDashboard();

    expect(screen.getByText('Total balance')).toBeInTheDocument();
    expect(screen.getAllByText('$1,250.00').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('Monthly income')).toBeInTheDocument();
    expect(screen.getAllByText('$3,000.00').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('Monthly expense')).toBeInTheDocument();
    expect(screen.getAllByText('$1,750.00').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('Net this month')).toBeInTheDocument();
    expect(screen.getByText('Transactions')).toBeInTheDocument();
    expect(screen.getByText('4')).toBeInTheDocument();
    expect(screen.getByText('Weekly groceries')).toBeInTheDocument();
    expect(screen.getByText('−$42.50')).toBeInTheDocument();
  });

  it('renders one balance and monthly row per currency', () => {
    useDashboardStore.setState({
      summary: {
        balances: [
          { currency: 'USD', total: 1250 },
          { currency: 'IDR', total: 5000000 },
        ],
        monthly: [
          { currency: 'USD', income: 3000, expense: 1750, net: 1250 },
          { currency: 'IDR', income: 10000000, expense: 4000000, net: 6000000 },
        ],
        transaction_count: 7,
        recent_transactions: [transaction],
      },
    });
    useAccountStore.setState({
      accounts: [
        account,
        { ...account, id: 'account-2', currency: 'IDR', balance: 5000000 },
      ],
    });
    useCategoryStore.setState({ categories: [category] });

    renderDashboard();

    expect(screen.getAllByText(/IDR/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/USD/).length).toBeGreaterThan(0);
    expect(screen.getAllByText('Income').length).toBeGreaterThan(1);
    expect(screen.getByText('Transactions')).toBeInTheDocument();
    expect(screen.getByText('7')).toBeInTheDocument();
    expect(screen.queryByText('Monthly income')).not.toBeInTheDocument();
  });

  it('shows the setup checklist and empty transaction state when nothing exists', () => {
    useDashboardStore.setState({
      summary: {
        balances: [],
        monthly: [],
        transaction_count: 0,
        recent_transactions: [],
      },
    });

    renderDashboard();

    expect(screen.getByText('Finish setting up Catatku')).toBeInTheDocument();
    expect(screen.getByText('Create an account')).toBeInTheDocument();
    expect(screen.getByText('Create a category')).toBeInTheDocument();
    expect(screen.getByText('No transactions yet')).toBeInTheDocument();
  });

  it('shows a loading state before the summary arrives', () => {
    useDashboardStore.setState({ isLoading: true });

    renderDashboard();

    expect(screen.getByText('Loading your dashboard…')).toBeInTheDocument();
  });

  it('shows an error state with a retry action', () => {
    useDashboardStore.setState({ error: 'Could not load summary' });

    renderDashboard();

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Could not load summary'
    );
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument();
  });
});
