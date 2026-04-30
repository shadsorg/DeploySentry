# Project & Application Edit & Delete

**Phase**: Design

## Overview

Add full edit and delete capabilities for both projects and applications, at the API and web UI layers. Currently, the project update endpoint only persists the `name` field, no delete functionality exists for either entity, and application edit/delete UI is missing.

### Delete Strategy: Instant vs. Guarded

- **No flags**: Delete immediately (hard delete). No soft-delete ceremony.
- **Has flags**: Soft-delete flow with guards. Blocked if any flags have evaluations in the last 14 days. Hard delete available 7 days after soft delete.
- **Rename**: Always immediate, regardless of flags.

This applies identically to both projects and applications.

## Requirements

### Project Edit
- Fix the repository to persist `name`, `description`, and `repo_url` (all three are already accepted by the handler but only `name` is saved).
- Fix `GetProjectBySlug` and `ListProjectsByOrg` queries to select all columns (`description`, `repo_url`, `settings`).
- No slug changes allowed.

### Application Edit
- Application update already persists `name`, `description`, `repo_url` correctly.
- No slug changes allowed.
- Add edit UI (currently missing from the frontend).

### Delete (Both Entities)

**Instant delete (no flags):**
- If the entity has zero flags (project: no flags in project; application: no flags with `application_id` matching), perform an immediate hard delete.
- For projects: also checks that all child applications have zero flags. If any application has flags, the project follows the soft-delete flow.
- Returns 204 on success.

**Soft delete (has flags):**
- Add `deleted_at TIMESTAMPTZ` column to `projects` and `applications` tables.
- Soft delete sets `deleted_at = now()`.
- **Blocked** if any flags have evaluations in the last 14 days (checked via `flag_evaluation_log`). Returns `409 Conflict` with a list of the active flags (key, name, last evaluation time).
- Soft-deleted entities are hidden from lists by default; shown when `?include_deleted=true` is passed.
- Soft-deleted entities can be restored (clears `deleted_at`).

**Hard delete (has flags, after soft delete):**
- Available only after the entity has been soft-deleted for at least 7 days.
- Performs a real `DELETE` (cascades via existing FK constraints).
- Returns `409 Conflict` with `eligible_at` date if attempted too early.

### Permissions

**Projects:**
- Edit, soft delete, restore: `PermProjectManage`
- Hard delete: `PermOrgManage` (higher privilege)

**Applications:**
- Edit, soft delete, restore: `PermProjectManage`
- Hard delete: `PermOrgManage`

## API Design

### Projects

**Existing (fixed):**
- `PUT /api/v1/orgs/:orgSlug/projects/:projectSlug` — now persists `name`, `description`, `repo_url`.

**New endpoints:**

| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| DELETE | `/api/v1/orgs/:orgSlug/projects/:projectSlug` | PermProjectManage | Delete (instant if no flags, soft delete if flags) |
| DELETE | `/api/v1/orgs/:orgSlug/projects/:projectSlug/permanent` | PermOrgManage | Hard delete (7-day gate, only after soft delete) |
| POST | `/api/v1/orgs/:orgSlug/projects/:projectSlug/restore` | PermProjectManage | Restore soft-deleted project |

### Applications

**Existing:**
- `PUT /api/v1/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug` — already works correctly.

**New endpoints:**

| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| DELETE | `/api/v1/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug` | PermProjectManage | Delete (instant if no flags, soft delete if flags) |
| DELETE | `/api/v1/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/permanent` | PermOrgManage | Hard delete (7-day gate) |
| POST | `/api/v1/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/restore` | PermProjectManage | Restore soft-deleted application |

### Delete Handler Logic (same for both entities)

```
1. Count flags for the entity
2. If count == 0:
   → Hard delete immediately, return 204
3. If count > 0:
   → Check HasRecentFlagActivity (14-day window)
   → If active flags: return 409 with active_flags list
   → If no active flags: soft delete (set deleted_at), return 204
```

### Error Responses

**Delete blocked — active flags (409):**
```json
{
  "error": "project has flags with recent activity",
  "active_flags": [
    {"key": "dark-mode", "name": "Dark Mode", "last_evaluated": "2026-04-12T15:30:00Z"}
  ]
}
```

**Hard delete too early (409):**
```json
{
  "error": "project must be soft-deleted for at least 7 days",
  "deleted_at": "2026-04-10T12:00:00Z",
  "eligible_at": "2026-04-17T12:00:00Z"
}
```

## Database Changes

### Migration 039: Add `deleted_at` to projects and applications
```sql
ALTER TABLE projects ADD COLUMN deleted_at TIMESTAMPTZ;
ALTER TABLE applications ADD COLUMN deleted_at TIMESTAMPTZ;
```

## Backend Changes

### Models

