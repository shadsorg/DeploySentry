# UI Reskin Plan — DeploySentry

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reskin the DeploySentry React dashboard to the "Sentry Dark UI" design system defined in `DESIGN.md`.

**Architecture:** Apply the design tokens (colors, typography, roundness) globally via CSS variables, then work page-by-page replacing ad-hoc styles with design-system classes, guided by the HTML mockups in `newscreens/` where available.

**Tech Stack:** React 18, TypeScript, Vite, plain CSS with CSS custom properties (no Tailwind), `Manrope` font via Google Fonts.

---

## Design System Reference

File: `DESIGN.md` (copied to worktree root)

| Token | Value |
|---|---|
| Primary font | Manrope, sans-serif (headings: 800 weight, -0.025em tracking) |
| Body font | Inter, sans-serif |
| Primary accent | `#6366f1` (indigo) |
| Surface base | `#0b1326` |
| Surface container | `#1b2339` |
| On-surface text | `#e2e8f0` |
| On-surface variant | `#94a3b8` |
| Tertiary (success) | `#10b981` |
| Error | `#ef4444` |
| Border/outline | `#334155` |
| Border-variant | `#1e293b` |
| Border radius | `8px` |

---

## Mockup → Page Mapping

### Pages WITH Mockups

| Mockup File | Page Component | Route | Status |
|---|---|---|---|
| `newscreens/org-overview.html` | `OrgStatusPage.tsx` | `/orgs/:orgSlug/status` | ✅ done |
| `newscreens/org-environments.html` | `OrgStatusPage.tsx` (environments tab) | `/orgs/:orgSlug/status` | ✅ done |
| `newscreens/deploy-history.html` | `OrgDeploymentsPage.tsx` | `/orgs/:orgSlug/deployments` | ✅ done |
| `newscreens/deployment-history.html` | `DeploymentsPage.tsx` | `…/apps/:appSlug/deployments` | ✅ done |
| `newscreens/project-applications.html` | `ProjectAppsTab.tsx` | `…/projects/:projectSlug/apps` | ✅ done |
| `newscreens/project-application.html` | `AppPage.tsx` | `…/apps/:appSlug` (shell) | ✅ done |
| `newscreens/flag-application.html` | `FlagListPage.tsx` | `…/apps/:appSlug/flags` | ✅ done |
| `newscreens/flag-details.html` | `FlagDetailPage.tsx` | `…/flags/:id` | ✅ done |
| `newscreens/targeting-rules.html` | `FlagDetailPage.tsx` (targeting tab) | `…/flags/:id` | ✅ done |
| `newscreens/project-analytics.html` | `AnalyticsPage.tsx` | `…/projects/:projectSlug/analytics` | ✅ done |
| `newscreens/project-settings.html` | `SettingsPage.tsx` (project level) | `…/projects/:projectSlug/settings` | ✅ done |
| `newscreens/environment-config.html` | `SettingsPage.tsx` (org level) | `/orgs/:orgSlug/settings` | ✅ done |
| `newscreens/members-permissions.html` | `MembersPage.tsx` | `/orgs/:orgSlug/members` | ✅ done |
| `newscreens/member-group.html` | `MembersPage.tsx` (group/role view) | `/orgs/:orgSlug/members` | ✅ done |
| `newscreens/api-key-management.html` | `APIKeysPage.tsx` (list) | `/orgs/:orgSlug/api-keys` | ✅ done |
| `newscreens/api-key-detailed.html` | `APIKeysPage.tsx` (detail) | `/orgs/:orgSlug/api-keys` | ✅ done |
| `newscreens/rollouts-active.html` | `RolloutsPage.tsx` | `/orgs/:orgSlug/rollouts` | ✅ done |
| `newscreens/rollout-groups.html` | `RolloutGroupsPage.tsx` | `/orgs/:orgSlug/rollout-groups` | ✅ done |
| `newscreens/rollout-strategy.html` | `StrategiesPage.tsx` / `StrategyEditor.tsx` | `/orgs/:orgSlug/strategies` | ✅ done |
| `newscreens/documentation.html` | `DocsPage.tsx` (CSS only) | `/docs`, `/docs/:slug` | ✅ done (CSS vars) |

