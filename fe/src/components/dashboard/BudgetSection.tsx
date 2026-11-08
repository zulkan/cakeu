import { useEffect, useMemo, useState } from 'react';
import { Loader2, Pencil, PiggyBank, Plus } from 'lucide-react';
import type { Category } from '../../types';
import { useBudgetStore } from '../../stores/budgetStore';
import { useAccountStore } from '../../stores/accountStore';
import { formatCurrency } from '../../lib/format';
import { formatMonthLabel } from '../../lib/dates';
import { cn } from '../../lib/cn';
import { Card } from '../ui/Card';
import { Button } from '../ui/Button';
import { Select } from '../ui/Select';
import { labelClass } from '../ui/formStyles';
import { BudgetEditorModal } from '../budgets/BudgetEditorModal';

interface BudgetSectionProps {
  categories: Category[];
}

export function BudgetSection({ categories }: BudgetSectionProps) {
  const budgets = useBudgetStore((state) => state.budgets);
  const month = useBudgetStore((state) => state.month);
  const isLoading = useBudgetStore((state) => state.isLoading);
  const error = useBudgetStore((state) => state.error);
  const fetchBudgets = useBudgetStore((state) => state.fetchBudgets);
  const accounts = useAccountStore((state) => state.accounts);
  const [editorOpen, setEditorOpen] = useState(false);

  useEffect(() => {
    void fetchBudgets();
  }, [fetchBudgets]);

  const accountCurrencies = useMemo(() => {
    const seen = new Set<string>();
    const currencies: string[] = [];

    accounts.forEach((account) => {
      if (!seen.has(account.currency)) {
        seen.add(account.currency);
        currencies.push(account.currency);
      }
    });

    return currencies;
  }, [accounts]);

  const budgetCurrencies = useMemo(() => {
    const seen = new Set<string>();
    const currencies: string[] = [];

    budgets.forEach((budget) => {
      if (!seen.has(budget.currency)) {
        seen.add(budget.currency);
        currencies.push(budget.currency);
      }
    });

    return currencies;
  }, [budgets]);

  const currencyOptions = useMemo(() => {
    const seen = new Set(accountCurrencies);
    const currencies = [...accountCurrencies];

    budgetCurrencies.forEach((currency) => {
      if (!seen.has(currency)) {
        seen.add(currency);
        currencies.push(currency);
      }
    });

    return currencies;
  }, [accountCurrencies, budgetCurrencies]);

  const defaultCurrency =
    budgets[0]?.currency ||
    accountCurrencies[0] ||
    budgetCurrencies[0] ||
    'USD';
  const [preferredCurrency, setPreferredCurrency] = useState<string | null>(
    null
  );
  const selectedCurrency =
    preferredCurrency !== null && currencyOptions.includes(preferredCurrency)
      ? preferredCurrency
      : defaultCurrency;

  const visibleBudgets = budgets.filter(
    (budget) => budget.currency === selectedCurrency
  );
  const hasAnyBudgets = budgets.length > 0;
  const hasExpenseCategories = categories.some(
    (category) => category.type === 'expense'
  );
  const canEdit = hasExpenseCategories && accountCurrencies.length > 0;

  return (
    <Card className="p-5">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 className="text-base font-bold text-stone-900">
            Budgets this month
          </h2>
          <p className="text-xs text-stone-500">{formatMonthLabel(month)}</p>
        </div>
        <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
          {currencyOptions.length > 1 && (
            <div className="w-full space-y-1.5 sm:w-40">
              <label htmlFor="budget-section-currency" className={labelClass}>
                Currency
              </label>
              <Select
                id="budget-section-currency"
                value={selectedCurrency}
                onChange={(event) =>
                  setPreferredCurrency(event.target.value)
                }
              >
                {currencyOptions.map((code) => (
                  <option key={code} value={code}>
                    {code}
                  </option>
                ))}
              </Select>
            </div>
          )}
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setEditorOpen(true)}
            disabled={!canEdit}
          >
            <Pencil className="h-4 w-4" aria-hidden="true" />
            Edit budgets
          </Button>
        </div>
      </div>

      {accountCurrencies.length === 0 && (
        <p className="mt-2 text-xs text-stone-500">
          Add an account to set budgets.
        </p>
      )}

      {error && hasAnyBudgets && (
        <div
          role="alert"
          className="mt-4 rounded-xl border border-expense/20 bg-expense-light/60 px-4 py-3 text-sm font-medium text-expense"
        >
          {error}
        </div>
      )}

      {isLoading && !hasAnyBudgets ? (
        <div
          role="status"
          className="mt-5 flex items-center justify-center gap-2 rounded-xl bg-warm-50/70 px-4 py-8 text-sm font-medium text-stone-500"
        >
          <Loader2
            className="h-4 w-4 animate-spin text-brand-500"
            aria-hidden="true"
          />
          Loading budgets…
        </div>
      ) : error && !hasAnyBudgets ? (
        <div
          role="alert"
          className="mt-5 flex flex-col items-center gap-3 rounded-xl border border-expense/20 bg-expense-light/40 px-4 py-8 text-center"
        >
          <p className="text-sm font-medium text-stone-700">{error}</p>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void fetchBudgets()}
          >
            Try again
          </Button>
        </div>
      ) : visibleBudgets.length === 0 ? (
        <div className="mt-5 flex flex-col items-center gap-2 rounded-xl border border-dashed border-warm-300 bg-warm-50/60 px-4 py-8 text-center">
          <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-brand-100 text-brand-600">
            <PiggyBank className="h-5 w-5" aria-hidden="true" />
          </div>
          <p className="text-sm font-bold text-stone-900">
            {hasAnyBudgets && currencyOptions.length > 1
              ? `No budgets for ${selectedCurrency}`
              : 'No budgets yet'}
          </p>
          <p className="max-w-sm text-xs text-stone-500">
            Give each expense category a monthly limit and watch your progress
            here.
          </p>
          <Button
            size="sm"
            className="mt-1"
            onClick={() => setEditorOpen(true)}
            disabled={!canEdit}
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            Set a budget
          </Button>
        </div>
      ) : (
        <ul className="mt-4 space-y-3">
          {visibleBudgets.map((budget) => {
            const overBudget = budget.percent > 100;
            const width = Math.min(Math.max(budget.percent, 0), 100);

            return (
              <li
                key={`${budget.currency}-${budget.category_id}`}
                className={cn(
                  'rounded-xl px-3 py-3',
                  overBudget ? 'bg-expense-light/50' : 'bg-warm-50/60'
                )}
              >
                <div className="flex items-baseline justify-between gap-3">
                  <p
                    className={cn(
                      'truncate text-sm font-bold',
                      overBudget ? 'text-expense' : 'text-stone-800'
                    )}
                  >
                    {budget.category_name}
                  </p>
                  <p className="shrink-0 text-sm font-semibold text-stone-600">
                    {formatCurrency(budget.spent, budget.currency)}
                    <span className="font-normal text-stone-400">
                      {' '}
                      / {formatCurrency(budget.budget, budget.currency)}
                    </span>
                  </p>
                </div>

                <div
                  role="progressbar"
                  aria-label={`${budget.category_name} budget used`}
                  aria-valuemin={0}
                  aria-valuemax={100}
                  aria-valuenow={Math.round(budget.percent)}
                  className="mt-2 h-2 overflow-hidden rounded-full bg-warm-200"
                >
                  <div
                    className={cn(
                      'h-full rounded-full transition-[width] duration-500',
                      overBudget
                        ? 'bg-expense'
                        : budget.percent >= 80
                          ? 'bg-brand-500'
                          : 'bg-income'
                    )}
                    style={{ width: `${width}%` }}
                  />
                </div>

                <div className="mt-1.5 flex items-center justify-between gap-3 text-xs">
                  <p
                    className={cn(
                      'font-semibold',
                      overBudget ? 'text-expense' : 'text-stone-500'
                    )}
                  >
                    {overBudget
                      ? `Over by ${formatCurrency(Math.abs(budget.remaining), budget.currency)}`
                      : `${formatCurrency(budget.remaining, budget.currency)} remaining`}
                  </p>
                  <p
                    className={cn(
                      'font-bold',
                      overBudget ? 'text-expense' : 'text-stone-600'
                    )}
                  >
                    {Math.round(budget.percent)}%
                  </p>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      <BudgetEditorModal
        open={editorOpen}
        onClose={() => setEditorOpen(false)}
        categories={categories}
        budgets={budgets}
        month={month}
        currencies={currencyOptions}
        currency={selectedCurrency}
      />
    </Card>
  );
}
