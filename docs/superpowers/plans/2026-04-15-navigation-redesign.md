# Navigation Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current sidebar-driven navigation with a tab-based drill-down hierarchy: Org → Project (tabs) → Application (tabs), with back buttons and a simplified org-only sidebar.

**Architecture:** Two new wrapper components (`ProjectPage`, `AppPage`) render title, back button, and tab bar, using `<Outlet />` for tab content. The sidebar is stripped to org-level items only. Routes are restructured so project and app pages are nested under their wrapper components. Existing page components (FlagListPage, DeploymentsPage, etc.) are reused as-is inside the tab outlets.

**Tech Stack:** React, React Router, TypeScript, CSS (existing `.tabs`/`.tab` classes)

**Spec:** `docs/superpowers/specs/2026-04-15-navigation-redesign-design.md`

---

### Task 1: Create `ProjectPage` wrapper component

**Files:**
- Create: `web/src/pages/ProjectPage.tsx`

- [ ] **Step 1: Create the ProjectPage component**

This component renders the project title, back button, tab bar, and an outlet for tab content.

```tsx
import { NavLink, Outlet, useParams, Navigate } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { entitiesApi } from '@/api';
import type { Project } from '@/types';

export default function ProjectPage() {
  const { orgSlug, projectSlug } = useParams();
  const [project, setProject] = useState<Project | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!orgSlug || !projectSlug) return;
    entitiesApi
      .getProject(orgSlug, projectSlug)
      .then(setProject)
      .catch(() => setProject(null))
      .finally(() => setLoading(false));
  }, [orgSlug, projectSlug]);

  if (!orgSlug || !projectSlug) return null;
  if (loading) return <div className="page-loading">Loading project...</div>;
  if (!project) return <div className="page-error">Project not found</div>;

  const base = `/orgs/${orgSlug}/projects/${projectSlug}`;

  return (
    <div>
      <div className="content-header">
        <h1 className="content-title">{project.name}</h1>
      </div>
      <NavLink to={`/orgs/${orgSlug}/projects`} className="back-link">
        ← Back to Projects
      </NavLink>
      <div className="tabs">
        <NavLink to={`${base}/apps`} className={({ isActive }) => `tab${isActive ? ' active' : ''}`} end>
          Applications
        </NavLink>
        <NavLink to={`${base}/flags`} className={({ isActive }) => `tab${isActive ? ' active' : ''}`}>
          Flags
        </NavLink>
        <NavLink to={`${base}/analytics`} className={({ isActive }) => `tab${isActive ? ' active' : ''}`}>
          Analytics
        </NavLink>
        <NavLink to={`${base}/settings`} className={({ isActive }) => `tab${isActive ? ' active' : ''}`}>
          Settings
        </NavLink>
      </div>
      <Outlet />
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No errors (component isn't wired into routes yet)

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/ProjectPage.tsx
git commit -m "feat: create ProjectPage wrapper with tabs and back button"
```

---

### Task 2: Create `AppPage` wrapper component

**Files:**
- Create: `web/src/pages/AppPage.tsx`

- [ ] **Step 1: Create the AppPage component**

Same pattern as ProjectPage but for application level.

```tsx
import { NavLink, Outlet, useParams } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { entitiesApi } from '@/api';
import type { Application } from '@/types';

export default function AppPage() {
  const { orgSlug, projectSlug, appSlug } = useParams();
  const [app, setApp] = useState<Application | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!orgSlug || !projectSlug || !appSlug) return;
    entitiesApi
      .getApp(orgSlug, projectSlug, appSlug)
      .then(setApp)
      .catch(() => setApp(null))
      .finally(() => setLoading(false));
  }, [orgSlug, projectSlug, appSlug]);

  if (!orgSlug || !projectSlug || !appSlug) return null;
  if (loading) return <div className="page-loading">Loading application...</div>;
  if (!app) return <div className="page-error">Application not found</div>;

  const base = `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}`;

  return (
    <div>
      <div className="content-header">
        <h1 className="content-title">{app.name}</h1>
      </div>
      <NavLink
        to={`/orgs/${orgSlug}/projects/${projectSlug}/apps`}
        className="back-link"
      >
        ← Back to {projectSlug}
      </NavLink>
      <div className="tabs">
        <NavLink to={`${base}/flags`} className={({ isActive }) => `tab${isActive ? ' active' : ''}`}>
          Flags
        </NavLink>
        <NavLink to={`${base}/deployments`} className={({ isActive }) => `tab${isActive ? ' active' : ''}`}>
          Deployments
        </NavLink>
        <NavLink to={`${base}/releases`} className={({ isActive }) => `tab${isActive ? ' active' : ''}`}>
          Releases
        </NavLink>
        <NavLink to={`${base}/settings`} className={({ isActive }) => `tab${isActive ? ' active' : ''}`}>
          Settings
        </NavLink>
      </div>
      <Outlet />
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/AppPage.tsx
git commit -m "feat: create AppPage wrapper with tabs and back button"
```

