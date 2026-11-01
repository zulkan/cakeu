import { useRef, useState } from 'react';
import type { ChangeEvent } from 'react';
import { AlertCircle, CheckCircle2, FileUp, Loader2 } from 'lucide-react';
import type { ImportResult } from '../../types';
import { api } from '../../lib/api';
import { cn } from '../../lib/cn';
import { formatCurrency, formatDate } from '../../lib/format';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { Select } from '../ui/Select';
import { TypeBadge } from '../ui/TypeBadge';
import { errorClass, hintClass, inputClass, labelClass } from '../ui/formStyles';

const PREVIEW_ROW_LIMIT = 10;

interface ImportCsvDialogProps {
  open: boolean;
  currency: string;
  onClose: () => void;
  onImported: () => Promise<void>;
}

function messageFor(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback;
}

export function ImportCsvDialog({
  open,
  currency,
  onClose,
  onImported,
}: ImportCsvDialogProps) {
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<ImportResult | null>(null);
  const [success, setSuccess] = useState<ImportResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [selectedSheet, setSelectedSheet] = useState<string | null>(null);
  const [sheets, setSheets] = useState<string[]>([]);
  const [isPreviewing, setIsPreviewing] = useState(false);
  const [isCommitting, setIsCommitting] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const requestId = useRef(0);

  const reset = () => {
    requestId.current += 1;
    setFile(null);
    setPreview(null);
    setSuccess(null);
    setError(null);
    setSelectedSheet(null);
    setSheets([]);
    setIsPreviewing(false);
    setIsCommitting(false);
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  const handleClose = () => {
    if (isCommitting) return;
    reset();
    onClose();
  };

  const runPreview = async (target: File, sheet?: string) => {
    const currentRequest = requestId.current + 1;
    requestId.current = currentRequest;

    setIsPreviewing(true);
    setError(null);

    try {
      const result = sheet
        ? await api.previewImport(target, sheet)
        : await api.previewImport(target);
      if (requestId.current !== currentRequest) return;
      setPreview(result);
      setSheets(result.sheets);
      setSelectedSheet(result.sheet);
    } catch (failure) {
      if (requestId.current === currentRequest) {
        setPreview(null);
        setError(messageFor(failure, 'Unable to preview this file'));
      }
    } finally {
      if (requestId.current === currentRequest) setIsPreviewing(false);
    }
  };

  const handleFileChange = async (event: ChangeEvent<HTMLInputElement>) => {
    const selected = event.target.files?.[0] ?? null;

    setFile(selected);
    setPreview(null);
    setSuccess(null);
    setError(null);
    setSelectedSheet(null);
    setSheets([]);

    if (!selected) {
      requestId.current += 1;
      setIsPreviewing(false);
      return;
    }

    await runPreview(selected);
  };

  const handleSheetChange = async (event: ChangeEvent<HTMLSelectElement>) => {
    const name = event.target.value;
    setSelectedSheet(name);
    if (!file) return;
    await runPreview(file, name);
  };

  const handleCommit = async () => {
    if (!file || !preview || preview.valid === 0) return;

    setIsCommitting(true);
    setError(null);

    try {
      const result =
        selectedSheet && sheets.length > 1
          ? await api.importTransactions(file, selectedSheet)
          : await api.importTransactions(file);
      setSuccess(result);
      await onImported();
    } catch (failure) {
      setError(messageFor(failure, 'Unable to import transactions'));
    } finally {
      setIsCommitting(false);
    }
  };

  const canCommit =
    Boolean(file) &&
    Boolean(preview) &&
    (preview?.valid ?? 0) > 0 &&
    !isPreviewing &&
    !isCommitting;

  const footer = success ? (
    <div className="flex justify-end">
      <Button onClick={handleClose}>Close</Button>
    </div>
  ) : (
    <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
      <Button
        variant="secondary"
        onClick={handleClose}
        disabled={isPreviewing || isCommitting}
      >
        Cancel
      </Button>
      <Button onClick={() => void handleCommit()} disabled={!canCommit}>
        {isCommitting ? (
          <>
            <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
            Importing…
          </>
        ) : (
          'Import transactions'
        )}
      </Button>
    </div>
  );

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title="Import transactions"
      description="Review a CSV or Excel workbook before adding it to your ledger."
      size="lg"
      footer={footer}
    >
      {success ? (
        <div className="flex items-start gap-3 rounded-2xl border border-income/20 bg-income-light/60 p-4">
          <CheckCircle2
            className="mt-0.5 h-5 w-5 shrink-0 text-income"
            aria-hidden="true"
          />
          <div>
            <p className="text-sm font-semibold text-stone-900">
              Import complete
            </p>
            <p className="mt-0.5 text-sm text-stone-700">
              Imported {success.valid} of {success.total}{' '}
              {success.total === 1 ? 'transaction' : 'transactions'}.
            </p>
            {success.invalid > 0 && (
              <p className="mt-1 text-sm text-expense">
                {success.invalid}{' '}
                {success.invalid === 1 ? 'row was' : 'rows were'} skipped.
              </p>
            )}
          </div>
        </div>
      ) : (
        <div className="space-y-5">
          <div className="space-y-1.5">
            <label htmlFor="import-csv-file" className={labelClass}>
              CSV file
            </label>
            <input
              ref={fileInputRef}
              id="import-csv-file"
              type="file"
              accept=".csv,.xlsx,.xls"
              onChange={(event) => void handleFileChange(event)}
              disabled={isPreviewing || isCommitting}
              aria-describedby="import-csv-hint"
              className={cn(
                inputClass,
                'file:mr-3 file:rounded-lg file:border-0 file:bg-brand-50 file:px-3 file:py-1.5 file:text-sm file:font-semibold file:text-brand-700 hover:file:bg-brand-100'
              )}
            />
            <p id="import-csv-hint" className={hintClass}>
              One .csv, .xlsx, or .xls file up to 5 MB and 5,000 rows per sheet.
              Required columns: date, type, amount, description, account,
              category.
            </p>
          </div>

          {sheets.length > 1 && (
            <div className="space-y-1.5">
              <label htmlFor="import-csv-sheet" className={labelClass}>
                Sheet
              </label>
              <Select
                id="import-csv-sheet"
                value={selectedSheet ?? sheets[0] ?? ''}
                onChange={(event) => void handleSheetChange(event)}
                disabled={isPreviewing || isCommitting}
              >
                {sheets.map((name) => (
                  <option key={name} value={name}>
                    {name}
                  </option>
                ))}
              </Select>
              <p className={hintClass}>
                This workbook has {sheets.length} sheets. Pick one to validate.
              </p>
            </div>
          )}

          {isPreviewing && (
            <div
              role="status"
              className="flex items-center gap-2 rounded-xl border border-warm-200 bg-warm-50/60 px-4 py-3 text-sm font-medium text-stone-600"
            >
              <Loader2
                className="h-4 w-4 animate-spin text-brand-500"
                aria-hidden="true"
              />
              Checking rows…
            </div>
          )}

          {error && (
            <p
              role="alert"
              className="rounded-xl border border-expense/20 bg-expense-light/60 px-4 py-3 text-sm font-medium text-expense"
            >
              {error}
            </p>
          )}

          {preview && (
            <>
              <dl
                aria-label="Import summary"
                className="grid grid-cols-3 gap-2"
              >
                {[
                  { label: 'Total', value: preview.total, tone: 'text-stone-900' },
                  { label: 'Valid', value: preview.valid, tone: 'text-income' },
                  {
                    label: 'Invalid',
                    value: preview.invalid,
                    tone: preview.invalid > 0 ? 'text-expense' : 'text-stone-900',
                  },
                ].map((stat) => (
                  <div
                    key={stat.label}
                    className="rounded-xl border border-warm-200 bg-warm-50/60 px-3 py-2"
                  >
                    <dt className="text-xs font-semibold uppercase tracking-wide text-stone-400">
                      {stat.label}
                    </dt>
                    <dd className={cn('mt-0.5 text-lg font-bold', stat.tone)}>
                      {stat.value}
                    </dd>
                  </div>
                ))}
              </dl>

              {preview.errors.length > 0 && (
                <section className="space-y-1.5">
                  <h3 className="flex items-center gap-1.5 text-sm font-semibold text-stone-700">
                    <AlertCircle
                      className="h-4 w-4 text-expense"
                      aria-hidden="true"
                    />
                    Row errors
                  </h3>
                  <ul className="max-h-40 space-y-1 overflow-y-auto rounded-xl border border-expense/20 bg-expense-light/30 p-3">
                    {preview.errors.map((rowError) => (
                      <li
                        key={`${rowError.row}-${rowError.message}`}
                        className="text-sm text-stone-700"
                      >
                        <span className={cn('font-semibold', errorClass)}>
                          Row {rowError.row}
                        </span>
                        <span className="text-stone-500">
                          {' '}
                          — {rowError.message}
                        </span>
                      </li>
                    ))}
                  </ul>
                </section>
              )}

              {preview.preview.length > 0 && (
                <section className="space-y-1.5">
                  <h3 className="text-sm font-semibold text-stone-700">
                    Preview
                  </h3>
                  <div className="overflow-x-auto rounded-xl border border-warm-200">
                    <table className="w-full text-left text-sm">
                      <thead>
                        <tr className="border-b border-warm-200 bg-warm-50 text-xs uppercase tracking-wide text-stone-400">
                          <th scope="col" className="px-3 py-2 font-semibold">
                            Date
                          </th>
                          <th scope="col" className="px-3 py-2 font-semibold">
                            Type
                          </th>
                          <th
                            scope="col"
                            className="px-3 py-2 text-right font-semibold"
                          >
                            Amount
                          </th>
                          <th scope="col" className="px-3 py-2 font-semibold">
                            Description
                          </th>
                          <th scope="col" className="px-3 py-2 font-semibold">
                            Account
                          </th>
                          <th scope="col" className="px-3 py-2 font-semibold">
                            Category
                          </th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-warm-100">
                        {preview.preview
                          .slice(0, PREVIEW_ROW_LIMIT)
                          .map((row) => (
                            <tr key={row.row}>
                              <td className="whitespace-nowrap px-3 py-2 text-stone-500">
                                {formatDate(row.date)}
                              </td>
                              <td className="whitespace-nowrap px-3 py-2">
                                <TypeBadge type={row.type} />
                              </td>
                              <td className="whitespace-nowrap px-3 py-2 text-right font-semibold text-stone-800">
                                {formatCurrency(row.amount, currency)}
                              </td>
                              <td className="max-w-[12rem] truncate px-3 py-2 text-stone-700">
                                {row.description || '—'}
                              </td>
                              <td className="whitespace-nowrap px-3 py-2 text-stone-600">
                                {row.account}
                              </td>
                              <td className="whitespace-nowrap px-3 py-2 text-stone-600">
                                {row.category}
                              </td>
                            </tr>
                          ))}
                      </tbody>
                    </table>
                  </div>
                  {preview.preview.length > PREVIEW_ROW_LIMIT && (
                    <p className={hintClass}>
                      Showing the first {PREVIEW_ROW_LIMIT} of{' '}
                      {preview.preview.length} validated rows.
                    </p>
                  )}
                </section>
              )}

              {preview.preview.length === 0 && preview.errors.length === 0 && (
                <p className="rounded-xl bg-warm-50 px-4 py-3 text-sm text-stone-600">
                  This file has no rows to import.
                </p>
              )}
            </>
          )}

          {!file && (
            <div className="flex items-center gap-2 rounded-xl bg-warm-50 px-4 py-3 text-sm text-stone-600">
              <FileUp className="h-4 w-4 text-brand-500" aria-hidden="true" />
              Choose a CSV, XLSX, or XLS file to validate its rows.
            </div>
          )}
        </div>
      )}
    </Modal>
  );
}
