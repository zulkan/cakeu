import { useEffect, useState } from 'react';
import { ChevronLeft, ChevronRight, Plus, Receipt } from 'lucide-react';
import { useAccountStore } from '../stores/accountStore';
import { useCategoryStore } from '../stores/categoryStore';
import { useTransactionStore } from '../stores/transactionStore';
import type { CreateTransactionRequest, Transaction } from '../types';
import { PageHeader } from '../components/ui/PageHeader';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { EmptyState, ErrorState, LoadingState } from '../components/ui/States';
import { ConfirmDialog } from '../components/ui/ConfirmDialog';
import { TransactionFiltersForm } from '../components/transactions/TransactionFiltersForm';
import { TransactionFormModal } from '../components/transactions/TransactionFormModal';
import { TransactionList } from '../components/transactions/TransactionList';

function Transactions() {
  const transactions = useTransactionStore((state) => state.transactions);
  const total = useTransactionStore((state) => state.total);
  const isLoading = useTransactionStore((state) => state.isLoading);
  const error = useTransactionStore((state) => state.error);
  const filters = useTransactionStore((state) => state.filters);
  const setFilters = useTransactionStore((state) => state.setFilters);
  const fetchTransactions = useTransactionStore(
    (state) => state.fetchTransactions
  );
  const createTransaction = useTransactionStore(
    (state) => state.createTransaction
  );
  const deleteTransaction = useTransactionStore(
    (state) => state.deleteTransaction
  );

  const accounts = useAccountStore((state) => state.accounts);
  const fetchAccounts = useAccountStore((state) => state.fetchAccounts);
  const categories = useCategoryStore((state) => state.categories);
  const fetchCategories = useCategoryStore((state) => state.fetchCategories);

  const [modalOpen, setModalOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Transaction | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  useEffect(() => {
    void fetchAccounts();
    void fetchCategories();
    void fetchTransactions();
  }, [fetchAccounts, fetchCategories, fetchTransactions]);

  const currency = accounts[0]?.currency ?? 'USD';
  const page = filters.page ?? 1;
  const pageSize = filters.page_size ?? 10;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const rangeStart = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const rangeEnd = Math.min(page * pageSize, total);
  const hasFilters = Boolean(
    filters.account_id ||
      filters.category_id ||
      filters.type ||
      filters.date_from ||
      filters.date_to
  );

  const handleCreate = async (data: CreateTransactionRequest) => {
    await createTransaction(data);
    await fetchAccounts();
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;

    setIsDeleting(true);
    setDeleteError(null);

    try {
      await deleteTransaction(deleteTarget.id);
      await fetchAccounts();
      setDeleteTarget(null);
    } catch (deleteFailure) {
      setDeleteError(
        deleteFailure instanceof Error
          ? deleteFailure.message
          : 'Unable to delete transaction'
      );
    } finally {
      setIsDeleting(false);
    }
  };

  const clearFilters = () => {
    setFilters({
      account_id: undefined,
      category_id: undefined,
      type: undefined,
      date_from: undefined,
      date_to: undefined,
      page: 1,
    });
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Transactions"
        description="Filter, review, and record every money movement."
        action={
          <Button onClick={() => setModalOpen(true)}>
            <Plus className="h-4 w-4" aria-hidden="true" />
            Add transaction
          </Button>
        }
      />

      <TransactionFiltersForm
        key={[
          filters.account_id,
          filters.category_id,
          filters.type,
          filters.date_from,
          filters.date_to,
        ].join('|')}
        accounts={accounts}
        categories={categories}
        initialFilters={filters}
        onApply={(next) => setFilters(next)}
        onReset={clearFilters}
      />

      {error && transactions.length > 0 && (
        <div
          role="alert"
          className="rounded-xl border border-expense/20 bg-expense-light/60 px-4 py-3 text-sm font-medium text-expense"
        >
          {error}
        </div>
      )}

      {isLoading && transactions.length === 0 ? (
        <LoadingState label="Loading transactions…" />
      ) : error && transactions.length === 0 ? (
        <ErrorState
          message={error}
          onRetry={() => void fetchTransactions()}
        />
      ) : transactions.length === 0 ? (
        <EmptyState
          icon={<Receipt className="h-6 w-6" aria-hidden="true" />}
          title={hasFilters ? 'No matching transactions' : 'No transactions yet'}
          description={
            hasFilters
              ? 'Try widening your filters or clearing them to see everything.'
              : 'Record your first income or expense to start building your history.'
          }
          action={
            hasFilters ? (
              <Button variant="secondary" onClick={clearFilters}>
                Clear filters
              </Button>
            ) : (
              <Button onClick={() => setModalOpen(true)}>
                <Plus className="h-4 w-4" aria-hidden="true" />
                Add transaction
              </Button>
            )
          }
        />
      ) : (
        <Card className="overflow-hidden">
          <div className="flex items-center justify-between border-b border-warm-200 px-4 py-3">
            <p className="text-sm font-semibold text-stone-700">
              {total} {total === 1 ? 'transaction' : 'transactions'}
            </p>
            <p className="text-xs text-stone-400">
              Page {page} of {totalPages}
            </p>
          </div>

          <TransactionList
            transactions={transactions}
            categories={categories}
            currency={currency}
            onDelete={(transaction) => {
              setDeleteError(null);
              setDeleteTarget(transaction);
            }}
          />

          <div className="flex flex-col gap-3 border-t border-warm-200 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-xs text-stone-500">
              Showing {rangeStart}–{rangeEnd} of {total}
            </p>
            <div className="flex items-center gap-2">
              <Button
                variant="secondary"
                size="sm"
                disabled={page <= 1 || isLoading}
                onClick={() => setFilters({ page: page - 1 })}
              >
                <ChevronLeft className="h-4 w-4" aria-hidden="true" />
                Previous
              </Button>
              <Button
                variant="secondary"
                size="sm"
                disabled={page >= totalPages || isLoading}
                onClick={() => setFilters({ page: page + 1 })}
              >
                Next
                <ChevronRight className="h-4 w-4" aria-hidden="true" />
              </Button>
            </div>
          </div>
        </Card>
      )}

      <TransactionFormModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        accounts={accounts}
        categories={categories}
        onSubmit={handleCreate}
      />

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Delete transaction?"
        description={
          deleteTarget
            ? `“${deleteTarget.description || deleteTarget.category_name}” will be removed and the account balance will be restored.`
            : ''
        }
        isLoading={isDeleting}
        error={deleteError}
        onConfirm={() => void handleDelete()}
        onCancel={() => {
          if (isDeleting) return;
          setDeleteTarget(null);
          setDeleteError(null);
        }}
      />
    </div>
  );
}

export default Transactions;
