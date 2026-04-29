import type {
  Application,
  AuditLogEntry,
  AuthUser,
  Flag,
  FlagEnvironmentState,
  Organization,
  OrgEnvironment,
  OrgStatusResponse,
  OrgDeploymentsFilters,
  OrgDeploymentsResponse,
  Project,
  RuleEnvironmentState,
  TargetingRule,
} from './types';
import { OfflineWriteBlockedError } from './lib/offlineError';

const WRITE_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

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
  const method = init?.method?.toUpperCase();
  if (
    method &&
    WRITE_METHODS.has(method) &&
    typeof navigator !== 'undefined' &&
    navigator.onLine === false
  ) {
    throw new OfflineWriteBlockedError();
  }
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

export const flagsApi = {
  list: (
    projectId: string,
    params: { category?: string; archived?: boolean; applicationId?: string } = {},
  ) => {
    const qs = new URLSearchParams({ project_id: projectId });
    if (params.category) qs.set('category', params.category);
    if (params.archived !== undefined) qs.set('archived', String(params.archived));
    if (params.applicationId) qs.set('application_id', params.applicationId);
    return request<{ flags: Flag[] }>(`/flags?${qs.toString()}`);
  },
  get: (id: string) => request<Flag>(`/flags/${id}`),
  listRules: (flagId: string) =>
    request<{ rules: TargetingRule[] }>(`/flags/${flagId}/rules`),
  listRuleEnvStates: (flagId: string) =>
    request<{ rule_environment_states: RuleEnvironmentState[] }>(
      `/flags/${flagId}/rules/environment-states`,
    ),
  updateRule: (flagId: string, ruleId: string, patch: Partial<TargetingRule>) =>
    request<TargetingRule>(`/flags/${flagId}/rules/${ruleId}`, {
      method: 'PUT',
      body: JSON.stringify(patch),
    }),
  setRuleEnvState: (
    flagId: string,
    ruleId: string,
    envId: string,
    patch: { enabled: boolean },
  ) =>
    request<RuleEnvironmentState>(
      `/flags/${flagId}/rules/${ruleId}/environments/${envId}`,
      { method: 'PUT', body: JSON.stringify(patch) },
    ),
};

export const flagEnvStateApi = {
  list: (flagId: string) =>
    request<{ environment_states: FlagEnvironmentState[] }>(`/flags/${flagId}/environments`),
  set: (
    flagId: string,
    envId: string,
    patch: { enabled?: boolean; value?: unknown },
  ) =>
    request<FlagEnvironmentState>(`/flags/${flagId}/environments/${envId}`, {
      method: 'PUT',
      body: JSON.stringify(patch),
    }),
};

export const envApi = {
  listOrg: (orgSlug: string) =>
    request<{ environments: OrgEnvironment[] }>(`/orgs/${orgSlug}/environments`),
};

export const appsApi = {
  list: (orgSlug: string, projectSlug: string) =>
    request<{ applications: Application[] }>(
      `/orgs/${orgSlug}/projects/${projectSlug}/apps`,
    ),
};

export const auditApi = {
  listForFlag: (flagId: string, opts: { limit?: number; offset?: number } = {}) => {
    const qs = new URLSearchParams({ resource_type: 'flag', resource_id: flagId });
    if (opts.limit !== undefined) qs.set('limit', String(opts.limit));
    if (opts.offset !== undefined) qs.set('offset', String(opts.offset));
    return request<{ entries: AuditLogEntry[]; total: number }>(`/audit-log?${qs.toString()}`);
  },
};
