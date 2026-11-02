export interface Account {
  id: string;
  name: string;
  balance: number;
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
}

export interface DashboardSummary {
  total_balance: number;
  monthly_income: number;
  monthly_expense: number;
  monthly_net: number;
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
  balance?: number;
  currency?: string;
}

export interface UpdateAccountRequest {
  name?: string;
  balance?: number;
  currency?: string;
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

export interface ImportResult {
  total: number;
  valid: number;
  invalid: number;
  sheet: string;
  sheets: string[];
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
