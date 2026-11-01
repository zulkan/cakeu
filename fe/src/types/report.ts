export interface CashflowPoint {
  period: string;
  income: number;
  expense: number;
  net: number;
}

export interface CategoryTotal {
  category_id: string;
  category_name: string;
  type: 'income' | 'expense';
  total: number;
  share: number;
}

export interface MerchantTotal {
  description: string;
  total: number;
}

export interface ReportRange {
  from: string;
  to: string;
}
