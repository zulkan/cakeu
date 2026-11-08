import type {
  Account,
  AdjustAccountRequest,
  Attachment,
  Budget,
  BudgetStatus,
  CashflowPoint,
  Category,
  CategoryTotal,
  CreateAccountRequest,
  CreateCategoryRequest,
  CreateRecurringRequest,
  CreateTransactionRequest,
  CreateTransferRequest,
  DashboardSummary,
  ImportMapping,
  ImportMode,
  ImportResult,
  MerchantTotal,
  PaginatedTransactions,
  RecurringRule,
  RunRecurringResult,
  Transaction,
  TransactionFilters,
  TransferResult,
  UpdateAccountRequest,
  UpdateRecurringRequest,
  UpdateTransactionRequest,
  UpsertBudgetRequest,
} from '../types';
import { useAuthStore } from '../stores/authStore';
import { apiUrl } from './apiBase';
import { DEFAULT_TIMEOUT_MS, abortableScope } from './http';

export function getToken(): string | null {
  return useAuthStore.getState().token ?? null;
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
  const scope = abortableScope(DEFAULT_TIMEOUT_MS, options.signal);

  try {
    const res = await fetch(apiUrl(path), {
      ...options,
      headers,
      signal: scope.signal,
    });

    handleUnauthorized(res);

    if (!res.ok) {
      throw new Error(await errorMessage(res));
    }

    if (res.status === 204) {
      return undefined as T;
    }

    return res.json() as Promise<T>;
  } finally {
    scope.dispose();
  }
}

async function requestMultipart<T>(
  path: string,
  body: FormData,
  signal?: AbortSignal
): Promise<T> {
  // Deliberately no Content-Type: the browser adds the multipart boundary.
  const scope = abortableScope(DEFAULT_TIMEOUT_MS, signal);

  try {
    const res = await fetch(apiUrl(path), {
      method: 'POST',
      body,
      headers: authHeaders(),
      signal: scope.signal,
    });

    handleUnauthorized(res);

    if (!res.ok) {
      throw new Error(await errorMessage(res));
    }

    return res.json() as Promise<T>;
  } finally {
    scope.dispose();
  }
}

function reportQuery(from: string, to: string, limit?: number): string {
  const params = new URLSearchParams({ from, to });
  if (limit !== undefined) params.set('limit', String(limit));
  return `?${params.toString()}`;
}

interface ImportOptions {
  sheet?: string;
  mode?: ImportMode;
  mapping?: ImportMapping;
  allow_duplicates?: boolean;
}

function importForm(file: File, confirm: boolean, opts: ImportOptions = {}): FormData {
  const form = new FormData();
  form.append('file', file);
  if (confirm) form.append('confirm', 'true');
  if (opts.sheet) form.append('sheet', opts.sheet);
  if (opts.mode) form.append('mode', opts.mode);
  if (opts.mapping) form.append('mapping', JSON.stringify(opts.mapping));
  if (opts.allow_duplicates) form.append('allow_duplicates', 'true');
  return form;
}


