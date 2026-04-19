# Rollout Groups & Coordination Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the RolloutGroup entity — a group for related rollouts (e.g., "v1.1 deploy + feature flag enable") — and optional cross-rollout coordination policies that react to sibling aborts. Backward-compatible: rollouts without a group continue to behave exactly as in Plans 2+3.

**Architecture:** A new `rollout_groups` table stores the bundle plus its `coordination_policy` (`independent`, `pause_on_sibling_abort`, `cascade_abort`). `Rollout.ReleaseID` is already wired from Plan 2 — this plan reuses that column as the group FK (name kept for Plan 2 compatibility; it semantically points at a `rollout_groups.id`). Adds RolloutGroup CRUD, a `RolloutGroupService`, an HTTP handler, and an event-driven **Coordinator** that listens for `rollouts.rollout.rolled_back` NATS events and applies the group's policy to sibling rollouts. CLI and docs complete the loop.

**Naming note:** The name "RolloutGroup" (not "Release") avoids collision with the existing `models.Release`, `internal/releases/` package, and `releases` DB table, which track version/commit metadata — a different concept.

**Tech Stack:** Go 1.22+, PostgreSQL (schema `deploy`), gin, NATS JetStream (reuse existing pub/sub), cobra. Builds on Plans 1+2+3 (already merged to main).

**Spec:** `docs/superpowers/specs/2026-04-18-configurable-rollout-strategies-design.md`
**Prior plans:** `docs/superpowers/plans/2026-04-18-rollout-strategies-foundation.md`, `-rollout-engine-deploy.md`, `-rollout-config-integration.md`

---

## File Structure

**New files:**

```
migrations/
  052_create_rollout_groups.up.sql
  052_create_rollout_groups.down.sql

internal/models/
  rollout_group.go              # RolloutGroup + CoordinationPolicy enum

internal/rollout/
  group_repository.go           # RolloutGroupRepository interface
  group_service.go              # RolloutGroupService (CRUD + attach + sibling lookup)
  group_service_test.go
  group_handler.go              # HTTP: GET list/get, POST create, PUT update, POST attach
  group_handler_test.go
  coordination.go               # Coordinator: event-driven sibling handling
  coordination_test.go

internal/platform/database/postgres/
  rollout_groups.go             # RolloutGroupRepo postgres impl
  rollout_groups_test.go

cmd/cli/
  rolloutgroups.go              # ds rollout-groups {list, get, create, attach}
```

**Modified files:**

```
internal/rollout/rollout_repository.go  # Add ListByRelease + SetReleaseID repo methods
internal/platform/database/postgres/rollouts.go  # ListByRelease + SetReleaseID impls
cmd/api/main.go                        # Wire RolloutGroupService + handler + coordinator subscriber
docs/Rollout_Strategies.md             # New "Rollout groups" section
docs/Current_Initiatives.md            # Phase → Implementation (Plans 1+2+3+4)
```

**Coordination semantics** (from spec):
- `independent` (default) — sibling rollback has no effect on other rollouts in the group.
- `pause_on_sibling_abort` — when any rollout in the group rolls back, active siblings transition to `paused` with reason `sibling_aborted:<id>`.
- `cascade_abort` — when any rollout rolls back, active siblings transition to `rolled_back` cascaded through `RolloutService.Rollback`.

**Event flow:**
Rollout A rolls back → `rollouts.rollout.rolled_back` NATS message published with `{rollout_id}` → `Coordinator` consumes → looks up rollout A's `release_id` (the Plan 2 FK column that points at a RolloutGroup) → if set, loads group's `coordination_policy` → lists sibling rollouts in the same group with active state → calls `RolloutService.Pause(sibling)` or `.Rollback(sibling)`.

**Loop-prevention:** the coordinator filters out the originating rollout when listing siblings. When cascade_abort triggers a sibling rollback, the sibling's own `rolled_back` event fires but the coordinator finds no other actives in the group, so cascade terminates.

**Out of scope (acknowledged):** sibling **Revert** via `RolloutService.Rollback` currently updates state only (pre-existing Plan 2 gap). Coordinated sibling rollback inherits this. Documented; not fixed here.

**Terminology legend for this plan:**
- `rollout_groups` — the new DB table.
- `RolloutGroup` — the new Go model type.
- `RolloutGroup*` — service/handler/repo types.
- `ds rollout-groups` — CLI command.
- `rollouts.release_id` / `Rollout.ReleaseID` — existing Plan 2 column/field; NOT renamed here. It now semantically points at a `rollout_groups.id`.

---

## Task 1: Migration — rollout_groups table

**Files:**
- Create: `migrations/052_create_rollout_groups.up.sql`
- Create: `migrations/052_create_rollout_groups.down.sql`

- [ ] **Step 1: Write up migration**

Create `migrations/052_create_rollout_groups.up.sql`:

```sql
-- Rollout groups: optional bundle grouping related rollouts with coordination policy.
-- Distinct from the pre-existing `releases` table (which tracks application version metadata).
CREATE TABLE rollout_groups (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_type           TEXT NOT NULL CHECK (scope_type IN ('org','project','app')),
    scope_id             UUID NOT NULL,
    name                 TEXT NOT NULL,
    description          TEXT NOT NULL DEFAULT '',
    coordination_policy  TEXT NOT NULL DEFAULT 'independent'
        CHECK (coordination_policy IN ('independent','pause_on_sibling_abort','cascade_abort')),
    created_by           UUID,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_rollout_groups_scope ON rollout_groups(scope_type, scope_id);
```

Note: `rollouts.release_id` already exists (Plan 2 migration 050) and will point at `rollout_groups.id`. No FK constraint is added — deliberate so groups can be deleted without cascading into the rollout ledger.

- [ ] **Step 2: Write down migration**

Create `migrations/052_create_rollout_groups.down.sql`:

```sql
DROP TABLE IF EXISTS rollout_groups;
```

- [ ] **Step 3: Run migrations and verify**

```bash
cd /Users/sgamel/git/DeploySentry-releases
make dev-up
make migrate-up
psql "$(grep '^MIGRATE_DSN' Makefile | head -1 | cut -d= -f2-)" -c "SET search_path TO deploy; \d rollout_groups"
make migrate-down
make migrate-up
```

Expected: 9 columns, `idx_rollout_groups_scope` index, 2 CHECK constraints. Migrate-down cleans; re-apply seeds again.

- [ ] **Step 4: Commit**

```bash
git add migrations/052_create_rollout_groups.up.sql migrations/052_create_rollout_groups.down.sql
git commit -m "feat(db): add rollout_groups table"
```

---

## Task 2: RolloutGroup Go model

**Files:**
- Create: `internal/models/rollout_group.go`
- Create: `internal/models/rollout_group_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/models/rollout_group_test.go`:

```go
package models

import "testing"

func TestCoordinationPolicyStrings(t *testing.T) {
	for p, want := range map[CoordinationPolicy]string{
		CoordinationIndependent:         "independent",
		CoordinationPauseOnSiblingAbort: "pause_on_sibling_abort",
		CoordinationCascadeAbort:        "cascade_abort",
	} {
		if string(p) != want {
			t.Errorf("%v: got %q want %q", p, string(p), want)
		}
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/models/ -run TestCoordinationPolicy -v
```

