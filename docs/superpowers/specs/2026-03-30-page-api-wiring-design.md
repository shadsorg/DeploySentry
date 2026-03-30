# Page API Wiring — Flags, Deployments, Releases

## Overview

Wire the 6 remaining mock-data pages (FlagListPage, FlagDetailPage, DeploymentsPage, DeploymentDetailPage, ReleasesPage, ReleaseDetailPage) to their real backend API endpoints. Each page resolves URL slugs to UUIDs via `entitiesApi`, then fetches domain data via existing `flagsApi`/`deploymentsApi`/`releasesApi` functions.

## Context

- All 6 pages currently use inline `MOCK_*` constants for display data.
- API functions already exist in `web/src/api.ts`: `flagsApi.list`, `flagsApi.get`, `deploymentsApi.list`, `deploymentsApi.get`, `releasesApi.list`, `releasesApi.get`.
- Slug-to-UUID resolution is available via `entitiesApi.getOrg()`, `entitiesApi.getProject()`, `entitiesApi.getApp()` (built in the entity management pass).
- One backend gap: no `GET /flags/:id/rules` endpoint for listing targeting rules. The repository method `ListRules(flagID)` already exists — just needs a route and handler.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Wiring pattern | Page-by-page, no shared abstraction | Simple, self-contained, follows existing patterns |
| Slug resolution | Each page resolves its own slugs via entitiesApi | Cheap calls, avoids over-engineering a shared context |
| Missing data (events, flag changes, env state) | Show empty state / "No data" | These need their own backend endpoints — out of scope |
| Rules listing | Add `GET /flags/:id/rules` backend endpoint | Repository method already exists, just needs handler wiring |

## Backend Change

### New endpoint

| Method | Endpoint | Description | Permission |
|--------|----------|-------------|------------|
| `GET` | `/flags/:id/rules` | List targeting rules for a flag | `flag:read` (`PermFlagRead`) |

Handler calls `service.ListRules(flagID)` → returns `[]*TargetingRule` (or empty array). The `FlagService.ListRules` method doesn't exist on the service interface, but the repository has `ListRules(ctx, flagID)`. Add a pass-through on the service, or call the repository method from a new handler. The simplest path: add a `listRules` handler method to the existing flags handler that calls the repository's `ListRules` via the service.

Check: The `FlagService` interface may not have a `ListRules` method. If not, the handler can be added alongside the existing `addRule`/`updateRule`/`deleteRule` methods which already access rules through the service. Look at how `addRule` works and follow the same pattern.

## Frontend Changes

### Page: FlagListPage

- Remove: `MOCK_FLAGS` inline constant
- Add: `useEffect` that resolves `orgSlug` → `projectSlug` → `project.id`, then calls `flagsApi.list(project.id)`
- Add: loading/error state
- Keep: existing filter/search logic, just operate on fetched `flags` state instead of `MOCK_FLAGS`

### Page: FlagDetailPage

- Remove: `MOCK_FLAG`, `MOCK_RULES` inline constants, `MOCK_FLAG_ENV_STATE` import from hierarchy
- Add: fetch flag via `flagsApi.get(id)`, fetch rules via new `GET /flags/:id/rules`
- Add: loading/error state
- Keep: `MOCK_APPLICATIONS` import (still used for app name display, not being wired this pass)
- Environment state tab: show empty state ("No environment data available") — the `flag_environment_state` API doesn't exist yet

### Page: DeploymentsPage

- Remove: `MOCK_DEPLOYMENTS` inline constant
- Add: resolve `orgSlug` → `projectSlug` → `appSlug` → `app.id`, then `deploymentsApi.list(app.id)`
- Add: loading/error state
- Keep: filter/tab logic

### Page: DeploymentDetailPage

- Remove: `MOCK_DEPLOYMENT_DETAIL`, `MOCK_DEPLOYMENT_EVENTS` imports from hierarchy
- Add: fetch deployment via `deploymentsApi.get(id)`
- Events timeline: show empty state or derive from deployment status fields — no events API exists
- Add: loading/error state

### Page: ReleasesPage

- Remove: `MOCK_RELEASES` inline constant
- Add: resolve `orgSlug` → `projectSlug` → `appSlug` → `app.id`, then `releasesApi.list(app.id)`
- Add: loading/error state
- Keep: tab/filter logic

### Page: ReleaseDetailPage

- Remove: `MOCK_RELEASE_DETAIL`, `MOCK_RELEASE_FLAG_CHANGES` imports from hierarchy
- Add: fetch release via `releasesApi.get(id)`
- Flag changes section: show empty state — no flag changes API exists
- Add: loading/error state

### API Module

Add to `flagsApi` in `web/src/api.ts`:
```typescript
listRules: (flagId: string) =>
  request<TargetingRule[]>(`/flags/${flagId}/rules`),
```

### Mock File Cleanup

After wiring, the following imports from `web/src/mocks/hierarchy.ts` can be removed:
- `MOCK_FLAG_ENV_STATE` (FlagDetailPage no longer uses it)
- `MOCK_DEPLOYMENT_DETAIL`, `MOCK_DEPLOYMENT_EVENTS` (DeploymentDetailPage no longer uses them)
- `MOCK_RELEASE_DETAIL`, `MOCK_RELEASE_FLAG_CHANGES` (ReleaseDetailPage no longer uses them)

These constants can be deleted from `hierarchy.ts` since no other file imports them.

`MOCK_APPLICATIONS` stays (used by FlagDetailPage and MembersPage).

## Testing

### Backend
- Add `TestListRules` to `internal/flags/handler_test.go` — verify `GET /flags/:id/rules` returns rules array

### Frontend
- No automated tests. Manual validation per page.

## Out of Scope

- Deployment events API
- Release flag changes API
- Flag environment state API
- Members/API Keys/Settings page wiring
- Ratings UI on flag pages
