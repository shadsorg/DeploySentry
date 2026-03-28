# Web UI Phase 1: Core Navigation Overhaul — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure the React dashboard from flat routing to org-scoped hierarchical navigation (Org → Project → Application), with context-aware pages and a redesigned sidebar.

**Architecture:** Replace the flat sidebar and route tree with a hierarchical layout driven by URL params. Org and project are selected via dropdowns, applications via accordion. All pages become context-aware by reading route params. URL is the single source of truth — no separate context state.

**Tech Stack:** React 18, React Router 6, TypeScript 5.5, Vite 5.4, custom CSS (dark theme)

**Spec:** `docs/superpowers/specs/2026-03-28-web-ui-phase1-navigation-design.md`

---

## File Structure

### New Files
- `web/src/mocks/hierarchy.ts` — mock org/project/app data for sidebar and pages
- `web/src/components/Breadcrumb.tsx` — subtle breadcrumb from URL params
- `web/src/components/OrgSwitcher.tsx` — org dropdown for sidebar
- `web/src/components/ProjectSwitcher.tsx` — project dropdown for sidebar
- `web/src/components/AppAccordion.tsx` — expandable app list with nav links
- `web/src/components/HierarchyLayout.tsx` — replaces Layout, wires sidebar + breadcrumb + outlet
- `web/src/components/DefaultRedirect.tsx` — redirects `/` to last-used org
- `web/src/components/LegacyRedirect.tsx` — redirects old flat routes to new hierarchy routes
- `web/src/pages/ProjectListPage.tsx` — org landing page showing project cards
- `web/src/pages/CreateOrgPage.tsx` — stub form for creating an org

### Modified Files
- `web/src/types.ts` — add Organization, Application interfaces
- `web/src/api.ts` — add orgsApi, applicationsApi modules
- `web/src/auth.tsx` — refactor RequireAuth/RedirectIfAuth to Outlet pattern
- `web/src/components/Sidebar.tsx` — complete rewrite with dropdowns + accordion
- `web/src/styles/globals.css` — add sidebar dropdown, accordion, breadcrumb styles
- `web/src/App.tsx` — rewrite route tree
- `web/src/pages/FlagListPage.tsx` — context-aware heading and links
- `web/src/pages/FlagCreatePage.tsx` — context-aware back link
- `web/src/pages/FlagDetailPage.tsx` — context-aware back link
- `web/src/pages/DeploymentsPage.tsx` — context-aware heading
- `web/src/pages/ReleasesPage.tsx` — context-aware heading
- `web/src/pages/AnalyticsPage.tsx` — context-aware heading
- `web/src/pages/SDKsPage.tsx` — context-aware heading
- `web/src/pages/SettingsPage.tsx` — accept level prop, show level-appropriate tabs

### Deleted Files
- `web/src/components/Layout.tsx` — replaced by HierarchyLayout
- `web/src/pages/DashboardPage.tsx` — replaced by ProjectListPage

---

## Task 1: Types and Mock Data

**Files:**
- Modify: `web/src/types.ts`
- Create: `web/src/mocks/hierarchy.ts`

- [ ] **Step 1: Add Organization and Application interfaces to types.ts**

Add to the end of `web/src/types.ts`:

```typescript
export interface Organization {
  id: string;
  name: string;
  slug: string;
  created_at: string;
  updated_at: string;
}

export interface Application {
  id: string;
  project_id: string;
  name: string;
  slug: string;
  description?: string;
  repo_url?: string;
  created_at: string;
  updated_at: string;
}
```

- [ ] **Step 2: Create mock hierarchy data**

Create `web/src/mocks/hierarchy.ts`:

```typescript
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
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/types.ts web/src/mocks/hierarchy.ts
git commit -m "feat(web): add Organization/Application types and mock hierarchy data"
```

---

## Task 2: API Client Updates

**Files:**
- Modify: `web/src/api.ts`

- [ ] **Step 1: Add orgsApi and applicationsApi modules**

Add to the end of `web/src/api.ts`, before the closing of the file (after the `analyticsApi` export):

```typescript
// Organizations
export const orgsApi = {
  list: () => request<{ organizations: Organization[] }>('/orgs'),
  get: (slug: string) => request<Organization>(`/orgs/${slug}`),
};

// Applications
export const applicationsApi = {
  list: (projectId: string) =>
    request<{ applications: Application[] }>(`/projects/${projectId}/applications`),
  get: (id: string) => request<Application>(`/applications/${id}`),
};
```

Also add the import at the top of the file:

```typescript
import type { Flag, Deployment, Release, ApiKey, CreateFlagRequest, UpdateFlagRequest, TargetingRule, Organization, Application } from './types';
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/api.ts
git commit -m "feat(web): add orgsApi and applicationsApi endpoint modules"
```

---

## Task 3: Breadcrumb Component

**Files:**
- Create: `web/src/components/Breadcrumb.tsx`

- [ ] **Step 1: Create the Breadcrumb component**

Create `web/src/components/Breadcrumb.tsx`:

```tsx
import { Link, useParams, useLocation } from 'react-router-dom';
import { getOrgName, getProjectName, getAppName } from '@/mocks/hierarchy';

/** Map the last URL segment to a human-readable page name.
 *  For detail routes like /flags/:id, the last segment is an ID —
 *  detect this and use the second-to-last segment instead. */
function pageName(pathname: string): string {
  const segments = pathname.split('/').filter(Boolean);
  const names: Record<string, string> = {
    projects: 'Projects',
    flags: 'Flags',
    new: 'Create',
    deployments: 'Deployments',
    releases: 'Releases',
    analytics: 'Analytics',
    sdks: 'SDKs & Docs',
    settings: 'Settings',
    members: 'Members',
    'api-keys': 'API Keys',
  };

  const last = segments[segments.length - 1] ?? '';
  if (names[last]) return names[last];

  // If last segment is an ID (not in names map), check parent segment
  const parent = segments[segments.length - 2] ?? '';
  const detailNames: Record<string, string> = {
    flags: 'Flag Detail',
    deployments: 'Deployment Detail',
    releases: 'Release Detail',
  };
  return detailNames[parent] ?? last;
}

export default function Breadcrumb() {
  const { orgSlug, projectSlug, appSlug } = useParams();
  const location = useLocation();

  if (!orgSlug) return null;

  const segments: { label: string; to: string }[] = [];

  // Org
  segments.push({
    label: getOrgName(orgSlug),
    to: `/orgs/${orgSlug}/projects`,
  });

  // Project
  if (projectSlug) {
    segments.push({
      label: getProjectName(projectSlug),
      to: `/orgs/${orgSlug}/projects/${projectSlug}/flags`,
    });
  }

  // App
  if (appSlug) {
    segments.push({
      label: getAppName(appSlug),
      to: `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/deployments`,
    });
  }

  // Current page name (not a link)
  const page = pageName(location.pathname);

  return (
    <nav className="breadcrumb">
      {segments.map((seg, i) => (
        <span key={seg.to}>
          <Link to={seg.to} className="breadcrumb-link">
            {seg.label}
          </Link>
          <span className="breadcrumb-sep">/</span>
        </span>
      ))}
      <span className="breadcrumb-current">{page}</span>
    </nav>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No errors (component is not yet imported anywhere).

- [ ] **Step 3: Commit**

```bash
git add web/src/components/Breadcrumb.tsx
git commit -m "feat(web): add Breadcrumb component"
```

---

## Task 4: Sidebar Sub-Components

**Files:**
- Create: `web/src/components/OrgSwitcher.tsx`
- Create: `web/src/components/ProjectSwitcher.tsx`
- Create: `web/src/components/AppAccordion.tsx`

- [ ] **Step 1: Create OrgSwitcher**

Create `web/src/components/OrgSwitcher.tsx`:

```tsx
import { useState, useRef, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { getMockOrgs, getOrgName } from '@/mocks/hierarchy';

export default function OrgSwitcher() {
  const { orgSlug } = useParams();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const orgs = getMockOrgs();

  // Close dropdown on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, []);

  function handleSelect(slug: string) {
    setOpen(false);
    if (slug !== orgSlug) {
      localStorage.setItem('ds_last_org', slug);
      navigate(`/orgs/${slug}/projects`);
    }
  }

  return (
    <div className="switcher" ref={ref}>
      <button className="switcher-btn" onClick={() => setOpen(!open)}>
        <span className="switcher-label">{orgSlug ? getOrgName(orgSlug) : 'Select Org'}</span>
        <span className="switcher-arrow">{open ? '\u25B4' : '\u25BE'}</span>
      </button>
      {open && (
        <div className="switcher-dropdown">
          {orgs.map((org) => (
            <button
              key={org.id}
              className={`switcher-option${org.slug === orgSlug ? ' active' : ''}`}
              onClick={() => handleSelect(org.slug)}
            >
              {org.name}
            </button>
          ))}
          <div className="switcher-divider" />
          <button
            className="switcher-option switcher-option-action"
            onClick={() => { setOpen(false); navigate('/orgs/new'); }}
          >
            + Create Organization
          </button>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Create ProjectSwitcher**

Create `web/src/components/ProjectSwitcher.tsx`:

```tsx
import { useState, useRef, useEffect, useMemo } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { getMockProjects, getProjectName } from '@/mocks/hierarchy';

export default function ProjectSwitcher() {
  const { orgSlug, projectSlug } = useParams();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const projects = useMemo(() => orgSlug ? getMockProjects(orgSlug) : [], [orgSlug]);

  // Close dropdown on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, []);

  if (!orgSlug) return null;

  function handleSelect(slug: string) {
    setOpen(false);
    if (slug !== projectSlug) {
      navigate(`/orgs/${orgSlug}/projects/${slug}/flags`);
    }
  }

  return (
    <div className="switcher" ref={ref}>
      <button className="switcher-btn" onClick={() => setOpen(!open)}>
        <span className="switcher-label">{projectSlug ? getProjectName(projectSlug) : 'Select Project'}</span>
        <span className="switcher-arrow">{open ? '\u25B4' : '\u25BE'}</span>
      </button>
      {open && (
        <div className="switcher-dropdown">
          {projects.map((proj) => (
            <button
              key={proj.id}
              className={`switcher-option${proj.slug === projectSlug ? ' active' : ''}`}
              onClick={() => handleSelect(proj.slug)}
            >
              {proj.name}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Create AppAccordion**

Create `web/src/components/AppAccordion.tsx`:

```tsx
import { useState, useEffect } from 'react';
import { NavLink, useParams } from 'react-router-dom';
import { getMockApps } from '@/mocks/hierarchy';

export default function AppAccordion() {
  const { orgSlug, projectSlug, appSlug } = useParams();
  const [expandedApp, setExpandedApp] = useState<string | null>(null);

  // Auto-expand the app from the URL
  useEffect(() => {
    if (appSlug) setExpandedApp(appSlug);
  }, [appSlug]);

  if (!orgSlug || !projectSlug) return null;

  const apps = getMockApps(projectSlug);

  if (apps.length === 0) return null;

  const basePath = `/orgs/${orgSlug}/projects/${projectSlug}/apps`;

  const appNavItems = [
    { path: 'deployments', label: 'Deployments' },
    { path: 'releases', label: 'Releases' },
    { path: 'flags', label: 'Flags' },
    { path: 'settings', label: 'Settings' },
  ];

  function toggleApp(slug: string) {
    setExpandedApp(expandedApp === slug ? null : slug);
  }

  return (
    <div className="app-accordion">
      <div className="sidebar-section">Applications</div>
      {apps.map((app) => {
        const isExpanded = expandedApp === app.slug;
        return (
          <div key={app.id} className="app-accordion-item">
            <button
              className={`app-accordion-header${app.slug === appSlug ? ' active' : ''}`}
              onClick={() => toggleApp(app.slug)}
            >
              <span className="app-accordion-arrow">{isExpanded ? '\u25BE' : '\u25B8'}</span>
              <span>{app.name}</span>
            </button>
            {isExpanded && (
              <div className="app-accordion-body">
                {appNavItems.map((item) => (
                  <NavLink
                    key={item.path}
                    to={`${basePath}/${app.slug}/${item.path}`}
                    className={({ isActive }) => `nav-item nav-item-nested${isActive ? ' active' : ''}`}
                  >
                    {item.label}
                  </NavLink>
                ))}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
```

- [ ] **Step 4: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No errors (components not yet imported).

- [ ] **Step 5: Commit**

```bash
git add web/src/components/OrgSwitcher.tsx web/src/components/ProjectSwitcher.tsx web/src/components/AppAccordion.tsx
git commit -m "feat(web): add OrgSwitcher, ProjectSwitcher, and AppAccordion components"
```

---

## Task 5: Sidebar Rewrite and HierarchyLayout

**Files:**
- Modify: `web/src/components/Sidebar.tsx`
- Create: `web/src/components/HierarchyLayout.tsx`

- [ ] **Step 1: Rewrite Sidebar**

Replace the entire contents of `web/src/components/Sidebar.tsx`:

```tsx
import { NavLink, useParams, useNavigate } from 'react-router-dom';
import { useAuth } from '@/auth';
import OrgSwitcher from './OrgSwitcher';
import ProjectSwitcher from './ProjectSwitcher';
import AppAccordion from './AppAccordion';

export default function Sidebar() {
  const { user, logout } = useAuth();
  const { orgSlug, projectSlug } = useParams();
  const navigate = useNavigate();

  function handleLogout() {
    logout();
    navigate('/login');
  }

  return (
    <aside className="sidebar">
      <div className="sidebar-header">
        <div className="sidebar-logo">DS</div>
        <span className="sidebar-title">DeploySentry</span>
      </div>

      <div className="sidebar-switchers">
        <OrgSwitcher />
        {projectSlug && <ProjectSwitcher />}
      </div>

      <nav className="sidebar-nav">
        {/* App accordion — only when a project is selected */}
        {projectSlug && <AppAccordion />}

        {/* Project-level nav */}
        {projectSlug && orgSlug && (
          <>
            <div className="sidebar-section">Project</div>
            <NavLink
              to={`/orgs/${orgSlug}/projects/${projectSlug}/flags`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">#</span>
              Feature Flags
            </NavLink>
            <NavLink
              to={`/orgs/${orgSlug}/projects/${projectSlug}/analytics`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">%</span>
              Analytics
            </NavLink>
            <NavLink
              to={`/orgs/${orgSlug}/projects/${projectSlug}/sdks`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">{'{'}</span>
              SDKs & Docs
            </NavLink>
            <NavLink
              to={`/orgs/${orgSlug}/projects/${projectSlug}/settings`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">*</span>
              Settings
            </NavLink>
          </>
        )}

        {/* Org-level nav */}
        {orgSlug && (
          <>
            <div className="sidebar-section">Organization</div>
            <NavLink
              to={`/orgs/${orgSlug}/members`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">@</span>
              Members
            </NavLink>
            <NavLink
              to={`/orgs/${orgSlug}/api-keys`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">!</span>
              API Keys
            </NavLink>
            <NavLink
              to={`/orgs/${orgSlug}/settings`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">*</span>
              Settings
            </NavLink>
          </>
        )}
      </nav>

      <div className="sidebar-footer">
        {user && (
          <div className="sidebar-user">
            <span className="text-sm">{user.name || user.email}</span>
            <button className="btn-link text-xs text-muted" onClick={handleLogout}>
              Sign out
            </button>
          </div>
        )}
        <div className="nav-item text-xs text-muted">v1.0.0</div>
      </div>
    </aside>
  );
}
```

- [ ] **Step 2: Create HierarchyLayout**

Create `web/src/components/HierarchyLayout.tsx`:

```tsx
import { useEffect } from 'react';
import { Outlet, useParams } from 'react-router-dom';
import Sidebar from './Sidebar';
import Breadcrumb from './Breadcrumb';
import RealtimeManager from '@/services/realtime';

export default function HierarchyLayout() {
  const { orgSlug } = useParams();

  // Persist last-used org for DefaultRedirect
  useEffect(() => {
    if (orgSlug) {
      localStorage.setItem('ds_last_org', orgSlug);
    }
  }, [orgSlug]);

  // Initialize realtime (moved from App.tsx)
  useEffect(() => {
    const initializeRealtime = async () => {
      try {
        const realtimeManager = RealtimeManager.getInstance();
        await realtimeManager.initialize({
          baseUrl: window.location.origin,
          refreshInterval: 30000,
        });
      } catch (error) {
        console.warn('[HierarchyLayout] Failed to initialize real-time updates:', error);
      }
    };

    initializeRealtime();
    return () => { RealtimeManager.getInstance().dispose(); };
  }, []);

  return (
    <div className="app-layout">
      <Sidebar />
      <main className="main-content">
        <Breadcrumb />
        <Outlet />
      </main>
    </div>
  );
}
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: May show errors since App.tsx still imports old Layout — that's expected and fixed in Task 6 which must follow immediately. HierarchyLayout.tsx itself should compile cleanly.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/Sidebar.tsx web/src/components/HierarchyLayout.tsx
git commit -m "feat(web): rewrite Sidebar and add HierarchyLayout with breadcrumb"
```

---

## Task 6: Route Tree, Auth Refactor, Redirects, and New Pages

**Files:**
- Modify: `web/src/auth.tsx` — refactor auth guards to Outlet pattern
- Create: `web/src/components/DefaultRedirect.tsx`
- Create: `web/src/components/LegacyRedirect.tsx`
- Create: `web/src/pages/ProjectListPage.tsx`
- Create: `web/src/pages/CreateOrgPage.tsx`
- Modify: `web/src/App.tsx` — complete route tree rewrite
- Delete: `web/src/components/Layout.tsx`
- Delete: `web/src/pages/DashboardPage.tsx`

**Note:** The auth guard refactor (RequireAuth/RedirectIfAuth to Outlet pattern) is done in this task alongside the route tree rewrite to avoid a broken build between tasks. The old Sidebar.tsx is also replaced here since it's imported by the old Layout — both must change atomically.

- [ ] **Step 1: Refactor auth guards to Outlet pattern**

In `web/src/auth.tsx`:

Update the react import (remove `type ReactNode`):
```typescript
import { createContext, useContext, useState, useEffect, useCallback } from 'react';
```

Update the react-router-dom import:
```typescript
import { Navigate, useLocation, Outlet } from 'react-router-dom';
```

Replace `RequireAuth`:
```typescript
export function RequireAuth() {
  const { user, loading } = useAuth();
  const location = useLocation();

  if (loading) {
    return <div className="page-loading">Loading...</div>;
  }

  if (!user) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return <Outlet />;
}
```

Replace `RedirectIfAuth`:
```typescript
export function RedirectIfAuth() {
  const { user, loading } = useAuth();

  if (loading) {
    return <div className="page-loading">Loading...</div>;
  }

  if (user) {
    return <Navigate to="/" replace />;
  }

  return <Outlet />;
}
```

- [ ] **Step 2: Create DefaultRedirect**

Create `web/src/components/DefaultRedirect.tsx`:

```tsx
import { Navigate } from 'react-router-dom';
import { MOCK_ORGS } from '@/mocks/hierarchy';

export default function DefaultRedirect() {
  const lastOrg = localStorage.getItem('ds_last_org');

  if (lastOrg) {
    return <Navigate to={`/orgs/${lastOrg}/projects`} replace />;
  }

  // Fall back to first org or create new
  if (MOCK_ORGS.length > 0) {
    return <Navigate to={`/orgs/${MOCK_ORGS[0].slug}/projects`} replace />;
  }

  return <Navigate to="/orgs/new" replace />;
}
```

- [ ] **Step 3: Create LegacyRedirect**

Create `web/src/components/LegacyRedirect.tsx`:

```tsx
import { Navigate } from 'react-router-dom';
import { MOCK_ORGS, MOCK_PROJECTS } from '@/mocks/hierarchy';

interface LegacyRedirectProps {
  to: string;
}

export default function LegacyRedirect({ to }: LegacyRedirectProps) {
  const lastOrg = localStorage.getItem('ds_last_org') || MOCK_ORGS[0]?.slug || '';
  const lastProject = localStorage.getItem('ds_last_project') || '';

  if (!lastOrg) {
    return <Navigate to="/orgs/new" replace />;
  }

  // Settings goes to org level
  if (to === 'settings') {
    return <Navigate to={`/orgs/${lastOrg}/settings`} replace />;
  }

  // Project-level pages
  const projectSlug = lastProject || MOCK_PROJECTS.find((p) => {
    const org = MOCK_ORGS.find((o) => o.slug === lastOrg);
    return org && p.org_id === org.id;
  })?.slug || '';

  if (!projectSlug) {
    return <Navigate to={`/orgs/${lastOrg}/projects`} replace />;
  }

  // App-level pages (deployments, releases)
  if (to === 'deployments' || to === 'releases') {
    const lastApp = localStorage.getItem('ds_last_app') || '';
    if (lastApp) {
      return <Navigate to={`/orgs/${lastOrg}/projects/${projectSlug}/apps/${lastApp}/${to}`} replace />;
    }
    // No app context — go to project
    return <Navigate to={`/orgs/${lastOrg}/projects/${projectSlug}/flags`} replace />;
  }

  return <Navigate to={`/orgs/${lastOrg}/projects/${projectSlug}/${to}`} replace />;
}
```

- [ ] **Step 4: Create ProjectListPage**

Create `web/src/pages/ProjectListPage.tsx`:

```tsx
import { useParams, Link } from 'react-router-dom';
import { getMockProjects, getMockApps, getOrgName } from '@/mocks/hierarchy';

export default function ProjectListPage() {
  const { orgSlug } = useParams();
  if (!orgSlug) return null;

  const projects = getMockProjects(orgSlug);
  const orgName = getOrgName(orgSlug);

  return (
    <div>
      <div className="page-header-row">
        <h1 className="page-header">{orgName} — Projects</h1>
      </div>

      {projects.length === 0 ? (
        <div className="empty-state">
          <p>No projects yet. Create one to get started.</p>
        </div>
      ) : (
        <div className="project-card-grid">
          {projects.map((project) => {
            const apps = getMockApps(project.slug);
            return (
              <Link
                key={project.id}
                to={`/orgs/${orgSlug}/projects/${project.slug}/flags`}
                className="project-card"
              >
                <h3 className="project-card-name">{project.name}</h3>
                <span className="project-card-slug">{project.slug}</span>
                <div className="project-card-meta">
                  {apps.length} application{apps.length !== 1 ? 's' : ''}
                </div>
              </Link>
            );
          })}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 5: Create CreateOrgPage stub**

Create `web/src/pages/CreateOrgPage.tsx`:

```tsx
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';

export default function CreateOrgPage() {
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name || !slug) return;
    // Stub — in the future this calls orgsApi.create()
    localStorage.setItem('ds_last_org', slug);
    navigate(`/orgs/${slug}/projects`);
  }

  function handleNameChange(value: string) {
    setName(value);
    // Auto-generate slug from name
    setSlug(value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, ''));
  }

  return (
    <div className="page-center">
      <div className="form-card">
        <h1 className="page-header">Create Organization</h1>
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label">Organization Name</label>
            <input
              type="text"
              className="form-input"
              value={name}
              onChange={(e) => handleNameChange(e.target.value)}
              placeholder="Acme Corp"
              required
            />
          </div>
          <div className="form-group">
            <label className="form-label">Slug</label>
            <input
              type="text"
              className="form-input"
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
              placeholder="acme-corp"
              required
            />
          </div>
          <button type="submit" className="btn btn-primary" style={{ width: '100%' }}>
            Create Organization
          </button>
        </form>
      </div>
    </div>
  );
}
```

- [ ] **Step 6: Rewrite App.tsx route tree**

Replace the entire contents of `web/src/App.tsx`:

```tsx
import { Routes, Route } from 'react-router-dom';
import { AuthProvider, RequireAuth, RedirectIfAuth } from './auth';
import HierarchyLayout from './components/HierarchyLayout';
import DefaultRedirect from './components/DefaultRedirect';
import LegacyRedirect from './components/LegacyRedirect';
import LoginPage from './pages/LoginPage';
import RegisterPage from './pages/RegisterPage';
import ProjectListPage from './pages/ProjectListPage';
import CreateOrgPage from './pages/CreateOrgPage';
import FlagListPage from './pages/FlagListPage';
import FlagDetailPage from './pages/FlagDetailPage';
import FlagCreatePage from './pages/FlagCreatePage';
import DeploymentsPage from './pages/DeploymentsPage';
import ReleasesPage from './pages/ReleasesPage';
import AnalyticsPage from './pages/AnalyticsPage';
import SDKsPage from './pages/SDKsPage';
import SettingsPage from './pages/SettingsPage';

export default function App() {
  return (
    <AuthProvider>
      <Routes>
        {/* Public routes */}
        <Route element={<RedirectIfAuth />}>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
        </Route>

        {/* Authenticated routes */}
        <Route element={<RequireAuth />}>
          {/* Default redirect */}
          <Route path="/" element={<DefaultRedirect />} />

          {/* Create org (outside HierarchyLayout — no sidebar context yet) */}
          <Route path="/orgs/new" element={<CreateOrgPage />} />

          {/* Hierarchy layout */}
          <Route path="/orgs/:orgSlug" element={<HierarchyLayout />}>
            {/* Org-level */}
            <Route path="projects" element={<ProjectListPage />} />
            <Route path="members" element={<SettingsPage level="org" tab="members" />} />
            <Route path="api-keys" element={<SettingsPage level="org" tab="api-keys" />} />
            <Route path="settings" element={<SettingsPage level="org" />} />

            {/* Project-level */}
            <Route path="projects/:projectSlug">
              <Route path="flags" element={<FlagListPage />} />
              <Route path="flags/new" element={<FlagCreatePage />} />
              <Route path="flags/:id" element={<FlagDetailPage />} />
              <Route path="analytics" element={<AnalyticsPage />} />
              <Route path="sdks" element={<SDKsPage />} />
              <Route path="settings" element={<SettingsPage level="project" />} />

              {/* App-level */}
              <Route path="apps/:appSlug">
                <Route path="deployments" element={<DeploymentsPage />} />
                <Route path="deployments/:id" element={<DeploymentsPage />} />
                <Route path="releases" element={<ReleasesPage />} />
                <Route path="releases/:id" element={<ReleasesPage />} />
                <Route path="flags" element={<FlagListPage />} />
                <Route path="flags/new" element={<FlagCreatePage />} />
                <Route path="flags/:id" element={<FlagDetailPage />} />
                <Route path="settings" element={<SettingsPage level="app" />} />
              </Route>
            </Route>
          </Route>

          {/* Legacy redirects */}
          <Route path="/flags" element={<LegacyRedirect to="flags" />} />
          <Route path="/flags/new" element={<LegacyRedirect to="flags/new" />} />
          <Route path="/deployments" element={<LegacyRedirect to="deployments" />} />
          <Route path="/releases" element={<LegacyRedirect to="releases" />} />
          <Route path="/analytics" element={<LegacyRedirect to="analytics" />} />
          <Route path="/sdks" element={<LegacyRedirect to="sdks" />} />
          <Route path="/settings" element={<LegacyRedirect to="settings" />} />
        </Route>
      </Routes>
    </AuthProvider>
  );
}
```

- [ ] **Step 7: Delete Layout.tsx and DashboardPage.tsx**

```bash
rm web/src/components/Layout.tsx web/src/pages/DashboardPage.tsx
```

- [ ] **Step 8: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: Errors in SettingsPage.tsx (doesn't accept `level`/`tab` props yet). That's fixed in Task 8. All other files should compile cleanly.

- [ ] **Step 9: Commit**

```bash
git add web/src/auth.tsx web/src/components/DefaultRedirect.tsx web/src/components/LegacyRedirect.tsx web/src/pages/ProjectListPage.tsx web/src/pages/CreateOrgPage.tsx web/src/App.tsx
git rm web/src/components/Layout.tsx web/src/pages/DashboardPage.tsx
git commit -m "feat(web): rewrite route tree and auth guards for org-scoped hierarchy"
```

---

## Task 7: CSS for Sidebar Dropdowns, Accordion, and Breadcrumb

**Files:**
- Modify: `web/src/styles/globals.css`

- [ ] **Step 1: Add new CSS to globals.css**

Append to the end of `web/src/styles/globals.css`:

```css
/* ------------------------------------------------------------------ */
/* Breadcrumb                                                          */
/* ------------------------------------------------------------------ */
.breadcrumb {
  font-size: 12px;
  color: var(--color-text-muted);
  margin-bottom: 8px;
}

.breadcrumb-link {
  color: var(--color-text-muted);
  transition: color 0.15s;
}

.breadcrumb-link:hover {
  color: var(--color-text-secondary);
}

.breadcrumb-sep {
  margin: 0 6px;
  color: var(--color-text-muted);
}

.breadcrumb-current {
  color: var(--color-text-secondary);
}

/* ------------------------------------------------------------------ */
/* Switcher dropdowns (Org / Project)                                  */
/* ------------------------------------------------------------------ */
.sidebar-switchers {
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  border-bottom: 1px solid var(--color-border);
}

.switcher {
  position: relative;
}

.switcher-btn {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
  background: var(--color-bg-elevated);
  color: var(--color-text);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: border-color 0.15s;
}

.switcher-btn:hover {
  border-color: var(--color-border-light);
}

.switcher-arrow {
  font-size: 10px;
  color: var(--color-text-muted);
}

.switcher-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 4px;
  z-index: 200;
  box-shadow: var(--shadow-md);
}

.switcher-option {
  display: block;
  width: 100%;
  padding: 6px 10px;
  border: none;
  background: none;
  color: var(--color-text-secondary);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  border-radius: var(--radius-sm);
  transition: all 0.1s;
}

.switcher-option:hover {
  background: var(--color-bg-hover);
  color: var(--color-text);
}

.switcher-option.active {
  color: var(--color-primary);
  font-weight: 500;
}

.switcher-option-action {
  color: var(--color-primary);
}

.switcher-divider {
  height: 1px;
  background: var(--color-border);
  margin: 4px 0;
}

/* ------------------------------------------------------------------ */
/* App accordion                                                       */
/* ------------------------------------------------------------------ */
.app-accordion {
  padding: 0 0 4px 0;
}

.app-accordion-item {
  margin: 0;
}

.app-accordion-header {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 6px 12px;
  border: none;
  background: none;
  color: var(--color-text-secondary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  border-radius: var(--radius-md);
  transition: all 0.15s;
}

.app-accordion-header:hover {
  background: var(--color-bg-hover);
  color: var(--color-text);
}

.app-accordion-header.active {
  color: var(--color-text);
}

.app-accordion-arrow {
  font-size: 10px;
  color: var(--color-text-muted);
  width: 12px;
}

.app-accordion-body {
  padding-left: 18px;
  border-left: 1px solid var(--color-border);
  margin-left: 17px;
  margin-bottom: 4px;
}

.nav-item-nested {
  padding: 4px 10px;
  font-size: 13px;
}

/* ------------------------------------------------------------------ */
/* Project card grid                                                   */
/* ------------------------------------------------------------------ */
.project-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
  margin-top: 16px;
}

.project-card {
  display: block;
  padding: 20px;
  background: var(--color-bg-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  color: var(--color-text);
  transition: all 0.15s;
}

.project-card:hover {
  border-color: var(--color-primary);
  background: var(--color-bg-elevated);
  color: var(--color-text);
}

.project-card-name {
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 4px;
}

.project-card-slug {
  font-size: 12px;
  color: var(--color-text-muted);
  font-family: var(--font-mono);
}

.project-card-meta {
  margin-top: 12px;
  font-size: 13px;
  color: var(--color-text-secondary);
}

/* ------------------------------------------------------------------ */
/* Page center (for Create Org and similar standalone forms)            */
/* ------------------------------------------------------------------ */
.page-center {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 60vh;
}

.form-card {
  width: 100%;
  max-width: 420px;
  padding: 32px;
  background: var(--color-bg-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
}

/* ------------------------------------------------------------------ */
/* Empty state                                                         */
/* ------------------------------------------------------------------ */
.empty-state {
  text-align: center;
  padding: 48px 16px;
  color: var(--color-text-secondary);
}
```

- [ ] **Step 2: Verify the dev server renders**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx vite build`
Expected: Build succeeds (may have TS errors from SettingsPage props not yet updated — that's Task 8).

- [ ] **Step 3: Commit**

```bash
git add web/src/styles/globals.css
git commit -m "feat(web): add CSS for sidebar dropdowns, accordion, breadcrumb, and project cards"
```

---

## Task 8: Update SettingsPage for Level Props

**Files:**
- Modify: `web/src/pages/SettingsPage.tsx`

- [ ] **Step 1: Update SettingsPage to accept level and tab props**

Read the full current `web/src/pages/SettingsPage.tsx`, then modify the component signature and tab logic.

At the top of the file, update the component signature:

```tsx
interface SettingsPageProps {
  level?: 'org' | 'project' | 'app';
  tab?: string;
}

export default function SettingsPage({ level = 'org', tab }: SettingsPageProps) {
```

Update the `SettingsTab` type to include 'members':

```tsx
type SettingsTab = 'api-keys' | 'webhooks' | 'notifications' | 'project' | 'members';
```

Update the default tab logic based on level:

```tsx
function defaultTab(level: string, tab?: string): SettingsTab {
  if (tab === 'members') return 'members';
  if (tab === 'api-keys') return 'api-keys';
  switch (level) {
    case 'org': return 'webhooks';
    case 'project': return 'project';
    case 'app': return 'project';
    default: return 'webhooks';
  }
}
```

Replace the `useState` for `activeTab`:

```tsx
const [activeTab, setActiveTab] = useState<SettingsTab>(defaultTab(level, tab));
```

Update the tabs array to be level-dependent:

```tsx
function getTabsForLevel(level: string): { key: SettingsTab; label: string }[] {
  switch (level) {
    case 'org':
      return [
        { key: 'members', label: 'Members' },
        { key: 'api-keys', label: 'API Keys' },
        { key: 'webhooks', label: 'Webhooks' },
        { key: 'notifications', label: 'Notifications' },
      ];
    case 'project':
      return [
        { key: 'project', label: 'Project Settings' },
      ];
    case 'app':
      return [
        { key: 'project', label: 'App Settings' },
      ];
    default:
      return [];
  }
}
```

In the JSX, replace the hardcoded tabs with:

```tsx
const tabs = getTabsForLevel(level);
```

And render them:

```tsx
<div className="tabs">
  {tabs.map((t) => (
    <button
      key={t.key}
      className={`tab${activeTab === t.key ? ' active' : ''}`}
      onClick={() => setActiveTab(t.key)}
    >
      {t.label}
    </button>
  ))}
</div>
```

Add a simple Members tab content (placeholder):

```tsx
{activeTab === 'members' && (
  <div className="settings-section">
    <h2>Members</h2>
    <p className="text-muted">Organization member management coming in Phase 3.</p>
  </div>
)}
```

Update the page heading to reflect the level:

```tsx
const headingMap = { org: 'Organization Settings', project: 'Project Settings', app: 'Application Settings' };
// ...
<h1 className="page-header">{headingMap[level]}</h1>
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/SettingsPage.tsx
git commit -m "feat(web): update SettingsPage to support org/project/app levels"
```

---

## Task 9: Update Existing Pages for Context Awareness

**Files:**
- Modify: `web/src/pages/FlagListPage.tsx`
- Modify: `web/src/pages/FlagCreatePage.tsx`
- Modify: `web/src/pages/FlagDetailPage.tsx`
- Modify: `web/src/pages/DeploymentsPage.tsx`
- Modify: `web/src/pages/ReleasesPage.tsx`
- Modify: `web/src/pages/AnalyticsPage.tsx`
- Modify: `web/src/pages/SDKsPage.tsx`

- [ ] **Step 1: Update FlagListPage**

In `web/src/pages/FlagListPage.tsx`:

Add imports:
```tsx
import { useParams } from 'react-router-dom';
import { getProjectName, getAppName } from '@/mocks/hierarchy';
```

Inside the component function, add:
```tsx
const { orgSlug, projectSlug, appSlug } = useParams();
const contextName = appSlug ? getAppName(appSlug) : projectSlug ? getProjectName(projectSlug) : '';
const heading = appSlug ? `${contextName} — Flags` : 'Feature Flags';
```

Replace the heading `<h1 className="page-header">Feature Flags</h1>` with:
```tsx
<h1 className="page-header">{heading}</h1>
```

Update the "Create Flag" link from `to="/flags/new"` to use the current context:
```tsx
const createPath = appSlug
  ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/flags/new`
  : `/orgs/${orgSlug}/projects/${projectSlug}/flags/new`;
```
Then: `<Link to={createPath} className="btn btn-primary">`

Update the flag detail link from `to={`/flags/${flag.id}`}` to:
```tsx
const flagDetailPath = (flagId: string) => appSlug
  ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/flags/${flagId}`
  : `/orgs/${orgSlug}/projects/${projectSlug}/flags/${flagId}`;
```

- [ ] **Step 2: Update FlagCreatePage**

In `web/src/pages/FlagCreatePage.tsx`:

Add imports:
```tsx
import { useParams } from 'react-router-dom';
```

Inside the component, add:
```tsx
const { orgSlug, projectSlug, appSlug } = useParams();
const backPath = appSlug
  ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/flags`
  : `/orgs/${orgSlug}/projects/${projectSlug}/flags`;
```

Replace any `navigate('/flags')` or Link `to="/flags"` with `backPath`.

If at app level, pre-select category as 'release':
```tsx
const [category, setCategory] = useState<FlagCategory>(appSlug ? 'release' : 'feature');
```

- [ ] **Step 3: Update FlagDetailPage**

In `web/src/pages/FlagDetailPage.tsx`:

Add imports:
```tsx
import { useParams } from 'react-router-dom';
```

Inside the component, add:
```tsx
const { orgSlug, projectSlug, appSlug } = useParams();
const backPath = appSlug
  ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/flags`
  : `/orgs/${orgSlug}/projects/${projectSlug}/flags`;
```

Replace any back navigation to `/flags` with `backPath`.

- [ ] **Step 4: Update DeploymentsPage**

In `web/src/pages/DeploymentsPage.tsx`:

Add imports:
```tsx
import { useParams } from 'react-router-dom';
import { getAppName } from '@/mocks/hierarchy';
```

Inside the component:
```tsx
const { appSlug } = useParams();
const appName = appSlug ? getAppName(appSlug) : '';
```

Replace the heading (search for the `<h1>` tag, likely says "Deployments") with:
```tsx
<h1 className="page-header">{appName ? `${appName} — Deployments` : 'Deployments'}</h1>
```

- [ ] **Step 5: Update ReleasesPage**

In `web/src/pages/ReleasesPage.tsx`:

Same pattern — add `useParams` and `getAppName`:
```tsx
const { appSlug } = useParams();
const appName = appSlug ? getAppName(appSlug) : '';
```

Update heading to `{appName ? `${appName} — Releases` : 'Releases'}`.

- [ ] **Step 6: Update AnalyticsPage**

In `web/src/pages/AnalyticsPage.tsx`:

Add:
```tsx
import { useParams } from 'react-router-dom';
import { getProjectName } from '@/mocks/hierarchy';

const { projectSlug } = useParams();
const projectName = projectSlug ? getProjectName(projectSlug) : '';
```

Update heading to `{projectName ? `${projectName} — Analytics` : 'Analytics'}`.

- [ ] **Step 7: Update SDKsPage**

In `web/src/pages/SDKsPage.tsx`:

Same pattern:
```tsx
import { useParams } from 'react-router-dom';
import { getProjectName } from '@/mocks/hierarchy';

const { projectSlug } = useParams();
const projectName = projectSlug ? getProjectName(projectSlug) : '';
```

Update heading to `{projectName ? `${projectName} — SDKs & Docs` : 'SDKs & Docs'}`.

- [ ] **Step 8: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 9: Commit**

```bash
git add web/src/pages/FlagListPage.tsx web/src/pages/FlagCreatePage.tsx web/src/pages/FlagDetailPage.tsx web/src/pages/DeploymentsPage.tsx web/src/pages/ReleasesPage.tsx web/src/pages/AnalyticsPage.tsx web/src/pages/SDKsPage.tsx
git commit -m "feat(web): make all pages context-aware with hierarchy params"
```

---

## Task 10: Full Build and Smoke Test

**Files:** None (verification only)

- [ ] **Step 1: Full TypeScript check**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: Zero errors.

- [ ] **Step 2: Vite build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx vite build`
Expected: Build succeeds with no errors.

- [ ] **Step 3: Start the dev server and verify routes**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx vite --port 3001 &`

Verify these URLs load correctly in a browser:
- `http://localhost:3001/` → should redirect to `/orgs/acme-corp/projects`
- `http://localhost:3001/orgs/acme-corp/projects` → shows project cards
- `http://localhost:3001/orgs/acme-corp/projects/platform/flags` → shows flags with "Platform — Feature Flags" heading
- `http://localhost:3001/orgs/acme-corp/projects/platform/apps/api-server/deployments` → shows deployments with "API Server — Deployments" heading
- `http://localhost:3001/orgs/acme-corp/settings` → shows org settings
- `http://localhost:3001/flags` → redirects to new hierarchy URL

- [ ] **Step 4: Verify sidebar renders correctly**

At `http://localhost:3001/orgs/acme-corp/projects/platform/apps/api-server/deployments`:
- Org switcher shows "Acme Corp"
- Project switcher shows "Platform"
- App accordion shows: API Server, Web App, Worker
- "API Server" is expanded with Deployments highlighted
- Project section shows: Feature Flags, Analytics, SDKs & Docs, Settings
- Org section shows: Members, API Keys, Settings
- Breadcrumb shows: `Acme Corp / Platform / API Server / Deployments`

- [ ] **Step 5: Stop dev server**

```bash
kill %1 2>/dev/null || true
```

---

## Task 11: Update Documentation

**Files:**
- Modify: `docs/Current_Initiatives.md`

- [ ] **Step 1: Update Current_Initiatives.md**

Add the Phase 1 plan to the initiatives table:

```markdown
| Web UI Phase 1 — Navigation Overhaul | Implementation | [Link](./superpowers/plans/2026-03-28-web-ui-phase1-navigation.md) |
```

- [ ] **Step 2: Commit all remaining changes**

```bash
git add docs/Current_Initiatives.md
git commit -m "docs: add Web UI Phase 1 navigation overhaul to current initiatives"
```
