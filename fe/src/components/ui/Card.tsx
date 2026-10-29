import type { ComponentProps } from 'react';
import { cn } from '../../lib/cn';

export function Card({ className, ...props }: ComponentProps<'div'>) {
  return (
    <div
      className={cn(
        'rounded-2xl border border-warm-200/80 bg-surface-card shadow-[0_1px_2px_rgba(120,53,15,0.04),0_12px_32px_-24px_rgba(120,53,15,0.35)]',
        className
      )}
      {...props}
    />
  );
}
