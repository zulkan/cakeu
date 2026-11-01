export type RecurringFrequency = 'daily' | 'weekly' | 'monthly';

export interface RecurringRule {
  id: string;
  user_id: string;
  account_id: string;
  category_id: string;
  type: 'income' | 'expense';
  amount: number;
  description: string;
  frequency: RecurringFrequency;
  interval: number;
  next_run: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateRecurringRequest {
  account_id: string;
  category_id: string;
  type: 'income' | 'expense';
  amount: number;
  description: string;
  frequency: RecurringFrequency;
  interval: number;
  next_run: string;
}

export interface UpdateRecurringRequest {
  amount?: number;
  description?: string;
  frequency?: RecurringFrequency;
  interval?: number;
  next_run?: string;
  active?: boolean;
}

export interface RunRecurringResult {
  posted: number;
}
