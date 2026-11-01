import { describe, it, expect, beforeEach, vi } from 'vitest';
import { api } from '../api';
import { useAuthStore } from '../../stores/authStore';
import { jsonResponse, mockFetchOnce, testUser } from '../../test/helpers';

function storedToken(token: string) {
  localStorage.setItem(
    'auth-storage',
    JSON.stringify({ state: { token }, version: 0 })
  );
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

  it('previews an import as multipart without a JSON content type', async () => {
    storedToken('abc123');
    const result = {
      total: 1,
      valid: 1,
      invalid: 0,
      errors: [],
      preview: [],
    };
    const fetchMock = mockFetchOnce(result);
    const file = new File(['date,type,amount\n'], 'rows.csv', {
      type: 'text/csv',
    });

    await expect(api.previewImport(file)).resolves.toEqual(result);

    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/transactions/import/preview');
    const body = (options as RequestInit).body as FormData;
    expect(body).toBeInstanceOf(FormData);
    expect(body.get('file')).toBe(file);
    expect(body.get('confirm')).toBeNull();
    expect(body.get('sheet')).toBeNull();
    const headers = (options as RequestInit).headers as Record<string, string>;
    expect(headers.Authorization).toBe('Bearer abc123');
    expect(headers['Content-Type']).toBeUndefined();
  });

  it('appends the selected sheet when previewing a workbook', async () => {
    storedToken('abc123');
    const result = {
      total: 1,
      valid: 1,
      invalid: 0,
      sheet: 'February',
      sheets: ['January', 'February'],
      errors: [],
      preview: [],
    };
    const fetchMock = mockFetchOnce(result);
    const file = new File(['sheet-bytes'], 'rows.xlsx');

    await expect(api.previewImport(file, 'February')).resolves.toEqual(result);

    const body = (fetchMock.mock.calls[0][1] as RequestInit).body as FormData;
    expect(body.get('file')).toBe(file);
    expect(body.get('sheet')).toBe('February');
    expect(body.get('confirm')).toBeNull();
  });

  it('commits an import with confirm=true', async () => {
    storedToken('abc123');
    const result = {
      total: 2,
      valid: 2,
      invalid: 0,
      errors: [],
      preview: [],
    };
    const fetchMock = mockFetchOnce(result);
    const file = new File(['date,type,amount\n'], 'rows.csv', {
      type: 'text/csv',
    });

    await expect(api.importTransactions(file)).resolves.toEqual(result);

    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/transactions/import');
    const body = (options as RequestInit).body as FormData;
    expect(body.get('file')).toBe(file);
    expect(body.get('confirm')).toBe('true');
    expect(body.get('sheet')).toBeNull();
    const headers = (options as RequestInit).headers as Record<string, string>;
    expect(headers.Authorization).toBe('Bearer abc123');
    expect(headers['Content-Type']).toBeUndefined();
  });

  it('appends the selected sheet when committing a workbook', async () => {
    storedToken('abc123');
    const result = {
      total: 1,
      valid: 1,
      invalid: 0,
      sheet: 'March',
      sheets: ['January', 'March'],
      errors: [],
      preview: [],
    };
    const fetchMock = mockFetchOnce(result);
    const file = new File(['sheet-bytes'], 'rows.xlsx');

    await expect(api.importTransactions(file, 'March')).resolves.toEqual(result);

    const body = (fetchMock.mock.calls[0][1] as RequestInit).body as FormData;
    expect(body.get('file')).toBe(file);
    expect(body.get('confirm')).toBe('true');
    expect(body.get('sheet')).toBe('March');
  });
});
