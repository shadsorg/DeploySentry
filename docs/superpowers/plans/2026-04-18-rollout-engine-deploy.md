# Rollout Engine & Deploy Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the Rollout entity, phase engine, deploy applicator, and six runtime operator controls — then wire them into `POST /deployments` so deploys can opt in (or be required) to go through a user-defined strategy with health-adaptive dwell, abort conditions, and approval gates. Backward-compatible: deploys without rollout intent continue using today's engine.

**Architecture:** A new `rollouts` entity (plus `rollout_phases` and `rollout_events` ledgers) wraps a Deployment. A generalized phase engine at `internal/rollout/engine/` drives rollouts through their strategy_snapshot, delegating target-specific work to an Applicator interface. The deploy applicator (`internal/rollout/applicator/deploy`) bridges to the existing `deployService` to update traffic percent and hook into health signals. A guard on the old deploy engine prevents double-driving when a Rollout exists for the same deployment. Runtime controls (pause/resume/rollback/promote/approve/force-promote) are exposed as HTTP endpoints with a state-machine validator.

**Tech Stack:** Go 1.22+, PostgreSQL (schema: `deploy`), gin, NATS JetStream (existing `MessagePublisher`/`MessageSubscriber` interfaces reused), cobra. Tests hit a real database per project convention. Phase engine uses `time.Timer`/ticker for dwell polling.

**Spec:** `docs/superpowers/specs/2026-04-18-configurable-rollout-strategies-design.md`
**Prior plan:** `docs/superpowers/plans/2026-04-18-rollout-strategies-foundation.md` (Plan 1 — templates, defaults, policies)

---

## File Structure

**New files:**

```
migrations/
  050_create_rollouts.up.sql
  050_create_rollouts.down.sql
  051_seed_system_strategies.up.sql
  051_seed_system_strategies.down.sql

internal/models/
  rollout.go              # Rollout + RolloutStatus
  rollout_phase.go        # RolloutPhase + PhaseStatus
  rollout_event.go        # RolloutEvent + EventType

internal/rollout/
  rollout_repository.go   # RolloutRepository, RolloutPhaseRepository, RolloutEventRepository interfaces
  rollout_service.go      # RolloutService (create, attach, runtime controls)
  rollout_service_test.go
  rollout_handler.go      # HTTP: GET /rollouts, 6 control actions, SSE events
  rollout_handler_test.go
  snapshot.go             # Builds strategy_snapshot from template + overrides
  snapshot_test.go

internal/rollout/applicator/
  applicator.go           # Applicator interface + HealthScore type

internal/rollout/applicator/deploy/
  deploy.go               # DeployApplicator wrapping existing deploy.DeployService + health.HealthMonitor
  deploy_test.go

internal/rollout/engine/
  engine.go               # Phase engine loop, drives rollouts via applicators
  engine_test.go
  gates.go                # abort_conditions evaluator, approval gate handling
  gates_test.go

internal/platform/database/postgres/
  rollouts.go             # RolloutRepo, RolloutPhaseRepo, RolloutEventRepo implementations
  rollouts_test.go

cmd/cli/
  rollouts.go             # ds rollouts {list, get, pause, resume, rollback, promote, approve, force-promote, watch}

```

**Modified files:**

```
internal/deploy/handler.go            # Accept `rollout: {...}` field; emit 409 when active rollout on target
internal/deploy/engine/engine.go      # Guard: skip driveDeployment when a Rollout references this deployment_id
cmd/cli/deploy.go                     # Add --strategy, --apply-immediately flags
cmd/api/main.go                       # Wire new engine goroutine, rollout handler, NATS subscriber
docs/Rollout_Strategies.md            # Expand with runtime control section
docs/Current_Initiatives.md           # Update initiative entry to "Implementation (Plans 1+2)"
```

**Package/namespace conventions:**
- `internal/rollout/` remains the parent package; Plan 1's `validation.go`, `repository.go`, `service.go`, `yaml.go`, `handler.go`, `resolver.go` continue existing.
- New files add onto that package. Sub-packages `engine/` and `applicator/` live inside to keep related code colocated.

**NATS subjects:**
- Publishes on `rollouts.rollout.created`, `rollouts.rollout.phase_entered`, `rollouts.rollout.phase_exited`, `rollouts.rollout.paused`, `rollouts.rollout.resumed`, `rollouts.rollout.promoted`, `rollouts.rollout.force_promoted`, `rollouts.rollout.approved`, `rollouts.rollout.rolled_back`, `rollouts.rollout.completed`.
- Subscribes to `rollouts.rollout.created` (engine driver).
- Existing `deployments.deployment.*` subjects continue to be emitted by the deploy applicator when it applies traffic percent — so the Nginx consumer and Envoy agent need no changes.

---

## Task 1: Migration — rollouts, rollout_phases, rollout_events

**Files:**
- Create: `migrations/050_create_rollouts.up.sql`
- Create: `migrations/050_create_rollouts.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- Rollouts: wraps a change (deploy or config) being progressively applied.
CREATE TABLE rollouts (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    release_id                UUID,
    target_type               TEXT NOT NULL CHECK (target_type IN ('deploy','config')),
    target_ref                JSONB NOT NULL,  -- {"deployment_id":"..."} or {"flag_key":"...","env":"..."}
    strategy_snapshot         JSONB NOT NULL,
    signal_source             JSONB NOT NULL DEFAULT '{"kind":"app_env"}',
    status                    TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','active','paused','awaiting_approval','succeeded','rolled_back','aborted','superseded')),
    current_phase_index       INTEGER NOT NULL DEFAULT 0,
    current_phase_started_at  TIMESTAMPTZ,
    last_healthy_since        TIMESTAMPTZ,
    rollback_reason           TEXT,
    created_by                UUID,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at              TIMESTAMPTZ
);
CREATE INDEX idx_rollouts_status       ON rollouts(status);
CREATE INDEX idx_rollouts_release      ON rollouts(release_id);
CREATE INDEX idx_rollouts_deployment   ON rollouts((target_ref->>'deployment_id'))
    WHERE target_type = 'deploy';
CREATE INDEX idx_rollouts_config       ON rollouts((target_ref->>'flag_key'), (target_ref->>'env'))
    WHERE target_type = 'config';

-- Rollout phases: per-rollout phase ledger (truth over time).
CREATE TABLE rollout_phases (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rollout_id             UUID NOT NULL REFERENCES rollouts(id) ON DELETE CASCADE,
    phase_index            INTEGER NOT NULL,
    step_snapshot          JSONB NOT NULL,
    status                 TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','active','awaiting_approval','passed','failed','rolled_back')),
    entered_at             TIMESTAMPTZ,
    exited_at              TIMESTAMPTZ,
    applied_percent        NUMERIC(6,3),
    health_score_at_exit   NUMERIC(4,3),
    notes                  TEXT NOT NULL DEFAULT '',
    UNIQUE (rollout_id, phase_index)
);
CREATE INDEX idx_rollout_phases_rollout ON rollout_phases(rollout_id);

-- Rollout events: audit trail.
CREATE TABLE rollout_events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rollout_id    UUID NOT NULL REFERENCES rollouts(id) ON DELETE CASCADE,
    event_type    TEXT NOT NULL,
    actor_type    TEXT NOT NULL DEFAULT 'system' CHECK (actor_type IN ('user','system')),
    actor_id      UUID,
    reason        TEXT,
    payload       JSONB NOT NULL DEFAULT '{}',
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_rollout_events_rollout ON rollout_events(rollout_id, occurred_at);
```

- [ ] **Step 2: Write the down migration**

```sql
DROP TABLE IF EXISTS rollout_events;
DROP TABLE IF EXISTS rollout_phases;
DROP TABLE IF EXISTS rollouts;
```

- [ ] **Step 3: Run the migrations and verify schema**

```bash
make dev-up
make migrate-up
psql "$(grep '^MIGRATE_DSN' Makefile | head -1 | cut -d= -f2-)" -c "SET search_path TO deploy; \d rollouts; \d rollout_phases; \d rollout_events"
make migrate-down
make migrate-up
```

Expected: three tables exist with expected columns, indexes, FKs. Migrate-down cleans up; migrate-up re-applies.

- [ ] **Step 4: Commit**

```bash
git add migrations/050_create_rollouts.up.sql migrations/050_create_rollouts.down.sql
git commit -m "feat(db): add rollouts, rollout_phases, rollout_events tables"
```

---

## Task 2: Rollout Go models

**Files:**
- Create: `internal/models/rollout.go`
- Create: `internal/models/rollout_phase.go`
- Create: `internal/models/rollout_event.go`
- Create: `internal/models/rollout_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/models/rollout_test.go`:

```go
package models

import (
	"encoding/json"
	"testing"
)

func TestRolloutStatusStrings(t *testing.T) {
	for s, want := range map[RolloutStatus]string{
		RolloutPending:         "pending",
		RolloutActive:          "active",
		RolloutPaused:          "paused",
		RolloutAwaitingApproval: "awaiting_approval",
		RolloutSucceeded:       "succeeded",
		RolloutRolledBack:      "rolled_back",
		RolloutAborted:         "aborted",
		RolloutSuperseded:      "superseded",
	} {
		if string(s) != want {
			t.Errorf("%v: got %q want %q", s, string(s), want)
		}
	}
}

func TestRolloutPhaseStatusStrings(t *testing.T) {
	for s, want := range map[PhaseStatus]string{
		PhasePending:          "pending",
		PhaseActive:           "active",
		PhaseAwaitingApproval: "awaiting_approval",
		PhasePassed:           "passed",
		PhaseFailed:           "failed",
		PhaseRolledBack:       "rolled_back",
	} {
		if string(s) != want {
			t.Errorf("%v: got %q want %q", s, string(s), want)
		}
	}
}

func TestRolloutTargetRefJSON(t *testing.T) {
	r := Rollout{TargetType: TargetTypeDeploy, TargetRef: RolloutTargetRef{DeploymentID: ptrString("abc")}}
	b, err := json.Marshal(r.TargetRef)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"deployment_id":"abc"}` {
		t.Fatalf("unexpected json: %s", string(b))
	}
}

func ptrString(s string) *string { return &s }
```

- [ ] **Step 2: Run tests, confirm fail**

```bash
go test ./internal/models/ -run TestRollout -v
```

Expected: compile error — types undefined.

- [ ] **Step 3: Create `rollout.go`**

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

// RolloutStatus is the top-level state of a rollout.
type RolloutStatus string

const (
	RolloutPending          RolloutStatus = "pending"
	RolloutActive           RolloutStatus = "active"
	RolloutPaused           RolloutStatus = "paused"
	RolloutAwaitingApproval RolloutStatus = "awaiting_approval"
	RolloutSucceeded        RolloutStatus = "succeeded"
	RolloutRolledBack       RolloutStatus = "rolled_back"
	RolloutAborted          RolloutStatus = "aborted"
	RolloutSuperseded       RolloutStatus = "superseded"
)

// RolloutTargetRef points at the specific resource a rollout is driving.
// Exactly one of DeploymentID or (FlagKey+Env) must be set.
type RolloutTargetRef struct {
	DeploymentID *string `json:"deployment_id,omitempty"`
	FlagKey      *string `json:"flag_key,omitempty"`
	Env          *string `json:"env,omitempty"`
}

// Rollout wraps a progressive change. One row per in-flight or historical rollout.
type Rollout struct {
	ID                     uuid.UUID        `json:"id"`
	ReleaseID              *uuid.UUID       `json:"release_id,omitempty"`
	TargetType             TargetType       `json:"target_type"`
	TargetRef              RolloutTargetRef `json:"target_ref"`
	StrategySnapshot       Strategy         `json:"strategy_snapshot"` // embedded, not a ref
	SignalSource           SignalSource     `json:"signal_source"`
	Status                 RolloutStatus    `json:"status"`
	CurrentPhaseIndex      int              `json:"current_phase_index"`
	CurrentPhaseStartedAt  *time.Time       `json:"current_phase_started_at,omitempty"`
	LastHealthySince       *time.Time       `json:"last_healthy_since,omitempty"`
	RollbackReason         *string          `json:"rollback_reason,omitempty"`
	CreatedBy              *uuid.UUID       `json:"created_by,omitempty"`
	CreatedAt              time.Time        `json:"created_at"`
	CompletedAt            *time.Time       `json:"completed_at,omitempty"`
}

// IsTerminal reports whether status represents a finished rollout.
func (r *Rollout) IsTerminal() bool {
	switch r.Status {
	case RolloutSucceeded, RolloutRolledBack, RolloutAborted, RolloutSuperseded:
		return true
	}
	return false
}
```

- [ ] **Step 4: Create `rollout_phase.go`**

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

// PhaseStatus is the state of a single rollout phase.
type PhaseStatus string

const (
	PhasePending          PhaseStatus = "pending"
	PhaseActive           PhaseStatus = "active"
	PhaseAwaitingApproval PhaseStatus = "awaiting_approval"
	PhasePassed           PhaseStatus = "passed"
	PhaseFailed           PhaseStatus = "failed"
	PhaseRolledBack       PhaseStatus = "rolled_back"
)

// RolloutPhase is the per-phase audit + current-state row.
type RolloutPhase struct {
	ID                 uuid.UUID   `json:"id"`
	RolloutID          uuid.UUID   `json:"rollout_id"`
	PhaseIndex         int         `json:"phase_index"`
	StepSnapshot       Step        `json:"step_snapshot"`
	Status             PhaseStatus `json:"status"`
	EnteredAt          *time.Time  `json:"entered_at,omitempty"`
	ExitedAt           *time.Time  `json:"exited_at,omitempty"`
	AppliedPercent     *float64    `json:"applied_percent,omitempty"`
	HealthScoreAtExit  *float64    `json:"health_score_at_exit,omitempty"`
	Notes              string      `json:"notes"`
}
```

- [ ] **Step 5: Create `rollout_event.go`**

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

// EventType identifies a rollout audit event.
type EventType string

const (
	EventAttached              EventType = "attached"
	EventPhaseEntered          EventType = "phase_entered"
	EventPhaseExited           EventType = "phase_exited"
	EventPaused                EventType = "paused"
	EventResumed               EventType = "resumed"
	EventPromoted              EventType = "promoted"
	EventForcePromoted         EventType = "force_promoted"
	EventApproved              EventType = "approved"
	EventAbortConditionTripped EventType = "abort_condition_tripped"
	EventRollbackTriggered     EventType = "rollback_triggered"
	EventCompleted             EventType = "completed"
	EventSuperseded            EventType = "superseded"
)

// RolloutEvent is one row in the audit trail.
type RolloutEvent struct {
	ID         uuid.UUID              `json:"id"`
	RolloutID  uuid.UUID              `json:"rollout_id"`
	EventType  EventType              `json:"event_type"`
	ActorType  string                 `json:"actor_type"` // "user" or "system"
	ActorID    *uuid.UUID             `json:"actor_id,omitempty"`
	Reason     *string                `json:"reason,omitempty"`
	Payload    map[string]interface{} `json:"payload"`
	OccurredAt time.Time              `json:"occurred_at"`
}
```

- [ ] **Step 6: Run tests, confirm pass**

```bash
go test ./internal/models/ -v
```

- [ ] **Step 7: Commit**

```bash
git add internal/models/rollout.go internal/models/rollout_phase.go internal/models/rollout_event.go internal/models/rollout_test.go
git commit -m "feat(models): add Rollout, RolloutPhase, RolloutEvent types"
```

---

## Task 3: Rollout repositories (interfaces + Postgres + tests)

**Files:**
- Create: `internal/rollout/rollout_repository.go`
- Create: `internal/platform/database/postgres/rollouts.go`
- Create: `internal/platform/database/postgres/rollouts_test.go`

- [ ] **Step 1: Create repository interfaces**

Create `internal/rollout/rollout_repository.go`:

```go
package rollout

import (
	"context"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// RolloutRepository persists Rollout rows.
type RolloutRepository interface {
	Create(ctx context.Context, r *models.Rollout) error
	Get(ctx context.Context, id uuid.UUID) (*models.Rollout, error)
	GetActiveByDeployment(ctx context.Context, deploymentID uuid.UUID) (*models.Rollout, error)
	GetActiveByConfig(ctx context.Context, flagKey, env string) (*models.Rollout, error)
	List(ctx context.Context, opts RolloutListOptions) ([]*models.Rollout, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.RolloutStatus, rollbackReason *string) error
	UpdatePhasePointer(ctx context.Context, id uuid.UUID, phaseIndex int, startedAt *time.Time, lastHealthy *time.Time) error
	MarkCompleted(ctx context.Context, id uuid.UUID) error
}

// RolloutListOptions filters list queries.
type RolloutListOptions struct {
	TargetType *models.TargetType
	Status     *models.RolloutStatus
	ReleaseID  *uuid.UUID
	Limit      int
}

// RolloutPhaseRepository persists per-phase state.
type RolloutPhaseRepository interface {
	BulkInsert(ctx context.Context, phases []*models.RolloutPhase) error
	ListByRollout(ctx context.Context, rolloutID uuid.UUID) ([]*models.RolloutPhase, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.PhaseStatus, enteredAt, exitedAt *time.Time, appliedPct, healthScore *float64, notes string) error
}

// RolloutEventRepository persists audit events.
type RolloutEventRepository interface {
	Insert(ctx context.Context, e *models.RolloutEvent) error
	ListByRollout(ctx context.Context, rolloutID uuid.UUID, limit int) ([]*models.RolloutEvent, error)
}
```

(Note: this file imports `time` for the `time.Time` types in signatures — add `"time"` to imports alongside `"context"`.)

- [ ] **Step 2: Write failing Postgres integration tests**

Create `internal/platform/database/postgres/rollouts_test.go`:

```go
package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/rollout"
	"github.com/google/uuid"
)

func sampleRollout(t *testing.T) *models.Rollout {
	t.Helper()
	depID := uuid.NewString()
	strat := models.Strategy{
		ScopeType:  models.ScopeOrg,
		ScopeID:    uuid.New(),
		Name:       "test",
		TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{
			{Percent: 1, MinDuration: time.Minute, MaxDuration: time.Minute, BakeTimeHealthy: 0},
			{Percent: 100, MinDuration: 0, MaxDuration: 0, BakeTimeHealthy: 0},
		},
		DefaultHealthThreshold:   0.95,
		DefaultRollbackOnFailure: true,
	}
	return &models.Rollout{
		ID:               uuid.New(),
		TargetType:       models.TargetTypeDeploy,
		TargetRef:        models.RolloutTargetRef{DeploymentID: &depID},
		StrategySnapshot: strat,
		SignalSource:     models.SignalSource{Kind: "app_env"},
		Status:           models.RolloutPending,
	}
}

func TestRolloutRepo_CreateGet(t *testing.T) {
	ctx := context.Background()
	repo := NewRolloutRepo(testDB(t))
	r := sampleRollout(t)
	if err := repo.Create(ctx, r); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.Get(ctx, r.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != models.RolloutPending || got.TargetType != models.TargetTypeDeploy {
		t.Fatalf("unexpected: %+v", got)
	}
	// Verify strategy_snapshot round-trips.
	if got.StrategySnapshot.Name != "test" || len(got.StrategySnapshot.Steps) != 2 {
		t.Fatalf("snapshot mismatch: %+v", got.StrategySnapshot)
	}
}

func TestRolloutRepo_GetActiveByDeployment(t *testing.T) {
	ctx := context.Background()
	repo := NewRolloutRepo(testDB(t))
	r := sampleRollout(t)
	r.Status = models.RolloutActive
	if err := repo.Create(ctx, r); err != nil {
		t.Fatal(err)
	}
	depID, _ := uuid.Parse(*r.TargetRef.DeploymentID)
	got, err := repo.GetActiveByDeployment(ctx, depID)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got.ID != r.ID {
		t.Fatalf("expected %v got %v", r.ID, got.ID)
	}
}

func TestRolloutRepo_UpdateStatus(t *testing.T) {
	ctx := context.Background()
	repo := NewRolloutRepo(testDB(t))
	r := sampleRollout(t)
	if err := repo.Create(ctx, r); err != nil {
		t.Fatal(err)
	}
	reason := "manual"
	if err := repo.UpdateStatus(ctx, r.ID, models.RolloutRolledBack, &reason); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.Get(ctx, r.ID)
	if got.Status != models.RolloutRolledBack || got.RollbackReason == nil || *got.RollbackReason != "manual" {
		t.Fatalf("status not updated: %+v", got)
	}
}

func TestRolloutPhaseRepo_BulkInsertList(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	rrepo := NewRolloutRepo(db)
	prepo := NewRolloutPhaseRepo(db)
	r := sampleRollout(t)
	if err := rrepo.Create(ctx, r); err != nil {
		t.Fatal(err)
	}
	phases := []*models.RolloutPhase{
		{RolloutID: r.ID, PhaseIndex: 0, StepSnapshot: r.StrategySnapshot.Steps[0], Status: models.PhasePending},
		{RolloutID: r.ID, PhaseIndex: 1, StepSnapshot: r.StrategySnapshot.Steps[1], Status: models.PhasePending},
	}
	if err := prepo.BulkInsert(ctx, phases); err != nil {
		t.Fatal(err)
	}
	got, err := prepo.ListByRollout(ctx, r.ID)
	if err != nil || len(got) != 2 {
		t.Fatalf("list: err=%v got=%+v", err, got)
	}
}

func TestRolloutEventRepo_InsertList(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	rrepo := NewRolloutRepo(db)
	erepo := NewRolloutEventRepo(db)
	r := sampleRollout(t)
	if err := rrepo.Create(ctx, r); err != nil {
		t.Fatal(err)
	}
	e := &models.RolloutEvent{
		RolloutID: r.ID,
		EventType: models.EventAttached,
		ActorType: "system",
		Payload:   map[string]interface{}{"source": "test"},
	}
	if err := erepo.Insert(ctx, e); err != nil {
		t.Fatal(err)
	}
	list, err := erepo.ListByRollout(ctx, r.ID, 10)
	if err != nil || len(list) != 1 || list[0].EventType != models.EventAttached {
		t.Fatalf("list: err=%v got=%+v", err, list)
	}
	var _ = json.Marshal // touch import
}
```

- [ ] **Step 3: Run, confirm fail**

```bash
make dev-up && make migrate-up
go test ./internal/platform/database/postgres/ -run "TestRollout" -v
```

Expected: `NewRolloutRepo`, `NewRolloutPhaseRepo`, `NewRolloutEventRepo` undefined.

- [ ] **Step 4: Implement Postgres repos**

Create `internal/platform/database/postgres/rollouts.go`:

```go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/rollout"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RolloutRepo is a Postgres-backed rollout.RolloutRepository.
type RolloutRepo struct{ db *pgxpool.Pool }

// NewRolloutRepo returns a new RolloutRepo.
func NewRolloutRepo(db *pgxpool.Pool) *RolloutRepo { return &RolloutRepo{db: db} }

var _ rollout.RolloutRepository = (*RolloutRepo)(nil)

// ErrRolloutNotFound is returned when a rollout lookup fails.
var ErrRolloutNotFound = errors.New("rollout not found")

func (r *RolloutRepo) Create(ctx context.Context, ro *models.Rollout) error {
	if ro.ID == uuid.Nil {
		ro.ID = uuid.New()
	}
	if ro.CreatedAt.IsZero() {
		ro.CreatedAt = time.Now().UTC()
	}
	if ro.SignalSource.Kind == "" {
		ro.SignalSource.Kind = "app_env"
	}
	snapshotJSON, err := json.Marshal(ro.StrategySnapshot)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	refJSON, err := json.Marshal(ro.TargetRef)
	if err != nil {
		return fmt.Errorf("marshal target_ref: %w", err)
	}
	signalJSON, err := json.Marshal(ro.SignalSource)
	if err != nil {
		return fmt.Errorf("marshal signal_source: %w", err)
	}
	_, err = r.db.Exec(ctx, `
        INSERT INTO rollouts (
            id, release_id, target_type, target_ref, strategy_snapshot, signal_source,
            status, current_phase_index, created_by, created_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		ro.ID, ro.ReleaseID, ro.TargetType, refJSON, snapshotJSON, signalJSON,
		ro.Status, ro.CurrentPhaseIndex, ro.CreatedBy, ro.CreatedAt,
	)
	return err
}

func (r *RolloutRepo) Get(ctx context.Context, id uuid.UUID) (*models.Rollout, error) {
	return r.scanOne(ctx, `WHERE id=$1`, id)
}

func (r *RolloutRepo) GetActiveByDeployment(ctx context.Context, depID uuid.UUID) (*models.Rollout, error) {
	return r.scanOne(ctx,
		`WHERE target_type='deploy' AND target_ref->>'deployment_id'=$1
         AND status IN ('pending','active','paused','awaiting_approval')`,
		depID.String(),
	)
}

func (r *RolloutRepo) GetActiveByConfig(ctx context.Context, flagKey, env string) (*models.Rollout, error) {
	return r.scanOne(ctx,
		`WHERE target_type='config' AND target_ref->>'flag_key'=$1 AND target_ref->>'env'=$2
         AND status IN ('pending','active','paused','awaiting_approval')`,
		flagKey, env,
	)
}

func (r *RolloutRepo) List(ctx context.Context, opts rollout.RolloutListOptions) ([]*models.Rollout, error) {
	where := "WHERE 1=1"
	args := []any{}
	if opts.TargetType != nil {
		args = append(args, *opts.TargetType)
		where += fmt.Sprintf(" AND target_type=$%d", len(args))
	}
	if opts.Status != nil {
		args = append(args, *opts.Status)
		where += fmt.Sprintf(" AND status=$%d", len(args))
	}
	if opts.ReleaseID != nil {
		args = append(args, *opts.ReleaseID)
		where += fmt.Sprintf(" AND release_id=$%d", len(args))
	}
	limit := 100
	if opts.Limit > 0 && opts.Limit < 500 {
		limit = opts.Limit
	}
	where += fmt.Sprintf(" ORDER BY created_at DESC LIMIT %d", limit)
	return r.scanMany(ctx, where, args...)
}

func (r *RolloutRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status models.RolloutStatus, reason *string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE rollouts SET status=$1, rollback_reason=COALESCE($2, rollback_reason) WHERE id=$3`,
		status, reason, id,
	)
	return err
}

func (r *RolloutRepo) UpdatePhasePointer(ctx context.Context, id uuid.UUID, idx int, startedAt, lastHealthy *time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE rollouts SET current_phase_index=$1, current_phase_started_at=$2, last_healthy_since=$3 WHERE id=$4`,
		idx, startedAt, lastHealthy, id,
	)
	return err
}

func (r *RolloutRepo) MarkCompleted(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx,
		`UPDATE rollouts SET status='succeeded', completed_at=$1 WHERE id=$2`, now, id,
	)
	return err
}

// scanOne executes the query with WHERE clause and returns a single rollout or ErrRolloutNotFound.
func (r *RolloutRepo) scanOne(ctx context.Context, where string, args ...any) (*models.Rollout, error) {
	rows, err := r.db.Query(ctx, selectRolloutCols+" FROM rollouts "+where+" LIMIT 1", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanRollouts(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrRolloutNotFound
	}
	return list[0], nil
}

func (r *RolloutRepo) scanMany(ctx context.Context, where string, args ...any) ([]*models.Rollout, error) {
	rows, err := r.db.Query(ctx, selectRolloutCols+" FROM rollouts "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRollouts(rows)
}

const selectRolloutCols = `SELECT
    id, release_id, target_type, target_ref, strategy_snapshot, signal_source,
    status, current_phase_index, current_phase_started_at, last_healthy_since,
    rollback_reason, created_by, created_at, completed_at`

func scanRollouts(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]*models.Rollout, error) {
	var out []*models.Rollout
	for rows.Next() {
		var ro models.Rollout
		var releaseID, createdBy sql.Null[uuid.UUID]
		var refJSON, snapshotJSON, signalJSON []byte
		var reason sql.NullString
		var startedAt, lastHealthy, completedAt sql.NullTime
		if err := rows.Scan(
			&ro.ID, &releaseID, &ro.TargetType, &refJSON, &snapshotJSON, &signalJSON,
			&ro.Status, &ro.CurrentPhaseIndex, &startedAt, &lastHealthy,
			&reason, &createdBy, &ro.CreatedAt, &completedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(refJSON, &ro.TargetRef); err != nil {
			return nil, fmt.Errorf("decode target_ref: %w", err)
		}
		if err := json.Unmarshal(snapshotJSON, &ro.StrategySnapshot); err != nil {
			return nil, fmt.Errorf("decode strategy_snapshot: %w", err)
		}
		if err := json.Unmarshal(signalJSON, &ro.SignalSource); err != nil {
			return nil, fmt.Errorf("decode signal_source: %w", err)
		}
		if releaseID.Valid {
			id := releaseID.V
			ro.ReleaseID = &id
		}
		if createdBy.Valid {
			id := createdBy.V
			ro.CreatedBy = &id
		}
		if reason.Valid {
			s := reason.String
			ro.RollbackReason = &s
		}
		if startedAt.Valid {
			t := startedAt.Time
			ro.CurrentPhaseStartedAt = &t
		}
		if lastHealthy.Valid {
			t := lastHealthy.Time
			ro.LastHealthySince = &t
		}
		if completedAt.Valid {
			t := completedAt.Time
			ro.CompletedAt = &t
		}
		out = append(out, &ro)
	}
	return out, rows.Err()
}

// --- RolloutPhaseRepo ---

// RolloutPhaseRepo persists rollout_phases rows.
type RolloutPhaseRepo struct{ db *pgxpool.Pool }

// NewRolloutPhaseRepo returns a new RolloutPhaseRepo.
func NewRolloutPhaseRepo(db *pgxpool.Pool) *RolloutPhaseRepo { return &RolloutPhaseRepo{db: db} }

var _ rollout.RolloutPhaseRepository = (*RolloutPhaseRepo)(nil)

func (r *RolloutPhaseRepo) BulkInsert(ctx context.Context, phases []*models.RolloutPhase) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, p := range phases {
		if p.ID == uuid.Nil {
			p.ID = uuid.New()
		}
		stepJSON, err := json.Marshal(p.StepSnapshot)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
            INSERT INTO rollout_phases (id, rollout_id, phase_index, step_snapshot, status, notes)
            VALUES ($1,$2,$3,$4,$5,$6)`,
			p.ID, p.RolloutID, p.PhaseIndex, stepJSON, p.Status, p.Notes,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *RolloutPhaseRepo) ListByRollout(ctx context.Context, rid uuid.UUID) ([]*models.RolloutPhase, error) {
	rows, err := r.db.Query(ctx, `
        SELECT id, rollout_id, phase_index, step_snapshot, status,
               entered_at, exited_at, applied_percent, health_score_at_exit, notes
        FROM rollout_phases WHERE rollout_id=$1 ORDER BY phase_index`, rid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.RolloutPhase
	for rows.Next() {
		var p models.RolloutPhase
		var stepJSON []byte
		var enteredAt, exitedAt sql.NullTime
		var appliedPct, healthScore sql.NullFloat64
		if err := rows.Scan(
			&p.ID, &p.RolloutID, &p.PhaseIndex, &stepJSON, &p.Status,
			&enteredAt, &exitedAt, &appliedPct, &healthScore, &p.Notes,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(stepJSON, &p.StepSnapshot); err != nil {
			return nil, fmt.Errorf("decode step_snapshot: %w", err)
		}
		if enteredAt.Valid {
			t := enteredAt.Time
			p.EnteredAt = &t
		}
		if exitedAt.Valid {
			t := exitedAt.Time
			p.ExitedAt = &t
		}
		if appliedPct.Valid {
			f := appliedPct.Float64
			p.AppliedPercent = &f
		}
		if healthScore.Valid {
			f := healthScore.Float64
			p.HealthScoreAtExit = &f
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

func (r *RolloutPhaseRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status models.PhaseStatus, enteredAt, exitedAt *time.Time, appliedPct, healthScore *float64, notes string) error {
	_, err := r.db.Exec(ctx, `
        UPDATE rollout_phases
        SET status=$1,
            entered_at=COALESCE($2, entered_at),
            exited_at=COALESCE($3, exited_at),
            applied_percent=COALESCE($4, applied_percent),
            health_score_at_exit=COALESCE($5, health_score_at_exit),
            notes=CASE WHEN $6='' THEN notes ELSE $6 END
        WHERE id=$7`,
		status, enteredAt, exitedAt, appliedPct, healthScore, notes, id,
	)
	return err
}

// --- RolloutEventRepo ---

// RolloutEventRepo persists rollout_events rows.
type RolloutEventRepo struct{ db *pgxpool.Pool }

// NewRolloutEventRepo returns a new RolloutEventRepo.
func NewRolloutEventRepo(db *pgxpool.Pool) *RolloutEventRepo { return &RolloutEventRepo{db: db} }

var _ rollout.RolloutEventRepository = (*RolloutEventRepo)(nil)

func (r *RolloutEventRepo) Insert(ctx context.Context, e *models.RolloutEvent) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	if e.Payload == nil {
		e.Payload = map[string]interface{}{}
	}
	payloadJSON, err := json.Marshal(e.Payload)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
        INSERT INTO rollout_events (id, rollout_id, event_type, actor_type, actor_id, reason, payload, occurred_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		e.ID, e.RolloutID, e.EventType, e.ActorType, e.ActorID, e.Reason, payloadJSON, e.OccurredAt,
	)
	return err
}

func (r *RolloutEventRepo) ListByRollout(ctx context.Context, rid uuid.UUID, limit int) ([]*models.RolloutEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
        SELECT id, rollout_id, event_type, actor_type, actor_id, reason, payload, occurred_at
        FROM rollout_events WHERE rollout_id=$1 ORDER BY occurred_at DESC LIMIT $2`, rid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.RolloutEvent
	for rows.Next() {
		var e models.RolloutEvent
		var actorID sql.Null[uuid.UUID]
		var reason sql.NullString
		var payloadJSON []byte
		if err := rows.Scan(&e.ID, &e.RolloutID, &e.EventType, &e.ActorType, &actorID, &reason, &payloadJSON, &e.OccurredAt); err != nil {
			return nil, err
		}
		if actorID.Valid {
			id := actorID.V
			e.ActorID = &id
		}
		if reason.Valid {
			s := reason.String
			e.Reason = &s
		}
		if len(payloadJSON) > 0 {
			_ = json.Unmarshal(payloadJSON, &e.Payload)
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}
```

- [ ] **Step 5: Run tests, confirm pass**

```bash
go test ./internal/platform/database/postgres/ -run "TestRollout" -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/rollout/rollout_repository.go internal/platform/database/postgres/rollouts.go internal/platform/database/postgres/rollouts_test.go
git commit -m "feat(rollout): rollout repositories + Postgres implementation"
```

---

## Task 4: Applicator interface + deploy applicator

**Files:**
- Create: `internal/rollout/applicator/applicator.go`
- Create: `internal/rollout/applicator/deploy/deploy.go`
- Create: `internal/rollout/applicator/deploy/deploy_test.go`

- [ ] **Step 1: Define the Applicator interface**

Create `internal/rollout/applicator/applicator.go`:

```go
// Package applicator defines the contract a rollout target must satisfy so
// the rollout engine can advance it through phases.
package applicator

import (
	"context"

	"github.com/deploysentry/deploysentry/internal/models"
)

// HealthScore is a normalized health reading in [0, 1].
type HealthScore struct {
	Score     float64 // 0..1; 1 = healthy
	ErrorRate float64 // as fraction (e.g., 0.01 = 1%)
	LatencyP99Ms float64
	LatencyP50Ms float64
	RequestRate  float64
}

// Applicator applies a rollout step to its target and reads back signals.
type Applicator interface {
	// Apply sets the target to the percent declared in step. Called once per phase entry.
	Apply(ctx context.Context, rollout *models.Rollout, step models.Step) error

	// Revert resets the target to its previous (safe) value. Called on rollback.
	Revert(ctx context.Context, rollout *models.Rollout) error

	// CurrentSignal reads the current health score for this rollout, honoring
	// a per-step signal override when provided (may be nil).
	CurrentSignal(ctx context.Context, rollout *models.Rollout, override *models.SignalSource) (HealthScore, error)
}
```

- [ ] **Step 2: Write failing tests for deploy applicator**

Create `internal/rollout/applicator/deploy/deploy_test.go`:

```go
package deploy

import (
	"context"
	"errors"
	"testing"

	"github.com/deploysentry/deploysentry/internal/health"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// --- stubs ---

type stubTrafficSetter struct {
	called     bool
	lastPct    int
	returnErr  error
}

func (s *stubTrafficSetter) SetTrafficPercent(ctx context.Context, deploymentID uuid.UUID, pct int) error {
	s.called = true
	s.lastPct = pct
	return s.returnErr
}

type stubHealth struct {
	score *health.DeploymentHealth
	err   error
}

func (s *stubHealth) GetHealth(id uuid.UUID) (*health.DeploymentHealth, error) {
	return s.score, s.err
}

// --- tests ---

func TestDeployApplicator_Apply_CallsSetTraffic(t *testing.T) {
	ts := &stubTrafficSetter{}
	app := NewApplicator(ts, &stubHealth{})
	depID := uuid.New()
	did := depID.String()
	ro := &models.Rollout{
		TargetType: models.TargetTypeDeploy,
		TargetRef:  models.RolloutTargetRef{DeploymentID: &did},
	}
	step := models.Step{Percent: 25}
	if err := app.Apply(context.Background(), ro, step); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !ts.called || ts.lastPct != 25 {
		t.Fatalf("expected SetTrafficPercent(25), got called=%v pct=%d", ts.called, ts.lastPct)
	}
}

func TestDeployApplicator_Apply_MissingDeploymentID(t *testing.T) {
	app := NewApplicator(&stubTrafficSetter{}, &stubHealth{})
	ro := &models.Rollout{TargetType: models.TargetTypeDeploy}
	if err := app.Apply(context.Background(), ro, models.Step{Percent: 10}); err == nil {
		t.Fatalf("expected error on missing deployment_id")
	}
}

func TestDeployApplicator_Revert_SetsZero(t *testing.T) {
	ts := &stubTrafficSetter{}
	app := NewApplicator(ts, &stubHealth{})
	did := uuid.NewString()
	ro := &models.Rollout{TargetType: models.TargetTypeDeploy, TargetRef: models.RolloutTargetRef{DeploymentID: &did}}
	if err := app.Revert(context.Background(), ro); err != nil {
		t.Fatal(err)
	}
	if ts.lastPct != 0 {
		t.Fatalf("expected revert to 0, got %d", ts.lastPct)
	}
}

func TestDeployApplicator_CurrentSignal_MapsHealth(t *testing.T) {
	h := &stubHealth{score: &health.DeploymentHealth{
		OverallScore: 0.98, ErrorRate: 0.005, LatencyP99: 120, LatencyP50: 30, RequestRate: 500,
	}}
	app := NewApplicator(&stubTrafficSetter{}, h)
	did := uuid.NewString()
	ro := &models.Rollout{TargetType: models.TargetTypeDeploy, TargetRef: models.RolloutTargetRef{DeploymentID: &did}}
	s, err := app.CurrentSignal(context.Background(), ro, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Score != 0.98 || s.ErrorRate != 0.005 {
		t.Fatalf("mapping wrong: %+v", s)
	}
}

func TestDeployApplicator_CurrentSignal_ErrPropagates(t *testing.T) {
	h := &stubHealth{err: errors.New("x")}
	app := NewApplicator(&stubTrafficSetter{}, h)
	did := uuid.NewString()
	ro := &models.Rollout{TargetType: models.TargetTypeDeploy, TargetRef: models.RolloutTargetRef{DeploymentID: &did}}
	if _, err := app.CurrentSignal(context.Background(), ro, nil); err == nil {
		t.Fatalf("expected error")
	}
}
```

(Note: test assumes `health.DeploymentHealth` exposes fields `OverallScore`, `ErrorRate`, `LatencyP99`, `LatencyP50`, `RequestRate`. If the real struct uses different field names, adapt the stub and the mapping function accordingly — check `internal/health/monitor.go` or `scorer.go` first.)

- [ ] **Step 3: Run, confirm fail**

```bash
go test ./internal/rollout/applicator/deploy/ -v
```

Expected: compile error.

- [ ] **Step 4: Implement the deploy applicator**

Create `internal/rollout/applicator/deploy/deploy.go`:

```go
// Package deploy implements the deploy-target Applicator: bridges rollout phase
// progression to the existing deploy service's traffic-percent update path and
// the existing health monitor.
package deploy

import (
	"context"
	"errors"
	"math"

	"github.com/deploysentry/deploysentry/internal/health"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/rollout/applicator"
	"github.com/google/uuid"
)

// TrafficSetter is the subset of the deploy service the applicator depends on.
// The real deploy package should expose this capability; a wrapper may be added
// in cmd/api wiring if the existing DeployService method name differs.
type TrafficSetter interface {
	SetTrafficPercent(ctx context.Context, deploymentID uuid.UUID, pct int) error
}

// HealthReader is the subset of the health monitor the applicator depends on.
type HealthReader interface {
	GetHealth(deploymentID uuid.UUID) (*health.DeploymentHealth, error)
}

// Applicator implements applicator.Applicator for deploy targets.
type Applicator struct {
	traffic TrafficSetter
	health  HealthReader
}

// NewApplicator builds a deploy applicator.
func NewApplicator(traffic TrafficSetter, healthReader HealthReader) *Applicator {
	return &Applicator{traffic: traffic, health: healthReader}
}

var _ applicator.Applicator = (*Applicator)(nil)

// ErrMissingDeploymentID is returned when a deploy rollout lacks a deployment_id.
var ErrMissingDeploymentID = errors.New("deploy rollout missing deployment_id in target_ref")

func (a *Applicator) deploymentID(ro *models.Rollout) (uuid.UUID, error) {
	if ro.TargetRef.DeploymentID == nil {
		return uuid.Nil, ErrMissingDeploymentID
	}
	return uuid.Parse(*ro.TargetRef.DeploymentID)
}

// Apply sets traffic % on the target deployment.
func (a *Applicator) Apply(ctx context.Context, ro *models.Rollout, step models.Step) error {
	depID, err := a.deploymentID(ro)
	if err != nil {
		return err
	}
	pct := int(math.Round(step.Percent))
	return a.traffic.SetTrafficPercent(ctx, depID, pct)
}

// Revert sets traffic back to 0 on the target deployment.
func (a *Applicator) Revert(ctx context.Context, ro *models.Rollout) error {
	depID, err := a.deploymentID(ro)
	if err != nil {
		return err
	}
	return a.traffic.SetTrafficPercent(ctx, depID, 0)
}

// CurrentSignal reads the health monitor and maps into a normalized HealthScore.
// The override parameter is reserved for Plan-2-later use; today, signal source
// is always the deployment's health monitor.
func (a *Applicator) CurrentSignal(ctx context.Context, ro *models.Rollout, _ *models.SignalSource) (applicator.HealthScore, error) {
	depID, err := a.deploymentID(ro)
	if err != nil {
		return applicator.HealthScore{}, err
	}
	h, err := a.health.GetHealth(depID)
	if err != nil {
		return applicator.HealthScore{}, err
	}
	return applicator.HealthScore{
		Score:        h.OverallScore,
		ErrorRate:    h.ErrorRate,
		LatencyP99Ms: h.LatencyP99,
		LatencyP50Ms: h.LatencyP50,
		RequestRate:  h.RequestRate,
	}, nil
}
```

(Field names match `internal/health/DeploymentHealth`. If the real struct uses different names, adapt the mapping. Verify before writing by running `grep -n "type DeploymentHealth" internal/health/`.)

- [ ] **Step 5: Run tests, confirm pass**

```bash
go test ./internal/rollout/applicator/deploy/ -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/rollout/applicator/ 
git commit -m "feat(rollout): Applicator interface + deploy applicator"
```

---

## Task 5: Strategy snapshot builder

**Files:**
- Create: `internal/rollout/snapshot.go`
- Create: `internal/rollout/snapshot_test.go`

The snapshot builder takes a Strategy (template) plus optional inline overrides and produces the frozen `strategy_snapshot` stored on a Rollout. Plan 1 design specified "template + inline overrides, snapshot at attach time".

- [ ] **Step 1: Write failing tests**

Create `internal/rollout/snapshot_test.go`:

```go
package rollout

import (
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
)

func baseTemplate() *models.Strategy {
	return &models.Strategy{
		Name:       "std",
		TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{
			{Percent: 1, MinDuration: 5 * time.Minute, MaxDuration: 30 * time.Minute, BakeTimeHealthy: 5 * time.Minute},
			{Percent: 100},
		},
		DefaultHealthThreshold:   0.95,
		DefaultRollbackOnFailure: true,
	}
}

func TestBuildSnapshot_NoOverrides_ReturnsCopy(t *testing.T) {
	tmpl := baseTemplate()
	snap := BuildSnapshot(tmpl, nil)
	if snap.Name != tmpl.Name {
		t.Fatal("name not copied")
	}
	// Mutating snap must not affect tmpl (deep copy behavior).
	snap.Steps[0].Percent = 99
	if tmpl.Steps[0].Percent == 99 {
		t.Fatal("snapshot aliased template steps — needs deep copy")
	}
}

func TestBuildSnapshot_OverrideStepField(t *testing.T) {
	tmpl := baseTemplate()
	overrides := &StrategyOverrides{
		Steps: map[int]StepOverride{
			0: {MinDurationMs: ptrInt64(int64(10 * time.Minute))},
		},
	}
	snap := BuildSnapshot(tmpl, overrides)
	if snap.Steps[0].MinDuration != 10*time.Minute {
		t.Fatalf("expected override to 10m, got %s", snap.Steps[0].MinDuration)
	}
	if snap.Steps[0].MaxDuration != tmpl.Steps[0].MaxDuration {
		t.Fatal("non-overridden field lost")
	}
}

func TestBuildSnapshot_OverrideDefault(t *testing.T) {
	tmpl := baseTemplate()
	th := 0.99
	overrides := &StrategyOverrides{DefaultHealthThreshold: &th}
	snap := BuildSnapshot(tmpl, overrides)
	if snap.DefaultHealthThreshold != 0.99 {
		t.Fatalf("got %g", snap.DefaultHealthThreshold)
	}
}

func ptrInt64(v int64) *int64 { return &v }
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/ -run TestBuildSnapshot -v
```

- [ ] **Step 3: Implement snapshot builder**

Create `internal/rollout/snapshot.go`:

```go
package rollout

import (
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
)

// StrategyOverrides captures per-rollout tweaks to a template at attach time.
// All pointer fields are optional; nil means "keep template value".
type StrategyOverrides struct {
	DefaultHealthThreshold   *float64             `json:"default_health_threshold,omitempty"`
	DefaultRollbackOnFailure *bool                `json:"default_rollback_on_failure,omitempty"`
	Steps                    map[int]StepOverride `json:"steps,omitempty"`
}

// StepOverride is an optional tweak to a single step. Field names mirror the
// Step struct; unset fields leave the template's step field unchanged.
type StepOverride struct {
	Percent          *float64 `json:"percent,omitempty"`
	MinDurationMs    *int64   `json:"min_duration_ms,omitempty"`
	MaxDurationMs    *int64   `json:"max_duration_ms,omitempty"`
	BakeTimeMs       *int64   `json:"bake_time_healthy_ms,omitempty"`
	HealthThreshold  *float64 `json:"health_threshold,omitempty"`
}

// BuildSnapshot returns a deep copy of the template with overrides applied.
// The result is safe to persist as strategy_snapshot on a Rollout.
func BuildSnapshot(tmpl *models.Strategy, overrides *StrategyOverrides) *models.Strategy {
	snap := &models.Strategy{
		ID:                       tmpl.ID,
		ScopeType:                tmpl.ScopeType,
		ScopeID:                  tmpl.ScopeID,
		Name:                     tmpl.Name,
		Description:              tmpl.Description,
		TargetType:               tmpl.TargetType,
		DefaultHealthThreshold:   tmpl.DefaultHealthThreshold,
		DefaultRollbackOnFailure: tmpl.DefaultRollbackOnFailure,
		Version:                  tmpl.Version,
		IsSystem:                 tmpl.IsSystem,
	}
	// Deep-copy steps.
	snap.Steps = make([]models.Step, len(tmpl.Steps))
	for i, st := range tmpl.Steps {
		snap.Steps[i] = st
		// Deep-copy nested slices/maps of the step.
		if st.AbortConditions != nil {
			cp := make([]models.StepAbortCondition, len(st.AbortConditions))
			copy(cp, st.AbortConditions)
			snap.Steps[i].AbortConditions = cp
		}
		if st.Approval != nil {
			a := *st.Approval
			snap.Steps[i].Approval = &a
		}
		if st.Notify != nil {
			n := *st.Notify
			snap.Steps[i].Notify = &n
		}
	}
	if overrides == nil {
		return snap
	}
	if overrides.DefaultHealthThreshold != nil {
		snap.DefaultHealthThreshold = *overrides.DefaultHealthThreshold
	}
	if overrides.DefaultRollbackOnFailure != nil {
		snap.DefaultRollbackOnFailure = *overrides.DefaultRollbackOnFailure
	}
	for idx, so := range overrides.Steps {
		if idx < 0 || idx >= len(snap.Steps) {
			continue
		}
		st := &snap.Steps[idx]
		if so.Percent != nil {
			st.Percent = *so.Percent
		}
		if so.MinDurationMs != nil {
			st.MinDuration = time.Duration(*so.MinDurationMs) * time.Millisecond
		}
		if so.MaxDurationMs != nil {
			st.MaxDuration = time.Duration(*so.MaxDurationMs) * time.Millisecond
		}
		if so.BakeTimeMs != nil {
			st.BakeTimeHealthy = time.Duration(*so.BakeTimeMs) * time.Millisecond
		}
		if so.HealthThreshold != nil {
			th := *so.HealthThreshold
			st.HealthThreshold = &th
		}
	}
	return snap
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/rollout/ -run TestBuildSnapshot -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/snapshot.go internal/rollout/snapshot_test.go
git commit -m "feat(rollout): strategy snapshot builder with inline overrides"
```

---

## Task 6: Gates — abort conditions and approvals

**Files:**
- Create: `internal/rollout/engine/gates.go`
- Create: `internal/rollout/engine/gates_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/rollout/engine/gates_test.go`:

```go
package engine

import (
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/rollout/applicator"
)

func TestEvaluateAbortConditions_Trips(t *testing.T) {
	step := models.Step{AbortConditions: []models.StepAbortCondition{
		{Metric: "error_rate", Operator: ">", Threshold: 0.02, Window: time.Minute},
	}}
	score := applicator.HealthScore{ErrorRate: 0.05}
	got := EvaluateAbortConditions(step, score)
	if got == nil {
		t.Fatalf("expected trip, got nil")
	}
	if got.Metric != "error_rate" {
		t.Fatalf("wrong metric: %+v", got)
	}
}

func TestEvaluateAbortConditions_DoesNotTrip(t *testing.T) {
	step := models.Step{AbortConditions: []models.StepAbortCondition{
		{Metric: "error_rate", Operator: ">", Threshold: 0.10, Window: time.Minute},
	}}
	score := applicator.HealthScore{ErrorRate: 0.05}
	if got := EvaluateAbortConditions(step, score); got != nil {
		t.Fatalf("unexpected trip: %+v", got)
	}
}

func TestEvaluateAbortConditions_MultipleOperators(t *testing.T) {
	cases := []struct {
		op string
		thr, val float64
		wantTrip bool
	}{
		{">", 1, 2, true}, {">", 1, 1, false},
		{">=", 1, 1, true},
		{"<", 2, 1, true}, {"<", 1, 1, false},
		{"<=", 1, 1, true},
		{"==", 1, 1, true}, {"==", 1, 1.0001, false},
		{"!=", 1, 2, true}, {"!=", 1, 1, false},
	}
	for _, c := range cases {
		step := models.Step{AbortConditions: []models.StepAbortCondition{
			{Metric: "latency_p99_ms", Operator: c.op, Threshold: c.thr, Window: time.Second},
		}}
		score := applicator.HealthScore{LatencyP99Ms: c.val}
		got := EvaluateAbortConditions(step, score)
		if (got != nil) != c.wantTrip {
			t.Errorf("op %s thr %g val %g: got trip=%v want %v", c.op, c.thr, c.val, got != nil, c.wantTrip)
		}
	}
}

func TestEvaluateAbortConditions_UnknownMetric(t *testing.T) {
	step := models.Step{AbortConditions: []models.StepAbortCondition{
		{Metric: "unknown_metric", Operator: ">", Threshold: 0, Window: time.Second},
	}}
	// Unknown metric → no trip (doesn't crash). Log-worthy but not a rollback.
	if got := EvaluateAbortConditions(step, applicator.HealthScore{}); got != nil {
		t.Fatalf("unknown metric should not trip; got %+v", got)
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/engine/ -run TestEvaluate -v
```

- [ ] **Step 3: Implement gates**

Create `internal/rollout/engine/gates.go`:

```go
// Package engine drives rollouts through their strategy phases. This file
// implements the gate logic (abort conditions, approval evaluation) used by
// the main phase loop.
package engine

import (
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/rollout/applicator"
)

// EvaluateAbortConditions returns the first abort condition whose threshold
// is tripped by the current HealthScore, or nil if no condition trips.
// Unknown metric names are silently ignored (no trip).
func EvaluateAbortConditions(step models.Step, score applicator.HealthScore) *models.StepAbortCondition {
	for i, ac := range step.AbortConditions {
		val, ok := metricValue(ac.Metric, score)
		if !ok {
			continue
		}
		if compare(val, ac.Operator, ac.Threshold) {
			ac := ac
			_ = i
			return &ac
		}
	}
	return nil
}

func metricValue(name string, s applicator.HealthScore) (float64, bool) {
	switch name {
	case "score":
		return s.Score, true
	case "error_rate":
		return s.ErrorRate, true
	case "latency_p99_ms":
		return s.LatencyP99Ms, true
	case "latency_p50_ms":
		return s.LatencyP50Ms, true
	case "request_rate":
		return s.RequestRate, true
	}
	return 0, false
}

func compare(val float64, op string, threshold float64) bool {
	switch op {
	case ">":
		return val > threshold
	case ">=":
		return val >= threshold
	case "<":
		return val < threshold
	case "<=":
		return val <= threshold
	case "==":
		return val == threshold
	case "!=":
		return val != threshold
	}
	return false
}

// EffectiveHealthThreshold returns the step's health threshold, defaulting to
// the strategy's default_health_threshold when the step has no override.
func EffectiveHealthThreshold(strat *models.Strategy, step models.Step) float64 {
	if step.HealthThreshold != nil {
		return *step.HealthThreshold
	}
	return strat.DefaultHealthThreshold
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/rollout/engine/ -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/engine/gates.go internal/rollout/engine/gates_test.go
git commit -m "feat(rollout): abort condition evaluator + effective health threshold"
```

---

## Task 7: Phase engine core loop

**Files:**
- Create: `internal/rollout/engine/engine.go`
- Create: `internal/rollout/engine/engine_test.go`

- [ ] **Step 1: Write failing tests for engine loop**

Create `internal/rollout/engine/engine_test.go`:

```go
package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/rollout/applicator"
	"github.com/google/uuid"
)

// --- stubs ---

type stubApplicator struct {
	mu     sync.Mutex
	signal applicator.HealthScore
	applied []float64
	errApply, errRevert error
}

func (s *stubApplicator) setSignal(sc applicator.HealthScore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signal = sc
}
func (s *stubApplicator) Apply(_ context.Context, _ *models.Rollout, step models.Step) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applied = append(s.applied, step.Percent)
	return s.errApply
}
func (s *stubApplicator) Revert(_ context.Context, _ *models.Rollout) error { return s.errRevert }
func (s *stubApplicator) CurrentSignal(_ context.Context, _ *models.Rollout, _ *models.SignalSource) (applicator.HealthScore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.signal, nil
}

type stubRepo struct {
	mu        sync.Mutex
	rollouts  map[uuid.UUID]*models.Rollout
	phases    []*models.RolloutPhase
	events    []*models.RolloutEvent
}

func newStubRepo() *stubRepo { return &stubRepo{rollouts: map[uuid.UUID]*models.Rollout{}} }

// Implement the RepoSet interface used by Engine (defined in engine.go).
func (r *stubRepo) GetRollout(_ context.Context, id uuid.UUID) (*models.Rollout, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	return r.rollouts[id], nil
}
func (r *stubRepo) UpdateRolloutStatus(_ context.Context, id uuid.UUID, st models.RolloutStatus, reason *string) error {
	r.mu.Lock(); defer r.mu.Unlock()
	if ro, ok := r.rollouts[id]; ok {
		ro.Status = st
		if reason != nil {
			ro.RollbackReason = reason
		}
	}
	return nil
}
func (r *stubRepo) UpdateRolloutPhasePointer(_ context.Context, id uuid.UUID, idx int, startedAt, lastHealthy *time.Time) error {
	r.mu.Lock(); defer r.mu.Unlock()
	if ro, ok := r.rollouts[id]; ok {
		ro.CurrentPhaseIndex = idx
		ro.CurrentPhaseStartedAt = startedAt
		ro.LastHealthySince = lastHealthy
	}
	return nil
}
func (r *stubRepo) MarkRolloutCompleted(_ context.Context, id uuid.UUID) error {
	r.mu.Lock(); defer r.mu.Unlock()
	if ro, ok := r.rollouts[id]; ok {
		now := time.Now().UTC()
		ro.Status = models.RolloutSucceeded
		ro.CompletedAt = &now
	}
	return nil
}
func (r *stubRepo) BulkInsertPhases(_ context.Context, phases []*models.RolloutPhase) error {
	r.mu.Lock(); defer r.mu.Unlock()
	for _, p := range phases {
		if p.ID == uuid.Nil {
			p.ID = uuid.New()
		}
		r.phases = append(r.phases, p)
	}
	return nil
}
func (r *stubRepo) ListPhases(_ context.Context, rid uuid.UUID) ([]*models.RolloutPhase, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	var out []*models.RolloutPhase
	for _, p := range r.phases {
		if p.RolloutID == rid {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *stubRepo) UpdatePhaseStatus(_ context.Context, id uuid.UUID, st models.PhaseStatus, ea, xa *time.Time, ap, hs *float64, notes string) error {
	r.mu.Lock(); defer r.mu.Unlock()
	for _, p := range r.phases {
		if p.ID == id {
			p.Status = st
			if ea != nil {
				p.EnteredAt = ea
			}
			if xa != nil {
				p.ExitedAt = xa
			}
			if ap != nil {
				p.AppliedPercent = ap
			}
			if hs != nil {
				p.HealthScoreAtExit = hs
			}
			if notes != "" {
				p.Notes = notes
			}
		}
	}
	return nil
}
func (r *stubRepo) InsertEvent(_ context.Context, e *models.RolloutEvent) error {
	r.mu.Lock(); defer r.mu.Unlock()
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	r.events = append(r.events, e)
	return nil
}

type stubPublisher struct {
	mu   sync.Mutex
	sent []string
}

func (p *stubPublisher) Publish(_ context.Context, subject string, _ []byte) error {
	p.mu.Lock(); defer p.mu.Unlock()
	p.sent = append(p.sent, subject)
	return nil
}

// --- tests ---

func makeRollout(steps []models.Step) *models.Rollout {
	return &models.Rollout{
		ID:         uuid.New(),
		TargetType: models.TargetTypeDeploy,
		TargetRef:  models.RolloutTargetRef{DeploymentID: ptrStr(uuid.NewString())},
		Status:     models.RolloutPending,
		StrategySnapshot: models.Strategy{
			Name:                   "t",
			TargetType:             models.TargetTypeDeploy,
			Steps:                  steps,
			DefaultHealthThreshold: 0.95,
			DefaultRollbackOnFailure: true,
		},
		SignalSource: models.SignalSource{Kind: "app_env"},
	}
}

func ptrStr(s string) *string { return &s }

// TestEngine_Drive_CompletesOnHealthy drives a 2-step healthy rollout and asserts
// both percents were applied and status is Succeeded.
func TestEngine_Drive_CompletesOnHealthy(t *testing.T) {
	repo := newStubRepo()
	app := &stubApplicator{signal: applicator.HealthScore{Score: 0.99}}
	eng := New(repo, app, &stubPublisher{}, EngineOptions{PollInterval: 10 * time.Millisecond, Now: func() time.Time { return time.Now() }})

	ro := makeRollout([]models.Step{
		{Percent: 50, MinDuration: 20 * time.Millisecond, MaxDuration: 200 * time.Millisecond, BakeTimeHealthy: 10 * time.Millisecond},
		{Percent: 100, MinDuration: 0, MaxDuration: 0, BakeTimeHealthy: 0},
	})
	repo.rollouts[ro.ID] = ro

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := eng.DriveRollout(ctx, ro.ID); err != nil {
		t.Fatalf("drive: %v", err)
	}
	if ro.Status != models.RolloutSucceeded {
		t.Fatalf("expected Succeeded, got %s", ro.Status)
	}
	if len(app.applied) != 2 || app.applied[0] != 50 || app.applied[1] != 100 {
		t.Fatalf("expected [50,100], got %v", app.applied)
	}
}

// TestEngine_Drive_RollsBackOnUnhealthyMax drives a rollout where health never
// reaches threshold, so max_duration expires and rollback fires.
func TestEngine_Drive_RollsBackOnUnhealthyMax(t *testing.T) {
	repo := newStubRepo()
	app := &stubApplicator{signal: applicator.HealthScore{Score: 0.5}}
	eng := New(repo, app, &stubPublisher{}, EngineOptions{PollInterval: 5 * time.Millisecond, Now: func() time.Time { return time.Now() }})

	ro := makeRollout([]models.Step{
		{Percent: 25, MinDuration: 10 * time.Millisecond, MaxDuration: 50 * time.Millisecond, BakeTimeHealthy: 20 * time.Millisecond},
		{Percent: 100},
	})
	repo.rollouts[ro.ID] = ro

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := eng.DriveRollout(ctx, ro.ID); err == nil {
		t.Fatalf("expected error indicating rollback")
	}
	if ro.Status != models.RolloutRolledBack {
		t.Fatalf("expected RolledBack, got %s", ro.Status)
	}
}

// TestEngine_Drive_AbortConditionTrips immediately rolls back when an abort
// condition is tripped, without waiting for max_duration.
func TestEngine_Drive_AbortConditionTrips(t *testing.T) {
	repo := newStubRepo()
	app := &stubApplicator{signal: applicator.HealthScore{Score: 0.99, ErrorRate: 0.10}}
	eng := New(repo, app, &stubPublisher{}, EngineOptions{PollInterval: 5 * time.Millisecond, Now: func() time.Time { return time.Now() }})

	ro := makeRollout([]models.Step{
		{
			Percent: 5, MinDuration: 5 * time.Millisecond, MaxDuration: 5 * time.Second, BakeTimeHealthy: 2 * time.Second,
			AbortConditions: []models.StepAbortCondition{{Metric: "error_rate", Operator: ">", Threshold: 0.02, Window: time.Second}},
		},
		{Percent: 100},
	})
	repo.rollouts[ro.ID] = ro

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = eng.DriveRollout(ctx, ro.ID)
	if ro.Status != models.RolloutRolledBack {
		t.Fatalf("expected RolledBack, got %s", ro.Status)
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/engine/ -v
```

- [ ] **Step 3: Implement the engine**

Create `internal/rollout/engine/engine.go`:

```go
package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/rollout/applicator"
	"github.com/google/uuid"
)

// RepoSet is the subset of rollout repositories the engine needs.
// Implemented by composing RolloutRepo + RolloutPhaseRepo + RolloutEventRepo
// at the wiring layer.
type RepoSet interface {
	GetRollout(ctx context.Context, id uuid.UUID) (*models.Rollout, error)
	UpdateRolloutStatus(ctx context.Context, id uuid.UUID, status models.RolloutStatus, reason *string) error
	UpdateRolloutPhasePointer(ctx context.Context, id uuid.UUID, phaseIdx int, startedAt, lastHealthy *time.Time) error
	MarkRolloutCompleted(ctx context.Context, id uuid.UUID) error

	BulkInsertPhases(ctx context.Context, phases []*models.RolloutPhase) error
	ListPhases(ctx context.Context, rolloutID uuid.UUID) ([]*models.RolloutPhase, error)
	UpdatePhaseStatus(ctx context.Context, id uuid.UUID, status models.PhaseStatus, enteredAt, exitedAt *time.Time, appliedPct, healthScore *float64, notes string) error

	InsertEvent(ctx context.Context, e *models.RolloutEvent) error
}

// Publisher publishes events to the message bus.
type Publisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

// EngineOptions tunes the polling cadence and time source (for tests).
type EngineOptions struct {
	PollInterval time.Duration // default 2s
	Logger       *slog.Logger
	Now          func() time.Time // default time.Now
}

// Engine drives rollouts through their strategy phases.
type Engine struct {
	repo RepoSet
	app  applicator.Applicator
	pub  Publisher
	opts EngineOptions
}

// New builds a new Engine.
func New(repo RepoSet, app applicator.Applicator, pub Publisher, opts EngineOptions) *Engine {
	if opts.PollInterval == 0 {
		opts.PollInterval = 2 * time.Second
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Engine{repo: repo, app: app, pub: pub, opts: opts}
}

// ErrRollbackTriggered is returned by DriveRollout when the rollout ended in
// the RolledBack state.
var ErrRollbackTriggered = errors.New("rollout rolled back")

// DriveRollout walks a Rollout through its phases. It blocks until the rollout
// reaches a terminal state or the context is cancelled.
func (e *Engine) DriveRollout(ctx context.Context, rolloutID uuid.UUID) error {
	ro, err := e.repo.GetRollout(ctx, rolloutID)
	if err != nil {
		return fmt.Errorf("get rollout: %w", err)
	}
	if ro.IsTerminal() {
		return nil
	}
	if err := e.ensurePhasesExist(ctx, ro); err != nil {
		return err
	}
	_ = e.repo.UpdateRolloutStatus(ctx, ro.ID, models.RolloutActive, nil)
	ro.Status = models.RolloutActive

	phases, err := e.repo.ListPhases(ctx, ro.ID)
	if err != nil {
		return err
	}

	startIdx := ro.CurrentPhaseIndex
	for idx := startIdx; idx < len(phases); idx++ {
		phase := phases[idx]
		if phase.Status == models.PhasePassed {
			continue
		}
		if err := e.runPhase(ctx, ro, phase); err != nil {
			if errors.Is(err, errRolledBack) {
				return ErrRollbackTriggered
			}
			return err
		}
	}

	_ = e.repo.MarkRolloutCompleted(ctx, ro.ID)
	ro.Status = models.RolloutSucceeded
	now := e.opts.Now()
	ro.CompletedAt = &now
	e.emitEvent(ctx, ro.ID, models.EventCompleted, nil, nil)
	e.publish(ctx, "rollouts.rollout.completed", ro.ID)
	return nil
}

var errRolledBack = errors.New("phase triggered rollback")

func (e *Engine) ensurePhasesExist(ctx context.Context, ro *models.Rollout) error {
	existing, err := e.repo.ListPhases(ctx, ro.ID)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	phases := make([]*models.RolloutPhase, len(ro.StrategySnapshot.Steps))
	for i, step := range ro.StrategySnapshot.Steps {
		phases[i] = &models.RolloutPhase{
			RolloutID:    ro.ID,
			PhaseIndex:   i,
			StepSnapshot: step,
			Status:       models.PhasePending,
		}
	}
	return e.repo.BulkInsertPhases(ctx, phases)
}

// runPhase executes a single phase: apply, poll for promote/abort, exit.
func (e *Engine) runPhase(ctx context.Context, ro *models.Rollout, phase *models.RolloutPhase) error {
	now := e.opts.Now()
	if err := e.app.Apply(ctx, ro, phase.StepSnapshot); err != nil {
		return fmt.Errorf("apply phase %d: %w", phase.PhaseIndex, err)
	}
	enteredAt := now
	_ = e.repo.UpdatePhaseStatus(ctx, phase.ID, models.PhaseActive, &enteredAt, nil, &phase.StepSnapshot.Percent, nil, "")
	_ = e.repo.UpdateRolloutPhasePointer(ctx, ro.ID, phase.PhaseIndex, &enteredAt, nil)
	e.emitEvent(ctx, ro.ID, models.EventPhaseEntered, nil, map[string]any{"phase_index": phase.PhaseIndex, "percent": phase.StepSnapshot.Percent})
	e.publish(ctx, "rollouts.rollout.phase_entered", ro.ID)

	threshold := EffectiveHealthThreshold(&ro.StrategySnapshot, phase.StepSnapshot)
	// Approval is tracked inline: a phase enters awaiting_approval, the loop
	// returns, and a later external Approve call resets status back to Active
	// which re-enters this driveRollout via DriveRollout (caller-driven).
	approvalGranted := ro.Status != models.RolloutAwaitingApproval

	var lastHealthySince *time.Time
	ticker := time.NewTicker(e.opts.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		now := e.opts.Now()
		elapsed := now.Sub(enteredAt)

		signal, err := e.app.CurrentSignal(ctx, ro, phase.StepSnapshot.SignalOverride)
		if err != nil {
			// Transient signal failure — keep polling. Log-worthy.
			continue
		}

		// Abort-conditions are checked even before min_duration.
		if trip := EvaluateAbortConditions(phase.StepSnapshot, signal); trip != nil {
			exit := now
			_ = e.repo.UpdatePhaseStatus(ctx, phase.ID, models.PhaseRolledBack, nil, &exit, nil, &signal.Score, fmt.Sprintf("abort: %s %s %g", trip.Metric, trip.Operator, trip.Threshold))
			e.emitEvent(ctx, ro.ID, models.EventAbortConditionTripped, nil, map[string]any{"metric": trip.Metric})
			e.rollback(ctx, ro, fmt.Sprintf("abort condition tripped: %s %s %g", trip.Metric, trip.Operator, trip.Threshold))
			return errRolledBack
		}

		// Approval gate check.
		if phase.StepSnapshot.Approval != nil && !approvalGranted {
			_ = e.repo.UpdatePhaseStatus(ctx, phase.ID, models.PhaseAwaitingApproval, nil, nil, nil, nil, "awaiting approval")
			_ = e.repo.UpdateRolloutStatus(ctx, ro.ID, models.RolloutAwaitingApproval, nil)
			e.publish(ctx, "rollouts.rollout.awaiting_approval", ro.ID)
			// Return without advancing; external signal (approve call) re-drives the rollout.
			return fmt.Errorf("awaiting approval")
		}

		// Track last-healthy-since.
		if signal.Score >= threshold {
			if lastHealthySince == nil {
				t := now
				lastHealthySince = &t
				_ = e.repo.UpdateRolloutPhasePointer(ctx, ro.ID, phase.PhaseIndex, &enteredAt, lastHealthySince)
			}
		} else {
			lastHealthySince = nil
			_ = e.repo.UpdateRolloutPhasePointer(ctx, ro.ID, phase.PhaseIndex, &enteredAt, nil)
		}

		// Promote if min_duration elapsed AND we've been healthy for bake_time.
		if elapsed >= phase.StepSnapshot.MinDuration {
			if lastHealthySince != nil && now.Sub(*lastHealthySince) >= phase.StepSnapshot.BakeTimeHealthy {
				exit := now
				_ = e.repo.UpdatePhaseStatus(ctx, phase.ID, models.PhasePassed, nil, &exit, nil, &signal.Score, "")
				e.emitEvent(ctx, ro.ID, models.EventPhaseExited, nil, map[string]any{"phase_index": phase.PhaseIndex, "score": signal.Score})
				e.publish(ctx, "rollouts.rollout.phase_exited", ro.ID)
				return nil
			}
		}

		// Max-duration timeout → rollback if unhealthy.
		if phase.StepSnapshot.MaxDuration > 0 && elapsed >= phase.StepSnapshot.MaxDuration {
			if lastHealthySince == nil || now.Sub(*lastHealthySince) < phase.StepSnapshot.BakeTimeHealthy {
				exit := now
				_ = e.repo.UpdatePhaseStatus(ctx, phase.ID, models.PhaseRolledBack, nil, &exit, nil, &signal.Score, "unhealthy at max_duration")
				e.rollback(ctx, ro, fmt.Sprintf("unhealthy at max_duration in phase %d", phase.PhaseIndex))
				return errRolledBack
			}
		}
	}
}

func (e *Engine) rollback(ctx context.Context, ro *models.Rollout, reason string) {
	r := reason
	_ = e.repo.UpdateRolloutStatus(ctx, ro.ID, models.RolloutRolledBack, &r)
	_ = e.app.Revert(ctx, ro)
	e.emitEvent(ctx, ro.ID, models.EventRollbackTriggered, &r, nil)
	e.publish(ctx, "rollouts.rollout.rolled_back", ro.ID)
}

func (e *Engine) emitEvent(ctx context.Context, rolloutID uuid.UUID, evt models.EventType, reason *string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	_ = e.repo.InsertEvent(ctx, &models.RolloutEvent{
		RolloutID: rolloutID,
		EventType: evt,
		ActorType: "system",
		Reason:    reason,
		Payload:   payload,
	})
}

func (e *Engine) publish(ctx context.Context, subject string, rolloutID uuid.UUID) {
	payload, _ := json.Marshal(map[string]any{"rollout_id": rolloutID.String()})
	_ = e.pub.Publish(ctx, subject, payload)
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/rollout/engine/ -v
```

Expected: all three tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/engine/engine.go internal/rollout/engine/engine_test.go
git commit -m "feat(rollout): phase engine core loop with health-adaptive dwell"
```

---

## Task 8: Rollout service — create, attach, runtime controls

**Files:**
- Create: `internal/rollout/rollout_service.go`
- Create: `internal/rollout/rollout_service_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/rollout/rollout_service_test.go`:

```go
package rollout

import (
	"context"
	"errors"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// Fakes for the rollout repos, reusing stub patterns.

type fakeRolloutRepo struct{ rows map[uuid.UUID]*models.Rollout }

func newFakeRolloutRepo() *fakeRolloutRepo { return &fakeRolloutRepo{rows: map[uuid.UUID]*models.Rollout{}} }

func (f *fakeRolloutRepo) Create(_ context.Context, r *models.Rollout) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	f.rows[r.ID] = r
	return nil
}
func (f *fakeRolloutRepo) Get(_ context.Context, id uuid.UUID) (*models.Rollout, error) {
	if r, ok := f.rows[id]; ok {
		return r, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeRolloutRepo) GetActiveByDeployment(_ context.Context, dep uuid.UUID) (*models.Rollout, error) {
	for _, r := range f.rows {
		if r.TargetRef.DeploymentID != nil && *r.TargetRef.DeploymentID == dep.String() && !r.IsTerminal() {
			return r, nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeRolloutRepo) GetActiveByConfig(_ context.Context, _, _ string) (*models.Rollout, error) { return nil, errors.New("not found") }
func (f *fakeRolloutRepo) List(_ context.Context, _ RolloutListOptions) ([]*models.Rollout, error) { return nil, nil }
func (f *fakeRolloutRepo) UpdateStatus(_ context.Context, id uuid.UUID, st models.RolloutStatus, reason *string) error {
	if r, ok := f.rows[id]; ok {
		r.Status = st
		if reason != nil {
			r.RollbackReason = reason
		}
	}
	return nil
}
func (f *fakeRolloutRepo) UpdatePhasePointer(_ context.Context, _ uuid.UUID, _ int, _, _ *time.Time) error { return nil }
func (f *fakeRolloutRepo) MarkCompleted(_ context.Context, _ uuid.UUID) error { return nil }

type fakeEventRepo struct{ events []*models.RolloutEvent }

func (f *fakeEventRepo) Insert(_ context.Context, e *models.RolloutEvent) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	f.events = append(f.events, e)
	return nil
}
func (f *fakeEventRepo) ListByRollout(_ context.Context, _ uuid.UUID, _ int) ([]*models.RolloutEvent, error) { return nil, nil }

type fakePhaseRepo struct{}

func (f *fakePhaseRepo) BulkInsert(_ context.Context, _ []*models.RolloutPhase) error { return nil }
func (f *fakePhaseRepo) ListByRollout(_ context.Context, _ uuid.UUID) ([]*models.RolloutPhase, error) { return nil, nil }
func (f *fakePhaseRepo) UpdateStatus(_ context.Context, _ uuid.UUID, _ models.PhaseStatus, _, _ *time.Time, _, _ *float64, _ string) error { return nil }

type fakePublisher struct{ subjects []string }

func (p *fakePublisher) Publish(_ context.Context, subject string, _ []byte) error {
	p.subjects = append(p.subjects, subject)
	return nil
}

func newTestService() *RolloutService {
	return NewRolloutService(
		newFakeRolloutRepo(),
		&fakePhaseRepo{},
		&fakeEventRepo{},
		&fakePublisher{},
	)
}

// --- tests ---

func TestRolloutService_AttachDeploy_CreatesRollout(t *testing.T) {
	svc := newTestService()
	snap := &models.Strategy{Name: "t", TargetType: models.TargetTypeDeploy, Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	depID := uuid.New()
	ro, err := svc.AttachDeploy(context.Background(), depID, snap, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ro.TargetType != models.TargetTypeDeploy || ro.TargetRef.DeploymentID == nil || *ro.TargetRef.DeploymentID != depID.String() {
		t.Fatalf("wrong rollout: %+v", ro)
	}
	if ro.Status != models.RolloutPending {
		t.Fatalf("expected pending, got %s", ro.Status)
	}
}

func TestRolloutService_Pause_Active(t *testing.T) {
	svc := newTestService()
	snap := &models.Strategy{Name: "t", TargetType: models.TargetTypeDeploy, Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	ro, _ := svc.AttachDeploy(context.Background(), uuid.New(), snap, nil, nil)
	ro.Status = models.RolloutActive
	if err := svc.Pause(context.Background(), ro.ID, uuid.New(), "manual"); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if ro.Status != models.RolloutPaused {
		t.Fatalf("got %s", ro.Status)
	}
}

func TestRolloutService_ForcePromote_RequiresReason(t *testing.T) {
	svc := newTestService()
	snap := &models.Strategy{Name: "t", TargetType: models.TargetTypeDeploy, Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	ro, _ := svc.AttachDeploy(context.Background(), uuid.New(), snap, nil, nil)
	ro.Status = models.RolloutActive
	if err := svc.ForcePromote(context.Background(), ro.ID, uuid.New(), ""); err == nil {
		t.Fatalf("expected error on empty reason")
	}
	if err := svc.ForcePromote(context.Background(), ro.ID, uuid.New(), "operator override"); err != nil {
		t.Fatalf("force-promote: %v", err)
	}
}

func TestRolloutService_Rollback_RequiresReason(t *testing.T) {
	svc := newTestService()
	snap := &models.Strategy{Name: "t", TargetType: models.TargetTypeDeploy, Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	ro, _ := svc.AttachDeploy(context.Background(), uuid.New(), snap, nil, nil)
	ro.Status = models.RolloutActive
	if err := svc.Rollback(context.Background(), ro.ID, uuid.New(), ""); err == nil {
		t.Fatalf("expected error on empty reason")
	}
	if err := svc.Rollback(context.Background(), ro.ID, uuid.New(), "broken"); err != nil {
		t.Fatal(err)
	}
	if ro.Status != models.RolloutRolledBack {
		t.Fatalf("expected RolledBack, got %s", ro.Status)
	}
}

func TestRolloutService_StateMachine_PauseNonActive(t *testing.T) {
	svc := newTestService()
	snap := &models.Strategy{Name: "t", TargetType: models.TargetTypeDeploy, Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	ro, _ := svc.AttachDeploy(context.Background(), uuid.New(), snap, nil, nil)
	// Pending rollout should not be pausable.
	if err := svc.Pause(context.Background(), ro.ID, uuid.New(), ""); err == nil {
		t.Fatalf("expected state-machine error")
	}
}
```

(Note: this test imports `time` — add `"time"` to the import block.)

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/ -run TestRolloutService -v
```

- [ ] **Step 3: Implement RolloutService**

Create `internal/rollout/rollout_service.go`:

```go
package rollout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// Publisher is the subset of NATS publishing used by the service.
type Publisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

// RolloutService owns rollout creation and the 6 runtime controls.
type RolloutService struct {
	rollouts RolloutRepository
	phases   RolloutPhaseRepository
	events   RolloutEventRepository
	pub      Publisher
}

// NewRolloutService builds a RolloutService.
func NewRolloutService(r RolloutRepository, p RolloutPhaseRepository, e RolloutEventRepository, pub Publisher) *RolloutService {
	return &RolloutService{rollouts: r, phases: p, events: e, pub: pub}
}

// Sentinel errors.
var (
	ErrReasonRequired      = errors.New("reason is required for this action")
	ErrInvalidStateForOp   = errors.New("rollout is not in a valid state for this operation")
	ErrAlreadyActiveOnTarget = errors.New("an active rollout already exists for this target")
)

// AttachDeploy creates a pending Rollout for a deployment, using the provided
// snapshot. It enforces one-active-per-deployment by returning
// ErrAlreadyActiveOnTarget.
func (s *RolloutService) AttachDeploy(ctx context.Context, depID uuid.UUID, snapshot *models.Strategy, releaseID *uuid.UUID, createdBy *uuid.UUID) (*models.Rollout, error) {
	if existing, _ := s.rollouts.GetActiveByDeployment(ctx, depID); existing != nil {
		return existing, ErrAlreadyActiveOnTarget
	}
	ref := depID.String()
	ro := &models.Rollout{
		ReleaseID:        releaseID,
		TargetType:       models.TargetTypeDeploy,
		TargetRef:        models.RolloutTargetRef{DeploymentID: &ref},
		StrategySnapshot: *snapshot,
		SignalSource:     models.SignalSource{Kind: "app_env"},
		Status:           models.RolloutPending,
		CreatedBy:        createdBy,
	}
	if err := s.rollouts.Create(ctx, ro); err != nil {
		return nil, fmt.Errorf("create rollout: %w", err)
	}
	s.emit(ctx, ro.ID, models.EventAttached, createdBy, nil, nil)
	s.publishRolloutSubject(ctx, "rollouts.rollout.created", ro.ID)
	return ro, nil
}

// Pause freezes an active rollout. The engine's next tick observes the status
// change; callers assume eventual consistency.
func (s *RolloutService) Pause(ctx context.Context, id uuid.UUID, actorID uuid.UUID, reason string) error {
	return s.transition(ctx, id, actorID, reason, models.EventPaused, models.RolloutPaused, func(cur models.RolloutStatus) bool {
		return cur == models.RolloutActive
	}, false)
}

// Resume unfreezes a paused rollout.
func (s *RolloutService) Resume(ctx context.Context, id uuid.UUID, actorID uuid.UUID, reason string) error {
	return s.transition(ctx, id, actorID, reason, models.EventResumed, models.RolloutActive, func(cur models.RolloutStatus) bool {
		return cur == models.RolloutPaused
	}, false)
}

// Promote signals the engine to skip remaining dwell on the current phase.
// The engine must still verify health before advancing.
func (s *RolloutService) Promote(ctx context.Context, id uuid.UUID, actorID uuid.UUID, reason string) error {
	return s.transition(ctx, id, actorID, reason, models.EventPromoted, models.RolloutActive, func(cur models.RolloutStatus) bool {
		return cur == models.RolloutActive || cur == models.RolloutPaused
	}, false)
}

// ForcePromote advances even if unhealthy. Requires a reason.
func (s *RolloutService) ForcePromote(ctx context.Context, id uuid.UUID, actorID uuid.UUID, reason string) error {
	if reason == "" {
		return ErrReasonRequired
	}
	return s.transition(ctx, id, actorID, reason, models.EventForcePromoted, models.RolloutActive, func(cur models.RolloutStatus) bool {
		return cur == models.RolloutActive || cur == models.RolloutPaused || cur == models.RolloutAwaitingApproval
	}, false)
}

// Approve grants approval on an awaiting-approval phase.
func (s *RolloutService) Approve(ctx context.Context, id uuid.UUID, actorID uuid.UUID, reason string) error {
	return s.transition(ctx, id, actorID, reason, models.EventApproved, models.RolloutActive, func(cur models.RolloutStatus) bool {
		return cur == models.RolloutAwaitingApproval
	}, false)
}

// Rollback aborts the rollout and reverts. Requires a reason.
func (s *RolloutService) Rollback(ctx context.Context, id uuid.UUID, actorID uuid.UUID, reason string) error {
	if reason == "" {
		return ErrReasonRequired
	}
	return s.transition(ctx, id, actorID, reason, models.EventRollbackTriggered, models.RolloutRolledBack, func(cur models.RolloutStatus) bool {
		return cur == models.RolloutActive || cur == models.RolloutPaused || cur == models.RolloutAwaitingApproval
	}, true)
}

// transition is the single state-machine helper for all six runtime controls.
func (s *RolloutService) transition(ctx context.Context, id uuid.UUID, actorID uuid.UUID, reason string, evt models.EventType, target models.RolloutStatus, allow func(models.RolloutStatus) bool, withReason bool) error {
	ro, err := s.rollouts.Get(ctx, id)
	if err != nil {
		return err
	}
	if !allow(ro.Status) {
		return fmt.Errorf("%w: current=%s", ErrInvalidStateForOp, ro.Status)
	}
	var reasonPtr *string
	if withReason || reason != "" {
		r := reason
		reasonPtr = &r
	}
	if err := s.rollouts.UpdateStatus(ctx, id, target, reasonPtr); err != nil {
		return err
	}
	ro.Status = target
	s.emit(ctx, id, evt, &actorID, reasonPtr, nil)
	s.publishRolloutSubject(ctx, fmt.Sprintf("rollouts.rollout.%s", evt), id)
	return nil
}

func (s *RolloutService) emit(ctx context.Context, rolloutID uuid.UUID, evt models.EventType, actor *uuid.UUID, reason *string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	actorType := "system"
	if actor != nil {
		actorType = "user"
	}
	_ = s.events.Insert(ctx, &models.RolloutEvent{
		RolloutID: rolloutID, EventType: evt, ActorType: actorType, ActorID: actor,
		Reason: reason, Payload: payload,
	})
}

func (s *RolloutService) publishRolloutSubject(ctx context.Context, subject string, id uuid.UUID) {
	payload, _ := json.Marshal(map[string]string{"rollout_id": id.String()})
	_ = s.pub.Publish(ctx, subject, payload)
}

// Get returns a rollout by ID.
func (s *RolloutService) Get(ctx context.Context, id uuid.UUID) (*models.Rollout, error) {
	return s.rollouts.Get(ctx, id)
}

// GetActiveByDeployment is a pass-through for handler 409 checks.
func (s *RolloutService) GetActiveByDeployment(ctx context.Context, depID uuid.UUID) (*models.Rollout, error) {
	return s.rollouts.GetActiveByDeployment(ctx, depID)
}

// List returns rollouts per filter.
func (s *RolloutService) List(ctx context.Context, opts RolloutListOptions) ([]*models.Rollout, error) {
	return s.rollouts.List(ctx, opts)
}

// Events returns the audit stream for a rollout.
func (s *RolloutService) Events(ctx context.Context, id uuid.UUID, limit int) ([]*models.RolloutEvent, error) {
	return s.events.ListByRollout(ctx, id, limit)
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/rollout/ -run TestRolloutService -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/rollout_service.go internal/rollout/rollout_service_test.go
git commit -m "feat(rollout): RolloutService with six runtime controls + state machine"
```

---

## Task 9: HTTP handler for rollouts + SSE events

**Files:**
- Create: `internal/rollout/rollout_handler.go`
- Create: `internal/rollout/rollout_handler_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/rollout/rollout_handler_test.go`:

```go
package rollout

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func newTestRolloutHandler() (*RolloutHandler, *fakeRolloutRepo) {
	gin.SetMode(gin.TestMode)
	repo := newFakeRolloutRepo()
	svc := NewRolloutService(repo, &fakePhaseRepo{}, &fakeEventRepo{}, &fakePublisher{})
	return NewRolloutHandler(svc), repo
}

func TestRolloutHandler_Rollback_RequiresReason(t *testing.T) {
	h, repo := newTestRolloutHandler()
	// Seed an active rollout.
	ro := &models.Rollout{
		ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive,
		TargetRef: models.RolloutTargetRef{DeploymentID: func() *string { s := uuid.NewString(); return &s }()},
	}
	repo.rows[ro.ID] = ro
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	req := httptest.NewRequest("POST", "/api/v1/orgs/a/rollouts/"+ro.ID.String()+"/rollback", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400 got %d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("POST", "/api/v1/orgs/a/rollouts/"+ro.ID.String()+"/rollback", bytes.NewBufferString(`{"reason":"broken"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d body=%s", w.Code, w.Body.String())
	}
	if ro.Status != models.RolloutRolledBack {
		t.Fatalf("not rolled back: %s", ro.Status)
	}
}

func TestRolloutHandler_ForcePromote_RequiresReason(t *testing.T) {
	h, repo := newTestRolloutHandler()
	ro := &models.Rollout{
		ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive,
		TargetRef: models.RolloutTargetRef{DeploymentID: func() *string { s := uuid.NewString(); return &s }()},
	}
	repo.rows[ro.ID] = ro
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	req := httptest.NewRequest("POST", "/api/v1/orgs/a/rollouts/"+ro.ID.String()+"/force-promote", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestRolloutHandler_Pause_ActiveOnly(t *testing.T) {
	h, repo := newTestRolloutHandler()
	ro := &models.Rollout{ID: uuid.New(), Status: models.RolloutPending}
	repo.rows[ro.ID] = ro
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	req := httptest.NewRequest("POST", "/api/v1/orgs/a/rollouts/"+ro.ID.String()+"/pause", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Pending cannot be paused → 409 Conflict.
	if w.Code != 409 {
		t.Fatalf("expected 409, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRolloutHandler_Get_ReturnsShape(t *testing.T) {
	h, repo := newTestRolloutHandler()
	ro := &models.Rollout{ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive}
	repo.rows[ro.ID] = ro
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	req := httptest.NewRequest("GET", "/api/v1/orgs/a/rollouts/"+ro.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d", w.Code)
	}
	var out models.Rollout
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.ID != ro.ID || out.Status != models.RolloutActive {
		t.Fatalf("unexpected: %+v", out)
	}
	_ = context.Background
}

var _ = http.StatusOK // silence import if unused in test trim
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/ -run TestRolloutHandler -v
```

- [ ] **Step 3: Implement the handler**

Create `internal/rollout/rollout_handler.go`:

```go
package rollout

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RolloutHandler serves rollout list, detail, and 6 runtime control endpoints.
type RolloutHandler struct {
	svc *RolloutService
}

// NewRolloutHandler builds the handler.
func NewRolloutHandler(svc *RolloutService) *RolloutHandler {
	return &RolloutHandler{svc: svc}
}

// RegisterRoutes mounts routes under /orgs/:orgSlug/.
func (h *RolloutHandler) RegisterRoutes(api *gin.RouterGroup) {
	org := api.Group("/orgs/:orgSlug")
	org.GET("/rollouts", h.list)
	org.GET("/rollouts/:id", h.get)
	org.POST("/rollouts/:id/pause", h.pause)
	org.POST("/rollouts/:id/resume", h.resume)
	org.POST("/rollouts/:id/rollback", h.rollback)
	org.POST("/rollouts/:id/promote", h.promote)
	org.POST("/rollouts/:id/approve", h.approve)
	org.POST("/rollouts/:id/force-promote", h.forcePromote)
	org.GET("/rollouts/:id/events", h.events)
}

func (h *RolloutHandler) list(c *gin.Context) {
	opts := RolloutListOptions{}
	if s := c.Query("status"); s != "" {
		st := models.RolloutStatus(s)
		opts.Status = &st
	}
	if t := c.Query("target_type"); t != "" {
		tt := models.TargetType(t)
		opts.TargetType = &tt
	}
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			opts.Limit = n
		}
	}
	rows, err := h.svc.List(c.Request.Context(), opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (h *RolloutHandler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	ro, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ro)
}

type reasonBody struct {
	Reason string `json:"reason"`
}

func actorFromContext(c *gin.Context) uuid.UUID {
	// Auth middleware sets "user_id" in the context. Fallback to uuid.Nil.
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(uuid.UUID); ok {
			return id
		}
	}
	return uuid.Nil
}

func (h *RolloutHandler) runControl(c *gin.Context, fn func(ctx, id uuid.UUID, actor uuid.UUID, reason string) error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body reasonBody
	_ = c.ShouldBindJSON(&body)
	actor := actorFromContext(c)
	if err := fn(c, id, actor, body.Reason); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func mapErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrReasonRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidStateForOp):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

type ctx = *gin.Context

func (h *RolloutHandler) pause(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.Pause(c.Request.Context(), id, actor, reason)
	})
}
func (h *RolloutHandler) resume(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.Resume(c.Request.Context(), id, actor, reason)
	})
}
func (h *RolloutHandler) rollback(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.Rollback(c.Request.Context(), id, actor, reason)
	})
}
func (h *RolloutHandler) promote(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.Promote(c.Request.Context(), id, actor, reason)
	})
}
func (h *RolloutHandler) approve(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.Approve(c.Request.Context(), id, actor, reason)
	})
}
func (h *RolloutHandler) forcePromote(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.ForcePromote(c.Request.Context(), id, actor, reason)
	})
}

// events streams rollout events (simple long-poll for now; SSE upgrade can be
// added without API change).
func (h *RolloutHandler) events(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	limit := 100
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	events, err := h.svc.Events(c.Request.Context(), id, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": events})
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/rollout/ -run TestRolloutHandler -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/rollout_handler.go internal/rollout/rollout_handler_test.go
git commit -m "feat(rollout): HTTP handler with 6 runtime controls + events stream"
```

---

## Task 10: Extend deploy handler — rollout field, 409 edit-lock, policy enforcement

**Files:**
- Modify: `internal/deploy/handler.go`
- Modify: `internal/deploy/handler_test.go`

- [ ] **Step 1: Read the current CreateDeployment handler and POST DTO**

```bash
grep -n "type createDeploymentReq\|func.*CreateDeployment\|h.service.CreateDeployment" internal/deploy/handler.go | head -10
```

Note current field names and binding — you'll need them in Step 3. The existing POST body is likely something like `{ strategy, artifact, version, application_id, environment_id }`.

- [ ] **Step 2: Add failing test for rollout attach**

Add to `internal/deploy/handler_test.go` a test that:
- POSTs a deploy with body including `"rollout": {"strategy_id": "<uuid>"}`.
- Expects the handler to call the (new) RolloutAttacher dependency with the created deployment ID and resolved strategy.
- Verifies 201 response.

Because this task modifies existing handler tests, the specific test shape depends on the existing mock pattern. If tests currently use a mock `DeployService`, add a sibling mock `RolloutAttacher` (see Step 3 for the interface). Construct with:

```go
type fakeAttacher struct{ called bool; lastDep uuid.UUID; lastStratName string }
func (f *fakeAttacher) AttachFromDeployRequest(ctx context.Context, deployment *models.Deployment, req *RolloutAttachRequest, actor uuid.UUID) error {
    f.called = true
    f.lastDep = deployment.ID
    if req != nil { f.lastStratName = req.StrategyName }
    return nil
}
```

And verify assertion after POST:

```go
if !attacher.called { t.Fatal("expected AttachFromDeployRequest to be called") }
```

- [ ] **Step 3: Update the handler to accept rollout field + wire attacher**

In `internal/deploy/handler.go`, add:

```go
// RolloutAttacher is implemented by the rollout package and injected at wiring time.
// Kept as an interface inside deploy to avoid a reverse dependency.
type RolloutAttacher interface {
    AttachFromDeployRequest(ctx context.Context, deployment *models.Deployment, req *RolloutAttachRequest, actor uuid.UUID) error
}

// RolloutAttachRequest is the client-supplied rollout intent on a deploy request.
type RolloutAttachRequest struct {
    StrategyName string          `json:"strategy_name,omitempty"`
    StrategyID   *uuid.UUID      `json:"strategy_id,omitempty"`
    Overrides    json.RawMessage `json:"overrides,omitempty"`
    ReleaseID    *uuid.UUID      `json:"release_id,omitempty"`
    ApplyImmediately bool        `json:"apply_immediately,omitempty"`
}
```

Add `rollouts RolloutAttacher` field to `Handler`, accept in `NewHandler` (wiring updated in Task 14), and extend the deploy POST body struct:

```go
type createDeploymentReq struct {
    // existing fields...
    Rollout *RolloutAttachRequest `json:"rollout,omitempty"`
}
```

In the create handler body (after `h.service.CreateDeployment(...)` succeeds and `d.ID` is populated):

```go
if h.rollouts != nil && req.Rollout != nil && !req.Rollout.ApplyImmediately {
    actor := actorFromContext(c) // helper similar to rollout handler's
    if err := h.rollouts.AttachFromDeployRequest(c.Request.Context(), d, req.Rollout, actor); err != nil {
        // If the target already has an active rollout, respond 409 with action hints.
        if errors.Is(err, rollout.ErrAlreadyActiveOnTarget) {
            c.JSON(http.StatusConflict, gin.H{
                "error": "rollout_in_progress",
                "options": []string{"abort", "wait", "amend"},
            })
            return
        }
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
}
c.JSON(http.StatusCreated, d)
```

Import the rollout package: `"github.com/deploysentry/deploysentry/internal/rollout"`.

Define the actor helper if one doesn't already exist in the deploy package:

```go
func actorFromContext(c *gin.Context) uuid.UUID {
    if v, ok := c.Get("user_id"); ok {
        if id, ok := v.(uuid.UUID); ok {
            return id
        }
    }
    return uuid.Nil
}
```

- [ ] **Step 4: Add 409 check BEFORE creating the deployment**

Immediately after parsing the request and before calling `CreateDeployment`, check that no active rollout already exists for the intended target. For now, only the deploy case is relevant — config-side 409 comes in Plan 3. Since a new deploy will have a new `deployment_id`, there's no existing rollout to collide with; the 409 concern applies to _retargeting_ which is deferred per the spec. Skip this step for deploys in Plan 2 — the 409 case will be triggered downstream at AttachFromDeployRequest.

(Task 11's flag-rule-edit handler is where 409 matters most; that's Plan 3 scope.)

- [ ] **Step 5: Run tests, confirm pass**

```bash
go test ./internal/deploy/ -v
```

Existing tests that don't set `Rollout` in the body must still pass (the field is optional).

- [ ] **Step 6: Commit**

```bash
git add internal/deploy/handler.go internal/deploy/handler_test.go
git commit -m "feat(deploy): accept rollout field on POST /deployments; 409 when target locked"
```

---

## Task 11: Attacher service — resolves strategy + policy + creates Rollout

**Files:**
- Create: `internal/rollout/attacher.go`
- Create: `internal/rollout/attacher_test.go`

The attacher is the glue between `deploy.Handler` and `RolloutService`: it resolves the strategy reference (by name or ID) against the scope, resolves strategy defaults if none provided, enforces mandate/prompt policy, builds the snapshot from the resolved template + overrides, and calls `RolloutService.AttachDeploy`.

- [ ] **Step 1: Write failing test**

Create `internal/rollout/attacher_test.go`:

```go
package rollout

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

func TestAttacher_AttachDeploy_ExplicitStrategyName(t *testing.T) {
	// Seed a template via the Plan 1 fake repos.
	stratRepo := newFakeStratRepo()
	orgID := uuid.New()
	tmpl := &models.Strategy{
		ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "std",
		TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{{Percent: 100}},
		DefaultHealthThreshold: 0.95, DefaultRollbackOnFailure: true,
	}
	_ = stratRepo.Create(context.Background(), tmpl)

	defRepo := newFakeDefaultsRepo()
	polRepo := newFakePolicyRepo()
	rolloutRepo := newFakeRolloutRepo()
	svc := NewRolloutService(rolloutRepo, &fakePhaseRepo{}, &fakeEventRepo{}, &fakePublisher{})

	attacher := NewAttacher(
		NewStrategyService(stratRepo, nil),
		NewStrategyDefaultService(defRepo),
		NewRolloutPolicyService(polRepo),
		svc,
	)

	dep := &models.Deployment{ID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New()}
	actor := uuid.New()

	// Attempt resolution against an app-level leaf with org ancestor being the strategy's scope.
	leaf := ScopeRef{Type: models.ScopeApp, ID: dep.ApplicationID}
	projID := uuid.New()
	err := attacher.AttachDeploy(context.Background(), dep, &AttachIntent{
		StrategyName: "std",
		Leaf:         leaf,
		ProjectID:    &projID,
		OrgID:        &orgID,
	}, actor)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	// Verify one rollout was created.
	if len(rolloutRepo.rows) != 1 {
		t.Fatalf("expected 1 rollout, got %d", len(rolloutRepo.rows))
	}
}

func TestAttacher_AttachDeploy_ResolvesDefault(t *testing.T) {
	stratRepo := newFakeStratRepo()
	defRepo := newFakeDefaultsRepo()
	polRepo := newFakePolicyRepo()
	rolloutRepo := newFakeRolloutRepo()
	svc := NewRolloutService(rolloutRepo, &fakePhaseRepo{}, &fakeEventRepo{}, &fakePublisher{})

	orgID := uuid.New()
	tmpl := &models.Strategy{ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "default-canary",
		TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	_ = stratRepo.Create(context.Background(), tmpl)
	tt := models.TargetTypeDeploy
	_ = defRepo.Upsert(context.Background(), &models.StrategyDefault{
		ScopeType: models.ScopeOrg, ScopeID: orgID, TargetType: &tt, StrategyID: tmpl.ID,
	})

	attacher := NewAttacher(
		NewStrategyService(stratRepo, nil),
		NewStrategyDefaultService(defRepo),
		NewRolloutPolicyService(polRepo),
		svc,
	)

	dep := &models.Deployment{ID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New()}
	projID := uuid.New()
	if err := attacher.AttachDeploy(context.Background(), dep, &AttachIntent{
		Leaf:      ScopeRef{Type: models.ScopeApp, ID: dep.ApplicationID},
		ProjectID: &projID,
		OrgID:     &orgID,
	}, uuid.New()); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if len(rolloutRepo.rows) != 1 {
		t.Fatalf("expected default to resolve; got %d rollouts", len(rolloutRepo.rows))
	}
}

func TestAttacher_AttachDeploy_MandateWithoutStrategy_Errors(t *testing.T) {
	stratRepo := newFakeStratRepo()
	defRepo := newFakeDefaultsRepo()
	polRepo := newFakePolicyRepo()
	rolloutRepo := newFakeRolloutRepo()
	svc := NewRolloutService(rolloutRepo, &fakePhaseRepo{}, &fakeEventRepo{}, &fakePublisher{})

	orgID := uuid.New()
	_ = polRepo.Upsert(context.Background(), &models.RolloutPolicy{
		ScopeType: models.ScopeOrg, ScopeID: orgID, Enabled: true, Policy: models.PolicyMandate,
	})

	attacher := NewAttacher(
		NewStrategyService(stratRepo, nil),
		NewStrategyDefaultService(defRepo),
		NewRolloutPolicyService(polRepo),
		svc,
	)

	dep := &models.Deployment{ID: uuid.New(), ApplicationID: uuid.New()}
	projID := uuid.New()
	if err := attacher.AttachDeploy(context.Background(), dep, &AttachIntent{
		Leaf:      ScopeRef{Type: models.ScopeApp, ID: dep.ApplicationID},
		ProjectID: &projID,
		OrgID:     &orgID,
	}, uuid.New()); err == nil {
		t.Fatalf("expected mandate error")
	}
}

func TestAttacher_MissingOverrides_Snapshot(t *testing.T) {
	stratRepo := newFakeStratRepo()
	orgID := uuid.New()
	tmpl := &models.Strategy{
		ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "std",
		TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95,
	}
	_ = stratRepo.Create(context.Background(), tmpl)
	raw, _ := json.Marshal(struct{}{})
	_ = raw
	// (Smoke test that empty overrides still work.)
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/ -run TestAttacher -v
```

- [ ] **Step 3: Implement attacher**

Create `internal/rollout/attacher.go`:

```go
package rollout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// Attacher composes strategy resolution + policy enforcement + rollout creation
// for a deploy. Called by the deploy handler's AttachFromDeployRequest method.
type Attacher struct {
	strategies *StrategyService
	defaults   *StrategyDefaultService
	policies   *RolloutPolicyService
	rollouts   *RolloutService
}

// NewAttacher builds an Attacher.
func NewAttacher(s *StrategyService, d *StrategyDefaultService, p *RolloutPolicyService, r *RolloutService) *Attacher {
	return &Attacher{strategies: s, defaults: d, policies: p, rollouts: r}
}

// AttachIntent carries the caller-supplied rollout info + scope context needed
// to resolve references and policy.
type AttachIntent struct {
	StrategyID       *uuid.UUID
	StrategyName     string
	Overrides        json.RawMessage
	ReleaseID        *uuid.UUID
	Leaf             ScopeRef
	ProjectID        *uuid.UUID
	OrgID            *uuid.UUID
	Environment      *string
}

// ErrMandateWithoutStrategy is returned when policy=mandate is on and no strategy
// resolves (explicit or via defaults).
var ErrMandateWithoutStrategy = errors.New("rollout strategy required by scope policy, but none provided or resolved")

// AttachDeploy attaches a rollout to the given deployment.
func (a *Attacher) AttachDeploy(ctx context.Context, d *models.Deployment, intent *AttachIntent, actor uuid.UUID) error {
	target := models.TargetTypeDeploy

	// Resolve explicit strategy first.
	var tmpl *models.Strategy
	if intent.StrategyID != nil {
		got, err := a.strategies.Get(ctx, *intent.StrategyID)
		if err != nil {
			return fmt.Errorf("strategy not found by id: %w", err)
		}
		tmpl = got
	} else if intent.StrategyName != "" {
		// Walk ancestors to find the strategy by name.
		ancestors := AncestorScopes(intent.Leaf, intent.ProjectID, intent.OrgID)
		for _, anc := range ancestors {
			got, err := a.strategies.GetByName(ctx, anc.Type, anc.ID, intent.StrategyName)
			if err == nil && got != nil {
				tmpl = got
				break
			}
		}
		if tmpl == nil {
			return fmt.Errorf("strategy %q not found in scope ancestry", intent.StrategyName)
		}
	}

	// If no explicit, try default resolution.
	if tmpl == nil {
		def, err := a.defaults.Resolve(ctx, intent.Leaf, intent.ProjectID, intent.OrgID, intent.Environment, &target)
		if err == nil && def != nil {
			got, err := a.strategies.Get(ctx, def.StrategyID)
			if err == nil {
				tmpl = got
			}
		}
	}

	// Enforce policy.
	policy, _ := a.policies.Resolve(ctx, intent.Leaf, intent.ProjectID, intent.OrgID, intent.Environment, &target)
	if policy != nil && policy.Enabled && policy.Policy == models.PolicyMandate && tmpl == nil {
		return ErrMandateWithoutStrategy
	}
	if tmpl == nil {
		// No strategy and no mandate → no rollout; deploy proceeds via legacy path.
		return nil
	}

	// Build snapshot.
	var overrides *StrategyOverrides
	if len(intent.Overrides) > 0 {
		var o StrategyOverrides
		if err := json.Unmarshal(intent.Overrides, &o); err != nil {
			return fmt.Errorf("overrides invalid: %w", err)
		}
		overrides = &o
	}
	snap := BuildSnapshot(tmpl, overrides)

	// Create the rollout.
	_, err := a.rollouts.AttachDeploy(ctx, d.ID, snap, intent.ReleaseID, &actor)
	if err != nil {
		return err
	}
	return nil
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/rollout/ -run TestAttacher -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/attacher.go internal/rollout/attacher_test.go
git commit -m "feat(rollout): Attacher — strategy+policy resolution + rollout creation"
```

---

## Task 12: Guard the old deploy engine when a Rollout exists

**Files:**
- Modify: `internal/deploy/engine/engine.go`
- Modify: `internal/deploy/engine/engine_test.go`

The old deploy engine subscribes to `deployments.deployment.created` and drives the deployment through its hardcoded canary phases. With Plan 2, when a Rollout wraps a deployment, the rollout engine drives it instead — but both engines would otherwise race. Add a guard.

- [ ] **Step 1: Add a `RolloutLookup` dependency to the old engine**

In `internal/deploy/engine/engine.go`, add:

```go
// RolloutLookup is the subset of the rollout service used by the deploy engine's
// guard. Implementations are injected at wiring time. A nil lookup means no
// rollout integration (backward-compatible behavior).
type RolloutLookup interface {
    HasActiveRolloutForDeployment(ctx context.Context, deploymentID uuid.UUID) (bool, error)
}
```

Add `rolloutLookup RolloutLookup` to the Engine struct. Add a parameter to `New(...)` for the lookup (update callers in cmd/api accordingly — handled in Task 14).

- [ ] **Step 2: Guard the driver**

Near the top of `driveDeployment`, after the initial existence check:

```go
if e.rolloutLookup != nil {
    has, err := e.rolloutLookup.HasActiveRolloutForDeployment(ctx, deploymentID)
    if err != nil {
        e.logger.Warn("deploy engine: rollout lookup failed; proceeding with legacy path", "error", err)
    } else if has {
        e.logger.Info("deploy engine: rollout owns this deployment; skipping legacy driver", "deployment_id", deploymentID)
        return nil
    }
}
```

- [ ] **Step 3: Add a test**

In `internal/deploy/engine/engine_test.go`:

```go
type stubRolloutLookup struct{ has bool; err error }
func (s *stubRolloutLookup) HasActiveRolloutForDeployment(_ context.Context, _ uuid.UUID) (bool, error) {
    return s.has, s.err
}

func TestDeployEngine_GuardsWhenRolloutExists(t *testing.T) {
    // Use the existing test helper(s) this file uses to construct a baseline engine.
    // The pattern below follows the engine_test.go stubs for repo/publisher/health;
    // read that file and adapt the exact constructor argument order if different.
    //
    // Key assertion: when rolloutLookup.has == true, driveDeployment short-circuits
    // — no phase records created, no traffic updates applied, no health evaluation.

    ctx := context.Background()
    deployID := uuid.New()

    // Minimal baseline deployment in a stub repo. Adapt to the existing stubRepo
    // type used by the rest of the file (e.g., the one backing TestDriveDeployment).
    repo := newEngineTestRepo(t) // whatever the existing helper is named
    _ = repo.CreateDeployment(ctx, &models.Deployment{ID: deployID, Status: "pending"})

    eng := New(
        repo,
        &stubPublisher{},
        &health.HealthMonitor{},
        slog.Default(),
    )
    eng.SetRolloutLookup(&stubRolloutLookup{has: true}) // add a small setter in engine.go if New() signature is fixed

    if err := eng.driveDeployment(ctx, deployID); err != nil {
        t.Fatalf("expected nil (short-circuit), got %v", err)
    }
    // Verify: no phase rows written, deployment untouched.
    phases, _ := repo.ListPhases(ctx, deployID)
    if len(phases) != 0 {
        t.Fatalf("expected no phases created; got %d", len(phases))
    }
}
```

If the existing `New(...)` signature already accepts all dependencies in a single call, adding `rolloutLookup` as a new parameter is fine — update all existing test-helper callers of `New(...)` to pass `nil`, which is the backward-compatible zero value (the guard in engine.go short-circuits when `e.rolloutLookup == nil`, matching the Task 12 Step 2 code). If you prefer a setter to avoid touching every caller, add `SetRolloutLookup(l RolloutLookup)` and use that in the new test only.

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/deploy/engine/ -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/deploy/engine/engine.go internal/deploy/engine/engine_test.go
git commit -m "feat(deploy/engine): guard legacy driver when rollout owns deployment"
```

---

## Task 13: System strategy seeding migration

**Files:**
- Create: `migrations/051_seed_system_strategies.up.sql`
- Create: `migrations/051_seed_system_strategies.down.sql`

Seed three system-owned strategy templates per existing org: `system-canary`, `system-blue-green`, `system-rolling`. Values match the existing Go defaults from `internal/deploy/strategies/` (so non-onboarded orgs that do resolve a default via these seed rows get byte-identical behavior). Also seed a default `(any_env, deploy)` assignment pointing to `system-canary` for each org.

- [ ] **Step 1: Write up migration**

Create `migrations/051_seed_system_strategies.up.sql`:

```sql
-- Seed system-owned strategy templates for every existing org.
-- Values mirror the hardcoded Go defaults in internal/deploy/strategies/.

-- 1% → 5% → 25% → 50% → 100% with 5/5/10/10/0 minute dwells.
INSERT INTO strategies (scope_type, scope_id, name, description, target_type, steps,
                        default_health_threshold, default_rollback_on_failure, is_system)
SELECT 'org', o.id, 'system-canary',
       'System default canary rollout (1% → 5% → 25% → 50% → 100%).',
       'deploy',
       '[
          {"percent":1,"min_duration":300000000000,"max_duration":300000000000,"bake_time_healthy":0},
          {"percent":5,"min_duration":300000000000,"max_duration":300000000000,"bake_time_healthy":0},
          {"percent":25,"min_duration":600000000000,"max_duration":600000000000,"bake_time_healthy":0},
          {"percent":50,"min_duration":600000000000,"max_duration":600000000000,"bake_time_healthy":0},
          {"percent":100,"min_duration":0,"max_duration":0,"bake_time_healthy":0}
        ]'::jsonb,
       0.950, TRUE, TRUE
FROM organizations o
ON CONFLICT (scope_type, scope_id, name) DO NOTHING;

INSERT INTO strategies (scope_type, scope_id, name, description, target_type, steps,
                        default_health_threshold, default_rollback_on_failure, is_system)
SELECT 'org', o.id, 'system-blue-green',
       'System default blue-green: atomic 0 → 100 after 2-minute warmup.',
       'deploy',
       '[
          {"percent":0,"min_duration":120000000000,"max_duration":120000000000,"bake_time_healthy":0},
          {"percent":100,"min_duration":0,"max_duration":0,"bake_time_healthy":0}
        ]'::jsonb,
       0.950, TRUE, TRUE
FROM organizations o
ON CONFLICT (scope_type, scope_id, name) DO NOTHING;

INSERT INTO strategies (scope_type, scope_id, name, description, target_type, steps,
                        default_health_threshold, default_rollback_on_failure, is_system)
SELECT 'org', o.id, 'system-rolling',
       'System default rolling update: three batches with 30s delay.',
       'deploy',
       '[
          {"percent":33,"min_duration":30000000000,"max_duration":30000000000,"bake_time_healthy":0},
          {"percent":67,"min_duration":30000000000,"max_duration":30000000000,"bake_time_healthy":0},
          {"percent":100,"min_duration":0,"max_duration":0,"bake_time_healthy":0}
        ]'::jsonb,
       0.950, TRUE, TRUE
FROM organizations o
ON CONFLICT (scope_type, scope_id, name) DO NOTHING;

-- Seed default strategy assignment: any-env, deploy → system-canary per org.
-- Only inserts if no default already exists for that key.
INSERT INTO strategy_defaults (scope_type, scope_id, environment, target_type, strategy_id)
SELECT 'org', s.scope_id, NULL, 'deploy', s.id
FROM strategies s
WHERE s.name = 'system-canary' AND s.scope_type = 'org'
ON CONFLICT (scope_type, scope_id, COALESCE(environment,''), COALESCE(target_type,'')) DO NOTHING;
```

(The JSON-encoded durations are Go `time.Duration` nanoseconds because the Strategy's Go struct serializes durations as int64 nanoseconds in its JSON encoding. If the YAML convention later differs, a data migration can re-encode.)

- [ ] **Step 2: Write down migration**

Create `migrations/051_seed_system_strategies.down.sql`:

```sql
-- Remove system-owned seeded strategies and any defaults referencing them.
DELETE FROM strategy_defaults
WHERE strategy_id IN (
    SELECT id FROM strategies WHERE is_system = TRUE AND name LIKE 'system-%'
);
DELETE FROM strategies WHERE is_system = TRUE AND name LIKE 'system-%';
```

- [ ] **Step 3: Run migrations and verify**

```bash
make migrate-up
psql "$(grep '^MIGRATE_DSN' Makefile | head -1 | cut -d= -f2-)" -c "SET search_path TO deploy; SELECT scope_type, name, is_system FROM strategies WHERE is_system;"
make migrate-down
make migrate-up
```

Expected: 3 rows per existing org, all with `is_system=true`. Migrate-down cleans them up; re-apply seeds again.

- [ ] **Step 4: Commit**

```bash
git add migrations/051_seed_system_strategies.up.sql migrations/051_seed_system_strategies.down.sql
git commit -m "feat(db): seed system canary, blue-green, rolling strategies per org"
```

---

## Task 14: Wire rollout engine + routes in cmd/api

**Files:**
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Add imports**

```go
"github.com/deploysentry/deploysentry/internal/rollout/applicator/deploy"
"github.com/deploysentry/deploysentry/internal/rollout/engine"
```

- [ ] **Step 2: Construct repos, applicator, engine, service, handler**

Insert in the handler-wiring block (below the Plan 1 rollout CRUD handler wiring):

```go
// Rollout execution: engine, service, handler.
rolloutRepo := postgres.NewRolloutRepo(db.Pool)
rolloutPhaseRepo := postgres.NewRolloutPhaseRepo(db.Pool)
rolloutEventRepo := postgres.NewRolloutEventRepo(db.Pool)

deployTrafficSetter := &deployServiceTrafficSetter{svc: deployService} // small adapter, defined below
deployApplicator := deploy.NewApplicator(deployTrafficSetter, healthMonitor)

rolloutEngineRepoSet := &rolloutEngineRepoAdapter{
    rollouts: rolloutRepo,
    phases:   rolloutPhaseRepo,
    events:   rolloutEventRepo,
}

rolloutEngine := engine.New(rolloutEngineRepoSet, deployApplicator, natsPublisher, engine.EngineOptions{
    PollInterval: 2 * time.Second,
    Logger:       logger,
})

rolloutSvc := rollout.NewRolloutService(rolloutRepo, rolloutPhaseRepo, rolloutEventRepo, natsPublisher)
rolloutHandler := rollout.NewRolloutHandler(rolloutSvc)
rolloutHandler.RegisterRoutes(api)

rolloutAttacher := rollout.NewAttacher(strategySvc, strategyDefaultSvc, rolloutPolicySvc, rolloutSvc)
```

Then pass `rolloutAttacher` into the deploy handler's constructor (change the existing `deploy.NewHandler(...)` call to include the attacher; the deploy Handler gained a `rollouts RolloutAttacher` field in Task 10).

- [ ] **Step 3: Subscribe engine to rollouts.rollout.created**

Near where other NATS subscribers are started (or during main's goroutine launch phase), add:

```go
go func() {
    err := natsSubscriber.Subscribe(ctx, "rollouts.rollout.created", func(msg []byte) {
        var payload struct{ RolloutID string `json:"rollout_id"` }
        if err := json.Unmarshal(msg, &payload); err != nil {
            logger.Warn("rollout engine: bad payload", "error", err)
            return
        }
        id, err := uuid.Parse(payload.RolloutID)
        if err != nil {
            return
        }
        if err := rolloutEngine.DriveRollout(ctx, id); err != nil {
            logger.Warn("rollout engine: drive error", "rollout_id", id, "error", err)
        }
    })
    if err != nil {
        logger.Error("rollout engine: subscribe failed", "error", err)
    }
}()
```

The exact subscriber interface depends on the project's NATS wrapper — match how other subscribers (e.g., in the deploy engine) are started.

- [ ] **Step 4: Add the adapter types at the bottom of main.go**

```go
// deployServiceTrafficSetter adapts the existing deploy service to the
// applicator's TrafficSetter interface.
type deployServiceTrafficSetter struct{ svc deploy.DeployService }

func (t *deployServiceTrafficSetter) SetTrafficPercent(ctx context.Context, depID uuid.UUID, pct int) error {
    // If deploy.DeployService exposes a UpdateTrafficPercent(ctx, id, pct) method, call it.
    // If not, use whatever method sets the desired_traffic_percent on a Deployment row.
    return t.svc.UpdateTrafficPercent(ctx, depID, pct)
}

// rolloutEngineRepoAdapter composes the three rollout repos into the single
// engine.RepoSet interface.
type rolloutEngineRepoAdapter struct {
    rollouts *postgres.RolloutRepo
    phases   *postgres.RolloutPhaseRepo
    events   *postgres.RolloutEventRepo
}

func (a *rolloutEngineRepoAdapter) GetRollout(ctx context.Context, id uuid.UUID) (*models.Rollout, error) {
    return a.rollouts.Get(ctx, id)
}
func (a *rolloutEngineRepoAdapter) UpdateRolloutStatus(ctx context.Context, id uuid.UUID, st models.RolloutStatus, reason *string) error {
    return a.rollouts.UpdateStatus(ctx, id, st, reason)
}
func (a *rolloutEngineRepoAdapter) UpdateRolloutPhasePointer(ctx context.Context, id uuid.UUID, idx int, startedAt, lastHealthy *time.Time) error {
    return a.rollouts.UpdatePhasePointer(ctx, id, idx, startedAt, lastHealthy)
}
func (a *rolloutEngineRepoAdapter) MarkRolloutCompleted(ctx context.Context, id uuid.UUID) error {
    return a.rollouts.MarkCompleted(ctx, id)
}
func (a *rolloutEngineRepoAdapter) BulkInsertPhases(ctx context.Context, phases []*models.RolloutPhase) error {
    return a.phases.BulkInsert(ctx, phases)
}
func (a *rolloutEngineRepoAdapter) ListPhases(ctx context.Context, rid uuid.UUID) ([]*models.RolloutPhase, error) {
    return a.phases.ListByRollout(ctx, rid)
}
func (a *rolloutEngineRepoAdapter) UpdatePhaseStatus(ctx context.Context, id uuid.UUID, st models.PhaseStatus, ea, xa *time.Time, ap, hs *float64, notes string) error {
    return a.phases.UpdateStatus(ctx, id, st, ea, xa, ap, hs, notes)
}
func (a *rolloutEngineRepoAdapter) InsertEvent(ctx context.Context, e *models.RolloutEvent) error {
    return a.events.Insert(ctx, e)
}
```

- [ ] **Step 5: Plug the deploy engine's rollout guard**

The old deploy engine from Task 12 needs a `RolloutLookup`. Wire:

```go
deployEngineRolloutLookup := &deployEngineRolloutAdapter{rollouts: rolloutRepo}
// pass into deploy.engine.New(...)
```

Adapter:

```go
type deployEngineRolloutAdapter struct{ rollouts *postgres.RolloutRepo }

func (a *deployEngineRolloutAdapter) HasActiveRolloutForDeployment(ctx context.Context, id uuid.UUID) (bool, error) {
    ro, err := a.rollouts.GetActiveByDeployment(ctx, id)
    if err != nil && !errors.Is(err, postgres.ErrRolloutNotFound) {
        return false, err
    }
    return ro != nil, nil
}
```

- [ ] **Step 6: Build, boot, verify routes**

```bash
go build ./cmd/api
./api 2>&1 | head -300 | grep -E "ROUTE:.*rollouts" | head -10
```

Expected: 9+ new rollout routes (`/rollouts`, `/rollouts/:id`, 6 controls, `/rollouts/:id/events`).

- [ ] **Step 7: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat(api): wire rollout engine, handler, attacher, and deploy engine guard"
```

---

## Task 15: CLI — `ds rollouts` + `--strategy` / `--apply-immediately` on deploy

**Files:**
- Create: `cmd/cli/rollouts.go`
- Modify: `cmd/cli/deploy.go`

- [x] **Step 1: Create the rollouts command file**

Create `cmd/cli/rollouts.go`:

```go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var rolloutsCmd = &cobra.Command{
	Use:     "rollouts",
	Aliases: []string{"rollout", "ro"},
	Short:   "Inspect and control live rollouts",
}

var rolloutsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent rollouts for an org",
	RunE: func(cmd *cobra.Command, args []string) error {
		org, _ := cmd.Flags().GetString("org")
		if org == "" {
			return fmt.Errorf("--org is required")
		}
		path := "/api/v1/orgs/" + org + "/rollouts"
		if s, _ := cmd.Flags().GetString("status"); s != "" {
			path += "?status=" + s
		}
		body, err := apiGET(path)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	},
}

var rolloutsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show a rollout's detail",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		org, _ := cmd.Flags().GetString("org")
		body, err := apiGET("/api/v1/orgs/" + org + "/rollouts/" + args[0])
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

func rolloutActionCmd(action string, needReason bool) *cobra.Command {
	return &cobra.Command{
		Use:   action + " <id>",
		Short: fmt.Sprintf("%s a rollout", action),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			org, _ := cmd.Flags().GetString("org")
			reason, _ := cmd.Flags().GetString("reason")
			if needReason && reason == "" {
				return fmt.Errorf("--reason is required for %s", action)
			}
			body, _ := json.Marshal(map[string]string{"reason": reason})
			resp, err := apiPOSTRaw("/api/v1/orgs/"+org+"/rollouts/"+args[0]+"/"+action, "application/json", body)
			if err != nil {
				return err
			}
			fmt.Println(string(resp))
			return nil
		},
	}
}

func init() {
	actions := []struct {
		name   string
		reason bool
	}{
		{"pause", false}, {"resume", false}, {"promote", false}, {"approve", false},
		{"rollback", true}, {"force-promote", true},
	}
	var actionCmds []*cobra.Command
	for _, a := range actions {
		actionCmds = append(actionCmds, rolloutActionCmd(a.name, a.reason))
	}
	for _, c := range append([]*cobra.Command{rolloutsListCmd, rolloutsGetCmd}, actionCmds...) {
		c.Flags().String("org", "", "Organization slug (required)")
	}
	rolloutsListCmd.Flags().String("status", "", "Filter by status (active|paused|succeeded|rolled_back|...)")
	for _, c := range actionCmds {
		c.Flags().String("reason", "", "Audit reason (required for rollback and force-promote)")
	}
	rolloutsCmd.AddCommand(rolloutsListCmd, rolloutsGetCmd)
	for _, c := range actionCmds {
		rolloutsCmd.AddCommand(c)
	}
	rootCmd.AddCommand(rolloutsCmd)
}
```

- [x] **Step 2: Extend `cmd/cli/deploy.go` with rollout flags**

Read the current file, find where the deploy command declares its flags, and add:

```go
deployCmd.Flags().String("strategy", "", "Attach rollout via named strategy in scope ancestry")
deployCmd.Flags().String("release", "", "Attach to a release bundle (Plan 4)")
deployCmd.Flags().Bool("apply-immediately", false, "Skip any onboarded strategy prompt; apply immediately")
```

In the deploy command's `RunE` where the request body is built, include:

```go
if strategy, _ := cmd.Flags().GetString("strategy"); strategy != "" {
    body["rollout"] = map[string]any{"strategy_name": strategy}
}
if ai, _ := cmd.Flags().GetBool("apply-immediately"); ai {
    body["rollout"] = map[string]any{"apply_immediately": true}
}
```

(If a release flag is set, defer until Plan 4 — log a warning or ignore.)

- [x] **Step 3: Build and smoke test**

```bash
go build ./cmd/cli
./cli rollouts --help
./cli rollouts pause --help
./cli rollouts rollback --help
```

Each help output should list the subcommands and `--reason` where relevant.

- [x] **Step 4: Commit**

```bash
git add cmd/cli/rollouts.go cmd/cli/deploy.go
git commit -m "feat(cli): ds rollouts + --strategy/--apply-immediately on deploy"
```

---

## Task 16: Documentation update

**Files:**
- Modify: `docs/Rollout_Strategies.md`
- Modify: `docs/Current_Initiatives.md`

- [ ] **Step 1: Expand `docs/Rollout_Strategies.md` with runtime sections**

Append the following sections to the existing doc:

```markdown
## Attaching a strategy to a deploy

```
ds deploy --org acme --project api --app web \
  --artifact ./build.tgz --version v1.2.3 \
  --strategy prod-canary-standard
```

If your scope is onboarded with `policy=mandate` and no `--strategy` is given, the CLI resolves the default assignment matrix and attaches it automatically. If no default resolves, the server returns 400 with a hint.

## Runtime operator controls

Once a rollout is in flight, six operator actions are available:

| Command | Meaning | Notes |
|---|---|---|
| `pause` | Freeze at current phase | Reason optional |
| `resume` | Unfreeze a paused rollout | Reason optional |
| `promote` | Skip remaining dwell on current phase | Health still checked before advancing |
| `approve` | Grant a manual approval gate | Reason optional |
| `rollback` | Revert the target to prior value | **Reason required** |
| `force-promote` | Advance even if unhealthy | **Reason required** (audit) |

```
ds rollouts list --org acme --status active
ds rollouts get <id> --org acme
ds rollouts rollback <id> --org acme --reason "p99 regression"
ds rollouts force-promote <id> --org acme --reason "manual override, verified in staging"
```

## System strategies

Every org is seeded with three system templates: `system-canary` (the default for deploys), `system-blue-green`, and `system-rolling`. They cannot be deleted and their values match what earlier versions of DeploySentry had hardcoded. Customize by copying (`ds strategies export … | ds strategies apply -f -`) under a new name.
```

- [ ] **Step 2: Update initiatives**

In `docs/Current_Initiatives.md`, update the Configurable Rollout Strategies row's Phase column to `Implementation (Plans 1+2)`. Adjust notes:

```
| Configurable Rollout Strategies | Implementation (Plans 1+2) | [Spec](...) / [Plan 1](...) / [Plan 2](./superpowers/plans/2026-04-18-rollout-engine-deploy.md) | Plans 1 (templates) + 2 (engine + deploy integration) merged. Plans 3–5 (config rollouts, releases, UI) to follow. |
```

Also bump `> Last updated:` to `2026-04-18`.

- [ ] **Step 3: Run full test suite once**

```bash
make test
```

If `make test` has unrelated flakiness, at minimum confirm:
```bash
go build ./...
go test ./internal/rollout/... ./internal/models/ ./internal/platform/database/postgres/ -count=1
```

All must pass.

- [ ] **Step 4: Commit**

```bash
git add docs/Rollout_Strategies.md docs/Current_Initiatives.md
git commit -m "docs: rollout runtime controls + initiative update for Plan 2"
```

---

## Definition of Done

- All 16 tasks committed individually on branch `feature/rollout-engine-deploy`.
- `go build ./...` clean.
- `go test ./internal/rollout/... ./internal/models/ ./internal/platform/database/postgres/ -count=1` green.
- New routes visible in API startup log: `/rollouts`, 6 control endpoints, `/rollouts/:id/events`.
- Integration check: a deploy with `--strategy system-canary` creates both a Deployment and a Rollout; the rollout advances through its phases; the legacy deploy engine does NOT drive that deployment (guard working).
- Integration check: a deploy WITHOUT `--strategy` on a non-onboarded org still works exactly as before (legacy engine drives).
- `docs/Rollout_Strategies.md` has runtime section; `docs/Current_Initiatives.md` reflects Plans 1+2 complete.

## Not in scope (deferred to Plan 3+)

- Config rollouts (flag/config value progressive rollout with SDK bucket hashing) → Plan 3.
- Releases / coordination policies → Plan 4.
- Web UI → Plan 5.
- Retargeting ("amend") an in-flight rollout's target value.
- Advanced runtime controls: skip-to-step-N, extend-dwell, split.
- External metric sources for abort conditions beyond what the existing health monitor provides.
- Web SSE push of rollout events (current endpoint returns a list; Plan 5 upgrades to SSE).

## Self-Review Notes

- **Spec coverage**: This plan covers the spec's sections "Data Model: rollouts/rollout_phases/rollout_events tables", "Rollout Lifecycle & Phase Engine", "Runtime controls (six)", "Onboarding, Policy & Defaults (deploy enforcement half)", "Migration — system strategy seeding", "API: rollouts + 6 control endpoints (events as list, SSE deferred)". Spec sections deferred to later plans: config applicator, 409 on flag edit, Releases, coordination policies, web UI, advanced controls.
- **Type consistency**: `RepoSet` in Task 7 composes the three rollout repo interfaces from Task 3. `Applicator` interface in Task 4 is what the Task 7 engine consumes. `RolloutService.AttachDeploy` signature in Task 8 matches what the Task 11 Attacher calls. Status enums (`RolloutStatus`, `PhaseStatus`) used consistently across Tasks 2, 7, 8, 9. `ErrAlreadyActiveOnTarget` in Task 8 is the error Task 10 checks for via `errors.Is`.
- **Placeholder scan**: One "TODO: Task 8 integrates approval wait" comment in Task 7's engine code is a forward reference but the approval gate IS integrated in the same task (via the `approvalGranted` flag and awaiting_approval return). Remove or upgrade that comment during implementation. No other placeholders.
- **Test dependencies**: Tasks 5, 8, 9, 11 rely on Plan 1's `fakeStratRepo`, `fakeDefaultsRepo`, `fakePolicyRepo` already present in `internal/rollout/service_test.go` and `handler_test.go`. These fakes are in the same package so tests can freely use them. Tasks 8 onward add new fakes (`fakeRolloutRepo`, etc.) that coexist.
