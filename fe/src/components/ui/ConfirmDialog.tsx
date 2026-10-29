import { AlertTriangle } from 'lucide-react';
import { Button } from './Button';
import { Modal } from './Modal';

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  description: string;
  confirmLabel?: string;
  isLoading?: boolean;
  error?: string | null;
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmDialog({
  open,
  title,
  description,
  confirmLabel = 'Delete',
  isLoading = false,
  error,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  return (
    <Modal
      open={open}
      onClose={onCancel}
      title={title}
      size="sm"
      footer={
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button variant="secondary" onClick={onCancel} disabled={isLoading}>
            Cancel
          </Button>
          <Button variant="danger" onClick={onConfirm} disabled={isLoading}>
            {isLoading ? 'Deleting…' : confirmLabel}
          </Button>
        </div>
      }
    >
      <div className="flex gap-3 rounded-2xl bg-expense-light/60 p-4">
        <AlertTriangle
          className="mt-0.5 h-5 w-5 shrink-0 text-expense"
          aria-hidden="true"
        />
        <p className="text-sm text-stone-700">{description}</p>
      </div>
      {error && (
        <p
          role="alert"
          className="mt-3 rounded-xl border border-expense/20 bg-expense-light/60 px-4 py-3 text-sm font-medium text-expense"
        >
          {error}
        </p>
      )}
    </Modal>
  );
}
