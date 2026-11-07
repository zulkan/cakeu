import { formatCurrency } from '../../lib/format';
import { Card } from '../ui/Card';

interface MonthlyBarsProps {
  income: number;
  expense: number;
  currency: string;
}

export function MonthlyBars({ income, expense, currency }: MonthlyBarsProps) {
  const max = Math.max(income, expense, 1);
  const incomeHeight = income > 0 ? Math.max((income / max) * 100, 8) : 0;
  const expenseHeight = expense > 0 ? Math.max((expense / max) * 100, 8) : 0;

  return (
    <Card className="p-5">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-base font-bold text-stone-900">This month</h2>
          <p className="text-xs text-stone-500">Income vs. expense</p>
        </div>
        <span className="rounded-full bg-warm-100 px-2.5 py-1 text-xs font-semibold text-warm-800">
          {currency}
        </span>
      </div>

      <div
        className="mt-6 flex h-40 items-end justify-center gap-8"
        role="img"
        aria-label={`Income ${formatCurrency(income, currency)} versus expense ${formatCurrency(expense, currency)}`}
      >
        <div className="flex h-full w-16 flex-col items-center justify-end gap-2">
          <span className="text-xs font-bold text-income">
            {formatCurrency(income, currency)}
          </span>
          <div
            className="w-full rounded-t-xl bg-gradient-to-t from-income to-emerald-400"
            style={{ height: `${incomeHeight}%` }}
          />
          <span className="text-xs font-semibold text-stone-500">Income</span>
        </div>

        <div className="flex h-full w-16 flex-col items-center justify-end gap-2">
          <span className="text-xs font-bold text-expense">
            {formatCurrency(expense, currency)}
          </span>
          <div
            className="w-full rounded-t-xl bg-gradient-to-t from-expense to-rose-400"
            style={{ height: `${expenseHeight}%` }}
          />
          <span className="text-xs font-semibold text-stone-500">Expense</span>
        </div>
      </div>
    </Card>
  );
}
