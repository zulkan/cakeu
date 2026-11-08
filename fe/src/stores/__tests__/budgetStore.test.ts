import { describe, it, expect, beforeEach, vi } from 'vitest';
import { api } from '../../lib/api';
import { useBudgetStore } from '../budgetStore';
import type { Budget, BudgetStatus } from '../../types';

vi.mock('../../lib/api', () => ({
  api: {
    listBudgets: vi.fn(),
    upsertBudget: vi.fn(),
    deleteBudget: vi.fn(),
  },
}));

const status: BudgetStatus = {
  currency: 'USD',
  category_id: 'category-1',
  category_name: 'Groceries',
  budget: 200,
  spent: 120,
  remaining: 80,
  percent: 60,
};

const savedBudget: Budget = {
  id: 'budget-1',
  user_id: 'user-1',
  category_id: 'category-1',
  category_name: 'Groceries',
  month: '2026-11-01T00:00:00Z',
  amount: 200,
  currency: 'USD',
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-01T00:00:00Z',
};

describe('budget store', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useBudgetStore.setState({
      month: '2026-11',
      budgets: [],
      isLoading: false,
      error: null,
      isSaving: false,
      saveError: null,
    });
  });

  it('loads the status list for the requested month', async () => {
    vi.mocked(api.listBudgets).mockResolvedValue([status]);

    await useBudgetStore.getState().fetchBudgets('2026-10');

    expect(api.listBudgets).toHaveBeenCalledWith('2026-10');
    expect(useBudgetStore.getState().month).toBe('2026-10');
    expect(useBudgetStore.getState().budgets).toEqual([status]);
    expect(useBudgetStore.getState().isLoading).toBe(false);
  });

  it('persists the selected month under budget-storage', async () => {
    vi.mocked(api.listBudgets).mockResolvedValue([]);

    await useBudgetStore.getState().fetchBudgets('2026-12');

    const stored = JSON.parse(
      localStorage.getItem('budget-storage') ?? '{}'
    ) as { state?: { month?: string } };
    expect(stored.state?.month).toBe('2026-12');
  });

  it('captures a load error', async () => {
    vi.mocked(api.listBudgets).mockRejectedValue(new Error('boom'));

    await useBudgetStore.getState().fetchBudgets();

    expect(useBudgetStore.getState().error).toBe('boom');
    expect(useBudgetStore.getState().isLoading).toBe(false);
  });

  it('saves a budget and refreshes the status list', async () => {
    vi.mocked(api.upsertBudget).mockResolvedValue(savedBudget);
    vi.mocked(api.listBudgets).mockResolvedValue([status]);

    await useBudgetStore.getState().upsertBudget('category-1', 200, 'USD');

    expect(api.upsertBudget).toHaveBeenCalledWith({
      category_id: 'category-1',
      month: '2026-11',
      amount: 200,
      currency: 'USD',
    });
    expect(api.listBudgets).toHaveBeenCalledWith('2026-11');
    expect(useBudgetStore.getState().budgets).toEqual([status]);
    expect(useBudgetStore.getState().isSaving).toBe(false);
  });

  it('surfaces a save failure without a stuck spinner', async () => {
    vi.mocked(api.upsertBudget).mockRejectedValue(
      new Error('amount must be greater than zero')
    );

    await expect(
      useBudgetStore.getState().upsertBudget('category-1', 0, 'USD')
    ).rejects.toThrow('amount must be greater than zero');

    expect(useBudgetStore.getState().saveError).toBe(
      'amount must be greater than zero'
    );
    expect(useBudgetStore.getState().isSaving).toBe(false);
  });

  it('removes a budget and refreshes the status list', async () => {
    useBudgetStore.setState({ budgets: [status] });
    vi.mocked(api.deleteBudget).mockResolvedValue(undefined);
    vi.mocked(api.listBudgets).mockResolvedValue([]);

    await useBudgetStore.getState().deleteBudget('category-1', 'USD');

    expect(api.deleteBudget).toHaveBeenCalledWith(
      'category-1',
      '2026-11',
      'USD'
    );
    expect(useBudgetStore.getState().budgets).toEqual([]);
  });
});
