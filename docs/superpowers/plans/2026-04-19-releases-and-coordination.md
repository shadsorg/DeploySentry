# Releases & Coordination Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the Release entity — a group for related rollouts (e.g., "v1.1 deploy + feature flag enable") — and optional cross-rollout coordination policies that react to sibling aborts. Backward-compatible: rollouts without a release continue to behave exactly as in Plans 2+3.

**Architecture:** A new `releases` table stores the bundle plus its `coordination_policy` (`independent`, `pause_on_sibling_abort`, `cascade_abort`). `Rollout.ReleaseID` is already wired from Plan 2 — this plan adds the Release CRUD, a `ReleaseService`, an HTTP handler, and an event-driven **CoordinationSubscriber** that listens for `rollouts.rollout.rolled_back` NATS events and applies the release's policy to sibling rollouts. CLI and docs complete the loop.

**Tech Stack:** Go 1.22+, PostgreSQL (schema `deploy`), gin, NATS JetStream (reuse existing pub/sub), cobra. Builds on Plans 1+2+3 (already merged to main).

**Spec:** `docs/superpowers/specs/2026-04-18-configurable-rollout-strategies-design.md`
**Prior plans:** `docs/superpowers/plans/2026-04-18-rollout-strategies-foundation.md`, `-rollout-engine-deploy.md`, `-rollout-config-integration.md`

---

## File Structure

**New files:**

```
migrations/
  052_create_releases.up.sql
  052_create_releases.down.sql

internal/models/
  release.go              # Release + CoordinationPolicy enum

internal/rollout/
  release_repository.go   # ReleaseRepository interface
  release_service.go      # ReleaseService (CRUD + attach + sibling lookup)
  release_service_test.go
  release_handler.go      # HTTP: GET list/get, POST create, PUT update, POST attach
  release_handler_test.go
  coordination.go         # CoordinationSubscriber: event-driven sibling handling
  coordination_test.go

internal/platform/database/postgres/
  releases.go             # ReleaseRepo postgres impl
  releases_test.go

cmd/cli/
  releases.go             # ds releases {list, get, create, attach}
```

**Modified files:**

```
internal/rollout/rollout_repository.go  # Add ListByRelease + SetReleaseID repo methods
internal/platform/database/postgres/rollouts.go  # ListByRelease + SetReleaseID impls
cmd/api/main.go                        # Wire ReleaseService + handler + coordination subscriber
docs/Rollout_Strategies.md             # New "Releases" section
docs/Current_Initiatives.md            # Phase → Implementation (Plans 1+2+3+4)
```

**Coordination semantics** (settled in spec):
- `independent` (default) — sibling rollback has no effect on other rollouts in the release.
- `pause_on_sibling_abort` — when any rollout in the release rolls back, active siblings transition to `paused` with reason `sibling_aborted:<id>`.
- `cascade_abort` — when any rollout rolls back, active siblings transition to `rolled_back` with reason `sibling_aborted:<id>` cascaded through `RolloutService.Rollback`.

**Event flow:**
Rollout A rolls back (either by engine abort or operator `POST /rollback`) → `rollouts.rollout.rolled_back` NATS message published with `{rollout_id}` → `CoordinationSubscriber` consumes → looks up rollout A's `release_id` → if set, loads Release's `coordination_policy` → lists sibling rollouts with same `release_id` in active state → calls `RolloutService.Pause(sibling)` or `.Rollback(sibling)` accordingly.

**Loop-prevention:** the subscriber filters out the originating rollout when listing siblings (only siblings with a DIFFERENT id). When cascade_abort triggers a sibling rollback, the sibling's own `rolled_back` event is published, but the subscriber sees no other actives in the release so the cascade stops naturally.

**What's NOT in scope here (acknowledged):** sibling **Revert** (applicator traffic/rule revert) is triggered via the existing `RolloutService.Rollback` path, which currently updates state only — engine performs applicator.Revert on its own path. Plan 2's gap (manual rollback doesn't invoke Revert) is documented but not fixed here; future work.

---

## Task 1: Migration — releases table

**Files:**
- Create: `migrations/052_create_releases.up.sql`
- Create: `migrations/052_create_releases.down.sql`

- [ ] **Step 1: Write up migration**

Create `migrations/052_create_releases.up.sql`:

```sql
-- Releases: optional bundle grouping related rollouts with coordination policy.
CREATE TABLE releases (
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
CREATE INDEX idx_releases_scope ON releases(scope_type, scope_id);
```

Note: `rollouts.release_id` already exists (Plan 2 migration 050) but has no FK — deliberately, so releases can be deleted without cascading into the rollout ledger. Enforce via app logic only.

- [ ] **Step 2: Write down migration**

Create `migrations/052_create_releases.down.sql`:

```sql
DROP TABLE IF EXISTS releases;
```

- [ ] **Step 3: Run migrations and verify**

```bash
cd /Users/sgamel/git/DeploySentry-releases
make dev-up
make migrate-up
psql "$(grep '^MIGRATE_DSN' Makefile | head -1 | cut -d= -f2-)" -c "SET search_path TO deploy; \d releases"
make migrate-down
make migrate-up
```

Expected: columns, index, CHECK constraint. Migrate-down cleans; re-apply succeeds.

- [ ] **Step 4: Commit**

```bash
git add migrations/052_create_releases.up.sql migrations/052_create_releases.down.sql
git commit -m "feat(db): add releases table"
```

---

## Task 2: Release Go model

**Files:**
- Create: `internal/models/release.go`
- Create: `internal/models/release_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/models/release_test.go`:

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

- [ ] **Step 3: Create the model**

Create `internal/models/release.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

// CoordinationPolicy determines how a Release reacts when any of its member
// rollouts rolls back.
type CoordinationPolicy string

const (
	// CoordinationIndependent means sibling rollbacks do not affect each other.
	CoordinationIndependent CoordinationPolicy = "independent"
	// CoordinationPauseOnSiblingAbort means active siblings are paused when any
	// rollout in the release rolls back.
	CoordinationPauseOnSiblingAbort CoordinationPolicy = "pause_on_sibling_abort"
	// CoordinationCascadeAbort means active siblings are rolled back when any
	// rollout in the release rolls back.
	CoordinationCascadeAbort CoordinationPolicy = "cascade_abort"
)

// Release is an optional bundle grouping related rollouts. Rollouts reference
// their release via Rollout.ReleaseID.
type Release struct {
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
git add internal/models/release.go internal/models/release_test.go
git commit -m "feat(models): add Release + CoordinationPolicy types"
```

---

## Task 3: Release repository (interface + Postgres + tests)

**Files:**
- Create: `internal/rollout/release_repository.go`
- Create: `internal/platform/database/postgres/releases.go`
- Create: `internal/platform/database/postgres/releases_test.go`

- [ ] **Step 1: Define interface**

Create `internal/rollout/release_repository.go`:

```go
package rollout

import (
	"context"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// ReleaseRepository persists Release rows.
type ReleaseRepository interface {
	Create(ctx context.Context, r *models.Release) error
	Get(ctx context.Context, id uuid.UUID) (*models.Release, error)
	ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.Release, error)
	Update(ctx context.Context, r *models.Release) error
	Delete(ctx context.Context, id uuid.UUID) error
}
```

- [ ] **Step 2: Write failing integration test**

Create `internal/platform/database/postgres/releases_test.go`:

```go
package postgres

import (
	"context"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

func sampleRelease(orgID uuid.UUID) *models.Release {
	return &models.Release{
		ScopeType:          models.ScopeOrg,
		ScopeID:            orgID,
		Name:               "v1.1-" + uuid.NewString()[:8],
		Description:        "test bundle",
		CoordinationPolicy: models.CoordinationIndependent,
	}
}

func TestReleaseRepo_CreateGet(t *testing.T) {
	ctx := context.Background()
	repo := NewReleaseRepo(testDB(t))
	orgID := uuid.New()
	r := sampleRelease(orgID)
	if err := repo.Create(ctx, r); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.Get(ctx, r.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != r.Name || got.CoordinationPolicy != models.CoordinationIndependent {
		t.Fatalf("mismatch: %+v", got)
	}
}

func TestReleaseRepo_UpdatePolicy(t *testing.T) {
	ctx := context.Background()
	repo := NewReleaseRepo(testDB(t))
	r := sampleRelease(uuid.New())
	if err := repo.Create(ctx, r); err != nil {
		t.Fatal(err)
	}
	r.CoordinationPolicy = models.CoordinationCascadeAbort
	r.Description = "updated"
	if err := repo.Update(ctx, r); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.Get(ctx, r.ID)
	if got.CoordinationPolicy != models.CoordinationCascadeAbort || got.Description != "updated" {
		t.Fatalf("update not applied: %+v", got)
	}
}

func TestReleaseRepo_ListByScope(t *testing.T) {
	ctx := context.Background()
	repo := NewReleaseRepo(testDB(t))
	orgID := uuid.New()
	r1 := sampleRelease(orgID)
	r2 := sampleRelease(orgID)
	r3 := sampleRelease(uuid.New()) // different org
	if err := repo.Create(ctx, r1); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, r2); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, r3); err != nil {
		t.Fatal(err)
	}
	got, err := repo.ListByScope(ctx, models.ScopeOrg, orgID)
	if err != nil || len(got) != 2 {
		t.Fatalf("list: err=%v len=%d", err, len(got))
	}
}

func TestReleaseRepo_Delete(t *testing.T) {
	ctx := context.Background()
	repo := NewReleaseRepo(testDB(t))
	r := sampleRelease(uuid.New())
	if err := repo.Create(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, r.ID); err == nil {
		t.Fatalf("expected not-found after delete")
	}
}
```

- [ ] **Step 3: Run, confirm fail**

```bash
make migrate-up
go test ./internal/platform/database/postgres/ -run TestReleaseRepo -v
```

Expected: `NewReleaseRepo` undefined.

- [ ] **Step 4: Implement Postgres repo**

Create `internal/platform/database/postgres/releases.go`:

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

// ReleaseRepo is a Postgres-backed rollout.ReleaseRepository.
type ReleaseRepo struct{ db *pgxpool.Pool }

// NewReleaseRepo returns a new ReleaseRepo.
func NewReleaseRepo(db *pgxpool.Pool) *ReleaseRepo { return &ReleaseRepo{db: db} }

var _ rollout.ReleaseRepository = (*ReleaseRepo)(nil)

// ErrReleaseNotFound is returned when a release lookup fails.
var ErrReleaseNotFound = errors.New("release not found")

func (r *ReleaseRepo) Create(ctx context.Context, rel *models.Release) error {
	if rel.ID == uuid.Nil {
		rel.ID = uuid.New()
	}
	now := time.Now().UTC()
	rel.CreatedAt, rel.UpdatedAt = now, now
	if rel.CoordinationPolicy == "" {
		rel.CoordinationPolicy = models.CoordinationIndependent
	}
	_, err := r.db.Exec(ctx, `
        INSERT INTO releases (id, scope_type, scope_id, name, description, coordination_policy, created_by, created_at, updated_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		rel.ID, rel.ScopeType, rel.ScopeID, rel.Name, rel.Description, rel.CoordinationPolicy,
		rel.CreatedBy, rel.CreatedAt, rel.UpdatedAt,
	)
	return err
}

func (r *ReleaseRepo) Get(ctx context.Context, id uuid.UUID) (*models.Release, error) {
	row := r.db.QueryRow(ctx, selectReleaseCols+` FROM releases WHERE id=$1`, id)
	rel, err := scanRelease(row)
	if err != nil {
		return nil, err
	}
	return rel, nil
}

func (r *ReleaseRepo) ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.Release, error) {
	rows, err := r.db.Query(ctx, selectReleaseCols+` FROM releases WHERE scope_type=$1 AND scope_id=$2 ORDER BY created_at DESC`, scopeType, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Release
	for rows.Next() {
		rel, err := scanRelease(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rel)
	}
	return out, rows.Err()
}

func (r *ReleaseRepo) Update(ctx context.Context, rel *models.Release) error {
	rel.UpdatedAt = time.Now().UTC()
	tag, err := r.db.Exec(ctx, `
        UPDATE releases SET name=$1, description=$2, coordination_policy=$3, updated_at=$4 WHERE id=$5`,
		rel.Name, rel.Description, rel.CoordinationPolicy, rel.UpdatedAt, rel.ID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrReleaseNotFound
	}
	return nil
}

func (r *ReleaseRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM releases WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrReleaseNotFound
	}
	return nil
}

const selectReleaseCols = `SELECT id, scope_type, scope_id, name, description, coordination_policy, created_by, created_at, updated_at`

// rowScanner matches the minimal surface of pgx.Rows / pgx.Row.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanRelease(r rowScanner) (*models.Release, error) {
	var rel models.Release
	var createdBy sql.Null[uuid.UUID]
	if err := r.Scan(&rel.ID, &rel.ScopeType, &rel.ScopeID, &rel.Name, &rel.Description, &rel.CoordinationPolicy, &createdBy, &rel.CreatedAt, &rel.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrReleaseNotFound
		}
		return nil, err
	}
	if createdBy.Valid {
		id := createdBy.V
		rel.CreatedBy = &id
	}
	return &rel, nil
}
```

Note: if Plan 2's other Postgres files use `*uuid.UUID` scanning directly instead of `sql.Null[uuid.UUID]`, adapt. Check the existing `rollouts.go` for the pattern.

- [ ] **Step 5: Run, confirm pass**

```bash
go test ./internal/platform/database/postgres/ -run TestReleaseRepo -v -count=1
```

- [ ] **Step 6: Commit**

```bash
git add internal/rollout/release_repository.go internal/platform/database/postgres/releases.go internal/platform/database/postgres/releases_test.go
git commit -m "feat(rollout): Release repository + Postgres implementation"
```

---

## Task 4: Rollout repo — `ListByRelease` + `SetReleaseID`

**Files:**
- Modify: `internal/rollout/rollout_repository.go`
- Modify: `internal/platform/database/postgres/rollouts.go`
- Modify: `internal/platform/database/postgres/rollouts_test.go`

Plan 4 needs to list sibling rollouts within a release (for coordination) and attach existing rollouts to a release after the fact (for the `POST /releases/:id/attach` endpoint).

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
	r3 := sampleRollout(t) // no release
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
// ListByRelease returns all rollouts (any status) attached to a release.
func (r *RolloutRepo) ListByRelease(ctx context.Context, releaseID uuid.UUID) ([]*models.Rollout, error) {
	return r.scanMany(ctx, `WHERE release_id=$1 ORDER BY created_at ASC`, releaseID)
}

// SetReleaseID attaches (or detaches with nil) a rollout to a release.
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

- [ ] **Step 5: Run, confirm pass**

```bash
go test ./internal/platform/database/postgres/ -run "TestRolloutRepo_ListByRelease|TestRolloutRepo_SetReleaseID" -v
go build ./...
```

Note: the `RolloutRepository` interface grew, so any fakes in other test files (e.g., `fakeRolloutRepo` in `internal/rollout/rollout_service_test.go`) need the two new methods stubbed. Add minimal implementations:

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

After that, run `go test ./internal/rollout/ -count=1` and confirm it still passes.

- [ ] **Step 6: Commit**

```bash
git add internal/rollout/rollout_repository.go internal/platform/database/postgres/rollouts.go internal/platform/database/postgres/rollouts_test.go internal/rollout/rollout_service_test.go
git commit -m "feat(rollout): ListByRelease + SetReleaseID repo methods"
```

---

## Task 5: `ReleaseService` — CRUD + attach

**Files:**
- Create: `internal/rollout/release_service.go`
- Create: `internal/rollout/release_service_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/rollout/release_service_test.go`:

```go
package rollout

import (
	"context"
	"errors"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

type fakeReleaseRepo struct {
	rows map[uuid.UUID]*models.Release
}

func newFakeReleaseRepo() *fakeReleaseRepo { return &fakeReleaseRepo{rows: map[uuid.UUID]*models.Release{}} }

func (f *fakeReleaseRepo) Create(_ context.Context, r *models.Release) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	f.rows[r.ID] = r
	return nil
}
func (f *fakeReleaseRepo) Get(_ context.Context, id uuid.UUID) (*models.Release, error) {
	if r, ok := f.rows[id]; ok {
		return r, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeReleaseRepo) ListByScope(_ context.Context, st models.ScopeType, sid uuid.UUID) ([]*models.Release, error) {
	var out []*models.Release
	for _, r := range f.rows {
		if r.ScopeType == st && r.ScopeID == sid {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeReleaseRepo) Update(_ context.Context, r *models.Release) error {
	if _, ok := f.rows[r.ID]; !ok {
		return errors.New("not found")
	}
	f.rows[r.ID] = r
	return nil
}
func (f *fakeReleaseRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(f.rows, id)
	return nil
}

var _ ReleaseRepository = (*fakeReleaseRepo)(nil)

// Use the existing fakeRolloutRepo from rollout_service_test.go as the rollouts repo.

func TestReleaseService_Create_DefaultsIndependent(t *testing.T) {
	svc := NewReleaseService(newFakeReleaseRepo(), newFakeRolloutRepo())
	r := &models.Release{ScopeType: models.ScopeOrg, ScopeID: uuid.New(), Name: "v1.1"}
	if err := svc.Create(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if r.CoordinationPolicy != models.CoordinationIndependent {
		t.Fatalf("expected default independent, got %s", r.CoordinationPolicy)
	}
}

func TestReleaseService_Attach_SetsRolloutReleaseID(t *testing.T) {
	relRepo := newFakeReleaseRepo()
	roRepo := newFakeRolloutRepo()
	svc := NewReleaseService(relRepo, roRepo)

	// Seed a release and a rollout.
	rel := &models.Release{ID: uuid.New(), ScopeType: models.ScopeOrg, ScopeID: uuid.New(), Name: "v1", CoordinationPolicy: models.CoordinationIndependent}
	_ = relRepo.Create(context.Background(), rel)
	ro := &models.Rollout{ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive}
	roRepo.rows[ro.ID] = ro

	if err := svc.Attach(context.Background(), rel.ID, ro.ID); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if ro.ReleaseID == nil || *ro.ReleaseID != rel.ID {
		t.Fatalf("release_id not attached: %+v", ro.ReleaseID)
	}
}

func TestReleaseService_Siblings_ExcludesOriginator(t *testing.T) {
	relRepo := newFakeReleaseRepo()
	roRepo := newFakeRolloutRepo()
	svc := NewReleaseService(relRepo, roRepo)

	relID := uuid.New()
	r1 := &models.Rollout{ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive, ReleaseID: &relID}
	r2 := &models.Rollout{ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive, ReleaseID: &relID}
	r3 := &models.Rollout{ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutSucceeded, ReleaseID: &relID}
	roRepo.rows[r1.ID] = r1
	roRepo.rows[r2.ID] = r2
	roRepo.rows[r3.ID] = r3

	siblings, err := svc.ActiveSiblings(context.Background(), relID, r1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(siblings) != 1 || siblings[0].ID != r2.ID {
		t.Fatalf("expected [r2], got %+v", siblings)
	}
}

func TestReleaseService_Policy_Lookup(t *testing.T) {
	relRepo := newFakeReleaseRepo()
	roRepo := newFakeRolloutRepo()
	svc := NewReleaseService(relRepo, roRepo)
	rel := &models.Release{ID: uuid.New(), ScopeType: models.ScopeOrg, ScopeID: uuid.New(), Name: "x", CoordinationPolicy: models.CoordinationCascadeAbort}
	_ = relRepo.Create(context.Background(), rel)
	pol, err := svc.GetPolicy(context.Background(), rel.ID)
	if err != nil || pol != models.CoordinationCascadeAbort {
		t.Fatalf("got pol=%s err=%v", pol, err)
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/ -run TestReleaseService -v
```

- [ ] **Step 3: Implement service**

Create `internal/rollout/release_service.go`:

```go
package rollout

import (
	"context"
	"fmt"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// ReleaseService provides CRUD + attach semantics for Release bundles.
type ReleaseService struct {
	releases ReleaseRepository
	rollouts RolloutRepository
}

// NewReleaseService builds a ReleaseService.
func NewReleaseService(rel ReleaseRepository, ro RolloutRepository) *ReleaseService {
	return &ReleaseService{releases: rel, rollouts: ro}
}

// Create persists a release. Defaults coordination_policy to independent.
func (s *ReleaseService) Create(ctx context.Context, r *models.Release) error {
	if r.CoordinationPolicy == "" {
		r.CoordinationPolicy = models.CoordinationIndependent
	}
	return s.releases.Create(ctx, r)
}

// Get returns a release by ID.
func (s *ReleaseService) Get(ctx context.Context, id uuid.UUID) (*models.Release, error) {
	return s.releases.Get(ctx, id)
}

// List returns releases defined directly on the scope.
func (s *ReleaseService) List(ctx context.Context, st models.ScopeType, sid uuid.UUID) ([]*models.Release, error) {
	return s.releases.ListByScope(ctx, st, sid)
}

// Update persists changes (name, description, coordination_policy).
func (s *ReleaseService) Update(ctx context.Context, r *models.Release) error {
	return s.releases.Update(ctx, r)
}

// Delete removes a release row. Rollouts keep their release_id (dangling);
// app reads tolerate missing releases.
func (s *ReleaseService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.releases.Delete(ctx, id)
}

// Attach sets the rollout's release_id. Both release and rollout must exist.
func (s *ReleaseService) Attach(ctx context.Context, releaseID uuid.UUID, rolloutID uuid.UUID) error {
	if _, err := s.releases.Get(ctx, releaseID); err != nil {
		return fmt.Errorf("release not found: %w", err)
	}
	id := releaseID
	return s.rollouts.SetReleaseID(ctx, rolloutID, &id)
}

// Members returns all rollouts attached to a release.
func (s *ReleaseService) Members(ctx context.Context, releaseID uuid.UUID) ([]*models.Rollout, error) {
	return s.rollouts.ListByRelease(ctx, releaseID)
}

// ActiveSiblings returns rollouts in the release with status in
// (pending, active, paused, awaiting_approval), excluding the given origin id.
// Used by the coordination subscriber to act on siblings when one rolls back.
func (s *ReleaseService) ActiveSiblings(ctx context.Context, releaseID uuid.UUID, excludeID uuid.UUID) ([]*models.Rollout, error) {
	all, err := s.rollouts.ListByRelease(ctx, releaseID)
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

// GetPolicy is a convenience lookup for the coordination subscriber.
func (s *ReleaseService) GetPolicy(ctx context.Context, releaseID uuid.UUID) (models.CoordinationPolicy, error) {
	rel, err := s.releases.Get(ctx, releaseID)
	if err != nil {
		return "", err
	}
	return rel.CoordinationPolicy, nil
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/rollout/ -v -count=1
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/release_service.go internal/rollout/release_service_test.go
git commit -m "feat(rollout): ReleaseService (CRUD + attach + siblings)"
```

---

## Task 6: CoordinationSubscriber

**Files:**
- Create: `internal/rollout/coordination.go`
- Create: `internal/rollout/coordination_test.go`

Event-driven handler: listens for `rollouts.rollout.rolled_back` NATS events, looks up the release, applies policy. Implemented as a plain function that the subscriber goroutine calls — testable without NATS.

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

// siblingAction lets tests assert which sibling got which action.
type capturedAction struct {
	siblingID uuid.UUID
	action    string // "pause" or "rollback"
}

// fakeSiblingActor stands in for RolloutService. Tracks the actions the
// coordinator invoked on each sibling.
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
	relRepo := newFakeReleaseRepo()
	roRepo := newFakeRolloutRepo()
	rsvc := NewReleaseService(relRepo, roRepo)

	relID := uuid.New()
	_ = relRepo.Create(context.Background(), &models.Release{ID: relID, Name: "r", ScopeType: models.ScopeOrg, ScopeID: uuid.New(), CoordinationPolicy: models.CoordinationIndependent})

	originID := uuid.New()
	origin := &models.Rollout{ID: originID, Status: models.RolloutRolledBack, ReleaseID: &relID}
	sibling := &models.Rollout{ID: uuid.New(), Status: models.RolloutActive, ReleaseID: &relID}
	roRepo.rows[origin.ID] = origin
	roRepo.rows[sibling.ID] = sibling

	actor := &fakeSiblingActor{}
	coord := NewCoordinator(rsvc, actor)
	if err := coord.OnRollback(context.Background(), originID); err != nil {
		t.Fatal(err)
	}
	if len(actor.calls) != 0 {
		t.Fatalf("independent policy should not act, got %v", actor.calls)
	}
}

func TestCoordinator_PauseOnSiblingAbort(t *testing.T) {
	relRepo := newFakeReleaseRepo()
	roRepo := newFakeRolloutRepo()
	rsvc := NewReleaseService(relRepo, roRepo)

	relID := uuid.New()
	_ = relRepo.Create(context.Background(), &models.Release{ID: relID, ScopeType: models.ScopeOrg, ScopeID: uuid.New(), Name: "r", CoordinationPolicy: models.CoordinationPauseOnSiblingAbort})

	originID := uuid.New()
	s1 := &models.Rollout{ID: uuid.New(), Status: models.RolloutActive, ReleaseID: &relID}
	s2 := &models.Rollout{ID: uuid.New(), Status: models.RolloutPaused, ReleaseID: &relID}
	sDone := &models.Rollout{ID: uuid.New(), Status: models.RolloutSucceeded, ReleaseID: &relID}
	origin := &models.Rollout{ID: originID, Status: models.RolloutRolledBack, ReleaseID: &relID}
	for _, r := range []*models.Rollout{origin, s1, s2, sDone} {
		roRepo.rows[r.ID] = r
	}

	actor := &fakeSiblingActor{}
	coord := NewCoordinator(rsvc, actor)
	if err := coord.OnRollback(context.Background(), originID); err != nil {
		t.Fatal(err)
	}
	// s1 (active) and s2 (paused) are both siblings; spec: pause the active ones.
	// Pausing an already-paused is illegal state-machine, so skip s2 for pause.
	// Implementation choice: only pause siblings whose status == active.
	if len(actor.calls) != 1 || actor.calls[0].siblingID != s1.ID || actor.calls[0].action != "pause" {
		t.Fatalf("expected [pause s1], got %+v", actor.calls)
	}
}

func TestCoordinator_CascadeAbort(t *testing.T) {
	relRepo := newFakeReleaseRepo()
	roRepo := newFakeRolloutRepo()
	rsvc := NewReleaseService(relRepo, roRepo)

	relID := uuid.New()
	_ = relRepo.Create(context.Background(), &models.Release{ID: relID, ScopeType: models.ScopeOrg, ScopeID: uuid.New(), Name: "r", CoordinationPolicy: models.CoordinationCascadeAbort})

	originID := uuid.New()
	s1 := &models.Rollout{ID: uuid.New(), Status: models.RolloutActive, ReleaseID: &relID}
	s2 := &models.Rollout{ID: uuid.New(), Status: models.RolloutPaused, ReleaseID: &relID}
	origin := &models.Rollout{ID: originID, Status: models.RolloutRolledBack, ReleaseID: &relID}
	for _, r := range []*models.Rollout{origin, s1, s2} {
		roRepo.rows[r.ID] = r
	}

	actor := &fakeSiblingActor{}
	coord := NewCoordinator(rsvc, actor)
	if err := coord.OnRollback(context.Background(), originID); err != nil {
		t.Fatal(err)
	}
	// Cascade: both s1 and s2 (active + paused — valid for rollback).
	gotIDs := map[uuid.UUID]string{}
	for _, c := range actor.calls {
		gotIDs[c.siblingID] = c.action
	}
	if gotIDs[s1.ID] != "rollback" || gotIDs[s2.ID] != "rollback" {
		t.Fatalf("expected rollback on both; got %+v", actor.calls)
	}
}

func TestCoordinator_NoReleaseID_NoOp(t *testing.T) {
	relRepo := newFakeReleaseRepo()
	roRepo := newFakeRolloutRepo()
	rsvc := NewReleaseService(relRepo, roRepo)

	originID := uuid.New()
	origin := &models.Rollout{ID: originID, Status: models.RolloutRolledBack} // no ReleaseID
	roRepo.rows[origin.ID] = origin

	actor := &fakeSiblingActor{}
	coord := NewCoordinator(rsvc, actor)
	if err := coord.OnRollback(context.Background(), originID); err != nil {
		t.Fatal(err)
	}
	if len(actor.calls) != 0 {
		t.Fatalf("no release → no coordination; got %+v", actor.calls)
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

// Coordinator applies a release's coordination_policy to sibling rollouts when
// any member rollout rolls back.
type Coordinator struct {
	releases *ReleaseService
	actor    SiblingActor
}

// NewCoordinator builds a Coordinator.
func NewCoordinator(rs *ReleaseService, a SiblingActor) *Coordinator {
	return &Coordinator{releases: rs, actor: a}
}

// OnRollback is invoked when a rollout transitions to rolled_back. It looks up
// the rollout's release (if any) and applies the release's coordination_policy
// to active siblings.
func (c *Coordinator) OnRollback(ctx context.Context, rolledBackID uuid.UUID) error {
	ro, err := c.releases.rollouts.Get(ctx, rolledBackID)
	if err != nil {
		return fmt.Errorf("lookup rolled-back rollout: %w", err)
	}
	if ro.ReleaseID == nil {
		return nil // no release → no coordination
	}
	policy, err := c.releases.GetPolicy(ctx, *ro.ReleaseID)
	if err != nil {
		return fmt.Errorf("lookup release policy: %w", err)
	}
	if policy == models.CoordinationIndependent {
		return nil
	}
	siblings, err := c.releases.ActiveSiblings(ctx, *ro.ReleaseID, rolledBackID)
	if err != nil {
		return fmt.Errorf("list siblings: %w", err)
	}

	systemActor := uuid.Nil // system-initiated
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
			// Rollback accepts Active/Paused/AwaitingApproval — our siblings list
			// filters to those states already.
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
git commit -m "feat(rollout): Coordinator applies release policy on sibling rollback"
```

---

## Task 7: Release HTTP handler

**Files:**
- Create: `internal/rollout/release_handler.go`
- Create: `internal/rollout/release_handler_test.go`

Endpoints under `/api/v1/orgs/:orgSlug/releases`:
- `GET  /releases` — list
- `POST /releases` — create
- `GET  /releases/:id` — detail with members
- `PUT  /releases/:id` — update name/description/coordination_policy
- `POST /releases/:id/attach` — body `{rollout_id}` sets Rollout.ReleaseID

- [ ] **Step 1: Write failing tests**

Create `internal/rollout/release_handler_test.go`:

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

// fakeReleaseScopeResolver stubs the scope resolver for tests.
type fakeReleaseScopeResolver struct{ orgID uuid.UUID }

func (f *fakeReleaseScopeResolver) ResolveOrg(_ *gin.Context) (uuid.UUID, error) {
	return f.orgID, nil
}

func newTestReleaseHandler(t *testing.T) (*ReleaseHandler, *fakeReleaseRepo, *fakeRolloutRepo, uuid.UUID) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	relRepo := newFakeReleaseRepo()
	roRepo := newFakeRolloutRepo()
	rsvc := NewReleaseService(relRepo, roRepo)
	orgID := uuid.New()
	resolver := &fakeReleaseScopeResolver{orgID: orgID}
	h := NewReleaseHandler(rsvc, resolver)
	return h, relRepo, roRepo, orgID
}

func TestReleaseHandler_Create_Defaults(t *testing.T) {
	h, _, _, _ := newTestReleaseHandler(t)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	body, _ := json.Marshal(map[string]any{"name": "v1.1"})
	req := httptest.NewRequest("POST", "/api/v1/orgs/acme/releases", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var resp models.Release
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.CoordinationPolicy != models.CoordinationIndependent {
		t.Fatalf("expected independent default, got %s", resp.CoordinationPolicy)
	}
}

func TestReleaseHandler_List_ReturnsShape(t *testing.T) {
	h, relRepo, _, orgID := newTestReleaseHandler(t)
	_ = relRepo.Create(nil, &models.Release{ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "v1", CoordinationPolicy: models.CoordinationIndependent})
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	req := httptest.NewRequest("GET", "/api/v1/orgs/acme/releases", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d", w.Code)
	}
	var resp struct{ Items []models.Release `json:"items"` }
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Items) != 1 || resp.Items[0].Name != "v1" {
		t.Fatalf("unexpected: %s", w.Body.String())
	}
}

func TestReleaseHandler_Attach_SetsRolloutReleaseID(t *testing.T) {
	h, relRepo, roRepo, orgID := newTestReleaseHandler(t)
	rel := &models.Release{ID: uuid.New(), ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "v1", CoordinationPolicy: models.CoordinationIndependent}
	_ = relRepo.Create(nil, rel)
	ro := &models.Rollout{ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive}
	roRepo.rows[ro.ID] = ro

	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	body, _ := json.Marshal(map[string]any{"rollout_id": ro.ID.String()})
	req := httptest.NewRequest("POST", "/api/v1/orgs/acme/releases/"+rel.ID.String()+"/attach", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if ro.ReleaseID == nil || *ro.ReleaseID != rel.ID {
		t.Fatalf("not attached: %+v", ro.ReleaseID)
	}
}

func TestReleaseHandler_UpdatePolicy(t *testing.T) {
	h, relRepo, _, orgID := newTestReleaseHandler(t)
	rel := &models.Release{ID: uuid.New(), ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "v1", CoordinationPolicy: models.CoordinationIndependent}
	_ = relRepo.Create(nil, rel)

	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	body, _ := json.Marshal(map[string]any{"coordination_policy": "cascade_abort", "name": "v1", "description": ""})
	req := httptest.NewRequest("PUT", "/api/v1/orgs/acme/releases/"+rel.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	got, _ := relRepo.Get(nil, rel.ID)
	if got.CoordinationPolicy != models.CoordinationCascadeAbort {
		t.Fatalf("policy not updated: %s", got.CoordinationPolicy)
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/ -run TestReleaseHandler -v
```

- [ ] **Step 3: Implement handler**

Create `internal/rollout/release_handler.go`:

```go
package rollout

import (
	"net/http"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ReleaseScopeResolver resolves the :orgSlug path param to an org ID.
// Provided by cmd/api wiring (same pattern used by Plan 1 handler).
type ReleaseScopeResolver interface {
	ResolveOrg(c *gin.Context) (uuid.UUID, error)
}

// ReleaseHandler serves release list/get/create/update and the attach endpoint.
type ReleaseHandler struct {
	svc   *ReleaseService
	scope ReleaseScopeResolver
}

// NewReleaseHandler builds a ReleaseHandler.
func NewReleaseHandler(svc *ReleaseService, scope ReleaseScopeResolver) *ReleaseHandler {
	return &ReleaseHandler{svc: svc, scope: scope}
}

// RegisterRoutes mounts releases under /orgs/:orgSlug/.
func (h *ReleaseHandler) RegisterRoutes(api *gin.RouterGroup) {
	org := api.Group("/orgs/:orgSlug")
	org.GET("/releases", h.list)
	org.POST("/releases", h.create)
	org.GET("/releases/:id", h.get)
	org.PUT("/releases/:id", h.update)
	org.POST("/releases/:id/attach", h.attach)
}

func (h *ReleaseHandler) list(c *gin.Context) {
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

type createReleaseReq struct {
	Name               string `json:"name"`
	Description        string `json:"description"`
	CoordinationPolicy string `json:"coordination_policy,omitempty"`
}

func (h *ReleaseHandler) create(c *gin.Context) {
	orgID, err := h.scope.ResolveOrg(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var req createReleaseReq
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
	rel := &models.Release{
		ScopeType:          models.ScopeOrg,
		ScopeID:            orgID,
		Name:               req.Name,
		Description:        req.Description,
		CoordinationPolicy: policy,
	}
	if err := h.svc.Create(c.Request.Context(), rel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rel)
}

func (h *ReleaseHandler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	rel, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	members, _ := h.svc.Members(c.Request.Context(), id)
	c.JSON(http.StatusOK, gin.H{"release": rel, "members": members})
}

type updateReleaseReq struct {
	Name               string `json:"name"`
	Description        string `json:"description"`
	CoordinationPolicy string `json:"coordination_policy"`
}

func (h *ReleaseHandler) update(c *gin.Context) {
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
	var req updateReleaseReq
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

func (h *ReleaseHandler) attach(c *gin.Context) {
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
git add internal/rollout/release_handler.go internal/rollout/release_handler_test.go
git commit -m "feat(rollout): Release HTTP handler (list/get/create/update/attach)"
```

---

## Task 8: Wire Release handler + Coordinator in cmd/api

**Files:**
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Construct repos, service, coordinator, handler**

Find the existing Plan 2 wiring block. After `rolloutAttacher := rollout.NewAttacher(...)`, add:

```go
// ---- Plan 4: Releases + coordination ----
releaseRepo := postgres.NewReleaseRepo(db.Pool)
releaseSvc := rollout.NewReleaseService(releaseRepo, rolloutRepo)

// Coordinator acts on siblings when a rollout rolls back.
// rolloutExecSvc satisfies SiblingActor via its Pause/Rollback methods.
coordinator := rollout.NewCoordinator(releaseSvc, rolloutExecSvc)

// Release HTTP handler uses the Plan 1 scope resolver — it already implements
// ResolveOrg with matching signature.
rollout.NewReleaseHandler(releaseSvc, rolloutScopeResolver).RegisterRoutes(api)
```

(Variable names must match what exists in main.go today — grep first if unsure.)

- [ ] **Step 2: Subscribe coordinator to `rollouts.rollout.rolled_back`**

Near where other rollout NATS subscribers are started (below the `rollouts.rollout.created` subscriber from Plan 2 Task 14), add:

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

Match the exact subscriber pattern used by the Plan 2 Task 14 block.

- [ ] **Step 3: Verify `ReleaseScopeResolver` is satisfied**

The Plan 1 `rolloutScopeAdapter` (or equivalent) already implements `ResolveOrg(c *gin.Context) (uuid.UUID, error)` — the Release handler's resolver interface is a strict subset. Confirm with `grep -n "ResolveOrg" cmd/api/main.go`. No adapter change needed.

- [ ] **Step 4: Build + verify routes**

```bash
cd /Users/sgamel/git/DeploySentry-releases
go build ./cmd/api
./api 2>&1 | head -300 | grep -E "ROUTE:.*/releases" | head -10
```

Expected 5 new routes: `GET/POST /releases`, `GET/PUT /releases/:id`, `POST /releases/:id/attach`.

- [ ] **Step 5: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat(api): wire Release handler and Coordinator subscriber"
```

---

## Task 9: CLI `ds releases` + docs + initiatives

**Files:**
- Create: `cmd/cli/releases.go`
- Modify: `docs/Rollout_Strategies.md`
- Modify: `docs/Current_Initiatives.md`

- [ ] **Step 1: Create the CLI**

Create `cmd/cli/releases.go`:

```go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var releasesCmd = &cobra.Command{
	Use:     "releases",
	Aliases: []string{"release", "rel"},
	Short:   "Manage release bundles (groups of related rollouts)",
}

var releasesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List releases in an org",
	RunE: func(cmd *cobra.Command, args []string) error {
		org, _ := cmd.Flags().GetString("org")
		if org == "" {
			return fmt.Errorf("--org is required")
		}
		body, err := apiGET("/api/v1/orgs/" + org + "/releases")
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	},
}

var releasesGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show a release and its members",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		org, _ := cmd.Flags().GetString("org")
		body, err := apiGET("/api/v1/orgs/" + org + "/releases/" + args[0])
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

var releasesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a release",
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
		body, err := apiPOSTRaw("/api/v1/orgs/"+org+"/releases", "application/json", b)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	},
}

var releasesAttachCmd = &cobra.Command{
	Use:   "attach <release-id>",
	Short: "Attach a rollout to a release",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		org, _ := cmd.Flags().GetString("org")
		rollout, _ := cmd.Flags().GetString("rollout")
		if rollout == "" {
			return fmt.Errorf("--rollout is required")
		}
		b, _ := json.Marshal(map[string]string{"rollout_id": rollout})
		body, err := apiPOSTRaw("/api/v1/orgs/"+org+"/releases/"+args[0]+"/attach", "application/json", b)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	},
}

func init() {
	for _, c := range []*cobra.Command{releasesListCmd, releasesGetCmd, releasesCreateCmd, releasesAttachCmd} {
		c.Flags().String("org", "", "Organization slug (required)")
	}
	releasesCreateCmd.Flags().String("name", "", "Release name (required)")
	releasesCreateCmd.Flags().String("description", "", "Release description")
	releasesCreateCmd.Flags().String("policy", "", "Coordination policy: independent | pause_on_sibling_abort | cascade_abort")
	releasesAttachCmd.Flags().String("rollout", "", "Rollout ID to attach (required)")

	releasesCmd.AddCommand(releasesListCmd, releasesGetCmd, releasesCreateCmd, releasesAttachCmd)
	rootCmd.AddCommand(releasesCmd)
}
```

If `apiGET` / `apiPOSTRaw` free functions don't exist, use `client.get(...)` / `client.postRaw(...)` matching how Plan 2/3 CLI files do it. Grep first: `grep -n "apiGET\|client.get\|client.postRaw" cmd/cli/*.go | head -5`.

- [ ] **Step 2: Build and smoke test**

```bash
go build ./cmd/cli
./cli releases --help
./cli releases create --help
```

Expected: four subcommands visible; flags sensible.

- [ ] **Step 3: Append docs**

Append to `docs/Rollout_Strategies.md`:

```markdown
## Releases

A **Release** groups related rollouts (e.g., a deploy + its associated flag enable) under one umbrella. Releases are optional — rollouts without a release behave exactly as before.

### Why group rollouts?

For UI: "v1.2 shipped these 3 rollouts" — a single bundle view instead of scattered entries.

For coordination: you can set the release's `coordination_policy` so that if any rollout rolls back, its siblings react:

| Policy | Effect on siblings |
|---|---|
| `independent` *(default)* | No effect. Siblings keep running. |
| `pause_on_sibling_abort` | Active siblings are paused. Operator decides whether to resume or rollback. |
| `cascade_abort` | Active/paused siblings are rolled back with reason `sibling_aborted:<id>`. |

### CLI

```
ds releases create --org acme --name "v1.2 ship" --policy pause_on_sibling_abort
ds releases attach <release-id> --org acme --rollout <rollout-id>
ds releases list --org acme
ds releases get <release-id> --org acme
```

### Attaching at rollout creation

Both deploy rollouts and config rollouts accept `release_id` in their attach request body:

```json
{ "rollout": { "strategy_name": "prod-canary", "release_id": "<uuid>" } }
```

### Coordination limitations

- Coordination fires on `rollouts.rollout.rolled_back` events. If a rollout was rolled back before Plan 4 shipped, no coordination replay happens.
- Sibling rollback via `cascade_abort` calls `RolloutService.Rollback` — which updates state but does NOT trigger applicator `Revert`. The engine performs Revert only on its own abort path; manually-triggered rollbacks (whether by operator or cascade) leave traffic/rule unchanged in-place. A follow-up should wire manual rollback to also call Revert.
```

- [ ] **Step 4: Update initiatives**

In `docs/Current_Initiatives.md`:
- Find the "Configurable Rollout Strategies" row.
- Phase → `Implementation (Plans 1+2+3+4)`.
- Add `[Plan 4](./superpowers/plans/2026-04-19-releases-and-coordination.md)` to the plan list.
- Notes → `Plans 1 (templates) + 2 (engine + deploy) + 3 (config) + 4 (releases + coordination) merged. Plan 5 (web UI) to follow.`
- Bump `> Last updated:` to `2026-04-19`.

- [ ] **Step 5: Full test run**

```bash
go build ./...
go test ./internal/rollout/... ./internal/models/ ./internal/platform/database/postgres/ ./internal/flags/ -count=1
```

All must pass (ignore unrelated pre-existing deploy handler failures).

- [ ] **Step 6: Commit**

```bash
git add cmd/cli/releases.go docs/Rollout_Strategies.md docs/Current_Initiatives.md
git commit -m "feat(cli+docs): ds releases commands + Plan 4 docs"
```

---

## Definition of Done

- All 9 tasks committed individually on branch `feature/releases-and-coordination`.
- `go build ./...` clean.
- Tests pass: rollout + models + postgres + flags packages.
- 5 new `/releases` routes visible in API startup log.
- Creating a release → attaching 2 rollouts → rolling back one → observe sibling pause (with `pause_on_sibling_abort`) or sibling rollback (with `cascade_abort`). (Manual integration check; no automated end-to-end test in this plan.)
- Docs + initiative row updated.

## Not in scope (deferred)

- Manual rollback triggering applicator `Revert` (pre-existing Plan 2 gap; documented in the Coordination limitations section).
- Web UI (Plan 5).
- Release templates / defaults.
- Rollback history / timeline replay (the existing Plan 2 `rollout_events` ledger captures state changes; Plan 4 doesn't add a separate release-level event log).
- Automatic release creation when multiple rollouts are submitted together (clients create the release explicitly).

## Self-Review Notes

- **Spec coverage**: implements spec sections "Data Model: releases", "Release coordination (three policies)", "Runtime behavior: pause/rollback siblings on abort", "API: /releases endpoints", "CLI: ds releases". Spec section "Status derived from members at read time" is implemented via the `get` handler returning `{release, members}`; clients derive display status.
- **Type consistency**: `CoordinationPolicy` defined in T2, used consistently in T3 (Postgres CHECK, repo), T5 (service Create default), T6 (coordinator switch), T7 (handler validation), T9 (docs table). `ReleaseRepository` from T3 is consumed by T5's ReleaseService. `SiblingActor` interface in T6 matches `RolloutService.Pause` / `Rollback` signatures from Plan 2 Task 8.
- **Placeholder scan**: No "TBD"/"TODO". Task 8 Step 1 says "variable names must match what exists in main.go today — grep first if unsure" — this is an actual instruction, not a placeholder.
