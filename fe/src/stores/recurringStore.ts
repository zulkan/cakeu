import { create } from 'zustand';
import type {
  CreateRecurringRequest,
  RecurringRule,
  UpdateRecurringRequest,
} from '../types';
import { api } from '../lib/api';

interface RecurringState {
  rules: RecurringRule[];
  isLoading: boolean;
  error: string | null;
  fetchRules: () => Promise<void>;
  createRule: (data: CreateRecurringRequest) => Promise<RecurringRule>;
  updateRule: (
    id: string,
    data: UpdateRecurringRequest
  ) => Promise<RecurringRule>;
  deleteRule: (id: string) => Promise<void>;
  runNow: (id: string) => Promise<number>;
}

export const useRecurringStore = create<RecurringState>()((set, get) => ({
  rules: [],
  isLoading: false,
  error: null,
  fetchRules: async () => {
    set({ isLoading: true, error: null });

    try {
      const rules = await api.listRecurring();
      set({ rules, isLoading: false });
    } catch (error) {
      set({
        error:
          error instanceof Error ? error.message : 'Unable to load recurring rules',
        isLoading: false,
      });
    }
  },
  createRule: async (data) => {
    const rule = await api.createRecurring(data);
    set((state) => ({ rules: [...state.rules, rule] }));
    return rule;
  },
  updateRule: async (id, data) => {
    const rule = await api.updateRecurring(id, data);
    set((state) => ({
      rules: state.rules.map((existing) => (existing.id === id ? rule : existing)),
    }));
    return rule;
  },
  deleteRule: async (id) => {
    await api.deleteRecurring(id);
    set((state) => ({ rules: state.rules.filter((rule) => rule.id !== id) }));
  },
  runNow: async (id) => {
    const result = await api.runRecurringNow(id);
    await get().fetchRules();
    return result.posted;
  },
}));
