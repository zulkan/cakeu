import { useCallback, useEffect, useState } from 'react';
import type { ChangeEvent } from 'react';
import {
  ChevronLeft,
  ChevronRight,
  Download,
  Loader2,
  Plus,
  Receipt,
  Upload,
} from 'lucide-react';
import { useAccountStore } from '../stores/accountStore';
import { useCategoryStore } from '../stores/categoryStore';
import { useTransactionStore } from '../stores/transactionStore';
import { useToastStore } from '../stores/toastStore';
import type { CreateTransactionRequest, Transaction } from '../types';
import { api } from '../lib/api';
import { downloadBlob } from '../lib/download';
import { PageHeader } from '../components/ui/PageHeader';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { Select } from '../components/ui/Select';
import { EmptyState, ErrorState, LoadingState } from '../components/ui/States';
import { ConfirmDialog } from '../components/ui/ConfirmDialog';
import { TransactionFiltersForm } from '../components/transactions/TransactionFiltersForm';
import { TransactionFormModal } from '../components/transactions/TransactionFormModal';
import { TransactionList } from '../components/transactions/TransactionList';
import { AttachmentsModal } from '../components/transactions/AttachmentsModal';
import { ImportCsvDialog } from '../components/transactions/ImportCsvDialog';

function messageOf(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

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
  const updateTransaction = useTransactionStore(
    (state) => state.updateTransaction
  );
  const deleteTransaction = useTransactionStore(
    (state) => state.deleteTransaction
  );
  const pushToast = useToastStore((state) => state.push);

  const accounts = useAccountStore((state) => state.accounts);
  const fetchAccounts = useAccountStore((state) => state.fetchAccounts);
  const categories = useCategoryStore((state) => state.categories);
  const fetchCategories = useCategoryStore((state) => state.fetchCategories);

  const [modalOpen, setModalOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Transaction | null>(null);
  const [importOpen, setImportOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Transaction | null>(null);
  const [attachmentsTarget, setAttachmentsTarget] =
    useState<Transaction | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);

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
    try {
      await createTransaction(data);
      await fetchAccounts();
      pushToast({ title: 'Transaction added', tone: 'success' });
    } catch (failure) {
      pushToast({ title: messageOf(failure, 'Unable to add transaction'), tone: 'error' });
      throw failure;
    }
  };

  const handleUpdate = async (data: CreateTransactionRequest) => {
    if (!editTarget) return;
    try {
      await updateTransaction(editTarget.id, data);
      await fetchAccounts();
      pushToast({ title: 'Transaction updated', tone: 'success' });
    } catch (failure) {
      pushToast({ title: messageOf(failure, 'Unable to update transaction'), tone: 'error' });
      throw failure;
    }
  };

  const handleExport = async () => {
    setIsExporting(true);
    setExportError(null);

    try {
      const blob = await api.exportTransactionsCsv(filters);
      const today = new Date().toISOString().slice(0, 10);
      downloadBlob(blob, `transactions-${today}.csv`);
    } catch (exportFailure) {
      setExportError(
        exportFailure instanceof Error
          ? exportFailure.message
          : 'Unable to export transactions'
      );
    } finally {
      setIsExporting(false);
    }
  };

  const handleImported = async () => {
    await Promise.all([fetchTransactions(), fetchAccounts()]);
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;

    setIsDeleting(true);
    const isTransfer = deleteTarget.kind === 'transfer';

    try {
      if (isTransfer) {
        await api.deleteTransfer(deleteTarget.transfer_group_id);
        await fetchTransactions();
      } else {
        await deleteTransaction(deleteTarget.id);
      }
      await fetchAccounts();
      setDeleteTarget(null);
      pushToast({
        title: isTransfer ? 'Transfer deleted' : 'Transaction deleted',
        tone: 'success',
      });
    } catch (deleteFailure) {
      pushToast({
        title: messageOf(deleteFailure, 'Unable to delete transaction'),
        tone: 'error',
      });
    } finally {
      setIsDeleting(false);
    }
  };

  const handleEdit = useCallback((target: Transaction) => {
    setEditTarget(target);
  }, []);

  const handleDeleteRequest = useCallback((target: Transaction) => {
    setDeleteTarget(target);
  }, []);

  const handleAttachments = useCallback((target: Transaction) => {
    setAttachmentsTarget(target);
  }, []);

  const handleAttachmentsClose = useCallback(() => {
    setAttachmentsTarget(null);
  }, []);

  const handlePageSizeChange = (event: ChangeEvent<HTMLSelectElement>) => {
    setFilters({ page_size: Number(event.target.value), page: 1 });
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
          <div className="flex flex-wrap items-center gap-2">
            <Button
              variant="secondary"
              disabled={isExporting}
              onClick={() => void handleExport()}
            >
              {isExporting ? (
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
              ) : (
                <Download className="h-4 w-4" aria-hidden="true" />
              )}
              {isExporting ? 'Exporting…' : 'Export CSV'}
            </Button>
            <Button variant="secondary" onClick={() => setImportOpen(true)}>
              <Upload className="h-4 w-4" aria-hidden="true" />
              Import CSV
            </Button>
            <Button onClick={() => setModalOpen(true)}>
              <Plus className="h-4 w-4" aria-hidden="true" />
              Add transaction
            </Button>
          </div>
        }
      />

      {exportError && (
        <div
          role="alert"
          className="rounded-xl border border-expense/20 bg-expense-light/60 px-4 py-3 text-sm font-medium text-expense"
        >
          {exportError}
        </div>
      )}

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
            onEdit={handleEdit}
            onDelete={handleDeleteRequest}
            onAttachments={handleAttachments}
          />

          <div className="flex flex-col gap-3 border-t border-warm-200 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex flex-wrap items-center gap-3">
              <p className="text-xs text-stone-500">
                Showing {rangeStart}–{rangeEnd} of {total}
              </p>
              <div className="flex items-center gap-2">
                <label
                  htmlFor="transactions-page-size"
                  className="text-xs font-medium text-stone-500"
                >
                  Rows per page
                </label>
                <Select
                  id="transactions-page-size"
                  value={pageSize}
                  onChange={handlePageSizeChange}
                  disabled={isLoading}
                  className="h-9 w-20 py-0 text-xs"
                >
                  {[10, 25, 50, 100].map((size) => (
                    <option key={size} value={size}>
                      {size}
                    </option>
                  ))}
                </Select>
              </div>
            </div>
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

      <TransactionFormModal
        open={editTarget !== null}
        onClose={() => setEditTarget(null)}
        accounts={accounts}
        categories={categories}
        transaction={editTarget}
        onSubmit={handleUpdate}
      />

      <ImportCsvDialog
        open={importOpen}
        currency={currency}
        onClose={() => setImportOpen(false)}
        onImported={handleImported}
      />

      <AttachmentsModal
        open={attachmentsTarget !== null}
        transaction={attachmentsTarget}
        onClose={handleAttachmentsClose}
      />

      <ConfirmDialog
        open={deleteTarget !== null}
        title={
          deleteTarget?.kind === 'transfer'
            ? 'Delete transfer?'
            : 'Delete transaction?'
        }
        description={
          deleteTarget
            ? deleteTarget.kind === 'transfer'
              ? 'Both sides of this transfer will be removed and both account balances will be restored.'
              : `“${deleteTarget.description || deleteTarget.category_name}” will be removed and the account balance will be restored.`
            : ''
        }
        isLoading={isDeleting}
        onConfirm={() => void handleDelete()}
        onCancel={() => {
          if (isDeleting) return;
          setDeleteTarget(null);
        }}
      />
    </div>
  );
}

export default Transactions;
