# Web UI Phase 1: Core Navigation Overhaul

## Overview

Restructure the React dashboard from flat, single-project navigation to org-scoped hierarchical navigation with Org → Project → Application → Environment hierarchy. Existing pages become context-aware under the new routing structure. This is Phase 1 of 3 for the full Web UI redesign.

**Parent spec:** `docs/superpowers/specs/2026-03-28-platform-redesign-design.md` (section 4)

## Goals

1. Replace the flat sidebar with a hierarchical sidebar: org dropdown → project dropdown → app accordion → nav items.
2. Migrate all routes from flat (`/flags`) to org-scoped (`/orgs/:orgSlug/projects/:projectSlug/flags`).
3. Make existing pages context-aware — they display which org/project/app they're scoped to and adjust headings accordingly.
4. Add subtle breadcrumb navigation above page titles.
5. Add org and project switcher dropdowns to the sidebar.

## Non-Goals

- New pages (org members, org settings, application CRUD) — Phase 3.
- Flag detail 3-tab redesign, deployment detail, release detail — Phase 2.
- Real API integration for hierarchy data — pages continue using mock data, but mock data is organized by hierarchy context.
- Mobile responsiveness improvements.
- Updating `Deployment`, `Release`, or `Environment` TypeScript interfaces to match the new backend data model — type updates are deferred to Phase 2 when those pages are redesigned. Phase 1 keeps existing interfaces and mock data structures.

---

## 1. Architecture Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Sidebar layout | Context selector + flat nav | Org/project selected via dropdowns, apps shown as accordion below. Max 2 levels of nesting. Scales to many projects. Common pattern (Vercel, Supabase). **Deliberate simplification from parent spec** which shows projects as accordions — dropdowns scale better and reduce nesting depth. |
| Breadcrumbs | Subtle, muted text above page title | Reinforces location without competing with sidebar for navigation. "You are here" indicator, not primary nav. |
| Routing | Full spec routes | URLs encode full hierarchy (`/orgs/:orgSlug/projects/:projectSlug/...`). Self-contained, shareable, bookmarkable. Old flat routes redirect. |
| State management | URL is source of truth | orgSlug, projectSlug, appSlug extracted from route params. Sidebar dropdowns controlled by URL. No separate context state needed. Avoids sync bugs. |

---

## 2. Routing Structure

### Authenticated Routes

```
/orgs/new                                                   — create org
/orgs/:orgSlug/projects                                     — project list (org landing)
/orgs/:orgSlug/projects/:projectSlug/flags                  — project-level flags
/orgs/:orgSlug/projects/:projectSlug/flags/:id              — project-level flag detail
/orgs/:orgSlug/projects/:projectSlug/settings               — project settings
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/deployments      — app deployments
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/deployments/:id  — deployment detail
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/releases         — app releases
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/releases/:id     — release detail
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/flags            — app-level flags
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/flags/new        — create app-level flag
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/flags/:id        — app-level flag detail
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/settings         — app settings
/orgs/:orgSlug/members                                      — org members
/orgs/:orgSlug/api-keys                                     — org API keys
/orgs/:orgSlug/settings                                     — org settings
/orgs/:orgSlug/projects/:projectSlug/flags/new              — create project-level flag
/orgs/:orgSlug/projects/:projectSlug/analytics              — project analytics (not in parent spec, carried from existing app)
/orgs/:orgSlug/projects/:projectSlug/sdks                   — project SDKs (not in parent spec, carried from existing app)
```

### Public Routes

```
/login
/register
```

### Default Route

`/` redirects to `/orgs/:lastOrgSlug/projects` where `lastOrgSlug` is read from localStorage. If no stored org, redirect to `/orgs/new` or the user's first org.

### Legacy Redirects

Old flat routes (`/flags`, `/deployments`, `/releases`, `/analytics`, `/sdks`) redirect to the equivalent route under the user's last-used org/project context. `/settings` redirects to org-level settings (`/orgs/:orgSlug/settings`) since it's the broadest scope.

### Error Handling

If URL params don't match any known entity (e.g., `/orgs/nonexistent-org/projects`), pages display an inline "Not found" message with a link back to `/`. No dedicated 404 page is needed — the sidebar still renders (with empty dropdowns if the org is invalid) and the main content area shows the error. This keeps the user oriented and avoids a jarring full-page 404.

---

## 3. Component Design

### 3.1 New: `HierarchyLayout`

Replaces the current `Layout` component as the wrapper for all authenticated routes.

