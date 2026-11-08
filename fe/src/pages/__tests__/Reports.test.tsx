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
  { currency: 'USD', period: '2026-06', income: 1000, expense: 400, net: 600 },
  { currency: 'USD', period: '2026-07', income: 1200, expense: 1500, net: -300 },
];

const categories: CategoryTotal[] = [
  {
    currency: 'USD',
    category_id: 'category-1',
    category_name: 'Salary',
    type: 'income',
    total: 2200,
    share: 100,
  },
  {
    currency: 'USD',
    category_id: 'category-2',
    category_name: 'Groceries',
    type: 'expense',
    total: 900,
    share: 60,
  },
  {
    currency: 'USD',
    category_id: 'category-3',
    category_name: 'Rent',
    type: 'expense',
    total: 600,
    share: 40,
  },
];

const merchants: MerchantTotal[] = [
  { currency: 'USD', description: 'Whole Foods', total: 240 },
  { currency: 'USD', description: 'Landlord', total: 600 },
];

const multiCurrencyCashflow: CashflowPoint[] = [
  ...cashflow,
  {
    currency: 'IDR',
    period: '2026-06',
    income: 15000000,
    expense: 5000000,
    net: 10000000,
  },
];

const multiCurrencyCategories: CategoryTotal[] = [
  ...categories,
  {
    currency: 'IDR',
    category_id: 'category-4',
    category_name: 'Makan',
    type: 'expense',
    total: 5000000,
    share: 100,
  },
];

const multiCurrencyMerchants: MerchantTotal[] = [
  ...merchants,
  { currency: 'IDR', description: 'Warung', total: 5000000 },
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

  it('shows an empty state when the range has no rows', () => {
    useReportStore.setState({ cashflow: [], categories: [], merchants: [] });
    renderReports();

    expect(screen.getByText('No report data')).toBeInTheDocument();
    expect(
      screen.getByText('No transactions were recorded in this date range.')
    ).toBeInTheDocument();
    expect(screen.queryByLabelText('Currency')).not.toBeInTheDocument();
  });

  it('defaults to the currency with the most rows and never mixes them', () => {
    useReportStore.setState({
      cashflow: multiCurrencyCashflow,
      categories: multiCurrencyCategories,
      merchants: multiCurrencyMerchants,
    });
    renderReports();

    expect(screen.getByLabelText('Currency')).toHaveValue('USD');
    expect(
      screen.getByText('Showing USD — 1 other currency available')
    ).toBeInTheDocument();
    expect(screen.getByText('-$300.00')).toBeInTheDocument();
    expect(screen.queryByText(/10,000,000\.00/)).not.toBeInTheDocument();
  });

  it('switches the displayed rows when the currency changes', async () => {
    const user = userEvent.setup();
    useReportStore.setState({
      cashflow: multiCurrencyCashflow,
      categories: multiCurrencyCategories,
      merchants: multiCurrencyMerchants,
    });
    renderReports();

    await user.selectOptions(screen.getByLabelText('Currency'), 'IDR');

    expect(screen.getByLabelText('Currency')).toHaveValue('IDR');
    expect(
      screen.getByText('Showing IDR — 1 other currency available')
    ).toBeInTheDocument();
    expect(
      screen.getAllByText(/10,000,000\.00/).length
    ).toBeGreaterThan(0);
    expect(screen.queryByText('-$300.00')).not.toBeInTheDocument();
  });

  it('breaks a row-count tie alphabetically', () => {
    useReportStore.setState({
      cashflow: [
        { currency: 'USD', period: '2026-06', income: 1, expense: 0, net: 1 },
        { currency: 'IDR', period: '2026-06', income: 1, expense: 0, net: 1 },
      ],
      categories: [],
      merchants: [],
    });
    renderReports();

    expect(screen.getByLabelText('Currency')).toHaveValue('IDR');
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
