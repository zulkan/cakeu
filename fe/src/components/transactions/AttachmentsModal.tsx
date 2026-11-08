import { useCallback, useEffect, useRef, useState } from 'react';
import type { ChangeEvent } from 'react';
import { Download, Loader2, Paperclip, Trash2 } from 'lucide-react';
import type { Attachment, Transaction } from '../../types';
import { api } from '../../lib/api';
import { cn } from '../../lib/cn';
import { downloadBlob } from '../../lib/download';
import { formatDate } from '../../lib/format';
import { useToastStore } from '../../stores/toastStore';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { ConfirmDialog } from '../ui/ConfirmDialog';
import { EmptyState, ErrorState, LoadingState } from '../ui/States';
import { errorClass, hintClass, inputClass, labelClass } from '../ui/formStyles';

const MAX_ATTACHMENT_BYTES = 5 * 1024 * 1024;
const ACCEPTED_TYPES =
  'image/jpeg,image/png,image/webp,image/heic,application/pdf';

interface AttachmentsModalProps {
  open: boolean;
  transaction: Transaction | null;
  onClose: () => void;
}

function messageFor(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback;
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const kilobytes = bytes / 1024;
  if (kilobytes < 1024) return `${Math.round(kilobytes)} KB`;
  return `${(kilobytes / 1024).toFixed(1)} MB`;
}