**`internal/models/project.go`:**
- Add `DeletedAt *time.Time` field with `json:"deleted_at,omitempty"`.

**`internal/models/application.go`:**
- Add `DeletedAt *time.Time` field with `json:"deleted_at,omitempty"`.

### Repository (`internal/platform/database/postgres/entities.go`)

**Fix existing (projects only):**
- `UpdateProject` — update `name`, `description`, `repo_url` (not just `name`).
- `GetProjectBySlug` — select all columns including `description`, `repo_url`, `settings`.
- `ListProjectsByOrg` — select all columns; add `includeDeleted bool` param; filter `deleted_at IS NULL` when false.

**Fix existing (applications):**
- `ListAppsByProject` — add `includeDeleted bool` param; filter `deleted_at IS NULL` when false.

**New methods (projects):**
- `CountFlagsByProject(ctx, projectID uuid.UUID) (int, error)` — `SELECT COUNT(*) FROM feature_flags WHERE project_id = $1`.
- `SoftDeleteProject(ctx, id uuid.UUID) error` — `UPDATE projects SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`.
- `HardDeleteProject(ctx, id uuid.UUID) error` — `DELETE FROM projects WHERE id = $1`.
- `RestoreProject(ctx, id uuid.UUID) error` — `UPDATE projects SET deleted_at = NULL WHERE id = $1 AND deleted_at IS NOT NULL`.

**New methods (applications):**
- `CountFlagsByApp(ctx, applicationID uuid.UUID) (int, error)` — `SELECT COUNT(*) FROM feature_flags WHERE application_id = $1`.
- `SoftDeleteApp(ctx, id uuid.UUID) error` — `UPDATE applications SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`.
- `HardDeleteApp(ctx, id uuid.UUID) error` — `DELETE FROM applications WHERE id = $1`.
- `RestoreApp(ctx, id uuid.UUID) error` — `UPDATE applications SET deleted_at = NULL WHERE id = $1 AND deleted_at IS NOT NULL`.

**Flag activity query (shared):**
- `HasRecentFlagActivity(ctx, projectID uuid.UUID, applicationID *uuid.UUID, since time.Time) ([]FlagActivitySummary, error)`
  - When `applicationID` is nil: checks all flags in the project.
  - When `applicationID` is non-nil: checks only flags with that `application_id`.
  - Joins `feature_flags` with `flag_evaluation_log` to find flags with evaluations after `since`.
  - Returns `[]FlagActivitySummary{Key, Name, LastEvaluated}`.

### Repository Interface (`internal/entities/repository.go`)
- Add `CountFlagsByProject`, `CountFlagsByApp` to entity repository interface.
- Add soft delete, hard delete, restore methods for both entities.
- Add `FlagActivityChecker` interface for `HasRecentFlagActivity`.

### Service (`internal/entities/service.go`)

**New methods (projects):**
- `DeleteProject(ctx, orgID, projectSlug)` — looks up project, counts flags. If zero: hard delete. If nonzero: check activity, soft delete or return 409.
- `HardDeleteProject(ctx, orgID, projectSlug)` — looks up project (including deleted), verifies `deleted_at` is 7+ days old, calls repo `HardDeleteProject`.
- `RestoreProject(ctx, orgID, projectSlug)` — looks up project (including deleted), calls repo `RestoreProject`.

**New methods (applications):**
- `DeleteApp(ctx, projectID, appSlug)` — same logic as `DeleteProject` but scoped to app flags.
- `HardDeleteApp(ctx, projectID, appSlug)` — same 7-day gate pattern.
- `RestoreApp(ctx, projectID, appSlug)` — clears `deleted_at`.

**Constants:**
```go
const recentActivityWindow = 14 * 24 * time.Hour
const hardDeleteEligibleAfter = 7 * 24 * time.Hour
```

### Handler (`internal/entities/handler.go`)

**New project routes:**
- `DELETE /:projectSlug` → `deleteProject` handler
- `DELETE /:projectSlug/permanent` → `hardDeleteProject` handler
- `POST /:projectSlug/restore` → `restoreProject` handler
- Update `listProjects` to read `?include_deleted=true` query param.

**New application routes:**
- `DELETE /:appSlug` → `deleteApp` handler
- `DELETE /:appSlug/permanent` → `hardDeleteApp` handler
- `POST /:appSlug/restore` → `restoreApp` handler
- Update `listApps` to read `?include_deleted=true` query param.

**Delete handler response:**
- Instant delete (no flags): 204 with `{"deleted": "permanent"}`.
- Soft delete (flags, no activity): 204 with `{"deleted": "soft", "eligible_for_hard_delete": "<ISO date>"}`.
- Blocked (active flags): 409 with active flags list.

## Web UI Changes

