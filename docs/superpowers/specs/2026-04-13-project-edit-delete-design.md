# Project Edit & Delete

**Phase**: Design

## Overview

Add full edit and soft-delete/hard-delete capabilities for projects, both at the API and web UI layers. Currently, the update endpoint only persists the `name` field and no delete functionality exists.

## Requirements

### Edit
- Fix the repository to persist `name`, `description`, and `repo_url` (all three are already accepted by the handler but only `name` is saved).
- Fix `GetProjectBySlug` and `ListProjectsByOrg` queries to select all columns (`description`, `repo_url`, `settings`).
- No slug changes allowed.

### Soft Delete
- Add `deleted_at TIMESTAMPTZ` column to the `projects` table.
- Soft delete sets `deleted_at = now()`.
- **Blocked** if any flags in the project have evaluations in the last 14 days (checked via `flag_evaluation_log`). Returns `409 Conflict` with a list of the active flags (key, name, last evaluation time).
- Soft-deleted projects are hidden from the list by default; shown when `?include_deleted=true` is passed.
- Soft-deleted projects can be restored (clears `deleted_at`).

### Hard Delete
- Available only after a project has been soft-deleted for at least 7 days.
- Performs a real `DELETE` (cascades to apps, flags, etc. via existing FK constraints).
- Returns `409 Conflict` with `eligible_at` date if attempted too early.

### Permissions
- Edit, soft delete, restore: `PermProjectManage`
- Hard delete: `PermOrgManage` (higher privilege)

## API Design

### Existing (fixed)
- `PUT /api/v1/orgs/:orgSlug/projects/:projectSlug` — now persists `name`, `description`, `repo_url`.

### New Endpoints
| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| DELETE | `/api/v1/orgs/:orgSlug/projects/:projectSlug` | PermProjectManage | Soft delete |
| DELETE | `/api/v1/orgs/:orgSlug/projects/:projectSlug/permanent` | PermOrgManage | Hard delete (7-day gate) |
| POST | `/api/v1/orgs/:orgSlug/projects/:projectSlug/restore` | PermProjectManage | Restore soft-deleted project |

### Error Responses

**Soft delete blocked (409):**
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

### Migration: Add `deleted_at` to projects
```sql
ALTER TABLE projects ADD COLUMN deleted_at TIMESTAMPTZ;
```

## Backend Changes

### Model (`internal/models/project.go`)
- Add `DeletedAt *time.Time` field with `json:"deleted_at,omitempty"`.

### Repository (`internal/platform/database/postgres/entities.go`)

**Fix existing:**
- `UpdateProject` — update `name`, `description`, `repo_url` (not just `name`).
- `GetProjectBySlug` — select all columns including `description`, `repo_url`, `settings`.
- `ListProjectsByOrg` — select all columns; add `includeDeleted bool` param; filter `deleted_at IS NULL` when false.

**New methods:**
- `SoftDeleteProject(ctx, id uuid.UUID) error` — `UPDATE projects SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`.
- `HardDeleteProject(ctx, id uuid.UUID) error` — `DELETE FROM projects WHERE id = $1 AND deleted_at <= now() - interval '7 days'`.
- `RestoreProject(ctx, id uuid.UUID) error` — `UPDATE projects SET deleted_at = NULL WHERE id = $1 AND deleted_at IS NOT NULL`.

**New flag query (flag repository):**
- `HasRecentFlagActivity(ctx, projectID uuid.UUID, since time.Time) ([]FlagActivitySummary, error)` — joins `feature_flags` with `flag_evaluation_log` to find flags with evaluations after `since`. Returns `[]FlagActivitySummary{Key, Name, LastEvaluated}`.

### Repository Interface (`internal/entities/repository.go`)
- Update `EntityRepository` interface with new method signatures.
- Add `FlagActivityChecker` interface (or extend existing flag repo interface) for `HasRecentFlagActivity`.

