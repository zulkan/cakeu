import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import Transactions from '../Transactions';
import { useAccountStore } from '../../stores/accountStore';
import { useCategoryStore } from '../../stores/categoryStore';
import { useTransactionStore } from '../../stores/transactionStore';
import { useToastStore } from '../../stores/toastStore';
import { api } from '../../lib/api';
import type {
  Account,
  Attachment,
  Category,
  ImportMapping,
  ImportResult,
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
  kind: 'normal',
  currency: 'USD',
  transfer_group_id: '00000000-0000-0000-0000-000000000000',
};

function renderTransactions() {
  return render(
    <MemoryRouter>
      <Transactions />
    </MemoryRouter>
  );
}

const attachment: Attachment = {
  id: 'attachment-1',
  transaction_id: transaction.id,
  filename: 'receipt.png',
  content_type: 'image/png',
  size_bytes: 2048,
  created_at: '2026-11-08T10:00:00Z',
};

async function openAttachmentsModal() {
  const user = userEvent.setup();
  renderTransactions();
  await user.click(
    screen.getAllByRole('button', {
      name: /attachments for weekly groceries/i,
    })[0]
  );
  const dialog = await screen.findByRole('dialog');
  return { user, dialog };
}

function stubObjectUrls() {
  const createObjectURL = vi.fn(() => 'blob:transactions');
  const revokeObjectURL = vi.fn();
  const target = URL as unknown as Record<string, unknown>;
  target.createObjectURL = createObjectURL;
  target.revokeObjectURL = revokeObjectURL;
  return { createObjectURL, revokeObjectURL };
}

const importMapping: ImportMapping = {
  sheet: 'csv',
  header_row: 1,
  data_start: 2,
  data_end: 3,
  fields: {
    date: { column: 1, date_format: 'iso' },
    amount: { column: 3, amount_format: 'plain', amount_mode: 'abs' },
    description: { column: 4, carry_forward: true },
    account: { column: 5 },
    category: { column: 6, value_map: { Food: 'Groceries' } },
  },
  type: { column: 2 },
  confidence: 0.86,
};

