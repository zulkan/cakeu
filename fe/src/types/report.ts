export interface CashflowPoint {
  currency: string;
  period: string;
  income: number;
  expense: number;
  net: number;
}

export interface CategoryTotal {
  currency: string;
  category_id: string;
  category_name: string;
  type: 'income' | 'expense';
  total: number;
  share: number;
}

export interface MerchantTotal {
  currency: string;
  description: string;
  total: number;
}

export interface ReportRange {
  from: string;
  to: string;
}