### Project List Page (`web/src/pages/ProjectListPage.tsx`)
- Add a gear icon to each project card linking to the project settings page.
- Soft-deleted projects: dimmed/grayed-out card with a "Deleted" badge, a "Restore" button, and text: "Hard delete available on MM/DD/YYYY" (computed from `deleted_at + 7 days`).
- Fetch with `?include_deleted=true` to show deleted projects in the list.

### Applications List Page (`web/src/pages/ApplicationsListPage.tsx`)
- Add a gear icon to each application card linking to the application settings page.
- Soft-deleted applications: same dimmed styling, badge, restore button as projects.
- Fetch with `?include_deleted=true`.

### Settings Page (`web/src/pages/SettingsPage.tsx`)

**Project-level "General" tab:**
- Edit form for `name`, `description`, `repo_url` with save button.

**Project-level "Danger Zone":**
- If no flags: show delete button. Confirmation dialog: "This project has no flags and will be permanently deleted. This cannot be undone."
- If flags with recent activity: show warning banner listing active flags. Delete button disabled.
- If flags with no recent activity: show delete button. Confirmation dialog: "This will soft-delete the project. It can be restored within 7 days."
- If already soft-deleted and 7+ days old: show hard delete button. Confirmation dialog: "This will permanently delete the project and all associated data. This cannot be undone."

**Application-level settings:**
- Same edit form pattern (name, description, repo_url).
- Same danger zone pattern as projects.

### Frontend API Client (`web/src/api.ts`)
- Fix `updateProject` to send all fields (currently may only send `name`).
- Add `deleteProject(orgSlug, projectSlug)` — `DELETE`.
- Add `hardDeleteProject(orgSlug, projectSlug)` — `DELETE /permanent`.
- Add `restoreProject(orgSlug, projectSlug)` — `POST /restore`.
- Add `deleteApp(orgSlug, projectSlug, appSlug)` — `DELETE`.
- Add `hardDeleteApp(orgSlug, projectSlug, appSlug)` — `DELETE /permanent`.
- Add `restoreApp(orgSlug, projectSlug, appSlug)` — `POST /restore`.

### Frontend Types (`web/src/types.ts`)
- Add `deleted_at?: string` to `Project` and `Application` interfaces.
- Add `FlagActivitySummary` type: `{ key: string; name: string; last_evaluated: string }`.
- Add `DeleteResponse` type: `{ deleted: 'permanent' | 'soft'; eligible_for_hard_delete?: string }`.

## Out of Scope
- CLI changes
- Slug editing
- Automated hard-delete cleanup (background job)
- Notifications on delete

## Checklist

### Database
- [ ] Migration 039: add `deleted_at` to projects and applications

### Models
- [ ] Add `DeletedAt` to Project model
- [ ] Add `DeletedAt` to Application model

### Repository
- [ ] Fix `UpdateProject` to persist all fields
- [ ] Fix `GetProjectBySlug` / `ListProjectsByOrg` to select all columns
- [ ] Add `includeDeleted` param to `ListProjectsByOrg` and `ListAppsByProject`
- [ ] `CountFlagsByProject`, `CountFlagsByApp`
- [ ] `SoftDeleteProject`, `HardDeleteProject`, `RestoreProject`
- [ ] `SoftDeleteApp`, `HardDeleteApp`, `RestoreApp`
- [ ] `HasRecentFlagActivity` query (with optional applicationID)
- [ ] Update repository interface

### Service
- [ ] `DeleteProject` with instant/soft delete logic
- [ ] `HardDeleteProject` with 7-day gate
- [ ] `RestoreProject`
- [ ] `DeleteApp` with instant/soft delete logic
- [ ] `HardDeleteApp` with 7-day gate
- [ ] `RestoreApp`

### Handler
- [ ] Project delete, hard delete, restore routes
- [ ] Application delete, hard delete, restore routes
- [ ] `listProjects` include_deleted param
- [ ] `listApps` include_deleted param

### Frontend
- [ ] Types: add `deleted_at`, `FlagActivitySummary`, `DeleteResponse`
- [ ] API client: project delete/hardDelete/restore methods
- [ ] API client: app delete/hardDelete/restore methods
- [ ] Project list: gear icon, deleted styling, restore button
- [ ] Application list: gear icon, deleted styling, restore button
- [ ] Project settings: edit form (name, description, repo_url)
- [ ] Project settings: danger zone
- [ ] Application settings: edit form
- [ ] Application settings: danger zone

### Tests
- [ ] Backend: instant delete (no flags)
- [ ] Backend: soft delete (flags, no activity)
- [ ] Backend: delete blocked (active flags)
- [ ] Backend: hard delete (7-day gate)
- [ ] Backend: restore

## Completion Record
<!-- Fill in when phase is set to Complete -->
- **Branch**: ``
- **Committed**: No
- **Pushed**: No
- **CI Checks**:
