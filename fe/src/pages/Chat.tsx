import { useEffect, useRef, useState } from 'react';
import type { ChangeEvent } from 'react';
import {
  Bot,
  FileSpreadsheet,
  Loader2,
  Paperclip,
  Send,
  Sparkles,
  Trash2,
  TriangleAlert,
  User,
  X,
} from 'lucide-react';
import { useChatStore } from '../stores/chatStore';
import { PageHeader } from '../components/ui/PageHeader';
import { MarkdownMessage } from '../components/chat/MarkdownMessage';
import { Button } from '../components/ui/Button';
import { ConfirmDialog } from '../components/ui/ConfirmDialog';
import { Select } from '../components/ui/Select';
import { inputClass } from '../components/ui/formStyles';
import { cn } from '../lib/cn';

const suggestions = [
  'What are my account balances?',
  'How much did I spend this month?',
  'Log 45.00 for groceries from Cash today',
];

function formatTokens(value: number): string {
  return value.toLocaleString('en-US');
}

function Chat() {
  const entries = useChatStore((state) => state.entries);
  const tokenUsage = useChatStore((state) => state.tokenUsage);
  const isLoading = useChatStore((state) => state.isLoading);
  const isLoadingHistory = useChatStore((state) => state.isLoadingHistory);
  const isConfirming = useChatStore((state) => state.isConfirming);
  const isClearing = useChatStore((state) => state.isClearing);
  const error = useChatStore((state) => state.error);
  const attachedFile = useChatStore((state) => state.attachedFile);
  const selectedSheet = useChatStore((state) => state.selectedSheet);
  const isUploading = useChatStore((state) => state.isUploading);
  const uploadError = useChatStore((state) => state.uploadError);
  const loadHistory = useChatStore((state) => state.loadHistory);
  const sendMessage = useChatStore((state) => state.sendMessage);
  const attachFile = useChatStore((state) => state.attachFile);
  const selectSheet = useChatStore((state) => state.selectSheet);
  const removeAttachment = useChatStore((state) => state.removeAttachment);
  const confirmAction = useChatStore((state) => state.confirmAction);
  const cancelAction = useChatStore((state) => state.cancelAction);
  const clear = useChatStore((state) => state.clear);
  const [draft, setDraft] = useState('');
  const [clearOpen, setClearOpen] = useState(false);
  const activeRows = attachedFile
    ? attachedFile.sheet_rows?.[selectedSheet ?? attachedFile.sheet] ??
      attachedFile.rows
    : 0;
  const scrollRef = useRef<HTMLDivElement>(null);
  const limitReached = tokenUsage?.limit_reached ?? false;
  const inputsDisabled =
    isLoading || isUploading || isLoadingHistory || limitReached;
  const tokenPercent =
    tokenUsage && tokenUsage.limit > 0
      ? Math.min(100, Math.round((tokenUsage.used / tokenUsage.limit) * 100))
      : 0;

  useEffect(() => {
    void loadHistory();
  }, [loadHistory]);

  useEffect(() => {
    const element = scrollRef.current;
    if (!element) return;
    element.scrollTop = element.scrollHeight;
  }, [entries, isLoading]);

  const submit = async () => {
    const text = draft.trim();
    if (!text || inputsDisabled) return;
    setDraft('');
    await sendMessage(text);
  };

  const handleFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (file) void attachFile(file);
    event.target.value = '';
  };

  return (
    <div className="flex h-[calc(100vh-9rem)] flex-col gap-4">
      <PageHeader
        title="Assistant"
        description="Ask about your money, or tell me what to record."
      />

      <div className="flex flex-wrap items-center justify-between gap-3">
        {tokenUsage && tokenUsage.limit > 0 ? (
          <div className="flex items-center gap-3">
            <span
              className="rounded-full bg-brand-50 px-3 py-1 text-xs font-semibold text-brand-700"
              title="Approximate conversation token usage — not billing."
            >
              ≈ {formatTokens(tokenUsage.used)} / {formatTokens(tokenUsage.limit)}
            </span>
            <div
              role="progressbar"
              aria-label="Conversation token usage"
              aria-valuemin={0}
              aria-valuemax={tokenUsage.limit}
              aria-valuenow={Math.min(tokenUsage.used, tokenUsage.limit)}
              className="h-1.5 w-28 overflow-hidden rounded-full bg-warm-200"
            >
              <div
                className={cn(
                  'h-full rounded-full transition-[width]',
                  limitReached ? 'bg-expense' : 'bg-brand-500'
                )}
                style={{ width: `${tokenPercent}%` }}
              />
            </div>
          </div>
        ) : (
          <span />
        )}
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setClearOpen(true)}
          disabled={entries.length === 0 || isLoading || isClearing}
        >
          <Trash2 className="h-4 w-4" aria-hidden="true" />
          Clear chat
        </Button>
      </div>

      {error && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-xl border border-expense/20 bg-expense-light/40 px-4 py-3 text-sm font-medium text-expense"
        >
          <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <span>{error}</span>
        </div>
      )}

      {limitReached && (
        <div
          role="alert"
          aria-live="assertive"
          className="flex items-start gap-2 rounded-xl border border-expense/20 bg-expense-light/40 px-4 py-3 text-sm font-medium text-expense"
        >
          <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <span>Token limit reached. Clear the chat to continue.</span>
        </div>
      )}

      <div
        ref={scrollRef}
        className="flex-1 space-y-4 overflow-y-auto rounded-2xl border border-warm-200 bg-surface p-4"
        aria-live="polite"
      >
        {isLoadingHistory ? (
          <div
            role="status"
            className="flex h-full items-center justify-center gap-2 text-sm text-stone-500"
          >
            <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
            Loading conversation…
          </div>
        ) : entries.length === 0 ? (
          <div className="flex h-full flex-col items-center justify-center gap-4 text-center">
            <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-brand-100 text-brand-600">
              <Sparkles className="h-7 w-7" aria-hidden="true" />
            </div>
            <div>
              <h2 className="text-base font-bold text-stone-900">
                Ask me anything about your finances
              </h2>
              <p className="mt-1 max-w-sm text-sm text-stone-500">
                I can look up balances, summarise spending, record transactions, and
                delete them once you confirm.
              </p>
            </div>
            <div className="flex flex-wrap justify-center gap-2">
              {suggestions.map((suggestion) => (
                <button
                  key={suggestion}
                  type="button"
                  onClick={() => void sendMessage(suggestion)}
                  disabled={inputsDisabled}
                  className="rounded-full border border-warm-200 bg-surface-card px-3 py-1.5 text-xs font-medium text-stone-600 transition-colors hover:border-brand-300 hover:bg-brand-50 hover:text-brand-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {suggestion}
                </button>
              ))}
            </div>
          </div>
        ) : (
          entries.map((entry) => (
            <div
              key={entry.id}
              className={
                entry.role === 'user'
                  ? 'flex justify-end gap-2'
                  : 'flex justify-start gap-2'
              }
            >
              {entry.role === 'assistant' && (
                <div className="mt-1 flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-brand-100 text-brand-600">
                  <Bot className="h-4 w-4" aria-hidden="true" />
                </div>
              )}

              <div
                className={
                  entry.role === 'user'
                    ? 'max-w-[80%] rounded-2xl rounded-br-sm bg-brand-600 px-4 py-2.5 text-sm text-white'
                    : 'min-w-0 max-w-[80%] space-y-2'
                }
              >
                {entry.role === 'user' ? (
                  <div className="whitespace-pre-wrap">{entry.content}</div>
                ) : (
                  <div className="min-w-0 rounded-2xl rounded-bl-sm border border-warm-200 bg-surface-card px-4 py-2.5 text-sm text-stone-700">
                    <MarkdownMessage content={entry.content} />
                  </div>
                )}

                {entry.toolCalls.length > 0 && (
                  <div className="flex flex-wrap gap-1.5">
                    {entry.toolCalls.map((call, index) => (
                      <span
                        key={`${entry.id}-tool-${index}`}
                        title={call.summary}
                        className="rounded-full bg-brand-50 px-2.5 py-1 text-xs font-medium text-brand-700"
                      >
                        {call.name.replaceAll('_', ' ')}
                      </span>
                    ))}
                  </div>
                )}

                {entry.pendingAction && (
                  <div className="rounded-2xl border border-brand-200 bg-brand-50/70 px-4 py-3">
                    <p className="text-sm font-semibold text-stone-800">
                      {entry.pendingAction.summary}
                    </p>
                    <div className="mt-3 flex gap-2">
                      <Button
                        onClick={() => void confirmAction(entry.pendingAction!)}
                        disabled={isConfirming}
                      >
                        {isConfirming ? 'Working…' : 'Confirm'}
                      </Button>
                      <Button
                        variant="secondary"
                        onClick={() => void cancelAction(entry.pendingAction!)}
                        disabled={isConfirming}
                      >
                        Cancel
                      </Button>
                    </div>
                  </div>
                )}
              </div>

              {entry.role === 'user' && (
                <div className="mt-1 flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-warm-200 text-stone-600">
                  <User className="h-4 w-4" aria-hidden="true" />
                </div>
              )}
            </div>
          ))
        )}

        {isLoading && (
          <div className="flex items-center gap-2 text-sm text-stone-500">
            <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
            Thinking…
          </div>
        )}
      </div>

      <div className="space-y-2">
        {attachedFile && (
          <div className="flex items-center gap-2.5 rounded-xl border border-brand-200 bg-brand-50/70 px-3 py-2">
            <FileSpreadsheet
              className="h-4 w-4 shrink-0 text-brand-600"
              aria-hidden="true"
            />
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-semibold text-stone-800">
                {attachedFile.name}
              </p>
              {attachedFile.sheets.length > 1 ? (
                <div className="mt-1 flex flex-wrap items-center gap-2">
                  <label
                    htmlFor="chat-sheet"
                    className="text-xs font-medium text-stone-500"
                  >
                    Sheet
                  </label>
                  <Select
                    id="chat-sheet"
                    value={selectedSheet ?? attachedFile.sheet}
                    onChange={(event) => selectSheet(event.target.value)}
                    disabled={inputsDisabled}
                    className="max-w-[11rem] text-xs"
                  >
                    {attachedFile.sheets.map((name) => (
                      <option key={name} value={name}>
                        {name}
                      </option>
                    ))}
                  </Select>
                  <span className="text-xs text-stone-500">
                    {activeRows} {activeRows === 1 ? 'row' : 'rows'}
                  </span>
                </div>
              ) : (
                <p className="text-xs text-stone-500">
                  Sheet {attachedFile.sheet} · {attachedFile.rows}{' '}
                  {attachedFile.rows === 1 ? 'row' : 'rows'}
                </p>
              )}
            </div>
            <button
              type="button"
              onClick={removeAttachment}
              aria-label={`Remove ${attachedFile.name}`}
              className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg text-stone-500 transition-colors hover:bg-brand-100 hover:text-brand-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
            >
              <X className="h-4 w-4" aria-hidden="true" />
            </button>
          </div>
        )}

        {isUploading && (
          <div
            role="status"
            className="flex items-center gap-2 rounded-xl border border-warm-200 bg-warm-50/60 px-4 py-2.5 text-sm font-medium text-stone-600"
          >
            <Loader2
              className="h-4 w-4 animate-spin text-brand-500"
              aria-hidden="true"
            />
            Uploading spreadsheet…
          </div>
        )}

        {uploadError && (
          <p
            role="alert"
            className="rounded-xl border border-expense/20 bg-expense-light/60 px-4 py-2.5 text-sm font-medium text-expense"
          >
            {uploadError}
          </p>
        )}

        <form
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
          className="flex items-end gap-2"
        >
          <label
            className={cn(
              'inline-flex h-11 w-11 shrink-0 cursor-pointer items-center justify-center rounded-xl border border-warm-200 bg-surface-card text-stone-600 transition-colors',
              'hover:border-brand-300 hover:bg-brand-50 hover:text-brand-800',
              'focus-within:ring-2 focus-within:ring-brand-500 focus-within:ring-offset-2 focus-within:ring-offset-surface',
              inputsDisabled && 'cursor-not-allowed opacity-50'
            )}
          >
            <Paperclip className="h-4 w-4" aria-hidden="true" />
            <span className="sr-only">Attach spreadsheet</span>
            <input
              type="file"
              accept=".csv,.xlsx,.xls"
              onChange={handleFileChange}
              disabled={inputsDisabled}
              className="sr-only"
            />
          </label>
          <label htmlFor="chat-input" className="sr-only">
            Message
          </label>
          <textarea
            id="chat-input"
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && !event.shiftKey) {
                event.preventDefault();
                void submit();
              }
            }}
            rows={2}
            placeholder="Ask about your spending, or log a transaction…"
            className={`${inputClass} min-h-[3rem] resize-none`}
            disabled={inputsDisabled}
          />
          <Button
            type="submit"
            disabled={inputsDisabled || draft.trim() === ''}
          >
            <Send className="h-4 w-4" aria-hidden="true" />
            Send
          </Button>
        </form>
      </div>

      <ConfirmDialog
        open={clearOpen}
        title="Clear conversation?"
        description="This permanently deletes the conversation from the server. Transactions already recorded are not affected."
        confirmLabel="Clear chat"
        isLoading={isClearing}
        onConfirm={() => {
          void clear().then(() => setClearOpen(false));
        }}
        onCancel={() => setClearOpen(false)}
      />
    </div>
  );
}

export default Chat;