**Responsibilities:**
- Renders the new `Sidebar` and main content area (`<Outlet />`)
- Extracts `orgSlug`, `projectSlug`, `appSlug` from `useParams()`
- Renders `Breadcrumb` above the outlet
- No context provider needed — child pages read their own params

**Structure:**
```
┌─────────────────┬──────────────────────────────────────┐
│    Sidebar      │  Breadcrumb                          │
│                 │  ┌──────────────────────────────────┐ │
│  [OrgSwitcher]  │  │                                  │ │
│  [ProjSwitcher] │  │     <Outlet />                   │ │
│                 │  │                                  │ │
│  Applications   │  │                                  │ │
│    ▶ web-app    │  └──────────────────────────────────┘ │
│    ▼ api-server │                                      │
│      Deploys    │                                      │
│      Releases   │                                      │
│      Flags      │                                      │
│      Settings   │                                      │
│                 │                                      │
│  ─ Project ──── │                                      │
│    Feature Flags│                                      │
│    Analytics    │                                      │
│    SDKs         │                                      │
│    Settings     │                                      │
│                 │                                      │
│  ─ Org ──────── │                                      │
│    Members      │                                      │
│    API Keys     │                                      │
│    Settings     │                                      │
│                 │                                      │
│  [User / Sign out]                                     │
└─────────────────┴──────────────────────────────────────┘
```

### 3.2 New: `OrgSwitcher`

Dropdown at the top of the sidebar.

**Behavior:**
- Displays the current org name (derived from `orgSlug` in the URL)
- Lists all orgs the user belongs to (mock data for now)
- Selecting a different org navigates to `/orgs/:newOrgSlug/projects`
- "Create Organization" option at the bottom navigates to `/orgs/new`

**Data source:** Mock array of orgs. Will be replaced with `orgsApi.list()` when API is ready.

### 3.3 New: `ProjectSwitcher`

Dropdown below the org switcher.

**Behavior:**
- Displays the current project name (derived from `projectSlug` in the URL)
- Lists all projects in the selected org (mock data for now)
- Selecting a different project navigates to `/orgs/:orgSlug/projects/:newProjectSlug/flags`
- Only visible when a project is selected (not on org-level pages like `/orgs/:orgSlug/members`)

**Data source:** Mock array of projects filtered by org. Will be replaced with `projectsApi.list(orgId)`.

### 3.4 New: `AppAccordion`

List of applications for the selected project, each expandable.

**Behavior:**
- Lists apps for the current project (mock data)
- Each app is a collapsible section. Clicking the app name toggles expand/collapse.
- Expanded app shows nav links: Deployments, Releases, Flags, Settings
- The app matching `appSlug` in the URL is auto-expanded and highlighted
- Nav links generate URLs using the current orgSlug/projectSlug/appSlug
- Only visible when a project is selected

**Data source:** Mock array of apps filtered by project.

### 3.5 New: `Breadcrumb`

Subtle navigation indicator above the page title.

**Behavior:**
- Reads `orgSlug`, `projectSlug`, `appSlug` from URL params
- Renders: `Acme Corp / Project Beta / api-server / Flags` (segments vary by route depth)
- Each segment is a `<Link>` to the appropriate level:
  - Org name → `/orgs/:orgSlug/projects`
  - Project name → `/orgs/:orgSlug/projects/:projectSlug/flags`
  - App name → `/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/deployments` (deployments is the app landing page — the primary action for an application is shipping code)
  - Page name → not a link (current page)
- Styled: small font, muted color, positioned above page heading with minimal spacing

**Segment resolution:** Slugs are displayed directly (e.g., `acme-corp`). When real API data is available, these will resolve to display names.

### 3.6 Rewrite: `Sidebar`

Complete rewrite of the existing sidebar.

**Structure (top to bottom):**
1. `OrgSwitcher` dropdown
2. `ProjectSwitcher` dropdown (hidden on org-level pages)
3. `AppAccordion` (hidden when no project selected)
4. **Project section** — separator label "Project", then: Feature Flags, Analytics, SDKs, Settings
5. **Org section** — separator label "Organization", then: Members, API Keys, Settings
6. **User section** — user name/email, sign out button
7. Version number at bottom

**Active state:** The nav item matching the current URL gets a highlighted background. The sidebar determines this by matching route params against nav link targets.

**Width:** Keep existing 240px fixed width.

### 3.7 Rewrite: `App.tsx` Route Tree

