# Org-Level Audit Page + One-Click Revert Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status**: Design
**Date**: 2026-04-30
**Spec**: [`../specs/2026-04-30-flag-lifecycle-and-org-audit-design.md`](../specs/2026-04-30-flag-lifecycle-and-org-audit-design.md) — Deliverable 3
**Branch**: `feature/org-audit-revert` (off `origin/main`)
**Estimated total**: ~10–14h split across 3 phases + 1 small follow-up

**Goal:** Surface the existing `audit_log` as a first-class org-level page at `/orgs/:orgSlug/audit` with old/new diff per row and a one-click Revert button for revertible actions. Reverts dispatch through a small registry mapping `(entity_type, action) → existing service-layer method`, write their own audit row so the trail stays append-only, and detect races before applying.

**Architecture:** Three thin layers. (1) Backend revert registry — pure mapping, no new revert DSL; each handler is a 5–15 line wrapper around an existing service method. (2) `OrgAuditPage` — mirrors `OrgDeploymentsPage`'s URL-serialized filter pattern, paginated row table, collapsible JSON diff per row. (3) Integration polish — race-detection confirmation, undo-revert toast, non-revertible tooltips. A small follow-up reuses the audit query for the MembersPage security-events panel (UI audit §13).

**Tech Stack:** Go 1.25 + gin + pgx (backend), React 18 + TypeScript + Vite (frontend), existing CSS tokens/glass-panel components.

---

## Reality check vs spec

The spec lists revertible actions assuming a richer audit surface than what's wired today. Before writing handlers, ground the plan in what `audit_log` actually receives:

| Spec entry | Actual action string in `internal/flags/handler.go` | Notes |
|---|---|---|
| `flag.archived` | `flag.archived` (line 548) | Needs new `Unarchive` service method (no restore exists) |
| `flag.created` | `flag.created` (line 230) | Reverts to existing `Archive` |
| `flag.updated` | `flag.updated` (line 494) | Single-field updates only — bail if `old_value` JSON shape can't round-trip |
| `flag.toggled` | `flag.toggled` (line 612) | Reverts via existing toggle |
| `rule.created` | `flag.rule.created` (line 979) | Spec uses `rule.*`; reality is `flag.rule.*` |
| `rule.deleted` | `flag.rule.deleted` (line 1073) | Recreate from `old_value` payload `{"rule_id": "..."}` |
| `rule.updated` | (not currently logged) | Out of scope for this PR |
| `flag.rule.env_state.updated` | (line 1130) | Not in spec table; treat as updateable (apply `old_value`) |
| `flag.env_state.updated` | (line 1212) | Not in spec table; treat as updateable (apply `old_value`) |
| `member.*`, `apikey.*`, `environment.*`, `deployment.*` | **none of these are written today** | Registry will declare them as known non-revertible (or absent); add handlers when those domains start writing audit rows |

**Implication:** The shipped registry handles the flag family only. Other entity types fall through to `ErrNotRevertible`, which the UI tooltip explains. Adding new revert handlers is a one-line registry call when those domains start auditing — no new infrastructure needed.

**Field-name note:** The DB columns are `resource_type` / `resource_id`, but the JSON payload (`AuditLogEntry`) and Go model use `EntityType` / `EntityID` (`internal/models/audit.go:18-19`). Registry keys, API responses, and frontend types should use `entity_type` consistently.

---

## File Structure

**New:**
- `internal/auth/revert_registry.go` — registry type + `RevertHandler` signature + `ErrRevertRace`/`ErrNotRevertible` sentinels
- `internal/auth/revert_handler.go` — `POST /audit-log/:entryId/revert` HTTP handler
- `internal/auth/revert_registry_test.go` — table tests for revertibility map
- `internal/flags/revert.go` — flag-family revert handlers (Archive↔Restore, Create↔Archive, Toggle↔Toggle, Rule create/delete inverses)
- `web/src/pages/OrgAuditPage.tsx` — main page (filter rail + row table + diff)
- `web/src/components/audit/AuditDiff.tsx` — collapsible old/new JSON diff
- `web/src/components/audit/AuditRow.tsx` — single row with When/Who/What/Where/Diff/Actions
- `web/src/components/audit/RevertConfirmDialog.tsx` — race-aware confirmation popover
- `web/src/styles/audit.css` (or extend `globals.css`) — page-specific styles

