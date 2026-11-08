import { useState } from 'react';
import {
  Check,
  Lock,
  Pencil,
  SlidersHorizontal,
  Trash2,
  Wallet,
  X,
} from 'lucide-react';
import type { Account, UpdateAccountRequest } from '../../types';
import { formatCurrency, formatDate } from '../../lib/format';
import { Card } from '../ui/Card';
import { Button } from '../ui/Button';
import { errorClass, inputClass, labelClass } from '../ui/formStyles';

interface AccountCardProps {
  account: Account;
  onUpdate: (id: string, data: UpdateAccountRequest) => Promise<void>;
  onAdjust: (account: Account) => void;
  onDelete: (account: Account) => void;
}

export function AccountCard({
  account,
  onUpdate,
  onAdjust,
  onDelete,
}: AccountCardProps) {
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(account.name);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const startEditing = () => {
    setName(account.name);
    setError(null);
    setEditing(true);
  };

  const cancelEditing = () => {
    setEditing(false);
    setError(null);
  };

  const save = async () => {
    const trimmedName = name.trim();

    if (!trimmedName) {
      setError('Account name is required');
      return;
    }

    setIsSaving(true);
    setError(null);

    try {
      await onUpdate(account.id, { name: trimmedName });
      setEditing(false);
    } catch (updateError) {
      setError(
        updateError instanceof Error
          ? updateError.message
          : 'Unable to update account'
      );
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <Card className="flex flex-col p-5">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-brand-100 text-brand-600">
            <Wallet className="h-5 w-5" aria-hidden="true" />
          </div>
          <div className="min-w-0">
            <p className="truncate text-sm font-bold text-stone-900">
              {account.name}
            </p>
            <p className="text-xs text-stone-400">
              Added {formatDate(account.created_at)}
            </p>
          </div>
        </div>
        <span
          className="inline-flex items-center gap-1 rounded-full bg-warm-100 px-2.5 py-1 text-xs font-bold text-warm-800"
          title={
            account.entry_count > 0
              ? 'Currency is locked after the first entry'
              : undefined
          }
        >
          {account.currency}
          {account.entry_count > 0 && (
            <Lock className="h-3 w-3" aria-hidden="true" />
          )}
        </span>
      </div>

      {editing ? (
        <div className="mt-5 space-y-3">
          <div className="space-y-1">
            <label htmlFor={`account-name-${account.id}`} className={labelClass}>
              Name
            </label>
            <input
              id={`account-name-${account.id}`}
              type="text"
              value={name}
              onChange={(event) => setName(event.target.value)}
              className={inputClass}
            />
          </div>
          {error && <p className={errorClass}>{error}</p>}
          <div className="flex gap-2">
            <Button size="sm" onClick={() => void save()} disabled={isSaving}>
              <Check className="h-4 w-4" aria-hidden="true" />
              {isSaving ? 'Saving…' : 'Save'}
            </Button>
            <Button
              size="sm"
              variant="secondary"
              onClick={cancelEditing}
              disabled={isSaving}
            >
              <X className="h-4 w-4" aria-hidden="true" />
              Cancel
            </Button>
          </div>
        </div>
      ) : (
        <>
          <p className="mt-5 text-3xl font-extrabold tracking-tight text-stone-900">
            {formatCurrency(account.balance, account.currency)}
          </p>
          <p className="mt-1 text-xs text-stone-400">Current balance</p>
          <p className="mt-1 text-xs text-stone-400">
            Opened with {formatCurrency(account.opening_balance, account.currency)}
          </p>

          <div className="mt-5 flex flex-wrap gap-2 border-t border-warm-100 pt-4">
            <Button size="sm" variant="secondary" onClick={startEditing}>
              <Pencil className="h-4 w-4" aria-hidden="true" />
              Edit
            </Button>
            <Button
              size="sm"
              variant="secondary"
              onClick={() => onAdjust(account)}
            >
              <SlidersHorizontal className="h-4 w-4" aria-hidden="true" />
              Adjust balance
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => onDelete(account)}
              aria-label={`Delete ${account.name}`}
              className="text-stone-500 hover:bg-expense-light hover:text-expense"
            >
              <Trash2 className="h-4 w-4" aria-hidden="true" />
              Delete
            </Button>
          </div>
        </>
      )}
    </Card>
  );
}
