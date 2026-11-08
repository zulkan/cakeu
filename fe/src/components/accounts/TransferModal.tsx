import { useEffect, useMemo, useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { ArrowLeftRight, Loader2 } from 'lucide-react';
import type { Account, CreateTransferRequest } from '../../types';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { Select } from '../ui/Select';
import {
  errorClass,
  hintClass,
  inputClass,
  labelClass,
} from '../ui/formStyles';

const transferSchema = z.object({
  from_account_id: z.string().min(1, 'Select a source account'),
  to_account_id: z.string().min(1, 'Select a destination account'),
  amount: z
    .string()
    .min(1, 'Enter an amount')
    .refine((value) => Number(value) > 0, 'Enter an amount greater than 0')
    .refine(
      (value) => /^\d+(\.\d{1,2})?$/.test(value),
      'Use no more than 2 decimal places'
    ),
  date: z.string().regex(/^\d{4}-\d{2}-\d{2}$/, 'Choose a date'),
  description: z.string().max(200, 'Keep it under 200 characters'),
});

type TransferFormData = z.infer<typeof transferSchema>;

interface TransferModalProps {
  open: boolean;
  onClose: () => void;
  accounts: Account[];
  onSubmit: (data: CreateTransferRequest) => Promise<void>;
}

function todayValue(): string {
  return new Date().toISOString().slice(0, 10);
}

function validTargets(accounts: Account[], fromId: string): Account[] {
  const from = accounts.find((account) => account.id === fromId);
  if (!from) return [];

  return accounts.filter(
    (account) =>
      account.id !== from.id && account.currency === from.currency
  );
}

function defaultFrom(accounts: Account[]): string {
  const candidate = accounts.find(
    (account) => validTargets(accounts, account.id).length > 0
  );
  return candidate?.id ?? accounts[0]?.id ?? '';
}

function formValues(accounts: Account[]): TransferFormData {
  const from = defaultFrom(accounts);

  return {
    from_account_id: from,
    to_account_id: validTargets(accounts, from)[0]?.id ?? '',
    amount: '',
    date: todayValue(),
    description: '',
  };
}

export function TransferModal({
  open,
  onClose,
  accounts,
  onSubmit,
}: TransferModalProps) {
  const [submitError, setSubmitError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    control,
    setValue,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<TransferFormData>({
    resolver: zodResolver(transferSchema),
    defaultValues: formValues(accounts),
  });

  const fromId = useWatch({ control, name: 'from_account_id' });
  const toId = useWatch({ control, name: 'to_account_id' });
  const targetOptions = useMemo(
    () => validTargets(accounts, fromId),
    [accounts, fromId]
  );
  const selectedCurrency = accounts.find(
    (account) => account.id === fromId
  )?.currency;
  const noTargets = targetOptions.length === 0;

  useEffect(() => {
    if (!open) return;
    reset(formValues(accounts));
  }, [open, accounts, reset]);

  useEffect(() => {
    if (!open) return;
    if (targetOptions.some((account) => account.id === toId)) return;
    setValue('to_account_id', targetOptions[0]?.id ?? '', {
      shouldValidate: true,
    });
  }, [open, targetOptions, toId, setValue]);

  const handleClose = () => {
    setSubmitError(null);
    onClose();
  };

  const submit = async (data: TransferFormData) => {
    setSubmitError(null);

    try {
      await onSubmit({
        from_account_id: data.from_account_id,
        to_account_id: data.to_account_id,
        amount: Number(data.amount),
        date: data.date,
        description: data.description.trim() || undefined,
      });
      handleClose();
    } catch (error) {
      setSubmitError(
        error instanceof Error ? error.message : 'Unable to transfer funds'
      );
    }
  };

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title="Transfer between accounts"
      description="Move money between two accounts without changing your total balance."
      size="md"
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

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="space-y-1.5">
            <label htmlFor="transfer-from" className={labelClass}>
              From account
            </label>
            <Select
              id="transfer-from"
              aria-invalid={errors.from_account_id ? 'true' : 'false'}
              aria-describedby={
                errors.from_account_id ? 'transfer-from-error' : undefined
              }
              {...register('from_account_id')}
            >
              <option value="">Select an account</option>
              {accounts.map((account) => (
                <option key={account.id} value={account.id}>
                  {account.name} ({account.currency})
                </option>
              ))}
            </Select>
            {errors.from_account_id && (
              <p id="transfer-from-error" className={errorClass}>
                {errors.from_account_id.message}
              </p>
            )}
          </div>

          <div className="space-y-1.5">
            <label htmlFor="transfer-to" className={labelClass}>
              To account
            </label>
            <Select
              id="transfer-to"
              disabled={noTargets}
              aria-invalid={errors.to_account_id ? 'true' : 'false'}
              aria-describedby={
                errors.to_account_id
                  ? 'transfer-to-error'
                  : noTargets
                    ? 'transfer-to-hint'
                    : undefined
              }
              {...register('to_account_id')}
            >
              <option value="">
                {noTargets
                  ? 'No matching accounts'
                  : 'Select a destination'}
              </option>
              {targetOptions.map((account) => (
                <option key={account.id} value={account.id}>
                  {account.name}
                </option>
              ))}
            </Select>
            {errors.to_account_id ? (
              <p id="transfer-to-error" className={errorClass}>
                {errors.to_account_id.message}
              </p>
            ) : noTargets ? (
              <p id="transfer-to-hint" className={hintClass}>
                No other account shares this currency.
              </p>
            ) : null}
          </div>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="space-y-1.5">
            <label htmlFor="transfer-amount" className={labelClass}>
              {selectedCurrency ? `Amount (${selectedCurrency})` : 'Amount'}
            </label>
            <input
              id="transfer-amount"
              type="number"
              min="0"
              step="0.01"
              inputMode="decimal"
              placeholder="0.00"
              aria-invalid={errors.amount ? 'true' : 'false'}
              aria-describedby={
                errors.amount ? 'transfer-amount-error' : undefined
              }
              {...register('amount')}
              className={inputClass}
            />
            {errors.amount && (
              <p id="transfer-amount-error" className={errorClass}>
                {errors.amount.message}
              </p>
            )}
          </div>

          <div className="space-y-1.5">
            <label htmlFor="transfer-date" className={labelClass}>
              Date
            </label>
            <input
              id="transfer-date"
              type="date"
              aria-invalid={errors.date ? 'true' : 'false'}
              aria-describedby={
                errors.date ? 'transfer-date-error' : undefined
              }
              {...register('date')}
              className={inputClass}
            />
            {errors.date && (
              <p id="transfer-date-error" className={errorClass}>
                {errors.date.message}
              </p>
            )}
          </div>
        </div>

        <div className="space-y-1.5">
          <label htmlFor="transfer-description" className={labelClass}>
            Description (optional)
          </label>
          <input
            id="transfer-description"
            type="text"
            placeholder="e.g. Monthly savings"
            aria-invalid={errors.description ? 'true' : 'false'}
            aria-describedby={
              errors.description ? 'transfer-description-error' : undefined
            }
            {...register('description')}
            className={inputClass}
          />
          {errors.description && (
            <p id="transfer-description-error" className={errorClass}>
              {errors.description.message}
            </p>
          )}
        </div>

        <p className="rounded-xl bg-warm-50 px-4 py-3 text-sm text-stone-600">
          Both sides are recorded and each account balance updates immediately.
        </p>

        <div className="flex flex-col-reverse gap-2 pt-2 sm:flex-row sm:justify-end">
          <Button variant="secondary" onClick={handleClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button type="submit" disabled={isSubmitting || noTargets}>
            {isSubmitting ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                Transferring…
              </>
            ) : (
              <>
                <ArrowLeftRight className="h-4 w-4" aria-hidden="true" />
                Transfer funds
              </>
            )}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
