import { ArrowDownLeft, ArrowUpRight } from 'lucide-react';
import { cn } from '../../lib/cn';

export function TypeBadge({ type }: { type: 'income' | 'expense' }) {
  const isIncome = type === 'income';
  const Icon = isIncome ? ArrowDownLeft : ArrowUpRight;

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-semibold',
        isIncome
          ? 'bg-income-light text-income'
          : 'bg-expense-light text-expense'
      )}
    >
      <Icon className="h-3.5 w-3.5" aria-hidden="true" />
      {isIncome ? 'Income' : 'Expense'}
    </span>
  );
}
