import { useState } from 'react';
import { Loader2, MailCheck, Send } from 'lucide-react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Link } from 'react-router-dom';
import { useAuthStore } from '../stores/authStore';
import { AuthLayout } from '../components/layout/AuthLayout';
import { Button } from '../components/ui/Button';
import {
  errorClass,
  inputClass,
  labelClass,
  secondaryLinkClass,
} from '../components/ui/formStyles';

const forgotPasswordSchema = z.object({
  email: z.string().min(1, 'Email is required').email('Invalid email format'),
});

type ForgotPasswordFormData = z.infer<typeof forgotPasswordSchema>;

function ForgotPassword() {
  const [sent, setSent] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<ForgotPasswordFormData>({
    resolver: zodResolver(forgotPasswordSchema),
  });

  const forgotPassword = useAuthStore((state) => state.forgotPassword);

  const onSubmit = async (data: ForgotPasswordFormData) => {
    setServerError(null);

    try {
      await forgotPassword(data.email);
      setSent(true);
    } catch (error) {
      setServerError(
        error instanceof Error
          ? error.message
          : 'Unable to send the reset link. Please try again.'
      );
    }
  };

  return (
    <AuthLayout
      title="Forgot your password?"
      subtitle="Enter the email on your account and we'll send you a reset link."
      footer={
        <p className="text-center text-sm text-stone-500">
          Remembered your password?{' '}
          <Link
            to="/login"
            className="font-semibold text-brand-600 underline-offset-4 hover:text-brand-700 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
          >
            Sign in
          </Link>
        </p>
      }
    >
      {sent ? (
        <div
          role="status"
          className="rounded-2xl border border-brand-200 bg-brand-50/70 px-5 py-6 text-center"
        >
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-brand-100 text-brand-600">
            <MailCheck className="h-6 w-6" aria-hidden="true" />
          </div>
          <h2 className="mt-4 text-base font-bold text-stone-900">
            Check your inbox
          </h2>
          <p className="mt-2 text-sm text-stone-600">
            If that email exists, a reset link is on its way.
          </p>
          <Link to="/login" className={`${secondaryLinkClass} mt-5`}>
            Back to sign in
          </Link>
        </div>
      ) : (
        <>
          {serverError && (
            <div
              role="alert"
              className="mb-5 rounded-xl border border-expense/20 bg-expense-light/60 px-4 py-3 text-sm font-medium text-expense"
            >
              {serverError}
            </div>
          )}

          <form
            onSubmit={handleSubmit(onSubmit)}
            className="space-y-5"
            noValidate
          >
            <div className="space-y-1.5">
              <label htmlFor="email" className={labelClass}>
                Email
              </label>
              <input
                id="email"
                type="email"
                autoComplete="email"
                disabled={isSubmitting}
                aria-invalid={errors.email ? 'true' : 'false'}
                aria-describedby={errors.email ? 'email-error' : undefined}
                {...register('email')}
                className={inputClass}
                placeholder="you@example.com"
              />
              {errors.email && (
                <p id="email-error" className={errorClass}>
                  {errors.email.message}
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
                  <Loader2
                    className="h-4 w-4 animate-spin"
                    aria-hidden="true"
                  />
                  Sending…
                </>
              ) : (
                <>
                  <Send className="h-4 w-4" aria-hidden="true" />
                  Send reset link
                </>
              )}
            </Button>
          </form>
        </>
      )}
    </AuthLayout>
  );
}

export default ForgotPassword;
