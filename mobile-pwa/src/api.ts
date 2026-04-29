import type {
  AuthUser,
  Organization,
  OrgStatusResponse,
  OrgDeploymentsFilters,
  OrgDeploymentsResponse,
  Project,
} from './types';

const BASE = '/api/v1';

type FetchFn = typeof fetch;
let fetchImpl: FetchFn = (...args) => globalThis.fetch(...args);

export function setFetch(impl: FetchFn) {
  fetchImpl = impl;
}

function handleUnauthorized() {
  const path = window.location.pathname;
  if (path === '/m/login' || path.endsWith('/login')) return;
  localStorage.removeItem('ds_token');
  const next = encodeURIComponent(path + window.location.search);
  window.location.assign(`/m/login?next=${next}`);
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = localStorage.getItem('ds_token') ?? '';
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(init?.headers as Record<string, string> | undefined),
  };
  if (token) {
    headers.Authorization = token.startsWith('ds_') ? `ApiKey ${token}` : `Bearer ${token}`;
  }
  const res = await fetchImpl(`${BASE}${path}`, { ...init, headers });
  if (res.status === 401) {
    handleUnauthorized();
    const body = await res.json().catch(() => ({}));
    throw new Error((body as { error?: string }).error ?? 'Session expired');
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as { error?: string }).error ?? `Request failed: ${res.status}`);
  }
  return (await res.json()) as T;
}

// Public (no Authorization header needed)
async function publicPost<T>(path: string, body: unknown): Promise<T> {
  const res = await fetchImpl(`${BASE}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const payload = await res.json().catch(() => ({}));
    throw new Error((payload as { error?: string }).error ?? `Request failed: ${res.status}`);
  }
  return (await res.json()) as T;
}

export const authApi = {
  login: (data: { email: string; password: string }) =>
    publicPost<{ token: string; user: AuthUser }>('/auth/login', data),
  register: (data: { email: string; password: string; name: string }) =>
    publicPost<{ token: string; user: AuthUser }>('/auth/register', data),
  me: () => request<AuthUser>('/users/me'),
  extend: () => request<{ token: string }>('/auth/extend', { method: 'POST' }),
  logout: () => {
    localStorage.removeItem('ds_token');
  },
};

export const orgsApi = {
  list: () => request<{ organizations: Organization[] }>('/orgs'),
};

function buildQueryString(params: Record<string, string | number | undefined>): string {
  const parts = Object.entries(params)
    .filter(([, v]) => v !== undefined && v !== '' && v !== null)
    .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`);
  return parts.length ? `?${parts.join('&')}` : '';
}

export const orgStatusApi = {
  get: (orgSlug: string) => request<OrgStatusResponse>(`/orgs/${orgSlug}/status`),
};

export const orgDeploymentsApi = {
  list: (orgSlug: string, filters: OrgDeploymentsFilters = {}) =>
    request<OrgDeploymentsResponse>(
      `/orgs/${orgSlug}/deployments${buildQueryString(filters as Record<string, string | number | undefined>)}`,
    ),
};

export const projectsApi = {
  list: (orgSlug: string) =>
    request<{ projects: Project[] }>(`/orgs/${orgSlug}/projects`),
};
