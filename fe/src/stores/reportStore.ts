import { create } from 'zustand';
import type {
  CashflowPoint,
  CategoryTotal,
  MerchantTotal,
  ReportRange,
} from '../types';
import { api } from '../lib/api';
import { defaultReportRange } from '../lib/dates';

interface ReportState {
  from: string;
  to: string;
  cashflow: CashflowPoint[];
  categories: CategoryTotal[];
  merchants: MerchantTotal[];
  isLoading: boolean;
  error: string | null;
  setRange: (from: string, to: string) => void;
  fetchReports: (range?: ReportRange) => Promise<void>;
}

const initialRange = defaultReportRange();

export const useReportStore = create<ReportState>()((set, get) => ({
  from: initialRange.from,
  to: initialRange.to,
  cashflow: [],
  categories: [],
  merchants: [],
  isLoading: false,
  error: null,
  setRange: (from, to) => set({ from, to }),
  fetchReports: async (range) => {
    const from = range?.from ?? get().from;
    const to = range?.to ?? get().to;
    set({ from, to, isLoading: true, error: null });

    try {
      const [cashflow, categories, merchants] = await Promise.all([
        api.getCashflowReport(from, to),
        api.getCategoryReport(from, to),
        api.getMerchantReport(from, to),
      ]);
      set({ cashflow, categories, merchants, isLoading: false });
    } catch (error) {
      set({
        error:
          error instanceof Error ? error.message : 'Unable to load reports',
        isLoading: false,
      });
    }
  },
}));
