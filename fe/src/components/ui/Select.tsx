import type { ComponentProps } from 'react';
import { ChevronDown } from 'lucide-react';
import { cn } from '../../lib/cn';
import { selectClass } from './formStyles';

export function Select({
  className,
  children,
  ...props
}: ComponentProps<'select'>) {
  return (
    <div className="relative">
      <select className={cn(selectClass, className)} {...props}>
        {children}
      </select>
      <ChevronDown
        className="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-stone-400"
        aria-hidden="true"
      />
    </div>
  );
}
