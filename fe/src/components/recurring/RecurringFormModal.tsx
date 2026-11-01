import { useEffect, useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { ArrowDownLeft, ArrowUpRight, Loader2 } from 'lucide-react';
import type {
  Account,
  Category,
  CreateRecurringRequest,
  RecurringRule,
  UpdateRecurringRequest,
} from '../../types';
import { todayISO } from '../../lib/dates';
import { cn } from '../../lib/cn';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { Select } from '../ui/Select';
import { errorClass, inputClass, labelClass } from '../ui/formStyles';

const recurringSchema = z.object({
  type: z.enum(['income', 'expense']),
  account_id: z.string().min(1, 'Select an account'),
  category_id: z.string().min(1, 'Select a category'),
  amount: z
    .string()
    .min(1, 'Enter an amount')
    .refine(
      (value) => Number.isFinite(Number(value)) && Number(value) > 0,
      'Enter an amount greater than 0'
    ),
  description: z
    .string()
    .min(1, 'Add a short description')
    .max(200, 'Keep it under 200 characters'),
  frequency: z.enum(['daily', 'weekly', 'monthly']),
  interval: z
    .string()
    .min(1, 'Enter an interval')
    .refine((value) => {
      const parsed = Number(value);
      return Number.isInteger(parsed) && parsed >= 1 && parsed <= 12;
    }, 'Use a whole number from 1 to 12'),
  next_run: z.string().regex(/^\d{4}-\d{2}-\d{2}$/, 'Choose a date'),
});

type RecurringFormData = z.infer<typeof recurringSchema>;

interface RecurringFormModalProps {
  open: boolean;
  onClose: () => void;
  accounts: Account[];
  categories: Category[];
  rule?: RecurringRule | null;
  onCreate: (data: CreateRecurringRequest) => Promise<void>;
  onUpdate: (id: string, data: UpdateRecurringRequest) => Promise<void>;
}

export function RecurringFormModal({
  open,
  onClose,
  accounts,
  categories,
  rule,
  onCreate,
  onUpdate,
}: RecurringFormModalProps) {
  const [submitError, setSubmitError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    control,
    setValue,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<RecurringFormData>({
    resolver: zodResolver(recurringSchema),
    defaultValues: {
      type: 'expense',
      account_id: '',
      category_id: '',
      amount: '',
      description: '',
      frequency: 'monthly',
      interval: '1',
      next_run: todayISO(),
    },
  });

  const type = useWatch({ control, name: 'type' });
  const filteredCategories = categories.filter(
    (category) => category.type === type
  );

  useEffect(() => {
    if (!open) return;

    if (rule) {
      reset({
        type: rule.type,
        account_id: rule.account_id,
        category_id: rule.category_id,
        amount: String(rule.amount),
        description: rule.description,
        frequency: rule.frequency,
        interval: String(rule.interval),
        next_run: rule.next_run.slice(0, 10),
      });
    } else {
      reset({
        type: 'expense',
        account_id: accounts[0]?.id ?? '',
        category_id: '',
        amount: '',
        description: '',
        frequency: 'monthly',
        interval: '1',
        next_run: todayISO(),
      });
    }
  }, [open, rule, accounts, reset]);

  useEffect(() => {
    if (rule) return;
    setValue('category_id', '');
  }, [type, rule, setValue]);

  const handleClose = () => {
    setSubmitError(null);
    onClose();
  };

  const submit = async (data: RecurringFormData) => {
    setSubmitError(null);

    const shared = {
      amount: Number(data.amount),
      description: data.description.trim(),
      frequency: data.frequency,
      interval: Number(data.interval),
      next_run: data.next_run,
    };

    try {
      if (rule) {
        await onUpdate(rule.id, shared);
      } else {
        await onCreate({
          account_id: data.account_id,
          category_id: data.category_id,
          type: data.type,
          ...shared,
        });
      }
      handleClose();
    } catch (error) {
      setSubmitError(
        error instanceof Error ? error.message : 'Unable to save recurring rule'
      );
    }
  };

  const noAccounts = accounts.length === 0;
  const noCategories = filteredCategories.length === 0;
  const editing = Boolean(rule);

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title={editing ? 'Edit recurring rule' : 'New recurring rule'}
      description={
        editing
          ? 'Adjust when and how much this rule posts.'
          : 'Automate a transaction that repeats on a schedule.'
      }
      size="lg"
    >
      <form onSubmit={handleSubmit(submit)} className="space-y-5" noValidate>
        {submitError && (
          <div
            role="alert"
            className="rounded-xl border border-expense/20 bg-expense-light/60 px-4 py-3 text-sm font-medium text-expense"
          >
            {submitError}
          </div>
        )}

        <div className="space-y-1.5">
          <span className={labelClass}>Type</span>
          <div
            role="group"
            aria-label="Recurring type"
            className="grid grid-cols-2 gap-2"
          >
            <button
              type="button"
              aria-pressed={type === 'expense'}
              disabled={editing}
              onClick={() =>
                setValue('type', 'expense', { shouldValidate: true })
              }
              className={cn(
                'flex items-center justify-center gap-2 rounded-xl border px-3 py-2.5 text-sm font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 disabled:cursor-not-allowed disabled:opacity-60',
                type === 'expense'
                  ? 'border-expense/30 bg-expense-light text-expense'
                  : 'border-warm-200 bg-surface-card text-stone-500 hover:bg-warm-50'
              )}
            >
              <ArrowUpRight className="h-4 w-4" aria-hidden="true" />
              Expense
            </button>
            <button
              type="button"
              aria-pressed={type === 'income'}
              disabled={editing}
              onClick={() =>
                setValue('type', 'income', { shouldValidate: true })
              }
              className={cn(
                'flex items-center justify-center gap-2 rounded-xl border px-3 py-2.5 text-sm font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 disabled:cursor-not-allowed disabled:opacity-60',
                type === 'income'
                  ? 'border-income/30 bg-income-light text-income'
                  : 'border-warm-200 bg-surface-card text-stone-500 hover:bg-warm-50'
              )}
            >
              <ArrowDownLeft className="h-4 w-4" aria-hidden="true" />
              Income
            </button>
          </div>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="space-y-1.5">
            <label htmlFor="recurring-account" className={labelClass}>
              Account
            </label>
            <Select
              id="recurring-account"
              disabled={editing || noAccounts}
              aria-invalid={errors.account_id ? 'true' : 'false'}
              aria-describedby={
                errors.account_id ? 'recurring-account-error' : undefined
              }
              {...register('account_id')}
            >
              <option value="">
                {noAccounts ? 'No accounts yet' : 'Select an account'}
              </option>
              {accounts.map((account) => (
                <option key={account.id} value={account.id}>
                  {account.name}
                </option>
              ))}
            </Select>
            {errors.account_id && (
              <p id="recurring-account-error" className={errorClass}>
                {errors.account_id.message}
              </p>
            )}
          </div>

          <div className="space-y-1.5">
            <label htmlFor="recurring-category" className={labelClass}>
              Category
            </label>
            <Select
              id="recurring-category"
              disabled={editing || noCategories}
              aria-invalid={errors.category_id ? 'true' : 'false'}
              aria-describedby={
                errors.category_id ? 'recurring-category-error' : undefined
              }
              {...register('category_id')}
            >
              <option value="">
                {noCategories
                  ? `No ${type} categories yet`
                  : 'Select a category'}
              </option>
              {filteredCategories.map((category) => (
                <option key={category.id} value={category.id}>
                  {category.name}
                </option>
              ))}
            </Select>
            {errors.category_id && (
              <p id="recurring-category-error" className={errorClass}>
                {errors.category_id.message}
              </p>
            )}
          </div>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="space-y-1.5">
            <label htmlFor="recurring-amount" className={labelClass}>
              Amount
            </label>
            <input
              id="recurring-amount"
              type="number"
              min="0"
              step="0.01"
              inputMode="decimal"
              placeholder="0.00"
              aria-invalid={errors.amount ? 'true' : 'false'}
              aria-describedby={
                errors.amount ? 'recurring-amount-error' : undefined
              }
              {...register('amount')}
              className={inputClass}
            />
            {errors.amount && (
              <p id="recurring-amount-error" className={errorClass}>
                {errors.amount.message}
              </p>
            )}
          </div>

          <div className="space-y-1.5">
            <label htmlFor="recurring-description" className={labelClass}>
              Description
            </label>
            <input
              id="recurring-description"
              type="text"
              placeholder="e.g. Rent"
              aria-invalid={errors.description ? 'true' : 'false'}
              aria-describedby={
                errors.description ? 'recurring-description-error' : undefined
              }
              {...register('description')}
              className={inputClass}
            />
            {errors.description && (
              <p id="recurring-description-error" className={errorClass}>
                {errors.description.message}
              </p>
            )}
          </div>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <div className="space-y-1.5">
            <label htmlFor="recurring-frequency" className={labelClass}>
              Frequency
            </label>
            <Select id="recurring-frequency" {...register('frequency')}>
              <option value="daily">Daily</option>
              <option value="weekly">Weekly</option>
              <option value="monthly">Monthly</option>
            </Select>
          </div>

          <div className="space-y-1.5">
            <label htmlFor="recurring-interval" className={labelClass}>
              Every
            </label>
            <input
              id="recurring-interval"
              type="number"
              min="1"
              max="12"
              step="1"
              inputMode="numeric"
              aria-invalid={errors.interval ? 'true' : 'false'}
              aria-describedby={
                errors.interval ? 'recurring-interval-error' : undefined
              }
              {...register('interval')}
              className={inputClass}
            />
            {errors.interval && (
              <p id="recurring-interval-error" className={errorClass}>
                {errors.interval.message}
              </p>
            )}
          </div>

          <div className="space-y-1.5">
            <label htmlFor="recurring-next-run" className={labelClass}>
              Next run
            </label>
            <input
              id="recurring-next-run"
              type="date"
              aria-invalid={errors.next_run ? 'true' : 'false'}
              aria-describedby={
                errors.next_run ? 'recurring-next-run-error' : undefined
              }
              {...register('next_run')}
              className={inputClass}
            />
            {errors.next_run && (
              <p id="recurring-next-run-error" className={errorClass}>
                {errors.next_run.message}
              </p>
            )}
          </div>
        </div>

        {editing && (
          <p className="rounded-xl bg-warm-50 px-4 py-3 text-sm text-stone-600">
            Account, category, and type are fixed after a rule is created.
          </p>
        )}

        {(noAccounts || noCategories) && !editing && (
          <p className="rounded-xl bg-warm-50 px-4 py-3 text-sm text-stone-600">
            {noAccounts
              ? 'Create an account first, then come back to schedule a rule.'
              : `Create a ${type} category first, then come back to schedule a rule.`}
          </p>
        )}

        <div className="flex flex-col-reverse gap-2 pt-2 sm:flex-row sm:justify-end">
          <Button variant="secondary" onClick={handleClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button
            type="submit"
            disabled={isSubmitting || (!editing && (noAccounts || noCategories))}
          >
            {isSubmitting ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                Saving…
              </>
            ) : editing ? (
              'Save changes'
            ) : (
              'Create rule'
            )}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
