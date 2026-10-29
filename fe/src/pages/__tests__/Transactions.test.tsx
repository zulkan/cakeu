import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import Transactions from '../Transactions';
import { useAccountStore } from '../../stores/accountStore';
import { useCategoryStore } from '../../stores/categoryStore';
import { useTransactionStore } from '../../stores/transactionStore';
import type { Account, Category, Transaction } from '../../types';

const account: Account = {
  id: 'account-1',
  name: 'Everyday checking',
  balance: 1250,
  currency: 'USD',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

const expenseCategory: Category = {
  id: 'category-expense',
  name: 'Groceries',
  type: 'expense',
  color: '#FB923C',
  created_at: '2026-01-01T00:00:00Z',
};

const incomeCategory: Category = {
  id: 'category-income',
  name: 'Salary',
  type: 'income',
  color: '#16A34A',
  created_at: '2026-01-01T00:00:00Z',
};

const transaction: Transaction = {
  id: 'transaction-1',
  account_id: account.id,
  category_id: expenseCategory.id,
  type: 'expense',
  amount: 42.5,
  description: 'Weekly groceries',
  date: '2026-10-20T00:00:00Z',
  created_at: '2026-10-20T00:00:00Z',
  account_name: account.name,
  category_name: expenseCategory.name,
};

function renderTransactions() {
  return render(
    <MemoryRouter>
      <Transactions />
    </MemoryRouter>
  );
}

describe('Transactions page', () => {
  beforeEach(() => {
    useAccountStore.setState({
      accounts: [account],
      isLoading: false,
      error: null,
      fetchAccounts: vi.fn(async () => undefined),
    });
    useCategoryStore.setState({
      categories: [expenseCategory, incomeCategory],
      isLoading: false,
      error: null,
      fetchCategories: vi.fn(async () => undefined),
    });
    useTransactionStore.setState({
      transactions: [transaction],
      total: 1,
      isLoading: false,
      error: null,
      filters: {},
      fetchTransactions: vi.fn(async () => undefined),
      setFilters: vi.fn(),
      createTransaction: vi.fn(async () => undefined),
      deleteTransaction: vi.fn(async () => undefined),
    });
  });

  it('renders the transaction list with amount and type', () => {
    renderTransactions();

    expect(screen.getAllByText('Weekly groceries').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Groceries').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Everyday checking').length).toBeGreaterThan(0);
    expect(screen.getAllByText('−$42.50').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Expense').length).toBeGreaterThan(0);
  });

  it('adds a transaction through the modal with an RFC3339 date', async () => {
    const user = userEvent.setup();
    renderTransactions();

    await user.click(
      screen.getByRole('button', { name: /add transaction/i })
    );

    const dialog = screen.getByRole('dialog');
    await user.selectOptions(
      within(dialog).getByLabelText('Category'),
      expenseCategory.id
    );
    await user.type(within(dialog).getByLabelText('Amount'), '42.50');
    await user.type(
      within(dialog).getByLabelText('Description'),
      'Coffee beans'
    );
    await user.click(
      within(dialog).getByRole('button', { name: /save transaction/i })
    );

    const createTransaction = vi.mocked(
      useTransactionStore.getState().createTransaction
    );

    await waitFor(() => {
      expect(createTransaction).toHaveBeenCalledWith(
        expect.objectContaining({
          account_id: account.id,
          category_id: expenseCategory.id,
          type: 'expense',
          amount: 42.5,
          description: 'Coffee beans',
          date: expect.stringMatching(
            /^\d{4}-\d{2}-\d{2}T00:00:00\.000Z$/
          ),
        })
      );
    });

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });

  it('only offers categories that match the selected type', async () => {
    const user = userEvent.setup();
    renderTransactions();

    await user.click(
      screen.getByRole('button', { name: /add transaction/i })
    );

    const dialog = screen.getByRole('dialog');
    const categorySelect = within(dialog).getByLabelText('Category');

    expect(
      within(categorySelect).queryByRole('option', { name: 'Salary' })
    ).not.toBeInTheDocument();

    await user.click(within(dialog).getByRole('button', { name: 'Income' }));

    expect(
      within(categorySelect).getByRole('option', { name: 'Salary' })
    ).toBeInTheDocument();
    expect(
      within(categorySelect).queryByRole('option', { name: 'Groceries' })
    ).not.toBeInTheDocument();
  });

  it('deletes a transaction after confirmation', async () => {
    const user = userEvent.setup();
    renderTransactions();

    const deleteButtons = screen.getAllByRole('button', {
      name: /delete transaction weekly groceries/i,
    });
    await user.click(deleteButtons[0]);

    const dialog = screen.getByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    const deleteTransaction = vi.mocked(
      useTransactionStore.getState().deleteTransaction
    );

    await waitFor(() => {
      expect(deleteTransaction).toHaveBeenCalledWith(transaction.id);
    });
  });

  it('applies filters through the store', async () => {
    const user = userEvent.setup();
    renderTransactions();

    await user.selectOptions(screen.getByLabelText('Account'), account.id);
    await user.click(screen.getByRole('button', { name: /apply filters/i }));

    const setFilters = vi.mocked(useTransactionStore.getState().setFilters);

    expect(setFilters).toHaveBeenCalledWith(
      expect.objectContaining({ account_id: account.id, page: 1 })
    );
  });

  it('requests the next page', async () => {
    const user = userEvent.setup();
    useTransactionStore.setState({
      total: 25,
      filters: { page: 1, page_size: 10 },
    });
    renderTransactions();

    await user.click(screen.getByRole('button', { name: /next/i }));

    const setFilters = vi.mocked(useTransactionStore.getState().setFilters);

    expect(setFilters).toHaveBeenCalledWith({ page: 2 });
  });

  it('shows an empty state when there are no transactions', () => {
    useTransactionStore.setState({ transactions: [], total: 0 });
    renderTransactions();

    expect(screen.getByText('No transactions yet')).toBeInTheDocument();
  });

  it('shows a loading state', () => {
    useTransactionStore.setState({ transactions: [], total: 0, isLoading: true });
    renderTransactions();

    expect(screen.getByText('Loading transactions…')).toBeInTheDocument();
  });

  it('shows an error state with retry', () => {
    useTransactionStore.setState({
      transactions: [],
      total: 0,
      error: 'Could not load transactions',
    });
    renderTransactions();

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Could not load transactions'
    );
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument();
  });
});
