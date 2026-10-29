import { create } from 'zustand';
import type { DashboardSummary } from '../types';
import { api } from '../lib/api';

interface DashboardState {
  summary: DashboardSummary | null;
  isLoading: boolean;
  error: string | null;
  fetchSummary: () => Promise<void>;
}

export const useDashboardStore = create<DashboardState>()((set) => ({
  summary: null,
  isLoading: false,
  error: null,
  fetchSummary: async () => {
    set({ isLoading: true, error: null });
    try {
      const summary = await api.getDashboardSummary();
      set({ summary, isLoading: false });
    } catch (e) {
      set({ error: (e as Error).message, isLoading: false });
    }
  },
}));