**Modified:**
- `internal/auth/audit_handler.go:139` — add `Revertible bool` to JSON response per entry; add `/orgs/:orgSlug/audit-log` alias
- `internal/flags/service.go` (or repository.go) — add `Unarchive(ctx, id) error` (clears `archived_at`)
- `cmd/api/main.go:434` — wire registry with flag handlers; mount revert endpoint
- `web/src/api.ts:691-705` — extend `auditApi.query` to accept all filter params + add `revert` method
- `web/src/types.ts:423` — add `revertible: boolean` to `AuditLogEntry`
- `web/src/components/Sidebar.tsx:4-14` — add `{ to: 'audit', icon: 'history_edu', label: 'Audit' }`
- `web/src/App.tsx` — add `<Route path="audit" element={<OrgAuditPage />} />` under the org HierarchyLayout

---

## Phase 1: Backend — Revertibility metadata + revert endpoint

**Goal:** Server can answer "is this entry revertible?" in the existing audit list response and execute a revert via a new endpoint that dispatches through the registry.

### Task 1.1: Add `Unarchive` to flag service

**Files:**
- Modify: `internal/flags/service.go` (new method) and `internal/flags/repository.go` (SQL)
- Test: `internal/flags/service_test.go`

**Context:** The existing flag handler archives via `flagService.Archive(id)` which sets `archived_at = now()`. Reverting `flag.archived` requires the inverse. There's currently no `Unarchive` method.

- [ ] **Step 1: Add `Unarchive(ctx context.Context, id uuid.UUID) error` to the `Service` interface in `internal/flags/service.go`.** Implementation calls `repository.Unarchive(ctx, id)`.

- [ ] **Step 2: Add `Unarchive(ctx, id)` to `internal/flags/repository.go`:**

```go
// Unarchive clears archived_at on a flag. Idempotent for already-active flags.
func (r *PostgresRepository) Unarchive(ctx context.Context, id uuid.UUID) error {
    const q = `UPDATE feature_flags SET archived_at = NULL, updated_at = now() WHERE id = $1`
    _, err := r.pool.Exec(ctx, q, id)
    if err != nil {
        return fmt.Errorf("flags.Unarchive: %w", err)
    }
    return nil
}
```

- [ ] **Step 3: Write unit test in `internal/flags/service_test.go`** that archives then unarchives a flag and asserts `archived_at` is `NULL` after the second call. Run with `go test ./internal/flags/...` and confirm it passes.

- [ ] **Step 4: Commit:** `feat(flags): add Unarchive service method for revert support`

---

### Task 1.2: Define the revert registry

**Files:**
- Create: `internal/auth/revert_registry.go`
- Test: `internal/auth/revert_registry_test.go`

- [ ] **Step 1: Write `internal/auth/revert_registry.go`:**

```go
package auth

import (
    "context"
    "errors"

    "github.com/shadsorg/deploysentry/internal/models"
)

// ErrRevertRace is returned when the current resource state differs from the
// audit entry's NewValue, meaning a revert would clobber a newer change.
var ErrRevertRace = errors.New("resource modified since audit entry; revert would overwrite newer change")

// ErrNotRevertible is returned for actions with no registered handler.
var ErrNotRevertible = errors.New("action is not revertible")

// RevertHandler undoes the action recorded in entry. force=true bypasses race
// detection. On success it returns the action name to write into the new
// audit row (e.g. "flag.archived.reverted"). Handlers are registered per
// (entity_type, action) pair.
type RevertHandler func(ctx context.Context, entry *models.AuditLogEntry, force bool) (newAction string, err error)

// RevertRegistry maps (entity_type, action) → RevertHandler.
type RevertRegistry struct {
    handlers map[string]RevertHandler
}

func NewRevertRegistry() *RevertRegistry {
    return &RevertRegistry{handlers: map[string]RevertHandler{}}
}

func (r *RevertRegistry) Register(entityType, action string, h RevertHandler) {
    r.handlers[key(entityType, action)] = h
}

func (r *RevertRegistry) IsRevertible(entityType, action string) bool {
    _, ok := r.handlers[key(entityType, action)]
    return ok
}

// Revert dispatches the entry's revert handler. Returns the action name to
// record in the new audit row.
func (r *RevertRegistry) Revert(ctx context.Context, entry *models.AuditLogEntry, force bool) (string, error) {
    h, ok := r.handlers[key(entry.EntityType, entry.Action)]
    if !ok {
        return "", ErrNotRevertible
    }
    return h(ctx, entry, force)
}

func key(entityType, action string) string { return entityType + ":" + action }
```

- [ ] **Step 2: Write `internal/auth/revert_registry_test.go`** with three table tests:
  1. `IsRevertible` returns true after Register, false otherwise
  2. `Revert` returns `ErrNotRevertible` for unregistered keys
  3. `Revert` calls the registered handler and returns its `newAction`

  Use a stub handler — no real services. Run `go test ./internal/auth/...`.

