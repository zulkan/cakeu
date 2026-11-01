import { useEffect } from 'react';
import { Link } from 'react-router-dom';
import {
  ArrowDownRight,
  ArrowUpRight,
  Plus,
  Receipt,
  Scale,
  Wallet,
} from 'lucide-react';
import { useAuthStore } from '../stores/authStore';
import { useAccountStore } from '../stores/accountStore';
import { useCategoryStore } from '../stores/categoryStore';
import { useDashboardStore } from '../stores/dashboardStore';
import { displayName, formatCurrency } from '../lib/format';
import { PageHeader } from '../components/ui/PageHeader';
import { LoadingState, ErrorState } from '../components/ui/States';
import { StatCard } from '../components/dashboard/StatCard';
import { MonthlyBars } from '../components/dashboard/MonthlyBars';
import { RecentTransactions } from '../components/dashboard/RecentTransactions';
import { SetupChecklist } from '../components/dashboard/SetupChecklist';
import { BudgetSection } from '../components/dashboard/BudgetSection';
import type { DashboardSummary } from '../types';

const emptySummary: DashboardSummary = {
  total_balance: 0,
  monthly_income: 0,
  monthly_expense: 0,
  monthly_net: 0,
  transaction_count: 0,
  recent_transactions: [],
};

function Dashboard() {
  const user = useAuthStore((state) => state.user);
  const summary = useDashboardStore((state) => state.summary);
  const isLoading = useDashboardStore((state) => state.isLoading);
  const error = useDashboardStore((state) => state.error);
  const fetchSummary = useDashboardStore((state) => state.fetchSummary);

  const accounts = useAccountStore((state) => state.accounts);
  const fetchAccounts = useAccountStore((state) => state.fetchAccounts);
  const categories = useCategoryStore((state) => state.categories);
  const fetchCategories = useCategoryStore((state) => state.fetchCategories);

  useEffect(() => {
    void fetchSummary();
    void fetchAccounts();
    void fetchCategories();
  }, [fetchSummary, fetchAccounts, fetchCategories]);

  if (isLoading && !summary) {
    return <LoadingState label="Loading your dashboard…" />;
  }

  if (error && !summary) {
    return <ErrorState message={error} onRetry={() => void fetchSummary()} />;
  }

  const data = summary ?? emptySummary;
  const currency = accounts[0]?.currency ?? 'USD';
  const netIsPositive = data.monthly_net >= 0;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Dashboard"
        description={`Welcome back, ${user ? displayName(user.email) : 'there'}. Here is your money at a glance.`}
        action={
          <Link
            to="/transactions"
            className="inline-flex h-11 items-center gap-2 rounded-xl bg-brand-500 px-4 text-sm font-semibold text-white shadow-sm shadow-brand-500/25 transition-colors hover:bg-brand-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2"
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            Add transaction
          </Link>
        }
      />

      <section className="relative overflow-hidden rounded-3xl bg-gradient-to-br from-brand-500 via-brand-400 to-warm-400 p-6 text-white shadow-xl shadow-brand-500/20 sm:p-8">
        <div
          className="pointer-events-none absolute -right-16 -top-20 h-64 w-64 rounded-full bg-white/15 blur-2xl"
          aria-hidden="true"
        />
        <div
          className="pointer-events-none absolute -bottom-24 left-1/3 h-56 w-56 rounded-full bg-brand-700/20 blur-3xl"
          aria-hidden="true"
        />

        <div className="relative flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <p className="text-sm font-semibold text-white/75">Total balance</p>
            <p className="mt-2 text-4xl font-extrabold tracking-tight sm:text-5xl">
              {formatCurrency(data.total_balance, currency)}
            </p>
            <p className="mt-2 text-sm text-white/75">
              Across {accounts.length}{' '}
              {accounts.length === 1 ? 'account' : 'accounts'}
            </p>
          </div>

          <div className="flex flex-wrap gap-2">
            <Link
              to="/transactions"
              className="inline-flex h-10 items-center gap-2 rounded-xl bg-white px-4 text-sm font-semibold text-brand-700 shadow-sm transition-colors hover:bg-brand-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white focus-visible:ring-offset-2 focus-visible:ring-offset-brand-500"
            >
              <Plus className="h-4 w-4" aria-hidden="true" />
              New transaction
            </Link>
            <Link
              to="/accounts"
              className="inline-flex h-10 items-center gap-2 rounded-xl bg-white/15 px-4 text-sm font-semibold text-white ring-1 ring-inset ring-white/30 transition-colors hover:bg-white/25 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white"
            >
              <Wallet className="h-4 w-4" aria-hidden="true" />
              Manage accounts
            </Link>
          </div>
        </div>
      </section>

      <SetupChecklist
        needsAccount={accounts.length === 0}
        needsCategory={categories.length === 0}
      />

      <section
        aria-label="Monthly summary"
        className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4"
      >
        <StatCard
          label="Monthly income"
          value={formatCurrency(data.monthly_income, currency)}
          icon={ArrowDownRight}
          tone="income"
          hint="Money in this month"
        />
        <StatCard
          label="Monthly expense"
          value={formatCurrency(data.monthly_expense, currency)}
          icon={ArrowUpRight}
          tone="expense"
          hint="Money out this month"
        />
        <StatCard
          label="Net this month"
          value={formatCurrency(data.monthly_net, currency)}
          icon={Scale}
          tone={netIsPositive ? 'brand' : 'expense'}
          hint="Income minus expenses"
        />
        <StatCard
          label="Transactions"
          value={String(data.transaction_count)}
          icon={Receipt}
          tone="neutral"
          hint="Recorded this month"
        />
      </section>

      <BudgetSection categories={categories} currency={currency} />

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-[1.5fr_1fr]">
        <RecentTransactions
          transactions={data.recent_transactions}
          currency={currency}
        />
        <MonthlyBars
          income={data.monthly_income}
          expense={data.monthly_expense}
          currency={currency}
        />
      </div>
    </div>
  );
}

export default Dashboard;
