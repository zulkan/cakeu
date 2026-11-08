import { useEffect, useState } from 'react';
import {
  CalendarClock,
  Loader2,
  Pause,
  Pencil,
  Play,
  Plus,
  Repeat,
  Trash2,
} from 'lucide-react';
import type {
  CreateRecurringRequest,
  RecurringFrequency,
  RecurringRule,
  UpdateRecurringRequest,
} from '../types';
import { useAccountStore } from '../stores/accountStore';
import { useCategoryStore } from '../stores/categoryStore';
import { useRecurringStore } from '../stores/recurringStore';
import { useToastStore } from '../stores/toastStore';
import { formatCurrency, formatDate } from '../lib/format';
import { cn } from '../lib/cn';
import { PageHeader } from '../components/ui/PageHeader';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { TypeBadge } from '../components/ui/TypeBadge';
import { EmptyState, ErrorState, LoadingState } from '../components/ui/States';
import { ConfirmDialog } from '../components/ui/ConfirmDialog';
import { RecurringFormModal } from '../components/recurring/RecurringFormModal';

const frequencyLabels: Record<RecurringFrequency, string> = {
  daily: 'Daily',
  weekly: 'Weekly',
  monthly: 'Monthly',
};

function describeFrequency(rule: RecurringRule): string {
  return `${frequencyLabels[rule.frequency]} ×${rule.interval}`;
}

function ruleLabel(rule: RecurringRule, categoryName: string): string {
  return rule.description || categoryName;
}

