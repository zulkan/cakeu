import { useEffect, useMemo, useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Loader2 } from 'lucide-react';
import type { BudgetStatus, Category } from '../../types';
import { useBudgetStore } from '../../stores/budgetStore';
import { formatMonthLabel } from '../../lib/dates';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { Select } from '../ui/Select';
import { errorClass, inputClass, labelClass } from '../ui/formStyles';

const budgetSchema = z.object({
  category_id: z.string().min(1, 'Choose a category'),
  amount: z
    .string()
    .min(1, 'Enter an amount')
    .refine(
      (value) => Number.isFinite(Number(value)) && Number(value) > 0,
      'Enter an amount greater than 0'
    ),
});

type BudgetFormData = z.infer<typeof budgetSchema>;

interface BudgetEditorModalProps {
  open: boolean;
  onClose: () => void;
  categories: Category[];
  budgets: BudgetStatus[];
  month: string;
}

export function BudgetEditorModal({
  open,
  onClose,
  categories,
  budgets,
  month,
}: BudgetEditorModalProps) {
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [isRemoving, setIsRemoving] = useState(false);
  const upsertBudget = useBudgetStore((state) => state.upsertBudget);
  const deleteBudget = useBudgetStore((state) => state.deleteBudget);
  const isSaving = useBudgetStore((state) => state.isSaving);

  const expenseCategories = useMemo(
    () => categories.filter((category) => category.type === 'expense'),
    [categories]
  );
  const {
    register,
    handleSubmit,
    reset,
    setValue,
    control,
    formState: { errors, isSubmitting },
  } = useForm<BudgetFormData>({
    resolver: zodResolver(budgetSchema),
    defaultValues: { category_id: '', amount: '' },
  });

  const categoryId = useWatch({ control, name: 'category_id' });
  const existing = budgets.find(
    (budget) => budget.category_id === categoryId
  );

  useEffect(() => {
    if (!open) return;

    const defaultCategory =
      budgets[0]?.category_id ?? expenseCategories[0]?.id ?? '';
    const defaultBudget = budgets.find(
      (budget) => budget.category_id === defaultCategory
    );

    reset({
      category_id: defaultCategory,
      amount: defaultBudget ? String(defaultBudget.budget) : '',
    });
  }, [open, budgets, expenseCategories, reset]);

  const handleCategoryChange = (value: string) => {
    setValue('category_id', value, { shouldValidate: true });
    const budget = budgets.find((item) => item.category_id === value);
    setValue('amount', budget ? String(budget.budget) : '');
  };

  const handleClose = () => {
    setSubmitError(null);
    onClose();
  };

  const submit = async (data: BudgetFormData) => {
    setSubmitError(null);

    try {
      await upsertBudget(data.category_id, Number(data.amount));
      handleClose();
    } catch (error) {
      setSubmitError(
        error instanceof Error ? error.message : 'Unable to save budget'
      );
    }
  };

  const handleRemove = async () => {
    if (!existing) return;

    setSubmitError(null);
    setIsRemoving(true);

    try {
      await deleteBudget(existing.category_id);
      handleClose();
    } catch (error) {
      setSubmitError(
        error instanceof Error ? error.message : 'Unable to remove budget'
      );
    } finally {
      setIsRemoving(false);
    }
  };

  const busy = isSubmitting || isSaving || isRemoving;
  const noCategories = expenseCategories.length === 0;

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title="Monthly budgets"
      description={`Set a spending limit per expense category for ${formatMonthLabel(month)}.`}
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
          <label htmlFor="budget-category" className={labelClass}>
            Category
          </label>
          <Select
            id="budget-category"
            value={categoryId}
            disabled={noCategories}
            onChange={(event) => handleCategoryChange(event.target.value)}
          >
            <option value="">
              {noCategories ? 'No expense categories yet' : 'Choose a category'}
            </option>
            {expenseCategories.map((category) => (
              <option key={category.id} value={category.id}>
                {category.name}
              </option>
            ))}
          </Select>
          {errors.category_id && (
            <p className={errorClass}>{errors.category_id.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label htmlFor="budget-amount" className={labelClass}>
            Monthly limit
          </label>
          <input
            id="budget-amount"
            type="number"
            min="0"
            step="0.01"
            inputMode="decimal"
            placeholder="0.00"
            aria-invalid={errors.amount ? 'true' : 'false'}
            aria-describedby={errors.amount ? 'budget-amount-error' : undefined}
            {...register('amount')}
            className={inputClass}
          />
          {errors.amount && (
            <p id="budget-amount-error" className={errorClass}>
              {errors.amount.message}
            </p>
          )}
          {existing && (
            <p className="text-xs text-stone-500">
              Currently {existing.percent.toFixed(0)}% used ·{' '}
              {existing.remaining >= 0
                ? 'under budget'
                : 'over budget'}
            </p>
          )}
        </div>

        {noCategories && (
          <p className="rounded-xl bg-warm-50 px-4 py-3 text-sm text-stone-600">
            Create an expense category first, then come back to set a budget.
          </p>
        )}

        <div className="flex flex-col-reverse gap-2 pt-2 sm:flex-row sm:items-center sm:justify-between">
          <div>
            {existing && (
              <Button
                variant="danger"
                size="sm"
                disabled={busy}
                onClick={() => void handleRemove()}
              >
                {isRemoving ? (
                  <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                ) : null}
                Remove budget
              </Button>
            )}
          </div>
          <div className="flex flex-col-reverse gap-2 sm:flex-row">
            <Button variant="secondary" onClick={handleClose} disabled={busy}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy || noCategories}>
              {busy && !isRemoving ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                  Saving…
                </>
              ) : (
                'Save budget'
              )}
            </Button>
          </div>
        </div>
      </form>
    </Modal>
  );
}
