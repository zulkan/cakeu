import { describe, it, expect, beforeEach, vi } from 'vitest';
import { api } from '../../lib/api';
import { useAccountStore } from '../accountStore';
import { useCategoryStore } from '../categoryStore';
import { useTransactionStore } from '../transactionStore';
import { useDashboardStore } from '../dashboardStore';
import type {
  Account,
  Category,
  DashboardSummary,
  Transaction,
} from '../../types';

vi.mock('../../lib/api', () => ({
  api: {
    listAccounts: vi.fn(),
    createAccount: vi.fn(),
    updateAccount: vi.fn(),
    deleteAccount: vi.fn(),
    listCategories: vi.fn(),
    createCategory: vi.fn(),
    deleteCategory: vi.fn(),
    listTransactions: vi.fn(),
    createTransaction: vi.fn(),
    deleteTransaction: vi.fn(),
    getDashboardSummary: vi.fn(),
  },
}));

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
};

const summary: DashboardSummary = {
  total_balance: 1250,
  monthly_income: 3000,
  monthly_expense: 1750,
  monthly_net: 1250,
  transaction_count: 1,
  recent_transactions: [transaction],
};

describe('finance stores', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useAccountStore.setState({ accounts: [], isLoading: false, error: null });
    useCategoryStore.setState({
      categories: [],
      isLoading: false,
      error: null,
    });
    useTransactionStore.setState({
      transactions: [],
      total: 0,
      isLoading: false,
      error: null,
      filters: {},
    });
    useDashboardStore.setState({
      summary: null,
      isLoading: false,
      error: null,
    });
  });

  describe('account store', () => {
    it('loads accounts', async () => {
      vi.mocked(api.listAccounts).mockResolvedValue([account]);

      await useAccountStore.getState().fetchAccounts();

      expect(useAccountStore.getState().accounts).toEqual([account]);
      expect(useAccountStore.getState().isLoading).toBe(false);
    });

    it('captures a load error', async () => {
      vi.mocked(api.listAccounts).mockRejectedValue(new Error('boom'));

      await useAccountStore.getState().fetchAccounts();

      expect(useAccountStore.getState().error).toBe('boom');
      expect(useAccountStore.getState().isLoading).toBe(false);
    });

    it('appends a created account', async () => {
      vi.mocked(api.createAccount).mockResolvedValue(account);

      await useAccountStore.getState().createAccount({
        name: account.name,
        balance: account.balance,
        currency: account.currency,
      });

      expect(useAccountStore.getState().accounts).toEqual([account]);
    });

    it('replaces an updated account', async () => {
      const updated = { ...account, name: 'Main checking', balance: 2000 };
      useAccountStore.setState({ accounts: [account] });
      vi.mocked(api.updateAccount).mockResolvedValue(updated);

      await useAccountStore.getState().updateAccount(account.id, {
        name: updated.name,
        balance: updated.balance,
      });

      expect(useAccountStore.getState().accounts).toEqual([updated]);
    });

    it('removes a deleted account', async () => {
      useAccountStore.setState({ accounts: [account] });
      vi.mocked(api.deleteAccount).mockResolvedValue(undefined);

      await useAccountStore.getState().deleteAccount(account.id);

      expect(useAccountStore.getState().accounts).toEqual([]);
    });
  });

  describe('category store', () => {
    it('loads categories', async () => {
      vi.mocked(api.listCategories).mockResolvedValue([category]);

      await useCategoryStore.getState().fetchCategories();

      expect(useCategoryStore.getState().categories).toEqual([category]);
    });

    it('captures a load error', async () => {
      vi.mocked(api.listCategories).mockRejectedValue(new Error('nope'));

      await useCategoryStore.getState().fetchCategories();

      expect(useCategoryStore.getState().error).toBe('nope');
    });

    it('appends and removes categories', async () => {
      vi.mocked(api.createCategory).mockResolvedValue(category);
      vi.mocked(api.deleteCategory).mockResolvedValue(undefined);

      await useCategoryStore.getState().createCategory({
        name: category.name,
        type: category.type,
        color: category.color,
      });
      expect(useCategoryStore.getState().categories).toEqual([category]);

      await useCategoryStore.getState().deleteCategory(category.id);
      expect(useCategoryStore.getState().categories).toEqual([]);
    });
  });

  describe('transaction store', () => {
    it('loads a paginated transaction page', async () => {
      vi.mocked(api.listTransactions).mockResolvedValue({
        data: [transaction],
        total: 1,
        page: 1,
        page_size: 10,
      });

      await useTransactionStore.getState().fetchTransactions();

      expect(useTransactionStore.getState().transactions).toEqual([
        transaction,
      ]);
      expect(useTransactionStore.getState().total).toBe(1);
    });

    it('merges filters and refetches', async () => {
      vi.mocked(api.listTransactions).mockResolvedValue({
        data: [],
        total: 0,
        page: 1,
        page_size: 10,
      });

      useTransactionStore.getState().setFilters({ type: 'expense' });

      expect(useTransactionStore.getState().filters.type).toBe('expense');
      await vi.waitFor(() => {
        expect(api.listTransactions).toHaveBeenCalledWith(
          expect.objectContaining({ type: 'expense' })
        );
      });
    });

    it('creates a transaction and refreshes the list', async () => {
      vi.mocked(api.createTransaction).mockResolvedValue(transaction);
      vi.mocked(api.listTransactions).mockResolvedValue({
        data: [transaction],
        total: 1,
        page: 1,
        page_size: 10,
      });

      await useTransactionStore.getState().createTransaction({
        account_id: account.id,
        category_id: category.id,
        type: 'expense',
        amount: 42.5,
        description: 'Weekly groceries',
        date: '2026-10-20T00:00:00Z',
      });

      expect(api.createTransaction).toHaveBeenCalled();
      expect(api.listTransactions).toHaveBeenCalled();
      expect(useTransactionStore.getState().transactions).toEqual([
        transaction,
      ]);
    });

    it('deletes a transaction and refreshes the list', async () => {
      vi.mocked(api.deleteTransaction).mockResolvedValue(undefined);
      vi.mocked(api.listTransactions).mockResolvedValue({
        data: [],
        total: 0,
        page: 1,
        page_size: 10,
      });

      await useTransactionStore.getState().deleteTransaction(transaction.id);

      expect(api.deleteTransaction).toHaveBeenCalledWith(transaction.id);
      expect(api.listTransactions).toHaveBeenCalled();
    });
  });

  describe('dashboard store', () => {
    it('loads the summary', async () => {
      vi.mocked(api.getDashboardSummary).mockResolvedValue(summary);

      await useDashboardStore.getState().fetchSummary();

      expect(useDashboardStore.getState().summary).toEqual(summary);
    });

    it('captures a summary error', async () => {
      vi.mocked(api.getDashboardSummary).mockRejectedValue(
        new Error('summary failed')
      );

      await useDashboardStore.getState().fetchSummary();

      expect(useDashboardStore.getState().error).toBe('summary failed');
    });
  });
});
