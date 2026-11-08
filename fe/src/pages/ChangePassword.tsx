import { useState } from 'react';
import {
  ArrowLeft,
  Eye,
  EyeOff,
  KeyRound,
  Loader2,
} from 'lucide-react';
import { useForm } from 'react-hook-form';
import type { UseFormRegisterReturn } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Link } from 'react-router-dom';
import { useAuthStore } from '../stores/authStore';
import { useToastStore } from '../stores/toastStore';
import { PageHeader } from '../components/ui/PageHeader';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import {
  errorClass,
  inputClass,
  labelClass,
  secondaryLinkClass,
} from '../components/ui/formStyles';

const MAX_PASSWORD_BYTES = 72;

const changePasswordSchema = z
  .object({
    currentPassword: z.string().min(1, 'Current password is required'),
    newPassword: z
      .string()
      .min(1, 'New password is required')
      .min(8, 'Password must be at least 8 characters')
      .refine(
        (value) =>
          new globalThis.TextEncoder().encode(value).length <=
          MAX_PASSWORD_BYTES,
        { message: 'Password must be at most 72 bytes' }
      ),
    confirmPassword: z.string().min(1, 'Please confirm your new password'),
  })
  .refine((data) => data.newPassword === data.confirmPassword, {
    message: 'Passwords do not match',
    path: ['confirmPassword'],
  });

type ChangePasswordFormData = z.infer<typeof changePasswordSchema>;

interface PasswordFieldProps {
  id: keyof ChangePasswordFormData;
  label: string;
  autoComplete: string;
  placeholder: string;
  error?: string;
  show: boolean;
  onToggle: () => void;
  registration: UseFormRegisterReturn;
  disabled: boolean;
}

function PasswordField({
  id,
  label,
  autoComplete,
  placeholder,
  error,
  show,
  onToggle,
  registration,
  disabled,
}: PasswordFieldProps) {
  return (
    <div className="space-y-1.5">
      <label htmlFor={id} className={labelClass}>
        {label}
      </label>
      <div className="relative">
        <input
          id={id}
          type={show ? 'text' : 'password'}
          autoComplete={autoComplete}
          disabled={disabled}
          aria-invalid={error ? 'true' : 'false'}
          aria-describedby={error ? `${id}-error` : undefined}
          {...registration}
          className={`${inputClass} pr-11`}
          placeholder={placeholder}
        />
        <button
          type="button"
          onClick={onToggle}
          aria-label={show ? `Hide ${label.toLowerCase()}` : `Show ${label.toLowerCase()}`}
          className="absolute right-2 top-1/2 -translate-y-1/2 rounded-lg p-1.5 text-stone-400 transition-colors hover:bg-warm-100 hover:text-stone-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
        >
          {show ? (
            <EyeOff className="h-4 w-4" aria-hidden="true" />
          ) : (
            <Eye className="h-4 w-4" aria-hidden="true" />
          )}
        </button>
      </div>
      {error && (
        <p id={`${id}-error`} className={errorClass}>
          {error}
        </p>
      )}
    </div>
  );
}

function ChangePassword() {
  const [showCurrent, setShowCurrent] = useState(false);
  const [showNew, setShowNew] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  const changePassword = useAuthStore((state) => state.changePassword);
  const pushToast = useToastStore((state) => state.push);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<ChangePasswordFormData>({
    resolver: zodResolver(changePasswordSchema),
  });

  const onSubmit = async (data: ChangePasswordFormData) => {
    setServerError(null);

    try {
      const message = await changePassword(
        data.currentPassword,
        data.newPassword
      );
      pushToast({
        title: 'Password changed',
        description: message,
        tone: 'success',
      });
      reset();
    } catch (error) {
      setServerError(
        error instanceof Error
          ? error.message
          : 'Unable to change your password. Please try again.'
      );
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Change password"
        description="Update the password you use to sign in to Catatku."
      />

      <Card className="max-w-xl p-6 sm:p-8">
        {serverError && (
          <div
            role="alert"
            className="mb-5 rounded-xl border border-expense/20 bg-expense-light/60 px-4 py-3 text-sm font-medium text-expense"
          >
            {serverError}
          </div>
        )}

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-5" noValidate>
          <PasswordField
            id="currentPassword"
            label="Current password"
            autoComplete="current-password"
            placeholder="Enter your current password"
            error={errors.currentPassword?.message}
            show={showCurrent}
            onToggle={() => setShowCurrent((visible) => !visible)}
            registration={register('currentPassword')}
            disabled={isSubmitting}
          />

          <PasswordField
            id="newPassword"
            label="New password"
            autoComplete="new-password"
            placeholder="At least 8 characters"
            error={errors.newPassword?.message}
            show={showNew}
            onToggle={() => setShowNew((visible) => !visible)}
            registration={register('newPassword')}
            disabled={isSubmitting}
          />

          <PasswordField
            id="confirmPassword"
            label="Confirm new password"
            autoComplete="new-password"
            placeholder="Repeat your new password"
            error={errors.confirmPassword?.message}
            show={showConfirm}
            onToggle={() => setShowConfirm((visible) => !visible)}
            registration={register('confirmPassword')}
            disabled={isSubmitting}
          />

          <div className="flex flex-col gap-3 pt-1 sm:flex-row sm:items-center">
            <Button
              type="submit"
              disabled={isSubmitting}
              className="w-full sm:w-auto"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                  Updating…
                </>
              ) : (
                <>
                  <KeyRound className="h-4 w-4" aria-hidden="true" />
                  Update password
                </>
              )}
            </Button>
            <Link
              to="/dashboard"
              className={`${secondaryLinkClass} w-full sm:w-auto`}
            >
              <ArrowLeft className="h-4 w-4" aria-hidden="true" />
              Back to dashboard
            </Link>
          </div>
        </form>
      </Card>
    </div>
  );
}

export default ChangePassword;
