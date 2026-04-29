import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  authApi,
  orgsApi,
  orgStatusApi,
  orgDeploymentsApi,
  projectsApi,
  flagsApi,
  flagEnvStateApi,
  envApi,
  appsApi,
  auditApi,
  setFetch,
} from './api';

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

  it('orgStatusApi.get fetches /orgs/:slug/status with Bearer token', async () => {
    localStorage.setItem('ds_token', 'header.payload.sig');
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          org: { id: '1', slug: 'acme', name: 'Acme' },
          generated_at: '2026-04-24T00:00:00Z',
          projects: [],
        }),
        { status: 200 },
      ),
    );
    const res = await orgStatusApi.get('acme');
    expect(res.org.slug).toBe('acme');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/orgs/acme/status',
      expect.objectContaining({}),
    );
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer header.payload.sig');
  });

  it('orgDeploymentsApi.list builds query string from filters and uses Bearer token', async () => {
    localStorage.setItem('ds_token', 'header.payload.sig');
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ deployments: [], next_cursor: 'abc' }), { status: 200 }),
    );
    const res = await orgDeploymentsApi.list('acme', { status: 'completed', limit: 25 });
    expect(res.next_cursor).toBe('abc');
    const url = fetchMock.mock.calls[0][0] as string;
    expect(url).toContain('/api/v1/orgs/acme/deployments');
    expect(url).toContain('status=completed');
    expect(url).toContain('limit=25');
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer header.payload.sig');
  });

  it('orgDeploymentsApi.list omits undefined filter values from the URL', async () => {
    localStorage.setItem('ds_token', 'header.payload.sig');
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ deployments: [] }), { status: 200 }));
    await orgDeploymentsApi.list('acme', { status: 'failed' });
    const url = fetchMock.mock.calls[0][0] as string;
    expect(url).toBe('/api/v1/orgs/acme/deployments?status=failed');
  });

  it('projectsApi.list fetches /orgs/:slug/projects', async () => {
    localStorage.setItem('ds_token', 'header.payload.sig');
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ projects: [{ id: 'p1', slug: 'pay', name: 'Pay', org_id: 'o1' }] }), {
        status: 200,
      }),
    );
    const res = await projectsApi.list('acme');
    expect(res.projects[0].slug).toBe('pay');
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/orgs/acme/projects');
  });

  it('flagsApi.list builds /flags?project_id=<id> with Bearer token', async () => {
    localStorage.setItem('ds_token', 'header.payload.sig');
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ flags: [] }), { status: 200 }));
    await flagsApi.list('proj-1');
    const url = fetchMock.mock.calls[0][0] as string;
    expect(url).toBe('/api/v1/flags?project_id=proj-1');
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer header.payload.sig');
  });

  it('flagsApi.list includes category and archived query params when provided', async () => {
    localStorage.setItem('ds_token', 'header.payload.sig');
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ flags: [] }), { status: 200 }));
    await flagsApi.list('proj-1', { category: 'release', archived: false });
    const url = fetchMock.mock.calls[0][0] as string;
    expect(url).toContain('/api/v1/flags?');
    expect(url).toContain('project_id=proj-1');
    expect(url).toContain('category=release');
    expect(url).toContain('archived=false');
  });

  it('flagsApi.list includes application_id when applicationId is provided', async () => {
    localStorage.setItem('ds_token', 'header.payload.sig');
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ flags: [] }), { status: 200 }));
    await flagsApi.list('proj-1', { applicationId: 'app-9' });
    const url = fetchMock.mock.calls[0][0] as string;
    expect(url).toContain('project_id=proj-1');
    expect(url).toContain('application_id=app-9');
  });

  it('flagsApi.get fetches /flags/:id', async () => {
    localStorage.setItem('ds_token', 'header.payload.sig');
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ id: 'f1', key: 'k', name: 'n' }), { status: 200 }),
    );
    await flagsApi.get('f1');
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/flags/f1');
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer header.payload.sig');
  });

  it('flagsApi.listRules fetches /flags/:flagId/rules', async () => {
    localStorage.setItem('ds_token', 'header.payload.sig');
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ rules: [] }), { status: 200 }));
    await flagsApi.listRules('f1');
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/flags/f1/rules');
  });

  it('flagsApi.listRuleEnvStates fetches /flags/:flagId/rules/environment-states', async () => {
    localStorage.setItem('ds_token', 'header.payload.sig');
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ rule_environment_states: [] }), { status: 200 }),
    );
    await flagsApi.listRuleEnvStates('f1');
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/flags/f1/rules/environment-states');
  });

  it('flagEnvStateApi.list fetches /flags/:flagId/environments', async () => {
    localStorage.setItem('ds_token', 'header.payload.sig');
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ environment_states: [] }), { status: 200 }),
    );
    await flagEnvStateApi.list('f1');
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/flags/f1/environments');
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer header.payload.sig');
  });

  it('envApi.listOrg fetches /orgs/:slug/environments', async () => {
    localStorage.setItem('ds_token', 'header.payload.sig');
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ environments: [] }), { status: 200 }));
    await envApi.listOrg('acme');
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/orgs/acme/environments');
  });

  it('appsApi.list fetches /orgs/:slug/projects/:proj/apps', async () => {
    localStorage.setItem('ds_token', 'header.payload.sig');
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ applications: [] }), { status: 200 }));
    await appsApi.list('acme', 'pay');
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/orgs/acme/projects/pay/apps');
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer header.payload.sig');
  });

  it('auditApi.listForFlag builds /audit-log query with flag scope, limit, offset', async () => {
    localStorage.setItem('ds_token', 'header.payload.sig');
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ entries: [], total: 0 }), { status: 200 }),
    );
    await auditApi.listForFlag('f1', { limit: 20, offset: 40 });
    const url = fetchMock.mock.calls[0][0] as string;
    expect(url).toContain('/api/v1/audit-log?');
    expect(url).toContain('resource_type=flag');
    expect(url).toContain('resource_id=f1');
    expect(url).toContain('limit=20');
    expect(url).toContain('offset=40');
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer header.payload.sig');
  });

  it('auditApi.listForFlag omits limit and offset when not provided', async () => {
    localStorage.setItem('ds_token', 'header.payload.sig');
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ entries: [], total: 0 }), { status: 200 }),
    );
    await auditApi.listForFlag('f1');
    const url = fetchMock.mock.calls[0][0] as string;
    expect(url).toContain('resource_type=flag');
    expect(url).toContain('resource_id=f1');
    expect(url).not.toContain('limit=');
    expect(url).not.toContain('offset=');
  });
});
