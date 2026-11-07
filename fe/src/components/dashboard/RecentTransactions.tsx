import { ArrowRight, Receipt } from 'lucide-react';
import { Link } from 'react-router-dom';
import type { Transaction } from '../../types';
import { formatCurrency, formatDate } from '../../lib/format';
import { Card } from '../ui/Card';
import { EmptyState } from '../ui/States';

interface RecentTransactionsProps {
  transactions: Transaction[];
}

export function RecentTransactions({
  transactions,
}: RecentTransactionsProps) {
  return (
    <Card className="p-5">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div>
          <h2 className="text-base font-bold text-stone-900">
            Recent transactions
          </h2>
          <p className="text-xs text-stone-500">Your latest money movements</p>
        </div>
        <Link
          to="/transactions"
          className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs font-semibold text-brand-600 transition-colors hover:bg-brand-50 hover:text-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
        >
          View all
          <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
        </Link>
      </div>

      {transactions.length === 0 ? (
        <EmptyState
          icon={<Receipt className="h-6 w-6" aria-hidden="true" />}
          title="No transactions yet"
          description="Add your first transaction to see it appear here."
          action={
            <Link
              to="/transactions"
              className="inline-flex items-center gap-2 rounded-xl bg-brand-500 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-brand-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2"
            >
              Add transaction
            </Link>
          }
        />
      ) : (
        <ul className="divide-y divide-warm-100">
          {transactions.map((transaction) => {
            const isIncome = transaction.type === 'income';

            return (
              <li
                key={transaction.id}
                className="flex items-center gap-3 py-3"
              >
                <span
                  className="h-2.5 w-2.5 shrink-0 rounded-full"
                  style={{ backgroundColor: isIncome ? '#16A34A' : '#DC2626' }}
                  aria-hidden="true"
                />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-semibold text-stone-900">
                    {transaction.description || transaction.category_name}
                  </p>
                  <p className="truncate text-xs text-stone-500">
                    {transaction.category_name} · {transaction.account_name} ·{' '}
                    {formatDate(transaction.date)}
                  </p>
                </div>
                <span
                  className={`shrink-0 text-sm font-bold ${
                    isIncome ? 'text-income' : 'text-expense'
                  }`}
                >
                  {isIncome ? '+' : '−'}
                  {formatCurrency(
                    Math.abs(transaction.amount),
                    transaction.currency
                  )}
                </span>
              </li>
            );
          })}
        </ul>
      )}
    </Card>
  );
}
