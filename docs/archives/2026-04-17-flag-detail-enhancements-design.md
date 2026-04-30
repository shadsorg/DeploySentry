# Flag Detail Page Enhancements

**Date**: 2026-04-17
**Status**: Approved

## Overview

Enhance the FlagDetailPage with a Settings tab for editing flag properties, a History tab showing an audit log of all flag mutations, fix the created_by UUID display, improve header spacing, and reorder tabs.

## Changes

### 1. Tab Reorder

Current: Targeting Rules | Environments | YAML
New: **Environments | Targeting Rules | YAML | Settings | History**

Default active tab changes from `rules` to `environments`.

### 2. Settings Tab

Editable form with all mutable flag properties:
- name
- description
- category (dropdown: release, feature, experiment, ops, permission)
- purpose
- owners (comma-separated text input)
- is_permanent (checkbox)
- expires_at (datetime-local input, shown when not permanent)
- default_value (text input, or true/false dropdown for boolean flags)
- tags (comma-separated text input)

**Not editable**: key (immutable identifier)

Uses existing `flagsApi.update()` / `PUT /flags/:id` endpoint and `UpdateFlagRequest` type. Save button at bottom. On success, update local flag state so the header reflects changes immediately without a page reload.

### 3. Created By — Server-Side Name Resolution

Modify the single-flag GET handler (`GetFlag` in `internal/flags/handler.go`) to LEFT JOIN `users` on `feature_flags.created_by = users.id` and include `created_by_name` in the JSON response.

Changes:
- **Go**: Add `CreatedByName string` field to the flag response (or a wrapper struct). Modify the `getFlag` handler to do a user lookup and attach the name.
- **TypeScript**: Add `created_by_name?: string` to the `Flag` interface.
- **UI**: Display `flag.created_by_name ?? flag.created_by` in the header.

### 4. Header Spacing

The `detail-secondary` div currently renders "Created by X", "Created date", "Updated date" without visual separation. Add a midpoint separator and consistent gap between items. Use the existing `detail-chips` pattern which already has proper spacing, or add explicit CSS gap.

### 5. History Tab — Audit Log

#### Backend: Instrument Flag Mutations

Add audit log writes to all flag mutation handlers in `internal/flags/handler.go`. Each entry writes to the existing `audit_log` table via the existing `AuditLogRepository`.

**Actions to instrument:**

| Handler | Action String | Entity Type | Old/New Value |
|---------|--------------|-------------|---------------|
| createFlag | `flag.created` | `flag` | — / flag JSON |
| updateFlag | `flag.updated` | `flag` | changed fields before/after |
| toggleFlag | `flag.toggled` | `flag` | `{enabled: old}` / `{enabled: new}` |
| archiveFlag | `flag.archived` | `flag` | — |
| setFlagEnvState | `flag.env_state.updated` | `flag` | `{env, enabled: old, value: old}` / `{env, enabled: new, value: new}` |
| addRule | `flag.rule.created` | `flag` | — / rule JSON |
| deleteRule | `flag.rule.deleted` | `flag` | rule JSON / — |
| setRuleEnvState | `flag.rule.env_state.updated` | `flag` | `{rule, env, enabled: old}` / `{rule, env, enabled: new}` |

The `entity_id` is always the flag's UUID so we can filter by flag.

**Wiring**: The flags handler needs access to an audit log writer. Add an `AuditWriter` interface to the handler (similar to how `entityRepo` is injected):

```go
type AuditWriter interface {
    WriteAuditLog(ctx context.Context, entry *models.AuditLogEntry) error
}
```

The existing `AuditLogRepository` will implement this (add a `WriteAuditLog` method to `postgres/audit.go`).

#### Backend: Resolve Actor Names in Audit Response

Modify the `QueryAuditLogs` query to LEFT JOIN `users` on `actor_id` and return `actor_name` in the response. Add `ActorName string` to `AuditLogEntry` model.

#### Frontend: History Tab UI

Call `GET /audit-log` endpoint with `resource_type=flag` and `resource_id={flagId}` filters. The existing endpoint supports `resource_type` but not `resource_id` filtering — add `resource_id` to `AuditLogFilter` and the query handler. Add this API call to `api.ts`.

Note: The `audit_log` table uses `user_id` (not `actor_id`). The existing Go model maps this as `ActorID` — follow that convention.

Render as a chronological list (most recent first):
- Timestamp (formatted datetime)
- Actor name
- Action description (human-readable, e.g. "Enabled flag in Production", "Updated description")
- Before/after values shown as diffs when present

## Files Changed

### Backend
- `internal/flags/handler.go` — Add audit writes to all mutation handlers, add `AuditWriter` dependency, user name lookup for created_by
- `internal/models/audit.go` — Add `ActorName` field
- `internal/platform/database/postgres/audit.go` — Add `WriteAuditLog` method, add actor name join to query
- `cmd/api/main.go` — Wire audit repo into flags handler

### Frontend
- `web/src/pages/FlagDetailPage.tsx` — Reorder tabs, add Settings tab form, add History tab, fix created_by display, fix header spacing
- `web/src/types.ts` — Add `created_by_name` to Flag, add audit log types
- `web/src/api.ts` — Add audit log query function

## Out of Scope
- Flag key editing (immutable by design)
- Targeting rule editing UX improvements
- Flag list page changes