Expected: undefined.

- [ ] **Step 3: Create model**

Create `internal/models/rollout_group.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

// CoordinationPolicy determines how a RolloutGroup reacts when any of its
// member rollouts rolls back.
type CoordinationPolicy string

const (
	// CoordinationIndependent means sibling rollbacks do not affect each other.
	CoordinationIndependent CoordinationPolicy = "independent"
	// CoordinationPauseOnSiblingAbort means active siblings are paused when any
	// rollout in the group rolls back.
	CoordinationPauseOnSiblingAbort CoordinationPolicy = "pause_on_sibling_abort"
	// CoordinationCascadeAbort means active siblings are rolled back when any
	// rollout in the group rolls back.
	CoordinationCascadeAbort CoordinationPolicy = "cascade_abort"
)

// RolloutGroup is an optional bundle grouping related rollouts. Rollouts
// reference their group via Rollout.ReleaseID (column name preserved from
// Plan 2 migration 050; the value is a rollout_groups.id).
type RolloutGroup struct {
	ID                 uuid.UUID          `json:"id"`
	ScopeType          ScopeType          `json:"scope_type"`
	ScopeID            uuid.UUID          `json:"scope_id"`
	Name               string             `json:"name"`
	Description        string             `json:"description"`
	CoordinationPolicy CoordinationPolicy `json:"coordination_policy"`
	CreatedBy          *uuid.UUID         `json:"created_by,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/models/ -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/models/rollout_group.go internal/models/rollout_group_test.go
git commit -m "feat(models): add RolloutGroup + CoordinationPolicy types"
```

---

## Task 3: RolloutGroup repository (interface + Postgres + tests)

**Files:**
- Create: `internal/rollout/group_repository.go`
- Create: `internal/platform/database/postgres/rollout_groups.go`
- Create: `internal/platform/database/postgres/rollout_groups_test.go`

- [ ] **Step 1: Define interface**

Create `internal/rollout/group_repository.go`:

```go
package rollout

import (
	"context"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// RolloutGroupRepository persists RolloutGroup rows.
type RolloutGroupRepository interface {
	Create(ctx context.Context, g *models.RolloutGroup) error
	Get(ctx context.Context, id uuid.UUID) (*models.RolloutGroup, error)
	ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.RolloutGroup, error)
	Update(ctx context.Context, g *models.RolloutGroup) error
	Delete(ctx context.Context, id uuid.UUID) error
}
```

- [ ] **Step 2: Write failing integration test**

Create `internal/platform/database/postgres/rollout_groups_test.go`:

```go
package postgres

import (
	"context"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

func sampleGroup(orgID uuid.UUID) *models.RolloutGroup {
	return &models.RolloutGroup{
		ScopeType:          models.ScopeOrg,
		ScopeID:            orgID,
		Name:               "v1.1-" + uuid.NewString()[:8],
		Description:        "test bundle",
		CoordinationPolicy: models.CoordinationIndependent,
	}
}

func TestRolloutGroupRepo_CreateGet(t *testing.T) {
	ctx := context.Background()
	repo := NewRolloutGroupRepo(testDB(t))
	g := sampleGroup(uuid.New())
	if err := repo.Create(ctx, g); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.Get(ctx, g.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != g.Name || got.CoordinationPolicy != models.CoordinationIndependent {
		t.Fatalf("mismatch: %+v", got)
	}
}

func TestRolloutGroupRepo_UpdatePolicy(t *testing.T) {
	ctx := context.Background()
	repo := NewRolloutGroupRepo(testDB(t))
	g := sampleGroup(uuid.New())
	if err := repo.Create(ctx, g); err != nil {
		t.Fatal(err)
	}
	g.CoordinationPolicy = models.CoordinationCascadeAbort
	g.Description = "updated"
	if err := repo.Update(ctx, g); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.Get(ctx, g.ID)
	if got.CoordinationPolicy != models.CoordinationCascadeAbort || got.Description != "updated" {
		t.Fatalf("update not applied: %+v", got)
	}
}

func TestRolloutGroupRepo_ListByScope(t *testing.T) {
	ctx := context.Background()
	repo := NewRolloutGroupRepo(testDB(t))
	orgID := uuid.New()
	g1 := sampleGroup(orgID)
	g2 := sampleGroup(orgID)
	g3 := sampleGroup(uuid.New()) // different org
	if err := repo.Create(ctx, g1); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, g2); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, g3); err != nil {
		t.Fatal(err)
	}
	got, err := repo.ListByScope(ctx, models.ScopeOrg, orgID)
	if err != nil || len(got) != 2 {
		t.Fatalf("list: err=%v len=%d", err, len(got))
	}
}

func TestRolloutGroupRepo_Delete(t *testing.T) {
	ctx := context.Background()
	repo := NewRolloutGroupRepo(testDB(t))
	g := sampleGroup(uuid.New())
	if err := repo.Create(ctx, g); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, g.ID); err == nil {
		t.Fatalf("expected not-found after delete")
	}
}
```

- [ ] **Step 3: Run, confirm fail**

```bash
make migrate-up
go test ./internal/platform/database/postgres/ -run TestRolloutGroupRepo -v
```

- [ ] **Step 4: Implement Postgres repo**

Create `internal/platform/database/postgres/rollout_groups.go`:

```go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/rollout"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RolloutGroupRepo is a Postgres-backed rollout.RolloutGroupRepository.
type RolloutGroupRepo struct{ db *pgxpool.Pool }

// NewRolloutGroupRepo returns a new RolloutGroupRepo.
func NewRolloutGroupRepo(db *pgxpool.Pool) *RolloutGroupRepo { return &RolloutGroupRepo{db: db} }

var _ rollout.RolloutGroupRepository = (*RolloutGroupRepo)(nil)

// ErrRolloutGroupNotFound is returned when a group lookup fails.
var ErrRolloutGroupNotFound = errors.New("rollout group not found")

