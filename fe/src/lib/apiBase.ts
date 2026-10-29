const rawBase = import.meta.env.VITE_API_BASE_URL ?? '';

export const API_BASE_URL = rawBase.trim().replace(/\/+$/, '');

export function apiUrl(path: string): string {
  return `${API_BASE_URL}/api${path}`;
}
