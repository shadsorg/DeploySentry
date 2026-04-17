import type {
  Flag,
  Deployment,
  DeploymentPhase,
  Release,
  ApiKey,
  CreateFlagRequest,
  UpdateFlagRequest,
  TargetingRule,
  Organization,
  Project,
  Application,
  FlagEnvironmentState,
  Setting,
  ReleaseFlagChangeAPI,
  Member,
  Environment,
  OrgEnvironment,
  DeleteResult,
  RuleEnvironmentState,
  AuditLogEntry,
  Agent,
  AgentHeartbeat,
} from './types';

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
  archive: (id: string) => request<{ status: string }>(`/flags/${id}/archive`, { method: 'POST' }),
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
  listRules: (flagId: string) => request<{ rules: TargetingRule[] }>(`/flags/${flagId}/rules`),
  setRuleEnvState: (flagId: string, ruleId: string, envId: string, data: { enabled: boolean }) =>
    request<RuleEnvironmentState>(`/flags/${flagId}/rules/${ruleId}/environments/${envId}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  listRuleEnvStates: (flagId: string) =>
    request<{ rule_environment_states: RuleEnvironmentState[] }>(`/flags/${flagId}/rules/environment-states`),
  exportFlags: async (projectId: string, application: string, format: 'yaml' | 'json' = 'yaml') => {
    const token = localStorage.getItem('ds_token') || '';
    const res = await fetch(`${BASE}/flags/export?project_id=${projectId}&application=${application}&format=${format}`, {
      headers: {
        Authorization: token.startsWith('ds_') ? `ApiKey ${token}` : `Bearer ${token}`,
      },
    });
    if (!res.ok) throw new Error(`Export failed: ${res.status}`);
    return res.text();
  },
};

// Deployments
export const deploymentsApi = {
  list: (applicationId: string) =>
    request<{ deployments: Deployment[] }>(`/deployments?app_id=${applicationId}`),
  get: (id: string) => request<Deployment>(`/deployments/${id}`),
  create: (data: {
    project_id: string;
    environment_id: string;
    version: string;
    strategy: string;
  }) => request<Deployment>('/deployments', { method: 'POST', body: JSON.stringify(data) }),
  promote: (id: string) => request<Deployment>(`/deployments/${id}/promote`, { method: 'POST' }),
  rollback: (id: string) => request<Deployment>(`/deployments/${id}/rollback`, { method: 'POST' }),
  pause: (id: string) => request<Deployment>(`/deployments/${id}/pause`, { method: 'POST' }),
  resume: (id: string) => request<Deployment>(`/deployments/${id}/resume`, { method: 'POST' }),
  cancel: (id: string) => request<Deployment>(`/deployments/${id}/cancel`, { method: 'POST' }),
  advance: (id: string) => request<Deployment>(`/deployments/${id}/advance`, { method: 'POST' }),
  desiredState: (id: string) => request<any>(`/deployments/${id}/desired-state`),
  rollbackHistory: (id: string) => request<{ rollbacks: any[] }>(`/deployments/${id}/rollback-history`),
  phases: (id: string) => request<{ phases: DeploymentPhase[] }>(`/deployments/${id}/phases`),
};

// Agents
export const agentsApi = {
  listByApp: (appId: string) =>
    request<{ agents: Agent[] }>(`/applications/${appId}/agents`),
  heartbeats: (agentId: string, deploymentId?: string) => {
    const qs = deploymentId ? `?deployment_id=${deploymentId}` : '';
    return request<{ heartbeats: AgentHeartbeat[] }>(`/agents/${agentId}/heartbeats${qs}`);
  },
};

// Releases
export const releasesApi = {
  list: (applicationId: string) =>
    request<{ releases: Release[] }>(`/applications/${applicationId}/releases`),
  get: (id: string) => request<Release>(`/releases/${id}`),
  create: (data: {
    project_id: string;
    version: string;
    description?: string;
    commit_sha?: string;
  }) => request<Release>('/releases', { method: 'POST', body: JSON.stringify(data) }),
  delete: (id: string) => request<void>(`/releases/${id}`, { method: 'DELETE' }),
  start: (id: string) => request<{ status: string }>(`/releases/${id}/start`, { method: 'POST' }),
  promote: (id: string, trafficPercent: number) =>
    request<{ status: string }>(`/releases/${id}/promote`, {
      method: 'POST',
      body: JSON.stringify({ traffic_percent: trafficPercent }),
    }),
  pause: (id: string) => request<{ status: string }>(`/releases/${id}/pause`, { method: 'POST' }),
  rollback: (id: string) =>
    request<{ status: string }>(`/releases/${id}/rollback`, { method: 'POST' }),
  complete: (id: string) =>
    request<{ status: string }>(`/releases/${id}/complete`, { method: 'POST' }),
  addFlagChange: (
    releaseId: string,
    data: { flag_id: string; environment_id: string; new_enabled?: boolean },
  ) =>
    request<ReleaseFlagChangeAPI>(`/releases/${releaseId}/flag-changes`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  listFlagChanges: (releaseId: string) =>
    request<{ flag_changes: ReleaseFlagChangeAPI[] }>(`/releases/${releaseId}/flag-changes`),
};

// Members
export const membersApi = {
  // Org members
  listByOrg: (orgSlug: string) => request<{ members: Member[] }>(`/orgs/${orgSlug}/members`),
  addToOrg: (orgSlug: string, email: string, role: string) =>
    request<{ member: Member }>(`/orgs/${orgSlug}/members`, {
      method: 'POST',
      body: JSON.stringify({ email, role }),
    }),
  updateOrgRole: (orgSlug: string, userId: string, role: string) =>
    request<{ member: Member }>(`/orgs/${orgSlug}/members/${userId}`, {
      method: 'PUT',
      body: JSON.stringify({ role }),
    }),
  removeFromOrg: (orgSlug: string, userId: string) =>
    request<void>(`/orgs/${orgSlug}/members/${userId}`, { method: 'DELETE' }),

  // Project members
  listByProject: (orgSlug: string, projectSlug: string) =>
    request<{ members: Member[] }>(`/orgs/${orgSlug}/projects/${projectSlug}/members`),
  addToProject: (orgSlug: string, projectSlug: string, email: string, role: string) =>
    request<{ member: Member }>(`/orgs/${orgSlug}/projects/${projectSlug}/members`, {
      method: 'POST',
      body: JSON.stringify({ email, role }),
    }),
  updateProjectRole: (orgSlug: string, projectSlug: string, userId: string, role: string) =>
    request<{ member: Member }>(`/orgs/${orgSlug}/projects/${projectSlug}/members/${userId}`, {
      method: 'PUT',
      body: JSON.stringify({ role }),
    }),
  removeFromProject: (orgSlug: string, projectSlug: string, userId: string) =>
    request<void>(`/orgs/${orgSlug}/projects/${projectSlug}/members/${userId}`, {
      method: 'DELETE',
    }),
};

// API Keys
export const apiKeysApi = {
  list: () => request<{ api_keys: ApiKey[] }>('/api-keys'),
  create: (data: { name: string; scopes: string[]; environment_ids?: string[] }) =>
    request<{ api_key: ApiKey; plaintext_key: string }>('/api-keys', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  revoke: (id: string) => request<void>(`/api-keys/${id}`, { method: 'DELETE' }),
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

// Health
export const healthApi = {
  check: () => fetch('/health').then((r) => r.json()),
  ready: () => fetch('/ready').then((r) => r.json()),
};

// Analytics
export const analyticsApi = {
  getSummary: (projectId: string, environmentId: string, timeRange: string) =>
    request<Record<string, unknown>>(
      `/analytics/summary?project_id=${projectId}&environment_id=${environmentId}&time_range=${timeRange}`,
    ),
  getFlagStats: (projectId: string, environmentId: string, timeRange: string, limit?: number) => {
    const qs = new URLSearchParams({
      project_id: projectId,
      environment_id: environmentId,
      time_range: timeRange,
    });
    if (limit) qs.set('limit', String(limit));
    return request<Record<string, unknown>>(`/analytics/flags/stats?${qs}`);
  },
  getFlagUsage: (projectId: string, environmentId: string, flagKey: string, timeRange: string) =>
    request<Record<string, unknown>>(
      `/analytics/flags/${flagKey}/usage?project_id=${projectId}&environment_id=${environmentId}&time_range=${timeRange}`,
    ),
  getDeploymentStats: (projectId: string, timeRange: string, environmentId?: string) => {
    const qs = new URLSearchParams({ project_id: projectId, time_range: timeRange });
    if (environmentId) qs.set('environment_id', environmentId);
    return request<Record<string, unknown>>(`/analytics/deployments/stats?${qs}`);
  },
  getSystemHealth: () => request<Record<string, unknown>>('/analytics/health'),
  streamMetrics: () => new EventSource('/api/v1/analytics/metrics/stream'),
  refreshAggregations: () =>
    request<{ message: string; timestamp: string }>('/analytics/admin/refresh', { method: 'POST' }),
  exportAnalytics: (
    projectId: string,
    startDate: string,
    endDate: string,
    format: string = 'json',
  ) => {
    const qs = new URLSearchParams({
      project_id: projectId,
      start_date: startDate,
      end_date: endDate,
      format,
    });
    return request<Record<string, unknown>>(`/analytics/admin/export?${qs}`);
  },
};

// Flag Environment State
export const flagEnvStateApi = {
  list: (flagId: string) =>
    request<{ environment_states: FlagEnvironmentState[] }>(`/flags/${flagId}/environments`),
  set: (flagId: string, envId: string, data: { enabled: boolean; value?: unknown }) =>
    request<FlagEnvironmentState>(`/flags/${flagId}/environments/${envId}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
};