- [ ] **Step 3: Commit:** `feat(auth): add revert registry with race + not-revertible sentinels`

---

### Task 1.3: Add flag-family revert handlers

**File:** Create `internal/flags/revert.go`

**Context:** Each handler is a thin wrapper. Race detection: load current resource, compare relevant field to `entry.NewValue`. If they differ and `!force`, return `auth.ErrRevertRace`.

- [ ] **Step 1: Write `internal/flags/revert.go`** with one constructor that returns a slice of `(entityType, action, RevertHandler)` tuples so `cmd/api/main.go` can register them all. Include handlers for:
  - `("flag", "flag.archived")` → load flag; if `current.ArchivedAt == nil`, race; else `service.Unarchive`. Returns `"flag.archived.reverted"`.
  - `("flag", "flag.created")` → load flag; if `current.ArchivedAt != nil`, race (already archived elsewhere); else `service.Archive`. Returns `"flag.created.reverted"`.
  - `("flag", "flag.updated")` → parse `entry.NewValue` and `entry.OldValue` as `map[string]any`; load current flag; for each key in NewValue compare current value — if any differs, race; else apply `OldValue` via `service.Update`. Returns `"flag.updated.reverted"`. If JSON parse fails, return `fmt.Errorf("cannot revert: malformed audit payload")`.
  - `("flag", "flag.toggled")` → parse `entry.NewValue` for the `enabled` bool, compare to current; if equal, toggle back via `service.Toggle`; else race.
  - `("flag", "flag.rule.created")` → parse `entry.NewValue` for the rule UUID; verify the rule still exists (load → if missing, race-as-already-deleted → no-op success); else `service.DeleteRule(ruleID)`. Returns `"flag.rule.created.reverted"`.
  - `("flag", "flag.rule.deleted")` → parse `entry.OldValue` for the full rule struct; if a rule with that ID already exists, race; else `service.CreateRule(...)`. Returns `"flag.rule.deleted.reverted"`.
  - `("flag", "flag.rule.env_state.updated")` → parse `entry.NewValue` and `entry.OldValue` for `{rule_id, environment_id, enabled}`; load current state; if differs from `NewValue`, race; else apply `OldValue` via `service.SetRuleEnvState`. Returns `"flag.rule.env_state.updated.reverted"`.
  - `("flag", "flag.env_state.updated")` → same shape but for default-value-per-env. Apply `OldValue` via `service.SetEnvState`. Returns `"flag.env_state.updated.reverted"`.

- [ ] **Step 2: Each handler reads `entry.NewValue` / `entry.OldValue` (JSON strings).** Use `encoding/json.Unmarshal` into a typed local struct. If unmarshal fails, return `fmt.Errorf("flag.<action>: malformed payload: %w", err)` — the HTTP layer converts that to a 400.

- [ ] **Step 3: Sketch the constructor signature so wiring is clean:**

```go
// FlagRevertHandlers returns the registry tuples for the flag family.
// Pass to RevertRegistry.Register one-by-one.
func FlagRevertHandlers(svc Service) []struct {
    Entity, Action string
    Handler        auth.RevertHandler
} {
    return []struct{ Entity, Action string; Handler auth.RevertHandler }{
        {"flag", "flag.archived", revertFlagArchived(svc)},
        {"flag", "flag.created", revertFlagCreated(svc)},
        // ... etc
    }
}
```

- [ ] **Step 4: Write a minimal table test in `internal/flags/revert_test.go`** for at least three handlers: archived (success), archived (race detected when already unarchived), updated (malformed JSON payload returns error). Use a fake `Service` interface satisfying just the methods the handlers call.

- [ ] **Step 5: Commit:** `feat(flags): add revert handlers for flag-family audit actions`

---

### Task 1.4: Add `Revertible` field to audit list response

**Files:**
- Modify: `internal/models/audit.go` — add `Revertible bool` field with `db:"-"` and `json:"revertible"`
- Modify: `internal/auth/audit_handler.go:127-138` — populate `Revertible` per entry from the registry
- Modify: `internal/auth/audit_handler.go:39-41` — `NewAuditHandler` takes a `*RevertRegistry`

- [ ] **Step 1: Add to `models.AuditLogEntry`:**

```go
Revertible bool `json:"revertible" db:"-"`
```

- [ ] **Step 2: Update `AuditHandler` constructor signature** to accept the registry; store it on the struct. Update `cmd/api/main.go:434` later in Task 1.6.

