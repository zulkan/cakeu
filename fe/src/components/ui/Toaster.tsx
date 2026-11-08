import { useEffect } from 'react';
import { AlertCircle, CheckCircle2, Info, X } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { useToastStore } from '../../stores/toastStore';
import type { Toast, ToastTone } from '../../stores/toastStore';
import { cn } from '../../lib/cn';

const icons: Record<ToastTone, LucideIcon> = {
  success: CheckCircle2,
  error: AlertCircle,
  info: Info,
};

const tones: Record<ToastTone, { icon: string; border: string }> = {
  success: { icon: 'text-income', border: 'border-income/30' },
  error: { icon: 'text-expense', border: 'border-expense/30' },
  info: { icon: 'text-brand-600', border: 'border-brand-200' },
};

const durations: Record<ToastTone, number> = {
  success: 4_000,
  info: 4_000,
  error: 6_000,
};

function ToastCard({ toast }: { toast: Toast }) {
  const dismiss = useToastStore((state) => state.dismiss);
  const Icon = icons[toast.tone];
  const tone = tones[toast.tone];

  useEffect(() => {
    const timeoutId = setTimeout(
      () => dismiss(toast.id),
      durations[toast.tone]
    );
    return () => clearTimeout(timeoutId);
  }, [dismiss, toast.id, toast.tone]);

  return (
    <div
      role={toast.tone === 'error' ? 'alert' : undefined}
      className={cn(
        'toast-enter pointer-events-auto flex w-80 max-w-[calc(100vw-2rem)] items-start gap-3 rounded-2xl border bg-surface-card p-4 shadow-lg shadow-stone-900/10',
        tone.border
      )}
    >
      <Icon
        className={cn('mt-0.5 h-5 w-5 shrink-0', tone.icon)}
        aria-hidden="true"
      />
      <div className="min-w-0 flex-1">
        <p className="text-sm font-bold text-stone-900">{toast.title}</p>
        {toast.description && (
          <p className="mt-0.5 text-sm text-stone-600">{toast.description}</p>
        )}
      </div>
      <button
        type="button"
        onClick={() => dismiss(toast.id)}
        aria-label={`Dismiss ${toast.title}`}
        className="rounded-lg p-1 text-stone-400 transition-colors hover:bg-warm-100 hover:text-stone-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
      >
        <X className="h-4 w-4" aria-hidden="true" />
      </button>
    </div>
  );
}

export function Toaster() {
  const toasts = useToastStore((state) => state.toasts);

  return (
    <div
      role="status"
      aria-live="polite"
      className="pointer-events-none fixed bottom-4 right-4 z-[60] flex flex-col items-end gap-2"
    >
      {toasts.map((toast) => (
        <ToastCard key={toast.id} toast={toast} />
      ))}
    </div>
  );
}