---

### Task 3: Create `ProjectAppsTab` — the Applications tab content

**Files:**
- Create: `web/src/pages/ProjectAppsTab.tsx`

The current `ApplicationsListPage.tsx` can't be reused directly because it renders as a full page. We need a simpler tab content component that lists app cards. This replaces `ApplicationsListPage` as the default tab in ProjectPage.

- [ ] **Step 1: Create ProjectAppsTab**

```tsx
import { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useApps } from '@/hooks/useEntities';
import { entitiesApi } from '@/api';

export default function ProjectAppsTab() {
  const { orgSlug, projectSlug } = useParams();
  const { apps, loading, error, refresh } = useApps(orgSlug!, projectSlug!, true);
  const [restoring, setRestoring] = useState<string | null>(null);

  if (!orgSlug || !projectSlug) return null;
  if (loading) return <div className="page-loading">Loading applications...</div>;
  if (error) return <div className="page-error">Error: {error}</div>;

  const handleRestore = async (appSlug: string) => {
    setRestoring(appSlug);
    try {
      await entitiesApi.restoreApp(orgSlug, projectSlug, appSlug);
      refresh();
    } catch (err) {
      console.error('Failed to restore app:', err);
    } finally {
      setRestoring(null);
    }
  };

  const formatHardDeleteDate = (deletedAt: string) => {
    const date = new Date(deletedAt);
    date.setDate(date.getDate() + 7);
    return date.toLocaleDateString();
  };

  const base = `/orgs/${orgSlug}/projects/${projectSlug}`;

  return (
    <div>
      <div className="page-header-row" style={{ marginBottom: 16 }}>
        <h2 style={{ margin: 0 }}>Applications</h2>
        <Link to={`${base}/apps/new`} className="btn btn-primary">
          Add Application
        </Link>
      </div>
      {apps.length === 0 ? (
        <div className="empty-state">
          <p>No applications yet.</p>
          <Link to={`${base}/apps/new`} className="btn btn-primary">
            Create Your First Application
          </Link>
        </div>
      ) : (
        <div className="project-card-grid">
          {apps.map((app) => {
            const isDeleted = !!app.deleted_at;
            return (
              <div
                key={app.id}
                className="project-card"
                style={isDeleted ? { opacity: 0.5 } : undefined}
              >
                <Link
                  to={`${base}/apps/${app.slug}/flags`}
                  style={{ textDecoration: 'none', color: 'inherit' }}
                >
                  <h3 className="project-card-name" style={{ margin: 0 }}>
                    {app.name}
                    {isDeleted && (
                      <span className="badge badge-disabled" style={{ marginLeft: 8, fontSize: 11 }}>
                        Deleted
                      </span>
                    )}
                  </h3>
                  <span className="project-card-slug">{app.slug}</span>
                </Link>
                {isDeleted && app.deleted_at && (
                  <div style={{ marginTop: 8 }}>
                    <p className="text-muted text-sm" style={{ margin: '4px 0' }}>
                      Hard delete available on {formatHardDeleteDate(app.deleted_at)}
                    </p>
                    <button
                      className="btn btn-sm"
                      onClick={() => handleRestore(app.slug)}
                      disabled={restoring === app.slug}
                    >
                      {restoring === app.slug ? 'Restoring...' : 'Restore'}
                    </button>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/ProjectAppsTab.tsx
git commit -m "feat: create ProjectAppsTab for applications listing within project tabs"
```

---

### Task 4: Add CSS for back-link and content-header

**Files:**
- Modify: `web/src/styles/globals.css`

- [ ] **Step 1: Add styles for the new layout elements**

Append to `web/src/styles/globals.css`:

```css
/* Content header for drill-down pages */
.content-header {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 4px;
}
.content-title {
  font-size: 1.5rem;
  font-weight: 700;
  margin: 0;
}
.back-link {
  display: inline-block;
  color: var(--color-primary);
  font-size: 0.8125rem;
  text-decoration: none;
  margin-bottom: 16px;
}
.back-link:hover {
  text-decoration: underline;
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/styles/globals.css
git commit -m "feat: add CSS for content-header, content-title, and back-link"
```