export function AttachmentsModal({
  open,
  transaction,
  onClose,
}: AttachmentsModalProps) {
  const pushToast = useToastStore((state) => state.push);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [attachments, setAttachments] = useState<Attachment[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const [downloadingId, setDownloadingId] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Attachment | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const loadAttachments = useCallback(
    async (signal?: AbortSignal) => {
      if (!transaction) return;

      try {
        const result = await api.listAttachments(transaction.id, signal);
        if (signal?.aborted) return;
        setAttachments(result);
        setLoadError(null);
      } catch (failure) {
        if (signal?.aborted) return;
        setLoadError(messageFor(failure, 'Unable to load attachments'));
      }
    },
    [transaction]
  );

  useEffect(() => {
    if (!open || !transaction) return;

    const controller = new AbortController();

    api
      .listAttachments(transaction.id, controller.signal)
      .then((result) => {
        if (controller.signal.aborted) return;
        setAttachments(result);
        setLoadError(null);
      })
      .catch((failure: unknown) => {
        if (controller.signal.aborted) return;
        setLoadError(messageFor(failure, 'Unable to load attachments'));
      });

    return () => controller.abort();
  }, [open, transaction]);

  const handleClose = () => {
    setAttachments(null);
    setLoadError(null);
    setUploadError(null);
    setIsUploading(false);
    setDownloadingId(null);
    setDeleteTarget(null);
    if (fileInputRef.current) fileInputRef.current.value = '';
    onClose();
  };

  const handleRetry = async () => {
    setAttachments(null);
    setLoadError(null);
    await loadAttachments();
  };

  const handleFileChange = async (event: ChangeEvent<HTMLInputElement>) => {
    const selected = event.target.files?.[0] ?? null;
    event.target.value = '';
    if (!selected || !transaction) return;

    setUploadError(null);

    if (selected.size > MAX_ATTACHMENT_BYTES) {
      setUploadError(
        `“${selected.name}” is ${formatBytes(selected.size)}. Attachments must be 5 MB or smaller.`
      );
      return;
    }

    setIsUploading(true);

    try {
      await api.uploadAttachment(transaction.id, selected);
      await loadAttachments();
      pushToast({ title: 'Attachment uploaded', tone: 'success' });
    } catch (failure) {
      setUploadError(messageFor(failure, 'Unable to upload this file'));
    } finally {
      setIsUploading(false);
    }
  };

  const handleDownload = async (attachment: Attachment) => {
    setDownloadingId(attachment.id);

    try {
      const blob = await api.downloadAttachment(attachment.id);
      downloadBlob(blob, attachment.filename);
    } catch (failure) {
      pushToast({
        title: messageFor(failure, 'Unable to download this file'),
        tone: 'error',
      });
    } finally {
      setDownloadingId(null);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;

    setIsDeleting(true);

    try {
      await api.deleteAttachment(deleteTarget.id);
      setAttachments((current) =>
        current ? current.filter((item) => item.id !== deleteTarget.id) : current
      );
      setDeleteTarget(null);
      pushToast({ title: 'Attachment deleted', tone: 'success' });
    } catch (failure) {
      pushToast({
        title: messageFor(failure, 'Unable to delete attachment'),
        tone: 'error',
      });
    } finally {
      setIsDeleting(false);
    }
  };

  const fileName = transaction
    ? transaction.description || transaction.category_name
    : '';

  return (
    <>
      <Modal
        open={open}
        onClose={handleClose}
        title="Attachments"
        description={
          transaction ? `Receipts and documents for “${fileName}”.` : undefined
        }
        size="md"
        footer={
          <div className="flex justify-end">
            <Button
              variant="secondary"
              onClick={handleClose}
              disabled={isUploading}
            >
              Close
            </Button>
          </div>
        }
      >
        <div className="space-y-5">
          <div className="rounded-2xl border border-dashed border-warm-300 bg-warm-50/60 p-4">
            <label htmlFor="attachments-file" className={labelClass}>
              Upload attachment
            </label>
            <input
              ref={fileInputRef}
              id="attachments-file"
              type="file"
              accept={ACCEPTED_TYPES}
              onChange={(event) => void handleFileChange(event)}
              disabled={isUploading}
              aria-describedby="attachments-file-hint"
              className={cn(
                inputClass,
                'mt-1.5 file:mr-3 file:rounded-lg file:border-0 file:bg-brand-50 file:px-3 file:py-1.5 file:text-sm file:font-semibold file:text-brand-700 hover:file:bg-brand-100'
              )}
            />
            <p id="attachments-file-hint" className={hintClass}>
              JPEG, PNG, WebP, HEIC, or PDF up to 5 MB.
            </p>
            {isUploading && (
              <p
                role="status"
                className="mt-2 flex items-center gap-2 text-sm font-medium text-stone-600"
              >
                <Loader2
                  className="h-4 w-4 animate-spin text-brand-500"
                  aria-hidden="true"
                />
                Uploading…
              </p>
            )}
            {uploadError && (
              <p role="alert" className={`mt-2 ${errorClass}`}>
                {uploadError}
              </p>
            )}
          </div>

          {loadError ? (
            <ErrorState
              message={loadError}
              onRetry={() => void handleRetry()}
            />
          ) : attachments === null ? (
            <LoadingState label="Loading attachments…" />
          ) : attachments.length === 0 ? (
            <EmptyState
              icon={<Paperclip className="h-6 w-6" aria-hidden="true" />}
              title="No attachments yet"
              description="Upload a receipt or supporting document to keep it with this transaction."
            />
          ) : (
            <ul
              aria-label="Attachments"
              className="space-y-2"
            >
              {attachments.map((attachment) => (
                <li
                  key={attachment.id}
                  className="flex items-center justify-between gap-3 rounded-2xl border border-warm-200 bg-surface-card px-4 py-3"
                >
                  <div className="min-w-0">
                    <p className="truncate text-sm font-semibold text-stone-900">
                      {attachment.filename}
                    </p>
                    <p className="mt-0.5 text-xs text-stone-500">
                      {formatBytes(attachment.size_bytes)} ·{' '}
                      {formatDate(attachment.created_at)}
                    </p>
                  </div>
                  <div className="flex shrink-0 items-center gap-1">
                    <button
                      type="button"
                      onClick={() => void handleDownload(attachment)}
                      disabled={downloadingId === attachment.id}
                      aria-label={`Download ${attachment.filename}`}
                      className="rounded-lg p-2 text-stone-400 transition-colors hover:bg-brand-50 hover:text-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {downloadingId === attachment.id ? (
                        <Loader2
                          className="h-4 w-4 animate-spin"
                          aria-hidden="true"
                        />
                      ) : (
                        <Download className="h-4 w-4" aria-hidden="true" />
                      )}
                    </button>
                    <button
                      type="button"
                      onClick={() => setDeleteTarget(attachment)}
                      aria-label={`Delete ${attachment.filename}`}
                      className="rounded-lg p-2 text-stone-400 transition-colors hover:bg-expense-light hover:text-expense focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
                    >
                      <Trash2 className="h-4 w-4" aria-hidden="true" />
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      </Modal>

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Delete attachment?"
        description={
          deleteTarget
            ? `“${deleteTarget.filename}” will be permanently removed from this transaction.`
            : ''
        }
        isLoading={isDeleting}
        onConfirm={() => void handleDelete()}
        onCancel={() => {
          if (isDeleting) return;
          setDeleteTarget(null);
        }}
      />
    </>
  );
}
