import type { ReactNode } from 'react';
import { ShieldCheck, Sparkles, TrendingUp } from 'lucide-react';
import { BrandMark } from '../BrandMark';

interface AuthLayoutProps {
  title: string;
  subtitle: string;
  children: ReactNode;
  footer: ReactNode;
}

const highlights = [
  {
    icon: TrendingUp,
    title: 'See the whole picture',
    body: 'Income, expenses, and balances in one calm dashboard.',
  },
  {
    icon: Sparkles,
    title: 'Built for daily habits',
    body: 'Log a transaction in seconds and keep every category tidy.',
  },
  {
    icon: ShieldCheck,
    title: 'Your data stays yours',
    body: 'Every request is authenticated with your own secure token.',
  },
];

export function AuthLayout({
  title,
  subtitle,
  children,
  footer,
}: AuthLayoutProps) {
  return (
    <div className="min-h-screen bg-surface lg:grid lg:grid-cols-[1.05fr_1fr]">
      <aside className="relative hidden overflow-hidden bg-gradient-to-br from-brand-500 via-brand-400 to-warm-400 p-12 text-white lg:flex lg:flex-col lg:justify-between">
        <div
          className="pointer-events-none absolute -right-24 -top-24 h-80 w-80 rounded-full bg-white/15 blur-2xl"
          aria-hidden="true"
        />
        <div
          className="pointer-events-none absolute -bottom-32 -left-20 h-96 w-96 rounded-full bg-brand-700/25 blur-3xl"
          aria-hidden="true"
        />

        <BrandMark tone="light" className="relative" />

        <div className="relative max-w-md">
          <p className="text-sm font-semibold uppercase tracking-[0.2em] text-white/70">
            Personal finance, simplified
          </p>
          <h2 className="mt-4 text-4xl font-extrabold leading-tight tracking-tight">
            Your money, finally in focus.
          </h2>
          <p className="mt-4 text-base leading-relaxed text-white/80">
            Catatku turns everyday spending into a clear, warm picture of where
            your money goes — without spreadsheets.
          </p>

          <ul className="mt-10 space-y-5">
            {highlights.map(({ icon: Icon, title: itemTitle, body }) => (
              <li key={itemTitle} className="flex gap-3">
                <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-white/15 ring-1 ring-white/20">
                  <Icon className="h-5 w-5" aria-hidden="true" />
                </div>
                <div>
                  <p className="text-sm font-bold">{itemTitle}</p>
                  <p className="text-sm text-white/75">{body}</p>
                </div>
              </li>
            ))}
          </ul>
        </div>

        <p className="relative text-xs text-white/60">
          © 2026 Catatku. Pencatatan Keuangan.
        </p>
      </aside>

      <main className="flex min-h-screen items-center justify-center px-4 py-10 sm:px-8">
        <div className="w-full max-w-md">
          <BrandMark className="mb-10 lg:hidden" />
          <h1 className="text-3xl font-extrabold tracking-tight text-stone-900">
            {title}
          </h1>
          <p className="mt-2 text-sm text-stone-500">{subtitle}</p>
          <div className="mt-8">{children}</div>
          <div className="mt-6">{footer}</div>
        </div>
      </main>
    </div>
  );
}