export const api = {
  // Accounts
  listAccounts: (signal?: AbortSignal) =>
    request<Account[]>('/accounts', { signal }),
  createAccount: (data: CreateAccountRequest, signal?: AbortSignal) =>
    request<Account>('/accounts', {
      method: 'POST',
      body: JSON.stringify(data),
      signal,
    }),
  updateAccount: (
    id: string,
    data: UpdateAccountRequest,
    signal?: AbortSignal
  ) =>
    request<Account>(`/accounts/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
      signal,
    }),
  adjustAccountBalance: (
    id: string,
    data: AdjustAccountRequest,
    signal?: AbortSignal
  ) =>
    request<Account>(`/accounts/${id}/adjustments`, {
      method: 'POST',
      body: JSON.stringify(data),
      signal,
    }),
  deleteAccount: (id: string, signal?: AbortSignal) =>
    request<void>(`/accounts/${id}`, { method: 'DELETE', signal }),

  // Categories
  listCategories: (signal?: AbortSignal) =>
    request<Category[]>('/categories', { signal }),
  createCategory: (data: CreateCategoryRequest, signal?: AbortSignal) =>
    request<Category>('/categories', {
      method: 'POST',
      body: JSON.stringify(data),
      signal,
    }),
  deleteCategory: (id: string, signal?: AbortSignal) =>
    request<void>(`/categories/${id}`, { method: 'DELETE', signal }),

  // Transactions
  listTransactions: (filters?: TransactionFilters, signal?: AbortSignal) => {
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
      `/transactions${qs ? `?${qs}` : ''}`,
      { signal }
    );
  },
  createTransaction: (data: CreateTransactionRequest, signal?: AbortSignal) =>
    request<Transaction>('/transactions', {
      method: 'POST',
      body: JSON.stringify(data),
      signal,
    }),
  updateTransaction: (
    id: string,
    data: UpdateTransactionRequest,
    signal?: AbortSignal
  ) =>
    request<Transaction>(`/transactions/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
      signal,
    }),
  deleteTransaction: (id: string, signal?: AbortSignal) =>
    request<void>(`/transactions/${id}`, { method: 'DELETE', signal }),

  // Attachments
  listAttachments: (transactionId: string, signal?: AbortSignal) =>
    request<Attachment[]>(`/transactions/${transactionId}/attachments`, {
      signal,
    }),
  uploadAttachment: (
    transactionId: string,
    file: File,
    signal?: AbortSignal
  ) => {
    const form = new FormData();
    form.append('file', file);
    return requestMultipart<Attachment>(
      `/transactions/${transactionId}/attachments`,
      form,
      signal
    );
  },
  deleteAttachment: (id: string, signal?: AbortSignal) =>
    request<void>(`/attachments/${id}`, { method: 'DELETE', signal }),
  downloadAttachment: async (
    id: string,
    signal?: AbortSignal
  ): Promise<Blob> => {
    const scope = abortableScope(DEFAULT_TIMEOUT_MS, signal);

    try {
      const res = await fetch(apiUrl(`/attachments/${id}`), {
        headers: authHeaders(),
        signal: scope.signal,
      });

      handleUnauthorized(res);

      if (!res.ok) {
        throw new Error(await errorMessage(res));
      }

      return await res.blob();
    } finally {
      scope.dispose();
    }
  },

  // Transfers
  createTransfer: (data: CreateTransferRequest, signal?: AbortSignal) =>
    request<TransferResult>('/transfers', {
      method: 'POST',
      body: JSON.stringify(data),
      signal,
    }),
  deleteTransfer: (groupId: string, signal?: AbortSignal) =>
    request<void>(`/transfers/${groupId}`, { method: 'DELETE', signal }),

  exportTransactionsCsv: async (
    filters?: TransactionFilters,
    signal?: AbortSignal
  ): Promise<Blob> => {
    const scope = abortableScope(DEFAULT_TIMEOUT_MS, signal);

    try {
      const res = await fetch(
        apiUrl(`/transactions/export.csv${buildFilterQuery(filters)}`),
        { headers: authHeaders(), signal: scope.signal }
      );

      handleUnauthorized(res);

      if (!res.ok) {
        throw new Error(await errorMessage(res));
      }

      return await res.blob();
    } finally {
      scope.dispose();
    }
  },
  previewImport: (file: File, opts: ImportOptions = {}, signal?: AbortSignal) =>
    requestMultipart<ImportResult>(
      '/transactions/import/preview',
      importForm(file, false, opts),
      signal
    ),
  importTransactions: (
    file: File,
    opts: ImportOptions = {},
    signal?: AbortSignal
  ) =>
    requestMultipart<ImportResult>(
      '/transactions/import',
      importForm(file, true, opts),
      signal
    ),

  // Budgets
  listBudgets: (month: string, signal?: AbortSignal) => {
    const params = new URLSearchParams({ month });
    return request<BudgetStatus[]>(`/budgets?${params.toString()}`, { signal });
  },
  upsertBudget: (data: UpsertBudgetRequest, signal?: AbortSignal) =>
    request<Budget>('/budgets', {
      method: 'PUT',
      body: JSON.stringify(data),
      signal,
    }),
  deleteBudget: (
    categoryId: string,
    month: string,
    currency: string,
    signal?: AbortSignal
  ) => {
    const params = new URLSearchParams({ month, currency });
    return request<void>(`/budgets/${categoryId}?${params.toString()}`, {
      method: 'DELETE',
      signal,
    });
  },

  // Recurring
  listRecurring: (signal?: AbortSignal) =>
    request<RecurringRule[]>('/recurring', { signal }),
  createRecurring: (data: CreateRecurringRequest, signal?: AbortSignal) =>
    request<RecurringRule>('/recurring', {
      method: 'POST',
      body: JSON.stringify(data),
      signal,
    }),
  updateRecurring: (
    id: string,
    data: UpdateRecurringRequest,
    signal?: AbortSignal
  ) =>
    request<RecurringRule>(`/recurring/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
      signal,
    }),
  deleteRecurring: (id: string, signal?: AbortSignal) =>
    request<void>(`/recurring/${id}`, { method: 'DELETE', signal }),
  runRecurringNow: (id: string, signal?: AbortSignal) =>
    request<RunRecurringResult>(`/recurring/${id}/run-now`, {
      method: 'POST',
      signal,
    }),

  // Reports
  getCashflowReport: (from: string, to: string, signal?: AbortSignal) =>
    request<CashflowPoint[]>(`/reports/cashflow${reportQuery(from, to)}`, {
      signal,
    }),
  getCategoryReport: (from: string, to: string, signal?: AbortSignal) =>
    request<CategoryTotal[]>(`/reports/categories${reportQuery(from, to)}`, {
      signal,
    }),
  getMerchantReport: (
    from: string,
    to: string,
    limit = 10,
    signal?: AbortSignal
  ) =>
    request<MerchantTotal[]>(
      `/reports/merchants${reportQuery(from, to, limit)}`,
      { signal }
    ),

  // Dashboard
  getDashboardSummary: (signal?: AbortSignal) =>
    request<DashboardSummary>('/dashboard/summary', { signal }),
};
