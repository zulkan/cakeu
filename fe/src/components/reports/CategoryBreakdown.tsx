import type { CategoryTotal } from '../../types';
import { formatCurrency } from '../../lib/format';
import { cn } from '../../lib/cn';
import { Card } from '../ui/Card';

interface CategoryBreakdownProps {
  categories: CategoryTotal[];
  currency: string;
}

interface CategoryGroupProps {
  title: string;
  description: string;
  totals: CategoryTotal[];
  tone: 'income' | 'expense';
  currency: string;
}

function CategoryGroup({
  title,
  description,
  totals,
  tone,
  currency,
}: CategoryGroupProps) {
  const isIncome = tone === 'income';

  return (
    <Card className="p-5">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h3 className="text-base font-bold text-stone-900">{title}</h3>
          <p className="text-xs text-stone-500">{description}</p>
        </div>
        <span
          className={cn(
            'rounded-full px-2.5 py-1 text-xs font-bold',
            isIncome
              ? 'bg-income-light text-income'
              : 'bg-expense-light text-expense'
          )}
        >
          {totals.length}
        </span>
      </div>

      {totals.length === 0 ? (
        <p className="mt-4 rounded-xl border border-dashed border-warm-300 bg-warm-50/60 px-4 py-8 text-center text-sm text-stone-500">
          No {title.toLowerCase()} recorded in this range.
        </p>
      ) : (
        <ul className="mt-4 space-y-4">
          {totals.map((total) => (
            <li key={total.category_id}>
              <div className="flex items-baseline justify-between gap-3">
                <p className="truncate text-sm font-semibold text-stone-800">
                  {total.category_name}
                </p>
                <p
                  className={cn(
                    'shrink-0 text-sm font-bold',
                    isIncome ? 'text-income' : 'text-expense'
                  )}
                >
                  {formatCurrency(total.total, currency)}
                </p>
              </div>
              <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-warm-100">
                <div
                  className={cn(
                    'h-full rounded-full',
                    isIncome
                      ? 'bg-gradient-to-r from-income to-emerald-400'
                      : 'bg-gradient-to-r from-expense to-rose-400'
                  )}
                  style={{
                    width: `${Math.min(Math.max(total.share, 0), 100)}%`,
                  }}
                />
              </div>
              <p className="mt-1 text-xs text-stone-400">
                {total.share.toFixed(1)}% of {title.toLowerCase()}
              </p>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

export function CategoryBreakdown({
  categories,
  currency,
}: CategoryBreakdownProps) {
  const byType = (type: 'income' | 'expense') =>
    categories
      .filter((category) => category.type === type)
      .sort((a, b) => b.total - a.total);

  const income = byType('income');
  const expense = byType('expense');

  return (
    <section aria-label="Category breakdown" className="space-y-3">
      <div>
        <h2 className="text-base font-bold text-stone-900">
          Category breakdown
        </h2>
        <p className="text-xs text-stone-500">
          Where the money came from and where it went
        </p>
      </div>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <CategoryGroup
          title="Income"
          description="Share of money in"
          totals={income}
          tone="income"
          currency={currency}
        />
        <CategoryGroup
          title="Expense"
          description="Share of money out"
          totals={expense}
          tone="expense"
          currency={currency}
        />
      </div>
    </section>
  );
}
