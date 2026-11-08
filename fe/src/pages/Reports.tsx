import { useEffect, useMemo, useState } from 'react';
import type { FormEvent } from 'react';
import { Loader2, RotateCcw } from 'lucide-react';
import { useReportStore } from '../stores/reportStore';
import { defaultReportRange } from '../lib/dates';
import { PageHeader } from '../components/ui/PageHeader';
import { Button } from '../components/ui/Button';
import { EmptyState, ErrorState, LoadingState } from '../components/ui/States';
import { Select } from '../components/ui/Select';
import { CashflowChart } from '../components/reports/CashflowChart';
import { CategoryBreakdown } from '../components/reports/CategoryBreakdown';
import { TopMerchants } from '../components/reports/TopMerchants';
import { inputClass, labelClass } from '../components/ui/formStyles';
import type {
  CashflowPoint,
  CategoryTotal,
  MerchantTotal,
} from '../types';

interface CurrencyOption {
  currency: string;
  rows: number;
}

function currencyOptions(
  cashflow: CashflowPoint[],
  categories: CategoryTotal[],
  merchants: MerchantTotal[]
): CurrencyOption[] {
  const counts = new Map<string, number>();
  const count = (rows: { currency: string }[]) => {
    rows.forEach((row) => {
      counts.set(row.currency, (counts.get(row.currency) ?? 0) + 1);
    });
  };

  count(cashflow);
  count(categories);
  count(merchants);

  return [...counts.entries()]
    .map(([currency, rows]) => ({ currency, rows }))
    .sort((a, b) => b.rows - a.rows || a.currency.localeCompare(b.currency));
}

