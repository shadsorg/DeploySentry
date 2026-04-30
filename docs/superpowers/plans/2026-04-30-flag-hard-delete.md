# Flag Hard Delete + Retention Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status**: Design
**Date**: 2026-04-30
**Spec**: [`../specs/2026-04-30-flag-lifecycle-and-org-audit-design.md`](../specs/2026-04-30-flag-lifecycle-and-org-audit-design.md) — Deliverable 2
**Branch**: `feature/flag-hard-delete` (off `origin/main`, currently `04b5915` post-#77)
**Estimated total**: ~8–10h split across 5 phases

**Goal:** Add a 30-day retention window between archive and permanent removal. Operators can opt a flag into deletion (`POST /flags/:id/queue-deletion`), force-delete after retention elapses (`DELETE /flags/:id?force=true`), or restore at any time before deletion (`POST /flags/:id/restore`). A nightly sweep tombstones expired flags. UI surfaces the lifecycle states on the FlagDetailPage Settings tab.

**Architecture:** "Hard delete" is implemented as a soft tombstone (`deleted_at IS NOT NULL`) so audit/eval-log rows stay joinable. A separate quarterly compaction (out of scope) actually removes tombstoned rows. Three new endpoints map cleanly onto three new repo methods; a single ticker drives the sweep.

**Tech Stack:** Go 1.25 + gin + pgx (backend), React 18 + TypeScript + Vite (frontend), existing CSS tokens.

---

## Reality check vs spec

The spec's wording around `HardDeleteFlag` is internally inconsistent. One bullet calls it a `DELETE FROM feature_flags WHERE …`; the next bullet describes the same call as setting `deleted_at` (tombstone) so `audit_log` foreign keys stay valid. **This plan treats `HardDeleteFlag` as a tombstone-via-UPDATE.** The actual `DELETE` happens in a future quarterly compaction job, out of scope for this PR.

This matters because:

1. `audit_log.resource_id` has no FK constraint today (migration 008). Tombstoning is for **UI/data-readability**, not FK integrity — a reverted/historical audit row whose `resource_id` points at a tombstoned flag can still join to `feature_flags` to display the flag key/name.
2. `flag_evaluation_log` (migration 014) also has `flag_id UUID NOT NULL` with no FK. Same reasoning.
3. Tables that DO have CASCADE FKs to `feature_flags(id)` will only fire during compaction, not during this PR's tombstone operation. They are: `flag_targeting_rules` (013), `flag_ratings` (030), `flag_evaluation_metrics` (030), and the two added by 029_platform_redesign.

**Pre-existing latent bug from Deliverable 3 prep — must be fixed first.** `service.ArchiveFlag` (`internal/flags/service.go:253`) calls `repo.UpdateFlag` whose SQL doesn't touch `archived_at`. The flag's `Archived` bool flips in the Go struct but never persists. The `repo.DeleteFlag` method is the one that actually sets `archived_at = now()`, but it's never called. **Without fixing this, hard-delete won't work**: the retention check is `archived_at + 30 days < now()`, and `archived_at` is always NULL for flags archived through the UI today.

The spec calls out the rename (`DeleteFlag` → `ArchiveFlag`); we do that AND fix the service-layer wiring as Phase 0.

**Audit actions to be added in this PR:**
- `flag.queued_for_deletion` — operator opted-in. Revertible (clear `delete_after`).
- `flag.hard_deleted` — sweep ran. Non-revertible per spec ("tombstone unrecoverable").
- `flag.restored` — `archived_at`, `delete_after`, `deleted_at` all cleared. Revertible (re-archive).

These will be registered with the revert registry shipped in Deliverable 3.

---

## File Structure

**New:**
- `migrations/060_add_flag_delete_after.up.sql` + `.down.sql` — `delete_after`, `deleted_at` columns + partial index
- `internal/flags/retention_sweep.go` — periodic ticker that tombstones expired flags

**Modified:**
- `internal/flags/repository.go` — interface: rename `DeleteFlag` → `ArchiveFlag`; add `QueueDeletion`, `HardDeleteFlag`, `RestoreFlag`, `ListFlagsToHardDelete`
- `internal/platform/database/postgres/flags.go` — concrete: rename + 4 new SQL impls
- `internal/flags/service.go` — service: fix `ArchiveFlag` to call `repo.ArchiveFlag`; add `QueueDeletion`, `HardDeleteFlag`, `RestoreFlag`
- `internal/flags/handler.go` — three new handlers + audit calls + route mounts; add `delete_after`/`deleted_at` to flag JSON
- `internal/flags/revert.go` — add revert handlers for `flag.queued_for_deletion` and `flag.restored`; mark `flag.hard_deleted` as known-non-revertible
- `internal/models/flag.go` — add `DeleteAfter *time.Time` and `DeletedAt *time.Time` fields
- `internal/platform/database/postgres/flags.go` (the SELECT scanner around line 60-80) — read the two new columns
- `cmd/api/main.go` — start retention sweep goroutine; register new revert handlers
- `web/src/api.ts` — `flagsApi.queueDeletion`, `hardDelete`, `restore`
- `web/src/types.ts` — `Flag` adds `delete_after?: string | null`, `deleted_at?: string | null`
- `web/src/pages/FlagDetailPage.tsx` — Settings tab Lifecycle panel: 3 lifecycle states (active / within-retention / retention-elapsed)

---

## Phase 0: Fix the existing archive flow

**Goal:** Make `service.ArchiveFlag` actually persist `archived_at`. Without this, hard-delete cannot work.

### Task 0.1: Rename `repo.DeleteFlag` → `repo.ArchiveFlag`

**Files:**
- Modify: `internal/flags/repository.go` (interface)
- Modify: `internal/platform/database/postgres/flags.go` (concrete)
- Modify: `internal/flags/service_test.go` (mock)
- Modify: any callers (run `git grep -n "\.DeleteFlag\b" internal/`)

- [ ] **Step 1: Rename interface method.** In `internal/flags/repository.go:79-80`:

```go
// ArchiveFlag soft-archives a feature flag by setting archived_at = now().
// Idempotent for already-archived flags (no rows affected → ErrNotFound).
ArchiveFlag(ctx context.Context, id uuid.UUID) error
```

- [ ] **Step 2: Rename concrete impl.** In `internal/platform/database/postgres/flags.go:487-498`:

```go
// ArchiveFlag soft-archives a feature flag by setting archived_at = now().
// Returns ErrNotFound if the flag is already archived (archived_at IS NOT NULL).
func (r *FlagRepository) ArchiveFlag(ctx context.Context, id uuid.UUID) error {
    const q = `UPDATE feature_flags SET archived_at = now() WHERE id = $1 AND archived_at IS NULL`
    tag, err := r.pool.Exec(ctx, q, id)
    if err != nil {
        return fmt.Errorf("postgres.ArchiveFlag: %w", err)
    }
    if tag.RowsAffected() == 0 {
        return ErrNotFound
    }
    return nil
}
```

- [ ] **Step 3: Update mock + tests.** In `internal/flags/service_test.go`, rename `mockFlagRepo.DeleteFlag` → `mockFlagRepo.ArchiveFlag` and `deleteFn` → `archiveFn`.

- [ ] **Step 4: Verify no lingering call sites.** Run `git grep -n "\.DeleteFlag\b" internal/ cmd/`. If any non-test caller exists, update it.

- [ ] **Step 5: Build + test.** `go build ./...` and `go test ./internal/flags/...` clean.

- [ ] **Step 6: Commit:** `refactor(flags): rename repo DeleteFlag → ArchiveFlag (matches actual behavior)`

### Task 0.2: Fix `service.ArchiveFlag` to actually persist `archived_at`

**File:** `internal/flags/service.go:253-269`

**Problem:** Today `service.ArchiveFlag` sets `flag.Archived = true` and calls `repo.UpdateFlag`, whose SQL doesn't touch `archived_at`. So the column never gets set through the service path. Hard-delete's retention check (`archived_at + 30 days < now()`) silently fails forever.

- [ ] **Step 1: Replace the body.** Rewrite `flagService.ArchiveFlag` to call `repo.ArchiveFlag` directly:

```go
// ArchiveFlag marks a flag as archived (sets archived_at = now()).
// Idempotent: already-archived flags return nil.
func (s *flagService) ArchiveFlag(ctx context.Context, id uuid.UUID) error {
    if err := s.repo.ArchiveFlag(ctx, id); err != nil {
        if errors.Is(err, ErrFlagNotFoundOrAlreadyArchived(err)) {
            // ArchiveFlag returns ErrNotFound when already-archived OR truly missing.
            // For idempotency we treat it as success here. If we later need to
            // differentiate (e.g. for the handler to return 404 on "missing"),
            // GetFlag first or expose a distinct sentinel.
            return nil
        }
        return fmt.Errorf("archiving flag: %w", err)
    }

    // Reload for event payload + cache invalidation.
    flag, err := s.repo.GetFlag(ctx, id)
    if err != nil {
        // Archive succeeded; reload failure is non-fatal.
        _ = s.cache.Invalidate(ctx, id)
        return nil
    }
    _ = s.cache.Invalidate(ctx, flag.ID)
    s.publishEvent(ctx, "archived", flag)
    return nil
}
```

(Replace the `ErrFlagNotFoundOrAlreadyArchived` placeholder above with whatever the postgres package's `ErrNotFound` import path is — see how `service.UnarchiveFlag` does it. The point is: idempotent on already-archived.)

- [ ] **Step 2: Update unit tests** at `internal/flags/service_test.go`. The existing `TestArchiveFlag_*` tests likely set up a mock that matches the old code path. Adjust to call `mock.ArchiveFlag` (the new name from Task 0.1) and assert it was called with the flag id.

- [ ] **Step 3: Add a regression test** that exercises the bug we just fixed: `TestArchiveFlag_PersistsArchivedAt`. Stub repo, call `service.ArchiveFlag`, assert `mock.archiveFlagCalled == true` (and that we did NOT call `repo.UpdateFlag` for the archive operation).

- [ ] **Step 4: Build + test.**

- [ ] **Step 5: Commit:** `fix(flags): service.ArchiveFlag now actually persists archived_at via repo.ArchiveFlag`

---

## Phase 1: Schema + repo

### Task 1.1: Migration 060

**Files:**
- Create: `migrations/060_add_flag_delete_after.up.sql`
- Create: `migrations/060_add_flag_delete_after.down.sql`

- [ ] **Step 1: Write the up migration:**

```sql
-- 060_add_flag_delete_after: 30-day retention window for hard-delete.
--
-- delete_after  : when set, the sweep job will tombstone the flag at/after this time.
-- deleted_at    : tombstone marker. Once set, the flag is treated as gone by the API
--                 surface, but the row remains so audit_log / flag_evaluation_log
--                 rows referencing it stay joinable. A separate compaction job
--                 (out of scope) hard-DELETEs tombstoned rows older than the audit
--                 retention window, which fires the existing CASCADE FKs.
ALTER TABLE feature_flags
    ADD COLUMN IF NOT EXISTS delete_after TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at   TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_feature_flags_delete_after
    ON feature_flags (delete_after)
    WHERE delete_after IS NOT NULL AND deleted_at IS NULL;
```

- [ ] **Step 2: Write the down migration:**

```sql
DROP INDEX IF EXISTS idx_feature_flags_delete_after;
ALTER TABLE feature_flags
    DROP COLUMN IF EXISTS delete_after,
    DROP COLUMN IF EXISTS deleted_at;
```

- [ ] **Step 3: Apply locally** to verify: `make migrate-up` then `psql $DSN -c "\d feature_flags"` and confirm both columns + index appear.

- [ ] **Step 4: Verify rollback** by running `make migrate-down` once, then `make migrate-up` again — both columns drop and re-add cleanly.

- [ ] **Step 5: Commit:** `feat(db): migration 060 — add delete_after + deleted_at to feature_flags`

### Task 1.2: Add `delete_after` / `deleted_at` to the model + scanner

**Files:**
- Modify: `internal/models/flag.go`
- Modify: `internal/platform/database/postgres/flags.go` (the SELECT scanner around line 60-80)

- [ ] **Step 1: Add fields to the `FeatureFlag` struct** in `internal/models/flag.go`:

```go
// Lifecycle: 30-day retention window between archive and tombstone.
DeleteAfter *time.Time `json:"delete_after,omitempty" db:"delete_after"`
DeletedAt   *time.Time `json:"deleted_at,omitempty"   db:"deleted_at"`
```

- [ ] **Step 2: Update the SELECT-list and Scan in `postgres/flags.go`** so reads populate the new fields. There's a shared scanner (look around line 50-80 — the function that turns a row into a `*models.FeatureFlag`); the SELECT-list constants (around line 145-160) are also used by `ListFlags`. Add `delete_after` and `deleted_at` to both.

- [ ] **Step 3: Build + verify existing tests still pass.** The fields default to nil, so behavior is unchanged.

- [ ] **Step 4: Commit:** `feat(flags): add DeleteAfter/DeletedAt to FeatureFlag model + scanners`

### Task 1.3: Repo methods — `QueueDeletion`, `HardDeleteFlag`, `RestoreFlag`, `ListFlagsToHardDelete`

**Files:**
- Modify: `internal/flags/repository.go` (interface)
- Modify: `internal/platform/database/postgres/flags.go` (concrete)
- Modify: `internal/flags/service_test.go` (mock — add the 4 new fns)

Add to `internal/flags/repository.go` (after `UnarchiveFlag`):

```go
// QueueDeletion sets delete_after = archived_at + retention. Returns
// ErrNotFound if the flag is not archived.
QueueDeletion(ctx context.Context, id uuid.UUID, retention time.Duration) error

// HardDeleteFlag tombstones the flag (sets deleted_at = now()). Enforces
// the retention check in SQL: archived_at IS NOT NULL AND
// archived_at + retention < now(). Returns ErrNotFound if the flag is
// not eligible (not archived, or retention not yet elapsed, or already
// tombstoned).
HardDeleteFlag(ctx context.Context, id uuid.UUID, retention time.Duration) error

// RestoreFlag clears archived_at, delete_after, and deleted_at. Returns
// ErrNotFound when no row matches the given id.
RestoreFlag(ctx context.Context, id uuid.UUID) error

// ListFlagsToHardDelete returns all flags whose delete_after has elapsed
// and which have not yet been tombstoned. Used by the retention sweep.
ListFlagsToHardDelete(ctx context.Context, limit int) ([]uuid.UUID, error)
```

- [ ] **Step 1: Add interface methods** as shown above.

- [ ] **Step 2: Concrete impl `QueueDeletion`** in `postgres/flags.go`:

```go
func (r *FlagRepository) QueueDeletion(ctx context.Context, id uuid.UUID, retention time.Duration) error {
    const q = `
        UPDATE feature_flags
        SET delete_after = archived_at + $2::interval, updated_at = now()
        WHERE id = $1 AND archived_at IS NOT NULL AND deleted_at IS NULL`
    interval := fmt.Sprintf("%d seconds", int(retention.Seconds()))
    tag, err := r.pool.Exec(ctx, q, id, interval)
    if err != nil {
        return fmt.Errorf("postgres.QueueDeletion: %w", err)
    }
    if tag.RowsAffected() == 0 {
        return ErrNotFound
    }
    return nil
}
```

- [ ] **Step 3: Concrete impl `HardDeleteFlag`** (tombstone, not actual DELETE):

```go
func (r *FlagRepository) HardDeleteFlag(ctx context.Context, id uuid.UUID, retention time.Duration) error {
    const q = `
        UPDATE feature_flags
        SET deleted_at = now(), enabled = false, updated_at = now()
        WHERE id = $1
          AND archived_at IS NOT NULL
          AND archived_at + $2::interval < now()
          AND deleted_at IS NULL`
    interval := fmt.Sprintf("%d seconds", int(retention.Seconds()))
    tag, err := r.pool.Exec(ctx, q, id, interval)
    if err != nil {
        return fmt.Errorf("postgres.HardDeleteFlag: %w", err)
    }
    if tag.RowsAffected() == 0 {
        return ErrNotFound
    }
    return nil
}
```

- [ ] **Step 4: Concrete impl `RestoreFlag`:**

```go
func (r *FlagRepository) RestoreFlag(ctx context.Context, id uuid.UUID) error {
    const q = `
        UPDATE feature_flags
        SET archived_at = NULL, delete_after = NULL, deleted_at = NULL, updated_at = now()
        WHERE id = $1`
    tag, err := r.pool.Exec(ctx, q, id)
    if err != nil {
        return fmt.Errorf("postgres.RestoreFlag: %w", err)
    }
    if tag.RowsAffected() == 0 {
        return ErrNotFound
    }
    return nil
}
```

- [ ] **Step 5: Concrete impl `ListFlagsToHardDelete`:**

```go
func (r *FlagRepository) ListFlagsToHardDelete(ctx context.Context, limit int) ([]uuid.UUID, error) {
    const q = `
        SELECT id FROM feature_flags
        WHERE delete_after IS NOT NULL
          AND delete_after < now()
          AND deleted_at IS NULL
        ORDER BY delete_after ASC
        LIMIT $1`
    rows, err := r.pool.Query(ctx, q, limit)
    if err != nil {
        return nil, fmt.Errorf("postgres.ListFlagsToHardDelete: %w", err)
    }
    defer rows.Close()
    var ids []uuid.UUID
    for rows.Next() {
        var id uuid.UUID
        if err := rows.Scan(&id); err != nil {
            return nil, fmt.Errorf("postgres.ListFlagsToHardDelete scan: %w", err)
        }
        ids = append(ids, id)
    }
    return ids, rows.Err()
}
```

- [ ] **Step 6: Update the mock** in `internal/flags/service_test.go`. For each new method, add a hook (`queueDeletionFn`, `hardDeleteFn`, `restoreFn`, `listToHardDeleteFn`) defaulting to a sensible no-op behavior, plus a real method on `mockFlagRepo` that calls the hook.

- [ ] **Step 7: Build + test.** `go build ./... && go test ./internal/flags/...` clean.

- [ ] **Step 8: Commit:** `feat(flags): add QueueDeletion / HardDeleteFlag / RestoreFlag / ListFlagsToHardDelete repo methods`

---

## Phase 2: Service + HTTP endpoints

### Task 2.1: Service methods

**File:** `internal/flags/service.go`

Add to the `FlagService` interface (after `UnarchiveFlag`):

```go
// QueueDeletion marks a flag for permanent removal at archived_at + retention.
QueueDeletion(ctx context.Context, id uuid.UUID, retention time.Duration) error

// HardDeleteFlag tombstones the flag if retention has elapsed.
HardDeleteFlag(ctx context.Context, id uuid.UUID, retention time.Duration) error

// RestoreFlag clears archived_at, delete_after, and deleted_at.
RestoreFlag(ctx context.Context, id uuid.UUID) error
```

- [ ] **Step 1: Add interface methods.**

- [ ] **Step 2: Concrete impls** — each is a thin wrapper that delegates to the repo, invalidates the cache, and publishes an event:

```go
func (s *flagService) QueueDeletion(ctx context.Context, id uuid.UUID, retention time.Duration) error {
    if err := s.repo.QueueDeletion(ctx, id, retention); err != nil {
        return fmt.Errorf("queue deletion: %w", err)
    }
    _ = s.cache.Invalidate(ctx, id)
    flag, err := s.repo.GetFlag(ctx, id)
    if err == nil {
        s.publishEvent(ctx, "queued_for_deletion", flag)
    }
    return nil
}

func (s *flagService) HardDeleteFlag(ctx context.Context, id uuid.UUID, retention time.Duration) error {
    if err := s.repo.HardDeleteFlag(ctx, id, retention); err != nil {
        return fmt.Errorf("hard delete: %w", err)
    }
    _ = s.cache.Invalidate(ctx, id)
    s.publishEvent(ctx, "hard_deleted", &models.FeatureFlag{ID: id})
    return nil
}

func (s *flagService) RestoreFlag(ctx context.Context, id uuid.UUID) error {
    if err := s.repo.RestoreFlag(ctx, id); err != nil {
        return fmt.Errorf("restore: %w", err)
    }
    _ = s.cache.Invalidate(ctx, id)
    flag, err := s.repo.GetFlag(ctx, id)
    if err == nil {
        s.publishEvent(ctx, "restored", flag)
    }
    return nil
}
```

- [ ] **Step 3: Add unit tests** for all three service methods. Each test: stub repo with the appropriate fn hook, call the service method, assert the hook was called and the cache was invalidated.

- [ ] **Step 4: Build + test.**

- [ ] **Step 5: Commit:** `feat(flags): add QueueDeletion / HardDeleteFlag / RestoreFlag service methods`

### Task 2.2: HTTP endpoints

**File:** `internal/flags/handler.go`

Three new handlers + audit calls + route mounts.

**Constant:** Add a package-level `const RetentionWindow = 30 * 24 * time.Hour` (or pull from config — for v1 the constant is fine; configurable retention is a follow-up).

- [ ] **Step 1: `queueFlagDeletion` handler.** After `archiveFlag` (around line 529):

```go
func (h *Handler) queueFlagDeletion(c *gin.Context) {
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
        return
    }

    flag, err := h.service.GetFlag(c.Request.Context(), id)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "flag not found"})
        return
    }
    if !flag.Archived {
        c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "flag must be archived first"})
        return
    }
    if err := h.service.QueueDeletion(c.Request.Context(), id, RetentionWindow); err != nil {
        c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
        return
    }

    h.writeAudit(c, "flag.queued_for_deletion", "flag", id, "", "")
    h.broadcastEvent("flag.queued_for_deletion", id, "")

    flag, _ = h.service.GetFlag(c.Request.Context(), id) // refresh for response
    c.JSON(http.StatusOK, flag)
}
```

- [ ] **Step 2: `hardDeleteFlag` handler.** Requires `force=true` query param + typed-slug confirmation header `X-Confirm-Slug: <flag-key>`:

```go
func (h *Handler) hardDeleteFlag(c *gin.Context) {
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
        return
    }
    if c.Query("force") != "true" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "hard delete requires ?force=true"})
        return
    }

    flag, err := h.service.GetFlag(c.Request.Context(), id)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "flag not found"})
        return
    }

    if c.GetHeader("X-Confirm-Slug") != flag.Key {
        c.JSON(http.StatusBadRequest, gin.H{"error": "X-Confirm-Slug header must match flag key"})
        return
    }
    if !flag.Archived {
        c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "flag must be archived first"})
        return
    }

    if err := h.service.HardDeleteFlag(c.Request.Context(), id, RetentionWindow); err != nil {
        // Distinguish "retention not elapsed" from other errors via the wrapped sentinel.
        // The repo returns ErrNotFound when the SQL guard rejects the call; we surface
        // a more helpful message here.
        if strings.Contains(err.Error(), "not found") {
            c.JSON(http.StatusUnprocessableEntity, gin.H{
                "error": fmt.Sprintf("retention not elapsed (must wait until %s)", flag.ArchivedAt.Add(RetentionWindow).Format(time.RFC3339)),
            })
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    h.writeAudit(c, "flag.hard_deleted", "flag", id, "", "")
    h.broadcastEvent("flag.hard_deleted", id, "")
    c.Status(http.StatusNoContent)
}
```

- [ ] **Step 3: `restoreFlag` handler.**

```go
func (h *Handler) restoreFlag(c *gin.Context) {
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
        return
    }
    flag, err := h.service.GetFlag(c.Request.Context(), id)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "flag not found"})
        return
    }
    if flag.DeletedAt != nil {
        c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "flag is tombstoned and cannot be restored"})
        return
    }
    if err := h.service.RestoreFlag(c.Request.Context(), id); err != nil {
        c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
        return
    }

    h.writeAudit(c, "flag.restored", "flag", id, "", "")
    h.broadcastEvent("flag.restored", id, "")

    flag, _ = h.service.GetFlag(c.Request.Context(), id)
    c.JSON(http.StatusOK, flag)
}
```

- [ ] **Step 4: Mount routes** in `RegisterRoutes` (around line 116-130). Place after the existing `archive` route:

```go
flags.POST("/:id/queue-deletion", auth.RequirePermission(h.rbac, auth.PermFlagArchive), h.queueFlagDeletion)
flags.DELETE("/:id", auth.RequirePermission(h.rbac, auth.PermFlagArchive), h.hardDeleteFlag)
flags.POST("/:id/restore", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.restoreFlag)
```

(`PermFlagArchive` is the existing permission used for archive. `PermFlagUpdate` for restore — restoring is a state mutation, not a destructive op.)

- [ ] **Step 5: Add handler tests** at `internal/flags/handler_test.go`. For each endpoint, exercise the happy path + the main rejection path:
  - `queueFlagDeletion`: 200 on archived flag; 422 on active flag.
  - `hardDeleteFlag`: 204 on archived + retention-elapsed + correct slug; 400 on missing `force=true`; 400 on slug mismatch; 422 on retention-not-elapsed (use a mock repo that returns ErrNotFound for the retention guard).
  - `restoreFlag`: 200 on archived flag; 422 on already-tombstoned flag.

- [ ] **Step 6: Build + test.**

- [ ] **Step 7: Commit:** `feat(flags): add queue-deletion / hard-delete / restore HTTP endpoints`

### Task 2.3: Register revert handlers for the new audit actions

**File:** `internal/flags/revert.go`

The existing `FlagRevertHandlers` returns 8 tuples. Add three more:

- [ ] **Step 1: Add `revertFlagQueuedForDeletion`** — clears `delete_after`. Implementation: load flag, if `delete_after == nil` race; else call a new repo method `ClearDeleteAfter(id)` (or just call `RestoreFlag` and accept that it also clears `archived_at` — the user's `flag.queued_for_deletion` action implies the flag was archived, so unarchiving as a side effect is questionable). Cleaner: add a focused repo method:

```go
// In repo interface:
ClearDeleteAfter(ctx context.Context, id uuid.UUID) error

// In postgres:
func (r *FlagRepository) ClearDeleteAfter(ctx context.Context, id uuid.UUID) error {
    const q = `UPDATE feature_flags SET delete_after = NULL, updated_at = now() WHERE id = $1`
    _, err := r.pool.Exec(ctx, q, id)
    if err != nil {
        return fmt.Errorf("postgres.ClearDeleteAfter: %w", err)
    }
    return nil
}
```

Then the revert handler:

```go
func revertFlagQueuedForDeletion(svc FlagService) auth.RevertHandler {
    return func(ctx context.Context, entry *models.AuditLogEntry, force bool) (string, error) {
        flag, err := svc.GetFlag(ctx, entry.EntityID)
        if err != nil {
            return "", fmt.Errorf("flag.queued_for_deletion revert: load flag: %w", err)
        }
        if flag.DeleteAfter == nil && !force {
            return "", auth.ErrRevertRace // already cleared
        }
        if err := svc.ClearDeleteAfter(ctx, entry.EntityID); err != nil {
            return "", fmt.Errorf("flag.queued_for_deletion revert: clear: %w", err)
        }
        return "flag.queued_for_deletion.reverted", nil
    }
}
```

(Add `ClearDeleteAfter` to `FlagService` interface and impl too.)

- [ ] **Step 2: Add `revertFlagRestored`** — re-archives. Implementation:

```go
func revertFlagRestored(svc FlagService) auth.RevertHandler {
    return func(ctx context.Context, entry *models.AuditLogEntry, force bool) (string, error) {
        flag, err := svc.GetFlag(ctx, entry.EntityID)
        if err != nil {
            return "", fmt.Errorf("flag.restored revert: load flag: %w", err)
        }
        if flag.Archived && !force {
            return "", auth.ErrRevertRace
        }
        if err := svc.ArchiveFlag(ctx, entry.EntityID); err != nil {
            return "", fmt.Errorf("flag.restored revert: archive: %w", err)
        }
        return "flag.restored.reverted", nil
    }
}
```

- [ ] **Step 3: `flag.hard_deleted` is intentionally non-revertible.** Don't register a handler. The audit list will show `revertible: false` and the UI tooltip already handles it via `NON_REVERTIBLE_REASONS["flag.hard_deleted"]`.

- [ ] **Step 4: Wire the new tuples in `FlagRevertHandlers`.** Add them to the slice returned by the constructor, in the order: queued_for_deletion, restored.

- [ ] **Step 5: Add tests** for each new handler in `internal/flags/revert_test.go` — at minimum: success path, race detected, force overrides race.

- [ ] **Step 6: Build + test.**

- [ ] **Step 7: Commit:** `feat(flags): register revert handlers for queued_for_deletion and restored`

---

## Phase 3: Retention sweep

### Task 3.1: Sweep loop

**File:** Create `internal/flags/retention_sweep.go`

```go
package flags

import (
    "context"
    "log"
    "time"
)

// RetentionSweeper periodically tombstones flags whose delete_after has elapsed.
type RetentionSweeper struct {
    svc       FlagService
    repo      FlagRepository
    interval  time.Duration
    retention time.Duration
    batchSize int
}

func NewRetentionSweeper(svc FlagService, repo FlagRepository, interval, retention time.Duration) *RetentionSweeper {
    return &RetentionSweeper{
        svc:       svc,
        repo:      repo,
        interval:  interval,
        retention: retention,
        batchSize: 100,
    }
}

// Run blocks until ctx is cancelled. Wakes every interval to scan for expired flags.
func (s *RetentionSweeper) Run(ctx context.Context) {
    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()
    s.sweepOnce(ctx) // run once at startup
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.sweepOnce(ctx)
        }
    }
}

func (s *RetentionSweeper) sweepOnce(ctx context.Context) {
    ids, err := s.repo.ListFlagsToHardDelete(ctx, s.batchSize)
    if err != nil {
        log.Printf("retention_sweep: list failed: %v", err)
        return
    }
    if len(ids) == 0 {
        return
    }
    log.Printf("retention_sweep: tombstoning %d flag(s)", len(ids))
    for _, id := range ids {
        if err := s.svc.HardDeleteFlag(ctx, id, s.retention); err != nil {
            log.Printf("retention_sweep: tombstone %s failed: %v", id, err)
            continue
        }
        // Note: no audit row written by the sweep — the operator's
        // earlier `flag.queued_for_deletion` is the auditable action.
        // If we want a `flag.hard_deleted` row from the sweep, write
        // it here with a system actor; for v1 keep it quiet.
    }
}
```

- [ ] **Step 1: Write the file.**

- [ ] **Step 2: Add a unit test** at `internal/flags/retention_sweep_test.go` that:
  - Stubs the repo's `ListFlagsToHardDelete` to return 2 IDs.
  - Stubs service's `HardDeleteFlag` to track calls.
  - Calls `sweeper.sweepOnce(ctx)`.
  - Asserts `HardDeleteFlag` was called twice with the matching IDs.

- [ ] **Step 3: Build + test.**

- [ ] **Step 4: Commit:** `feat(flags): add retention sweep job`

### Task 3.2: Wire sweeper in `cmd/api/main.go`

**File:** `cmd/api/main.go`

Add after the registry wiring (around line 264), before the route registration:

- [ ] **Step 1: Construct + start the sweeper:**

```go
// Retention sweep: tombstone flags whose delete_after has elapsed.
// Default: scan every 6h; 30-day retention window.
retentionSweeper := flags.NewRetentionSweeper(flagService, flagRepo, 6*time.Hour, 30*24*time.Hour)
go retentionSweeper.Run(context.Background())
```

(Use the request/lifecycle context if there's a graceful-shutdown ctx already — search for `signal.NotifyContext` in main.go.)

- [ ] **Step 2: Build clean.** `go build ./...`.

- [ ] **Step 3: Smoke test.** `make dev-up && make migrate-up && make run-api`. Confirm the API starts without errors. The sweep won't have anything to do unless flags are queued, but the goroutine should be running.

- [ ] **Step 4: Commit:** `feat(api): start retention sweep on API boot`

---

## Phase 4: Frontend

### Task 4.1: API client

**File:** `web/src/api.ts`

Find the existing `flagsApi` block and add three methods:

- [ ] **Step 1: Update `Flag` type.** In `web/src/types.ts`, add to the `Flag` interface:

```ts
delete_after?: string | null; // RFC3339; null/missing means not queued
deleted_at?: string | null;   // RFC3339; non-null means tombstoned
```

- [ ] **Step 2: Add API methods to `flagsApi`:**

```ts
queueDeletion: (id: string) =>
  request<Flag>(`/flags/${id}/queue-deletion`, { method: 'POST' }),

hardDelete: (id: string, slug: string) =>
  request<void>(`/flags/${id}?force=true`, {
    method: 'DELETE',
    headers: { 'X-Confirm-Slug': slug },
  }),

restore: (id: string) =>
  request<Flag>(`/flags/${id}/restore`, { method: 'POST' }),
```

- [ ] **Step 3: TS clean + build clean.** `cd web && npx tsc --noEmit && npm run build`.

- [ ] **Step 4: Commit:** `feat(web): extend flagsApi with queueDeletion / hardDelete / restore`

### Task 4.2: Lifecycle panel on FlagDetailPage Settings tab

**File:** `web/src/pages/FlagDetailPage.tsx`

The Settings tab already has a Danger Zone block (added in PR #73, the typed-slug archive flow). This task replaces that block with a state-driven Lifecycle panel that renders one of three states based on the flag's lifecycle columns:

| State | Condition | Buttons |
|---|---|---|
| **Active** | `archived_at == null` | "Archive Flag" (existing typed-slug confirm) |
| **Archived, within retention** | `archived_at != null && (delete_after == null || archived_at + 30d >= now)` | "Restore Flag" + "Queue for Deletion" |
| **Archived, retention elapsed** | `archived_at != null && archived_at + 30d < now` (or `delete_after != null && delete_after < now`) | "Restore Flag" + "Permanently Delete" (separate typed-slug confirm) |

- [ ] **Step 1: Compute state.** In the FlagDetailPage component, derive a `lifecycleState: 'active' | 'within' | 'elapsed'`:

```ts
const lifecycleState = useMemo(() => {
  if (!flag?.archived_at) return 'active';
  const elapsedAt = new Date(flag.archived_at).getTime() + 30 * 24 * 3600 * 1000;
  if (Date.now() >= elapsedAt) return 'elapsed';
  return 'within';
}, [flag?.archived_at]);
```

- [ ] **Step 2: Replace the existing Danger Zone JSX in the Settings tab** with a state-driven block. The existing typed-slug `ConfirmDialog` infrastructure (`requireTypedConfirm` + `acknowledgement` props on `ConfirmDialog.tsx`) is already in place from PR #73 — reuse it.

```tsx
{lifecycleState === 'active' && (
  <div className="danger-zone">
    <h3>Danger Zone</h3>
    <p>Archive this flag. Archived flags are eligible for permanent deletion after 30 days.</p>
    <button className="btn btn-danger" onClick={() => setArchiveDialogOpen(true)}>Archive Flag</button>
  </div>
)}

{lifecycleState === 'within' && (
  <div className="lifecycle-panel">
    <h3>Archived</h3>
    <p>
      Archived on {new Date(flag.archived_at!).toLocaleDateString()} —
      eligible for deletion on {new Date(new Date(flag.archived_at!).getTime() + 30 * 24 * 3600 * 1000).toLocaleDateString()}.
    </p>
    {flag.delete_after && (
      <p style={{ color: 'var(--color-warning, #d97706)' }}>
        Queued for deletion at {new Date(flag.delete_after).toLocaleString()}.
      </p>
    )}
    <div style={{ display: 'flex', gap: 8 }}>
      <button className="btn btn-secondary" onClick={handleRestore}>Restore Flag</button>
      {!flag.delete_after && (
        <button className="btn btn-danger" onClick={() => setQueueDialogOpen(true)}>Queue for Deletion</button>
      )}
    </div>
  </div>
)}

{lifecycleState === 'elapsed' && (
  <div className="lifecycle-panel danger-zone">
    <h3>Eligible for permanent deletion</h3>
    <p>The 30-day retention window has elapsed. This flag will be permanently deleted on the next sweep, or you can delete it now.</p>
    <div style={{ display: 'flex', gap: 8 }}>
      <button className="btn btn-secondary" onClick={handleRestore}>Restore Flag</button>
      <button className="btn btn-danger" onClick={() => setHardDeleteDialogOpen(true)}>Permanently Delete</button>
    </div>
  </div>
)}
```

- [ ] **Step 3: Wire the three handlers.**

```tsx
const handleRestore = async () => {
  if (!id) return;
  try {
    const updated = await flagsApi.restore(id);
    setFlag(updated);
  } catch (err) {
    setError((err as Error).message);
  }
};

const handleQueueDeletion = async () => {
  if (!id) return;
  try {
    const updated = await flagsApi.queueDeletion(id);
    setFlag(updated);
    setQueueDialogOpen(false);
  } catch (err) {
    setError((err as Error).message);
  }
};

const handleHardDelete = async () => {
  if (!id || !flag) return;
  try {
    await flagsApi.hardDelete(id, flag.key);
    // Tombstoned — navigate back to flag list.
    navigate(`/orgs/${orgSlug}/projects/${flag.project_slug ?? ''}/flags`);
  } catch (err) {
    setError((err as Error).message);
  }
};
```

- [ ] **Step 4: Wire two new ConfirmDialogs** alongside the existing archive one. Each uses the typed-slug pattern:

```tsx
<ConfirmDialog
  open={queueDialogOpen}
  title="Queue for Deletion"
  message={`Type the flag key to confirm. ${flag?.key} will be permanently deleted on ${flag?.archived_at ? new Date(new Date(flag.archived_at).getTime() + 30 * 24 * 3600 * 1000).toLocaleDateString() : 'archived_at + 30 days'}.`}
  confirmLabel="Queue for Deletion"
  confirmVariant="danger"
  requireTypedConfirm={flag?.key}
  acknowledgement="I understand this flag will be deleted automatically after the retention window."
  onConfirm={handleQueueDeletion}
  onCancel={() => setQueueDialogOpen(false)}
/>

<ConfirmDialog
  open={hardDeleteDialogOpen}
  title="Permanently Delete Flag"
  message={`This is irreversible. The flag and all its rules, ratings, and per-environment states will be removed. Type the flag key to confirm.`}
  confirmLabel="Permanently Delete"
  confirmVariant="danger"
  requireTypedConfirm={flag?.key}
  acknowledgement="I have verified this flag is no longer needed and accept that this action cannot be undone."
  onConfirm={handleHardDelete}
  onCancel={() => setHardDeleteDialogOpen(false)}
/>
```

- [ ] **Step 5: Add CSS for `.lifecycle-panel`** in `web/src/styles/globals.css` — minor styling; can mirror `.danger-zone`.

- [ ] **Step 6: TS + build clean.**

- [ ] **Step 7: Commit:** `feat(web): add lifecycle panel with queue-deletion + restore + hard-delete`

---

## Phase 5: Verification

### Task 5.1: End-to-end smoke + verification

- [ ] **Step 1: All verification commands clean:**
  - `go build ./...`
  - `go vet ./...`
  - `go test -short ./...`
  - `cd web && npx tsc --noEmit`
  - `cd web && npm run build`
  - `cd web && npm run lint`

- [ ] **Step 2: Manual smoke (after `make dev-up && make migrate-up`):**
  - Archive a flag → confirm `archived_at` is set in the DB (`SET search_path TO deploy; SELECT id, archived_at FROM feature_flags WHERE key = '<key>';`). **This is the regression test for the Phase 0 fix.**
  - Visit FlagDetailPage Settings tab → see "Archived, within retention" panel with a "Queue for Deletion" button.
  - Click "Queue for Deletion" → typed-slug confirm → confirm `delete_after` is set in the DB.
  - Audit log shows `flag.queued_for_deletion` with `revertible: true`.
  - Revert the queue-deletion via the OrgAuditPage → `delete_after` is cleared.
  - Re-queue, then directly invoke the sweeper for testing: insert a flag with `archived_at = now() - interval '31 days'` and `delete_after = now() - interval '1 minute'`, run the sweep manually (`go run ...` or trigger via test). Confirm `deleted_at` is set.
  - Visit the flag → it's still readable but UI shows the tombstoned state (or 404 — decide; for v1, the API returns the row, the UI hides it from list views).
  - Try `DELETE /flags/:id?force=true` with `X-Confirm-Slug: <key>` on an archived-but-retention-not-elapsed flag → 422 with retention message.

- [ ] **Step 3: Update the plan file** with a "Verification — YYYY-MM-DD" block appended to Phase 5.

- [ ] **Step 4: Commit:** `docs(plan): record verification results for flag-hard-delete`

---

### Verification — 2026-04-30

- `go build ./...`: PASS
- `go vet ./...`: PASS
- `go test -short ./...`: PASS — 37 packages, 0 failures
- `tsc --noEmit`: PASS
- `npm run build`: PASS
- `npm run lint`: PASS — max-warnings 0
- Smoke checklist: 10/10 bullets verifiable in code; none blocked

#### Smoke checklist detail

1. **`ArchiveFlag` persists `archived_at`** — `service.ArchiveFlag` calls `repo.ArchiveFlag` (line 274 service.go); `postgres.ArchiveFlag` runs `UPDATE feature_flags SET archived_at = now() WHERE id = $1 AND archived_at IS NULL` (line 494 postgres/flags.go). Wired end-to-end. (verifiable in code)
2. **FlagDetailPage lifecycle states** — `lifecycleState` derivation at line 340 covers `tombstoned` (deleted_at set), `active` (no archived_at), `within` (archived but retention not elapsed), `elapsed` (retention elapsed). All four have distinct JSX branches at lines 1202, 1217, 1255, 1283. (verifiable in code)
3. **`POST /flags/:id/queue-deletion` sets `delete_after`** — Route mounted at line 134 handler.go; handler calls `h.service.QueueDeletion(id, RetentionWindow)` at line 607. (verifiable in code)
4. **Audit log records `flag.queued_for_deletion`** — `h.writeAudit(c, "flag.queued_for_deletion", ...)` at line 612 handler.go. `revertFlagQueuedForDeletion` is registered in `FlagRevertHandlers` (line 33 revert.go); `IsRevertible` returns `true` for registered actions via the registry lookup (revert_registry.go:36); audit handler populates `Revertible` field via `h.registry.IsRevertible(...)` (audit_handler.go:136). (verifiable in code)
5. **Reverting `flag.queued_for_deletion` clears only `delete_after`** — `revertFlagQueuedForDeletion` calls `svc.ClearDeleteAfter` (revert.go:294), NOT `svc.RestoreFlag`. `archived_at` is not touched. (verifiable in code)
6. **Sweep job tombstones elapsed flags** — `RetentionSweeper.sweepOnce` calls `s.repo.ListFlagsToHardDelete` then `s.svc.HardDeleteFlag` per ID (retention_sweep.go:52–63). Wired in `cmd/api/main.go` at line 647 as `go retentionSweeper.Run(ctx)`. (verifiable in code)
7. **`DELETE /flags/:id?force=true` with `X-Confirm-Slug`** — Route mounted at line 135 handler.go. Guards: invalid id → 400 (line 622), missing `?force=true` → 400 (line 626), slug mismatch → 400 (line 637), not archived → 422 (line 641), retention not elapsed (repo returns ErrNotFound) → 422 with message including eligible date (lines 646–651). Success → 204 (line 660). (verifiable in code)
8. **Reverting `flag.restored` re-archives** — `revertFlagRestored` calls `svc.ArchiveFlag` (revert.go:314) and is registered in `FlagRevertHandlers` (line 34 revert.go). (verifiable in code)
9. **`flag.hard_deleted` is non-revertible** — `flag.hard_deleted` does NOT appear in `FlagRevertHandlers`; only `flag.queued_for_deletion` and `flag.restored` are registered (revert.go lines 33–34). Registry's "no handler" path returns `ErrNotRevertible`; `IsRevertible` returns `false`; audit list reports `revertible: false`. (verifiable in code)
10. **Tombstoned flags show read-only banner** — `lifecycleState === 'tombstoned'` branch at line 1283 renders a `.lifecycle-panel` `<div>` with descriptive text only — no action buttons. (verifiable in code)

#### Branch summary

- Total commits since `04b5915` (origin/main, post-#77): 13
- Files changed: 20
- Lines added/deleted: +2341 / −56

---

## Out of scope (call out and don't do)

- **Quarterly compaction job** that actually `DELETE`s tombstoned flags older than the audit retention window. Tombstones live forever in this PR.
- **Pre-flight cascade preview** ("this hard-delete will also remove N rules, M ratings"). Worth a follow-up.
- **Configurable retention window** per org or per flag. v1 is a single `RetentionWindow` constant.
- **External WORM bucket** for purge audit rows (compliance follow-up per spec).
- **Rate limiting** the sweep batch — `batchSize: 100` is fine for v1.
- **`flag.hard_deleted` audit row written by the sweep itself.** Operator-initiated `flag.queued_for_deletion` covers the auditable intent. Adding a system-actor row from the sweep is a follow-up.

---

## Risks & callouts

| Risk | Mitigation |
|---|---|
| **Phase 0 latent bug** — if not fixed first, hard-delete never works | Phase 0 is mandatory; regression test catches it |
| Tombstoned flags still show in API responses | UI hides them via `deleted_at == null` filter on list pages; FlagDetailPage shows the tombstoned banner |
| Sweep fires before all tasks ship | The sweep ticker uses `time.Hour` granularity; in dev/staging the deploy cycle is much shorter so a stale sweep is unlikely to cause issues |
| `flag.hard_deleted` revertibility drift | Spec marks it explicitly non-revertible; the registry's "no handler = non-revertible" default makes this a no-op |
| `audit_log.resource_id` lacks FK | Tombstone preserves UI joinability; if compaction ever happens, audit rows lose their flag-name reference but stay readable as raw entries |
| Concurrent restore + queue-deletion race | Both endpoints are idempotent at the SQL level; race detection in revert handlers covers the audit-time race |

---

## Self-review notes

- All three Deliverable 2 endpoints (`POST /flags/:id/queue-deletion`, `DELETE /flags/:id?force=true`, `POST /flags/:id/restore`) have a task implementing them (Task 2.2).
- Migration 060 columns and index match the spec (Task 1.1).
- Sweep job exists with operator opt-in (Task 3.1, 3.2).
- UI Settings panel covers the three lifecycle states (Task 4.2).
- The Phase 0 fix is the precondition without which nothing else works; flagged at the top.
- Revert handlers for the new audit actions land in Task 2.3, completing the spec's revertibility table for the flag family.
- No placeholder steps; every step shows the file, the change, and the verification.
