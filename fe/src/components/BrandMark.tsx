import { Wallet } from 'lucide-react';
import { cn } from '../lib/cn';

interface BrandMarkProps {
  tone?: 'dark' | 'light';
  className?: string;
}

export function BrandMark({ tone = 'dark', className }: BrandMarkProps) {
  const isLight = tone === 'light';

  return (
    <div className={cn('flex items-center gap-3', className)}>
      <div
        className={cn(
          'flex h-11 w-11 items-center justify-center rounded-2xl shadow-lg',
          isLight
            ? 'bg-white/15 text-white shadow-stone-950/10 ring-1 ring-white/25'
            : 'bg-gradient-to-br from-brand-400 to-brand-600 text-white shadow-brand-500/25'
        )}
      >
        <Wallet className="h-5 w-5" aria-hidden="true" />
      </div>
      <div>
        <p
          className={cn(
            'text-lg font-extrabold tracking-tight',
            isLight ? 'text-white' : 'text-stone-900'
          )}
        >
          Catatku
        </p>
        <p
          className={cn(
            'text-[11px] font-semibold uppercase tracking-[0.16em]',
            isLight ? 'text-white/75' : 'text-brand-600'
          )}
        >
          Pencatatan Keuangan
        </p>
      </div>
    </div>
  );
}