- [ ] **Step 3: In `queryAuditLog` after fetching `entries`, populate the field:**

```go
for _, e := range entries {
    e.Revertible = h.registry.IsRevertible(e.EntityType, e.Action)
}
```

- [ ] **Step 4: Adjust `internal/auth/audit_handler_test.go` (if present) or add a new test** that verifies a `flag.archived` entry comes back with `revertible: true` when the registry has that handler registered.

- [ ] **Step 5: Commit:** `feat(audit): expose revertible bool on audit log entries`

---

### Task 1.5: Add the revert HTTP endpoint

**File:** Create `internal/auth/revert_handler.go`

- [ ] **Step 1: Write the handler:**

```go
package auth

import (
    "context"
    "encoding/json"
    "errors"
    "net/http"
    "strings"
    "time"

    "github.com/shadsorg/deploysentry/internal/models"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

type RevertWriter interface {
    GetAuditLogEntry(ctx context.Context, id uuid.UUID) (*models.AuditLogEntry, error)
    WriteAuditLog(ctx context.Context, entry *models.AuditLogEntry) error
}

type RevertHandlerHTTP struct {
    registry *RevertRegistry
    repo     RevertWriter
}

func NewRevertHandler(registry *RevertRegistry, repo RevertWriter) *RevertHandlerHTTP {
    return &RevertHandlerHTTP{registry: registry, repo: repo}
}

func (h *RevertHandlerHTTP) RegisterRoutes(rg *gin.RouterGroup) {
    rg.POST("/audit-log/:entryId/revert", h.revert)
}

func (h *RevertHandlerHTTP) revert(c *gin.Context) {
    entryID, err := uuid.Parse(c.Param("entryId"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entry id"})
        return
    }
    var body struct{ Force bool `json:"force"` }
    _ = json.NewDecoder(c.Request.Body).Decode(&body) // body is optional

    entry, err := h.repo.GetAuditLogEntry(c.Request.Context(), entryID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "audit entry not found"})
        return
    }

    // Org isolation: the entry must belong to the caller's org.
    orgIDStr := c.GetString("org_id")
    if orgIDStr == "" || entry.OrgID.String() != orgIDStr {
        c.JSON(http.StatusForbidden, gin.H{"error": "audit entry not in this org"})
        return
    }

    newAction, err := h.registry.Revert(c.Request.Context(), entry, body.Force)
    switch {
    case errors.Is(err, ErrNotRevertible):
        c.JSON(http.StatusBadRequest, gin.H{"error": "this action is not revertible"})
        return
    case errors.Is(err, ErrRevertRace):
        c.JSON(http.StatusConflict, gin.H{
            "error": "resource has changed since this entry; pass {\"force\":true} to revert anyway",
            "code":  "race",
        })
        return
    case err != nil:
        if strings.Contains(err.Error(), "malformed payload") {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "revert failed"})
        return
    }

    // Append a new audit row recording the revert.
    var actorID uuid.UUID
    if uid, ok := c.Get(ContextKeyUserID); ok {
        actorID, _ = uid.(uuid.UUID)
    }
    revertEntry := &models.AuditLogEntry{
        ID:         uuid.New(),
        OrgID:      entry.OrgID,
        ProjectID:  entry.ProjectID,
        ActorID:    actorID,
        Action:     newAction,
        EntityType: entry.EntityType,
        EntityID:   entry.EntityID,
        OldValue:   entry.NewValue,                                  // before revert (current state)
        NewValue:   entry.OldValue,                                  // after revert (restored state)
        IPAddress:  c.ClientIP(),
        UserAgent:  c.GetHeader("User-Agent"),
        CreatedAt:  time.Now(),
    }
    if err := h.repo.WriteAuditLog(c.Request.Context(), revertEntry); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "revert succeeded but audit write failed"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"reverted": true, "audit_entry_id": revertEntry.ID, "action": newAction})
}
```

- [ ] **Step 2: Add `GetAuditLogEntry(ctx, id) (*models.AuditLogEntry, error)`** to `internal/platform/database/postgres/audit.go`. Single-row variant of `QueryAuditLogs` filtered by `id`.

- [ ] **Step 3: Add a unit test** at `internal/auth/revert_handler_test.go` covering: 200 success path, 400 not-revertible, 409 race, 403 cross-org, 404 missing entry. Use a stub `RevertWriter` and a registry with one fake handler.

- [ ] **Step 4: Commit:** `feat(audit): POST /audit-log/:entryId/revert dispatches through registry`

---

