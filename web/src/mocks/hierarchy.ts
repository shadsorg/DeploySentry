import type { Application, Member, Group, OrgEnvironment, ApiKey } from '@/types';

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

export function getMockEnvironments(): OrgEnvironment[] {
  return MOCK_ENVIRONMENTS;
}

export function getEnvironmentName(envId: string): string {
  return MOCK_ENVIRONMENTS.find((e) => e.id === envId)?.name ?? envId;
}

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
