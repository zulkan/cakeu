import { Trash2 } from 'lucide-react';
import type { Category } from '../../types';
import { Card } from '../ui/Card';
import { cn } from '../../lib/cn';

interface CategoryGroupProps {
  title: string;
  description: string;
  categories: Category[];
  tone: 'income' | 'expense';
  onDelete: (category: Category) => void;
}

export function CategoryGroup({
  title,
  description,
  categories,
  tone,
  onDelete,
}: CategoryGroupProps) {
  const isIncome = tone === 'income';

  return (
    <Card className="p-5">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h2 className="text-base font-bold text-stone-900">{title}</h2>
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
          {categories.length}
        </span>
      </div>

      {categories.length === 0 ? (
        <p className="mt-5 rounded-xl border border-dashed border-warm-300 bg-warm-50/60 px-4 py-6 text-center text-sm text-stone-500">
          No {title.toLowerCase()} categories yet.
        </p>
      ) : (
        <ul className="mt-4 divide-y divide-warm-100">
          {categories.map((category) => (
            <li
              key={category.id}
              className="flex items-center justify-between gap-3 py-3"
            >
              <div className="flex min-w-0 items-center gap-3">
                <span
                  className="h-3 w-3 shrink-0 rounded-full ring-2 ring-white"
                  style={{ backgroundColor: category.color }}
                  aria-hidden="true"
                />
                <p className="truncate text-sm font-semibold text-stone-800">
                  {category.name}
                </p>
              </div>
              <button
                type="button"
                onClick={() => onDelete(category)}
                aria-label={`Delete category ${category.name}`}
                className="rounded-lg p-2 text-stone-400 transition-colors hover:bg-expense-light hover:text-expense focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
              >
                <Trash2 className="h-4 w-4" aria-hidden="true" />
              </button>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
