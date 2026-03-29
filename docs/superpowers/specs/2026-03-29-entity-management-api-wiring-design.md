# Entity Management API & UI Wiring

## Overview

Build backend CRUD endpoints for organizations, projects, and applications, then wire the entire web dashboard to use real API calls instead of hardcoded mock data. This fixes three user-facing bugs: Create Project button does nothing, new orgs disappear from the switcher, and Settings is a navigation dead-end.

## Context

- The backend has no CRUD endpoints for orgs, projects, or applications. Only membership management routes exist (in UserHandler).
- The frontend reads all entity data from `src/mocks/hierarchy.ts` — a hardcoded file with two orgs, three projects, and six apps. Creates go nowhere.
- The CLI expects slug-based URL hierarchy (`/orgs/:slug/projects`) but these endpoints don't exist yet.
- Models for `Organization`, `Project`, and `Application` already exist in `internal/models/`.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Handler structure | Single `internal/entities/` package | Org/project/app are a tight hierarchy; separate packages would be over-engineered for CRUD |
| URL scheme | Slug-based hierarchy (`/orgs/:slug/projects`) | Matches CLI expectations, human-readable |
| Org creation | Auto-add creator as owner in `org_members` | Standard pattern — creator owns what they create |
| Mock removal | Delete `src/mocks/hierarchy.ts` entirely | Clean break; all entity data comes from the API |
| Frontend state | Custom hooks (`useOrgs`, `useProjects`, `useApps`) | Simple fetch-on-mount pattern, no external state library needed |

## Backend

### Repository Interface

A single `EntityRepository` in `internal/entities/repository.go` covering:

- **Orgs:** `CreateOrg`, `GetOrgBySlug`, `ListOrgsByUser`, `UpdateOrg`
- **Projects:** `CreateProject`, `GetProjectBySlug`, `ListProjectsByOrg`, `UpdateProject`
- **Applications:** `CreateApp`, `GetAppBySlug`, `ListAppsByProject`, `UpdateApp`
- **Membership:** `AddOrgMember` (for auto-adding creator as owner on org create)

PostgreSQL implementation in `internal/platform/database/postgres/entities.go`.

### API Routes

All routes require authentication. Write operations require `org:manage` permission.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/orgs` | Create org (auto-adds creator as owner) |
| `GET` | `/orgs` | List orgs for authenticated user |
| `GET` | `/orgs/:orgSlug` | Get org by slug |
| `PUT` | `/orgs/:orgSlug` | Update org |
| `POST` | `/orgs/:orgSlug/projects` | Create project in org |
| `GET` | `/orgs/:orgSlug/projects` | List projects in org |
| `GET` | `/orgs/:orgSlug/projects/:projectSlug` | Get project by slug |
| `PUT` | `/orgs/:orgSlug/projects/:projectSlug` | Update project |
| `POST` | `/orgs/:orgSlug/projects/:projectSlug/apps` | Create app in project |
| `GET` | `/orgs/:orgSlug/projects/:projectSlug/apps` | List apps in project |
| `GET` | `/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug` | Get app by slug |
| `PUT` | `/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug` | Update app |

The handler resolves slugs to UUIDs internally. Duplicate slug returns 409 Conflict. Not found returns 404.

### Create Org Flow

1. Validate name and slug (non-empty, slug format)
2. Insert into `organizations` table
3. Insert into `org_members` with `role = 'owner'` and `user_id` from JWT context
4. Return the created org

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

### Hooks

New file `src/hooks/useEntities.ts` with three hooks:

- `useOrgs()` → `{ orgs, loading, error, refresh }`
- `useProjects(orgSlug)` → `{ projects, loading, error, refresh }`
- `useApps(orgSlug, projectSlug)` → `{ apps, loading, error, refresh }`

Each hook calls the API on mount and re-fetches when slug params change. The `refresh()` callback lets create pages trigger a refetch after success.

### Mock Removal

Delete `src/mocks/hierarchy.ts`. All components that import from it are updated to use hooks. Inline mock data in individual pages (flags, deployments, releases) stays for now — those get wired in a separate pass.

### Pages Modified

| Page | Change |
|------|--------|
| `OrgSwitcher` | Use `useOrgs()` instead of `getMockOrgs()` |
| `ProjectSwitcher` | Use `useProjects(orgSlug)` instead of `getMockProjects()` |
| `Sidebar` | Use `useApps()` instead of `getMockApps()` |
| `AppAccordion` | Use `useApps()` instead of `getMockApps()` |
| `Breadcrumb` | Resolve names from hook data instead of `getOrgName()` etc. |
| `ProjectListPage` | Use `useProjects()`, enable Create button linking to `/orgs/:orgSlug/projects/new` |
| `CreateOrgPage` | Call `entitiesApi.createOrg()`, navigate on success |
| `CreateAppPage` | Call `entitiesApi.createApp()`, navigate on success |

### New Page

`CreateProjectPage` at `src/pages/CreateProjectPage.tsx` — form with name + slug fields (auto-generates slug from name, same pattern as CreateOrgPage). Route: `/orgs/:orgSlug/projects/new`.

### Navigation Fixes

1. **Sidebar "Projects" link** — Add a nav item that navigates to `/orgs/:orgSlug/projects` so users can always get back to the project list.
2. **Loading/error states** — Pages show a loading indicator while fetching and an error message on failure, instead of silently rendering empty.
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
- Permission edge cases beyond basic auth