function messageOf(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function Recurring() {
  const rules = useRecurringStore((state) => state.rules);
  const isLoading = useRecurringStore((state) => state.isLoading);
  const error = useRecurringStore((state) => state.error);
  const fetchRules = useRecurringStore((state) => state.fetchRules);
  const createRule = useRecurringStore((state) => state.createRule);
  const updateRule = useRecurringStore((state) => state.updateRule);
  const deleteRule = useRecurringStore((state) => state.deleteRule);
  const runNow = useRecurringStore((state) => state.runNow);
  const pushToast = useToastStore((state) => state.push);

  const accounts = useAccountStore((state) => state.accounts);
  const fetchAccounts = useAccountStore((state) => state.fetchAccounts);
  const categories = useCategoryStore((state) => state.categories);
  const fetchCategories = useCategoryStore((state) => state.fetchCategories);

  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<RecurringRule | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<RecurringRule | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  useEffect(() => {
    void fetchRules();
    void fetchAccounts();
    void fetchCategories();
  }, [fetchRules, fetchAccounts, fetchCategories]);

  const currency = accounts[0]?.currency ?? 'USD';
  const accountNames = new Map(accounts.map((item) => [item.id, item.name]));
  const categoryNames = new Map(
    categories.map((item) => [item.id, item.name])
  );

  const handleCreate = async (data: CreateRecurringRequest) => {
    try {
      await createRule(data);
      pushToast({ title: 'Recurring rule created', tone: 'success' });
    } catch (failure) {
      pushToast({ title: messageOf(failure, 'Unable to create the rule'), tone: 'error' });
      throw failure;
    }
  };

  const handleUpdate = async (id: string, data: UpdateRecurringRequest) => {
    try {
      await updateRule(id, data);
      pushToast({ title: 'Recurring rule updated', tone: 'success' });
    } catch (failure) {
      pushToast({ title: messageOf(failure, 'Unable to update the rule'), tone: 'error' });
      throw failure;
    }
  };

  const handleToggle = async (rule: RecurringRule) => {
    setBusyId(rule.id);

    try {
      await updateRule(rule.id, { active: !rule.active });
      pushToast({
        title: rule.active ? 'Rule paused' : 'Rule resumed',
        tone: 'success',
      });
    } catch (toggleFailure) {
      pushToast({
        title: messageOf(toggleFailure, 'Unable to update the recurring rule'),
        tone: 'error',
      });
    } finally {
      setBusyId(null);
    }
  };

  const handleRunNow = async (rule: RecurringRule) => {
    setBusyId(rule.id);

    try {
      const posted = await runNow(rule.id);
      const label =
        rule.description ||
        categoryNames.get(rule.category_id) ||
        'this rule';

      pushToast({
        title:
          posted === 0
            ? `No transactions were posted for “${label}” — it may already have run for this period.`
            : `Posted ${posted} ${posted === 1 ? 'transaction' : 'transactions'} for “${label}”.`,
        tone: posted === 0 ? 'info' : 'success',
      });
    } catch (runFailure) {
      pushToast({
        title: messageOf(runFailure, 'Unable to run the recurring rule'),
        tone: 'error',
      });
    } finally {
      setBusyId(null);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;

    setIsDeleting(true);

    try {
      await deleteRule(deleteTarget.id);
      setDeleteTarget(null);
      pushToast({ title: 'Recurring rule deleted', tone: 'success' });
    } catch (deleteFailure) {
      const message = messageOf(
        deleteFailure,
        'Unable to delete the recurring rule'
      );
      setDeleteError(message);
      pushToast({ title: message, tone: 'error' });
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Recurring"
        description="Schedule the transactions that happen again and again."
        action={
          <Button
            onClick={() => {
              setEditing(null);
              setModalOpen(true);
            }}
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            New rule
          </Button>
        }
      />

      {isLoading && rules.length === 0 ? (
        <LoadingState label="Loading recurring rules…" />
      ) : error && rules.length === 0 ? (
        <ErrorState message={error} onRetry={() => void fetchRules()} />
      ) : rules.length === 0 ? (
        <EmptyState
          icon={<Repeat className="h-6 w-6" aria-hidden="true" />}
          title="No recurring rules yet"
          description="Automate rent, subscriptions, and salary so they post on schedule."
          action={
            <Button
              onClick={() => {
                setEditing(null);
                setModalOpen(true);
              }}
            >
              <Plus className="h-4 w-4" aria-hidden="true" />
              New rule
            </Button>
          }
        />
      ) : (
        <>
          {error && (
            <div
              role="alert"
              className="rounded-xl border border-expense/20 bg-expense-light/60 px-4 py-3 text-sm font-medium text-expense"
            >
              {error}
            </div>
          )}

          <Card className="overflow-hidden">
            <div className="flex items-center justify-between border-b border-warm-200 px-4 py-3 sm:px-5">
              <p className="text-sm font-semibold text-stone-700">
                {rules.length} {rules.length === 1 ? 'rule' : 'rules'}
              </p>
              <p className="text-xs text-stone-400">
                {rules.filter((rule) => rule.active).length} active
              </p>
            </div>

            <ul className="divide-y divide-warm-100">
              {rules.map((rule) => {
                const categoryName =
                  categoryNames.get(rule.category_id) ?? 'Uncategorized';
                const accountName =
                  accountNames.get(rule.account_id) ?? 'Unknown account';
                const label = ruleLabel(rule, categoryName);
                const isIncome = rule.type === 'income';
                const isBusy = busyId === rule.id;

                return (
                  <li
                    key={rule.id}
                    className="flex flex-col gap-4 px-4 py-4 sm:px-5 lg:flex-row lg:items-center lg:justify-between"
                  >
                    <div className="flex min-w-0 items-start gap-3">
                      <div
                        className={cn(
                          'mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-xl',
                          isIncome
                            ? 'bg-income-light text-income'
                            : 'bg-expense-light text-expense'
                        )}
                      >
                        <Repeat className="h-5 w-5" aria-hidden="true" />
                      </div>
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <p className="truncate text-sm font-bold text-stone-900">
                            {label}
                          </p>
                          <TypeBadge type={rule.type} />
                          <span
                            className={cn(
                              'rounded-full px-2.5 py-1 text-xs font-bold',
                              rule.active
                                ? 'bg-income-light text-income'
                                : 'bg-warm-100 text-stone-500'
                            )}
                          >
                            {rule.active ? 'Active' : 'Paused'}
                          </span>
                        </div>
                        <p className="mt-1 text-xs text-stone-500">
                          {accountName} · {categoryName} ·{' '}
                          {describeFrequency(rule)}
                        </p>
                        <p className="mt-1 flex items-center gap-1 text-xs text-stone-400">
                          <CalendarClock className="h-3.5 w-3.5" aria-hidden="true" />
                          Next run {formatDate(rule.next_run)}
                        </p>
                      </div>
                    </div>

                    <div className="flex flex-wrap items-center gap-2 lg:justify-end">
                      <p
                        className={cn(
                          'mr-auto text-sm font-extrabold lg:mr-2',
                          isIncome ? 'text-income' : 'text-expense'
                        )}
                      >
                        {isIncome ? '+' : '−'}
                        {formatCurrency(rule.amount, currency)}
                      </p>
                      <Button
                        variant="secondary"
                        size="sm"
                        disabled={isBusy}
                        onClick={() => void handleToggle(rule)}
                      >
                        {rule.active ? (
                          <Pause className="h-4 w-4" aria-hidden="true" />
                        ) : (
                          <Play className="h-4 w-4" aria-hidden="true" />
                        )}
                        {rule.active ? 'Pause' : 'Resume'}
                      </Button>
                      <Button
                        variant="secondary"
                        size="sm"
                        disabled={isBusy}
                        onClick={() => void handleRunNow(rule)}
                      >
                        {isBusy ? (
                          <Loader2
                            className="h-4 w-4 animate-spin"
                            aria-hidden="true"
                          />
                        ) : null}
                        Run now
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        aria-label={`Edit ${label}`}
                        onClick={() => {
                          setEditing(rule);
                          setModalOpen(true);
                        }}
                      >
                        <Pencil className="h-4 w-4" aria-hidden="true" />
                        Edit
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        aria-label={`Delete ${label}`}
                        onClick={() => {
                          setDeleteError(null);
                          setDeleteTarget(rule);
                        }}
                        className="text-stone-400 hover:bg-expense-light hover:text-expense"
                      >
                        <Trash2 className="h-4 w-4" aria-hidden="true" />
                      </Button>
                    </div>
                  </li>
                );
              })}
            </ul>
          </Card>
        </>
      )}

      <RecurringFormModal
        open={modalOpen}
        onClose={() => {
          setModalOpen(false);
          setEditing(null);
        }}
        accounts={accounts}
        categories={categories}
        rule={editing}
        onCreate={handleCreate}
        onUpdate={handleUpdate}
      />

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Delete recurring rule?"
        description={
          deleteTarget
            ? `“${ruleLabel(
                deleteTarget,
                categoryNames.get(deleteTarget.category_id) ?? 'this rule'
              )}” will stop posting. Transactions it already created stay put.`
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

export default Recurring;