Replace the flat route structure with nested routes.

**Auth guard refactor:** The existing `RequireAuth` and `RedirectIfAuth` components use a children pattern (`<RequireAuth><Layout /></RequireAuth>`). These must be refactored to render `<Outlet />` so they work as layout routes in React Router v6.

```tsx
<Routes>
  {/* Public */}
  <Route element={<RedirectIfAuth />}>
    <Route path="/login" element={<LoginPage />} />
    <Route path="/register" element={<RegisterPage />} />
  </Route>

  {/* Authenticated */}
  <Route element={<RequireAuth />}>
    {/* Default redirect */}
    <Route path="/" element={<DefaultRedirect />} />

    <Route path="/orgs/new" element={<CreateOrgPage />} />

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
          <Route path="deployments/:id" element={<DeploymentDetailPage />} />
          <Route path="releases" element={<ReleasesPage />} />
          <Route path="releases/:id" element={<ReleaseDetailPage />} />
          <Route path="flags" element={<FlagListPage />} />
          <Route path="flags/new" element={<FlagCreatePage />} />
          <Route path="flags/:id" element={<FlagDetailPage />} />
          <Route path="settings" element={<SettingsPage level="app" />} />
        </Route>
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
</Routes>
```

**`DefaultRedirect`**: Reads `lastOrgSlug` from localStorage, redirects to `/orgs/:lastOrgSlug/projects`. Falls back to first org or `/orgs/new`.

**`LegacyRedirect`**: Reads last org/project/app context from localStorage, constructs the new URL, redirects. `/settings` maps to org-level settings.

**`CreateOrgPage`**: A new stub page with a form for org name/slug. Minimal implementation — just a form that navigates to `/orgs/:newSlug/projects` on submit. Full org management is Phase 3.

**Settings routing:** Instead of separate `OrgSettingsPage`, `ProjectSettingsPage`, and `AppSettingsPage` components, a single `SettingsPage` receives a `level` prop ("org", "project", or "app") to determine which tabs to show. This avoids creating three near-identical page files.

**RealtimeManager:** The existing `App.tsx` initializes `RealtimeManager` in a `useEffect`. This initialization moves into `HierarchyLayout` so it runs once for all authenticated routes, same as today.

---

## 4. Page Updates

Each existing page receives minimal updates to be context-aware. Content and functionality stay the same — only headings, context display, and route param extraction change.

### 4.1 `DashboardPage` → `ProjectListPage`

- Route: `/orgs/:orgSlug/projects`
- Shows a card grid of projects in the org (mock data)
- Each card shows: project name, slug, description, number of applications, last activity
- Clicking a project card navigates to `/orgs/:orgSlug/projects/:projectSlug/flags`
- "Create Project" button at top (stub — navigates nowhere for now, full CRUD is Phase 3)
- Replaces the old dashboard as the org landing page

### 4.2 `FlagListPage`

- Works at two levels: project (`/orgs/.../projects/:projectSlug/flags`) and app (`/orgs/.../apps/:appSlug/flags`)
- Extracts `appSlug` from params — if present, heading shows "api-server — Flags", if absent shows "Project Beta — Feature Flags"
- Filter/search/table stay the same

### 4.3 `FlagDetailPage`

- Route params now include orgSlug/projectSlug and optionally appSlug
- Back button links to the correct flag list (project or app level)
- Content unchanged

### 4.4 `FlagCreatePage`

- Same as FlagDetailPage — extract params, adjust back link
- If at app level, pre-select category as "release"

### 4.5 `DeploymentsPage`

- Always at app level: `/orgs/.../apps/:appSlug/deployments`
- Heading: "api-server — Deployments"
- Content unchanged

### 4.6 `ReleasesPage`

- Always at app level: `/orgs/.../apps/:appSlug/releases`
- Heading: "api-server — Releases"
- Content unchanged

### 4.7 `AnalyticsPage`

- Project level: `/orgs/.../projects/:projectSlug/analytics`
- Heading: "Project Beta — Analytics"
- Content unchanged

### 4.8 `SDKsPage`

- Project level: `/orgs/.../projects/:projectSlug/sdks`
- Heading: "Project Beta — SDKs & Docs"
- Content unchanged

### 4.9 `SettingsPage`

- Receives a `level` prop ("org", "project", or "app") from the route definition
- Shows different tab sets based on level:
  - **Org** (`/orgs/:orgSlug/settings`): Webhooks, Notifications. The Members and API Keys org routes also render `SettingsPage` with a `tab` prop to pre-select those tabs.
  - **Project** (`/orgs/.../projects/:projectSlug/settings`): Project Settings (name, defaults)
  - **App** (`/orgs/.../apps/:appSlug/settings`): App Settings (name, repo URL)
- Existing tab content from the current single `SettingsPage` is distributed across the appropriate levels

---

## 5. Types

New interfaces in `types.ts`:

```typescript
interface Organization {
  id: string;
  name: string;
  slug: string;
  created_at: string;
  updated_at: string;
}

interface Application {
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

The existing `Project` interface already has `org_id`.

---

## 6. API Client Updates

New endpoint modules in `api.ts`:

```typescript
orgsApi: {
  list()                    // GET /api/v1/orgs
  get(slug: string)         // GET /api/v1/orgs/:slug (if exists, else by ID)
}

applicationsApi: {
  list(projectId: string)   // GET /api/v1/projects/:projectId/applications
  get(id: string)           // GET /api/v1/applications/:id
}
```

Existing modules update parameter names:
- `deploymentsApi.list(projectId)` → `deploymentsApi.list(applicationId)`
- `releasesApi.list(projectId)` → `releasesApi.list(applicationId)`

These return mock data until the backend endpoints are implemented.

---

## 7. Mock Data

A new file `web/src/mocks/hierarchy.ts` provides mock hierarchy data for the sidebar:

```typescript
const MOCK_ORGS = [
  { id: '...', name: 'Acme Corp', slug: 'acme-corp' },
  { id: '...', name: 'Personal', slug: 'personal' },
];

const MOCK_PROJECTS = [
  { id: '...', org_id: '...', name: 'Platform', slug: 'platform' },
  { id: '...', org_id: '...', name: 'Mobile', slug: 'mobile' },
];

const MOCK_APPLICATIONS = [
  { id: '...', project_id: '...', name: 'API Server', slug: 'api-server' },
  { id: '...', project_id: '...', name: 'Web App', slug: 'web-app' },
  { id: '...', project_id: '...', name: 'Mobile', slug: 'mobile' },
];
```

Pages and sidebar components import from this file. When real API integration happens, these imports are replaced with API calls.

---

## 8. File Structure

### New Files
- `web/src/components/HierarchyLayout.tsx`
- `web/src/components/OrgSwitcher.tsx`
- `web/src/components/ProjectSwitcher.tsx`
- `web/src/components/AppAccordion.tsx`
- `web/src/components/Breadcrumb.tsx`
- `web/src/components/DefaultRedirect.tsx`
- `web/src/components/LegacyRedirect.tsx`
- `web/src/mocks/hierarchy.ts`
- `web/src/pages/ProjectListPage.tsx`
- `web/src/pages/CreateOrgPage.tsx` — stub page with org name/slug form

### Modified Files
- `web/src/App.tsx` — route tree rewrite
- `web/src/auth.tsx` — refactor `RequireAuth` and `RedirectIfAuth` from children pattern to `<Outlet />` layout route pattern
- `web/src/components/Sidebar.tsx` — complete rewrite
- `web/src/styles/globals.css` — new styles for sidebar dropdowns, accordion, breadcrumb
- `web/src/types.ts` — add Organization, Application interfaces
- `web/src/api.ts` — add orgsApi, applicationsApi modules
- `web/src/pages/FlagListPage.tsx` — context-aware heading
- `web/src/pages/FlagDetailPage.tsx` — context-aware back link
- `web/src/pages/FlagCreatePage.tsx` — context-aware back link
- `web/src/pages/DeploymentsPage.tsx` — context-aware heading
- `web/src/pages/ReleasesPage.tsx` — context-aware heading
- `web/src/pages/AnalyticsPage.tsx` — context-aware heading
- `web/src/pages/SDKsPage.tsx` — context-aware heading
- `web/src/pages/SettingsPage.tsx` — accepts `level` prop ("org" | "project" | "app"), shows different tabs per level

### Deleted Files
- `web/src/components/Layout.tsx` — replaced by HierarchyLayout
- `web/src/pages/DashboardPage.tsx` — replaced by ProjectListPage

---

## 9. Phases 2 and 3 (Out of Scope)

**Phase 2:** New/redesigned pages — flag detail 3-tab layout, deployment detail with status timeline, release detail with flag changes and rollout actions.

**Phase 3:** New entity management pages — org members, org-level API keys and settings, application CRUD, environment management.
