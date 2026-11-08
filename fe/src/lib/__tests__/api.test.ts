import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { api } from '../api';
import { useAuthStore } from '../../stores/authStore';
import {
  jsonResponse,
  mockFetchOnce,
  mockFetchSequence,
  testUser,
} from '../../test/helpers';
import type { ImportMapping } from '../../types';

function storedToken(token: string) {
  useAuthStore.setState({ token });
}

function hangingFetch() {
  const fetchMock = vi.fn(
    (_input: RequestInfo | URL, init?: RequestInit) =>
      new Promise<Response>((_resolve, reject) => {
        init?.signal?.addEventListener('abort', () => {
          reject(
            init.signal?.reason ??
              new DOMException('The operation was aborted', 'AbortError')
          );
        });
      })
  );
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

describe('api client', () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
    useAuthStore.setState({
      user: null,
      token: null,
      isAuthenticated: false,
      isLoading: false,
      error: null,
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('attaches the persisted JWT as a Bearer token', async () => {
    storedToken('abc123');
    const fetchMock = mockFetchOnce([]);

    await api.listAccounts();

    const options = fetchMock.mock.calls[0][1] as RequestInit;
    const headers = options.headers as Record<string, string>;
    expect(headers.Authorization).toBe('Bearer abc123');
  });

  it('omits the Authorization header when no token is stored', async () => {
    const fetchMock = mockFetchOnce([]);

    await api.listAccounts();

    const options = fetchMock.mock.calls[0][1] as RequestInit;
    const headers = options.headers as Record<string, string>;
    expect(headers.Authorization).toBeUndefined();
  });

  it('logs out and rejects when the API returns 401', async () => {
    storedToken('expired');
    useAuthStore.setState({
      user: testUser,
      token: 'expired',
      isAuthenticated: true,
    });
    mockFetchOnce({ error: 'invalid or expired token' }, 401);

    await expect(api.listAccounts()).rejects.toThrow('invalid or expired token');

    expect(useAuthStore.getState().isAuthenticated).toBe(false);
    expect(useAuthStore.getState().token).toBeNull();
  });

  it('aborts a request that exceeds the default timeout', async () => {
    vi.useFakeTimers();
    hangingFetch();

    const pending = api.listAccounts();
    const rejection = expect(pending).rejects.toThrow(/timed out/i);

    await vi.advanceTimersByTimeAsync(30_000);

    await rejection;
  });

  it('honours a caller-provided abort signal', async () => {
    hangingFetch();
    const controller = new AbortController();

    const pending = api.listAccounts(controller.signal);
    const rejection = expect(pending).rejects.toThrow('Cancelled by caller');

    controller.abort(new DOMException('Cancelled by caller', 'AbortError'));

    await rejection;
  });

  it('does not abort a completed request early', async () => {
    vi.useFakeTimers();
    const fetchMock = mockFetchOnce([]);

    await expect(api.listAccounts()).resolves.toEqual([]);
    await vi.advanceTimersByTimeAsync(30_000);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect((init.signal as AbortSignal).aborted).toBe(false);
  });

  it('returns undefined for a 204 response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(null, 204)));

    await expect(api.deleteAccount('account-1')).resolves.toBeUndefined();
  });

  it('surfaces the backend error message', async () => {
    mockFetchOnce({ error: 'account not found' }, 404);

    await expect(api.updateAccount('missing', { name: 'x' })).rejects.toThrow(
      'account not found'
    );
  });

  it('exports the filtered CSV as an authenticated blob', async () => {
    storedToken('abc123');
    const blob = new Blob(['date,type,amount\n'], { type: 'text/csv' });
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      statusText: '',
      blob: async () => blob,
    } as unknown as Response);
    vi.stubGlobal('fetch', fetchMock);

    const result = await api.exportTransactionsCsv({
      account_id: 'account-1',
      category_id: 'category-1',
      type: 'expense',
      date_from: '2026-01-01',
      date_to: '2026-01-31',
      page: 3,
    });

    expect(result).toBe(blob);
    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe(
      '/api/transactions/export.csv?account_id=account-1&category_id=category-1&type=expense&date_from=2026-01-01&date_to=2026-01-31'
    );
    const headers = (options as RequestInit).headers as Record<string, string>;
    expect(headers.Authorization).toBe('Bearer abc123');
    expect(headers['Content-Type']).toBeUndefined();
  });

  it('rejects export with the backend error message', async () => {
    storedToken('abc123');
    mockFetchOnce({ error: 'invalid filter' }, 400);

    await expect(api.exportTransactionsCsv()).rejects.toThrow('invalid filter');
  });

  it('sends the currency when saving and deleting a budget', async () => {
    storedToken('abc123');
    const fetchMock = mockFetchSequence(
      { body: { id: 'budget-1' }, status: 200 },
      { body: undefined, status: 204 }
    );

    await api.upsertBudget({
      category_id: 'category-1',
      month: '2026-11',
      amount: 200,
      currency: 'IDR',
    });
    await api.deleteBudget('category-1', '2026-11', 'IDR');

    const [putUrl, putOptions] = fetchMock.mock.calls[0];
    expect(putUrl).toBe('/api/budgets');
    expect((putOptions as RequestInit).method).toBe('PUT');
    expect(
      JSON.parse(String((putOptions as RequestInit).body))
    ).toMatchObject({ currency: 'IDR' });

    const [deleteUrl, deleteOptions] = fetchMock.mock.calls[1];
    expect(deleteUrl).toBe(
      '/api/budgets/category-1?month=2026-11&currency=IDR'
    );
    expect((deleteOptions as RequestInit).method).toBe('DELETE');
  });

  it('previews an import as multipart without a JSON content type', async () => {
    storedToken('abc123');
    const result = { total: 1, valid: 1, invalid: 0, skipped: 0, imported: 0, duplicate_count: 0, errors: [], duplicates: [], preview: [] };
    const fetchMock = mockFetchOnce(result);
    const file = new File(['date,type,amount\n'], 'rows.csv', { type: 'text/csv' });

    await expect(api.previewImport(file)).resolves.toEqual(result);

    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/transactions/import/preview');
    const body = (options as RequestInit).body as FormData;
    expect(body.get('file')).toBe(file);
    expect(body.get('confirm')).toBeNull();
    expect(body.get('sheet')).toBeNull();
    expect(body.get('mode')).toBeNull();
    expect(body.get('allow_duplicates')).toBeNull();
    const headers = (options as RequestInit).headers as Record<string, string>;
    expect(headers.Authorization).toBe('Bearer abc123');
    expect(headers['Content-Type']).toBeUndefined();
  });

  it('appends the selected sheet and mode when previewing a workbook', async () => {
    storedToken('abc123');
    const result = { total: 1, valid: 1, invalid: 0, skipped: 0, imported: 0, duplicate_count: 0, sheet: 'February', sheets: ['January', 'February'], errors: [], duplicates: [], preview: [] };
    const fetchMock = mockFetchOnce(result);
    const file = new File(['sheet-bytes'], 'rows.xlsx');

    await expect(api.previewImport(file, { sheet: 'February', mode: 'strict' })).resolves.toEqual(result);

    const body = (fetchMock.mock.calls[0][1] as RequestInit).body as FormData;
    expect(body.get('file')).toBe(file);
    expect(body.get('sheet')).toBe('February');
    expect(body.get('mode')).toBe('strict');
    expect(body.get('confirm')).toBeNull();
  });

  it('commits an import with confirm=true', async () => {
    storedToken('abc123');
    const result = { total: 2, valid: 2, invalid: 0, skipped: 0, imported: 2, duplicate_count: 0, errors: [], duplicates: [], preview: [] };
    const fetchMock = mockFetchOnce(result);
    const file = new File(['date,type,amount\n'], 'rows.csv', { type: 'text/csv' });

    await expect(api.importTransactions(file)).resolves.toEqual(result);

    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/transactions/import');
    const body = (options as RequestInit).body as FormData;
    expect(body.get('file')).toBe(file);
    expect(body.get('confirm')).toBe('true');
    expect(body.get('sheet')).toBeNull();
    expect(body.get('mode')).toBeNull();
    expect(body.get('mapping')).toBeNull();
    expect(body.get('allow_duplicates')).toBeNull();
  });

  it('sends allow_duplicates=true only when requested', async () => {
    storedToken('abc123');
    const result = { total: 2, valid: 2, invalid: 0, skipped: 0, imported: 1, duplicate_count: 1, errors: [], duplicates: [], preview: [] };
    const fetchMock = mockFetchOnce(result);
    const file = new File(['date,type,amount\n'], 'rows.csv', { type: 'text/csv' });

    await expect(
      api.importTransactions(file, {
        sheet: 'March',
        mode: 'ai',
        allow_duplicates: true,
      })
    ).resolves.toEqual(result);

    const body = (fetchMock.mock.calls[0][1] as RequestInit).body as FormData;
    expect(body.get('sheet')).toBe('March');
    expect(body.get('mode')).toBe('ai');
    expect(body.get('allow_duplicates')).toBe('true');
  });

  it('creates a transfer through POST /transfers', async () => {
    storedToken('abc123');
    const result = {
      transfer_group_id: 'group-1',
      from: { id: 'transaction-1' },
      to: { id: 'transaction-2' },
    };
    const fetchMock = mockFetchOnce(result, 201);
    const payload = {
      from_account_id: 'account-1',
      to_account_id: 'account-2',
      amount: 100,
      date: '2026-11-08',
      description: 'Move to savings',
    };

    await expect(api.createTransfer(payload)).resolves.toEqual(result);

    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/transfers');
    expect((options as RequestInit).method).toBe('POST');
    expect(JSON.parse(String((options as RequestInit).body))).toEqual(payload);
    const headers = (options as RequestInit).headers as Record<string, string>;
    expect(headers.Authorization).toBe('Bearer abc123');
  });

  it('deletes both sides of a transfer and tolerates 204', async () => {
    storedToken('abc123');
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(null, 204));
    vi.stubGlobal('fetch', fetchMock);

    await expect(api.deleteTransfer('group-1')).resolves.toBeUndefined();

    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/transfers/group-1');
    expect((options as RequestInit).method).toBe('DELETE');
  });

  it('appends the sheet, mode and mapping when committing a workbook', async () => {
    storedToken('abc123');
    const result = { total: 1, valid: 1, invalid: 0, skipped: 0, imported: 1, duplicate_count: 0, sheet: 'March', sheets: ['January', 'March'], errors: [], duplicates: [], preview: [] };
    const fetchMock = mockFetchOnce(result);
    const file = new File(['sheet-bytes'], 'rows.xlsx');
    const mapping: ImportMapping = {
      sheet: 'March',
      header_row: 1,
      data_start: 2,
      data_end: 2,
      fields: {
        date: { column: 1 },
        amount: { column: 3 },
        description: { column: 4 },
        account: { column: 5 },
        category: { column: 6 },
      },
      type: { column: 2 },
    };

    await expect(api.importTransactions(file, { sheet: 'March', mode: 'ai', mapping })).resolves.toEqual(result);

    const body = (fetchMock.mock.calls[0][1] as RequestInit).body as FormData;
    expect(body.get('file')).toBe(file);
    expect(body.get('confirm')).toBe('true');
    expect(body.get('sheet')).toBe('March');
    expect(body.get('mode')).toBe('ai');
    expect(JSON.parse(String(body.get('mapping')))).toEqual(mapping);
  });

  it('lists a transaction’s attachments', async () => {
    storedToken('abc123');
    const metadata = {
      id: 'attachment-1',
      transaction_id: 'transaction-1',
      filename: 'receipt.png',
      content_type: 'image/png',
      size_bytes: 2048,
      created_at: '2026-11-08T10:00:00Z',
    };
    const fetchMock = mockFetchOnce([metadata]);

    await expect(api.listAttachments('transaction-1')).resolves.toEqual([
      metadata,
    ]);

    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/transactions/transaction-1/attachments');
    const headers = (options as RequestInit).headers as Record<string, string>;
    expect(headers.Authorization).toBe('Bearer abc123');
  });

  it('uploads an attachment as multipart with a file field', async () => {
    storedToken('abc123');
    const metadata = {
      id: 'attachment-1',
      transaction_id: 'transaction-1',
      filename: 'receipt.png',
      content_type: 'image/png',
      size_bytes: 2048,
      created_at: '2026-11-08T10:00:00Z',
    };
    const fetchMock = mockFetchOnce(metadata, 201);
    const file = new File(['png-bytes'], 'receipt.png', { type: 'image/png' });

    await expect(api.uploadAttachment('transaction-1', file)).resolves.toEqual(
      metadata
    );

    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/transactions/transaction-1/attachments');
    expect((options as RequestInit).method).toBe('POST');
    const body = (options as RequestInit).body as FormData;
    expect(body.get('file')).toBe(file);
    const headers = (options as RequestInit).headers as Record<string, string>;
    expect(headers.Authorization).toBe('Bearer abc123');
    expect(headers['Content-Type']).toBeUndefined();
  });

  it('deletes an attachment and tolerates 204', async () => {
    storedToken('abc123');
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(null, 204));
    vi.stubGlobal('fetch', fetchMock);

    await expect(api.deleteAttachment('attachment-1')).resolves.toBeUndefined();

    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/attachments/attachment-1');
    expect((options as RequestInit).method).toBe('DELETE');
  });

  it('downloads an attachment as an authenticated blob', async () => {
    storedToken('abc123');
    const blob = new Blob(['png-bytes'], { type: 'image/png' });
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      statusText: '',
      blob: async () => blob,
    } as unknown as Response);
    vi.stubGlobal('fetch', fetchMock);

    const result = await api.downloadAttachment('attachment-1');

    expect(result).toBe(blob);
    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/attachments/attachment-1');
    const headers = (options as RequestInit).headers as Record<string, string>;
    expect(headers.Authorization).toBe('Bearer abc123');
    expect(headers['Content-Type']).toBeUndefined();
  });

  it('logs out when an attachment download returns 401', async () => {
    useAuthStore.setState({
      user: testUser,
      token: 'expired',
      isAuthenticated: true,
    });
    mockFetchOnce({ error: 'invalid or expired token' }, 401);

    await expect(api.downloadAttachment('attachment-1')).rejects.toThrow(
      'invalid or expired token'
    );

    expect(useAuthStore.getState().isAuthenticated).toBe(false);
    expect(useAuthStore.getState().token).toBeNull();
  });
});