const importPreview: ImportResult = {
  total: 3,
  valid: 2,
  invalid: 1,
  skipped: 0,
  imported: 0,
  duplicate_count: 0,
  sheet: 'csv',
  sheets: ['csv'],
  mode: 'ai',
  mapping: importMapping,
  errors: [{ row: 4, message: 'unknown account "Cash"' }],
  duplicates: [],
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
      amount: 100,
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

function xlsxFile() {
  return new File(['sheet-bytes'], 'transactions.xlsx', {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  });
}

const workbookPreview: ImportResult = {
  ...importPreview,
  sheet: 'January',
  sheets: ['January', 'February', 'March'],
  mapping: { ...importMapping, sheet: 'January' },
};

const februaryPreview: ImportResult = {
  total: 5,
  valid: 4,
  invalid: 1,
  skipped: 0,
  imported: 0,
  duplicate_count: 0,
  sheet: 'February',
  sheets: ['January', 'February', 'March'],
  mode: 'ai',
  mapping: { ...importMapping, sheet: 'February', data_end: 5 },
  errors: [{ row: 6, message: 'invalid amount "x"' }],
  duplicates: [],
  preview: [
    {
      row: 2,
      date: '2026-02-01',
      type: 'expense',
      amount: 9.5,
      description: 'Lunch',
      account: account.name,
      category: expenseCategory.name,
    },
  ],
};

const duplicatePreview: ImportResult = {
  ...importPreview,
  valid: 2,
  invalid: 0,
  duplicate_count: 1,
  duplicates: [{ row: 2, message: 'already exists in the ledger' }],
  errors: [],
  preview: [
    {
      row: 2,
      date: '2026-10-01',
      type: 'expense',
      amount: 12.5,
      description: 'Coffee',
      account: account.name,
      category: expenseCategory.name,
      duplicate: true,
    },
    {
      row: 3,
      date: '2026-10-02',
      type: 'income',
      amount: 100,
      description: 'Salary',
      account: account.name,
      category: incomeCategory.name,
    },
  ],
};

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
      updateTransaction: vi.fn(async () => undefined),
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

  it('renders opening entries as locked system rows', () => {
    useTransactionStore.setState({
      transactions: [
        {
          ...transaction,
          id: 'opening-1',
          category_id: '00000000-0000-0000-0000-000000000000',
          category_name: '',
          description: 'Opening balance',
          type: 'income',
          amount: 1000,
          kind: 'opening',
        },
      ],
      total: 1,
    });

    renderTransactions();

    expect(screen.getAllByText('Opening balance').length).toBeGreaterThan(0);
    expect(screen.getAllByText('System').length).toBeGreaterThan(0);
    expect(
      screen.queryByRole('button', { name: /edit transaction/i })
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /delete transaction/i })
    ).not.toBeInTheDocument();
  });

  it('renders transfer rows with a Transfer label and no edit action', () => {
    useTransactionStore.setState({
      transactions: [
        {
          ...transaction,
          id: 'transfer-leg-1',
          category_id: '00000000-0000-0000-0000-000000000000',
          category_name: '',
          description: 'Move to savings',
          kind: 'transfer',
          transfer_group_id: 'group-1',
        },
      ],
      total: 1,
    });

    renderTransactions();

    expect(screen.getAllByText('Transfer').length).toBeGreaterThan(0);
    expect(
      screen.queryByRole('button', { name: /edit transaction/i })
    ).not.toBeInTheDocument();
    expect(
      screen.getAllByRole('button', { name: /delete transfer/i }).length
    ).toBeGreaterThan(0);
  });

  it('deletes both sides of a transfer through the transfer endpoint', async () => {
    const user = userEvent.setup();
    const deleteTransfer = vi
      .spyOn(api, 'deleteTransfer')
      .mockResolvedValue(undefined);
    useTransactionStore.setState({
      transactions: [
        {
          ...transaction,
          id: 'transfer-leg-1',
          category_id: '00000000-0000-0000-0000-000000000000',
          category_name: '',
          description: 'Move to savings',
          kind: 'transfer',
          transfer_group_id: 'group-1',
        },
      ],
      total: 1,
    });

    renderTransactions();

    await user.click(
      screen.getAllByRole('button', { name: /delete transfer/i })[0]
    );

    const dialog = screen.getByRole('dialog');
    expect(
      within(dialog).getByText(/both sides of this transfer/i)
    ).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    await waitFor(() => {
      expect(deleteTransfer).toHaveBeenCalledWith('group-1');
    });
    expect(
      useTransactionStore.getState().deleteTransaction
    ).not.toHaveBeenCalled();
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
    await user.type(within(dialog).getByLabelText(/Amount/), '42.50');
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

  it('edits a transaction through the prefilled modal', async () => {
    const user = userEvent.setup();
    renderTransactions();

    await user.click(
      screen.getAllByRole('button', {
        name: /edit transaction weekly groceries/i,
      })[0]
    );

    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByLabelText(/Amount/)).toHaveValue(42.5);
    expect(within(dialog).getByLabelText('Date')).toHaveValue('2026-10-20');
    expect(within(dialog).getByLabelText('Category')).toHaveValue(
      expenseCategory.id
    );
    expect(within(dialog).getByLabelText('Account')).toHaveValue(account.id);
    expect(within(dialog).getByLabelText('Description')).toHaveValue(
      'Weekly groceries'
    );

    await user.clear(within(dialog).getByLabelText(/Amount/));
    await user.type(within(dialog).getByLabelText(/Amount/), '55.75');
    await user.click(within(dialog).getByRole('button', { name: 'Income' }));
    await user.selectOptions(
      within(dialog).getByLabelText('Category'),
      incomeCategory.id
    );
    await user.click(
      within(dialog).getByRole('button', { name: /save changes/i })
    );

    const updateTransaction = vi.mocked(
      useTransactionStore.getState().updateTransaction
    );

    await waitFor(() => {
      expect(updateTransaction).toHaveBeenCalledWith(
        transaction.id,
        expect.objectContaining({
          account_id: account.id,
          category_id: incomeCategory.id,
          type: 'income',
          amount: 55.75,
          description: 'Weekly groceries',
          date: '2026-10-20T00:00:00.000Z',
        })
      );
    });

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
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

  it('changes the page size and resets to the first page', async () => {
    const user = userEvent.setup();
    useTransactionStore.setState({
      total: 25,
      filters: { page: 2, page_size: 10 },
    });
    renderTransactions();

    await user.selectOptions(screen.getByLabelText('Rows per page'), '25');

    const setFilters = vi.mocked(useTransactionStore.getState().setFilters);
    expect(setFilters).toHaveBeenCalledWith({ page_size: 25, page: 1 });
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

    await waitFor(() =>
      expect(api.previewImport).toHaveBeenCalledWith(file, {
        sheet: undefined,
        mode: 'ai',
      })
    );

    const summary = await within(dialog).findByLabelText('Import summary');
    expect(within(summary).getByText('3')).toBeInTheDocument();
    expect(within(summary).getByText('2')).toBeInTheDocument();
    expect(within(summary).getByText('1')).toBeInTheDocument();
    expect(within(summary).getByText('Skipped')).toBeInTheDocument();
    expect(within(dialog).getByText('Row 4')).toBeInTheDocument();
    expect(
      within(dialog).getByText(/unknown account "Cash"/)
    ).toBeInTheDocument();
    expect(within(dialog).getByText('Coffee')).toBeInTheDocument();
  });

  it('summarises duplicate rows and marks them in the preview', async () => {
    vi.spyOn(api, 'previewImport').mockResolvedValue(duplicatePreview);
    const { user, dialog } = await openImportDialog();

    await user.upload(within(dialog).getByLabelText(/csv file/i), csvFile());

    const summary = await within(dialog).findByLabelText('Import summary');
    expect(within(summary).getByText('Duplicates')).toBeInTheDocument();
    expect(
      within(dialog).getByText(/1 duplicate will be skipped/i)
    ).toBeInTheDocument();
    expect(within(dialog).getByText('Duplicate')).toBeInTheDocument();
    expect(
      within(dialog).getByText(/already exists in the ledger/i)
    ).toBeInTheDocument();
    expect(
      within(dialog).getByLabelText(/import duplicates anyway/i)
    ).not.toBeChecked();
  });

  it('sends allow_duplicates when the duplicate checkbox is checked', async () => {
    vi.spyOn(api, 'previewImport').mockResolvedValue(duplicatePreview);
    const importTransactions = vi
      .spyOn(api, 'importTransactions')
      .mockResolvedValue({
        ...duplicatePreview,
        total: 2,
        imported: 1,
      });
    const { user, dialog } = await openImportDialog();
    const file = csvFile();

    await user.upload(within(dialog).getByLabelText(/csv file/i), file);
    await within(dialog).findByLabelText('Import summary');

    const checkbox = within(dialog).getByLabelText(/import duplicates anyway/i);
    expect(checkbox).not.toBeChecked();
    await user.click(checkbox);
    expect(checkbox).toBeChecked();

    const commit = within(dialog).getByRole('button', {
      name: /import transactions/i,
    });
    await waitFor(() => expect(commit).toBeEnabled());
    await user.click(commit);

    await waitFor(() =>
      expect(importTransactions).toHaveBeenCalledWith(
        file,
        expect.objectContaining({ allow_duplicates: true })
      )
    );
    expect(
      await within(dialog).findByText(
        /imported 1 of 2 transactions, skipped 1 duplicate/i
      )
    ).toBeInTheDocument();
  });

  it('omits allow_duplicates when the duplicate checkbox stays unchecked', async () => {
    vi.spyOn(api, 'previewImport').mockResolvedValue(duplicatePreview);
    const importTransactions = vi
      .spyOn(api, 'importTransactions')
      .mockResolvedValue({
        ...duplicatePreview,
        total: 2,
        imported: 1,
      });
    const { user, dialog } = await openImportDialog();
    const file = csvFile();

    await user.upload(within(dialog).getByLabelText(/csv file/i), file);
    await within(dialog).findByLabelText('Import summary');

    const commit = within(dialog).getByRole('button', {
      name: /import transactions/i,
    });
    await waitFor(() => expect(commit).toBeEnabled());
    await user.click(commit);

    await waitFor(() => expect(importTransactions).toHaveBeenCalled());
    const [, options] = importTransactions.mock.calls[0];
    expect(options).not.toHaveProperty('allow_duplicates');
  });

  it('shows the AI mapping and sends it on commit', async () => {
    vi.spyOn(api, 'previewImport').mockResolvedValue(importPreview);
    const importTransactions = vi
      .spyOn(api, 'importTransactions')
      .mockResolvedValue(importPreview);
    const user = userEvent.setup();
    renderTransactions();

    await user.click(screen.getByRole('button', { name: /import csv/i }));
    const dialog = await screen.findByRole('dialog');
    await user.upload(within(dialog).getByLabelText(/csv file/i), csvFile());

    await within(dialog).findByText(/AI mapping/i);
    expect(within(dialog).getByText(/header row 1/i)).toBeInTheDocument();
    expect(within(dialog).getByText(/data rows 2–3/i)).toBeInTheDocument();
    expect(within(dialog).getByText(/date iso/i)).toBeInTheDocument();
    expect(within(dialog).getByText(/amount plain/i)).toBeInTheDocument();
    expect(within(dialog).getByText(/mode abs/i)).toBeInTheDocument();
    expect(within(dialog).getByText(/carry forward/i)).toBeInTheDocument();
    expect(within(dialog).getByText(/Food → Groceries/i)).toBeInTheDocument();
    expect(within(dialog).getByText(/confidence 86%/i)).toBeInTheDocument();

    const toggle = within(dialog).getByRole('button', { name: /AI mapping/i });
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    await user.click(toggle);
    expect(within(dialog).queryByText(/data rows 2–3/i)).not.toBeInTheDocument();

    await user.click(
      within(dialog).getByRole('button', { name: /import transactions/i })
    );

    await waitFor(() =>
      expect(importTransactions).toHaveBeenCalledWith(
        expect.any(File),
        expect.objectContaining({ mode: 'ai', mapping: importPreview.mapping })
      )
    );
  });

  it('shows the privacy note before uploading a file', async () => {
    const preview = vi.spyOn(api, 'previewImport');
    const { dialog } = await openImportDialog();

    expect(
      within(dialog).getByText(/account\/category names are sent to the AI provider/i)
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText(/nothing is imported until you confirm/i)
    ).toBeInTheDocument();
    expect(preview).not.toHaveBeenCalled();
  });

  it('keeps the selected sheet after an AI failure and retries it', async () => {
    vi.spyOn(api, 'previewImport')
      .mockResolvedValueOnce(workbookPreview)
      .mockRejectedValueOnce(new Error('AI mapping failed'))
      .mockResolvedValueOnce(februaryPreview);
    const { user, dialog } = await openImportDialog();
    const file = xlsxFile();

    await user.upload(within(dialog).getByLabelText(/csv file/i), file);
    await within(dialog).findByLabelText('Import summary');
    await user.selectOptions(within(dialog).getByLabelText('Sheet'), 'February');

    expect(await within(dialog).findByText('AI mapping failed')).toBeInTheDocument();
    expect(within(dialog).getByLabelText('Sheet')).toHaveValue('February');

    await user.click(within(dialog).getByRole('button', { name: /try again/i }));

    await waitFor(() =>
      expect(api.previewImport).toHaveBeenLastCalledWith(file, {
        sheet: 'February',
        mode: 'ai',
      })
    );
    expect(await within(dialog).findByLabelText('Import summary')).toBeInTheDocument();
  });

  it('standard import after an AI failure targets the selected sheet', async () => {
    vi.spyOn(api, 'previewImport')
      .mockResolvedValueOnce(workbookPreview)
      .mockRejectedValueOnce(new Error('AI mapping failed'))
      .mockResolvedValueOnce({ ...februaryPreview, mode: 'strict', mapping: undefined });
    const { user, dialog } = await openImportDialog();
    const file = xlsxFile();

    await user.upload(within(dialog).getByLabelText(/csv file/i), file);
    await within(dialog).findByLabelText('Import summary');
    await user.selectOptions(within(dialog).getByLabelText('Sheet'), 'February');

    expect(await within(dialog).findByText('AI mapping failed')).toBeInTheDocument();
    await user.click(
      within(dialog).getByRole('button', { name: /standard import/i })
    );

    await waitFor(() =>
      expect(api.previewImport).toHaveBeenLastCalledWith(file, {
        sheet: 'February',
        mode: 'strict',
      })
    );
    expect(await within(dialog).findByLabelText('Import summary')).toBeInTheDocument();
  });

  it('keeps the commit button disabled when no rows are valid', async () => {
    vi.spyOn(api, 'previewImport').mockResolvedValue({
      total: 1,
      valid: 0,
      invalid: 1,
      skipped: 0,
      imported: 0,
      duplicate_count: 0,
      sheet: 'Sheet1',
      sheets: ['Sheet1'],
      errors: [{ row: 2, message: 'invalid amount "x"' }],
      duplicates: [],
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
      skipped: 0,
      imported: 2,
      duplicate_count: 0,
      sheet: 'Sheet1',
      sheets: ['Sheet1'],
      errors: [],
      duplicates: [],
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

    await waitFor(() =>
      expect(importCsv).toHaveBeenCalledWith(file, {
        sheet: 'csv',
        mode: 'ai',
        mapping: importPreview.mapping,
      })
    );
    expect(
      await within(dialog).findByText(/imported 2 of 2 transactions/i)
    ).toBeInTheDocument();
    expect(fetchTransactions).toHaveBeenCalled();
  });

  it('accepts CSV, XLSX, and legacy XLS files', async () => {
    const { dialog } = await openImportDialog();

    const input = within(dialog).getByLabelText(/csv file/i);
    expect(input).toHaveAttribute('accept', '.csv,.xlsx,.xls');
    expect(
      within(dialog).getByText(/\.xlsx, or \.xls file/i)
    ).toBeInTheDocument();
  });

  it('defaults the sheet selector to the previewed sheet and hides it for single-sheet workbooks', async () => {
    vi.spyOn(api, 'previewImport').mockResolvedValue(importPreview);
    const { user, dialog } = await openImportDialog();

    await user.upload(within(dialog).getByLabelText(/csv file/i), csvFile());
    await within(dialog).findByLabelText('Import summary');

    expect(within(dialog).queryByLabelText('Sheet')).not.toBeInTheDocument();
  });

  it('re-previews the workbook when a different sheet is selected', async () => {
    vi.spyOn(api, 'previewImport')
      .mockResolvedValueOnce(workbookPreview)
      .mockResolvedValueOnce(februaryPreview);
    const { user, dialog } = await openImportDialog();
    const file = xlsxFile();

    await user.upload(within(dialog).getByLabelText(/csv file/i), file);
    await within(dialog).findByLabelText('Import summary');

    const sheetSelect = within(dialog).getByLabelText('Sheet');
    expect(sheetSelect).toHaveValue('January');
    expect(
      within(sheetSelect).getByRole('option', { name: 'February' })
    ).toBeInTheDocument();

    await user.selectOptions(sheetSelect, 'February');

    await waitFor(() =>
      expect(api.previewImport).toHaveBeenLastCalledWith(file, {
        sheet: 'February',
        mode: 'ai',
      })
    );

    const summary = await within(dialog).findByLabelText('Import summary');
    await waitFor(() =>
      expect(within(summary).getByText('5')).toBeInTheDocument()
    );
    expect(within(summary).getByText('4')).toBeInTheDocument();
    expect(within(dialog).getByText('Lunch')).toBeInTheDocument();
    expect(within(dialog).getByLabelText('Sheet')).toHaveValue('February');
  });

  it('omits the mapping for a strict commit', async () => {
    vi.spyOn(api, 'previewImport').mockResolvedValue({
      ...importPreview,
      mode: 'strict',
      mapping: undefined,
    });
    const importTransactions = vi
      .spyOn(api, 'importTransactions')
      .mockResolvedValue({
        total: 2,
        valid: 2,
        invalid: 0,
        skipped: 0,
        imported: 2,
        duplicate_count: 0,
        sheet: 'csv',
        sheets: ['csv'],
        mode: 'strict',
        errors: [],
        duplicates: [],
        preview: [],
      });
    const { user, dialog } = await openImportDialog();
    const file = csvFile();

    await user.upload(within(dialog).getByLabelText(/csv file/i), file);
    await within(dialog).findByLabelText('Import summary');

    const commit = within(dialog).getByRole('button', {
      name: /import transactions/i,
    });
    await waitFor(() => expect(commit).toBeEnabled());
    await user.click(commit);

    await waitFor(() => expect(importTransactions).toHaveBeenCalled());
    const [, options] = importTransactions.mock.calls[0];
    expect(options).toEqual(
      expect.objectContaining({ sheet: 'csv', mode: 'strict' })
    );
    expect(options).not.toHaveProperty('mapping');
  });

  it('distinguishes invalid rows from skipped rows after importing', async () => {
    vi.spyOn(api, 'previewImport').mockResolvedValue({
      ...importPreview,
      invalid: 1,
      skipped: 2,
    });
    vi.spyOn(api, 'importTransactions').mockResolvedValue({
      total: 3,
      valid: 2,
      invalid: 1,
      skipped: 2,
      imported: 2,
      duplicate_count: 0,
      sheet: 'csv',
      sheets: ['csv'],
      mode: 'ai',
      errors: [],
      duplicates: [],
      preview: [],
    });
    const { user, dialog } = await openImportDialog();

    await user.upload(within(dialog).getByLabelText(/csv file/i), csvFile());
    await within(dialog).findByLabelText('Import summary');
    const commit = within(dialog).getByRole('button', {
      name: /import transactions/i,
    });
    await waitFor(() => expect(commit).toBeEnabled());
    await user.click(commit);

    expect(
      await within(dialog).findByText(/1 row could not be imported/i)
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText(/2 rows were skipped as non-transaction rows/i)
    ).toBeInTheDocument();
  });

  it('commits the selected sheet of a multi-sheet workbook', async () => {
    vi.spyOn(api, 'previewImport')
      .mockResolvedValueOnce(workbookPreview)
      .mockResolvedValueOnce(februaryPreview);
    const importCsv = vi.spyOn(api, 'importTransactions').mockResolvedValue({
      total: 4,
      valid: 4,
      invalid: 0,
      skipped: 0,
      imported: 4,
      duplicate_count: 0,
      sheet: 'February',
      sheets: ['January', 'February', 'March'],
      errors: [],
      duplicates: [],
      preview: [],
    });
    const { user, dialog } = await openImportDialog();
    const file = xlsxFile();

    await user.upload(within(dialog).getByLabelText(/csv file/i), file);
    await within(dialog).findByLabelText('Import summary');
    await user.selectOptions(within(dialog).getByLabelText('Sheet'), 'February');
    await waitFor(() =>
      expect(api.previewImport).toHaveBeenLastCalledWith(file, {
        sheet: 'February',
        mode: 'ai',
      })
    );

    const commit = within(dialog).getByRole('button', {
      name: /import transactions/i,
    });
    await waitFor(() => expect(commit).toBeEnabled());
    await user.click(commit);

    await waitFor(() =>
      expect(importCsv).toHaveBeenCalledWith(file, {
        sheet: 'February',
        mode: 'ai',
        mapping: februaryPreview.mapping,
      })
    );
    expect(
      await within(dialog).findByText(/imported 4 of 4 transactions/i)
    ).toBeInTheDocument();
  });

  it('shows a paperclip action on normal transactions', () => {
    renderTransactions();

    expect(
      screen.getAllByRole('button', {
        name: /attachments for weekly groceries/i,
      }).length
    ).toBeGreaterThan(0);
  });

  it('hides the paperclip action on system rows', () => {
    useTransactionStore.setState({
      transactions: [
        {
          ...transaction,
          id: 'opening-1',
          category_id: '00000000-0000-0000-0000-000000000000',
          category_name: '',
          description: 'Opening balance',
          type: 'income',
          amount: 1000,
          kind: 'opening',
        },
      ],
      total: 1,
    });

    renderTransactions();

    expect(
      screen.queryByRole('button', { name: /attachments for/i })
    ).not.toBeInTheDocument();
  });

  it('opens the attachments modal and lists the files', async () => {
    vi.spyOn(api, 'listAttachments').mockResolvedValue([attachment]);

    const { dialog } = await openAttachmentsModal();

    expect(api.listAttachments).toHaveBeenCalledWith(
      transaction.id,
      expect.anything()
    );
    expect(await within(dialog).findByText('receipt.png')).toBeInTheDocument();
    expect(within(dialog).getByText(/2 KB/)).toBeInTheDocument();
  });

  it('shows the empty state when a transaction has no attachments', async () => {
    vi.spyOn(api, 'listAttachments').mockResolvedValue([]);

    const { dialog } = await openAttachmentsModal();

    expect(
      await within(dialog).findByText('No attachments yet')
    ).toBeInTheDocument();
  });

  it('uploads an attachment, refreshes the list, and toasts', async () => {
    const list = vi
      .spyOn(api, 'listAttachments')
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([attachment]);
    const upload = vi
      .spyOn(api, 'uploadAttachment')
      .mockResolvedValue(attachment);
    useToastStore.setState({ toasts: [] });

    const { user, dialog } = await openAttachmentsModal();
    await within(dialog).findByText('No attachments yet');

    const file = new File(['png-bytes'], 'receipt.png', {
      type: 'image/png',
    });
    await user.upload(
      within(dialog).getByLabelText('Upload attachment'),
      file
    );

    await waitFor(() =>
      expect(upload).toHaveBeenCalledWith(transaction.id, file)
    );
    expect(await within(dialog).findByText('receipt.png')).toBeInTheDocument();
    expect(list).toHaveBeenCalledTimes(2);
    expect(
      useToastStore
        .getState()
        .toasts.some((toast) => toast.title === 'Attachment uploaded')
    ).toBe(true);
  });

  it('rejects attachments larger than 5 MB before uploading', async () => {
    vi.spyOn(api, 'listAttachments').mockResolvedValue([]);
    const upload = vi
      .spyOn(api, 'uploadAttachment')
      .mockResolvedValue(attachment);

    const { user, dialog } = await openAttachmentsModal();
    await within(dialog).findByText('No attachments yet');

    const oversized = new File(['png-bytes'], 'huge.png', {
      type: 'image/png',
    });
    Object.defineProperty(oversized, 'size', {
      value: 5 * 1024 * 1024 + 1,
    });

    await user.upload(
      within(dialog).getByLabelText('Upload attachment'),
      oversized
    );

    expect(await within(dialog).findByRole('alert')).toHaveTextContent(
      /5 MB or smaller/i
    );
    expect(upload).not.toHaveBeenCalled();
  });

  it('surfaces a server upload error inline', async () => {
    vi.spyOn(api, 'listAttachments').mockResolvedValue([]);
    vi.spyOn(api, 'uploadAttachment').mockRejectedValue(
      new Error('file exceeds the 5 MB limit')
    );

    const { user, dialog } = await openAttachmentsModal();
    await within(dialog).findByText('No attachments yet');

    const file = new File(['png-bytes'], 'receipt.png', {
      type: 'image/png',
    });
    await user.upload(
      within(dialog).getByLabelText('Upload attachment'),
      file
    );

    expect(
      await within(dialog).findByText(/file exceeds the 5 MB limit/i)
    ).toBeInTheDocument();
  });

  it('deletes an attachment after confirmation and toasts', async () => {
    vi.spyOn(api, 'listAttachments').mockResolvedValue([attachment]);
    const remove = vi
      .spyOn(api, 'deleteAttachment')
      .mockResolvedValue(undefined);
    useToastStore.setState({ toasts: [] });

    const { user, dialog } = await openAttachmentsModal();
    await user.click(
      await within(dialog).findByRole('button', {
        name: /delete receipt\.png/i,
      })
    );

    const confirm = await screen.findByRole('dialog', {
      name: /delete attachment/i,
    });
    await user.click(within(confirm).getByRole('button', { name: 'Delete' }));

    await waitFor(() =>
      expect(remove).toHaveBeenCalledWith(attachment.id)
    );
    await waitFor(() =>
      expect(
        within(dialog).queryByText('receipt.png')
      ).not.toBeInTheDocument()
    );
    expect(
      useToastStore
        .getState()
        .toasts.some((toast) => toast.title === 'Attachment deleted')
    ).toBe(true);
  });

  it('downloads an attachment through a revoked object URL', async () => {
    vi.spyOn(api, 'listAttachments').mockResolvedValue([attachment]);
    const blob = new Blob(['png-bytes'], { type: 'image/png' });
    const download = vi
      .spyOn(api, 'downloadAttachment')
      .mockResolvedValue(blob);
    const { createObjectURL, revokeObjectURL } = stubObjectUrls();
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(() => undefined);

    const { user, dialog } = await openAttachmentsModal();
    await user.click(
      await within(dialog).findByRole('button', {
        name: /download receipt\.png/i,
      })
    );

    await waitFor(() =>
      expect(download).toHaveBeenCalledWith(attachment.id)
    );
    expect(createObjectURL).toHaveBeenCalledWith(blob);
    expect(click).toHaveBeenCalledTimes(1);
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:transactions');
  });
});
