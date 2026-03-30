# ApplicationsListPage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a dedicated ApplicationsListPage with card grid layout, route, and sidebar nav link.

**Architecture:** Single new page component using the existing `useApps` hook and `entitiesApi.listApps()`. Route added to App.tsx, nav link added to Sidebar.tsx. No backend changes.

**Tech Stack:** React, TypeScript, React Router

---

### Task 1: Create ApplicationsListPage

**Files:**
- Create: `web/src/pages/ApplicationsListPage.tsx`

- [ ] **Step 1: Create the page component**

```tsx
import { useParams, Link } from 'react-router-dom';
import { useApps } from '@/hooks/useEntities';

export default function ApplicationsListPage() {
  const { orgSlug, projectSlug } = useParams();
  const { apps, loading, error } = useApps(orgSlug, projectSlug);

  if (!orgSlug || !projectSlug) return null;
  if (loading) return <div className="page-loading">Loading applications...</div>;
  if (error) return <div className="page-error">Error: {error}</div>;

  return (
    <div>
      <div className="page-header-row">
        <h1 className="page-header">Applications</h1>
        <Link to={`/orgs/${orgSlug}/projects/${projectSlug}/apps/new`} className="btn btn-primary">
          Create App
        </Link>
      </div>
      {apps.length === 0 ? (
        <div className="empty-state">
          <p>No applications yet.</p>
          <Link to={`/orgs/${orgSlug}/projects/${projectSlug}/apps/new`} className="btn btn-primary">
            Create Your First App
          </Link>
        </div>
      ) : (
        <div className="project-card-grid">
          {apps.map((app) => (
            <Link
              key={app.id}
              to={`/orgs/${orgSlug}/projects/${projectSlug}/apps/${app.slug}/deployments`}
              className="project-card"
            >
              <h3 className="project-card-name">{app.name}</h3>
              <span className="project-card-slug">{app.slug}</span>
              {app.description && (
                <p className="project-card-desc">{app.description}</p>
              )}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/pages/ApplicationsListPage.tsx
git commit -m "feat: add ApplicationsListPage with card grid layout"
```

---

### Task 2: Add Route and Sidebar Link

**Files:**
- Modify: `web/src/App.tsx`
- Modify: `web/src/components/Sidebar.tsx`

- [ ] **Step 1: Add import and route to App.tsx**

Add import after line 22 (`import CreateAppPage`):
```tsx
import ApplicationsListPage from './pages/ApplicationsListPage';
```

Add route at line 62 (before `apps/new`):
```tsx
<Route path="apps" element={<ApplicationsListPage />} />
```

- [ ] **Step 2: Add sidebar nav link to Sidebar.tsx**

Add an "Applications" NavLink in the project-level section, right after the AppAccordion (line 31) and before the "Project" section divider (line 36). Insert this block:

```tsx
<NavLink
  to={`/orgs/${orgSlug}/projects/${projectSlug}/apps`}
  className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
>
  <span className="nav-icon">□</span>
  Applications
</NavLink>
```

- [ ] **Step 3: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npm run build`
Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add web/src/App.tsx web/src/components/Sidebar.tsx
git commit -m "feat: add route and sidebar link for ApplicationsListPage"
```
