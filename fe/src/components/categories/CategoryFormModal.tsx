import { useEffect, useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { ArrowDownLeft, ArrowUpRight, Loader2 } from 'lucide-react';
import type { CreateCategoryRequest } from '../../types';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { errorClass, inputClass, labelClass } from '../ui/formStyles';
import { cn } from '../../lib/cn';

const categorySchema = z.object({
  name: z.string().min(1, 'Category name is required').max(60, 'Keep it short'),
  type: z.enum(['income', 'expense']),
  color: z
    .string()
    .regex(/^#[0-9A-Fa-f]{6}$/, 'Choose a valid color'),
});

type CategoryFormData = z.infer<typeof categorySchema>;

interface CategoryFormModalProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (data: CreateCategoryRequest) => Promise<void>;
}

export function CategoryFormModal({
  open,
  onClose,
  onSubmit,
}: CategoryFormModalProps) {
  const [submitError, setSubmitError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    control,
    setValue,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<CategoryFormData>({
    resolver: zodResolver(categorySchema),
    defaultValues: { name: '', type: 'expense', color: '#FB923C' },
  });

  const type = useWatch({ control, name: 'type' });
  const color = useWatch({ control, name: 'color' });

  useEffect(() => {
    if (!open) return;
    reset({ name: '', type: 'expense', color: '#FB923C' });
  }, [open, reset]);

  const handleClose = () => {
    setSubmitError(null);
    onClose();
  };

  const submit = async (data: CategoryFormData) => {
    setSubmitError(null);

    try {
      await onSubmit({
        name: data.name.trim(),
        type: data.type,
        color: data.color,
      });
      handleClose();
    } catch (error) {
      setSubmitError(
        error instanceof Error ? error.message : 'Unable to create category'
      );
    }
  };

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title="Add category"
      description="Group your income and expenses with a color."
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
          <label htmlFor="category-name" className={labelClass}>
            Name
          </label>
          <input
            id="category-name"
            type="text"
            placeholder="e.g. Groceries"
            aria-invalid={errors.name ? 'true' : 'false'}
            aria-describedby={errors.name ? 'category-name-error' : undefined}
            {...register('name')}
            className={inputClass}
          />
          {errors.name && (
            <p id="category-name-error" className={errorClass}>
              {errors.name.message}
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <span className={labelClass}>Type</span>
          <div
            role="group"
            aria-label="Category type"
            className="grid grid-cols-2 gap-2"
          >
            <button
              type="button"
              aria-pressed={type === 'expense'}
              onClick={() =>
                setValue('type', 'expense', { shouldValidate: true })
              }
              className={cn(
                'flex items-center justify-center gap-2 rounded-xl border px-3 py-2.5 text-sm font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500',
                type === 'expense'
                  ? 'border-expense/30 bg-expense-light text-expense'
                  : 'border-warm-200 bg-surface-card text-stone-500 hover:bg-warm-50'
              )}
            >
              <ArrowUpRight className="h-4 w-4" aria-hidden="true" />
              Expense
            </button>
            <button
              type="button"
              aria-pressed={type === 'income'}
              onClick={() =>
                setValue('type', 'income', { shouldValidate: true })
              }
              className={cn(
                'flex items-center justify-center gap-2 rounded-xl border px-3 py-2.5 text-sm font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500',
                type === 'income'
                  ? 'border-income/30 bg-income-light text-income'
                  : 'border-warm-200 bg-surface-card text-stone-500 hover:bg-warm-50'
              )}
            >
              <ArrowDownLeft className="h-4 w-4" aria-hidden="true" />
              Income
            </button>
          </div>
        </div>

        <div className="space-y-1.5">
          <label htmlFor="category-color" className={labelClass}>
            Color
          </label>
          <div className="flex items-center gap-3">
            <input
              id="category-color"
              type="color"
              aria-invalid={errors.color ? 'true' : 'false'}
              aria-describedby={
                errors.color ? 'category-color-error' : undefined
              }
              {...register('color')}
              className="h-11 w-14 cursor-pointer rounded-xl border border-warm-200 bg-surface-card p-1 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
            />
            <div className="flex items-center gap-2">
              <span
                className="h-8 w-8 rounded-full ring-2 ring-white"
                style={{ backgroundColor: color }}
                aria-hidden="true"
              />
              <span className="font-mono text-sm font-semibold uppercase text-stone-600">
                {color}
              </span>
            </div>
          </div>
          {errors.color && (
            <p id="category-color-error" className={errorClass}>
              {errors.color.message}
            </p>
          )}
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
              'Create category'
            )}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