func (r *RolloutGroupRepo) Create(ctx context.Context, g *models.RolloutGroup) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	now := time.Now().UTC()
	g.CreatedAt, g.UpdatedAt = now, now
	if g.CoordinationPolicy == "" {
		g.CoordinationPolicy = models.CoordinationIndependent
	}
	_, err := r.db.Exec(ctx, `
        INSERT INTO rollout_groups (id, scope_type, scope_id, name, description, coordination_policy, created_by, created_at, updated_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		g.ID, g.ScopeType, g.ScopeID, g.Name, g.Description, g.CoordinationPolicy,
		g.CreatedBy, g.CreatedAt, g.UpdatedAt,
	)
	return err
}

func (r *RolloutGroupRepo) Get(ctx context.Context, id uuid.UUID) (*models.RolloutGroup, error) {
	row := r.db.QueryRow(ctx, selectGroupCols+` FROM rollout_groups WHERE id=$1`, id)
	return scanGroup(row)
}

func (r *RolloutGroupRepo) ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.RolloutGroup, error) {
	rows, err := r.db.Query(ctx, selectGroupCols+` FROM rollout_groups WHERE scope_type=$1 AND scope_id=$2 ORDER BY created_at DESC`, scopeType, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.RolloutGroup
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *RolloutGroupRepo) Update(ctx context.Context, g *models.RolloutGroup) error {
	g.UpdatedAt = time.Now().UTC()
	tag, err := r.db.Exec(ctx, `
        UPDATE rollout_groups SET name=$1, description=$2, coordination_policy=$3, updated_at=$4 WHERE id=$5`,
		g.Name, g.Description, g.CoordinationPolicy, g.UpdatedAt, g.ID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRolloutGroupNotFound
	}
	return nil
}

func (r *RolloutGroupRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM rollout_groups WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRolloutGroupNotFound
	}
	return nil
}

const selectGroupCols = `SELECT id, scope_type, scope_id, name, description, coordination_policy, created_by, created_at, updated_at`

type groupRowScanner interface {
	Scan(dest ...any) error
}

func scanGroup(r groupRowScanner) (*models.RolloutGroup, error) {
	var g models.RolloutGroup
	var createdBy *uuid.UUID
	if err := r.Scan(&g.ID, &g.ScopeType, &g.ScopeID, &g.Name, &g.Description, &g.CoordinationPolicy, &createdBy, &g.CreatedAt, &g.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRolloutGroupNotFound
		}
		return nil, err
	}
	g.CreatedBy = createdBy
	return &g, nil
}
```

Note: nullable UUID scanning pattern — Plan 1/2 used either `*uuid.UUID` directly or `sql.Null[uuid.UUID]`. Check `internal/platform/database/postgres/rollout.go` (Plan 1 strategies file) for the pattern used there and match it. The code above uses `*uuid.UUID` direct — if `pgx` can't scan directly into a `*uuid.UUID`, use the `sql.Null[uuid.UUID]` pattern seen in existing rollouts.go file.

- [ ] **Step 5: Run, confirm pass**

```bash
go test ./internal/platform/database/postgres/ -run TestRolloutGroupRepo -v -count=1
```

- [ ] **Step 6: Commit**

```bash
git add internal/rollout/group_repository.go internal/platform/database/postgres/rollout_groups.go internal/platform/database/postgres/rollout_groups_test.go
git commit -m "feat(rollout): RolloutGroup repository + Postgres implementation"
```

---

## Task 4: Rollout repo — `ListByRelease` + `SetReleaseID`

**Files:**
- Modify: `internal/rollout/rollout_repository.go`
- Modify: `internal/platform/database/postgres/rollouts.go`
- Modify: `internal/platform/database/postgres/rollouts_test.go`
- Modify: `internal/rollout/rollout_service_test.go` (fake repo stubs)

Plan 4 needs to list sibling rollouts within a group (for coordination) and attach existing rollouts to a group after the fact. Method names preserve the Plan 2 `release_id` column name for consistency — see plan's Terminology legend above.

- [ ] **Step 1: Add interface methods**

In `internal/rollout/rollout_repository.go`, add to the `RolloutRepository` interface (after `GetActiveByRule`):

```go
	ListByRelease(ctx context.Context, releaseID uuid.UUID) ([]*models.Rollout, error)
	SetReleaseID(ctx context.Context, rolloutID uuid.UUID, releaseID *uuid.UUID) error
```

- [ ] **Step 2: Add failing integration tests**

Append to `internal/platform/database/postgres/rollouts_test.go`:

```go
func TestRolloutRepo_ListByRelease(t *testing.T) {
	ctx := context.Background()
	repo := NewRolloutRepo(testDB(t))

	relID := uuid.New()
	r1 := sampleRollout(t)
	r1.ReleaseID = &relID
	r2 := sampleRollout(t)
	r2.ReleaseID = &relID
	r3 := sampleRollout(t) // no group
	if err := repo.Create(ctx, r1); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, r2); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, r3); err != nil {
		t.Fatal(err)
	}
	got, err := repo.ListByRelease(ctx, relID)
	if err != nil || len(got) != 2 {
		t.Fatalf("list: err=%v len=%d", err, len(got))
	}
}

func TestRolloutRepo_SetReleaseID(t *testing.T) {
	ctx := context.Background()
	repo := NewRolloutRepo(testDB(t))

	r := sampleRollout(t)
	if err := repo.Create(ctx, r); err != nil {
		t.Fatal(err)
	}
	relID := uuid.New()
	if err := repo.SetReleaseID(ctx, r.ID, &relID); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.Get(ctx, r.ID)
	if got.ReleaseID == nil || *got.ReleaseID != relID {
		t.Fatalf("release_id not set: %+v", got.ReleaseID)
	}

	// Clear via nil.
	if err := repo.SetReleaseID(ctx, r.ID, nil); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.Get(ctx, r.ID)
	if got.ReleaseID != nil {
		t.Fatalf("expected nil release_id, got %+v", got.ReleaseID)
	}
}
```

- [ ] **Step 3: Run, confirm fail**

```bash
go test ./internal/platform/database/postgres/ -run "TestRolloutRepo_ListByRelease|TestRolloutRepo_SetReleaseID" -v
```

- [ ] **Step 4: Implement**

Append to `internal/platform/database/postgres/rollouts.go`:

```go
// ListByRelease returns all rollouts (any status) attached to a rollout group.
// Method name mirrors the `release_id` column name from Plan 2 migration 050.
func (r *RolloutRepo) ListByRelease(ctx context.Context, releaseID uuid.UUID) ([]*models.Rollout, error) {
	return r.scanMany(ctx, `WHERE release_id=$1 ORDER BY created_at ASC`, releaseID)
}

// SetReleaseID attaches (or detaches with nil) a rollout to a group.
func (r *RolloutRepo) SetReleaseID(ctx context.Context, rolloutID uuid.UUID, releaseID *uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `UPDATE rollouts SET release_id=$1 WHERE id=$2`, releaseID, rolloutID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRolloutNotFound
	}
	return nil
}
```

- [ ] **Step 5: Update existing test fakes to satisfy the extended interface**

The `RolloutRepository` interface grew — any `fakeRolloutRepo` in the rollout package's tests needs the two new methods. Append to `fakeRolloutRepo` in `internal/rollout/rollout_service_test.go`:

```go
func (f *fakeRolloutRepo) ListByRelease(_ context.Context, relID uuid.UUID) ([]*models.Rollout, error) {
	var out []*models.Rollout
	for _, r := range f.rows {
		if r.ReleaseID != nil && *r.ReleaseID == relID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeRolloutRepo) SetReleaseID(_ context.Context, id uuid.UUID, relID *uuid.UUID) error {
	if r, ok := f.rows[id]; ok {
		r.ReleaseID = relID
		return nil
	}
	return errors.New("not found")
}
```

- [ ] **Step 6: Run, confirm pass**

```bash
go test ./internal/platform/database/postgres/ -run "TestRolloutRepo_ListByRelease|TestRolloutRepo_SetReleaseID" -v
go test ./internal/rollout/ -count=1
go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add internal/rollout/rollout_repository.go internal/platform/database/postgres/rollouts.go internal/platform/database/postgres/rollouts_test.go internal/rollout/rollout_service_test.go
git commit -m "feat(rollout): ListByRelease + SetReleaseID repo methods"
```

---

## Task 5: `RolloutGroupService` — CRUD + siblings

**Files:**
- Create: `internal/rollout/group_service.go`
- Create: `internal/rollout/group_service_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/rollout/group_service_test.go`:

```go
package rollout

import (
	"context"
	"errors"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

type fakeGroupRepo struct {
	rows map[uuid.UUID]*models.RolloutGroup
}

func newFakeGroupRepo() *fakeGroupRepo { return &fakeGroupRepo{rows: map[uuid.UUID]*models.RolloutGroup{}} }

func (f *fakeGroupRepo) Create(_ context.Context, g *models.RolloutGroup) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	f.rows[g.ID] = g
	return nil
}
func (f *fakeGroupRepo) Get(_ context.Context, id uuid.UUID) (*models.RolloutGroup, error) {
	if g, ok := f.rows[id]; ok {
		return g, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeGroupRepo) ListByScope(_ context.Context, st models.ScopeType, sid uuid.UUID) ([]*models.RolloutGroup, error) {
	var out []*models.RolloutGroup
	for _, g := range f.rows {
		if g.ScopeType == st && g.ScopeID == sid {
			out = append(out, g)
		}
	}
	return out, nil
}
func (f *fakeGroupRepo) Update(_ context.Context, g *models.RolloutGroup) error {
	if _, ok := f.rows[g.ID]; !ok {
		return errors.New("not found")
	}
	f.rows[g.ID] = g
	return nil
}
func (f *fakeGroupRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(f.rows, id)
	return nil
}

var _ RolloutGroupRepository = (*fakeGroupRepo)(nil)

func TestRolloutGroupService_Create_DefaultsIndependent(t *testing.T) {
	svc := NewRolloutGroupService(newFakeGroupRepo(), newFakeRolloutRepo())
	g := &models.RolloutGroup{ScopeType: models.ScopeOrg, ScopeID: uuid.New(), Name: "v1.1"}
	if err := svc.Create(context.Background(), g); err != nil {
		t.Fatal(err)
	}
	if g.CoordinationPolicy != models.CoordinationIndependent {
		t.Fatalf("expected default independent, got %s", g.CoordinationPolicy)
	}
}

func TestRolloutGroupService_Attach_SetsRolloutReleaseID(t *testing.T) {
	grpRepo := newFakeGroupRepo()
	roRepo := newFakeRolloutRepo()
	svc := NewRolloutGroupService(grpRepo, roRepo)

	g := &models.RolloutGroup{ID: uuid.New(), ScopeType: models.ScopeOrg, ScopeID: uuid.New(), Name: "v1", CoordinationPolicy: models.CoordinationIndependent}
	_ = grpRepo.Create(context.Background(), g)
	ro := &models.Rollout{ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive}
	roRepo.rows[ro.ID] = ro

	if err := svc.Attach(context.Background(), g.ID, ro.ID); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if ro.ReleaseID == nil || *ro.ReleaseID != g.ID {
		t.Fatalf("release_id not attached: %+v", ro.ReleaseID)
	}
}

func TestRolloutGroupService_Siblings_ExcludesOriginator(t *testing.T) {
	grpRepo := newFakeGroupRepo()
	roRepo := newFakeRolloutRepo()
	svc := NewRolloutGroupService(grpRepo, roRepo)

	gID := uuid.New()
	r1 := &models.Rollout{ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive, ReleaseID: &gID}
	r2 := &models.Rollout{ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive, ReleaseID: &gID}
	rDone := &models.Rollout{ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutSucceeded, ReleaseID: &gID}
	roRepo.rows[r1.ID] = r1
	roRepo.rows[r2.ID] = r2
	roRepo.rows[rDone.ID] = rDone

	siblings, err := svc.ActiveSiblings(context.Background(), gID, r1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(siblings) != 1 || siblings[0].ID != r2.ID {
		t.Fatalf("expected [r2], got %+v", siblings)
	}
}

func TestRolloutGroupService_Policy_Lookup(t *testing.T) {
	grpRepo := newFakeGroupRepo()
	roRepo := newFakeRolloutRepo()
	svc := NewRolloutGroupService(grpRepo, roRepo)
	g := &models.RolloutGroup{ID: uuid.New(), ScopeType: models.ScopeOrg, ScopeID: uuid.New(), Name: "x", CoordinationPolicy: models.CoordinationCascadeAbort}
	_ = grpRepo.Create(context.Background(), g)
	pol, err := svc.GetPolicy(context.Background(), g.ID)
	if err != nil || pol != models.CoordinationCascadeAbort {
		t.Fatalf("got pol=%s err=%v", pol, err)
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/ -run TestRolloutGroupService -v
```

- [ ] **Step 3: Implement service**

Create `internal/rollout/group_service.go`:

```go
package rollout

import (
	"context"
	"fmt"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// RolloutGroupService provides CRUD + attach semantics for RolloutGroup bundles.
type RolloutGroupService struct {
	groups   RolloutGroupRepository
	rollouts RolloutRepository
}

// NewRolloutGroupService builds a RolloutGroupService.
func NewRolloutGroupService(g RolloutGroupRepository, ro RolloutRepository) *RolloutGroupService {
	return &RolloutGroupService{groups: g, rollouts: ro}
}

// Create persists a group. Defaults coordination_policy to independent.
func (s *RolloutGroupService) Create(ctx context.Context, g *models.RolloutGroup) error {
	if g.CoordinationPolicy == "" {
		g.CoordinationPolicy = models.CoordinationIndependent
	}
	return s.groups.Create(ctx, g)
}

// Get returns a group by ID.
func (s *RolloutGroupService) Get(ctx context.Context, id uuid.UUID) (*models.RolloutGroup, error) {
	return s.groups.Get(ctx, id)
}

// List returns groups defined directly on the scope.
func (s *RolloutGroupService) List(ctx context.Context, st models.ScopeType, sid uuid.UUID) ([]*models.RolloutGroup, error) {
	return s.groups.ListByScope(ctx, st, sid)
}

// Update persists changes (name, description, coordination_policy).
func (s *RolloutGroupService) Update(ctx context.Context, g *models.RolloutGroup) error {
	return s.groups.Update(ctx, g)
}

// Delete removes a group row. Rollouts keep their release_id (dangling);
// app reads tolerate missing groups.
func (s *RolloutGroupService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.groups.Delete(ctx, id)
}

// Attach sets the rollout's release_id to this group. Both group and rollout must exist.
func (s *RolloutGroupService) Attach(ctx context.Context, groupID uuid.UUID, rolloutID uuid.UUID) error {
	if _, err := s.groups.Get(ctx, groupID); err != nil {
		return fmt.Errorf("group not found: %w", err)
	}
	id := groupID
	return s.rollouts.SetReleaseID(ctx, rolloutID, &id)
}

// Members returns all rollouts attached to a group.
func (s *RolloutGroupService) Members(ctx context.Context, groupID uuid.UUID) ([]*models.Rollout, error) {
	return s.rollouts.ListByRelease(ctx, groupID)
}

// ActiveSiblings returns rollouts in the group with status in
// (pending, active, paused, awaiting_approval), excluding the given origin id.
// Used by the coordinator to act on siblings when one rolls back.
func (s *RolloutGroupService) ActiveSiblings(ctx context.Context, groupID uuid.UUID, excludeID uuid.UUID) ([]*models.Rollout, error) {
	all, err := s.rollouts.ListByRelease(ctx, groupID)
	if err != nil {
		return nil, err
	}
	var out []*models.Rollout
	for _, r := range all {
		if r.ID == excludeID {
			continue
		}
		switch r.Status {
		case models.RolloutPending, models.RolloutActive, models.RolloutPaused, models.RolloutAwaitingApproval:
			out = append(out, r)
		}
	}
	return out, nil
}

// GetPolicy is a convenience lookup for the coordinator.
func (s *RolloutGroupService) GetPolicy(ctx context.Context, groupID uuid.UUID) (models.CoordinationPolicy, error) {
	g, err := s.groups.Get(ctx, groupID)
	if err != nil {
		return "", err
	}
	return g.CoordinationPolicy, nil
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/rollout/ -v -count=1
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/group_service.go internal/rollout/group_service_test.go
git commit -m "feat(rollout): RolloutGroupService (CRUD + attach + siblings)"
```

---

## Task 6: Coordinator

**Files:**
- Create: `internal/rollout/coordination.go`
- Create: `internal/rollout/coordination_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/rollout/coordination_test.go`:

```go
package rollout

import (
	"context"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

type capturedAction struct {
	siblingID uuid.UUID
	action    string // "pause" or "rollback"
}

type fakeSiblingActor struct {
	calls []capturedAction
}

func (f *fakeSiblingActor) Pause(_ context.Context, id uuid.UUID, _ uuid.UUID, _ string) error {
	f.calls = append(f.calls, capturedAction{siblingID: id, action: "pause"})
	return nil
}
func (f *fakeSiblingActor) Rollback(_ context.Context, id uuid.UUID, _ uuid.UUID, _ string) error {
	f.calls = append(f.calls, capturedAction{siblingID: id, action: "rollback"})
	return nil
}

func TestCoordinator_Independent_NoOp(t *testing.T) {
	grpRepo := newFakeGroupRepo()
	roRepo := newFakeRolloutRepo()
	gsvc := NewRolloutGroupService(grpRepo, roRepo)

	gID := uuid.New()
	_ = grpRepo.Create(context.Background(), &models.RolloutGroup{ID: gID, Name: "r", ScopeType: models.ScopeOrg, ScopeID: uuid.New(), CoordinationPolicy: models.CoordinationIndependent})

	originID := uuid.New()
	origin := &models.Rollout{ID: originID, Status: models.RolloutRolledBack, ReleaseID: &gID}
	sibling := &models.Rollout{ID: uuid.New(), Status: models.RolloutActive, ReleaseID: &gID}
	roRepo.rows[origin.ID] = origin
	roRepo.rows[sibling.ID] = sibling

	actor := &fakeSiblingActor{}
	coord := NewCoordinator(gsvc, actor)
	if err := coord.OnRollback(context.Background(), originID); err != nil {
		t.Fatal(err)
	}
	if len(actor.calls) != 0 {
		t.Fatalf("independent policy should not act, got %v", actor.calls)
	}
}

func TestCoordinator_PauseOnSiblingAbort(t *testing.T) {
	grpRepo := newFakeGroupRepo()
	roRepo := newFakeRolloutRepo()
	gsvc := NewRolloutGroupService(grpRepo, roRepo)

	gID := uuid.New()
	_ = grpRepo.Create(context.Background(), &models.RolloutGroup{ID: gID, ScopeType: models.ScopeOrg, ScopeID: uuid.New(), Name: "r", CoordinationPolicy: models.CoordinationPauseOnSiblingAbort})

	originID := uuid.New()
	s1 := &models.Rollout{ID: uuid.New(), Status: models.RolloutActive, ReleaseID: &gID}
	s2 := &models.Rollout{ID: uuid.New(), Status: models.RolloutPaused, ReleaseID: &gID}
	sDone := &models.Rollout{ID: uuid.New(), Status: models.RolloutSucceeded, ReleaseID: &gID}
	origin := &models.Rollout{ID: originID, Status: models.RolloutRolledBack, ReleaseID: &gID}
	for _, r := range []*models.Rollout{origin, s1, s2, sDone} {
		roRepo.rows[r.ID] = r
	}

	actor := &fakeSiblingActor{}
	coord := NewCoordinator(gsvc, actor)
	if err := coord.OnRollback(context.Background(), originID); err != nil {
		t.Fatal(err)
	}
	// Only active siblings get paused (state-machine rule: Pause requires Active).
	if len(actor.calls) != 1 || actor.calls[0].siblingID != s1.ID || actor.calls[0].action != "pause" {
		t.Fatalf("expected [pause s1], got %+v", actor.calls)
	}
}

func TestCoordinator_CascadeAbort(t *testing.T) {
	grpRepo := newFakeGroupRepo()
	roRepo := newFakeRolloutRepo()
	gsvc := NewRolloutGroupService(grpRepo, roRepo)

	gID := uuid.New()
	_ = grpRepo.Create(context.Background(), &models.RolloutGroup{ID: gID, ScopeType: models.ScopeOrg, ScopeID: uuid.New(), Name: "r", CoordinationPolicy: models.CoordinationCascadeAbort})

	originID := uuid.New()
	s1 := &models.Rollout{ID: uuid.New(), Status: models.RolloutActive, ReleaseID: &gID}
	s2 := &models.Rollout{ID: uuid.New(), Status: models.RolloutPaused, ReleaseID: &gID}
	origin := &models.Rollout{ID: originID, Status: models.RolloutRolledBack, ReleaseID: &gID}
	for _, r := range []*models.Rollout{origin, s1, s2} {
		roRepo.rows[r.ID] = r
	}

	actor := &fakeSiblingActor{}
	coord := NewCoordinator(gsvc, actor)
	if err := coord.OnRollback(context.Background(), originID); err != nil {
		t.Fatal(err)
	}
	gotIDs := map[uuid.UUID]string{}
	for _, c := range actor.calls {
		gotIDs[c.siblingID] = c.action
	}
	if gotIDs[s1.ID] != "rollback" || gotIDs[s2.ID] != "rollback" {
		t.Fatalf("expected rollback on both; got %+v", actor.calls)
	}
}

func TestCoordinator_NoGroup_NoOp(t *testing.T) {
	grpRepo := newFakeGroupRepo()
	roRepo := newFakeRolloutRepo()
	gsvc := NewRolloutGroupService(grpRepo, roRepo)

	originID := uuid.New()
	origin := &models.Rollout{ID: originID, Status: models.RolloutRolledBack} // no ReleaseID
	roRepo.rows[origin.ID] = origin

	actor := &fakeSiblingActor{}
	coord := NewCoordinator(gsvc, actor)
	if err := coord.OnRollback(context.Background(), originID); err != nil {
		t.Fatal(err)
	}
	if len(actor.calls) != 0 {
		t.Fatalf("no group → no coordination; got %+v", actor.calls)
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/ -run TestCoordinator -v
```

- [ ] **Step 3: Implement coordinator**

Create `internal/rollout/coordination.go`:

```go
package rollout

import (
	"context"
	"fmt"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// SiblingActor is the subset of RolloutService the coordinator needs to act on
// siblings. Injected at wiring time; defined here so the coordinator can be
// unit-tested without the full RolloutService surface.
type SiblingActor interface {
	Pause(ctx context.Context, rolloutID uuid.UUID, actor uuid.UUID, reason string) error
	Rollback(ctx context.Context, rolloutID uuid.UUID, actor uuid.UUID, reason string) error
}

// Coordinator applies a group's coordination_policy to sibling rollouts when
// any member rollout rolls back.
type Coordinator struct {
	groups *RolloutGroupService
	actor  SiblingActor
}

// NewCoordinator builds a Coordinator.
func NewCoordinator(gs *RolloutGroupService, a SiblingActor) *Coordinator {
	return &Coordinator{groups: gs, actor: a}
}

// OnRollback is invoked when a rollout transitions to rolled_back. It looks up
// the rollout's group (if any) and applies the group's coordination_policy to
// active siblings.
func (c *Coordinator) OnRollback(ctx context.Context, rolledBackID uuid.UUID) error {
	ro, err := c.groups.rollouts.Get(ctx, rolledBackID)
	if err != nil {
		return fmt.Errorf("lookup rolled-back rollout: %w", err)
	}
	if ro.ReleaseID == nil {
		return nil // no group → no coordination
	}
	policy, err := c.groups.GetPolicy(ctx, *ro.ReleaseID)
	if err != nil {
		return fmt.Errorf("lookup group policy: %w", err)
	}
	if policy == models.CoordinationIndependent {
		return nil
	}
	siblings, err := c.groups.ActiveSiblings(ctx, *ro.ReleaseID, rolledBackID)
	if err != nil {
		return fmt.Errorf("list siblings: %w", err)
	}

	systemActor := uuid.Nil
	reason := fmt.Sprintf("sibling_aborted:%s", rolledBackID)

	for _, s := range siblings {
		switch policy {
		case models.CoordinationPauseOnSiblingAbort:
			if s.Status == models.RolloutActive {
				if err := c.actor.Pause(ctx, s.ID, systemActor, reason); err != nil {
					return fmt.Errorf("pause sibling %s: %w", s.ID, err)
				}
			}
		case models.CoordinationCascadeAbort:
			if err := c.actor.Rollback(ctx, s.ID, systemActor, reason); err != nil {
				return fmt.Errorf("rollback sibling %s: %w", s.ID, err)
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/rollout/ -v -count=1
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/coordination.go internal/rollout/coordination_test.go
git commit -m "feat(rollout): Coordinator applies group policy on sibling rollback"
```

---

## Task 7: RolloutGroup HTTP handler

**Files:**
- Create: `internal/rollout/group_handler.go`
- Create: `internal/rollout/group_handler_test.go`

Endpoints under `/api/v1/orgs/:orgSlug/rollout-groups`:
- `GET  /rollout-groups` — list
- `POST /rollout-groups` — create
- `GET  /rollout-groups/:id` — detail with members
- `PUT  /rollout-groups/:id` — update name/description/coordination_policy
- `POST /rollout-groups/:id/attach` — body `{rollout_id}` sets Rollout.ReleaseID

- [ ] **Step 1: Write failing tests**

Create `internal/rollout/group_handler_test.go`:

```go
package rollout

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeGroupScopeResolver struct{ orgID uuid.UUID }

func (f *fakeGroupScopeResolver) ResolveOrg(_ *gin.Context) (uuid.UUID, error) {
	return f.orgID, nil
}

func newTestGroupHandler(t *testing.T) (*RolloutGroupHandler, *fakeGroupRepo, *fakeRolloutRepo, uuid.UUID) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	grpRepo := newFakeGroupRepo()
	roRepo := newFakeRolloutRepo()
	gsvc := NewRolloutGroupService(grpRepo, roRepo)
	orgID := uuid.New()
	resolver := &fakeGroupScopeResolver{orgID: orgID}
	h := NewRolloutGroupHandler(gsvc, resolver)
	return h, grpRepo, roRepo, orgID
}

func TestRolloutGroupHandler_Create_Defaults(t *testing.T) {
	h, _, _, _ := newTestGroupHandler(t)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	body, _ := json.Marshal(map[string]any{"name": "v1.1"})
	req := httptest.NewRequest("POST", "/api/v1/orgs/acme/rollout-groups", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var resp models.RolloutGroup
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.CoordinationPolicy != models.CoordinationIndependent {
		t.Fatalf("expected independent default, got %s", resp.CoordinationPolicy)
	}
}

func TestRolloutGroupHandler_List_ReturnsShape(t *testing.T) {
	h, grpRepo, _, orgID := newTestGroupHandler(t)
	_ = grpRepo.Create(nil, &models.RolloutGroup{ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "v1", CoordinationPolicy: models.CoordinationIndependent})
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	req := httptest.NewRequest("GET", "/api/v1/orgs/acme/rollout-groups", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d", w.Code)
	}
	var resp struct{ Items []models.RolloutGroup `json:"items"` }
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Items) != 1 || resp.Items[0].Name != "v1" {
		t.Fatalf("unexpected: %s", w.Body.String())
	}
}

func TestRolloutGroupHandler_Attach_SetsRolloutReleaseID(t *testing.T) {
	h, grpRepo, roRepo, orgID := newTestGroupHandler(t)
	g := &models.RolloutGroup{ID: uuid.New(), ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "v1", CoordinationPolicy: models.CoordinationIndependent}
	_ = grpRepo.Create(nil, g)
	ro := &models.Rollout{ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive}
	roRepo.rows[ro.ID] = ro

	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	body, _ := json.Marshal(map[string]any{"rollout_id": ro.ID.String()})
	req := httptest.NewRequest("POST", "/api/v1/orgs/acme/rollout-groups/"+g.ID.String()+"/attach", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if ro.ReleaseID == nil || *ro.ReleaseID != g.ID {
		t.Fatalf("not attached: %+v", ro.ReleaseID)
	}
}

func TestRolloutGroupHandler_UpdatePolicy(t *testing.T) {
	h, grpRepo, _, orgID := newTestGroupHandler(t)
	g := &models.RolloutGroup{ID: uuid.New(), ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "v1", CoordinationPolicy: models.CoordinationIndependent}
	_ = grpRepo.Create(nil, g)

	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	body, _ := json.Marshal(map[string]any{"coordination_policy": "cascade_abort", "name": "v1", "description": ""})
	req := httptest.NewRequest("PUT", "/api/v1/orgs/acme/rollout-groups/"+g.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	got, _ := grpRepo.Get(nil, g.ID)
	if got.CoordinationPolicy != models.CoordinationCascadeAbort {
		t.Fatalf("policy not updated: %s", got.CoordinationPolicy)
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/ -run TestRolloutGroupHandler -v
```

- [ ] **Step 3: Implement handler**

Create `internal/rollout/group_handler.go`:

```go
package rollout

import (
	"net/http"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RolloutGroupScopeResolver resolves :orgSlug to an org ID.
type RolloutGroupScopeResolver interface {
	ResolveOrg(c *gin.Context) (uuid.UUID, error)
}

// RolloutGroupHandler serves rollout-group endpoints.
type RolloutGroupHandler struct {
	svc   *RolloutGroupService
	scope RolloutGroupScopeResolver
}

// NewRolloutGroupHandler builds a RolloutGroupHandler.
func NewRolloutGroupHandler(svc *RolloutGroupService, scope RolloutGroupScopeResolver) *RolloutGroupHandler {
	return &RolloutGroupHandler{svc: svc, scope: scope}
}

// RegisterRoutes mounts routes under /orgs/:orgSlug/.
func (h *RolloutGroupHandler) RegisterRoutes(api *gin.RouterGroup) {
	org := api.Group("/orgs/:orgSlug")
	org.GET("/rollout-groups", h.list)
	org.POST("/rollout-groups", h.create)
	org.GET("/rollout-groups/:id", h.get)
	org.PUT("/rollout-groups/:id", h.update)
	org.POST("/rollout-groups/:id/attach", h.attach)
}

func (h *RolloutGroupHandler) list(c *gin.Context) {
	orgID, err := h.scope.ResolveOrg(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rows, err := h.svc.List(c.Request.Context(), models.ScopeOrg, orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

type createGroupReq struct {
	Name               string `json:"name"`
	Description        string `json:"description"`
	CoordinationPolicy string `json:"coordination_policy,omitempty"`
}

func (h *RolloutGroupHandler) create(c *gin.Context) {
	orgID, err := h.scope.ResolveOrg(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var req createGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	policy := models.CoordinationPolicy(req.CoordinationPolicy)
	if policy == "" {
		policy = models.CoordinationIndependent
	}
	switch policy {
	case models.CoordinationIndependent, models.CoordinationPauseOnSiblingAbort, models.CoordinationCascadeAbort:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid coordination_policy"})
		return
	}
	g := &models.RolloutGroup{
		ScopeType:          models.ScopeOrg,
		ScopeID:            orgID,
		Name:               req.Name,
		Description:        req.Description,
		CoordinationPolicy: policy,
	}
	if err := h.svc.Create(c.Request.Context(), g); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, g)
}

func (h *RolloutGroupHandler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	g, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	members, _ := h.svc.Members(c.Request.Context(), id)
	c.JSON(http.StatusOK, gin.H{"group": g, "members": members})
}

type updateGroupReq struct {
	Name               string `json:"name"`
	Description        string `json:"description"`
	CoordinationPolicy string `json:"coordination_policy"`
}

func (h *RolloutGroupHandler) update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	existing, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	var req updateGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	policy := models.CoordinationPolicy(req.CoordinationPolicy)
	switch policy {
	case models.CoordinationIndependent, models.CoordinationPauseOnSiblingAbort, models.CoordinationCascadeAbort:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid coordination_policy"})
		return
	}
	existing.Name = req.Name
	existing.Description = req.Description
	existing.CoordinationPolicy = policy
	if err := h.svc.Update(c.Request.Context(), existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, existing)
}

type attachReq struct {
	RolloutID uuid.UUID `json:"rollout_id"`
}

func (h *RolloutGroupHandler) attach(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req attachReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.RolloutID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rollout_id is required"})
		return
	}
	if err := h.svc.Attach(c.Request.Context(), id, req.RolloutID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/rollout/ -v -count=1
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/group_handler.go internal/rollout/group_handler_test.go
git commit -m "feat(rollout): RolloutGroup HTTP handler (list/get/create/update/attach)"
```

---

## Task 8: Wire RolloutGroup handler + Coordinator in cmd/api

**Files:**
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Construct repos, service, coordinator, handler**

Find the existing Plan 2 wiring block (after `rolloutAttacher := rollout.NewAttacher(...)`). Add:

```go
// ---- Plan 4: Rollout groups + coordination ----
groupRepo := postgres.NewRolloutGroupRepo(db.Pool)
groupSvc := rollout.NewRolloutGroupService(groupRepo, rolloutRepo)

// Coordinator acts on siblings when a rollout rolls back.
// rolloutExecSvc satisfies SiblingActor via its Pause/Rollback methods.
coordinator := rollout.NewCoordinator(groupSvc, rolloutExecSvc)

// The Plan 1 scope resolver already implements ResolveOrg(c *gin.Context) (uuid.UUID, error).
// Reuse it for the group handler.
rollout.NewRolloutGroupHandler(groupSvc, rolloutScopeResolver).RegisterRoutes(api)
```

Match existing variable names for `db.Pool`, `rolloutRepo`, `rolloutExecSvc`, `rolloutScopeResolver`, `api`.

- [ ] **Step 2: Subscribe coordinator to `rollouts.rollout.rolled_back`**

Below the existing `rollouts.rollout.created` subscriber from Plan 2, add:

```go
go func() {
    _ = natsSubscriber.Subscribe(ctx, "rollouts.rollout.rolled_back", func(msg []byte) {
        var payload struct {
            RolloutID string `json:"rollout_id"`
        }
        if err := json.Unmarshal(msg, &payload); err != nil {
            logger.Warn("coordinator: bad payload", "error", err)
            return
        }
        id, err := uuid.Parse(payload.RolloutID)
        if err != nil {
            return
        }
        if err := coordinator.OnRollback(ctx, id); err != nil {
            logger.Warn("coordinator: apply failed", "rollout_id", id, "error", err)
        }
    })
}()
```

Match the existing subscriber wiring pattern in this file.

- [ ] **Step 3: Verify the scope resolver is compatible**

The Plan 1 `rolloutScopeAdapter` (or equivalent) already implements `ResolveOrg(c *gin.Context) (uuid.UUID, error)`. Confirm with `grep -n "ResolveOrg" cmd/api/main.go`. If missing, the handler's interface requires one — wire the existing scope adapter directly.

- [ ] **Step 4: Build + verify routes**

```bash
cd /Users/sgamel/git/DeploySentry-releases
go build ./cmd/api
./api 2>&1 | head -300 | grep -E "ROUTE:.*/rollout-groups" | head -10
```

Expected 5 new routes: `GET/POST /rollout-groups`, `GET/PUT /rollout-groups/:id`, `POST /rollout-groups/:id/attach`.

- [ ] **Step 5: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat(api): wire RolloutGroup handler and Coordinator subscriber"
```

---

## Task 9: CLI `ds rollout-groups` + docs + initiatives

**Files:**
- Create: `cmd/cli/rolloutgroups.go`
- Modify: `docs/Rollout_Strategies.md`
- Modify: `docs/Current_Initiatives.md`

- [ ] **Step 1: Create the CLI**

Create `cmd/cli/rolloutgroups.go`:

```go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var rolloutGroupsCmd = &cobra.Command{
	Use:     "rollout-groups",
	Aliases: []string{"rollout-group", "rg"},
	Short:   "Manage rollout groups (bundles of related rollouts)",
}

var rolloutGroupsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List rollout groups in an org",
	RunE: func(cmd *cobra.Command, args []string) error {
		org, _ := cmd.Flags().GetString("org")
		if org == "" {
			return fmt.Errorf("--org is required")
		}
		body, err := apiGET("/api/v1/orgs/" + org + "/rollout-groups")
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	},
}

var rolloutGroupsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show a rollout group and its members",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		org, _ := cmd.Flags().GetString("org")
		body, err := apiGET("/api/v1/orgs/" + org + "/rollout-groups/" + args[0])
		if err != nil {
			return err
		}
		var pretty any
		_ = json.Unmarshal(body, &pretty)
		out, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Println(string(out))
		return nil
	},
}

var rolloutGroupsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a rollout group",
	RunE: func(cmd *cobra.Command, args []string) error {
		org, _ := cmd.Flags().GetString("org")
		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("description")
		policy, _ := cmd.Flags().GetString("policy")
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		payload := map[string]any{"name": name, "description": desc}
		if policy != "" {
			payload["coordination_policy"] = policy
		}
		b, _ := json.Marshal(payload)
		body, err := apiPOSTRaw("/api/v1/orgs/"+org+"/rollout-groups", "application/json", b)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	},
}

var rolloutGroupsAttachCmd = &cobra.Command{
	Use:   "attach <group-id>",
	Short: "Attach a rollout to a group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		org, _ := cmd.Flags().GetString("org")
		rollout, _ := cmd.Flags().GetString("rollout")
		if rollout == "" {
			return fmt.Errorf("--rollout is required")
		}
		b, _ := json.Marshal(map[string]string{"rollout_id": rollout})
		body, err := apiPOSTRaw("/api/v1/orgs/"+org+"/rollout-groups/"+args[0]+"/attach", "application/json", b)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	},
}

