import { useEffect, useState } from 'react';
import { ArrowLeftRight, Plus, Wallet } from 'lucide-react';
import { useAccountStore } from '../stores/accountStore';
import { useToastStore } from '../stores/toastStore';
import type {
  Account,
  AdjustAccountRequest,
  CreateAccountRequest,
  CreateTransferRequest,
  UpdateAccountRequest,
} from '../types';
import { api } from '../lib/api';
import { PageHeader } from '../components/ui/PageHeader';
import { Button } from '../components/ui/Button';
import { EmptyState, ErrorState, LoadingState } from '../components/ui/States';
import { ConfirmDialog } from '../components/ui/ConfirmDialog';
import { hintClass } from '../components/ui/formStyles';
import { AccountFormModal } from '../components/accounts/AccountFormModal';
import { AccountCard } from '../components/accounts/AccountCard';
import { AdjustBalanceModal } from '../components/accounts/AdjustBalanceModal';
import { TransferModal } from '../components/accounts/TransferModal';

function messageOf(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function hasTransferPair(accounts: Account[]): boolean {
  return accounts.some((account) =>
    accounts.some(
      (other) =>
        other.id !== account.id && other.currency === account.currency
    )
  );
}

function Accounts() {
  const accounts = useAccountStore((state) => state.accounts);
  const isLoading = useAccountStore((state) => state.isLoading);
  const error = useAccountStore((state) => state.error);
  const fetchAccounts = useAccountStore((state) => state.fetchAccounts);
  const createAccount = useAccountStore((state) => state.createAccount);
  const updateAccount = useAccountStore((state) => state.updateAccount);
  const adjustBalance = useAccountStore((state) => state.adjustBalance);
  const deleteAccount = useAccountStore((state) => state.deleteAccount);
  const pushToast = useToastStore((state) => state.push);

  const [modalOpen, setModalOpen] = useState(false);
  const [transferOpen, setTransferOpen] = useState(false);
  const [adjustTarget, setAdjustTarget] = useState<Account | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Account | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const canTransfer = hasTransferPair(accounts);

  useEffect(() => {
    void fetchAccounts();
  }, [fetchAccounts]);

  const handleCreate = async (data: CreateAccountRequest) => {
    try {
      await createAccount(data);
      pushToast({ title: 'Account added', tone: 'success' });
    } catch (failure) {
      pushToast({ title: messageOf(failure, 'Unable to add account'), tone: 'error' });
      throw failure;
    }
  };

  const handleUpdate = async (id: string, data: UpdateAccountRequest) => {
    try {
      await updateAccount(id, data);
      pushToast({ title: 'Account updated', tone: 'success' });
    } catch (failure) {
      pushToast({ title: messageOf(failure, 'Unable to update account'), tone: 'error' });
      throw failure;
    }
  };

  const handleTransfer = async (data: CreateTransferRequest) => {
    try {
      await api.createTransfer(data);
      await fetchAccounts();
      pushToast({ title: 'Transfer completed', tone: 'success' });
    } catch (failure) {
      pushToast({
        title: messageOf(failure, 'Unable to transfer funds'),
        tone: 'error',
      });
      throw failure;
    }
  };

  const handleAdjust = async (id: string, data: AdjustAccountRequest) => {
    try {
      await adjustBalance(id, data);
      pushToast({ title: 'Balance adjusted', tone: 'success' });
    } catch (failure) {
      pushToast({ title: messageOf(failure, 'Unable to adjust balance'), tone: 'error' });
      throw failure;
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;

    setIsDeleting(true);

    try {
      await deleteAccount(deleteTarget.id);
      setDeleteTarget(null);
      pushToast({ title: 'Account deleted', tone: 'success' });
    } catch (deleteFailure) {
      pushToast({
        title: messageOf(deleteFailure, 'Unable to delete account'),
        tone: 'error',
      });
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Accounts"
        description="Every wallet and bank balance in one place."
        action={
          <div className="flex flex-col gap-2 sm:items-end">
            <div className="flex flex-wrap items-center gap-2">
              <Button
                variant="secondary"
                disabled={!canTransfer}
                onClick={() => setTransferOpen(true)}
              >
                <ArrowLeftRight className="h-4 w-4" aria-hidden="true" />
                Transfer
              </Button>
              <Button onClick={() => setModalOpen(true)}>
                <Plus className="h-4 w-4" aria-hidden="true" />
                Add account
              </Button>
            </div>
            {!canTransfer && (
              <p className={hintClass}>
                Add a second account in the same currency to transfer.
              </p>
            )}
          </div>
        }
      />

      {isLoading && accounts.length === 0 ? (
        <LoadingState label="Loading accounts…" />
      ) : error && accounts.length === 0 ? (
        <ErrorState message={error} onRetry={() => void fetchAccounts()} />
      ) : accounts.length === 0 ? (
        <EmptyState
          icon={<Wallet className="h-6 w-6" aria-hidden="true" />}
          title="No accounts yet"
          description="Add your first account so transactions have somewhere to live."
          action={
            <Button onClick={() => setModalOpen(true)}>
              <Plus className="h-4 w-4" aria-hidden="true" />
              Add account
            </Button>
          }
        />
      ) : (
        <section
          aria-label="Your accounts"
          className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3"
        >
          {accounts.map((account) => (
            <AccountCard
              key={account.id}
              account={account}
              onUpdate={handleUpdate}
              onAdjust={setAdjustTarget}
              onDelete={setDeleteTarget}
            />
          ))}
        </section>
      )}

      <AccountFormModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        onSubmit={handleCreate}
      />

      <TransferModal
        open={transferOpen}
        onClose={() => setTransferOpen(false)}
        accounts={accounts}
        onSubmit={handleTransfer}
      />

      {adjustTarget && (
        <AdjustBalanceModal
          key={adjustTarget.id}
          open
          account={adjustTarget}
          onClose={() => setAdjustTarget(null)}
          onSubmit={handleAdjust}
        />
      )}

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Delete account?"
        description={
          deleteTarget
            ? `“${deleteTarget.name}” and all of its transactions will be permanently removed. Accounts with transfers cannot be deleted until those transfers are removed.`
            : ''
        }
        isLoading={isDeleting}
        onConfirm={() => void handleDelete()}
        onCancel={() => {
          if (isDeleting) return;
          setDeleteTarget(null);
        }}
      />
    </div>
  );
}

export default Accounts;