---

### Task 5: Restructure routes in `App.tsx`

**Files:**
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Update imports**

Replace the App.tsx imports and route structure. Add the new components and remove `ApplicationsListPage`:

```tsx
import { Routes, Route, Navigate } from 'react-router-dom';
import { lazy, Suspense } from 'react';
import { AuthProvider, RequireAuth, RedirectIfAuth } from './auth';
import HierarchyLayout from './components/HierarchyLayout';
import DefaultRedirect from './components/DefaultRedirect';
import LegacyRedirect from './components/LegacyRedirect';
import LoginPage from './pages/LoginPage';
import RegisterPage from './pages/RegisterPage';
import ProjectListPage from './pages/ProjectListPage';
import CreateOrgPage from './pages/CreateOrgPage';
import ProjectPage from './pages/ProjectPage';
import ProjectAppsTab from './pages/ProjectAppsTab';
import AppPage from './pages/AppPage';
import FlagListPage from './pages/FlagListPage';
import FlagDetailPage from './pages/FlagDetailPage';
import FlagCreatePage from './pages/FlagCreatePage';
import DeploymentsPage from './pages/DeploymentsPage';
import ReleasesPage from './pages/ReleasesPage';
import DeploymentDetailPage from './pages/DeploymentDetailPage';
import ReleaseDetailPage from './pages/ReleaseDetailPage';
import AnalyticsPage from './pages/AnalyticsPage';
import SettingsPage from './pages/SettingsPage';
import MembersPage from './pages/MembersPage';
import APIKeysPage from './pages/APIKeysPage';
import CreateAppPage from './pages/CreateAppPage';
import CreateProjectPage from './pages/CreateProjectPage';
const LandingPage = lazy(() => import('./pages/LandingPage'));
const DocsPage = lazy(() => import('./pages/DocsPage'));
```

- [ ] **Step 2: Replace the route structure**

```tsx
export default function App() {
  return (
    <AuthProvider>
      <Suspense fallback={<div className="page-loading">Loading...</div>}>
      <Routes>
        {/* Public routes */}
        <Route path="/" element={<LandingPage />} />
        <Route element={<RedirectIfAuth />}>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
        </Route>

        {/* Authenticated routes */}
        <Route element={<RequireAuth />}>
          <Route path="/portal" element={<DefaultRedirect />} />
          <Route path="/docs" element={<DocsPage />} />
          <Route path="/docs/:slug" element={<DocsPage />} />

          {/* Create org (outside HierarchyLayout) */}
          <Route path="/orgs/new" element={<CreateOrgPage />} />

          {/* Hierarchy layout */}
          <Route path="/orgs/:orgSlug" element={<HierarchyLayout />}>
            {/* Org-level */}
            <Route path="projects" element={<ProjectListPage />} />
            <Route path="projects/new" element={<CreateProjectPage />} />
            <Route path="members" element={<MembersPage />} />
            <Route path="api-keys" element={<APIKeysPage />} />
            <Route path="settings" element={<SettingsPage level="org" />} />

            {/* Project-level — wrapped by ProjectPage (tabs) */}
            <Route path="projects/:projectSlug" element={<ProjectPage />}>
              <Route index element={<Navigate to="apps" replace />} />
              <Route path="apps" element={<ProjectAppsTab />} />
              <Route path="apps/new" element={<CreateAppPage />} />
              <Route path="flags" element={<FlagListPage />} />
              <Route path="flags/new" element={<FlagCreatePage />} />
              <Route path="flags/:id" element={<FlagDetailPage />} />
              <Route path="analytics" element={<AnalyticsPage />} />
              <Route path="settings" element={<SettingsPage level="project" />} />

              {/* App-level — wrapped by AppPage (tabs) */}
              <Route path="apps/:appSlug" element={<AppPage />}>
                <Route index element={<Navigate to="flags" replace />} />
                <Route path="flags" element={<FlagListPage />} />
                <Route path="flags/new" element={<FlagCreatePage />} />
                <Route path="flags/:id" element={<FlagDetailPage />} />
                <Route path="deployments" element={<DeploymentsPage />} />
                <Route path="deployments/:id" element={<DeploymentDetailPage />} />
                <Route path="releases" element={<ReleasesPage />} />
                <Route path="releases/:id" element={<ReleaseDetailPage />} />
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
      </Suspense>
    </AuthProvider>
  );
}
```

