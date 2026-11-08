import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { BudgetStatus } from '../types';
import { api } from '../lib/api';
import { currentMonth } from '../lib/dates';

interface BudgetState {
  month: string;
  budgets: BudgetStatus[];
  isLoading: boolean;
  error: string | null;
  isSaving: boolean;
  saveError: string | null;
  fetchBudgets: (month?: string) => Promise<void>;
  upsertBudget: (
    categoryId: string,
    amount: number,
    currency: string
  ) => Promise<void>;
  deleteBudget: (categoryId: string, currency: string) => Promise<void>;
}

function messageOf(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

export const useBudgetStore = create<BudgetState>()(
  persist(
    (set, get) => ({
      month: currentMonth(),
      budgets: [],
      isLoading: false,
      error: null,
      isSaving: false,
      saveError: null,
      fetchBudgets: async (month) => {
        const target = month ?? get().month;
        set({ month: target, isLoading: true, error: null });

        try {
          const budgets = await api.listBudgets(target);
          set({ budgets, isLoading: false });
        } catch (error) {
          set({
            error: messageOf(error, 'Unable to load budgets'),
            isLoading: false,
          });
        }
      },
      upsertBudget: async (categoryId, amount, currency) => {
        set({ isSaving: true, saveError: null });

        try {
          await api.upsertBudget({
            category_id: categoryId,
            month: get().month,
            amount,
            currency,
          });
          await get().fetchBudgets();
          set({ isSaving: false });
        } catch (error) {
          const message = messageOf(error, 'Unable to save budget');
          set({ saveError: message, isSaving: false });
          throw new Error(message, { cause: error });
        }
      },
      deleteBudget: async (categoryId, currency) => {
        set({ isSaving: true, saveError: null });

        try {
          await api.deleteBudget(categoryId, get().month, currency);
          await get().fetchBudgets();
          set({ isSaving: false });
        } catch (error) {
          const message = messageOf(error, 'Unable to remove budget');
          set({ saveError: message, isSaving: false });
          throw new Error(message, { cause: error });
        }
      },
    }),
    {
      name: 'budget-storage',
      partialize: (state) => ({ month: state.month }),
    }
  )
);