// Settings
export const settingsApi = {
  list: (scope: string, targetId: string) =>
    request<{ settings: Setting[] }>(`/settings?scope=${scope}&target=${targetId}`),
  resolve: (
    key: string,
    params: {
      org_id?: string;
      project_id?: string;
      application_id?: string;
      environment_id?: string;
    },
  ) => {
    const qs = new URLSearchParams({ key });
    Object.entries(params).forEach(([k, v]) => {
      if (v) qs.set(k, v);
    });
    return request<Setting>(`/settings/resolve?${qs}`);
  },
  set: (data: { scope: string; target_id: string; key: string; value: unknown }) =>
    request<Setting>('/settings', { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) => request<void>(`/settings/${id}`, { method: 'DELETE' }),
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
  listProjects: (orgSlug: string, includeDeleted = false) =>
    request<{ projects: Project[] }>(`/orgs/${orgSlug}/projects${includeDeleted ? '?include_deleted=true' : ''}`),
  getProject: (orgSlug: string, projectSlug: string) =>
    request<Project>(`/orgs/${orgSlug}/projects/${projectSlug}`),
  createProject: (orgSlug: string, data: { name: string; slug: string }) =>
    request<Project>(`/orgs/${orgSlug}/projects`, { method: 'POST', body: JSON.stringify(data) }),
  updateProject: (orgSlug: string, projectSlug: string, data: { name?: string; description?: string; repo_url?: string }) =>
    request<Project>(`/orgs/${orgSlug}/projects/${projectSlug}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  deleteProject: (orgSlug: string, projectSlug: string) =>
    request<DeleteResult>(`/orgs/${orgSlug}/projects/${projectSlug}`, { method: 'DELETE' }),
  hardDeleteProject: (orgSlug: string, projectSlug: string) =>
    request<void>(`/orgs/${orgSlug}/projects/${projectSlug}/permanent`, { method: 'DELETE' }),
  restoreProject: (orgSlug: string, projectSlug: string) =>
    request<Project>(`/orgs/${orgSlug}/projects/${projectSlug}/restore`, { method: 'POST' }),

  // Apps
  listApps: (orgSlug: string, projectSlug: string, includeDeleted = false) =>
    request<{ applications: Application[] }>(`/orgs/${orgSlug}/projects/${projectSlug}/apps${includeDeleted ? '?include_deleted=true' : ''}`),
  getApp: (orgSlug: string, projectSlug: string, appSlug: string) =>
    request<Application>(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}`),
  createApp: (
    orgSlug: string,
    projectSlug: string,
    data: { name: string; slug: string; description?: string },
  ) =>
    request<Application>(`/orgs/${orgSlug}/projects/${projectSlug}/apps`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  updateApp: (
    orgSlug: string,
    projectSlug: string,
    appSlug: string,
    data: { name: string; description?: string },
  ) =>
    request<Application>(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  deleteApp: (orgSlug: string, projectSlug: string, appSlug: string) =>
    request<DeleteResult>(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}`, { method: 'DELETE' }),
  hardDeleteApp: (orgSlug: string, projectSlug: string, appSlug: string) =>
    request<void>(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/permanent`, { method: 'DELETE' }),
  restoreApp: (orgSlug: string, projectSlug: string, appSlug: string) =>
    request<Application>(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/restore`, { method: 'POST' }),

  // Environments (app-level, legacy)
  listEnvironments: (orgSlug: string, projectSlug: string, appSlug: string) =>
    request<{ environments: Environment[] }>(
      `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/environments`,
    ),

  // Org-level environments
  listOrgEnvironments: (orgSlug: string) =>
    request<{ environments: OrgEnvironment[] }>(`/orgs/${orgSlug}/environments`),
  createEnvironment: (orgSlug: string, data: { name: string; slug: string; is_production: boolean }) =>
    request<OrgEnvironment>(`/orgs/${orgSlug}/environments`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  updateEnvironment: (orgSlug: string, envSlug: string, data: Partial<{ name: string; slug: string; is_production: boolean; sort_order: number }>) =>
    request<OrgEnvironment>(`/orgs/${orgSlug}/environments/${envSlug}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  deleteEnvironment: (orgSlug: string, envSlug: string) =>
    request<{ deleted: boolean }>(`/orgs/${orgSlug}/environments/${envSlug}`, {
      method: 'DELETE',
    }),
};

// Groups
export interface Group {
  id: string;
  org_id: string;
  name: string;
  slug: string;
  description: string;
  member_count: number;
  created_at: string;
}

export interface GroupMember {
  group_id: string;
  user_id: string;
  name: string;
  email: string;
  avatar_url?: string;
  created_at: string;
}

export const groupsApi = {
  list: (orgSlug: string) =>
    request<{ groups: Group[] }>(`/orgs/${orgSlug}/groups`).then((r) => r.groups),
  create: (orgSlug: string, data: { name: string; description?: string }) =>
    request<Group>(`/orgs/${orgSlug}/groups`, { method: 'POST', body: JSON.stringify(data) }),
  update: (orgSlug: string, groupSlug: string, data: { name: string; description?: string }) =>
    request<Group>(`/orgs/${orgSlug}/groups/${groupSlug}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (orgSlug: string, groupSlug: string) =>
    request<void>(`/orgs/${orgSlug}/groups/${groupSlug}`, { method: 'DELETE' }),
  listMembers: (orgSlug: string, groupSlug: string) =>
    request<{ members: GroupMember[] }>(`/orgs/${orgSlug}/groups/${groupSlug}/members`).then((r) => r.members),
  addMember: (orgSlug: string, groupSlug: string, userId: string) =>
    request<void>(`/orgs/${orgSlug}/groups/${groupSlug}/members`, { method: 'POST', body: JSON.stringify({ user_id: userId }) }),
  removeMember: (orgSlug: string, groupSlug: string, userId: string) =>
    request<void>(`/orgs/${orgSlug}/groups/${groupSlug}/members/${userId}`, { method: 'DELETE' }),
};

// Resource Grants
export interface ResourceGrant {
  id: string;
  org_id: string;
  project_id?: string;
  application_id?: string;
  user_id?: string;
  group_id?: string;
  permission: 'read' | 'write';
  grantee_name: string;
  grantee_type: 'user' | 'group';
  created_at: string;
}

export const grantsApi = {
  listProjectGrants: (orgSlug: string, projectSlug: string) =>
    request<{ grants: ResourceGrant[] }>(`/orgs/${orgSlug}/projects/${projectSlug}/grants`).then((r) => r.grants),
  createProjectGrant: (orgSlug: string, projectSlug: string, data: { user_id?: string; group_id?: string; permission: string }) =>
    request<ResourceGrant>(`/orgs/${orgSlug}/projects/${projectSlug}/grants`, { method: 'POST', body: JSON.stringify(data) }),
  listAppGrants: (orgSlug: string, projectSlug: string, appSlug: string) =>
    request<{ grants: ResourceGrant[] }>(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/grants`).then((r) => r.grants),
  createAppGrant: (orgSlug: string, projectSlug: string, appSlug: string, data: { user_id?: string; group_id?: string; permission: string }) =>
    request<ResourceGrant>(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/grants`, { method: 'POST', body: JSON.stringify(data) }),
  deleteGrant: (orgSlug: string, projectSlug: string, grantId: string, appSlug?: string) => {
    const base = appSlug
      ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/grants/${grantId}`
      : `/orgs/${orgSlug}/projects/${projectSlug}/grants/${grantId}`;
    return request<void>(base, { method: 'DELETE' });
  },
};

// Webhooks
export interface Webhook {
  id: string;
  url: string;
  events: string[];
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export const webhooksApi = {
  list: () => request<{ webhooks: Webhook[] }>('/webhooks'),
  get: (id: string) => request<Webhook>(`/webhooks/${id}`),
  create: (data: { url: string; events: string[]; is_active?: boolean }) =>
    request<Webhook>('/webhooks', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  update: (id: string, data: Partial<{ url: string; events: string[]; is_active: boolean }>) =>
    request<Webhook>(`/webhooks/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  delete: (id: string) =>
    request<{ deleted: boolean }>(`/webhooks/${id}`, { method: 'DELETE' }),
  test: (id: string) =>
    request<{ success: boolean; status_code: number }>(`/webhooks/${id}/test`, {
      method: 'POST',
    }),
  deliveries: (id: string) =>
    request<{ deliveries: unknown[] }>(`/webhooks/${id}/deliveries`),
};

// Notifications
export interface ChannelConfig {
  enabled: boolean;
  webhook_url?: string;
  channel?: string;
  smtp_host?: string;
  smtp_port?: number;
  username?: string;
  password?: string;
  from?: string;
  routing_key?: string;
  source: 'config' | 'api';
}

export interface NotificationPreferences {
  channels: Record<string, ChannelConfig>;
  event_routing: Record<string, string[]>;
}

export const notificationsApi = {
  getPreferences: () =>
    request<NotificationPreferences>('/notifications/preferences'),
  savePreferences: (data: {
    channels?: Record<string, Partial<ChannelConfig>>;
    event_routing?: Record<string, string[]>;
  }) =>
    request<{ saved: boolean }>('/notifications/preferences', {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  resetPreferences: () =>
    request<{ reset: boolean }>('/notifications/preferences', {
      method: 'DELETE',
    }),
};

// Audit Log
export const auditApi = {
  query: (params: { resource_type?: string; resource_id?: string; limit?: number; offset?: number }) => {
    const qs = new URLSearchParams();
    if (params.resource_type) qs.set('resource_type', params.resource_type);
    if (params.resource_id) qs.set('resource_id', params.resource_id);
    if (params.limit) qs.set('limit', String(params.limit));
    if (params.offset) qs.set('offset', String(params.offset));
    return request<{ entries: AuditLogEntry[]; total: number }>(`/audit-log?${qs}`);
  },
};
