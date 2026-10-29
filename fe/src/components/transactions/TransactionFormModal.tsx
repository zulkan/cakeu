import { useEffect, useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { ArrowDownLeft, ArrowUpRight, Loader2 } from 'lucide-react';
import type { Account, Category, CreateTransactionRequest } from '../../types';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { Select } from '../ui/Select';
import { errorClass, inputClass, labelClass } from '../ui/formStyles';
import { cn } from '../../lib/cn';

const transactionSchema = z.object({
  type: z.enum(['income', 'expense']),
  account_id: z.string().min(1, 'Select an account'),
  category_id: z.string().min(1, 'Select a category'),
  amount: z
    .string()
    .min(1, 'Enter an amount')
    .refine((value) => Number(value) > 0, 'Enter an amount greater than 0'),
  description: z
    .string()
    .min(1, 'Add a short description')
    .max(200, 'Keep it under 200 characters'),
  date: z.string().regex(/^\d{4}-\d{2}-\d{2}$/, 'Choose a date'),
});

type TransactionFormData = z.infer<typeof transactionSchema>;

interface TransactionFormModalProps {
  open: boolean;
  onClose: () => void;
  accounts: Account[];
  categories: Category[];
  onSubmit: (data: CreateTransactionRequest) => Promise<void>;
}

function todayValue(): string {
  return new Date().toISOString().slice(0, 10);
}

export function TransactionFormModal({
  open,
  onClose,
  accounts,
  categories,
  onSubmit,
}: TransactionFormModalProps) {
  const [submitError, setSubmitError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    control,
    setValue,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<TransactionFormData>({
    resolver: zodResolver(transactionSchema),
    defaultValues: {
      type: 'expense',
      account_id: '',
      category_id: '',
      amount: '',
      description: '',
      date: todayValue(),
    },
  });

  const type = useWatch({ control, name: 'type' });
  const filteredCategories = categories.filter(
    (category) => category.type === type
  );

  useEffect(() => {
    if (!open) return;

    reset({
      type: 'expense',
      account_id: accounts[0]?.id ?? '',
      category_id: '',
      amount: '',
      description: '',
      date: todayValue(),
    });
  }, [open, accounts, reset]);

  useEffect(() => {
    setValue('category_id', '');
  }, [type, setValue]);

  const handleClose = () => {
    setSubmitError(null);
    onClose();
  };

  const submit = async (data: TransactionFormData) => {
    setSubmitError(null);

    try {
      await onSubmit({
        account_id: data.account_id,
        category_id: data.category_id,
        type: data.type,
        amount: Number(data.amount),
        description: data.description.trim(),
        date: new Date(`${data.date}T00:00:00Z`).toISOString(),
      });
      handleClose();
    } catch (error) {
      setSubmitError(
        error instanceof Error ? error.message : 'Unable to save transaction'
      );
    }
  };

  const noAccounts = accounts.length === 0;
  const noCategories = filteredCategories.length === 0;

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title="Add transaction"
      description="Record money coming in or going out."
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
            aria-label="Transaction type"
            className="grid grid-cols-2 gap-2"
          >
            <button
              type="button"
              aria-pressed={type === 'expense'}
              onClick={() =>
                setValue('type', 'expense', { shouldValidate: true })
              }
              className={cn(
                'flex items-center justify-center gap-2 rounded-xl border px-3 py-2.5 text-sm font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500',
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
              onClick={() =>
                setValue('type', 'income', { shouldValidate: true })
              }
              className={cn(
                'flex items-center justify-center gap-2 rounded-xl border px-3 py-2.5 text-sm font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500',
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
            <label htmlFor="transaction-account" className={labelClass}>
              Account
            </label>
            <Select
              id="transaction-account"
              disabled={noAccounts}
              aria-invalid={errors.account_id ? 'true' : 'false'}
              aria-describedby={
                errors.account_id ? 'transaction-account-error' : undefined
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
              <p id="transaction-account-error" className={errorClass}>
                {errors.account_id.message}
              </p>
            )}
          </div>

          <div className="space-y-1.5">
            <label htmlFor="transaction-category" className={labelClass}>
              Category
            </label>
            <Select
              id="transaction-category"
              disabled={noCategories}
              aria-invalid={errors.category_id ? 'true' : 'false'}
              aria-describedby={
                errors.category_id ? 'transaction-category-error' : undefined
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
              <p id="transaction-category-error" className={errorClass}>
                {errors.category_id.message}
              </p>
            )}
          </div>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="space-y-1.5">
            <label htmlFor="transaction-amount" className={labelClass}>
              Amount
            </label>
            <input
              id="transaction-amount"
              type="number"
              min="0"
              step="0.01"
              inputMode="decimal"
              placeholder="0.00"
              aria-invalid={errors.amount ? 'true' : 'false'}
              aria-describedby={
                errors.amount ? 'transaction-amount-error' : undefined
              }
              {...register('amount')}
              className={inputClass}
            />
            {errors.amount && (
              <p id="transaction-amount-error" className={errorClass}>
                {errors.amount.message}
              </p>
            )}
          </div>

          <div className="space-y-1.5">
            <label htmlFor="transaction-date" className={labelClass}>
              Date
            </label>
            <input
              id="transaction-date"
              type="date"
              aria-invalid={errors.date ? 'true' : 'false'}
              aria-describedby={
                errors.date ? 'transaction-date-error' : undefined
              }
              {...register('date')}
              className={inputClass}
            />
            {errors.date && (
              <p id="transaction-date-error" className={errorClass}>
                {errors.date.message}
              </p>
            )}
          </div>
        </div>

        <div className="space-y-1.5">
          <label htmlFor="transaction-description" className={labelClass}>
            Description
          </label>
          <input
            id="transaction-description"
            type="text"
            placeholder="e.g. Weekly groceries"
            aria-invalid={errors.description ? 'true' : 'false'}
            aria-describedby={
              errors.description ? 'transaction-description-error' : undefined
            }
            {...register('description')}
            className={inputClass}
          />
          {errors.description && (
            <p id="transaction-description-error" className={errorClass}>
              {errors.description.message}
            </p>
          )}
        </div>

        {(noAccounts || noCategories) && (
          <p className="rounded-xl bg-warm-50 px-4 py-3 text-sm text-stone-600">
            {noAccounts
              ? 'Create an account first, then come back to add a transaction.'
              : `Create a ${type} category first, then come back to add a transaction.`}
          </p>
        )}

        <div className="flex flex-col-reverse gap-2 pt-2 sm:flex-row sm:justify-end">
          <Button variant="secondary" onClick={handleClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button
            type="submit"
            disabled={isSubmitting || noAccounts || noCategories}
          >
            {isSubmitting ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                Saving…
              </>
            ) : (
              'Save transaction'
            )}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
