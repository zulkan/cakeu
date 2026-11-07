import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import Accounts from '../Accounts';
import { useAccountStore } from '../../stores/accountStore';
import type { Account } from '../../types';

const account: Account = {
  id: 'account-1',
  name: 'Everyday checking',
  balance: 1250,
  opening_balance: 1000,
  entry_count: 1,
  currency: 'USD',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

function renderAccounts() {
  return render(
    <MemoryRouter>
      <Accounts />
    </MemoryRouter>
  );
}

describe('Accounts page', () => {
  beforeEach(() => {
    useAccountStore.setState({
      accounts: [account],
      isLoading: false,
      error: null,
      fetchAccounts: vi.fn(async () => undefined),
      createAccount: vi.fn(async () => account),
      updateAccount: vi.fn(async () => undefined),
      adjustBalance: vi.fn(async () => undefined),
      deleteAccount: vi.fn(async () => undefined),
    });
  });

  it('renders account cards with balance and currency', () => {
    renderAccounts();

    expect(screen.getByText('Everyday checking')).toBeInTheDocument();
    expect(screen.getByText('$1,250.00')).toBeInTheDocument();
    expect(screen.getByText('USD')).toBeInTheDocument();
    expect(screen.getByText('Current balance')).toBeInTheDocument();
  });

  it('creates an account through the modal', async () => {
    const user = userEvent.setup();
    renderAccounts();

    await user.click(screen.getByRole('button', { name: /add account/i }));

    const dialog = screen.getByRole('dialog');
    await user.type(within(dialog).getByLabelText('Name'), 'Savings');
    await user.clear(within(dialog).getByLabelText('Opening balance'));
    await user.type(within(dialog).getByLabelText('Opening balance'), '500');
    await user.selectOptions(within(dialog).getByLabelText('Currency'), 'EUR');
    await user.click(
      within(dialog).getByRole('button', { name: /create account/i })
    );

    const createAccount = vi.mocked(
      useAccountStore.getState().createAccount
    );

    await waitFor(() => {
      expect(createAccount).toHaveBeenCalledWith({
        name: 'Savings',
        opening_balance: 500,
        currency: 'EUR',
      });
    });

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });

  it('renames an account without a balance field', async () => {
    const user = userEvent.setup();
    renderAccounts();

    await user.click(screen.getByRole('button', { name: /edit/i }));

    const nameInput = screen.getByLabelText('Name');

    await user.clear(nameInput);
    await user.type(nameInput, 'Main checking');
    await user.click(screen.getByRole('button', { name: /save/i }));

    const updateAccount = vi.mocked(
      useAccountStore.getState().updateAccount
    );

    await waitFor(() => {
      expect(updateAccount).toHaveBeenCalledWith(account.id, {
        name: 'Main checking',
      });
    });

    await waitFor(() => {
      expect(
        screen.queryByRole('button', { name: /save/i })
      ).not.toBeInTheDocument();
    });
  });

  it('records an adjustment from the adjust balance modal', async () => {
    const user = userEvent.setup();
    renderAccounts();

    await user.click(
      screen.getByRole('button', { name: /adjust balance/i })
    );

    const dialog = screen.getByRole('dialog');
    const target = within(dialog).getByLabelText('Actual balance');
    await user.clear(target);
    await user.type(target, '2000');
    await user.click(
      within(dialog).getByRole('button', { name: /record adjustment/i })
    );

    const adjustBalance = vi.mocked(
      useAccountStore.getState().adjustBalance
    );

    await waitFor(() => {
      expect(adjustBalance).toHaveBeenCalledWith(account.id, {
        target_balance: 2000,
        description: undefined,
      });
    });
  });

  it('deletes an account after confirmation', async () => {
    const user = userEvent.setup();
    renderAccounts();

    await user.click(screen.getByRole('button', { name: /delete/i }));

    const dialog = screen.getByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    const deleteAccount = vi.mocked(
      useAccountStore.getState().deleteAccount
    );

    await waitFor(() => {
      expect(deleteAccount).toHaveBeenCalledWith(account.id);
    });
  });

  it('shows an empty state when there are no accounts', () => {
    useAccountStore.setState({ accounts: [] });
    renderAccounts();

    expect(screen.getByText('No accounts yet')).toBeInTheDocument();
  });

  it('shows a loading state', () => {
    useAccountStore.setState({ accounts: [], isLoading: true });
    renderAccounts();

    expect(screen.getByText('Loading accounts…')).toBeInTheDocument();
  });

  it('shows an error state with retry', () => {
    useAccountStore.setState({
      accounts: [],
      error: 'Could not load accounts',
    });
    renderAccounts();

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Could not load accounts'
    );
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument();
  });
});
