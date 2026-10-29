import { describe, it, expect, vi } from 'vitest';

describe('apiBase', () => {
  it('uses a relative /api path when the env var is unset/empty', async () => {
    vi.stubEnv('VITE_API_BASE_URL', '');
    vi.resetModules();

    const { apiUrl } = await import('../apiBase');
    expect(apiUrl('/accounts')).toBe('/api/accounts');
  });

  it('builds an absolute URL from a configured base', async () => {
    vi.stubEnv('VITE_API_BASE_URL', 'http://api.test');
    vi.resetModules();

    const { apiUrl } = await import('../apiBase');
    expect(apiUrl('/accounts')).toBe('http://api.test/api/accounts');
  });

  it('strips trailing slashes from the configured base', async () => {
    vi.stubEnv('VITE_API_BASE_URL', 'http://localhost:8080/');
    vi.resetModules();

    const { apiUrl } = await import('../apiBase');
    expect(apiUrl('/accounts')).toBe('http://localhost:8080/api/accounts');
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.resetModules();
  });
});
