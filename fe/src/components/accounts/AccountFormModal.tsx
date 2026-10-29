import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Loader2 } from 'lucide-react';
import type { CreateAccountRequest } from '../../types';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { Select } from '../ui/Select';
import { errorClass, inputClass, labelClass } from '../ui/formStyles';

const accountSchema = z.object({
  name: z.string().min(1, 'Account name is required').max(80, 'Keep it short'),
  balance: z
    .string()
    .min(1, 'Enter a starting balance')
    .refine(
      (value) => Number.isFinite(Number(value)) && Number(value) >= 0,
      'Enter a valid amount'
    ),
  currency: z.string().min(1, 'Choose a currency'),
});

type AccountFormData = z.infer<typeof accountSchema>;

const currencies = ['USD', 'EUR', 'GBP', 'IDR', 'SGD', 'JPY', 'AUD'];

interface AccountFormModalProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (data: CreateAccountRequest) => Promise<void>;
}

export function AccountFormModal({
  open,
  onClose,
  onSubmit,
}: AccountFormModalProps) {
  const [submitError, setSubmitError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<AccountFormData>({
    resolver: zodResolver(accountSchema),
    defaultValues: { name: '', balance: '0', currency: 'USD' },
  });

  useEffect(() => {
    if (!open) return;
    reset({ name: '', balance: '0', currency: 'USD' });
  }, [open, reset]);

  const handleClose = () => {
    setSubmitError(null);
    onClose();
  };

  const submit = async (data: AccountFormData) => {
    setSubmitError(null);

    try {
      await onSubmit({
        name: data.name.trim(),
        balance: Number(data.balance),
        currency: data.currency,
      });
      handleClose();
    } catch (error) {
      setSubmitError(
        error instanceof Error ? error.message : 'Unable to create account'
      );
    }
  };

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title="Add account"
      description="Track a wallet, bank account, or cash balance."
      size="sm"
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
          <label htmlFor="account-name" className={labelClass}>
            Name
          </label>
          <input
            id="account-name"
            type="text"
            placeholder="e.g. Everyday checking"
            aria-invalid={errors.name ? 'true' : 'false'}
            aria-describedby={errors.name ? 'account-name-error' : undefined}
            {...register('name')}
            className={inputClass}
          />
          {errors.name && (
            <p id="account-name-error" className={errorClass}>
              {errors.name.message}
            </p>
          )}
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="space-y-1.5">
            <label htmlFor="account-balance" className={labelClass}>
              Starting balance
            </label>
            <input
              id="account-balance"
              type="number"
              min="0"
              step="0.01"
              inputMode="decimal"
              aria-invalid={errors.balance ? 'true' : 'false'}
              aria-describedby={
                errors.balance ? 'account-balance-error' : undefined
              }
              {...register('balance')}
              className={inputClass}
            />
            {errors.balance && (
              <p id="account-balance-error" className={errorClass}>
                {errors.balance.message}
              </p>
            )}
          </div>

          <div className="space-y-1.5">
            <label htmlFor="account-currency" className={labelClass}>
              Currency
            </label>
            <Select id="account-currency" {...register('currency')}>
              {currencies.map((currency) => (
                <option key={currency} value={currency}>
                  {currency}
                </option>
              ))}
            </Select>
          </div>
        </div>

        <div className="flex flex-col-reverse gap-2 pt-2 sm:flex-row sm:justify-end">
          <Button variant="secondary" onClick={handleClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                Creating…
              </>
            ) : (
              'Create account'
            )}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
