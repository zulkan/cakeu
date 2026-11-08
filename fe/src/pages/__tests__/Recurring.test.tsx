import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import Recurring from '../Recurring';
import { useRecurringStore } from '../../stores/recurringStore';
import { useAccountStore } from '../../stores/accountStore';
import { useCategoryStore } from '../../stores/categoryStore';
import { useToastStore } from '../../stores/toastStore';
import { Toaster } from '../../components/ui/Toaster';
import { formatDate } from '../../lib/format';
import type { Account, Category, RecurringRule } from '../../types';

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

const category: Category = {
  id: 'category-1',
  name: 'Housing',
  type: 'expense',
  color: '#FB923C',
  created_at: '2026-01-01T00:00:00Z',
};

const rule: RecurringRule = {
  id: 'rule-1',
  user_id: 'user-1',
  account_id: account.id,
  category_id: category.id,
  type: 'expense',
  amount: 1200,
  description: 'Rent',
  frequency: 'monthly',
  interval: 1,
  next_run: '2026-12-01T00:00:00Z',
  active: true,
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-01T00:00:00Z',
};

function renderRecurring() {
  return render(
    <MemoryRouter>
      <Recurring />
      <Toaster />
    </MemoryRouter>
  );
}

describe('Recurring page', () => {
  beforeEach(() => {
    useToastStore.setState({ toasts: [] });
    useRecurringStore.setState({
      rules: [],
      isLoading: false,
      error: null,
      fetchRules: vi.fn(async () => undefined),
      createRule: vi.fn(async () => rule),
      updateRule: vi.fn(async () => rule),
      deleteRule: vi.fn(async () => undefined),
      runNow: vi.fn(async () => 2),
    });
    useAccountStore.setState({
      accounts: [account],
      isLoading: false,
      error: null,
      fetchAccounts: vi.fn(async () => undefined),
    });
    useCategoryStore.setState({
      categories: [category],
      isLoading: false,
      error: null,
      fetchCategories: vi.fn(async () => undefined),
    });
  });

  it('renders each rule with its schedule and status', () => {
    useRecurringStore.setState({ rules: [rule] });

    renderRecurring();

    expect(screen.getByText('Rent')).toBeInTheDocument();
    expect(
      screen.getByText('Everyday checking · Housing · Monthly ×1')
    ).toBeInTheDocument();
    expect(
      screen.getByText(`Next run ${formatDate(rule.next_run)}`)
    ).toBeInTheDocument();
    expect(screen.getByText('Active')).toBeInTheDocument();
    expect(screen.getByText('−$1,200.00')).toBeInTheDocument();
  });

  it('pauses an active rule', async () => {
    const user = userEvent.setup();
    useRecurringStore.setState({ rules: [rule] });
    renderRecurring();

    await user.click(screen.getByRole('button', { name: 'Pause' }));

    await waitFor(() => {
      expect(useRecurringStore.getState().updateRule).toHaveBeenCalledWith(
        rule.id,
        { active: false }
      );
    });
  });

  it('reports how many transactions were posted by run now', async () => {
    const user = userEvent.setup();
    useRecurringStore.setState({ rules: [rule] });
    renderRecurring();

    await user.click(screen.getByRole('button', { name: 'Run now' }));

    expect(
      await screen.findByText(/Posted 2 transactions for/)
    ).toBeInTheDocument();
    expect(useRecurringStore.getState().runNow).toHaveBeenCalledWith(rule.id);
  });

  it('creates a rule from the dialog', async () => {
    const user = userEvent.setup();
    renderRecurring();

    await user.click(
      screen.getAllByRole('button', { name: /new rule/i })[0]
    );

    const dialog = screen.getByRole('dialog');
    await user.selectOptions(
      within(dialog).getByLabelText('Category'),
      category.id
    );
    await user.type(within(dialog).getByLabelText('Amount'), '50');
    await user.type(within(dialog).getByLabelText('Description'), 'Gym');
    await user.click(within(dialog).getByRole('button', { name: /create rule/i }));

    await waitFor(() => {
      expect(useRecurringStore.getState().createRule).toHaveBeenCalledWith(
        expect.objectContaining({
          account_id: account.id,
          category_id: category.id,
          type: 'expense',
          amount: 50,
          description: 'Gym',
          frequency: 'monthly',
          interval: 1,
        })
      );
    });
  });

  it('edits an existing rule', async () => {
    const user = userEvent.setup();
    useRecurringStore.setState({ rules: [rule] });
    renderRecurring();

    await user.click(screen.getByRole('button', { name: 'Edit Rent' }));

    const dialog = screen.getByRole('dialog');
    expect(
      within(dialog).getByText(/account, category, and type are fixed/i)
    ).toBeInTheDocument();
    await user.clear(within(dialog).getByLabelText('Amount'));
    await user.type(within(dialog).getByLabelText('Amount'), '1300');
    await user.click(within(dialog).getByRole('button', { name: /save changes/i }));

    await waitFor(() => {
      expect(useRecurringStore.getState().updateRule).toHaveBeenCalledWith(
        rule.id,
        expect.objectContaining({ amount: 1300 })
      );
    });
  });

  it('deletes a rule after confirmation', async () => {
    const user = userEvent.setup();
    useRecurringStore.setState({ rules: [rule] });
    renderRecurring();

    await user.click(screen.getByRole('button', { name: 'Delete Rent' }));

    const dialog = screen.getByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    await waitFor(() => {
      expect(useRecurringStore.getState().deleteRule).toHaveBeenCalledWith(
        rule.id
      );
    });
  });

  it('shows an empty state when there are no rules', () => {
    renderRecurring();

    expect(screen.getByText('No recurring rules yet')).toBeInTheDocument();
  });

  it('shows a loading state', () => {
    useRecurringStore.setState({ isLoading: true });
    renderRecurring();

    expect(screen.getByText('Loading recurring rules…')).toBeInTheDocument();
  });

  it('shows an error state with retry', async () => {
    const user = userEvent.setup();
    useRecurringStore.setState({ error: 'Could not load rules' });
    renderRecurring();

    expect(screen.getByRole('alert')).toHaveTextContent('Could not load rules');
    await user.click(screen.getByRole('button', { name: /try again/i }));

    expect(useRecurringStore.getState().fetchRules).toHaveBeenCalled();
  });
});
