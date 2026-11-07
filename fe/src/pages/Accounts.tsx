import { useEffect, useState } from 'react';
import { Plus, Wallet } from 'lucide-react';
import { useAccountStore } from '../stores/accountStore';
import type {
  Account,
  AdjustAccountRequest,
  CreateAccountRequest,
  UpdateAccountRequest,
} from '../types';
import { PageHeader } from '../components/ui/PageHeader';
import { Button } from '../components/ui/Button';
import { EmptyState, ErrorState, LoadingState } from '../components/ui/States';
import { ConfirmDialog } from '../components/ui/ConfirmDialog';
import { AccountFormModal } from '../components/accounts/AccountFormModal';
import { AccountCard } from '../components/accounts/AccountCard';
import { AdjustBalanceModal } from '../components/accounts/AdjustBalanceModal';

function Accounts() {
  const accounts = useAccountStore((state) => state.accounts);
  const isLoading = useAccountStore((state) => state.isLoading);
  const error = useAccountStore((state) => state.error);
  const fetchAccounts = useAccountStore((state) => state.fetchAccounts);
  const createAccount = useAccountStore((state) => state.createAccount);
  const updateAccount = useAccountStore((state) => state.updateAccount);
  const adjustBalance = useAccountStore((state) => state.adjustBalance);
  const deleteAccount = useAccountStore((state) => state.deleteAccount);

  const [modalOpen, setModalOpen] = useState(false);
  const [adjustTarget, setAdjustTarget] = useState<Account | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Account | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  useEffect(() => {
    void fetchAccounts();
  }, [fetchAccounts]);

  const handleCreate = async (data: CreateAccountRequest) => {
    await createAccount(data);
  };

  const handleUpdate = async (id: string, data: UpdateAccountRequest) => {
    await updateAccount(id, data);
  };

  const handleAdjust = async (id: string, data: AdjustAccountRequest) => {
    await adjustBalance(id, data);
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;

    setIsDeleting(true);
    setDeleteError(null);

    try {
      await deleteAccount(deleteTarget.id);
      setDeleteTarget(null);
    } catch (deleteFailure) {
      setDeleteError(
        deleteFailure instanceof Error
          ? deleteFailure.message
          : 'Unable to delete account'
      );
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
          <Button onClick={() => setModalOpen(true)}>
            <Plus className="h-4 w-4" aria-hidden="true" />
            Add account
          </Button>
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
              onDelete={(target) => {
                setDeleteError(null);
                setDeleteTarget(target);
              }}
            />
          ))}
        </section>
      )}

      <AccountFormModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        onSubmit={handleCreate}
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
            ? `“${deleteTarget.name}” and all of its transactions will be permanently removed.`
            : ''
        }
        isLoading={isDeleting}
        error={deleteError}
        onConfirm={() => void handleDelete()}
        onCancel={() => {
          if (isDeleting) return;
          setDeleteTarget(null);
          setDeleteError(null);
        }}
      />
    </div>
  );
}

export default Accounts;
