# Entity Management API & UI Wiring

## Overview

Build backend CRUD endpoints for organizations, projects, and applications, then wire the entire web dashboard to use real API calls instead of hardcoded mock data. This fixes three user-facing bugs: Create Project button does nothing, new orgs disappear from the switcher, and Settings is a navigation dead-end.

## Context

- The backend has no CRUD endpoints for orgs, projects, or applications. Only membership management routes exist (in UserHandler).
- The frontend reads all entity data from `src/mocks/hierarchy.ts` — a hardcoded file with two orgs, three projects, and six apps. Creates go nowhere.
- The CLI expects slug-based URL hierarchy (`/orgs/:slug/projects`) but these endpoints don't exist yet.
- Models for `Organization`, `Project`, and `Application` already exist in `internal/models/`.
- The existing `UserHandler` registers membership routes at `/orgs/:org_id/members` using UUID params. The new entity routes use slug-based params at different sub-paths (`/projects`, `/apps`), so there is no route conflict. The membership routes remain UUID-based and are unchanged.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Handler structure | Single `internal/entities/` package | Org/project/app are a tight hierarchy; separate packages would be over-engineered for CRUD |
| URL scheme | Slug-based hierarchy (`/orgs/:slug/projects`) | Matches CLI expectations, human-readable |
| Org creation | Auto-add creator as owner in `org_members`, wrapped in a transaction | Standard pattern — creator owns what they create; transaction prevents orphaned orgs |
| Mock strategy | Split `src/mocks/hierarchy.ts` — remove entity exports, keep non-entity mocks (flags, deployments, releases, members, environments, API keys) | Entity data comes from API; other mock data stays until those pages are wired |
| Frontend state | Custom hooks (`useOrgs`, `useProjects`, `useApps`) | Simple fetch-on-mount pattern, no external state library needed |
| Slug uniqueness | Org slugs globally unique; project slugs unique per org; app slugs unique per project | Matches URL hierarchy — each level's slug is unique within its parent scope |
| Org Plan field | Default to `"free"` on creation | Matches the database column default (`DEFAULT 'free'`) |
| Pagination | Not included for list endpoints | Entity counts are small (tens, not thousands); pagination can be added later if needed |

## Backend

### Database

The `organizations`, `projects`, `applications`, and `org_members` tables already exist (migrations 001, 004, 029, 003). Verify the following unique constraints exist; if not, add migration 031:

- `organizations`: `UNIQUE (slug)` — already exists (migration 001)
- `projects`: `UNIQUE (org_id, slug)` — verify, add if missing
- `applications`: `UNIQUE (project_id, slug)` — verify, add if missing

### Repository Interface

A single `EntityRepository` in `internal/entities/repository.go` covering:

- **Orgs:** `CreateOrg`, `GetOrgBySlug`, `ListOrgsByUser`, `UpdateOrg`
- **Projects:** `CreateProject`, `GetProjectBySlug` (within an org), `ListProjectsByOrg`, `UpdateProject`
- **Applications:** `CreateApp`, `GetAppBySlug` (within a project), `ListAppsByProject`, `UpdateApp`
- **Membership:** `AddOrgMember` (for auto-adding creator as owner on org create)

PostgreSQL implementation in `internal/platform/database/postgres/entities.go`.

### API Routes

All routes require authentication. Permissions per operation:

| Method | Endpoint | Description | Permission |
|--------|----------|-------------|------------|
| `POST` | `/orgs` | Create org (auto-adds creator as owner) | Authenticated (any user) |
| `GET` | `/orgs` | List orgs for authenticated user | Authenticated (any user) |
| `GET` | `/orgs/:orgSlug` | Get org by slug | `org:manage` or org member |
| `PUT` | `/orgs/:orgSlug` | Update org | `org:manage` |
| `POST` | `/orgs/:orgSlug/projects` | Create project in org | `org:manage` |
| `GET` | `/orgs/:orgSlug/projects` | List projects in org | Org member |
| `GET` | `/orgs/:orgSlug/projects/:projectSlug` | Get project by slug | Org member |
| `PUT` | `/orgs/:orgSlug/projects/:projectSlug` | Update project | `project:manage` |
| `POST` | `/orgs/:orgSlug/projects/:projectSlug/apps` | Create app in project | `project:manage` |
| `GET` | `/orgs/:orgSlug/projects/:projectSlug/apps` | List apps in project | Org member |
| `GET` | `/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug` | Get app by slug | Org member |
| `PUT` | `/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug` | Update app | `project:manage` |

Duplicate slug returns 409 Conflict. Not found returns 404.

**Note:** Existing membership routes at `/orgs/:org_id/members` (UserHandler) remain unchanged and use UUID-based params. There is no route conflict because the new entity routes use different sub-paths (`/projects`, `/apps`).

### Create Org Flow

1. Validate name and slug (non-empty, slug format)
2. Begin database transaction
3. Insert into `organizations` table with `owner_id = user_id` from JWT context and `plan = 'free'`
4. Insert into `org_members` with `role = 'owner'` and `user_id` from JWT context
5. Commit transaction
6. Return the created org (201)

### Slug Resolution Pattern

Each nested endpoint resolves parent slugs before operating:
- `/orgs/:orgSlug/projects` → look up org by slug, use org.id to query projects
- `/orgs/:orgSlug/projects/:projectSlug/apps` → resolve org, then project within org, then query apps

