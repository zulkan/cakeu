import type {
  Account,
  Budget,
  BudgetStatus,
  CashflowPoint,
  Category,
  CategoryTotal,
  CreateAccountRequest,
  CreateCategoryRequest,
  CreateRecurringRequest,
  CreateTransactionRequest,
  DashboardSummary,
  ImportResult,
  MerchantTotal,
  PaginatedTransactions,
  RecurringRule,
  RunRecurringResult,
  Transaction,
  TransactionFilters,
  UpdateAccountRequest,
  UpdateRecurringRequest,
  UpsertBudgetRequest,
} from '../types';
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

function authHeaders(): Record<string, string> {
  const token = getToken();
  return token ? { Authorization: `Bearer ${token}` } : {};
}

async function errorMessage(res: Response): Promise<string> {
  const body = (await res
    .json()
    .catch(() => ({ error: res.statusText }))) as { error?: string };
  return body.error || `Request failed: ${res.status}`;
}

function handleUnauthorized(res: Response): void {
  if (res.status === 401) {
    useAuthStore.getState().logout();
  }
}

function buildFilterQuery(filters?: TransactionFilters): string {
  const params = new URLSearchParams();
  if (filters?.account_id) params.set('account_id', filters.account_id);
  if (filters?.category_id) params.set('category_id', filters.category_id);
  if (filters?.type) params.set('type', filters.type);
  if (filters?.date_from) params.set('date_from', filters.date_from);
  if (filters?.date_to) params.set('date_to', filters.date_to);
  const qs = params.toString();
  return qs ? `?${qs}` : '';
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...authHeaders(),
    ...(options.headers as Record<string, string>),
  };

  const res = await fetch(apiUrl(path), { ...options, headers });

  handleUnauthorized(res);

  if (!res.ok) {
    throw new Error(await errorMessage(res));
  }

  if (res.status === 204) {
    return undefined as T;
  }

  return res.json() as Promise<T>;
}

async function requestMultipart<T>(path: string, body: FormData): Promise<T> {
  // Deliberately no Content-Type: the browser adds the multipart boundary.
  const res = await fetch(apiUrl(path), {
    method: 'POST',
    body,
    headers: authHeaders(),
  });

  handleUnauthorized(res);

  if (!res.ok) {
    throw new Error(await errorMessage(res));
  }

  return res.json() as Promise<T>;
}

function reportQuery(from: string, to: string, limit?: number): string {
  const params = new URLSearchParams({ from, to });
  if (limit !== undefined) params.set('limit', String(limit));
  return `?${params.toString()}`;
}

function importForm(file: File, confirm: boolean, sheet?: string): FormData {
  const form = new FormData();
  form.append('file', file);
  if (confirm) form.append('confirm', 'true');
  if (sheet) form.append('sheet', sheet);
  return form;
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

  exportTransactionsCsv: async (
    filters?: TransactionFilters
  ): Promise<Blob> => {
    const res = await fetch(
      apiUrl(`/transactions/export.csv${buildFilterQuery(filters)}`),
      { headers: authHeaders() }
    );

    handleUnauthorized(res);

    if (!res.ok) {
      throw new Error(await errorMessage(res));
    }

    return res.blob();
  },
  previewImport: (file: File, sheet?: string) =>
    requestMultipart<ImportResult>(
      '/transactions/import/preview',
      importForm(file, false, sheet)
    ),
  importTransactions: (file: File, sheet?: string) =>
    requestMultipart<ImportResult>(
      '/transactions/import',
      importForm(file, true, sheet)
    ),

  // Budgets
  listBudgets: (month: string) => {
    const params = new URLSearchParams({ month });
    return request<BudgetStatus[]>(`/budgets?${params.toString()}`);
  },
  upsertBudget: (data: UpsertBudgetRequest) =>
    request<Budget>('/budgets', {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  deleteBudget: (categoryId: string, month: string) => {
    const params = new URLSearchParams({ month });
    return request<void>(
      `/budgets/${categoryId}?${params.toString()}`,
      { method: 'DELETE' }
    );
  },

  // Recurring
  listRecurring: () => request<RecurringRule[]>('/recurring'),
  createRecurring: (data: CreateRecurringRequest) =>
    request<RecurringRule>('/recurring', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  updateRecurring: (id: string, data: UpdateRecurringRequest) =>
    request<RecurringRule>(`/recurring/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),
  deleteRecurring: (id: string) =>
    request<void>(`/recurring/${id}`, { method: 'DELETE' }),
  runRecurringNow: (id: string) =>
    request<RunRecurringResult>(`/recurring/${id}/run-now`, {
      method: 'POST',
    }),

  // Reports
  getCashflowReport: (from: string, to: string) =>
    request<CashflowPoint[]>(`/reports/cashflow${reportQuery(from, to)}`),
  getCategoryReport: (from: string, to: string) =>
    request<CategoryTotal[]>(`/reports/categories${reportQuery(from, to)}`),
  getMerchantReport: (from: string, to: string, limit = 10) =>
    request<MerchantTotal[]>(
      `/reports/merchants${reportQuery(from, to, limit)}`
    ),

  // Dashboard
  getDashboardSummary: () => request<DashboardSummary>('/dashboard/summary'),
};
