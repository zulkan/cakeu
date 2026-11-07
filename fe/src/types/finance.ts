export interface Account {
  id: string;
  name: string;
  balance: number;
  opening_balance: number;
  entry_count: number;
  currency: string;
  created_at: string;
  updated_at: string;
}

export interface Category {
  id: string;
  name: string;
  type: 'income' | 'expense';
  color: string;
  created_at: string;
}

export type TransactionKind = 'normal' | 'opening' | 'adjustment';

export interface Transaction {
  id: string;
  account_id: string;
  category_id: string;
  type: 'income' | 'expense';
  amount: number;
  description: string;
  date: string;
  created_at: string;
  account_name: string;
  category_name: string;
  kind: TransactionKind;
  currency: string;
}

export interface CurrencyTotal {
  currency: string;
  total: number;
}

export interface MonthlyCurrencyTotal {
  currency: string;
  income: number;
  expense: number;
  net: number;
}

export interface DashboardSummary {
  balances: CurrencyTotal[];
  monthly: MonthlyCurrencyTotal[];
  transaction_count: number;
  recent_transactions: Transaction[];
}

export interface PaginatedTransactions {
  data: Transaction[];
  total: number;
  page: number;
  page_size: number;
}

export interface TransactionFilters {
  account_id?: string;
  category_id?: string;
  type?: 'income' | 'expense';
  date_from?: string;
  date_to?: string;
  page?: number;
  page_size?: number;
}

export interface CreateAccountRequest {
  name: string;
  opening_balance?: number;
  currency?: string;
}

export interface UpdateAccountRequest {
  name?: string;
  currency?: string;
}

export interface AdjustAccountRequest {
  target_balance: number;
  description?: string;
}

export interface CreateCategoryRequest {
  name: string;
  type: 'income' | 'expense';
  color?: string;
}

export interface ImportRowError {
  row: number;
  message: string;
}

export interface ImportPreviewRow {
  row: number;
  date: string;
  type: 'income' | 'expense';
  amount: number;
  description: string;
  account: string;
  category: string;
}

export type ImportMode = 'ai' | 'strict';

export interface ImportFieldRule {
  column?: number;
  constant?: string;
  carry_forward?: boolean;
  date_format?: string;
  amount_format?: string;
  amount_mode?: string;
  value_map?: Record<string, string>;
}

export interface ImportTypeRule {
  column?: number;
  constant?: 'income' | 'expense';
  value_map?: Record<string, string>;
}

export interface ImportFields {
  date: ImportFieldRule;
  amount: ImportFieldRule;
  description: ImportFieldRule;
  account: ImportFieldRule;
  category: ImportFieldRule;
}

export interface ImportMapping {
  sheet: string;
  header_row: number;
  data_start: number;
  data_end: number;
  skip_rows?: number[];
  fields: ImportFields;
  type: ImportTypeRule;
  confidence?: number;
  notes?: string;
}

export interface ImportResult {
  total: number;
  valid: number;
  invalid: number;
  skipped: number;
  sheet: string;
  sheets: string[];
  mode?: ImportMode;
  mapping?: ImportMapping;
  errors: ImportRowError[];
  preview: ImportPreviewRow[];
}

export interface CreateTransactionRequest {
  account_id: string;
  category_id: string;
  type: 'income' | 'expense';
  amount: number;
  description: string;
  date: string;
}

export type UpdateTransactionRequest = Partial<CreateTransactionRequest>;
