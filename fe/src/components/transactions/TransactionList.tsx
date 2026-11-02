import { Pencil, Trash2 } from 'lucide-react';
import type { Category, Transaction } from '../../types';
import { formatCurrency, formatDate } from '../../lib/format';
import { TypeBadge } from '../ui/TypeBadge';

interface TransactionListProps {
  transactions: Transaction[];
  categories: Category[];
  currency: string;
  onEdit: (transaction: Transaction) => void;
  onDelete: (transaction: Transaction) => void;
}

export function TransactionList({
  transactions,
  categories,
  currency,
  onEdit,
  onDelete,
}: TransactionListProps) {
  const colorById = new Map(
    categories.map((category) => [category.id, category.color])
  );

  return (
    <>
      <div className="hidden overflow-x-auto md:block">
        <table className="w-full text-left text-sm">
          <thead>
            <tr className="border-b border-warm-200 text-xs uppercase tracking-wide text-stone-400">
              <th scope="col" className="px-4 py-3 font-semibold">
                Date
              </th>
              <th scope="col" className="px-4 py-3 font-semibold">
                Description
              </th>
              <th scope="col" className="px-4 py-3 font-semibold">
                Category
              </th>
              <th scope="col" className="px-4 py-3 font-semibold">
                Account
              </th>
              <th scope="col" className="px-4 py-3 font-semibold">
                Type
              </th>
              <th scope="col" className="px-4 py-3 text-right font-semibold">
                Amount
              </th>
              <th scope="col" className="px-4 py-3 text-right font-semibold">
                <span className="sr-only">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-warm-100">
            {transactions.map((transaction) => {
              const isIncome = transaction.type === 'income';

              return (
                <tr
                  key={transaction.id}
                  className="transition-colors hover:bg-warm-50/70"
                >
                  <td className="whitespace-nowrap px-4 py-3 text-stone-500">
                    {formatDate(transaction.date)}
                  </td>
                  <td className="max-w-[16rem] px-4 py-3">
                    <p className="truncate font-semibold text-stone-900">
                      {transaction.description || '—'}
                    </p>
                  </td>
                  <td className="whitespace-nowrap px-4 py-3">
                    <span className="inline-flex items-center gap-2 text-stone-600">
                      <span
                        className="h-2.5 w-2.5 rounded-full"
                        style={{
                          backgroundColor:
                            colorById.get(transaction.category_id) ??
                            '#FB923C',
                        }}
                        aria-hidden="true"
                      />
                      {transaction.category_name}
                    </span>
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-stone-600">
                    {transaction.account_name}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3">
                    <TypeBadge type={transaction.type} />
                  </td>
                  <td
                    className={`whitespace-nowrap px-4 py-3 text-right font-bold ${
                      isIncome ? 'text-income' : 'text-expense'
                    }`}
                  >
                    {isIncome ? '+' : '−'}
                    {formatCurrency(Math.abs(transaction.amount), currency)}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center justify-end gap-1">
                      <button
                        type="button"
                        onClick={() => onEdit(transaction)}
                        aria-label={`Edit transaction ${transaction.description || transaction.category_name}`}
                        className="rounded-lg p-2 text-stone-400 transition-colors hover:bg-brand-50 hover:text-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
                      >
                        <Pencil className="h-4 w-4" aria-hidden="true" />
                      </button>
                      <button
                        type="button"
                        onClick={() => onDelete(transaction)}
                        aria-label={`Delete transaction ${transaction.description || transaction.category_name}`}
                        className="rounded-lg p-2 text-stone-400 transition-colors hover:bg-expense-light hover:text-expense focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
                      >
                        <Trash2 className="h-4 w-4" aria-hidden="true" />
                      </button>
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <ul className="divide-y divide-warm-100 md:hidden">
        {transactions.map((transaction) => {
          const isIncome = transaction.type === 'income';

          return (
            <li key={transaction.id} className="px-4 py-4">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="truncate font-semibold text-stone-900">
                    {transaction.description || transaction.category_name}
                  </p>
                  <p className="mt-0.5 truncate text-xs text-stone-500">
                    {transaction.category_name} · {transaction.account_name}
                  </p>
                  <p className="mt-0.5 text-xs text-stone-400">
                    {formatDate(transaction.date)}
                  </p>
                </div>
                <div className="flex shrink-0 flex-col items-end gap-2">
                  <span
                    className={`text-sm font-bold ${
                      isIncome ? 'text-income' : 'text-expense'
                    }`}
                  >
                    {isIncome ? '+' : '−'}
                    {formatCurrency(Math.abs(transaction.amount), currency)}
                  </span>
                  <div className="flex items-center gap-1">
                    <button
                      type="button"
                      onClick={() => onEdit(transaction)}
                      aria-label={`Edit transaction ${transaction.description || transaction.category_name}`}
                      className="rounded-lg p-1.5 text-stone-400 transition-colors hover:bg-brand-50 hover:text-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
                    >
                      <Pencil className="h-4 w-4" aria-hidden="true" />
                    </button>
                    <button
                      type="button"
                      onClick={() => onDelete(transaction)}
                      aria-label={`Delete transaction ${transaction.description || transaction.category_name}`}
                      className="rounded-lg p-1.5 text-stone-400 transition-colors hover:bg-expense-light hover:text-expense focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
                    >
                      <Trash2 className="h-4 w-4" aria-hidden="true" />
                    </button>
                  </div>
                </div>
              </div>
            </li>
          );
        })}
      </ul>
    </>
  );
}
