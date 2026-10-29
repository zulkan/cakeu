import type {
  Account,
  Category,
  CreateAccountRequest,
  CreateCategoryRequest,
  CreateTransactionRequest,
  DashboardSummary,
  PaginatedTransactions,
  Transaction,
  TransactionFilters,
  UpdateAccountRequest,
} from '../types/finance';
import { useAuthStore } from '../stores/authStore';
import { apiUrl } from './apiBase';

function getToken(): string | null {
  try {
    const stored = localStorage.getItem('auth-storage');
    if (!stored) return null;
    const parsed = JSON.parse(stored) as {
      state?: { token?: string | null };
    };
    return parsed?.state?.token ?? null;
  } catch {
    return null;
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(apiUrl(path), { ...options, headers });

  if (res.status === 401) {
    useAuthStore.getState().logout();
  }

  if (!res.ok) {
    const body = (await res
      .json()
      .catch(() => ({ error: res.statusText }))) as { error?: string };
    throw new Error(body.error || `Request failed: ${res.status}`);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  return res.json() as Promise<T>;
}

export const api = {
  // Accounts
  listAccounts: () => request<Account[]>('/accounts'),
  createAccount: (data: CreateAccountRequest) =>
    request<Account>('/accounts', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  updateAccount: (id: string, data: UpdateAccountRequest) =>
    request<Account>(`/accounts/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),
  deleteAccount: (id: string) =>
    request<void>(`/accounts/${id}`, { method: 'DELETE' }),

  // Categories
  listCategories: () => request<Category[]>('/categories'),
  createCategory: (data: CreateCategoryRequest) =>
    request<Category>('/categories', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  deleteCategory: (id: string) =>
    request<void>(`/categories/${id}`, { method: 'DELETE' }),

  // Transactions
  listTransactions: (filters?: TransactionFilters) => {
    const params = new URLSearchParams();
    if (filters?.account_id) params.set('account_id', filters.account_id);
    if (filters?.category_id) params.set('category_id', filters.category_id);
    if (filters?.type) params.set('type', filters.type);
    if (filters?.date_from) params.set('date_from', filters.date_from);
    if (filters?.date_to) params.set('date_to', filters.date_to);
    if (filters?.page) params.set('page', String(filters.page));
    if (filters?.page_size) params.set('page_size', String(filters.page_size));
    const qs = params.toString();
    return request<PaginatedTransactions>(
      `/transactions${qs ? `?${qs}` : ''}`
    );
  },
  createTransaction: (data: CreateTransactionRequest) =>
    request<Transaction>('/transactions', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  deleteTransaction: (id: string) =>
    request<void>(`/transactions/${id}`, { method: 'DELETE' }),

  // Dashboard
  getDashboardSummary: () => request<DashboardSummary>('/dashboard/summary'),
};
