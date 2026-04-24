# UI Reskin Plan — DeploySentry

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reskin the DeploySentry React dashboard to the "Sentry Dark UI" design system defined in `DESIGN.md`.

**Architecture:** Apply the design tokens (colors, typography, roundness) globally via CSS variables, then work page-by-page replacing ad-hoc styles with design-system classes, guided by the HTML mockups in `newscreens/` where available.

**Tech Stack:** React 18, TypeScript, Vite, TailwindCSS (or inline styles — verify in `web/tailwind.config.*`), `Manrope` font.

---

## Design System Reference

File: `DESIGN.md` (copied to worktree root)

| Token | Value |
|---|---|
| Primary font | Manrope, sans-serif (headings: 800 weight, -0.025em tracking) |
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
| `newscreens/org-overview.html` | `OrgStatusPage.tsx` | `/orgs/:orgSlug/status` | not started |
| `newscreens/org-environments.html` | `OrgStatusPage.tsx` (environments tab) | `/orgs/:orgSlug/status` | not started |
| `newscreens/deploy-history.html` | `OrgDeploymentsPage.tsx` | `/orgs/:orgSlug/deployments` | not started |
| `newscreens/deployment-history.html` | `DeploymentsPage.tsx` | `…/apps/:appSlug/deployments` | not started |
| `newscreens/project-applications.html` | `ProjectAppsTab.tsx` | `…/projects/:projectSlug/apps` | not started |
| `newscreens/project-application.html` | `AppPage.tsx` | `…/apps/:appSlug` (shell) | not started |
| `newscreens/flag-application.html` | `FlagListPage.tsx` | `…/apps/:appSlug/flags` | not started |
| `newscreens/flag-details.html` | `FlagDetailPage.tsx` | `…/flags/:id` | not started |
| `newscreens/targeting-rules.html` | `FlagDetailPage.tsx` (targeting tab) | `…/flags/:id` | not started |
| `newscreens/project-analytics.html` | `AnalyticsPage.tsx` | `…/projects/:projectSlug/analytics` | not started |
| `newscreens/project-settings.html` | `SettingsPage.tsx` (project level) | `…/projects/:projectSlug/settings` | not started |
| `newscreens/environment-config.html` | `SettingsPage.tsx` (org level) | `/orgs/:orgSlug/settings` | not started |
| `newscreens/members-permissions.html` | `MembersPage.tsx` | `/orgs/:orgSlug/members` | not started |
| `newscreens/member-group.html` | `MembersPage.tsx` (group/role view) | `/orgs/:orgSlug/members` | not started |
| `newscreens/api-key-management.html` | `APIKeysPage.tsx` (list) | `/orgs/:orgSlug/api-keys` | not started |
| `newscreens/api-key-detailed.html` | `APIKeysPage.tsx` (detail) | `/orgs/:orgSlug/api-keys` | not started |
| `newscreens/rollouts-active.html` | `RolloutsPage.tsx` | `/orgs/:orgSlug/rollouts` | not started |
| `newscreens/rollout-groups.html` | `RolloutGroupsPage.tsx` | `/orgs/:orgSlug/rollout-groups` | not started |
| `newscreens/rollout-strategy.html` | `StrategiesPage.tsx` / `StrategyEditor.tsx` | `/orgs/:orgSlug/strategies` | not started |
| `newscreens/documentation.html` | `DocsPage.tsx` | `/docs`, `/docs/:slug` | not started |

### Pages WITHOUT Mockups — Apply Design System Anyway

| Page Component | Route | Status |
|---|---|---|
| `LandingPage.tsx` | `/` | not started |
| `LoginPage.tsx` | `/login` | not started |
| `RegisterPage.tsx` | `/register` | not started |
| `CreateOrgPage.tsx` | `/orgs/new` | not started |
| `CreateProjectPage.tsx` | `/orgs/:orgSlug/projects/new` | not started |
| `CreateAppPage.tsx` | `…/projects/:projectSlug/apps/new` | not started |
| `ProjectListPage.tsx` | `/orgs/:orgSlug/projects` | not started |
| `ProjectPage.tsx` | `…/projects/:projectSlug` (tab shell) | not started |
| `FlagListPage.tsx` (project level) | `…/projects/:projectSlug/flags` | not started |
| `FlagCreatePage.tsx` | `…/flags/new` | not started |
| `ReleasesPage.tsx` | `…/apps/:appSlug/releases` | not started |
| `ReleaseDetailPage.tsx` | `…/apps/:appSlug/releases/:id` | not started |
| `RolloutDetailPage.tsx` | `/orgs/:orgSlug/rollouts/:id` | not started |
| `RolloutGroupDetailPage.tsx` | `/orgs/:orgSlug/rollout-groups/:id` | not started |
| `SDKsPage.tsx` | (legacy redirect target `/sdks`) | not started |
| `PolicyAndDefaultsTab.tsx` | (tab within SettingsPage) | not started |
| `SettingsPage.tsx` (app level) | `…/apps/:appSlug/settings` | not started |
| `DeploymentDetailPage.tsx` | `…/deployments/:id` | not started |

### Shared Components — Apply Design System

| Component | File | Status |
|---|---|---|
| Sidebar | `components/Sidebar.tsx` | not started |
| Site header | `components/SiteHeader.tsx` | not started |
| Hierarchy layout | `components/HierarchyLayout.tsx` | not started |
| Org switcher | `components/OrgSwitcher.tsx` | not started |
| User menu | `components/UserMenu.tsx` | not started |
| Action bar | `components/ActionBar.tsx` | not started |
| Confirm dialog | `components/ConfirmDialog.tsx` | not started |
| Rollout components | `components/rollout/*.tsx` (5 files) | not started |
| Analytics components | `components/analytics/*.tsx` (4 files) | not started |
| Landing components | `components/landing/*.tsx` (6 files) | not started |
| Docs components | `components/docs/*.tsx` (2 files) | not started |

---

## Phase Sequence (recommended)

1. **Foundation** — CSS variables / Tailwind config with all design tokens; load Manrope font
2. **Shell** — Sidebar, SiteHeader, HierarchyLayout (these frame every page)
3. **High-mockup pages** — Work through the 20 pages that have mockups, highest-traffic first
4. **No-mockup pages** — Apply design system tokens to remaining pages
5. **Shared components** — Rollout, analytics, landing sub-components

---

## Notes

- Mockups are HTML files (not images); open in browser to inspect actual rendered design
- `newscreens/deploy-history.html` and `newscreens/deployment-history.html` are similar but map to different routes (org-level vs app-level)
- `FlagListPage` is reused at three levels (project, app — same component, different context)
- `StrategyEditor.tsx` and `PolicyAndDefaultsTab.tsx` are sub-components, not top-level routes
- `SDKsPage.tsx` has a legacy redirect but no direct route — confirm whether to include in reskin scope
