import { describe, it, expect, beforeEach, vi } from 'vitest';
import { api } from '../../lib/api';
import { useReportStore } from '../reportStore';
import type { CashflowPoint, CategoryTotal, MerchantTotal } from '../../types';

vi.mock('../../lib/api', () => ({
  api: {
    getCashflowReport: vi.fn(),
    getCategoryReport: vi.fn(),
    getMerchantReport: vi.fn(),
  },
}));

const cashflow: CashflowPoint[] = [
  { currency: 'USD', period: '2026-06', income: 1000, expense: 400, net: 600 },
];

const categories: CategoryTotal[] = [
  {
    currency: 'USD',
    category_id: 'category-1',
    category_name: 'Groceries',
    type: 'expense',
    total: 400,
    share: 100,
  },
];

const merchants: MerchantTotal[] = [
  { currency: 'USD', description: 'Whole Foods', total: 240 },
];

describe('report store', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useReportStore.setState({
      from: '2026-06-01',
      to: '2026-11-01',
      cashflow: [],
      categories: [],
      merchants: [],
      isLoading: false,
      error: null,
    });
  });

  it('loads all three reports for the current range', async () => {
    vi.mocked(api.getCashflowReport).mockResolvedValue(cashflow);
    vi.mocked(api.getCategoryReport).mockResolvedValue(categories);
    vi.mocked(api.getMerchantReport).mockResolvedValue(merchants);

    await useReportStore.getState().fetchReports();

    expect(api.getCashflowReport).toHaveBeenCalledWith(
      '2026-06-01',
      '2026-11-01'
    );
    expect(api.getCategoryReport).toHaveBeenCalledWith(
      '2026-06-01',
      '2026-11-01'
    );
    expect(api.getMerchantReport).toHaveBeenCalledWith(
      '2026-06-01',
      '2026-11-01'
    );
    expect(useReportStore.getState().cashflow).toEqual(cashflow);
    expect(useReportStore.getState().categories).toEqual(categories);
    expect(useReportStore.getState().merchants).toEqual(merchants);
    expect(useReportStore.getState().isLoading).toBe(false);
  });

  it('adopts a new range when one is supplied', async () => {
    vi.mocked(api.getCashflowReport).mockResolvedValue([]);
    vi.mocked(api.getCategoryReport).mockResolvedValue([]);
    vi.mocked(api.getMerchantReport).mockResolvedValue([]);

    await useReportStore
      .getState()
      .fetchReports({ from: '2026-01-01', to: '2026-03-31' });

    expect(useReportStore.getState().from).toBe('2026-01-01');
    expect(useReportStore.getState().to).toBe('2026-03-31');
    expect(api.getCashflowReport).toHaveBeenCalledWith(
      '2026-01-01',
      '2026-03-31'
    );
  });

  it('captures a report error', async () => {
    vi.mocked(api.getCashflowReport).mockRejectedValue(
      new Error('invalid report range')
    );
    vi.mocked(api.getCategoryReport).mockResolvedValue([]);
    vi.mocked(api.getMerchantReport).mockResolvedValue([]);

    await useReportStore.getState().fetchReports();

    expect(useReportStore.getState().error).toBe('invalid report range');
    expect(useReportStore.getState().isLoading).toBe(false);
  });

  it('updates the stored range without fetching', () => {
    useReportStore.getState().setRange('2025-01-01', '2025-12-31');

    expect(useReportStore.getState().from).toBe('2025-01-01');
    expect(useReportStore.getState().to).toBe('2025-12-31');
    expect(api.getCashflowReport).not.toHaveBeenCalled();
  });
});
