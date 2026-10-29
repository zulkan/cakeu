import { ArrowRight, CheckCircle2, Circle, Sparkles } from 'lucide-react';
import { Link } from 'react-router-dom';
import { Card } from '../ui/Card';

interface SetupChecklistProps {
  needsAccount: boolean;
  needsCategory: boolean;
}

export function SetupChecklist({
  needsAccount,
  needsCategory,
}: SetupChecklistProps) {
  if (!needsAccount && !needsCategory) return null;

  return (
    <Card className="border-brand-200 bg-gradient-to-br from-brand-50 to-warm-50 p-5">
      <div className="flex items-start gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-brand-500 text-white">
          <Sparkles className="h-5 w-5" aria-hidden="true" />
        </div>
        <div className="min-w-0 flex-1">
          <h2 className="text-base font-bold text-stone-900">
            Finish setting up Catatku
          </h2>
          <p className="mt-1 text-sm text-stone-600">
            Two quick steps unlock the full dashboard.
          </p>

          <div className="mt-4 grid gap-3 sm:grid-cols-2">
            <SetupStep
              done={!needsAccount}
              title="Create an account"
              description="A wallet, bank account, or cash balance."
              to="/accounts"
              cta="Add account"
            />
            <SetupStep
              done={!needsCategory}
              title="Create a category"
              description="Group income and expenses your way."
              to="/categories"
              cta="Add category"
            />
          </div>
        </div>
      </div>
    </Card>
  );
}

function SetupStep({
  done,
  title,
  description,
  to,
  cta,
}: {
  done: boolean;
  title: string;
  description: string;
  to: string;
  cta: string;
}) {
  return (
    <div className="rounded-xl border border-warm-200 bg-surface-card p-3">
      <div className="flex items-center gap-2">
        {done ? (
          <CheckCircle2
            className="h-4 w-4 text-income"
            aria-hidden="true"
          />
        ) : (
          <Circle className="h-4 w-4 text-warm-400" aria-hidden="true" />
        )}
        <p className="text-sm font-bold text-stone-900">{title}</p>
      </div>
      <p className="mt-1 text-xs text-stone-500">{description}</p>
      {!done && (
        <Link
          to={to}
          className="mt-2 inline-flex items-center gap-1 text-xs font-semibold text-brand-600 hover:text-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
        >
          {cta}
          <ArrowRight className="h-3 w-3" aria-hidden="true" />
        </Link>
      )}
    </div>
  );
}
