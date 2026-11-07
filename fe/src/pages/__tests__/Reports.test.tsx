import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import Reports from '../Reports';
import { useReportStore } from '../../stores/reportStore';
import { useAccountStore } from '../../stores/accountStore';
import type {
  Account,
  CashflowPoint,
  CategoryTotal,
  MerchantTotal,
} from '../../types';

const account: Account = {
  id: 'account-1',
  name: 'Everyday checking',
  balance: 1250,
  opening_balance: 1250,
  entry_count: 1,
  currency: 'USD',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

const cashflow: CashflowPoint[] = [
  { period: '2026-06', income: 1000, expense: 400, net: 600 },
  { period: '2026-07', income: 1200, expense: 1500, net: -300 },
];

const categories: CategoryTotal[] = [
  {
    category_id: 'category-1',
    category_name: 'Salary',
    type: 'income',
    total: 2200,
    share: 100,
  },
  {
    category_id: 'category-2',
    category_name: 'Groceries',
    type: 'expense',
    total: 900,
    share: 60,
  },
  {
    category_id: 'category-3',
    category_name: 'Rent',
    type: 'expense',
    total: 600,
    share: 40,
  },
];

const merchants: MerchantTotal[] = [
  { description: 'Whole Foods', total: 240 },
  { description: 'Landlord', total: 600 },
];

function renderReports() {
  return render(
    <MemoryRouter>
      <Reports />
    </MemoryRouter>
  );
}

describe('Reports page', () => {
  beforeEach(() => {
    useReportStore.setState({
      from: '2026-06-01',
      to: '2026-11-01',
      cashflow,
      categories,
      merchants,
      isLoading: false,
      error: null,
      fetchReports: vi.fn(async () => undefined),
      setRange: vi.fn(),
    });
    useAccountStore.setState({
      accounts: [account],
      isLoading: false,
      error: null,
      fetchAccounts: vi.fn(async () => undefined),
    });
  });

  it('renders cashflow, category breakdown, and top merchants', () => {
    renderReports();

    expect(
      screen.getByRole('heading', { name: 'Cashflow by month' })
    ).toBeInTheDocument();
    expect(screen.getByText('Jun')).toBeInTheDocument();
    expect(screen.getByText('Jul')).toBeInTheDocument();
    expect(screen.getByText('-$300.00')).toBeInTheDocument();

    expect(
      screen.getByRole('heading', { name: 'Category breakdown' })
    ).toBeInTheDocument();
    expect(screen.getByText('Salary')).toBeInTheDocument();
    expect(screen.getByText('Groceries')).toBeInTheDocument();
    expect(screen.getByText('60.0% of expense')).toBeInTheDocument();

    expect(
      screen.getByRole('heading', { name: 'Top merchants' })
    ).toBeInTheDocument();
    expect(screen.getByText('Whole Foods')).toBeInTheDocument();
    expect(screen.getByText('$240.00')).toBeInTheDocument();
  });

  it('shows per-section empty states when the range has no data', () => {
    useReportStore.setState({ cashflow: [], categories: [], merchants: [] });
    renderReports();

    expect(
      screen.getByText('No cashflow recorded in this range.')
    ).toBeInTheDocument();
    expect(
      screen.getByText('No income recorded in this range.')
    ).toBeInTheDocument();
    expect(
      screen.getByText('No expense recorded in this range.')
    ).toBeInTheDocument();
    expect(
      screen.getByText('No merchant activity recorded in this range.')
    ).toBeInTheDocument();
  });

  it('shows a loading state', () => {
    useReportStore.setState({ cashflow: [], categories: [], merchants: [], isLoading: true });
    renderReports();

    expect(screen.getByText('Loading reports…')).toBeInTheDocument();
  });

  it('shows an error state with retry', async () => {
    const user = userEvent.setup();
    useReportStore.setState({
      cashflow: [],
      categories: [],
      merchants: [],
      error: 'Could not load reports',
    });
    renderReports();

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Could not load reports'
    );

    await user.click(screen.getByRole('button', { name: /try again/i }));

    expect(useReportStore.getState().fetchReports).toHaveBeenCalled();
  });

  it('applies a custom date range', async () => {
    const user = userEvent.setup();
    renderReports();

    fireEvent.change(screen.getByLabelText('From'), {
      target: { value: '2026-01-01' },
    });
    await user.click(screen.getByRole('button', { name: /apply range/i }));

    expect(useReportStore.getState().fetchReports).toHaveBeenCalledWith({
      from: '2026-01-01',
      to: '2026-11-01',
    });
  });

  it('rejects an inverted date range without fetching', async () => {
    const user = userEvent.setup();
    renderReports();
    const fetchReports = vi.mocked(
      useReportStore.getState().fetchReports
    );
    fetchReports.mockClear();

    fireEvent.change(screen.getByLabelText('From'), {
      target: { value: '2026-12-01' },
    });
    await user.click(screen.getByRole('button', { name: /apply range/i }));

    expect(screen.getByRole('alert')).toHaveTextContent(
      'The start date must be on or before the end date.'
    );
    expect(fetchReports).not.toHaveBeenCalled();
  });
});
