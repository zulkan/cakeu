import { create } from 'zustand';
import type {
  Account,
  AdjustAccountRequest,
  CreateAccountRequest,
  UpdateAccountRequest,
} from '../types';
import { api } from '../lib/api';

interface AccountState {
  accounts: Account[];
  isLoading: boolean;
  error: string | null;
  fetchAccounts: () => Promise<void>;
  createAccount: (data: CreateAccountRequest) => Promise<Account>;
  updateAccount: (id: string, data: UpdateAccountRequest) => Promise<void>;
  adjustBalance: (id: string, data: AdjustAccountRequest) => Promise<void>;
  deleteAccount: (id: string) => Promise<void>;
}

export const useAccountStore = create<AccountState>()((set) => ({
  accounts: [],
  isLoading: false,
  error: null,
  fetchAccounts: async () => {
    set({ isLoading: true, error: null });
    try {
      const accounts = await api.listAccounts();
      set({ accounts, isLoading: false });
    } catch (e) {
      set({ error: (e as Error).message, isLoading: false });
    }
  },
  createAccount: async (data) => {
    const account = await api.createAccount(data);
    set((s) => ({ accounts: [...s.accounts, account] }));
    return account;
  },
  updateAccount: async (id, data) => {
    const updated = await api.updateAccount(id, data);
    set((s) => ({
      accounts: s.accounts.map((a) => (a.id === id ? updated : a)),
    }));
  },
  adjustBalance: async (id, data) => {
    const updated = await api.adjustAccountBalance(id, data);
    set((s) => ({
      accounts: s.accounts.map((a) => (a.id === id ? updated : a)),
    }));
  },
  deleteAccount: async (id) => {
    await api.deleteAccount(id);
    set((s) => ({ accounts: s.accounts.filter((a) => a.id !== id) }));
  },
}));
