import type { ReactNode } from 'react';
import { AlertCircle, Inbox, Loader2 } from 'lucide-react';
import { Button } from './Button';

export function LoadingState({ label = 'Loading…' }: { label?: string }) {
  return (
    <div
      role="status"
      className="flex flex-col items-center justify-center gap-3 rounded-2xl border border-warm-200/70 bg-surface-card px-6 py-16 text-center"
    >
      <Loader2
        className="h-6 w-6 animate-spin text-brand-500"
        aria-hidden="true"
      />
      <p className="text-sm font-medium text-stone-500">{label}</p>
    </div>
  );
}

export function ErrorState({
  message,
  onRetry,
}: {
  message: string;
  onRetry?: () => void;
}) {
  return (
    <div
      role="alert"
      className="flex flex-col items-center justify-center gap-3 rounded-2xl border border-expense/20 bg-expense-light/40 px-6 py-12 text-center"
    >
      <AlertCircle className="h-6 w-6 text-expense" aria-hidden="true" />
      <p className="max-w-md text-sm font-medium text-stone-700">{message}</p>
      {onRetry && (
        <Button variant="secondary" size="sm" onClick={onRetry}>
          Try again
        </Button>
      )}
    </div>
  );
}

export function EmptyState({
  icon,
  title,
  description,
  action,
}: {
  icon?: ReactNode;
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 rounded-2xl border border-dashed border-warm-300 bg-warm-50/60 px-6 py-14 text-center">
      <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-brand-100 text-brand-600">
        {icon ?? <Inbox className="h-6 w-6" aria-hidden="true" />}
      </div>
      <h3 className="text-base font-bold text-stone-900">{title}</h3>
      <p className="max-w-sm text-sm text-stone-500">{description}</p>
      {action && <div className="mt-2">{action}</div>}
    </div>
  );
}
