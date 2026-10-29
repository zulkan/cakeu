import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import Categories from '../Categories';
import { useCategoryStore } from '../../stores/categoryStore';
import type { Category } from '../../types';

const incomeCategory: Category = {
  id: 'category-income',
  name: 'Salary',
  type: 'income',
  color: '#16A34A',
  created_at: '2026-01-01T00:00:00Z',
};

const expenseCategory: Category = {
  id: 'category-expense',
  name: 'Groceries',
  type: 'expense',
  color: '#FB923C',
  created_at: '2026-01-01T00:00:00Z',
};

function renderCategories() {
  return render(
    <MemoryRouter>
      <Categories />
    </MemoryRouter>
  );
}

describe('Categories page', () => {
  beforeEach(() => {
    useCategoryStore.setState({
      categories: [incomeCategory, expenseCategory],
      isLoading: false,
      error: null,
      fetchCategories: vi.fn(async () => undefined),
      createCategory: vi.fn(async () => expenseCategory),
      deleteCategory: vi.fn(async () => undefined),
    });
  });

  it('groups categories into income and expense', () => {
    renderCategories();

    expect(screen.getByRole('heading', { name: 'Income' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Expense' })).toBeInTheDocument();
    expect(screen.getByText('Salary')).toBeInTheDocument();
    expect(screen.getByText('Groceries')).toBeInTheDocument();
  });

  it('creates a category with a type and color', async () => {
    const user = userEvent.setup();
    renderCategories();

    await user.click(screen.getByRole('button', { name: /add category/i }));

    const dialog = screen.getByRole('dialog');
    await user.type(within(dialog).getByLabelText('Name'), 'Freelance');
    await user.click(within(dialog).getByRole('button', { name: 'Income' }));
    await user.click(
      within(dialog).getByRole('button', { name: /create category/i })
    );

    const createCategory = vi.mocked(
      useCategoryStore.getState().createCategory
    );

    await waitFor(() => {
      expect(createCategory).toHaveBeenCalledWith({
        name: 'Freelance',
        type: 'income',
        color: '#FB923C',
      });
    });

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });

  it('deletes a category after confirmation', async () => {
    const user = userEvent.setup();
    renderCategories();

    await user.click(
      screen.getByRole('button', { name: /delete category groceries/i })
    );

    const dialog = screen.getByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    const deleteCategory = vi.mocked(
      useCategoryStore.getState().deleteCategory
    );

    await waitFor(() => {
      expect(deleteCategory).toHaveBeenCalledWith(expenseCategory.id);
    });
  });

  it('shows an empty state when there are no categories', () => {
    useCategoryStore.setState({ categories: [] });
    renderCategories();

    expect(screen.getByText('No categories yet')).toBeInTheDocument();
  });

  it('shows a loading state', () => {
    useCategoryStore.setState({ categories: [], isLoading: true });
    renderCategories();

    expect(screen.getByText('Loading categories…')).toBeInTheDocument();
  });

  it('shows an error state with retry', () => {
    useCategoryStore.setState({
      categories: [],
      error: 'Could not load categories',
    });
    renderCategories();

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Could not load categories'
    );
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument();
  });
});
