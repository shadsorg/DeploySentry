# Page API Wiring — Flags, Deployments, Releases

## Overview

Wire the 6 remaining mock-data pages (FlagListPage, FlagDetailPage, DeploymentsPage, DeploymentDetailPage, ReleasesPage, ReleaseDetailPage) to their real backend API endpoints. Each page resolves URL slugs to UUIDs via `entitiesApi`, then fetches domain data via existing `flagsApi`/`deploymentsApi`/`releasesApi` functions.

## Context

- All 6 pages currently use inline `MOCK_*` constants for display data.
- API functions already exist in `web/src/api.ts`: `flagsApi.list`, `flagsApi.get`, `deploymentsApi.list`, `deploymentsApi.get`, `releasesApi.list`, `releasesApi.get`.
- Slug-to-UUID resolution is available via `entitiesApi.getOrg()`, `entitiesApi.getProject()`, `entitiesApi.getApp()` (built in the entity management pass).
- One backend gap: no `GET /flags/:id/rules` endpoint for listing targeting rules. The repository method `ListRules(flagID)` already exists — just needs a service method and handler route.
- All list API functions return wrapped objects (`{ flags: [] }`, `{ deployments: [] }`, etc.), not bare arrays. Pages must unwrap the response.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Wiring pattern | Page-by-page, no shared abstraction | Simple, self-contained, follows existing patterns |
| Slug resolution | Each page resolves its own slugs via entitiesApi | Cheap calls, avoids over-engineering a shared context |
| Missing data (events, flag changes, env state) | Show empty state / "No data" | These need their own backend endpoints — out of scope |
| Rules listing | Add `GET /flags/:id/rules` via service layer | Repository method exists; add `ListRules` to `FlagService` interface + pass-through implementation, then handler route |
| Error handling pattern | Each page renders inline loading/error UI | No shared error component exists; pages use `if (loading) return <div>Loading...</div>; if (error) return <div>Error: {error}</div>` |

## Backend Change

### New endpoint

| Method | Endpoint | Description | Permission |
|--------|----------|-------------|------------|
| `GET` | `/flags/:id/rules` | List targeting rules for a flag | `flag:read` (`PermFlagRead`) |

Implementation path (follows existing handler → service → repository layering):

1. Add `ListRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error)` to the `FlagService` interface in `internal/flags/service.go`
2. Add pass-through implementation on `flagService` struct calling `s.repo.ListRules(ctx, flagID)`
3. Add `listRules` handler method to `internal/flags/handler.go`
4. Register route: `rules.GET("", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.listRules)` in the existing `/:id/rules` group
5. Handler returns `{ "rules": [...] }` (wrapped object, consistent with all other list endpoints)

## Frontend Changes

### Response unwrapping note

All list API functions return wrapped objects:
- `flagsApi.list()` → `{ flags: Flag[] }` — use `result.flags`
- `deploymentsApi.list()` → `{ deployments: Deployment[] }` — use `result.deployments`
- `releasesApi.list()` → `{ releases: Release[] }` — use `result.releases`
- `flagsApi.listRules()` → `{ rules: TargetingRule[] }` — use `result.rules`

### Page: FlagListPage

- Remove: `MOCK_FLAGS` inline constant
- Add: `useEffect` that resolves `orgSlug` → `projectSlug` → `project.id` via `entitiesApi.getProject(orgSlug, projectSlug)`, then calls `flagsApi.list(project.id)` and unwraps `result.flags`
- Add: loading/error state
- Keep: existing filter/search logic, operate on fetched `flags` state instead of `MOCK_FLAGS`
- Note: `appSlug` may be present in the URL (flags are accessible at both project and app level). Flags are project-scoped, so always resolve the project ID regardless of whether appSlug is present. Ignore appSlug for the flags list call.

### Page: FlagDetailPage

- Remove: `MOCK_FLAG`, `MOCK_RULES` inline constants, `MOCK_FLAG_ENV_STATE` import from hierarchy
- Add: fetch flag via `flagsApi.get(id)`, fetch rules via `flagsApi.listRules(id)` (unwrap `result.rules`)
- Add: loading/error state
- Keep: `MOCK_APPLICATIONS` import (still used for app name display, not being wired this pass)
- Environment state tab: show empty state ("No environment data available") — the flag environment state API doesn't exist yet

### Page: DeploymentsPage

- Remove: `MOCK_DEPLOYMENTS` inline constant
- Add: resolve `orgSlug` → `projectSlug` → `appSlug` → `app.id` via `entitiesApi.getApp(orgSlug, projectSlug, appSlug)`, then `deploymentsApi.list(app.id)`, unwrap `result.deployments`
- Add: loading/error state
- Keep: filter/tab logic
- Note: if `appSlug` is missing from URL params, show "Select an application to view deployments" instead of fetching

### Page: DeploymentDetailPage

- Remove: `MOCK_DEPLOYMENT_DETAIL`, `MOCK_DEPLOYMENT_EVENTS` imports from hierarchy
- Add: fetch deployment via `deploymentsApi.get(id)`
- Events timeline: show empty state — no events API exists
- Add: loading/error state

### Page: ReleasesPage

- Remove: `MOCK_RELEASES` inline constant
- Add: resolve `orgSlug` → `projectSlug` → `appSlug` → `app.id`, then `releasesApi.list(app.id)`, unwrap `result.releases`
- Add: loading/error state
- Keep: tab/filter logic
- Note: if `appSlug` is missing, show "Select an application to view releases"

### Page: ReleaseDetailPage

- Remove: `MOCK_RELEASE_DETAIL`, `MOCK_RELEASE_FLAG_CHANGES` imports from hierarchy
- Add: fetch release via `releasesApi.get(id)`
- Flag changes section: show empty state — no flag changes API exists
- Add: loading/error state

### API Module

Add to `flagsApi` in `web/src/api.ts`:
```typescript
listRules: (flagId: string) =>
  request<{ rules: TargetingRule[] }>(`/flags/${flagId}/rules`),
```

### Mock File Cleanup

After wiring, remove from `web/src/mocks/hierarchy.ts`:
- `MOCK_FLAG_ENV_STATE` constant and its `FlagEnvState` type import (only used by FlagDetailPage, which now shows empty state)
- `MOCK_DEPLOYMENT_DETAIL`, `MOCK_DEPLOYMENT_EVENTS` constants (only used by DeploymentDetailPage)
- `MOCK_RELEASE_DETAIL`, `MOCK_RELEASE_FLAG_CHANGES` constants (only used by ReleaseDetailPage)

Clean up any now-unused type imports at the top of hierarchy.ts.

`MOCK_APPLICATIONS` stays (used by FlagDetailPage and MembersPage).

## Testing

### Backend
- Add `TestListRules` to `internal/flags/handler_test.go` — verify `GET /flags/:id/rules` returns `{ "rules": [...] }`
- Add mock method `listRulesFn` to `mockFlagService` in the test file

### Frontend
- No automated tests. Manual validation per page.

## Out of Scope

- Deployment events API
- Release flag changes API
- Flag environment state API
- Members/API Keys/Settings page wiring
- Ratings UI on flag pages
