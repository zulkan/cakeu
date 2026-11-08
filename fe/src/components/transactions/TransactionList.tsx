import { memo } from 'react';
import { ArrowLeftRight, Lock, Paperclip, Pencil, Trash2 } from 'lucide-react';
import type { Category, Transaction } from '../../types';
import { formatCurrency, formatDate } from '../../lib/format';
import { TypeBadge } from '../ui/TypeBadge';

interface TransactionListProps {
  transactions: Transaction[];
  categories: Category[];
  onEdit: (transaction: Transaction) => void;
  onDelete: (transaction: Transaction) => void;
  onAttachments?: (transaction: Transaction) => void;
}

function systemLabel(transaction: Transaction): string {
  if (transaction.kind === 'opening') return 'Opening balance';
  if (transaction.kind === 'transfer') return 'Transfer';
  return 'Balance adjustment';
}

function actionLabel(transaction: Transaction, action: 'Edit' | 'Delete'): string {
  const subject =
    transaction.description ||
    (transaction.kind === 'transfer' ? 'Transfer' : transaction.category_name);
  return `${action} ${transaction.kind === 'transfer' ? 'transfer' : 'transaction'} ${subject}`;
}

function attachmentsLabel(transaction: Transaction): string {
  const subject = transaction.description || transaction.category_name;
  return `Attachments for ${subject}`;
}

export const TransactionList = memo(function TransactionList({
  transactions,
  categories,
  onEdit,
  onDelete,
  onAttachments,
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
              const isSystem = transaction.kind !== 'normal';
              const isTransfer = transaction.kind === 'transfer';

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
                    {isSystem ? (
                      <span className="inline-flex items-center gap-1.5 rounded-full bg-warm-100 px-2.5 py-1 text-xs font-semibold text-warm-800">
                        {isTransfer ? (
                          <ArrowLeftRight
                            className="h-3 w-3"
                            aria-hidden="true"
                          />
                        ) : (
                          <Lock className="h-3 w-3" aria-hidden="true" />
                        )}
                        {systemLabel(transaction)}
                      </span>
                    ) : (
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
                    )}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-stone-600">
                    {transaction.account_name}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3">
                    {isSystem ? (
                      <span className="rounded-full bg-warm-100 px-2.5 py-1 text-xs font-semibold text-warm-800">
                        System
                      </span>
                    ) : (
                      <TypeBadge type={transaction.type} />
                    )}
                  </td>
                  <td
                    className={`whitespace-nowrap px-4 py-3 text-right font-bold ${
                      isIncome ? 'text-income' : 'text-expense'
                    }`}
                  >
                    {isIncome ? '+' : '−'}
                    {formatCurrency(
                      Math.abs(transaction.amount),
                      transaction.currency
                    )}
                  </td>
                  <td className="px-4 py-3">
                    {isSystem && !isTransfer ? (
                      <div className="flex items-center justify-end">
                        <span
                          className="rounded-lg p-2 text-stone-300"
                          title="System entries cannot be edited or deleted"
                        >
                          <Lock className="h-4 w-4" aria-hidden="true" />
                        </span>
                      </div>
                    ) : (
                      <div className="flex items-center justify-end gap-1">
                        {!isSystem && onAttachments && (
                          <button
                            type="button"
                            onClick={() => onAttachments(transaction)}
                            aria-label={attachmentsLabel(transaction)}
                            className="rounded-lg p-2 text-stone-400 transition-colors hover:bg-brand-50 hover:text-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
                          >
                            <Paperclip className="h-4 w-4" aria-hidden="true" />
                          </button>
                        )}
                        {!isSystem && (
                          <button
                            type="button"
                            onClick={() => onEdit(transaction)}
                            aria-label={actionLabel(transaction, 'Edit')}
                            className="rounded-lg p-2 text-stone-400 transition-colors hover:bg-brand-50 hover:text-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
                          >
                            <Pencil className="h-4 w-4" aria-hidden="true" />
                          </button>
                        )}
                        <button
                          type="button"
                          onClick={() => onDelete(transaction)}
                          aria-label={actionLabel(transaction, 'Delete')}
                          title={
                            isTransfer
                              ? 'Removes both sides of this transfer'
                              : undefined
                          }
                          className="rounded-lg p-2 text-stone-400 transition-colors hover:bg-expense-light hover:text-expense focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
                        >
                          <Trash2 className="h-4 w-4" aria-hidden="true" />
                        </button>
                      </div>
                    )}
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
          const isSystem = transaction.kind !== 'normal';
          const isTransfer = transaction.kind === 'transfer';

          return (
            <li key={transaction.id} className="px-4 py-4">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="truncate font-semibold text-stone-900">
                    {transaction.description || systemLabel(transaction)}
                  </p>
                  <p className="mt-0.5 truncate text-xs text-stone-500">
                    {isSystem
                      ? systemLabel(transaction)
                      : transaction.category_name}{' '}
                    · {transaction.account_name}
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
                    {formatCurrency(
                      Math.abs(transaction.amount),
                      transaction.currency
                    )}
                  </span>
                  {isSystem && !isTransfer ? (
                    <span
                      className="rounded-lg p-1.5 text-stone-300"
                      title="System entries cannot be edited or deleted"
                    >
                      <Lock className="h-4 w-4" aria-hidden="true" />
                    </span>
                  ) : (
                    <div className="flex items-center gap-1">
                      {!isSystem && onAttachments && (
                        <button
                          type="button"
                          onClick={() => onAttachments(transaction)}
                          aria-label={attachmentsLabel(transaction)}
                          className="rounded-lg p-1.5 text-stone-400 transition-colors hover:bg-brand-50 hover:text-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
                        >
                          <Paperclip className="h-4 w-4" aria-hidden="true" />
                        </button>
                      )}
                      {!isSystem && (
                        <button
                          type="button"
                          onClick={() => onEdit(transaction)}
                          aria-label={actionLabel(transaction, 'Edit')}
                          className="rounded-lg p-1.5 text-stone-400 transition-colors hover:bg-brand-50 hover:text-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
                        >
                          <Pencil className="h-4 w-4" aria-hidden="true" />
                        </button>
                      )}
                      <button
                        type="button"
                        onClick={() => onDelete(transaction)}
                        aria-label={actionLabel(transaction, 'Delete')}
                        title={
                          isTransfer
                            ? 'Removes both sides of this transfer'
                            : undefined
                        }
                        className="rounded-lg p-1.5 text-stone-400 transition-colors hover:bg-expense-light hover:text-expense focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
                      >
                        <Trash2 className="h-4 w-4" aria-hidden="true" />
                      </button>
                    </div>
                  )}
                </div>
              </div>
            </li>
          );
        })}
      </ul>
    </>
  );
});