## Frontend

### API Module

Replace the existing `orgsApi` and `applicationsApi` in `src/api.ts` with a unified `entitiesApi`:

```typescript
entitiesApi = {
  listOrgs, getOrg, createOrg, updateOrg,
  listProjects, getProject, createProject, updateProject,
  listApps, getApp, createApp, updateApp,
}
```

The existing `orgsApi.list()` and `orgsApi.get()` paths are already correct — they just need create/update methods added. The existing `applicationsApi` (UUID-based) is replaced entirely.

### Hooks

New file `src/hooks/useEntities.ts` with three hooks:

- `useOrgs()` → `{ orgs, loading, error, refresh }`
- `useProjects(orgSlug)` → `{ projects, loading, error, refresh }`
- `useApps(orgSlug, projectSlug)` → `{ apps, loading, error, refresh }`

Each hook calls the API on mount and re-fetches when slug params change. The `refresh()` callback lets create pages trigger a refetch after success.

Helper functions derived from hook data replace the old `getOrgName()`, `getProjectName()`, `getAppName()` utilities — e.g., `projects.find(p => p.slug === projectSlug)?.name`.

### Mock File Split

Do NOT delete `src/mocks/hierarchy.ts` entirely. Instead:

1. **Remove** from `hierarchy.ts`: `MOCK_ORGS`, `MOCK_PROJECTS`, `MOCK_APPLICATIONS`, and all helper functions (`getMockOrgs`, `getMockProjects`, `getMockApps`, `getOrgName`, `getProjectName`, `getAppName`).
2. **Keep** in `hierarchy.ts`: `MOCK_FLAG_ENV_STATE`, `MOCK_DEPLOYMENT_DETAIL`, `MOCK_DEPLOYMENT_EVENTS`, `MOCK_RELEASE_DETAIL`, `MOCK_RELEASE_FLAG_CHANGES`, `MOCK_ENVIRONMENTS`, `MOCK_MEMBERS`, `MOCK_GROUPS`, `MOCK_API_KEYS`, `getMockEnvironments`, `getEnvironmentName`.

These non-entity mocks are used by pages that are not being wired in this pass (FlagDetailPage, DeploymentsPage, ReleasesPage, MembersPage, APIKeysPage, SettingsPage, etc.).

### Pages & Components Modified

| Page/Component | Change |
|----------------|--------|
| `OrgSwitcher` | Use `useOrgs()` instead of `getMockOrgs()` / `getOrgName()` |
| `ProjectSwitcher` | Use `useProjects(orgSlug)`, resolve name from projects list |
| `Sidebar` | Use `useApps()` instead of `getMockApps()` |
| `AppAccordion` | Use `useApps()` instead of `getMockApps()` |
| `Breadcrumb` | Resolve names from hook data instead of `getOrgName()` etc. |
| `ProjectListPage` | Use `useProjects()`, enable Create button linking to `/orgs/:orgSlug/projects/new` |
| `CreateOrgPage` | Call `entitiesApi.createOrg()`, show API errors inline, navigate on success |
| `CreateAppPage` | Call `entitiesApi.createApp()`, remove mock environment import, show API errors inline, navigate on success |
| `DefaultRedirect` | Replace `MOCK_ORGS` with `entitiesApi.listOrgs()` call to pick first org for redirect |
| `LegacyRedirect` | Replace `MOCK_ORGS`/`MOCK_PROJECTS` with API calls for redirect resolution |
| `FlagListPage` | Replace `getProjectName`/`getAppName` with name resolution from route context or hooks |
| `SDKsPage` | Replace `getProjectName` with name resolution from route context |
| `DeploymentsPage` | Replace `getAppName` with name resolution from route context |
| `ReleasesPage` | Replace `getAppName` with name resolution from route context |

### New Page

`CreateProjectPage` at `src/pages/CreateProjectPage.tsx` — form with name + slug fields (auto-generates slug from name, same pattern as CreateOrgPage). Shows API errors inline (e.g., 409 "slug already exists").

### Route Registration

Add to `App.tsx` inside the HierarchyLayout routes, **before** the `:projectSlug` route (so "new" isn't matched as a slug):

```
/orgs/:orgSlug/projects/new → CreateProjectPage
```

### Navigation Fixes

1. **Sidebar "Projects" link** — Add a nav item that navigates to `/orgs/:orgSlug/projects` so users can always get back to the project list.
2. **Loading/error states** — Pages show a loading indicator while fetching and an error message on failure. Create pages show inline error messages from the API (e.g., "slug already taken" on 409).
3. **Empty states** — When no projects or apps exist, show helpful empty state with "Create" call-to-action.

## Testing

### Backend

Handler tests with mock repository:
- Create org → 201 + auto-owner membership
- List orgs → returns only user's orgs
- Create project → 201 within org
- Duplicate slug → 409 Conflict
- Unknown slug → 404 Not Found
- Slug resolution → nested routes correctly resolve parent entities

### Frontend

No automated frontend tests for this pass. Manual validation: create org → see it in switcher → create project → see it in list → create app → see it in sidebar.

### Out of scope

- Wiring flag/deploy/release pages to their real APIs (separate pass)
- Delete endpoints (orgs/projects/apps — future work, needs confirmation dialogs per production safety requirements)
- Permission edge cases beyond basic org membership checks
- CLI config endpoints (`/orgs/:slug/projects/:slug/config`) — separate scope
