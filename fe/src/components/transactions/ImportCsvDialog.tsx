import { useRef, useState } from 'react';
import type { ChangeEvent } from 'react';
import {
  AlertCircle,
  CheckCircle2,
  ChevronDown,
  Copy,
  FileUp,
  Loader2,
} from 'lucide-react';
import type {
  ImportFieldRule,
  ImportMapping,
  ImportMode,
  ImportResult,
} from '../../types';
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

function ruleDetails(rule: ImportFieldRule): string {
  const details: string[] = [];
  if (rule.date_format) details.push(`date ${rule.date_format}`);
  if (rule.amount_format) details.push(`amount ${rule.amount_format}`);
  if (rule.amount_mode) details.push(`mode ${rule.amount_mode}`);
  if (rule.carry_forward) details.push('carry forward');
  if (rule.value_map && Object.keys(rule.value_map).length > 0) {
    details.push(
      `value_map ${Object.entries(rule.value_map)
        .map(([from, to]) => `${from} → ${to}`)
        .join(', ')}`
    );
  }
  return details.join('; ');
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
  const [mapping, setMapping] = useState<ImportMapping | null>(null);
  const [mode, setMode] = useState<ImportMode>('ai');
  const [mappingOpen, setMappingOpen] = useState(true);
  const [allowDuplicates, setAllowDuplicates] = useState(false);
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
    setMapping(null);
    setMode('ai');
    setMappingOpen(true);
    setAllowDuplicates(false);
    setIsPreviewing(false);
    setIsCommitting(false);
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  const handleClose = () => {
    if (isCommitting) return;
    reset();
    onClose();
  };

  const runPreview = async (
    target: File,
    sheet?: string,
    nextMode: ImportMode = mode
  ) => {
    const currentRequest = requestId.current + 1;
    requestId.current = currentRequest;

    setIsPreviewing(true);
    setError(null);

    try {
      const result = await api.previewImport(target, { sheet, mode: nextMode });
      if (requestId.current !== currentRequest) return;
      setPreview(result);
      setSheets(result.sheets);
      setSelectedSheet(result.sheet);
      setMapping(result.mapping ?? null);
      setMode(result.mode ?? nextMode);
      setAllowDuplicates(false);
    } catch (failure) {
      if (requestId.current === currentRequest) {
        // Keep the workbook's sheet list and selection so Try again and
        // Standard import retry the sheet the user actually chose.
        setPreview(null);
        setMapping(null);
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
    setMapping(null);
    setMode('ai');
    setMappingOpen(true);
    setAllowDuplicates(false);

    if (!selected) {
      requestId.current += 1;
      setIsPreviewing(false);
      return;
    }

    await runPreview(selected, undefined, 'ai');
  };

  const handleSheetChange = async (event: ChangeEvent<HTMLSelectElement>) => {
    const name = event.target.value;
    setSelectedSheet(name);
    if (!file) return;
    await runPreview(file, name, mode);
  };

  const handleCommit = async () => {
    if (!file || !preview || preview.valid === 0) return;

    setIsCommitting(true);
    setError(null);

    try {
      const result = await api.importTransactions(file, {
        sheet: selectedSheet ?? undefined,
        mode,
        ...(mode === 'ai' && mapping ? { mapping } : {}),
        ...(allowDuplicates && preview.duplicate_count > 0
          ? { allow_duplicates: true }
          : {}),
      });
      setSuccess(result);
      await onImported();
    } catch (failure) {
      setError(messageFor(failure, 'Unable to import transactions'));
    } finally {
      setIsCommitting(false);
    }
  };

  const handleStandardImport = async () => {
    if (!file) return;
    setMode('strict');
    await runPreview(file, selectedSheet ?? undefined, 'strict');
  };

  const canCommit =
    Boolean(file) &&
    Boolean(preview) &&
    (preview?.valid ?? 0) > 0 &&
    !isPreviewing &&
    !isCommitting;

  const previewMapping = preview?.mapping ?? null;
  const mappingRows = previewMapping
    ? ([
        ['date', previewMapping.fields.date],
        ['amount', previewMapping.fields.amount],
        ['description', previewMapping.fields.description],
        ['account', previewMapping.fields.account],
        ['category', previewMapping.fields.category],
      ] as const)
    : [];

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
              Imported {success.imported} of {success.total}{' '}
              {success.total === 1 ? 'transaction' : 'transactions'}
              {success.duplicate_count > 0 && (
                <>
                  , skipped {success.duplicate_count}{' '}
                  {success.duplicate_count === 1 ? 'duplicate' : 'duplicates'}
                </>
              )}
              .
            </p>
            {success.invalid > 0 && (
              <p className="mt-1 text-sm text-expense">
                {success.invalid}{' '}
                {success.invalid === 1 ? 'row could' : 'rows could'} not be
                imported.
              </p>
            )}
            {success.skipped > 0 && (
              <p className="mt-1 text-sm text-stone-600">
                {success.skipped}{' '}
                {success.skipped === 1 ? 'row was' : 'rows were'} skipped as
                non-transaction rows.
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
              AI maps the columns; you can confirm the mapping before anything
              is imported.
            </p>
            <p className={hintClass}>
              A sample of the selected sheet and your account/category names
              are sent to the AI provider to map columns. Nothing is imported
              until you confirm.
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
              {mode === 'ai' ? 'Analyzing with AI…' : 'Checking rows…'}
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

          {error && file && (
            <div className="flex flex-col gap-2 sm:flex-row">
              <Button
                variant="secondary"
                onClick={() =>
                  void runPreview(file, selectedSheet ?? undefined, mode)
                }
                disabled={isPreviewing || isCommitting}
              >
                Try again
              </Button>
              {mode === 'ai' && (
                <Button
                  variant="secondary"
                  onClick={() => void handleStandardImport()}
                  disabled={isPreviewing || isCommitting}
                >
                  Standard import
                </Button>
              )}
            </div>
          )}

          {mode === 'ai' && previewMapping && (
            <section className="space-y-2 rounded-xl border border-brand-200 bg-brand-50/40 p-3">
              <button
                type="button"
                onClick={() => setMappingOpen((open) => !open)}
                aria-expanded={mappingOpen}
                className="flex w-full items-center justify-between gap-2 text-left"
              >
                <span className="text-sm font-semibold text-stone-800">
                  AI mapping
                </span>
                <ChevronDown
                  className={cn(
                    'h-4 w-4 text-stone-500 transition-transform',
                    mappingOpen && 'rotate-180'
                  )}
                  aria-hidden="true"
                />
              </button>
              {mappingOpen && (
                <div className="space-y-2">
                  <p className={hintClass}>
                    Header row {previewMapping.header_row || '—'}; data rows{' '}
                    {previewMapping.data_start}–{previewMapping.data_end}. AI
                    read a sample of this sheet to map the columns. Nothing is
                    imported until you confirm.
                  </p>
                  <ul className="space-y-1 text-sm text-stone-700">
                    {mappingRows.map(([field, rule]) => (
                      <li key={field}>
                        <span className="font-semibold">{field}</span>
                        {' → '}
                        {rule.column ? `column ${rule.column}` : rule.constant}
                        {ruleDetails(rule) && (
                          <span className="text-stone-500">
                            {' '}
                            ({ruleDetails(rule)})
                          </span>
                        )}
                      </li>
                    ))}
                    <li>
                      <span className="font-semibold">type</span>
                      {' → '}
                      {previewMapping.type.column
                        ? `column ${previewMapping.type.column}`
                        : previewMapping.type.constant}
                      {previewMapping.type.value_map &&
                        Object.keys(previewMapping.type.value_map).length > 0 && (
                          <span className="text-stone-500">
                            {' '}
                            (value_map{' '}
                            {Object.entries(previewMapping.type.value_map)
                              .map(([from, to]) => `${from} → ${to}`)
                              .join(', ')}
                            )
                          </span>
                        )}
                    </li>
                  </ul>
                  {typeof previewMapping.confidence === 'number' && (
                    <p className={hintClass}>
                      Confidence{' '}
                      {Math.round(previewMapping.confidence * 100)}%.
                    </p>
                  )}
                  {previewMapping.notes && (
                    <p className={hintClass}>{previewMapping.notes}</p>
                  )}
                </div>
              )}
            </section>
          )}

          {preview && (
            <>
              <dl
                aria-label="Import summary"
                className="grid grid-cols-2 gap-2 sm:grid-cols-4"
              >
                {[
                  { label: 'Total', value: preview.total, tone: 'text-stone-900' },
                  { label: 'Valid', value: preview.valid, tone: 'text-income' },
                  {
                    label: 'Invalid',
                    value: preview.invalid,
                    tone: preview.invalid > 0 ? 'text-expense' : 'text-stone-900',
                  },
                  {
                    label: 'Skipped',
                    value: preview.skipped,
                    tone: 'text-stone-600',
                  },
                  ...(preview.duplicate_count > 0
                    ? [
                        {
                          label: 'Duplicates',
                          value: preview.duplicate_count,
                          tone: 'text-warm-700',
                        },
                      ]
                    : []),
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

              {preview.duplicate_count > 0 && (
                <div className="space-y-2 rounded-xl border border-warm-200 bg-warm-50/60 p-3">
                  <p className="flex items-start gap-2 text-sm font-medium text-warm-900">
                    <Copy
                      className="mt-0.5 h-4 w-4 shrink-0 text-warm-600"
                      aria-hidden="true"
                    />
                    <span>
                      {preview.duplicate_count}{' '}
                      {preview.duplicate_count === 1
                        ? 'duplicate'
                        : 'duplicates'}{' '}
                      {allowDuplicates ? 'will be imported.' : 'will be skipped.'}
                    </span>
                  </p>
                  <div className="space-y-1">
                    <div className="flex items-start gap-3">
                      <input
                        id="import-allow-duplicates"
                        type="checkbox"
                        checked={allowDuplicates}
                        onChange={(event) =>
                          setAllowDuplicates(event.target.checked)
                        }
                        disabled={isCommitting}
                        aria-describedby="import-allow-duplicates-hint"
                        className="mt-0.5 h-4 w-4 rounded border-warm-300 accent-brand-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2"
                      />
                      <label
                        htmlFor="import-allow-duplicates"
                        className="cursor-pointer text-sm font-semibold text-stone-800"
                      >
                        Import duplicates anyway
                      </label>
                    </div>
                    <p
                      id="import-allow-duplicates-hint"
                      className={cn('pl-7', hintClass)}
                    >
                      In-file duplicates are always skipped. Check this to also
                      import rows that already exist in your ledger.
                    </p>
                  </div>
                </div>
              )}

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

              {preview.duplicates.length > 0 && (
                <section className="space-y-1.5">
                  <h3 className="flex items-center gap-1.5 text-sm font-semibold text-stone-700">
                    <Copy
                      className="h-4 w-4 text-warm-600"
                      aria-hidden="true"
                    />
                    Duplicate rows
                  </h3>
                  <ul className="max-h-40 space-y-1 overflow-y-auto rounded-xl border border-warm-300 bg-warm-50 p-3">
                    {preview.duplicates.map((duplicate) => (
                      <li
                        key={`${duplicate.row}-${duplicate.message}`}
                        className="text-sm text-stone-700"
                      >
                        <span className="font-semibold text-warm-800">
                          Row {duplicate.row}
                        </span>
                        <span className="text-stone-500">
                          {' '}
                          — {duplicate.message}
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
                            <tr
                              key={row.row}
                              className={cn(row.duplicate && 'bg-warm-50/70')}
                            >
                              <td className="whitespace-nowrap px-3 py-2 text-stone-500">
                                {formatDate(row.date)}
                              </td>
                              <td className="whitespace-nowrap px-3 py-2">
                                <TypeBadge type={row.type} />
                              </td>
                              <td className="whitespace-nowrap px-3 py-2 text-right font-semibold text-stone-800">
                                {formatCurrency(row.amount, currency)}
                              </td>
                              <td className="max-w-[12rem] px-3 py-2 text-stone-700">
                                <div className="flex items-center gap-2">
                                  <span className="truncate">
                                    {row.description || '—'}
                                  </span>
                                  {row.duplicate && (
                                    <span className="inline-flex shrink-0 items-center rounded-full bg-warm-200 px-2 py-0.5 text-xs font-semibold text-warm-800">
                                      Duplicate
                                    </span>
                                  )}
                                </div>
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
