import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BudgetSection } from '../BudgetSection';
import { useBudgetStore } from '../../../stores/budgetStore';
import type { BudgetStatus, Category } from '../../../types';

const expenseCategory: Category = {
  id: 'category-1',
  name: 'Groceries',
  type: 'expense',
  color: '#FB923C',
  created_at: '2026-01-01T00:00:00Z',
};

const incomeCategory: Category = {
  id: 'category-2',
  name: 'Salary',
  type: 'income',
  color: '#16A34A',
  created_at: '2026-01-01T00:00:00Z',
};

const underBudget: BudgetStatus = {
  category_id: 'category-1',
  category_name: 'Groceries',
  budget: 200,
  spent: 120,
  remaining: 80,
  percent: 60,
};

const overBudget: BudgetStatus = {
  category_id: 'category-3',
  category_name: 'Dining',
  budget: 100,
  spent: 150,
  remaining: -50,
  percent: 150,
};

function renderSection(categories: Category[] = [expenseCategory, incomeCategory]) {
  return render(<BudgetSection categories={categories} currency="USD" />);
}

describe('BudgetSection', () => {
  beforeEach(() => {
    useBudgetStore.setState({
      month: '2026-11',
      budgets: [],
      isLoading: false,
      error: null,
      isSaving: false,
      saveError: null,
      fetchBudgets: vi.fn(async () => undefined),
      upsertBudget: vi.fn(async () => undefined),
      deleteBudget: vi.fn(async () => undefined),
    });
  });

  it('renders progress for each budget and flags overspending', () => {
    useBudgetStore.setState({ budgets: [underBudget, overBudget] });

    renderSection();

    expect(
      screen.getByRole('heading', { name: 'Budgets this month' })
    ).toBeInTheDocument();
    expect(screen.getByText('November 2026')).toBeInTheDocument();
    expect(screen.getByText('Groceries')).toBeInTheDocument();
    expect(screen.getByText('$80.00 remaining')).toHaveClass('text-stone-500');

    const bar = screen.getByRole('progressbar', {
      name: 'Groceries budget used',
    });
    expect(bar).toHaveAttribute('aria-valuenow', '60');

    expect(screen.getByText('Over by $50.00')).toHaveClass('text-expense');
    expect(screen.getByText('150%')).toHaveClass('text-expense');
  });

  it('shows an empty state when nothing is budgeted', () => {
    renderSection();

    expect(screen.getByText('No budgets yet')).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /set a budget/i })
    ).toBeInTheDocument();
  });

  it('shows a loading state', () => {
    useBudgetStore.setState({ isLoading: true });

    renderSection();

    expect(screen.getByText('Loading budgets…')).toBeInTheDocument();
  });

  it('shows an error state with retry', async () => {
    const user = userEvent.setup();
    useBudgetStore.setState({ error: 'Could not load budgets' });
    renderSection();

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Could not load budgets'
    );

    await user.click(screen.getByRole('button', { name: /try again/i }));

    expect(useBudgetStore.getState().fetchBudgets).toHaveBeenCalled();
  });

  it('saves a budget from the editor', async () => {
    const user = userEvent.setup();
    renderSection();

    await user.click(screen.getByRole('button', { name: /edit budgets/i }));

    const dialog = screen.getByRole('dialog');
    await user.type(within(dialog).getByLabelText('Monthly limit'), '250');
    await user.click(within(dialog).getByRole('button', { name: /save budget/i }));

    await waitFor(() => {
      expect(useBudgetStore.getState().upsertBudget).toHaveBeenCalledWith(
        expenseCategory.id,
        250
      );
    });
  });

  it('removes an existing budget from the editor', async () => {
    const user = userEvent.setup();
    useBudgetStore.setState({ budgets: [underBudget] });
    renderSection();

    await user.click(screen.getByRole('button', { name: /edit budgets/i }));

    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByText(/currently 60% used/i)).toBeInTheDocument();
    await user.click(
      within(dialog).getByRole('button', { name: /remove budget/i })
    );

    await waitFor(() => {
      expect(useBudgetStore.getState().deleteBudget).toHaveBeenCalledWith(
        underBudget.category_id
      );
    });
  });
});