### Task 1.6: Wire registry + handlers in `cmd/api/main.go`

**File:** Modify `cmd/api/main.go` around lines 434, 543

- [ ] **Step 1: After `auditRepo := postgres.NewAuditLogRepository(db.Pool)` (line 239), construct the registry and register the flag handlers:**

```go
revertRegistry := auth.NewRevertRegistry()
for _, t := range flags.FlagRevertHandlers(flagService) {
    revertRegistry.Register(t.Entity, t.Action, t.Handler)
}
```

- [ ] **Step 2: Update line 434 to pass the registry to the audit handler:**

```go
auth.NewAuditHandler(auditRepo, revertRegistry).RegisterRoutes(api)
auth.NewRevertHandler(revertRegistry, auditRepo).RegisterRoutes(api)
```

- [ ] **Step 3: Run `go build ./...`** and resolve compile errors. Run `go test ./internal/auth/... ./internal/flags/...` and confirm green.

- [ ] **Step 4: Smoke test by hand:**
  - `make dev-up && make migrate-up && make run-api`
  - Archive a flag via the dashboard (or `curl POST /flags/:id/archive`) → confirm an `audit_log` row appears with `action='flag.archived'`
  - `curl 'localhost:8080/api/v1/audit-log?org_id=<id>'` → confirm `revertible: true` on that entry
  - `curl -X POST 'localhost:8080/api/v1/audit-log/<entryId>/revert'` → confirm 200 + new `flag.archived.reverted` row + flag's `archived_at` is now NULL
  - Archive again → revert with the *original* entry id → confirm 409 race (because current state is `archived`, but the entry's `new_value` matches — actually verify behavior matches expectation; document if not)
  - Pass `{"force":true}` → confirm 200

- [ ] **Step 5: Commit:** `feat(api): wire revert registry with flag handlers + mount revert endpoint`

---

## Phase 2: Frontend — `OrgAuditPage`

**Goal:** A page at `/orgs/:orgSlug/audit` that lists every audit entry for the org with When/Who/What/Where/Diff/Actions columns, URL-serialized filters mirroring `OrgDeploymentsPage`, and a Revert button on revertible rows.

### Task 2.1: Extend `auditApi` and types

**Files:**
- Modify: `web/src/types.ts:423`
- Modify: `web/src/api.ts:691-705`

- [ ] **Step 1: Update `AuditLogEntry`:**

```ts
export interface AuditLogEntry {
  id: string;
  org_id: string;
  project_id: string;
  actor_id: string;
  actor_name: string;
  action: string;
  entity_type: string;
  entity_id: string;
  old_value: string;
  new_value: string;
  ip_address?: string;
  user_agent?: string;
  created_at: string;
  revertible: boolean;
}
```

- [ ] **Step 2: Replace `auditApi` in `web/src/api.ts`:**

```ts
export interface AuditQueryParams {
  action?: string;
  entity_type?: string;
  project_id?: string;
  user_id?: string;
  resource_id?: string;
  start_date?: string; // RFC3339
  end_date?: string;   // RFC3339
  limit?: number;
  offset?: number;
}

export const auditApi = {
  query: (params: AuditQueryParams = {}) => {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([k, v]) => { if (v !== undefined && v !== '') qs.set(k, String(v)); });
    return request<{ entries: AuditLogEntry[]; total: number; limit: number; offset: number }>(
      `/audit-log?${qs.toString()}`,
    );
  },

  revert: (entryId: string, force = false) =>
    request<{ reverted: true; audit_entry_id: string; action: string }>(
      `/audit-log/${entryId}/revert`,
      { method: 'POST', body: JSON.stringify({ force }) },
    ),
};
```

- [ ] **Step 3: Run `npx tsc --noEmit` from `web/`. Confirm no errors propagating from the existing `FlagDetailPage` audit usage at `web/src/pages/FlagDetailPage.tsx:178` and `:41`.

- [ ] **Step 4: Commit:** `feat(web): extend auditApi with full filter set + revert action`

---

### Task 2.2: Diff and row components

**Files:**
- Create: `web/src/components/audit/AuditDiff.tsx`
- Create: `web/src/components/audit/AuditRow.tsx`

- [ ] **Step 1: `AuditDiff.tsx`** — collapsible disclosure rendering two `<pre>` blocks (old / new) side-by-side at ≥768px, stacked below. Pretty-print with `JSON.stringify(JSON.parse(value), null, 2)` wrapped in `try/catch` (raw value shown when not valid JSON). No semantic diff in v1; per spec out-of-scope.

- [ ] **Step 2: `AuditRow.tsx`** — accepts `entry: AuditLogEntry`, `onRevert: (entry) => void`, `actionLabel(entry) => string` (humanizes `flag.archived` → `Archived flag enable-dark-mode`). Layout (CSS grid):

```
┌───────────────────────────────────────────────────────────────────┐
│ When  │ Who         │ What                  │ Where    │ Actions │
├───────┼─────────────┼───────────────────────┼──────────┼─────────┤
│ 2m ago│ Alice (a@x) │ Archived flag dark-mode│ proj/app │ [Revert]│
└───────────────────────────────────────────────────────────────────┘
       └─ click row → expands to show <AuditDiff /> below ─┘
```

  - When: relative time (e.g. `2m ago`) with ISO tooltip on hover
  - Who: `actor_name` + email if available; falls back to actor UUID
  - What: humanized action via `actionLabel(entry)` — see Step 3
  - Where: project/app/env breadcrumb if `entity_type` indicates a flag/rule (resolve via cached `projects`/`apps`/`envs` lists passed in via props); empty cell otherwise
  - Actions: `Revert` button when `entry.revertible === true`; otherwise a tooltip-only `cannot-revert` icon with reason from a small map:

```ts
const NON_REVERTIBLE_REASONS: Record<string, string> = {
  'flag.hard_deleted': 'Hard-deleted flags cannot be restored',
  'apikey.created': 'API keys are sensitive — create a new one instead',
  'apikey.revoked': 'API keys are sensitive — create a new one instead',
  // Generic fallback handled inline.
};
```

- [ ] **Step 3: Add `actionLabel` helper in `web/src/components/audit/labels.ts`:**

```ts
export function actionLabel(entry: AuditLogEntry): string {
  switch (entry.action) {
    case 'flag.archived': return `Archived flag`;
    case 'flag.created':  return `Created flag`;
    case 'flag.updated':  return `Updated flag`;
    case 'flag.toggled':  return `Toggled flag`;
    case 'flag.rule.created':           return `Added rule`;
    case 'flag.rule.deleted':           return `Removed rule`;
    case 'flag.rule.env_state.updated': return `Changed rule env state`;
    case 'flag.env_state.updated':      return `Changed env default value`;
    default: return entry.action;
  }
}
```

  Note: looking up the flag key for the `<resource>` chip belongs to the page-level resolver (Task 2.3), not the row.

- [ ] **Step 4: Commit:** `feat(web): add AuditDiff + AuditRow components`

---

### Task 2.3: `OrgAuditPage` with filter rail

**File:** Create `web/src/pages/OrgAuditPage.tsx`

**Reference pattern:** `OrgDeploymentsPage.tsx` — filter rail on the left, paginated table on the right, URL-serialized filter state via `useSearchParams`.

- [ ] **Step 1: Page skeleton — copy the `Filters` interface, `useSearchParams` plumbing, and `setFilter`/`resetFilters` helpers from `OrgDeploymentsPage.tsx:24-145`. Adapt to:**

```ts
interface Filters {
  action: string;        // exact match (free-text input or select once we have a known set)
  entity_type: string;   // 'flag' | 'rule' | '' (start with these two)
  project_id: string;
  user_id: string;       // populated by org-members select
  start_date: string;    // datetime-local; converted to RFC3339 on submit
  end_date: string;
}
const ENTITY_TYPE_OPTIONS = ['', 'flag', 'rule'];
```

- [ ] **Step 2: Filter rail (mirrors `OrgDeploymentsPage` aside).** Selects for entity type, project, user; date-range pickers for start/end; free-text input for action.

- [ ] **Step 3: Main panel.** Header card titled "Activity Stream" with row count badge and `sync` spinner during loading. Table body: rows from `auditApi.query` paginated by 50, "Load more" button when `entries.length < total`.

- [ ] **Step 4: Hook up `auditApi.query` with the filters serialized into RFC3339 dates and pass `entity_type` rather than `resource_type` (the spec uses `resource_type`, but the JSON / Go model use `entity_type` — keep the wire and UI aligned).**

  ```ts
  // ⚠️ Reality alignment: the existing audit_handler reads `resource_type` from
  // the query string (audit_handler.go:70). Frontend filter param must be
  // `entity_type` for our UI but POSTed as `resource_type`. Two options:
  //   1. Change the handler to accept `entity_type` (preferred — the JSON /
  //      model already use that name; the handler is the only `resource_type`
  //      reference left).
  //   2. Keep handler as-is; translate in api.ts before sending.
  // Pick option 1 in this task — single rename in audit_handler.go:70 and
  // propagated through filter struct.
  ```

  Adjust `internal/auth/audit_handler.go:70` from `c.Query("resource_type")` to `c.Query("entity_type")`. Update the `AuditLogFilter.ResourceType` field name to `EntityType` for consistency. (DB column stays `resource_type` — only the API surface renames.)

- [ ] **Step 5: Resolve project / app / env breadcrumbs client-side.** Load `entitiesApi.listProjects(orgSlug)` once on mount; resolve `entity_id` against the list when an audit row's `entity_type === 'flag'` (or whatever the row references). For v1, the "Where" column shows the project name only when the audit entry's `project_id` is non-zero; deeper resolution is a polish follow-up.

- [ ] **Step 6: Wire the Revert button to a confirmation popover (built in Task 3.1) that, on confirm, calls `auditApi.revert(entry.id)`, refreshes the page (or optimistically inserts the new `<action>.reverted` row at the top), and toasts.**

- [ ] **Step 7: Add to `web/src/App.tsx` under the org HierarchyLayout:**

```tsx
<Route path="audit" element={<OrgAuditPage />} />
```

- [ ] **Step 8: Add to `web/src/components/Sidebar.tsx:4-14` between `api-keys` and `strategies`:**

```ts
{ to: 'audit', icon: 'history_edu', label: 'Audit' },
```

- [ ] **Step 9: Run `npm run build` from `web/`. Confirm clean. Run the dev server (`make run-web`), log in to a populated org, navigate to `/orgs/<slug>/audit`, and verify rows render with the diff expander and Revert button on flag rows.**

- [ ] **Step 10: Commit:** `feat(web): add OrgAuditPage with URL-serialized filter rail`

---

### Task 2.4: Style polish

**File:** Extend `web/src/styles/globals.css` (or new `web/src/styles/audit.css` imported from the page)

- [ ] **Step 1: Add `.audit-grid` (CSS grid for row columns), `.audit-row`, `.audit-row.expanded`, `.audit-diff-pre`, `.audit-revert-btn` styles. Reuse glass-panel tokens; keep the page visually consistent with `OrgDeploymentsPage`.

- [ ] **Step 2: Visually compare side-by-side with `/orgs/<slug>/deployments` to ensure the layouts feel like siblings.

- [ ] **Step 3: Commit:** `feat(web): style OrgAuditPage rows + diff expander`

---

## Phase 3: Integration polish

**Goal:** Race-aware confirmation, success toast with `Undo Revert`, and cross-cutting tests.

### Task 3.1: `RevertConfirmDialog` with race handling

**File:** Create `web/src/components/audit/RevertConfirmDialog.tsx`

- [ ] **Step 1: Build a small popover/modal that opens from the Revert button. Body shows the `actionLabel(entry)` summary plus a highlighted `<AuditDiff old={current} new={old_value} />` preview (so the user sees the post-revert state). Confirm button calls `auditApi.revert(entry.id, false)`.**

- [ ] **Step 2: On 409 response (`code: "race"`), swap the dialog body to a warning state: "This entry's resource has been modified since the change was made. Reverting will overwrite the newer change." Add a "Revert anyway" button that calls `auditApi.revert(entry.id, true)`.**

- [ ] **Step 3: On 200, close, fire an "ok" toast via the existing toast system (`web/src/components/Toast.tsx` if present — search; otherwise reuse the dashboard's existing notification primitive).**

- [ ] **Step 4: Toast includes an `Undo Revert` link that calls `auditApi.revert(<the new entry id from the response>)` — recursion is fine, it's just another revert from the registry's perspective.**

- [ ] **Step 5: Commit:** `feat(web): add race-aware RevertConfirmDialog with undo-revert toast`

---

### Task 3.2: Tooltip for non-revertible rows

**File:** Modify `web/src/components/audit/AuditRow.tsx`

- [ ] **Step 1: When `entry.revertible === false`, replace the Revert button slot with a muted `lock` icon and a `title=`/tooltip drawn from the `NON_REVERTIBLE_REASONS` map (Task 2.2 Step 2). Default fallback: "Cannot revert this action".**

- [ ] **Step 2: Visually verify against a `flag.hard_deleted` entry (manually insert one via SQL if Deliverable 2 isn't yet shipped) and against a `deployment.created` entry.**

- [ ] **Step 3: Commit:** `feat(web): tooltip on non-revertible audit rows`

---

### Task 3.3: End-to-end smoke

- [ ] **Step 1: Manual checklist (no Playwright in this PR — keep scope tight):**
  - As an org `owner`, the Audit sidebar entry is visible
  - As `viewer` or `developer`, the Audit entry is hidden (route still renders if URL is typed; the page itself returns 200 from the API since the existing handler doesn't enforce role beyond `org_id` — flag this as a follow-up; for v1 hide it from the sidebar only)
  - Filter by entity_type=flag, project, user, date range — URL updates and filtered list returns
  - Click an entry → diff expands with old/new
  - Click Revert on a `flag.archived` entry → confirm popover → confirm → toast → flag is unarchived in the dashboard
  - Click "Undo Revert" in the toast → flag is archived again, new audit row appears
  - Try Revert on a `flag.toggled` entry where the flag has been toggled twice since → 409 "race" → confirm via "Revert anyway"
  - Refresh the page → all reverts and original entries are visible, append-only

- [ ] **Step 2: Commit:** `chore(web): manual smoke checklist for OrgAuditPage`

---

### Task 3.4: Verification before declaring done

- [ ] `go test ./...` — green
- [ ] `npm run build` (web) — clean
- [ ] `npm run lint` (web) — clean, max-warnings 0
- [ ] `tsc --noEmit` (web) — clean
- [ ] Manual smoke from Task 3.3 — all bullets pass
- [ ] CI on the PR — Lint, Lint UI, E2E SDK Tests, Test all green

---

## Follow-up: MembersPage security events feed (UI audit §13)

Once `OrgAuditPage` ships, §13 is a thin filtered view on top of the same data. File a separate small plan after this lands.

**Sketch (~1h):**
1. Add a `<RecentSecurityEvents />` panel to `web/src/pages/MembersPage.tsx`.
2. The panel calls `auditApi.query({ entity_type: 'user' /* or whatever member events use */, limit: 10 })`.
3. Render the rows in a compact list (no diff, no revert button — link to the full Audit page for details).
4. Caveat: requires the auth/login domain to start writing audit rows for `login`, `logout`, `member.role_changed`, `member.added`, `member.removed`. Today none of those are written. The panel shows an "Audit logging for membership events not yet wired — see #<issue>" empty state until those handlers exist.

**Decision needed before implementing:** wire the audit writes in `internal/auth/login_handler.go` and `internal/members/handler.go` first (small change; ~30 min each), or ship the empty-state panel and backfill audit writes as a separate piece of work? Recommend: write the audit calls during this follow-up so the panel has real data on day one.

---

## Out of scope (call out and don't do)

- **Hard-delete revert** (`flag.hard_deleted`) — depends on Deliverable 2's tombstone/restore. Registry leaves it absent; UI tooltip handles it.
- **Bulk revert** — single-row only, per spec.
- **Semantic diff** — pretty-printed JSON only, per spec.
- **Audit-log retention / archival** — separate concern.
- **Member / API key / environment / deployment audit logging** — those domains don't write audit rows today; only flag-family handlers ship.
- **Pre-flight cascade preview** for hard delete — Deliverable 2 territory.
- **Playwright e2e** for the audit page — manual smoke only in this PR; add when the page stabilizes.

---

## Risks & callouts (from spec, mapped to plan)

| Risk | Mitigation |
|---|---|
| Cascade scope on hard delete | N/A here — Deliverable 2 |
| Revert race | Per-handler load-and-compare in Task 1.3; 409 path in Task 3.1 with explicit "Revert anyway" |
| Sensitive data in `old_value` | Existing audit conventions already redact; nothing new in this PR. |
| Revertibility drift | Add a `TestKnownActionsHaveRevertibilityDecision` in `internal/auth/revert_registry_test.go` that asserts every action string emitted by `internal/flags/handler.go` is either registered or explicitly listed in a `KNOWN_NON_REVERTIBLE` set. CI will fail if a new audit action lands without a decision. |
| `entity_type` vs `resource_type` mismatch | Renamed in Task 2.3 Step 4 — handler reads `entity_type`, DB column stays `resource_type`. Single rename, kept consistent across UI and JSON. |

---

## Self-review notes

- All 8 actions in `internal/flags/handler.go` have either a registered handler (Task 1.3) or an explicit non-handler decision in the test (Task 1.4 Step 4 / Risks table).
- Filter UI matches `OrgDeploymentsPage.tsx:48-145` pattern exactly — no novel state shape.
- No placeholder steps; every step names the file, the change, and the verification.
- Race-detection logic spelled out per handler (Task 1.3 Step 1) — no "handle edge cases" hand-waving.
- The `entity_type` rename note in Task 2.3 Step 4 was caught during plan-write; without it the filter would silently 400.
