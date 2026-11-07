import { useState } from 'react';
import { Loader2 } from 'lucide-react';
import type { Account, AdjustAccountRequest } from '../../types';
import { formatCurrency } from '../../lib/format';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { inputClass, labelClass } from '../ui/formStyles';

interface AdjustBalanceModalProps {
  open: boolean;
  account: Account;
  onClose: () => void;
  onSubmit: (accountId: string, data: AdjustAccountRequest) => Promise<void>;
}

export function AdjustBalanceModal({
  open,
  account,
  onClose,
  onSubmit,
}: AdjustBalanceModalProps) {
  const [target, setTarget] = useState(() => String(account.balance));
  const [description, setDescription] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const targetBalance = Number(target);
  const delta = Number.isFinite(targetBalance)
    ? Math.round((targetBalance - account.balance) * 100) / 100
    : 0;

  const submit = async () => {
    if (!Number.isFinite(targetBalance)) {
      setError('Enter a valid balance');
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      await onSubmit(account.id, {
        target_balance: targetBalance,
        description: description.trim() || undefined,
      });
      onClose();
    } catch (submitError) {
      setError(
        submitError instanceof Error
          ? submitError.message
          : 'Unable to adjust balance'
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Adjust balance"
      description="Record a correction so this account matches the real world."
      size="sm"
    >
      <div className="space-y-5">
        {error && (
          <div
            role="alert"
            className="rounded-xl border border-expense/20 bg-expense-light/60 px-4 py-3 text-sm font-medium text-expense"
          >
            {error}
          </div>
        )}

        <div className="rounded-xl bg-warm-50 px-4 py-3">
          <p className="text-xs font-semibold uppercase tracking-wide text-stone-400">
            Current balance
          </p>
          <p className="mt-1 text-2xl font-extrabold text-stone-900">
            {formatCurrency(account.balance, account.currency)}
          </p>
        </div>

        <div className="space-y-1.5">
          <label htmlFor="adjust-target" className={labelClass}>
            Actual balance
          </label>
          <input
            id="adjust-target"
            type="number"
            step="0.01"
            inputMode="decimal"
            value={target}
            onChange={(event) => setTarget(event.target.value)}
            className={inputClass}
          />
          <p className="text-xs text-stone-400">
            {delta === 0
              ? 'No change will be recorded.'
              : `This records a ${delta > 0 ? 'positive' : 'negative'} adjustment of ${formatCurrency(Math.abs(delta), account.currency)}.`}
          </p>
        </div>

        <div className="space-y-1.5">
          <label htmlFor="adjust-description" className={labelClass}>
            Note (optional)
          </label>
          <input
            id="adjust-description"
            type="text"
            placeholder="e.g. Matched bank statement"
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            className={inputClass}
          />
        </div>

        <div className="flex flex-col-reverse gap-2 pt-2 sm:flex-row sm:justify-end">
          <Button variant="secondary" onClick={onClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button onClick={() => void submit()} disabled={isSubmitting}>
            {isSubmitting ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                Saving…
              </>
            ) : (
              'Record adjustment'
            )}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