func init() {
	for _, c := range []*cobra.Command{rolloutGroupsListCmd, rolloutGroupsGetCmd, rolloutGroupsCreateCmd, rolloutGroupsAttachCmd} {
		c.Flags().String("org", "", "Organization slug (required)")
	}
	rolloutGroupsCreateCmd.Flags().String("name", "", "Group name (required)")
	rolloutGroupsCreateCmd.Flags().String("description", "", "Group description")
	rolloutGroupsCreateCmd.Flags().String("policy", "", "Coordination policy: independent | pause_on_sibling_abort | cascade_abort")
	rolloutGroupsAttachCmd.Flags().String("rollout", "", "Rollout ID to attach (required)")

	rolloutGroupsCmd.AddCommand(rolloutGroupsListCmd, rolloutGroupsGetCmd, rolloutGroupsCreateCmd, rolloutGroupsAttachCmd)
	rootCmd.AddCommand(rolloutGroupsCmd)
}
```

If `apiGET` / `apiPOSTRaw` free functions don't exist in `cmd/cli/`, use `client.get(...)` / `client.postRaw(...)` matching how Plans 2/3 CLI files do it. Check: `grep -n "apiGET\|client.get\|client.postRaw" cmd/cli/*.go | head -5`.

- [ ] **Step 2: Build and smoke test**

```bash
go build ./cmd/cli
./cli rollout-groups --help
./cli rollout-groups create --help
```

Expected: four subcommands visible; flags sensible.

- [ ] **Step 3: Append docs**

Append to `docs/Rollout_Strategies.md`:

```markdown
## Rollout groups

A **Rollout group** bundles related rollouts (e.g., a deploy + its associated flag enable) under one umbrella. Groups are optional — rollouts without a group behave exactly as before.

**Naming note:** "Rollout group" (not "Release") avoids collision with the existing `releases` concept in this codebase (which tracks version/commit metadata).

### Why group rollouts?

For UI: "v1.2 shipped these 3 rollouts" — a single bundle view instead of scattered entries.

For coordination: set the group's `coordination_policy` so that if any member rollout rolls back, siblings react:

| Policy | Effect on siblings |
|---|---|
| `independent` *(default)* | No effect. Siblings keep running. |
| `pause_on_sibling_abort` | Active siblings are paused. Operator decides next. |
| `cascade_abort` | Active/paused siblings are rolled back with reason `sibling_aborted:<id>`. |

### CLI

```
ds rollout-groups create --org acme --name "v1.2 ship" --policy pause_on_sibling_abort
ds rollout-groups attach <group-id> --org acme --rollout <rollout-id>
ds rollout-groups list --org acme
ds rollout-groups get <group-id> --org acme
```

### Attaching at rollout creation

Both deploy rollouts and config rollouts accept `release_id` in their attach request body — a group ID (column name preserved from Plan 2):

```json
{ "rollout": { "strategy_name": "prod-canary", "release_id": "<group-uuid>" } }
```

### Coordination limitations

- Coordination fires on `rollouts.rollout.rolled_back` NATS events. If a rollout was rolled back before Plan 4 shipped, no coordination replay happens.
- Sibling rollback via `cascade_abort` calls `RolloutService.Rollback` which updates state but does NOT trigger applicator `Revert`. The engine performs Revert only on its own abort path; manually-triggered rollbacks (whether by operator or cascade) leave traffic/rule unchanged in-place. A follow-up should wire manual rollback to also call Revert.
```

- [ ] **Step 4: Update initiatives**

In `docs/Current_Initiatives.md`:
- Find the "Configurable Rollout Strategies" row.
- Phase → `Implementation (Plans 1+2+3+4)`.
- Add `[Plan 4](./superpowers/plans/2026-04-19-releases-and-coordination.md)` to the plan list.
- Notes → `Plans 1 (templates) + 2 (engine + deploy) + 3 (config) + 4 (rollout groups + coordination) merged. Plan 5 (web UI) to follow.`
- Bump `> Last updated:` to `2026-04-19`.

- [ ] **Step 5: Full test run**

```bash
go build ./...
go test ./internal/rollout/... ./internal/models/ ./internal/platform/database/postgres/ ./internal/flags/ -count=1
```

All must pass (ignore pre-existing `TestRollback*` failures in `internal/deploy/` if still present).

- [ ] **Step 6: Commit**

```bash
git add cmd/cli/rolloutgroups.go docs/Rollout_Strategies.md docs/Current_Initiatives.md
git commit -m "feat(cli+docs): ds rollout-groups commands + Plan 4 docs"
```

---

## Definition of Done

- All 9 tasks committed individually on branch `feature/releases-and-coordination`.
- `go build ./...` clean.
- Tests pass: rollout + models + postgres + flags packages.
- 5 new `/rollout-groups` routes visible in API startup log.
- Creating a group → attaching 2 rollouts → rolling back one → observe sibling pause or sibling rollback per coordination policy (manual integration check).
- Docs + initiative row updated.

## Not in scope (deferred)

- Manual rollback triggering applicator `Revert` (pre-existing Plan 2 gap; documented).
- Web UI (Plan 5).
- Group templates / defaults.
- Renaming `rollouts.release_id` column to `group_id` (cosmetic cleanup; out of scope).
- Automatic group creation when multiple rollouts are submitted together.

## Self-Review Notes

- **Spec coverage**: implements spec sections for the bundle entity (renamed RolloutGroup), coordination policies, attach/list/get API, CLI. Spec used "Release" terminology — plan deliberately renames to avoid collision with the existing `releases` table and `models.Release` type.
- **Type consistency**: `RolloutGroup` + `CoordinationPolicy` types used consistently across T2 (model), T3 (repo + Postgres CHECK), T5 (service defaults), T6 (coordinator switch), T7 (handler validation), T9 (docs). `RolloutGroupRepository` from T3 consumed by T5. `SiblingActor` interface in T6 matches `RolloutService.Pause`/`Rollback` signatures from Plan 2 Task 8.
- **Preserved Plan 2 names**: `rollouts.release_id` column and `Rollout.ReleaseID` field kept as-is to minimize blast radius. Semantically now points at a `rollout_groups.id`.
- **Placeholder scan**: No "TBD"/"TODO"/"fill in". Task 8 Step 1 says "Match existing variable names" — that's an actual instruction, not a placeholder.