function Reports() {
  const from = useReportStore((state) => state.from);
  const to = useReportStore((state) => state.to);
  const cashflow = useReportStore((state) => state.cashflow);
  const categories = useReportStore((state) => state.categories);
  const merchants = useReportStore((state) => state.merchants);
  const isLoading = useReportStore((state) => state.isLoading);
  const error = useReportStore((state) => state.error);
  const fetchReports = useReportStore((state) => state.fetchReports);

  const [range, setRange] = useState({ from, to });
  const [formError, setFormError] = useState<string | null>(null);

  const currencyOptionsForRange = useMemo(
    () => currencyOptions(cashflow, categories, merchants),
    [cashflow, categories, merchants]
  );
  const defaultCurrency = currencyOptionsForRange[0]?.currency ?? null;
  const rangeKey = `${from}|${to}`;
  const [selection, setSelection] = useState<{
    rangeKey: string;
    currency: string | null;
  }>({ rangeKey, currency: null });

  useEffect(() => {
    void fetchReports();
  }, [fetchReports]);

  const preferredCurrency =
    selection.rangeKey === rangeKey ? selection.currency : null;
  const selectedCurrency =
    preferredCurrency !== null &&
    currencyOptionsForRange.some(
      (option) => option.currency === preferredCurrency
    )
      ? preferredCurrency
      : defaultCurrency;

  const chooseCurrency = (currency: string) => {
    setSelection({ rangeKey, currency });
  };

  const selectedCashflow = useMemo(
    () =>
      selectedCurrency
        ? cashflow.filter((point) => point.currency === selectedCurrency)
        : [],
    [cashflow, selectedCurrency]
  );
  const selectedCategories = useMemo(
    () =>
      selectedCurrency
        ? categories.filter((total) => total.currency === selectedCurrency)
        : [],
    [categories, selectedCurrency]
  );
  const selectedMerchants = useMemo(
    () =>
      selectedCurrency
        ? merchants.filter((total) => total.currency === selectedCurrency)
        : [],
    [merchants, selectedCurrency]
  );

  const hasData =
    cashflow.length > 0 || categories.length > 0 || merchants.length > 0;

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!range.from || !range.to) {
      setFormError('Choose both a start and an end date.');
      return;
    }

    if (range.from > range.to) {
      setFormError('The start date must be on or before the end date.');
      return;
    }

    setFormError(null);
    void fetchReports(range);
  };

  const handleReset = () => {
    const next = defaultReportRange();
    setRange(next);
    setFormError(null);
    void fetchReports(next);
  };

  const otherCurrencies = currencyOptionsForRange.length - 1;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Reports"
        description="See how money moved over time, by category, and by merchant."
      />

      <form
        onSubmit={handleSubmit}
        noValidate
        className="rounded-2xl border border-warm-200/80 bg-surface-card p-4 shadow-sm sm:p-5"
      >
        <div className="flex flex-col gap-4 sm:flex-row sm:items-end">
          <div className="space-y-1.5">
            <label htmlFor="report-from" className={labelClass}>
              From
            </label>
            <input
              id="report-from"
              type="date"
              value={range.from}
              max={range.to || undefined}
              onChange={(event) =>
                setRange((current) => ({
                  ...current,
                  from: event.target.value,
                }))
              }
              className={inputClass}
            />
          </div>

          <div className="space-y-1.5">
            <label htmlFor="report-to" className={labelClass}>
              To
            </label>
            <input
              id="report-to"
              type="date"
              value={range.to}
              min={range.from || undefined}
              onChange={(event) =>
                setRange((current) => ({
                  ...current,
                  to: event.target.value,
                }))
              }
              className={inputClass}
            />
          </div>

          <div className="flex flex-wrap gap-2 sm:ml-auto">
            <Button variant="secondary" onClick={handleReset}>
              <RotateCcw className="h-4 w-4" aria-hidden="true" />
              Last 6 months
            </Button>
            <Button type="submit" disabled={isLoading}>
              {isLoading ? (
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
              ) : null}
              {isLoading ? 'Loading…' : 'Apply range'}
            </Button>
          </div>
        </div>

        {formError && (
          <p
            role="alert"
            className="mt-3 rounded-xl border border-expense/20 bg-expense-light/60 px-4 py-3 text-sm font-medium text-expense"
          >
            {formError}
          </p>
        )}
      </form>

      {isLoading && !hasData ? (
        <LoadingState label="Loading reports…" />
      ) : error && !hasData ? (
        <ErrorState message={error} onRetry={() => void fetchReports()} />
      ) : (
        <>
          {error && (
            <div
              role="alert"
              className="rounded-xl border border-expense/20 bg-expense-light/60 px-4 py-3 text-sm font-medium text-expense"
            >
              {error}
            </div>
          )}

          {currencyOptionsForRange.length > 1 && (
            <div className="flex flex-col gap-3 rounded-2xl border border-warm-200/80 bg-surface-card p-4 shadow-sm sm:flex-row sm:items-end sm:justify-between sm:p-5">
              <div className="w-full space-y-1.5 sm:max-w-48">
                <label htmlFor="report-currency" className={labelClass}>
                  Currency
                </label>
                <Select
                  id="report-currency"
                  value={selectedCurrency ?? ''}
                  onChange={(event) => chooseCurrency(event.target.value)}
                >
                  {currencyOptionsForRange.map((option) => (
                    <option key={option.currency} value={option.currency}>
                      {option.currency}
                    </option>
                  ))}
                </Select>
              </div>
              <p className="text-xs text-stone-500">
                Showing {selectedCurrency} — {otherCurrencies}{' '}
                {otherCurrencies === 1
                  ? 'other currency available'
                  : 'other currencies available'}
              </p>
            </div>
          )}

          {selectedCurrency === null ? (
            <EmptyState
              title="No report data"
              description="No transactions were recorded in this date range."
            />
          ) : (
            <>
              <CashflowChart
                points={selectedCashflow}
                currency={selectedCurrency}
              />
              <CategoryBreakdown
                categories={selectedCategories}
                currency={selectedCurrency}
              />
              <TopMerchants
                merchants={selectedMerchants}
                currency={selectedCurrency}
              />
            </>
          )}
        </>
      )}
    </div>
  );
}

export default Reports;
