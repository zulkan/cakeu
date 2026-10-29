import { useState } from 'react';
import type { FormEvent } from 'react';
import type { Account, Category, TransactionFilters } from '../../types';
import { Button } from '../ui/Button';
import { Select } from '../ui/Select';
import { inputClass, labelClass } from '../ui/formStyles';

interface TransactionFiltersFormProps {
  accounts: Account[];
  categories: Category[];
  initialFilters: TransactionFilters;
  onApply: (filters: TransactionFilters) => void;
  onReset: () => void;
}

interface FilterValues {
  account_id: string;
  category_id: string;
  type: string;
  date_from: string;
  date_to: string;
}

function toValues(filters: TransactionFilters): FilterValues {
  return {
    account_id: filters.account_id ?? '',
    category_id: filters.category_id ?? '',
    type: filters.type ?? '',
    date_from: filters.date_from ?? '',
    date_to: filters.date_to ?? '',
  };
}

export function TransactionFiltersForm({
  accounts,
  categories,
  initialFilters,
  onApply,
  onReset,
}: TransactionFiltersFormProps) {
  const [values, setValues] = useState<FilterValues>(() =>
    toValues(initialFilters)
  );

  const update = (key: keyof FilterValues, value: string) => {
    setValues((current) => ({ ...current, [key]: value }));
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onApply({
      account_id: values.account_id || undefined,
      category_id: values.category_id || undefined,
      type:
        values.type === 'income' || values.type === 'expense'
          ? values.type
          : undefined,
      date_from: values.date_from || undefined,
      date_to: values.date_to || undefined,
      page: 1,
    });
  };

  const handleReset = () => {
    setValues({
      account_id: '',
      category_id: '',
      type: '',
      date_from: '',
      date_to: '',
    });
    onReset();
  };

  return (
    <form
      onSubmit={handleSubmit}
      className="rounded-2xl border border-warm-200/80 bg-surface-card p-4 shadow-sm sm:p-5"
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-5">
        <div className="space-y-1.5">
          <label htmlFor="filter-account" className={labelClass}>
            Account
          </label>
          <Select
            id="filter-account"
            value={values.account_id}
            onChange={(event) => update('account_id', event.target.value)}
          >
            <option value="">All accounts</option>
            {accounts.map((account) => (
              <option key={account.id} value={account.id}>
                {account.name}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1.5">
          <label htmlFor="filter-category" className={labelClass}>
            Category
          </label>
          <Select
            id="filter-category"
            value={values.category_id}
            onChange={(event) => update('category_id', event.target.value)}
          >
            <option value="">All categories</option>
            {categories.map((category) => (
              <option key={category.id} value={category.id}>
                {category.name}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1.5">
          <label htmlFor="filter-type" className={labelClass}>
            Type
          </label>
          <Select
            id="filter-type"
            value={values.type}
            onChange={(event) => update('type', event.target.value)}
          >
            <option value="">All types</option>
            <option value="income">Income</option>
            <option value="expense">Expense</option>
          </Select>
        </div>

        <div className="space-y-1.5">
          <label htmlFor="filter-from" className={labelClass}>
            From
          </label>
          <input
            id="filter-from"
            type="date"
            value={values.date_from}
            onChange={(event) => update('date_from', event.target.value)}
            className={inputClass}
          />
        </div>

        <div className="space-y-1.5">
          <label htmlFor="filter-to" className={labelClass}>
            To
          </label>
          <input
            id="filter-to"
            type="date"
            value={values.date_to}
            onChange={(event) => update('date_to', event.target.value)}
            className={inputClass}
          />
        </div>
      </div>

      <div className="mt-4 flex flex-wrap justify-end gap-2">
        <Button variant="ghost" size="sm" onClick={handleReset}>
          Reset
        </Button>
        <Button type="submit" size="sm">
          Apply filters
        </Button>
      </div>
    </form>
  );
}
