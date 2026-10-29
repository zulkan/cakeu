import { useState } from 'react';
import { AlertCircle, Eye, EyeOff, KeyRound, Loader2 } from 'lucide-react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useAuthStore } from '../stores/authStore';
import { AuthLayout } from '../components/layout/AuthLayout';
import { Button } from '../components/ui/Button';
import {
  errorClass,
  inputClass,
  labelClass,
  secondaryLinkClass,
} from '../components/ui/formStyles';

const resetPasswordSchema = z
  .object({
    password: z
      .string()
      .min(1, 'Password is required')
      .min(8, 'Password must be at least 8 characters'),
    confirmPassword: z.string().min(1, 'Please confirm your password'),
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: 'Passwords do not match',
    path: ['confirmPassword'],
  });

type ResetPasswordFormData = z.infer<typeof resetPasswordSchema>;

function ResetPassword() {
  const [searchParams] = useSearchParams();
  const token = searchParams.get('token');
  const [showPassword, setShowPassword] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<ResetPasswordFormData>({
    resolver: zodResolver(resetPasswordSchema),
  });

  const navigate = useNavigate();
  const resetPassword = useAuthStore((state) => state.resetPassword);

  const onSubmit = async (data: ResetPasswordFormData) => {
    if (!token) {
      return;
    }

    setServerError(null);

    try {
      await resetPassword(token, data.password);
      navigate('/login', { replace: true, state: { resetSuccess: true } });
    } catch (error) {
      setServerError(
        error instanceof Error
          ? error.message
          : 'Unable to reset your password. Please try again.'
      );
    }
  };

  const errorMessage = token
    ? serverError
    : 'This reset link is missing its token.';

  return (
    <AuthLayout
      title="Set a new password"
      subtitle="Choose a new password for your Catatku account."
      footer={
        <p className="text-center text-sm text-stone-500">
          Back to{' '}
          <Link
            to="/login"
            className="font-semibold text-brand-600 underline-offset-4 hover:text-brand-700 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
          >
            sign in
          </Link>
        </p>
      }
    >
      {errorMessage ? (
        <div
          role="alert"
          className="rounded-2xl border border-expense/20 bg-expense-light/40 px-5 py-6 text-center"
        >
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-expense-light text-expense">
            <AlertCircle className="h-6 w-6" aria-hidden="true" />
          </div>
          <h2 className="mt-4 text-base font-bold text-stone-900">
            This link can't be used
          </h2>
          <p className="mt-2 text-sm text-stone-600">{errorMessage}</p>
          <Link
            to="/forgot-password"
            className={`${secondaryLinkClass} mt-5`}
          >
            Request a new link
          </Link>
        </div>
      ) : (
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-5" noValidate>
          <div className="space-y-1.5">
            <label htmlFor="password" className={labelClass}>
              New password
            </label>
            <div className="relative">
              <input
                id="password"
                type={showPassword ? 'text' : 'password'}
                autoComplete="new-password"
                disabled={isSubmitting}
                aria-invalid={errors.password ? 'true' : 'false'}
                aria-describedby={
                  errors.password ? 'password-error' : undefined
                }
                {...register('password')}
                className={`${inputClass} pr-11`}
                placeholder="At least 8 characters"
              />
              <button
                type="button"
                onClick={() => setShowPassword((visible) => !visible)}
                aria-label={showPassword ? 'Hide password' : 'Show password'}
                className="absolute right-2 top-1/2 -translate-y-1/2 rounded-lg p-1.5 text-stone-400 transition-colors hover:bg-warm-100 hover:text-stone-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
              >
                {showPassword ? (
                  <EyeOff className="h-4 w-4" aria-hidden="true" />
                ) : (
                  <Eye className="h-4 w-4" aria-hidden="true" />
                )}
              </button>
            </div>
            {errors.password && (
              <p id="password-error" className={errorClass}>
                {errors.password.message}
              </p>
            )}
          </div>

          <div className="space-y-1.5">
            <label htmlFor="confirmPassword" className={labelClass}>
              Confirm new password
            </label>
            <input
              id="confirmPassword"
              type={showPassword ? 'text' : 'password'}
              autoComplete="new-password"
              disabled={isSubmitting}
              aria-invalid={errors.confirmPassword ? 'true' : 'false'}
              aria-describedby={
                errors.confirmPassword ? 'confirm-password-error' : undefined
              }
              {...register('confirmPassword')}
              className={inputClass}
              placeholder="Repeat your new password"
            />
            {errors.confirmPassword && (
              <p id="confirm-password-error" className={errorClass}>
                {errors.confirmPassword.message}
              </p>
            )}
          </div>

          <Button
            type="submit"
            disabled={isSubmitting}
            className="w-full"
          >
            {isSubmitting ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                Resetting…
              </>
            ) : (
              <>
                <KeyRound className="h-4 w-4" aria-hidden="true" />
                Reset password
              </>
            )}
          </Button>
        </form>
      )}
    </AuthLayout>
  );
}

export default ResetPassword;
