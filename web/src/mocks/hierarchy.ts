import type { Organization, Application, Project, FlagEnvState, DeploymentEvent, Deployment, Release, ReleaseFlagChange, Member, Group, OrgEnvironment, ApiKey } from '@/types';

export const MOCK_ORGS: Organization[] = [
  {
    id: 'org-1',
    name: 'Acme Corp',
    slug: 'acme-corp',
    created_at: '2025-06-01T00:00:00Z',
    updated_at: '2026-03-01T00:00:00Z',
  },
  {
    id: 'org-2',
    name: 'Personal',
    slug: 'personal',
    created_at: '2025-08-15T00:00:00Z',
    updated_at: '2026-02-01T00:00:00Z',
  },
];

export const MOCK_PROJECTS: Project[] = [
  { id: 'proj-1', name: 'Platform', slug: 'platform', org_id: 'org-1' },
  { id: 'proj-2', name: 'Mobile', slug: 'mobile', org_id: 'org-1' },
  { id: 'proj-3', name: 'Side Project', slug: 'side-project', org_id: 'org-2' },
];

export const MOCK_APPLICATIONS: Application[] = [
  {
    id: 'app-1',
    project_id: 'proj-1',
    name: 'API Server',
    slug: 'api-server',
    description: 'Core REST API',
    repo_url: 'https://github.com/acme/api-server',
    created_at: '2025-07-01T00:00:00Z',
    updated_at: '2026-03-20T00:00:00Z',
  },
  {
    id: 'app-2',
    project_id: 'proj-1',
    name: 'Web App',
    slug: 'web-app',
    description: 'Customer-facing React SPA',
    created_at: '2025-07-01T00:00:00Z',
    updated_at: '2026-03-15T00:00:00Z',
  },
  {
    id: 'app-3',
    project_id: 'proj-1',
    name: 'Worker',
    slug: 'worker',
    description: 'Background job processor',
    created_at: '2025-09-01T00:00:00Z',
    updated_at: '2026-02-10T00:00:00Z',
  },
  {
    id: 'app-4',
    project_id: 'proj-2',
    name: 'iOS App',
    slug: 'ios-app',
    description: 'iOS mobile application',
    created_at: '2025-10-01T00:00:00Z',
    updated_at: '2026-03-01T00:00:00Z',
  },
  {
    id: 'app-5',
    project_id: 'proj-2',
    name: 'Android App',
    slug: 'android-app',
    description: 'Android mobile application',
    created_at: '2025-10-01T00:00:00Z',
    updated_at: '2026-03-01T00:00:00Z',
  },
  {
    id: 'app-6',
    project_id: 'proj-3',
    name: 'CLI Tool',
    slug: 'cli-tool',
    description: 'Command-line utility',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-03-10T00:00:00Z',
  },
];

/** Get orgs for current user */
export function getMockOrgs(): Organization[] {
  return MOCK_ORGS;
}

/** Get projects for an org by slug */
export function getMockProjects(orgSlug: string): Project[] {
  const org = MOCK_ORGS.find((o) => o.slug === orgSlug);
  if (!org) return [];
  return MOCK_PROJECTS.filter((p) => p.org_id === org.id);
}

/** Get applications for a project by slug */
export function getMockApps(projectSlug: string): Application[] {
  const project = MOCK_PROJECTS.find((p) => p.slug === projectSlug);
  if (!project) return [];
  return MOCK_APPLICATIONS.filter((a) => a.project_id === project.id);
}

/** Resolve a slug to a display name */
export function getOrgName(orgSlug: string): string {
  return MOCK_ORGS.find((o) => o.slug === orgSlug)?.name ?? orgSlug;
}

export function getProjectName(projectSlug: string): string {
  return MOCK_PROJECTS.find((p) => p.slug === projectSlug)?.name ?? projectSlug;
}

export function getAppName(appSlug: string): string {
  return MOCK_APPLICATIONS.find((a) => a.slug === appSlug)?.name ?? appSlug;
}

export function getMockEnvironments(): OrgEnvironment[] {
  return MOCK_ENVIRONMENTS;
}

export function getEnvironmentName(envId: string): string {
  return MOCK_ENVIRONMENTS.find((e) => e.id === envId)?.name ?? envId;
}

export const MOCK_FLAG_ENV_STATE: FlagEnvState[] = [
  {
    flag_id: 'flag-001',
    environment_id: 'env-dev',
    environment_name: 'Development',
    enabled: true,
    value: 'true',
    updated_by: 'alice',
    updated_at: '2026-03-18T14:30:00Z',
  },
  {
    flag_id: 'flag-001',
    environment_id: 'env-staging',
    environment_name: 'Staging',
    enabled: true,
    value: 'true',
    updated_by: 'bob',
    updated_at: '2026-03-19T10:00:00Z',
  },
  {
    flag_id: 'flag-001',
    environment_id: 'env-prod',
    environment_name: 'Production',
    enabled: false,
    value: 'false',
    updated_by: 'alice',
    updated_at: '2026-03-15T09:00:00Z',
  },
];

export const MOCK_DEPLOYMENT_DETAIL: Deployment = {
  id: 'dep-1',
  application_id: 'app-1',
  environment_id: 'env-prod',
  version: 'v2.4.1',
  commit_sha: 'abc123f',
  artifact: 'https://registry.acme.com/api-server:v2.4.1',
  strategy: 'canary',
  status: 'running',
  traffic_percent: 25,
  health_score: 99.8,
  created_by: 'alice@example.com',
  created_at: '2026-03-21T09:15:00Z',
  updated_at: '2026-03-21T10:30:00Z',
  started_at: '2026-03-21T09:20:00Z',
  completed_at: null,
};

