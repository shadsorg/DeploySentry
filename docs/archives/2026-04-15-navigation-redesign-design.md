# Navigation Redesign: Tab-Based Drill-Down Hierarchy

**Phase**: Design

## Overview

Replace the current sidebar-driven navigation with a clean drill-down hierarchy using tabs in the content area. The sidebar becomes org-only. Project and application navigation moves into the content area with tabs and back buttons, creating a clear Org → Project → Application flow.

## Current Problems

- Sidebar shows all levels simultaneously (org + project + app items), making it unclear what scope you're in
- "Applications" is a sidebar item that navigates to a separate list page — an unnecessary intermediate step
- Clicking an app from the list jumps to deployments with no clear way to understand the context switch
- Too many items in the sidebar at once

## Design

### Three Navigation Levels

**Level 1 — Org (project list)**
- Content shows list of project cards with "+ Add Project" option
- Title: "Projects"
- No back button (top level)
- No tabs (projects are the only org-level content in the main area)

**Level 2 — Project**
- Title: project name (top-left, large/bold), with edit gear icon
- Back button: "← Back to Projects" directly below the title
- Tabs: **Applications** (default) | Flags | Analytics | Settings
- Applications tab shows app cards with "+ Add App" option
- Flags tab shows project-level flags
- Analytics tab shows project analytics
- Settings tab shows project settings (name, description, repo_url, danger zone)

**Level 3 — Application**
- Title: application name (top-left, large/bold), with edit gear icon
- Back button: "← Back to {project name}" directly below the title
- Tabs: **Flags** (default) | Deployments | Releases | Settings
- Each tab renders the existing page content for that feature

### Sidebar (Simplified)

The sidebar becomes org-scoped only, consistent across all levels:

- **Org switcher** (dropdown at top, always visible)
- **Members** → `/orgs/{orgSlug}/members`
- **API Keys** → `/orgs/{orgSlug}/api-keys`
- **Settings** → `/orgs/{orgSlug}/settings`

Removed from sidebar:
- ProjectSwitcher dropdown
- AppAccordion
- All project-level items (Flags, Analytics, SDKs & Docs, Settings)
- All app-level items (Deployments, Releases, Flags, Settings)

### Header Changes

- Move "SDKs & Docs" to the site header (top bar), accessible from any page
- Remove it from the sidebar

### Content Area Layout

Every level follows the same pattern:
```
┌─────────────────────────────────────┐
│ Entity Name                    ⚙ Edit│
│ ← Back to {parent}                  │
│                                     │
│ Tab1 │ Tab2 │ Tab3 │ Tab4           │
│─────────────────────────────────────│
│                                     │
│  Tab content renders here           │
│                                     │
└─────────────────────────────────────┘
```

## Routing Changes

### New URL Structure

The URL structure stays the same — only the components that render at each route change.

```
/orgs/:orgSlug/projects                           → ProjectListPage (no tabs, just cards)
/orgs/:orgSlug/projects/:projectSlug              → redirect to /apps (default tab)
/orgs/:orgSlug/projects/:projectSlug/apps          → ProjectPage, Applications tab
/orgs/:orgSlug/projects/:projectSlug/flags         → ProjectPage, Flags tab
/orgs/:orgSlug/projects/:projectSlug/analytics     → ProjectPage, Analytics tab
/orgs/:orgSlug/projects/:projectSlug/settings      → ProjectPage, Settings tab
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug            → redirect to /flags (default tab)
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/flags      → AppPage, Flags tab
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/flags/new  → FlagCreatePage (within AppPage)
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/flags/:id  → FlagDetailPage (within AppPage)
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/deployments     → AppPage, Deployments tab
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/deployments/:id → DeploymentDetailPage
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/releases        → AppPage, Releases tab
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/releases/:id    → ReleaseDetailPage
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/settings        → AppPage, Settings tab
```

### Route-to-Tab Mapping

The active tab is determined by the current URL path segment:

**ProjectPage tabs:**
| Path segment | Tab |
|---|---|
| `/apps` | Applications |
| `/flags` | Flags |
| `/analytics` | Analytics |
| `/settings` | Settings |

