import { create } from 'zustand';
import type {
  CreateTransactionRequest,
  Transaction,
  TransactionFilters,
} from '../types';
import { api } from '../lib/api';

interface TransactionState {
  transactions: Transaction[];
  total: number;
  isLoading: boolean;
  error: string | null;
  filters: TransactionFilters;
  setFilters: (filters: TransactionFilters) => void;
  fetchTransactions: (filters?: TransactionFilters) => Promise<void>;
  createTransaction: (data: CreateTransactionRequest) => Promise<void>;
  deleteTransaction: (id: string) => Promise<void>;
}

export const useTransactionStore = create<TransactionState>()((set, get) => ({
  transactions: [],
  total: 0,
  isLoading: false,
  error: null,
  filters: { page: 1, page_size: 10 },
  setFilters: (filters) => {
    set((s) => ({ filters: { ...s.filters, ...filters } }));
    void get().fetchTransactions();
  },
  fetchTransactions: async (filters) => {
    const merged = { ...get().filters, ...filters };
    set({ isLoading: true, error: null, filters: merged });
    try {
      const result = await api.listTransactions(merged);
      set({
        transactions: result.data,
        total: result.total,
        isLoading: false,
      });
    } catch (e) {
      set({ error: (e as Error).message, isLoading: false });
    }
  },
  createTransaction: async (data) => {
    await api.createTransaction(data);
    await get().fetchTransactions();
  },
  deleteTransaction: async (id) => {
    await api.deleteTransaction(id);
    await get().fetchTransactions();
  },
}));