Key changes:
- `projects/:projectSlug` now wraps all children with `<ProjectPage>` (renders tabs)
- `apps/:appSlug` wraps all children with `<AppPage>` (renders tabs)
- `index` routes redirect to default tabs (`apps` for project, `flags` for app)
- `ApplicationsListPage` removed — replaced by `ProjectAppsTab`
- `SDKsPage` route removed (moves to header link)

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`

- [ ] **Step 4: Commit**

```bash
git add web/src/App.tsx
git commit -m "feat: restructure routes with ProjectPage and AppPage tab wrappers"
```

---

### Task 6: Simplify Sidebar to org-only

**Files:**
- Modify: `web/src/components/Sidebar.tsx`

- [ ] **Step 1: Replace Sidebar content**

Remove ProjectSwitcher, AppAccordion, all project-level and app-level nav items. Keep org switcher and org-level items.

```tsx
import { NavLink, useParams } from 'react-router-dom';
import OrgSwitcher from './OrgSwitcher';

export default function Sidebar() {
  const { orgSlug } = useParams();

  return (
    <aside className="sidebar">
      <div className="sidebar-switchers">
        <OrgSwitcher />
      </div>

      <nav className="sidebar-nav">
        {orgSlug && (
          <>
            <NavLink
              to={`/orgs/${orgSlug}/projects`}
              className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
            >
              <span className="nav-icon">□</span>
              Projects
            </NavLink>
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
        <div className="sidebar-section">Help</div>
        <NavLink
          to="/docs"
          className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
        >
          <span className="nav-icon">?</span>
          Documentation
        </NavLink>
      </nav>
    </aside>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/src/components/Sidebar.tsx
git commit -m "feat: simplify sidebar to org-level items only"
```

---

### Task 7: Update HierarchyLayout — remove Breadcrumb

**Files:**
- Modify: `web/src/components/HierarchyLayout.tsx`

- [ ] **Step 1: Remove Breadcrumb import and usage**

```tsx
import { useEffect } from 'react';
import { Outlet, useParams } from 'react-router-dom';
import Sidebar from './Sidebar';
import SiteHeader from './SiteHeader';
import RealtimeManager from '@/services/realtime';

export default function HierarchyLayout() {
  const { orgSlug, projectSlug, appSlug } = useParams();

  useEffect(() => {
    if (orgSlug) localStorage.setItem('ds_last_org', orgSlug);
    if (projectSlug) localStorage.setItem('ds_last_project', projectSlug);
    if (appSlug) localStorage.setItem('ds_last_app', appSlug);
  }, [orgSlug, projectSlug, appSlug]);

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
    return () => {
      RealtimeManager.getInstance().dispose();
    };
  }, []);

  return (
    <div className="app-shell">
      <SiteHeader variant="app" />
      <div className="app-layout">
        <Sidebar />
        <main className="main-content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/HierarchyLayout.tsx
git commit -m "feat: remove breadcrumb from HierarchyLayout"
```

---

### Task 8: Add SDKs & Docs link to SiteHeader

**Files:**
- Modify: `web/src/components/SiteHeader.tsx`

- [ ] **Step 1: Add Docs and SDKs links to the app variant of the header**

Update the component so that when `variant === 'app'` and a user is logged in, it shows navigation links:

```tsx
import { Link } from 'react-router-dom';
import { useAuth } from '@/authHooks';
import UserMenu from './UserMenu';

type SiteHeaderProps = {
  variant: 'landing' | 'app';
};

export default function SiteHeader({ variant }: SiteHeaderProps) {
  const { user } = useAuth();
  return (
    <header className="site-header">
      <Link to="/" className="site-header-brand" aria-label="DeploySentry home">
        <span className="site-header-logo">DS</span>
        <span className="site-header-wordmark">DeploySentry</span>
      </Link>

      {variant === 'landing' && (
        <nav className="site-header-nav">
          <a href="#pillars" className="site-header-link">Product</a>
          <Link to="/docs" className="site-header-link">Docs</Link>
          <Link to="/docs/sdks" className="site-header-link">SDKs</Link>
        </nav>
      )}

      {variant === 'app' && user && (
        <nav className="site-header-nav">
          <Link to="/docs" className="site-header-link">Docs</Link>
          <Link to="/docs/sdks" className="site-header-link">SDKs</Link>
        </nav>
      )}

      <div className="site-header-right">
        {!user && variant === 'landing' && (
          <>
            <Link to="/login" className="site-header-link">Log in</Link>
            <Link to="/register" className="btn-primary site-header-cta">Sign up</Link>
          </>
        )}
        {user && variant === 'landing' && (
          <Link to="/portal" className="btn-primary site-header-cta">Portal</Link>
        )}
        {user && <UserMenu />}
      </div>
    </header>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/SiteHeader.tsx
git commit -m "feat: add Docs and SDKs links to app header"
```

---

### Task 9: Update ProjectListPage — cards navigate to project

**Files:**
- Modify: `web/src/pages/ProjectListPage.tsx`

- [ ] **Step 1: Update card links**

Change the project card links from `/projects/{slug}/flags` to `/projects/{slug}/apps` (the default tab). Remove the gear icon (settings is now a tab within the project page).

Find all instances of:
```
/orgs/${orgSlug}/projects/${project.slug}/flags
```
Replace with:
```
/orgs/${orgSlug}/projects/${project.slug}/apps
```

Remove the gear icon `<Link>` block (the one with `&#x2699;`).

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/ProjectListPage.tsx
git commit -m "feat: project cards navigate to apps tab, remove gear icon"
```

---

### Task 10: Cleanup — remove unused components

**Files:**
- Delete: `web/src/components/ProjectSwitcher.tsx`
- Delete: `web/src/components/AppAccordion.tsx`
- Delete: `web/src/components/Breadcrumb.tsx`
- Delete: `web/src/pages/ApplicationsListPage.tsx`

- [ ] **Step 1: Delete the files**

```bash
rm web/src/components/ProjectSwitcher.tsx
rm web/src/components/AppAccordion.tsx
rm web/src/components/Breadcrumb.tsx
rm web/src/pages/ApplicationsListPage.tsx
```

- [ ] **Step 2: Verify no remaining imports reference these files**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No errors. If there are import errors, fix them by removing the stale imports.

- [ ] **Step 3: Commit**

```bash
git add -A web/src/components/ProjectSwitcher.tsx web/src/components/AppAccordion.tsx web/src/components/Breadcrumb.tsx web/src/pages/ApplicationsListPage.tsx
git commit -m "chore: remove ProjectSwitcher, AppAccordion, Breadcrumb, ApplicationsListPage"
```

---

### Task 11: Update LegacyRedirect

**Files:**
- Modify: `web/src/components/LegacyRedirect.tsx`

- [ ] **Step 1: Update redirect targets**

The `sdks` redirect should now go to `/docs/sdks` instead of a project-level page. Update the component:

```tsx
import { Navigate } from 'react-router-dom';

interface LegacyRedirectProps {
  to: string;
}

export default function LegacyRedirect({ to }: LegacyRedirectProps) {
  const lastOrg = localStorage.getItem('ds_last_org') || '';
  const lastProject = localStorage.getItem('ds_last_project') || '';
  const lastApp = localStorage.getItem('ds_last_app') || '';

  if (!lastOrg) return <Navigate to="/orgs/new" replace />;
  if (to === 'settings') return <Navigate to={`/orgs/${lastOrg}/settings`} replace />;
  if (to === 'sdks') return <Navigate to="/docs/sdks" replace />;
  if (!lastProject) return <Navigate to={`/orgs/${lastOrg}/projects`} replace />;

  if ((to === 'deployments' || to === 'releases') && lastApp) {
    return (
      <Navigate to={`/orgs/${lastOrg}/projects/${lastProject}/apps/${lastApp}/${to}`} replace />
    );
  }
  if (to === 'deployments' || to === 'releases') {
    return <Navigate to={`/orgs/${lastOrg}/projects/${lastProject}/apps`} replace />;
  }
  return <Navigate to={`/orgs/${lastOrg}/projects/${lastProject}/${to}`} replace />;
}
```

Changes:
- `sdks` → `/docs/sdks` (no longer a project-level page)
- Deployments/releases without app context → project apps tab instead of flags

- [ ] **Step 2: Commit**

```bash
git add web/src/components/LegacyRedirect.tsx
git commit -m "feat: update legacy redirects for new navigation structure"
```

---

### Task 12: Full verification

- [ ] **Step 1: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 2: Start dev server and test navigation flow**

Run: `cd /Users/sgamel/git/DeploySentry && make run-web`

Test the following flow:
1. Navigate to org → see project list
2. Click a project → see project page with tabs (Applications default)
3. Click "Flags" tab → see project-level flags
4. Click "Applications" tab → see app cards
5. Click an application → see app page with tabs (Flags default)
6. Click "Deployments" tab → see deployments
7. Click back button → return to project's Applications tab
8. Click back button from project → return to project list
9. Sidebar shows org items only at all levels
10. Docs/SDKs accessible from header

- [ ] **Step 3: Verify org-level pages still work**

Test: Members page, API Keys page, Org Settings page — should be unaffected.
