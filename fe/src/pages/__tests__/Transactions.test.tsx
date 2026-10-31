import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import Transactions from '../Transactions';
import { useAccountStore } from '../../stores/accountStore';
import { useCategoryStore } from '../../stores/categoryStore';
import { useTransactionStore } from '../../stores/transactionStore';
import { api } from '../../lib/api';
import type {
  Account,
  Category,
  ImportResult,
  Transaction,
} from '../../types';

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

function stubObjectUrls() {
  const createObjectURL = vi.fn(() => 'blob:transactions');
  const revokeObjectURL = vi.fn();
  const target = URL as unknown as Record<string, unknown>;
  target.createObjectURL = createObjectURL;
  target.revokeObjectURL = revokeObjectURL;
  return { createObjectURL, revokeObjectURL };
}

const importPreview: ImportResult = {
  total: 3,
  valid: 2,
  invalid: 1,
  errors: [{ row: 4, message: 'unknown account "Cash"' }],
  preview: [
    {
      row: 2,
      date: '2026-10-01',
      type: 'expense',
      amount: 12.5,
      description: 'Coffee',
      account: account.name,
      category: expenseCategory.name,
    },
    {
      row: 3,
      date: '2026-10-02',
      type: 'income',
      amount: 500,
      description: 'Salary',
      account: account.name,
      category: incomeCategory.name,
    },
  ],
};

function csvFile() {
  return new File(['date,type,amount\n'], 'transactions.csv', {
    type: 'text/csv',
  });
}

async function openImportDialog() {
  const user = userEvent.setup();
  renderTransactions();
  await user.click(screen.getByRole('button', { name: /import csv/i }));
  return { user, dialog: screen.getByRole('dialog') };
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

  afterEach(() => {
    const target = URL as unknown as Record<string, unknown>;
    delete target.createObjectURL;
    delete target.revokeObjectURL;
    vi.restoreAllMocks();
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

  it('exports the filtered transactions as a CSV download', async () => {
    const user = userEvent.setup();
    const blob = new Blob(['date,type,amount\n'], { type: 'text/csv' });
    const exportCsv = vi
      .spyOn(api, 'exportTransactionsCsv')
      .mockResolvedValue(blob);
    const { createObjectURL, revokeObjectURL } = stubObjectUrls();
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(() => undefined);

    useTransactionStore.setState({
      filters: { account_id: account.id, type: 'expense', page: 2 },
    });
    renderTransactions();

    await user.click(screen.getByRole('button', { name: /export csv/i }));

    await waitFor(() => {
      expect(exportCsv).toHaveBeenCalledWith(
        expect.objectContaining({ account_id: account.id, type: 'expense' })
      );
    });
    expect(createObjectURL).toHaveBeenCalledWith(blob);
    expect(click).toHaveBeenCalledTimes(1);
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:transactions');
  });

  it('previews a CSV import with counts, rows, and row errors', async () => {
    vi.spyOn(api, 'previewImport').mockResolvedValue(importPreview);
    const { user, dialog } = await openImportDialog();
    const file = csvFile();

    await user.upload(within(dialog).getByLabelText(/csv file/i), file);

    await waitFor(() => expect(api.previewImport).toHaveBeenCalledWith(file));

    const summary = await within(dialog).findByLabelText('Import summary');
    expect(within(summary).getByText('3')).toBeInTheDocument();
    expect(within(summary).getByText('2')).toBeInTheDocument();
    expect(within(summary).getByText('1')).toBeInTheDocument();
    expect(within(dialog).getByText('Row 4')).toBeInTheDocument();
    expect(
      within(dialog).getByText(/unknown account "Cash"/)
    ).toBeInTheDocument();
    expect(within(dialog).getByText('Coffee')).toBeInTheDocument();
  });

  it('keeps the commit button disabled when no rows are valid', async () => {
    vi.spyOn(api, 'previewImport').mockResolvedValue({
      total: 1,
      valid: 0,
      invalid: 1,
      errors: [{ row: 2, message: 'invalid amount "x"' }],
      preview: [],
    });
    const { user, dialog } = await openImportDialog();

    await user.upload(within(dialog).getByLabelText(/csv file/i), csvFile());

    const commit = await within(dialog).findByRole('button', {
      name: /import transactions/i,
    });
    expect(commit).toBeDisabled();
  });

  it('commits a valid import, reports success, and refreshes the list', async () => {
    vi.spyOn(api, 'previewImport').mockResolvedValue(importPreview);
    const importCsv = vi.spyOn(api, 'importTransactions').mockResolvedValue({
      total: 2,
      valid: 2,
      invalid: 0,
      errors: [],
      preview: [],
    });
    const { user, dialog } = await openImportDialog();
    const file = csvFile();

    await user.upload(within(dialog).getByLabelText(/csv file/i), file);
    await within(dialog).findByLabelText('Import summary');

    const fetchTransactions = vi.mocked(
      useTransactionStore.getState().fetchTransactions
    );
    fetchTransactions.mockClear();

    const commit = within(dialog).getByRole('button', {
      name: /import transactions/i,
    });
    await waitFor(() => expect(commit).toBeEnabled());
    await user.click(commit);

    await waitFor(() => expect(importCsv).toHaveBeenCalledWith(file));
    expect(
      await within(dialog).findByText(/imported 2 of 2 transactions/i)
    ).toBeInTheDocument();
    expect(fetchTransactions).toHaveBeenCalled();
  });
});
