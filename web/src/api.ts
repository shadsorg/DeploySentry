import type { Flag, Deployment, Release, ApiKey, CreateFlagRequest, UpdateFlagRequest, TargetingRule, Organization, Project, Application } from './types';

const BASE = '/api/v1';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = localStorage.getItem('ds_token') || '';
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      Authorization: token.startsWith('ds_') ? `ApiKey ${token}` : `Bearer ${token}`,
      ...init?.headers,
    },
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `Request failed: ${res.status}`);
  }
  return res.json();
}

// Flags
export const flagsApi = {
  list: (projectId: string, params?: { category?: string; archived?: boolean }) => {
    const qs = new URLSearchParams({ project_id: projectId });
    if (params?.category) qs.set('category', params.category);
    if (params?.archived !== undefined) qs.set('archived', String(params.archived));
    return request<{ flags: Flag[] }>(`/flags?${qs}`);
  },
  get: (id: string) => request<Flag>(`/flags/${id}`),
  create: (data: CreateFlagRequest) =>
    request<Flag>('/flags', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: UpdateFlagRequest) =>
    request<Flag>(`/flags/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  toggle: (id: string, enabled: boolean) =>
    request<{ enabled: boolean }>(`/flags/${id}/toggle`, {
      method: 'POST',
      body: JSON.stringify({ enabled }),
    }),
  archive: (id: string) =>
    request<{ status: string }>(`/flags/${id}/archive`, { method: 'POST' }),
  addRule: (flagId: string, rule: Partial<TargetingRule>) =>
    request<TargetingRule>(`/flags/${flagId}/rules`, {
      method: 'POST',
      body: JSON.stringify(rule),
    }),
  updateRule: (flagId: string, ruleId: string, rule: Partial<TargetingRule>) =>
    request<TargetingRule>(`/flags/${flagId}/rules/${ruleId}`, {
      method: 'PUT',
      body: JSON.stringify(rule),
    }),
  deleteRule: (flagId: string, ruleId: string) =>
    request<void>(`/flags/${flagId}/rules/${ruleId}`, { method: 'DELETE' }),
  listRules: (flagId: string) =>
    request<{ rules: TargetingRule[] }>(`/flags/${flagId}/rules`),
};

// Deployments
export const deploymentsApi = {
  list: (applicationId: string) =>
    request<{ deployments: Deployment[] }>(`/deployments?application_id=${applicationId}`),
  get: (id: string) => request<Deployment>(`/deployments/${id}`),
  create: (data: { project_id: string; environment_id: string; version: string; strategy: string }) =>
    request<Deployment>('/deployments', { method: 'POST', body: JSON.stringify(data) }),
  promote: (id: string) =>
    request<Deployment>(`/deployments/${id}/promote`, { method: 'POST' }),
  rollback: (id: string) =>
    request<Deployment>(`/deployments/${id}/rollback`, { method: 'POST' }),
  pause: (id: string) =>
    request<Deployment>(`/deployments/${id}/pause`, { method: 'POST' }),
  resume: (id: string) =>
    request<Deployment>(`/deployments/${id}/resume`, { method: 'POST' }),
};

// Releases
export const releasesApi = {
  list: (applicationId: string) =>
    request<{ releases: Release[] }>(`/releases?application_id=${applicationId}`),
  get: (id: string) => request<Release>(`/releases/${id}`),
  create: (data: { project_id: string; version: string; description?: string; commit_sha?: string }) =>
    request<Release>('/releases', { method: 'POST', body: JSON.stringify(data) }),
  promote: (id: string, environmentId: string) =>
    request<Release>(`/releases/${id}/promote`, {
      method: 'POST',
      body: JSON.stringify({ environment_id: environmentId }),
    }),
};

// API Keys
export const apiKeysApi = {
  list: () => request<{ api_keys: ApiKey[] }>('/api-keys'),
  create: (data: { name: string; scopes: string[] }) =>
    request<{ api_key: ApiKey; token: string }>('/api-keys', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  revoke: (id: string) =>
    request<void>(`/api-keys/${id}`, { method: 'DELETE' }),
};

// Auth (public - no token required)
export interface AuthUser {
  id: string;
  email: string;
  name: string;
  avatar_url?: string;
}

export const authApi = {
  register: (data: { email: string; password: string; name: string }) =>
    fetch(`${BASE}/auth/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }).then(async (res) => {
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.error || 'Registration failed');
      }
      return res.json() as Promise<{ token: string; user: AuthUser }>;
    }),

  login: (data: { email: string; password: string }) =>
    fetch(`${BASE}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }).then(async (res) => {
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.error || 'Login failed');
      }
      return res.json() as Promise<{ token: string; user: AuthUser }>;
    }),

  me: () => request<AuthUser>('/users/me'),

  logout: () => {
    localStorage.removeItem('ds_token');
  },
};

// Notifications
export const notificationsApi = {
  getSettings: () => request<any>('/notifications/settings'),
  updateSettings: (data: any) =>
    request<any>('/notifications/settings', { method: 'PUT', body: JSON.stringify(data) }),
  testChannel: (channel: string) =>
    request<{ status: string }>(`/notifications/test/${channel}`, { method: 'POST' }),
};

// Health
export const healthApi = {
  check: () => fetch('/health').then((r) => r.json()),
  ready: () => fetch('/ready').then((r) => r.json()),
};

// Analytics
export const analyticsApi = {
  getSummary: (projectId: string, environmentId: string, timeRange: string) =>
    request<any>(`/analytics/summary?project_id=${projectId}&environment_id=${environmentId}&time_range=${timeRange}`),
  getFlagStats: (projectId: string, environmentId: string, timeRange: string, limit?: number) => {
    const qs = new URLSearchParams({ project_id: projectId, environment_id: environmentId, time_range: timeRange });
    if (limit) qs.set('limit', String(limit));
    return request<any>(`/analytics/flags/stats?${qs}`);
  },
  getFlagUsage: (projectId: string, environmentId: string, flagKey: string, timeRange: string) =>
    request<any>(`/analytics/flags/${flagKey}/usage?project_id=${projectId}&environment_id=${environmentId}&time_range=${timeRange}`),
  getDeploymentStats: (projectId: string, timeRange: string, environmentId?: string) => {
    const qs = new URLSearchParams({ project_id: projectId, time_range: timeRange });
    if (environmentId) qs.set('environment_id', environmentId);
    return request<any>(`/analytics/deployments/stats?${qs}`);
  },
  getSystemHealth: () => request<any>('/analytics/health'),
  streamMetrics: () => new EventSource('/api/v1/analytics/metrics/stream'),
  refreshAggregations: () =>
    request<{ message: string; timestamp: string }>('/analytics/admin/refresh', { method: 'POST' }),
  exportAnalytics: (projectId: string, startDate: string, endDate: string, format: string = 'json') => {
    const qs = new URLSearchParams({ project_id: projectId, start_date: startDate, end_date: endDate, format });
    return request<any>(`/analytics/admin/export?${qs}`);
  },
};

// Entities (Orgs / Projects / Apps)
export const entitiesApi = {
  // Orgs
  listOrgs: () => request<{ organizations: Organization[] }>('/orgs'),
  getOrg: (slug: string) => request<Organization>(`/orgs/${slug}`),
  createOrg: (data: { name: string; slug: string }) =>
    request<Organization>('/orgs', { method: 'POST', body: JSON.stringify(data) }),
  updateOrg: (slug: string, data: { name: string }) =>
    request<Organization>(`/orgs/${slug}`, { method: 'PUT', body: JSON.stringify(data) }),

  // Projects
  listProjects: (orgSlug: string) =>
    request<{ projects: Project[] }>(`/orgs/${orgSlug}/projects`),
  getProject: (orgSlug: string, projectSlug: string) =>
    request<Project>(`/orgs/${orgSlug}/projects/${projectSlug}`),
  createProject: (orgSlug: string, data: { name: string; slug: string }) =>
    request<Project>(`/orgs/${orgSlug}/projects`, { method: 'POST', body: JSON.stringify(data) }),
  updateProject: (orgSlug: string, projectSlug: string, data: { name: string }) =>
    request<Project>(`/orgs/${orgSlug}/projects/${projectSlug}`, { method: 'PUT', body: JSON.stringify(data) }),

  // Apps
  listApps: (orgSlug: string, projectSlug: string) =>
    request<{ applications: Application[] }>(`/orgs/${orgSlug}/projects/${projectSlug}/apps`),
  getApp: (orgSlug: string, projectSlug: string, appSlug: string) =>
    request<Application>(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}`),
  createApp: (orgSlug: string, projectSlug: string, data: { name: string; slug: string; description?: string }) =>
    request<Application>(`/orgs/${orgSlug}/projects/${projectSlug}/apps`, { method: 'POST', body: JSON.stringify(data) }),
  updateApp: (orgSlug: string, projectSlug: string, appSlug: string, data: { name: string; description?: string }) =>
    request<Application>(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}`, { method: 'PUT', body: JSON.stringify(data) }),
};
