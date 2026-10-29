import type { LucideIcon } from 'lucide-react';
import { cn } from '../../lib/cn';
import { Card } from '../ui/Card';

type Tone = 'income' | 'expense' | 'brand' | 'neutral';

interface StatCardProps {
  label: string;
  value: string;
  icon: LucideIcon;
  tone: Tone;
  hint?: string;
}

const toneStyles: Record<Tone, { icon: string; value: string }> = {
  income: { icon: 'bg-income-light text-income', value: 'text-income' },
  expense: { icon: 'bg-expense-light text-expense', value: 'text-expense' },
  brand: { icon: 'bg-brand-100 text-brand-600', value: 'text-stone-900' },
  neutral: { icon: 'bg-warm-100 text-warm-700', value: 'text-stone-900' },
};

export function StatCard({
  label,
  value,
  icon: Icon,
  tone,
  hint,
}: StatCardProps) {
  const styles = toneStyles[tone];

  return (
    <Card className="p-5">
      <div className="flex items-center justify-between gap-3">
        <p className="text-sm font-semibold text-stone-500">{label}</p>
        <div
          className={cn(
            'flex h-9 w-9 items-center justify-center rounded-xl',
            styles.icon
          )}
        >
          <Icon className="h-4 w-4" aria-hidden="true" />
        </div>
      </div>
      <p
        className={cn(
          'mt-3 text-2xl font-extrabold tracking-tight',
          styles.value
        )}
      >
        {value}
      </p>
      {hint && <p className="mt-1 text-xs text-stone-400">{hint}</p>}
    </Card>
  );
}