**AppPage tabs:**
| Path segment | Tab |
|---|---|
| `/flags` | Flags |
| `/deployments` | Deployments |
| `/releases` | Releases |
| `/settings` | Settings |

## Component Changes

### New Components

**`ProjectPage`** — wrapper component for project-level views:
- Renders title (project name), back button, and tab bar
- Tab clicks navigate to the corresponding route (e.g., clicking "Flags" navigates to `/orgs/{org}/projects/{project}/flags`)
- Active tab determined from URL
- Renders existing page components as tab content via `<Outlet />`

**`AppPage`** — wrapper component for app-level views:
- Same pattern as ProjectPage but with app-level tabs
- Back button goes to `/orgs/{org}/projects/{project}/apps`
- Renders existing page components as tab content via `<Outlet />`

### Modified Components

**`Sidebar.tsx`** — remove ProjectSwitcher, AppAccordion, project-level nav, app-level nav. Keep only org switcher and org-level items (Members, API Keys, Settings).

**`HierarchyLayout.tsx`** — remove Breadcrumb component (replaced by back button in content area). Keep localStorage context persistence.

**`SiteHeader.tsx`** — add "SDKs & Docs" link.

### Removed Components

- **`ProjectSwitcher.tsx`** — no longer needed (projects are selected by clicking cards)
- **`AppAccordion.tsx`** — no longer needed (apps are listed in the project's Applications tab)
- **`Breadcrumb.tsx`** — replaced by back button in content area
- **`ApplicationsListPage.tsx`** — replaced by the Applications tab in ProjectPage

### Reused Components (no changes needed)

- `FlagListPage`, `FlagCreatePage`, `FlagDetailPage` — render inside tab content
- `DeploymentsPage`, `DeploymentDetailPage` — render inside tab content
- `ReleasesPage`, `ReleaseDetailPage` — render inside tab content
- `AnalyticsPage` — render inside tab content
- `SettingsPage` — render inside tab content (already supports `level` prop)
- `MembersPage`, `APIKeysPage` — org-level, no changes

### ProjectListPage Changes

- Remove gear icon (added in prior work) — projects are now clicked to drill in, not linked to settings directly
- Keep "+ Add Project" card
- Keep deleted project styling (dimmed, restore button) from prior work
- Each card click navigates to `/orgs/{org}/projects/{project}/apps`

## Legacy Redirect Updates

Update `LegacyRedirect.tsx` to handle the new default tabs:
- `/flags` → check context, route to project-level or app-level flags
- `/deployments` → route to app-level deployments
- `/releases` → route to app-level releases

## Out of Scope

- Mobile/responsive navigation
- Keyboard navigation between tabs
- Animated transitions between levels
- Search/filter within project or app lists
- Drag-and-drop reordering

## Checklist

### New Components
- [ ] `ProjectPage` — title, back button, tab bar, outlet
- [ ] `AppPage` — title, back button, tab bar, outlet

### Modified Components
- [ ] Simplify `Sidebar.tsx` — org-only items
- [ ] Update `HierarchyLayout.tsx` — remove breadcrumb
- [ ] Update `SiteHeader.tsx` — add SDKs & Docs link
- [ ] Update `ProjectListPage.tsx` — remove gear icon, cards navigate to project

### Routing
- [ ] Update `App.tsx` — new route structure with ProjectPage/AppPage wrappers
- [ ] Add redirects: `/projects/:slug` → `/projects/:slug/apps`, `/apps/:slug` → `/apps/:slug/flags`
- [ ] Update `LegacyRedirect.tsx`

### Cleanup
- [ ] Remove `ProjectSwitcher.tsx`
- [ ] Remove `AppAccordion.tsx`
- [ ] Remove `Breadcrumb.tsx`
- [ ] Remove `ApplicationsListPage.tsx` (content moves to ProjectPage Applications tab)

### Verify
- [ ] All existing pages render correctly within new tab wrappers
- [ ] Back button navigates up correctly at each level
- [ ] Tab switching preserves URL structure
- [ ] Org-level pages (Members, API Keys, Settings) unaffected
- [ ] Delete/restore functionality still works within new layout

## Completion Record
<!-- Fill in when phase is set to Complete -->
- **Branch**: ``
- **Committed**: No
- **Pushed**: No
- **CI Checks**:
