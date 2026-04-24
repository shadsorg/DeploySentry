import { describe, it, expect, beforeEach, vi } from 'vitest';
import { authApi, orgsApi, setFetch } from './api';

describe('api', () => {
  let fetchMock: ReturnType<typeof vi.fn<typeof fetch>>;

  beforeEach(() => {
    fetchMock = vi.fn<typeof fetch>();
    setFetch(fetchMock);
    localStorage.clear();
  });

  it('authApi.login POSTs without Authorization header', async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ token: 't', user: { id: '1', email: 'a@b.c', name: 'A' } }), { status: 200 }),
    );
    const res = await authApi.login({ email: 'a@b.c', password: 'pw' });
    expect(res.token).toBe('t');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/auth/login',
      expect.objectContaining({ method: 'POST' }),
    );
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>).Authorization).toBeUndefined();
  });

  it('orgsApi.list includes Bearer token when JWT stored', async () => {
    localStorage.setItem('ds_token', 'header.payload.sig');
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ organizations: [] }), { status: 200 }),
    );
    await orgsApi.list();
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer header.payload.sig');
  });

  it('orgsApi.list uses ApiKey scheme when token starts with ds_', async () => {
    localStorage.setItem('ds_token', 'ds_abc123');
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ organizations: [] }), { status: 200 }),
    );
    await orgsApi.list();
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>).Authorization).toBe('ApiKey ds_abc123');
  });

  it('401 clears token and redirects', async () => {
    localStorage.setItem('ds_token', 'expired');
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ error: 'nope' }), { status: 401 }));
    const assignMock = vi.fn();
    Object.defineProperty(window, 'location', {
      value: { pathname: '/m/orgs', search: '', assign: assignMock },
      writable: true,
    });
    await expect(orgsApi.list()).rejects.toThrow();
    expect(localStorage.getItem('ds_token')).toBeNull();
    expect(assignMock).toHaveBeenCalledWith('/m/login?next=%2Fm%2Forgs');
  });
});