### Service (`internal/entities/service.go`)

**New methods:**
- `SoftDeleteProject(ctx, orgID, projectSlug)` — looks up project, calls `HasRecentFlagActivity` with 14-day window, returns error with active flags if any, otherwise calls repo `SoftDeleteProject`.
- `HardDeleteProject(ctx, orgID, projectSlug)` — looks up project (including deleted), verifies `deleted_at` is 7+ days old, calls repo `HardDeleteProject`.
- `RestoreProject(ctx, orgID, projectSlug)` — looks up project (including deleted), calls repo `RestoreProject`.

**Constants:**
```go
const recentActivityWindow = 14 * 24 * time.Hour
const hardDeleteEligibleAfter = 7 * 24 * time.Hour
```

### Handler (`internal/entities/handler.go`)
- Register three new routes (DELETE, DELETE /permanent, POST /restore).
- `softDeleteProject` handler — calls service, returns 409 with active flags or 204 on success.
- `hardDeleteProject` handler — calls service, returns 409 with eligible date or 204 on success.
- `restoreProject` handler — calls service, returns 200 with restored project.
- Update `listProjects` to read `?include_deleted=true` query param.

## Web UI Changes

### Project List Page (`web/src/pages/ProjectListPage.tsx`)

**Project cards:**
- Add a gear icon to each project card linking to the project settings page.
- Soft-deleted projects: dimmed/grayed-out card with a "Deleted" badge, a "Restore" button, and text: "Hard delete available on MM/DD/YYYY" (computed from `deleted_at + 7 days`).
- Fetch with `?include_deleted=true` to show deleted projects in the list.

### Project Settings Page (`web/src/pages/SettingsPage.tsx`)

**Edit form (project-level "General" tab):**
- The existing project settings form should allow editing `name`, `description`, and `repo_url` with a save button. (Verify current state — may already exist partially.)

**Danger Zone (project-level):**
- Add a danger zone section at the bottom of the project settings page (follows existing app-level pattern with `.danger-zone` CSS class).
- **When flags have recent activity:** Show a warning banner listing the active flags (name, last evaluation date). Delete button is disabled.
- **When no blockers:** Show a confirmation dialog before soft delete. Dialog text: "This will soft-delete the project. It can be restored within 7 days."
- **When project is soft-deleted and 7+ days old:** Show a hard-delete button with confirmation dialog. Dialog text: "This will permanently delete the project and all associated data. This cannot be undone."

## Out of Scope
- CLI changes
- Slug editing
- Automated hard-delete cleanup (background job)
- Notifications on delete

## Checklist
- [ ] Migration: add `deleted_at` column
- [ ] Model: add `DeletedAt` field
- [ ] Fix repo `UpdateProject` to persist all fields
- [ ] Fix repo `GetProjectBySlug` / `ListProjectsByOrg` to select all columns
- [ ] Repo: `SoftDeleteProject`, `HardDeleteProject`, `RestoreProject`
- [ ] Repo: `HasRecentFlagActivity` query
- [ ] Update repository interface
- [ ] Service: `SoftDeleteProject` with activity check
- [ ] Service: `HardDeleteProject` with 7-day gate
- [ ] Service: `RestoreProject`
- [ ] Handler: soft delete, hard delete, restore routes
- [ ] Handler: `listProjects` include_deleted param
- [ ] Web: gear icon on project cards
- [ ] Web: soft-deleted card styling (dimmed, badge, restore, hard-delete date)
- [ ] Web: project settings edit form (name, description, repo_url)
- [ ] Web: danger zone with activity check warning
- [ ] Web: soft delete confirmation dialog
- [ ] Web: hard delete button and confirmation dialog
- [ ] Tests for all new backend functionality

## Completion Record
<!-- Fill in when phase is set to Complete -->
- **Branch**: ``
- **Committed**: No
- **Pushed**: No
- **CI Checks**: 
