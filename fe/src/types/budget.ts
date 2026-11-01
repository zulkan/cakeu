export interface Budget {
  id: string;
  user_id: string;
  category_id: string;
  category_name: string;
  month: string;
  amount: number;
  created_at: string;
  updated_at: string;
}

export interface BudgetStatus {
  category_id: string;
  category_name: string;
  budget: number;
  spent: number;
  remaining: number;
  percent: number;
}

export interface UpsertBudgetRequest {
  category_id: string;
  month: string;
  amount: number;
}
