import type { ReportRange } from '../types';

function pad(value: number): string {
  return String(value).padStart(2, '0');
}

export function toISODate(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

export function todayISO(): string {
  return toISODate(new Date());
}

export function currentMonth(): string {
  const now = new Date();
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}`;
}

/**
 * Default report window: the current month plus the five before it, expressed
 * as inclusive day boundaries. The backend expands these to whole months.
 */
export function defaultReportRange(months = 6): ReportRange {
  const now = new Date();
  const start = new Date(now.getFullYear(), now.getMonth() - (months - 1), 1);

  return { from: toISODate(start), to: toISODate(now) };
}

function parsePeriod(period: string): { year: number; month: number } | null {
  const [year, month] = period.split('-').map(Number);

  if (!Number.isInteger(year) || !Number.isInteger(month)) return null;
  if (month < 1 || month > 12) return null;

  return { year, month };
}

export function formatMonthLabel(period: string): string {
  const parsed = parsePeriod(period);
  if (!parsed) return period;

  return new Date(parsed.year, parsed.month - 1, 1).toLocaleDateString('en-US', {
    month: 'long',
    year: 'numeric',
  });
}

export function formatShortMonth(period: string): string {
  const parsed = parsePeriod(period);
  if (!parsed) return period;

  return new Date(parsed.year, parsed.month - 1, 1).toLocaleDateString('en-US', {
    month: 'short',
  });
}