export const MOCK_DEPLOYMENT_EVENTS: DeploymentEvent[] = [
  { status: 'running', timestamp: '2026-03-21T10:30:00Z', note: 'Traffic increased to 25%' },
  { status: 'running', timestamp: '2026-03-21T09:45:00Z', note: 'Traffic increased to 10%' },
  { status: 'running', timestamp: '2026-03-21T09:20:00Z', note: 'Canary deployment started' },
  { status: 'pending', timestamp: '2026-03-21T09:15:00Z', note: 'Deployment created' },
];

export const MOCK_RELEASE_DETAIL: Release = {
  id: 'rel-1',
  application_id: 'app-1',
  name: 'Enable Checkout V2',
  description: 'Gradual rollout of checkout v2 flags across all environments',
  session_sticky: true,
  sticky_header: 'X-Session-ID',
  traffic_percent: 25,
  status: 'rolling_out',
  created_by: 'alice@example.com',
  started_at: '2026-03-21T10:00:00Z',
  created_at: '2026-03-20T14:00:00Z',
  updated_at: '2026-03-21T10:30:00Z',
};

export const MOCK_ENVIRONMENTS: OrgEnvironment[] = [
  { id: 'env-dev', name: 'Development', slug: 'development', is_production: false, created_at: '2025-06-01T00:00:00Z' },
  { id: 'env-staging', name: 'Staging', slug: 'staging', is_production: false, created_at: '2025-06-01T00:00:00Z' },
  { id: 'env-prod', name: 'Production', slug: 'production', is_production: true, created_at: '2025-06-01T00:00:00Z' },
];

export const MOCK_MEMBERS: Member[] = [
  { id: 'user-1', name: 'Alice Chen', email: 'alice@acme.com', role: 'owner', group_ids: ['grp-1'], joined_at: '2025-06-01T00:00:00Z' },
  { id: 'user-2', name: 'Bob Smith', email: 'bob@acme.com', role: 'member', group_ids: ['grp-1', 'grp-2'], joined_at: '2025-07-15T00:00:00Z' },
  { id: 'user-3', name: 'Carol Davis', email: 'carol@acme.com', role: 'member', group_ids: ['grp-3'], joined_at: '2025-09-01T00:00:00Z' },
  { id: 'user-4', name: 'Dave Wilson', email: 'dave@acme.com', role: 'member', group_ids: ['grp-4'], joined_at: '2026-01-10T00:00:00Z' },
];

export const MOCK_GROUPS: Group[] = [
  { id: 'grp-1', name: 'Platform Admins', role: 'admin', environment_ids: [], application_ids: [], member_ids: ['user-1', 'user-2'], created_at: '2025-06-01T00:00:00Z' },
  { id: 'grp-2', name: 'Production Ops', role: 'editor', environment_ids: ['env-prod'], application_ids: [], member_ids: ['user-2'], created_at: '2025-08-01T00:00:00Z' },
  { id: 'grp-3', name: 'API Team', role: 'editor', environment_ids: [], application_ids: ['app-1', 'app-3'], member_ids: ['user-3'], created_at: '2025-09-15T00:00:00Z' },
  { id: 'grp-4', name: 'Junior Devs', role: 'viewer', environment_ids: ['env-dev', 'env-staging'], application_ids: [], member_ids: ['user-4'], created_at: '2026-01-15T00:00:00Z' },
];

export const MOCK_API_KEYS: ApiKey[] = [
  {
    id: 'key-1',
    name: 'Production Backend',
    prefix: 'ds_prod_abc1****',
    scopes: ['flags:read'],
    environment_targets: ['env-prod'],
    created_at: '2025-11-15T00:00:00Z',
    last_used_at: '2026-03-28T10:00:00Z',
    expires_at: null,
  },
  {
    id: 'key-2',
    name: 'CI/CD Pipeline',
    prefix: 'ds_ci_def2****',
    scopes: ['deploys:read', 'deploys:write'],
    environment_targets: [],
    created_at: '2025-12-01T00:00:00Z',
    last_used_at: '2026-03-28T09:00:00Z',
    expires_at: null,
  },
  {
    id: 'key-3',
    name: 'Admin Dashboard',
    prefix: 'ds_admin_ghi3****',
    scopes: ['admin'],
    environment_targets: [],
    created_at: '2026-01-10T00:00:00Z',
    last_used_at: '2026-03-25T14:00:00Z',
    expires_at: null,
  },
];

export const MOCK_RELEASE_FLAG_CHANGES: ReleaseFlagChange[] = [
  {
    id: 'rfc-1',
    release_id: 'rel-1',
    flag_key: 'checkout-v2-rollout',
    environment_name: 'Production',
    previous_enabled: false,
    new_enabled: true,
    previous_value: 'false',
    new_value: 'true',
    applied_at: '2026-03-21T10:00:00Z',
  },
  {
    id: 'rfc-2',
    release_id: 'rel-1',
    flag_key: 'checkout-v2-rollout',
    environment_name: 'Staging',
    previous_enabled: true,
    new_enabled: true,
    previous_value: 'false',
    new_value: 'true',
    applied_at: '2026-03-21T10:00:00Z',
  },
  {
    id: 'rfc-3',
    release_id: 'rel-1',
    flag_key: 'checkout-theme',
    environment_name: 'Production',
    previous_enabled: true,
    new_enabled: true,
    previous_value: '"v1"',
    new_value: '"v2"',
    applied_at: null,
  },
  {
    id: 'rfc-4',
    release_id: 'rel-1',
    flag_key: 'legacy-checkout-disable',
    environment_name: 'Production',
    previous_enabled: true,
    new_enabled: false,
    previous_value: 'true',
    new_value: 'true',
    applied_at: null,
  },
];
