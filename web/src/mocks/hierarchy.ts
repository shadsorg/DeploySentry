import type { Organization, Application, Project } from '@/types';

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
