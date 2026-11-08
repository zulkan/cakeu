import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import Accounts from '../Accounts';
import { useAccountStore } from '../../stores/accountStore';
import { api } from '../../lib/api';
import type { Account, Transaction, TransferResult } from '../../types';

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

const savings: Account = {
  ...account,
  id: 'account-2',
  name: 'Savings',
  balance: 5000,
  opening_balance: 5000,
};

const euroWallet: Account = {
  ...account,
  id: 'account-3',
  name: 'Euro wallet',
  currency: 'EUR',
};

const transferLeg: Transaction = {
  id: 'transfer-leg-1',
  account_id: account.id,
  category_id: '00000000-0000-0000-0000-000000000000',
  type: 'expense',
  amount: 25.5,
  description: 'Move to savings',
  date: '2026-11-08T00:00:00Z',
  created_at: '2026-11-08T00:00:00Z',
  account_name: account.name,
  category_name: '',
  kind: 'transfer',
  currency: 'USD',
  transfer_group_id: 'group-1',
};

const transferResult: TransferResult = {
  transfer_group_id: 'group-1',
  from: transferLeg,
  to: {
    ...transferLeg,
    id: 'transfer-leg-2',
    account_id: savings.id,
    type: 'income',
    account_name: savings.name,
  },
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

  it('names the delete action after the account', () => {
    renderAccounts();

    expect(
      screen.getByRole('button', { name: 'Delete Everyday checking' })
    ).toBeInTheDocument();
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

  it('disables transfer without a same-currency pair and explains why', () => {
    renderAccounts();

    expect(screen.getByRole('button', { name: 'Transfer' })).toBeDisabled();
    expect(
      screen.getByText(
        'Add a second account in the same currency to transfer.'
      )
    ).toBeInTheDocument();
  });

  it('filters transfer destinations by currency and submits a transfer', async () => {
    const user = userEvent.setup();
    useAccountStore.setState({ accounts: [account, savings, euroWallet] });
    const createTransfer = vi
      .spyOn(api, 'createTransfer')
      .mockResolvedValue(transferResult);
    const fetchAccounts = vi.mocked(
      useAccountStore.getState().fetchAccounts
    );

    renderAccounts();
    fetchAccounts.mockClear();

    await user.click(screen.getByRole('button', { name: 'Transfer' }));

    const dialog = screen.getByRole('dialog');
    const fromSelect = within(dialog).getByLabelText('From account');
    const toSelect = within(dialog).getByLabelText('To account');

    expect(
      within(toSelect).getByRole('option', { name: 'Savings' })
    ).toBeInTheDocument();
    expect(
      within(toSelect).queryByRole('option', { name: 'Everyday checking' })
    ).not.toBeInTheDocument();
    expect(
      within(toSelect).queryByRole('option', { name: 'Euro wallet' })
    ).not.toBeInTheDocument();

    await user.selectOptions(fromSelect, savings.id);

    expect(
      within(toSelect).getByRole('option', { name: 'Everyday checking' })
    ).toBeInTheDocument();
    expect(
      within(toSelect).queryByRole('option', { name: 'Savings' })
    ).not.toBeInTheDocument();

    await user.type(within(dialog).getByLabelText(/Amount/), '25.50');
    await user.type(
      within(dialog).getByLabelText(/Description/),
      'Move to checking'
    );
    await user.click(
      within(dialog).getByRole('button', { name: /transfer funds/i })
    );

    await waitFor(() => {
      expect(createTransfer).toHaveBeenCalledWith(
        expect.objectContaining({
          from_account_id: savings.id,
          to_account_id: account.id,
          amount: 25.5,
          description: 'Move to checking',
          date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
        })
      );
    });

    await waitFor(() => {
      expect(fetchAccounts).toHaveBeenCalled();
    });
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
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
