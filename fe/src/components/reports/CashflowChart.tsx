import type { CashflowPoint } from '../../types';
import { formatCurrency } from '../../lib/format';
import { formatMonthLabel, formatShortMonth } from '../../lib/dates';
import { cn } from '../../lib/cn';
import { Card } from '../ui/Card';

interface CashflowChartProps {
  points: CashflowPoint[];
  currency: string;
}

export function CashflowChart({ points, currency }: CashflowChartProps) {
  if (points.length === 0) {
    return (
      <Card className="p-5">
        <h2 className="text-base font-bold text-stone-900">
          Cashflow by month
        </h2>
        <p className="mt-4 rounded-xl border border-dashed border-warm-300 bg-warm-50/60 px-4 py-10 text-center text-sm text-stone-500">
          No cashflow recorded in this range.
        </p>
      </Card>
    );
  }

  const max = Math.max(
    ...points.map((point) => Math.max(point.income, point.expense)),
    1
  );

  const barHeight = (value: number): string => {
    if (value <= 0) return '0%';
    return `${Math.max((value / max) * 100, 4)}%`;
  };

  return (
    <Card className="p-5">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 className="text-base font-bold text-stone-900">
            Cashflow by month
          </h2>
          <p className="text-xs text-stone-500">
            Income, expense, and net for each month
          </p>
        </div>
        <div className="flex items-center gap-4 text-xs font-semibold text-stone-500">
          <span className="flex items-center gap-1.5">
            <span
              className="h-2.5 w-2.5 rounded-full bg-income"
              aria-hidden="true"
            />
            Income
          </span>
          <span className="flex items-center gap-1.5">
            <span
              className="h-2.5 w-2.5 rounded-full bg-expense"
              aria-hidden="true"
            />
            Expense
          </span>
        </div>
      </div>

      <div className="mt-6 overflow-x-auto pb-1">
        <div className="flex min-w-[32rem] items-end gap-3">
          {points.map((point) => (
            <div
              key={point.period}
              role="group"
              aria-label={`${formatMonthLabel(point.period)}: income ${formatCurrency(
                point.income,
                currency
              )}, expense ${formatCurrency(
                point.expense,
                currency
              )}, net ${formatCurrency(point.net, currency)}`}
              className="flex min-w-14 flex-1 flex-col items-center gap-2"
            >
              <div className="flex h-44 w-full items-end justify-center gap-1.5 border-b border-warm-200">
                <div
                  className="w-3.5 rounded-t-md bg-gradient-to-t from-income to-emerald-400 sm:w-4"
                  style={{ height: barHeight(point.income) }}
                  title={`Income ${formatCurrency(point.income, currency)}`}
                  aria-hidden="true"
                />
                <div
                  className="w-3.5 rounded-t-md bg-gradient-to-t from-expense to-rose-400 sm:w-4"
                  style={{ height: barHeight(point.expense) }}
                  title={`Expense ${formatCurrency(point.expense, currency)}`}
                  aria-hidden="true"
                />
              </div>
              <p className="text-xs font-semibold text-stone-500">
                {formatShortMonth(point.period)}
              </p>
              <p
                className={cn(
                  'text-xs font-bold',
                  point.net >= 0 ? 'text-income' : 'text-expense'
                )}
              >
                {formatCurrency(point.net, currency)}
              </p>
            </div>
          ))}
        </div>
      </div>
    </Card>
  );
}