### Pages WITHOUT Mockups — Apply Design System Anyway

| Page Component | Route | Status |
|---|---|---|
| `LandingPage.tsx` | `/` | deferred (public marketing, out of scope) |
| `LoginPage.tsx` | `/login` | ✅ done |
| `RegisterPage.tsx` | `/register` | ✅ done |
| `CreateOrgPage.tsx` | `/orgs/new` | ✅ done |
| `CreateProjectPage.tsx` | `/orgs/:orgSlug/projects/new` | ✅ done |
| `CreateAppPage.tsx` | `…/projects/:projectSlug/apps/new` | ✅ done |
| `ProjectListPage.tsx` | `/orgs/:orgSlug/projects` | ✅ done |
| `ProjectPage.tsx` | `…/projects/:projectSlug` (tab shell) | ✅ done (via AppPage shell) |
| `FlagListPage.tsx` (project level) | `…/projects/:projectSlug/flags` | ✅ done |
| `FlagCreatePage.tsx` | `…/flags/new` | ✅ done |
| `ReleasesPage.tsx` | `…/apps/:appSlug/releases` | ✅ done |
| `ReleaseDetailPage.tsx` | `…/apps/:appSlug/releases/:id` | ✅ done |
| `RolloutDetailPage.tsx` | `/orgs/:orgSlug/rollouts/:id` | ✅ done |
| `RolloutGroupDetailPage.tsx` | `/orgs/:orgSlug/rollout-groups/:id` | ✅ done |
| `SDKsPage.tsx` | `/sdks` | ✅ done |
| `PolicyAndDefaultsTab.tsx` | (tab within SettingsPage) | ✅ done |
| `SettingsPage.tsx` (app level) | `…/apps/:appSlug/settings` | ✅ done |
| `DeploymentDetailPage.tsx` | `…/deployments/:id` | ✅ done |

### Shared Components — Apply Design System

| Component | File | Status |
|---|---|---|
| Sidebar | `components/Sidebar.tsx` | ✅ done |
| Site header | `components/SiteHeader.tsx` | ✅ done |
| Hierarchy layout | `components/HierarchyLayout.tsx` | deferred |
| Org switcher | `components/OrgSwitcher.tsx` | deferred |
| User menu | `components/UserMenu.tsx` | deferred |
| Action bar | `components/ActionBar.tsx` | deferred |
| Confirm dialog | `components/ConfirmDialog.tsx` | deferred |
| Rollout components | `components/rollout/*.tsx` (5 files) | deferred |
| Analytics components | `components/analytics/*.tsx` (4 files) | deferred |
| Landing components | `components/landing/*.tsx` (6 files) | deferred |
| Docs components | `components/docs/*.tsx` (2 files) | deferred |

---

## Phase Sequence (recommended)

1. **Foundation** — CSS variables / Tailwind config with all design tokens; load Manrope font ✅
2. **Shell** — Sidebar, SiteHeader, HierarchyLayout (these frame every page) ✅
3. **High-mockup pages** — Work through the 20 pages that have mockups, highest-traffic first ✅
4. **No-mockup pages** — Apply design system tokens to remaining pages ✅
5. **Shared components** — Rollout, analytics, landing sub-components (deferred)

---

## Completion Record

- **Branch**: `claude/epic-dirac-09650a`
- **Committed**: Yes (5 commits in this session)
- **Pushed**: No
- **CI Checks**: Pending

---

## Notes

- Mockups are HTML files (not images); open in browser to inspect actual rendered design
- `newscreens/deploy-history.html` and `newscreens/deployment-history.html` are similar but map to different routes (org-level vs app-level)
- `FlagListPage` is reused at three levels (project, app — same component, different context)
- `StrategyEditor.tsx` and `PolicyAndDefaultsTab.tsx` are sub-components, not top-level routes
- `SDKsPage.tsx` has a legacy redirect but no direct route — confirm whether to include in reskin scope
- No Tailwind in this project — everything is plain CSS with `globals.css` custom properties
