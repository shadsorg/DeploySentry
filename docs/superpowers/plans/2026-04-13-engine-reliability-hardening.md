# Engine Reliability Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the phase engine production-safe with PostgreSQL advisory locks for concurrency, database transactions for atomicity, and crash recovery for durability.

**Architecture:** The database becomes the single source of truth. Advisory locks prevent concurrent processing of the same deployment. Each phase step is wrapped in a DB transaction. A periodic sweep resumes orphaned deployments on startup and every 60 seconds.

**Tech Stack:** Go 1.22, pgx/v5, PostgreSQL advisory locks, existing engine/repository patterns

**Spec:** `docs/superpowers/specs/2026-04-13-engine-reliability-hardening-design.md`

---

## Task 1: Querier Interface and Transaction Support

Introduce a `querier` interface that both `*pgxpool.Pool` and `pgx.Tx` satisfy, allowing repository methods to work in either context. Add `WithTx` and `TxRepository` to the deploy repository.

**Files:**
- Create: `internal/platform/database/postgres/querier.go`
- Modify: `internal/platform/database/postgres/deploy.go`
- Modify: `internal/deploy/repository.go`

- [ ] **Step 1: Create querier.go**

Create `internal/platform/database/postgres/querier.go`:

```go
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/corepgx"
	"github.com/jackc/pgx/v5/pgconn"
)

// querier is the common interface between *pgxpool.Pool and pgx.Tx.
// Both types satisfy this interface, allowing repository methods to
// operate transparently inside or outside a transaction.
type querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
```

- [ ] **Step 2: Refactor DeployRepository to use querier**

In `internal/platform/database/postgres/deploy.go`, change the struct to hold a `querier` instead of `*pgxpool.Pool`, and keep a reference to the pool for transaction creation:

```go
// DeployRepository implements deploy.DeployRepository using PostgreSQL.
type DeployRepository struct {
	q    querier        // pool or tx — used for all queries
	pool *pgxpool.Pool  // kept for Begin() and advisory locks
}

// NewDeployRepository creates a new DeployRepository backed by the given pool.
func NewDeployRepository(pool *pgxpool.Pool) *DeployRepository {
	return &DeployRepository{q: pool, pool: pool}
}
```

Then replace every occurrence of `r.pool.Exec`, `r.pool.Query`, `r.pool.QueryRow` with `r.q.Exec`, `r.q.Query`, `r.q.QueryRow` throughout the file. The advisory lock methods (Task 2) will use `r.pool` directly since they must run on the session, not the transaction.

- [ ] **Step 3: Add TxRepository interface to deploy package**

In `internal/deploy/repository.go`, add:

```go
// TxRepository is a subset of DeployRepository scoped to a database transaction.
// It provides only the write methods needed by the engine's phase loop.
type TxRepository interface {
	UpdateDeployment(ctx context.Context, d *models.Deployment) error
	UpdatePhase(ctx context.Context, phase *models.DeploymentPhase) error
	CreateRollbackRecord(ctx context.Context, record *models.RollbackRecord) error
}

// TxFunc is a function executed inside a database transaction.
type TxFunc func(tx TxRepository) error
```

Add `WithTx` to the `DeployRepository` interface:

```go
	// WithTx executes fn inside a database transaction. If fn returns an error
	// or panics, the transaction is rolled back. Otherwise it is committed.
	WithTx(ctx context.Context, fn TxFunc) error
```

- [ ] **Step 4: Implement WithTx in postgres**

In `internal/platform/database/postgres/deploy.go`, add:

```go
// WithTx starts a database transaction, creates a tx-scoped DeployRepository,
// calls fn, and commits or rolls back based on the result.
func (r *DeployRepository) WithTx(ctx context.Context, fn deploy.TxFunc) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres.WithTx: begin: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after commit

	txRepo := &DeployRepository{q: tx, pool: r.pool}
	if err := fn(txRepo); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres.WithTx: commit: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry/.worktrees/canary-rollout-e2e && go build ./...`
Expected: Build succeeds. The `r.pool` → `r.q` replacement must cover every method in the file.

- [ ] **Step 6: Run existing tests**

Run: `go test ./internal/deploy/... -timeout 30s`
Expected: All tests pass (mocks don't implement WithTx yet, but nothing calls it).

- [ ] **Step 7: Commit**

```bash
git add internal/platform/database/postgres/querier.go internal/platform/database/postgres/deploy.go internal/deploy/repository.go
git commit -m "feat(repo): add querier interface and WithTx transaction support"
```

---

## Task 2: Advisory Lock Repository Methods

Add `TryAdvisoryLock` and `AdvisoryUnlock` to the repository for concurrency protection.

**Files:**
- Modify: `internal/deploy/repository.go`
- Modify: `internal/platform/database/postgres/deploy.go`
- Create: `internal/platform/database/postgres/deploy_advisory_test.go`

- [ ] **Step 1: Add interface methods**

In `internal/deploy/repository.go`, add to the `DeployRepository` interface:

```go
	// TryAdvisoryLock attempts to acquire a PostgreSQL advisory lock for the
	// given deployment. Returns true if acquired, false if already held.
	// The lock is session-scoped and released by AdvisoryUnlock or when the
	// connection is returned to the pool.
	TryAdvisoryLock(ctx context.Context, deploymentID uuid.UUID) (bool, error)

	// AdvisoryUnlock releases a previously acquired advisory lock.
	AdvisoryUnlock(ctx context.Context, deploymentID uuid.UUID) error
```

- [ ] **Step 2: Implement in postgres**

In `internal/platform/database/postgres/deploy.go`, add:

```go
// advisoryLockKey derives a bigint key from a UUID for pg_try_advisory_lock.
// Uses the first 8 bytes of the UUID interpreted as a signed int64.
func advisoryLockKey(id uuid.UUID) int64 {
	b := id[:]
	return int64(b[0])<<56 | int64(b[1])<<48 | int64(b[2])<<40 | int64(b[3])<<32 |
		int64(b[4])<<24 | int64(b[5])<<16 | int64(b[6])<<8 | int64(b[7])
}

// TryAdvisoryLock attempts to acquire a session-level advisory lock.
func (r *DeployRepository) TryAdvisoryLock(ctx context.Context, deploymentID uuid.UUID) (bool, error) {
	key := advisoryLockKey(deploymentID)
	var locked bool
	err := r.pool.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&locked)
	if err != nil {
		return false, fmt.Errorf("postgres.TryAdvisoryLock: %w", err)
	}
	return locked, nil
}

// AdvisoryUnlock releases a session-level advisory lock.
func (r *DeployRepository) AdvisoryUnlock(ctx context.Context, deploymentID uuid.UUID) error {
	key := advisoryLockKey(deploymentID)
	_, err := r.pool.Exec(ctx, "SELECT pg_advisory_unlock($1)", key)
	if err != nil {
		return fmt.Errorf("postgres.AdvisoryUnlock: %w", err)
	}
	return nil
}
```

Note: These use `r.pool` directly (not `r.q`) because advisory locks must be on the session connection, not inside a transaction.

- [ ] **Step 3: Write unit test for advisoryLockKey**

Create `internal/platform/database/postgres/deploy_advisory_test.go`:

```go
package postgres

import (
	"testing"

	"github.com/google/uuid"
)

func TestAdvisoryLockKey_Deterministic(t *testing.T) {
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	k1 := advisoryLockKey(id)
	k2 := advisoryLockKey(id)
	if k1 != k2 {
		t.Errorf("advisoryLockKey not deterministic: %d != %d", k1, k2)
	}
}

func TestAdvisoryLockKey_DifferentUUIDs(t *testing.T) {
	id1 := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	id2 := uuid.MustParse("660e8400-e29b-41d4-a716-446655440000")
	k1 := advisoryLockKey(id1)
	k2 := advisoryLockKey(id2)
	if k1 == k2 {
		t.Errorf("different UUIDs should produce different keys: %d == %d", k1, k2)
	}
}
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/platform/database/postgres/ -run TestAdvisoryLockKey -v`
Expected: PASS

- [ ] **Step 5: Update mock repo in engine tests**

In `internal/deploy/engine/engine_test.go`, add to `mockEngineRepo`:

```go
func (r *mockEngineRepo) TryAdvisoryLock(_ context.Context, _ uuid.UUID) (bool, error) {
	return true, nil // always acquire in tests
}

func (r *mockEngineRepo) AdvisoryUnlock(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (r *mockEngineRepo) WithTx(_ context.Context, fn deploy.TxFunc) error {
	return fn(r) // mock just runs fn directly, no actual tx
}
```

- [ ] **Step 6: Update mock in deploy/service_test.go and deploy/handler_test.go**

Add the same stub methods to `mockDeployRepo` in `internal/deploy/service_test.go`:

```go
func (m *mockDeployRepo) TryAdvisoryLock(_ context.Context, _ uuid.UUID) (bool, error) {
	return true, nil
}

func (m *mockDeployRepo) AdvisoryUnlock(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockDeployRepo) WithTx(_ context.Context, fn deploy.TxFunc) error {
	return fn(m)
}
```

And to `mockDeployService` in `handler_test.go` — but wait, `DeployService` doesn't expose these methods, only `DeployRepository` does. So handler_test.go doesn't need changes.

- [ ] **Step 7: Verify all tests pass**

Run: `go test ./internal/deploy/... ./internal/platform/database/postgres/ -timeout 30s`
Expected: All pass.

- [ ] **Step 8: Commit**

```bash
git add internal/deploy/repository.go internal/platform/database/postgres/deploy.go internal/platform/database/postgres/deploy_advisory_test.go internal/deploy/engine/engine_test.go internal/deploy/service_test.go
git commit -m "feat(repo): add advisory lock and WithTx mock support"
```

---

## Task 3: Add ListNonTerminalDeployments

New repository method for crash recovery — finds all canary deployments that should be actively driven.

**Files:**
- Modify: `internal/deploy/repository.go`
- Modify: `internal/platform/database/postgres/deploy.go`

- [ ] **Step 1: Add interface method**

In `internal/deploy/repository.go`, add to `DeployRepository`:

```go
	// ListNonTerminalDeployments returns all canary deployments in a non-terminal
	// state (pending, running, paused, promoting), ordered by created_at ASC.
	// Used by the engine's crash recovery sweep.
	ListNonTerminalDeployments(ctx context.Context) ([]*models.Deployment, error)
```

- [ ] **Step 2: Implement in postgres**

In `internal/platform/database/postgres/deploy.go`, add:

```go
// ListNonTerminalDeployments returns all canary deployments that are not in a
// terminal state, ordered by creation time ascending.
func (r *DeployRepository) ListNonTerminalDeployments(ctx context.Context) ([]*models.Deployment, error) {
	q := `SELECT` + deploymentSelectCols + `
		FROM deployments
		WHERE status IN ('pending', 'running', 'paused', 'promoting')
		  AND strategy = 'canary'
		ORDER BY created_at ASC`

	rows, err := r.q.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListNonTerminalDeployments: %w", err)
	}
	defer rows.Close()

	var deployments []*models.Deployment
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListNonTerminalDeployments: scan: %w", err)
		}
		deployments = append(deployments, d)
	}
	return deployments, rows.Err()
}
```

- [ ] **Step 3: Add mock stub**

In `internal/deploy/engine/engine_test.go`, add to `mockEngineRepo`:

```go
func (r *mockEngineRepo) ListNonTerminalDeployments(_ context.Context) ([]*models.Deployment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*models.Deployment
	for _, d := range r.deployments {
		if !d.IsTerminal() && d.Strategy == models.DeployStrategyCanary {
			copy := *d
			result = append(result, &copy)
		}
	}
	return result, nil
}
```

In `internal/deploy/service_test.go`, add to `mockDeployRepo`:

```go
func (m *mockDeployRepo) ListNonTerminalDeployments(_ context.Context) ([]*models.Deployment, error) {
	return nil, nil
}
```

- [ ] **Step 4: Verify build and tests**

Run: `go build ./... && go test ./internal/deploy/... -timeout 30s`
Expected: Build and tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/deploy/repository.go internal/platform/database/postgres/deploy.go internal/deploy/engine/engine_test.go internal/deploy/service_test.go
git commit -m "feat(repo): add ListNonTerminalDeployments for crash recovery"
```

---

## Task 4: Add Advisory Locks to Engine

Wrap `driveDeployment` with advisory lock acquire/release so only one goroutine processes a deployment at a time.

**Files:**
- Modify: `internal/deploy/engine/engine.go`

- [ ] **Step 1: Add lock at top of driveDeployment**

In `internal/deploy/engine/engine.go`, at the very beginning of `driveDeployment` (after the function signature, before getting the deployment), add:

```go
	// Acquire advisory lock — if another goroutine is already processing
	// this deployment, skip silently.
	locked, err := e.repo.TryAdvisoryLock(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("driveDeployment: advisory lock: %w", err)
	}
	if !locked {
		e.logger.Debug("engine: deployment already locked, skipping", "deployment_id", deploymentID)
		return nil
	}
	defer func() {
		if err := e.repo.AdvisoryUnlock(ctx, deploymentID); err != nil {
			e.logger.Warn("engine: failed to release advisory lock", "deployment_id", deploymentID, "error", err)
		}
	}()
```

- [ ] **Step 2: Run existing tests**

Run: `go test ./internal/deploy/engine/ -v -timeout 15s`
Expected: All pass (mock returns `true` for TryAdvisoryLock).

- [ ] **Step 3: Write concurrency test**

Add to `internal/deploy/engine/engine_test.go`:

```go
func TestDriveDeployment_AdvisoryLockPreventsDoubleProcessing(t *testing.T) {
	strategies.SetDefaultCanaryConfigForTest(strategies.CanaryConfig{
		Steps: []strategies.CanaryStep{
			{TrafficPercent: 50, Duration: 0},
			{TrafficPercent: 100, Duration: 0},
		},
		AutoPromote:       true,
		RollbackOnFailure: true,
		HealthThreshold:   0.95,
	})

	depID := uuid.New()
	repo := newMockEngineRepo()
	repo.deployments[depID] = &models.Deployment{
		ID:       depID,
		Strategy: models.DeployStrategyCanary,
		Status:   models.DeployStatusPending,
		Artifact: "test:v1",
		Version:  "v1",
	}

	// Override TryAdvisoryLock to return false (already locked)
	lockRepo := &lockingMockRepo{mockEngineRepo: repo, lockResult: false}

	pub := &mockPublisher{}
	eng := engine.New(lockRepo, pub, nil, nil)

	err := eng.DriveDeploymentForTest(context.Background(), depID)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	// Deployment should NOT have been processed (still pending)
	d, _ := repo.GetDeployment(context.Background(), depID)
	if d.Status != models.DeployStatusPending {
		t.Errorf("deployment should still be pending, got %s", d.Status)
	}
}
```

Add the `lockingMockRepo` wrapper:

```go
type lockingMockRepo struct {
	*mockEngineRepo
	lockResult bool
}

func (r *lockingMockRepo) TryAdvisoryLock(_ context.Context, _ uuid.UUID) (bool, error) {
	return r.lockResult, nil
}
```

Note: This test requires `driveDeployment` to be exported or a test helper. Add to `engine.go`:

```go
// DriveDeploymentForTest exposes driveDeployment for testing. Not for production use.
func (e *Engine) DriveDeploymentForTest(ctx context.Context, id uuid.UUID) error {
	return e.driveDeployment(ctx, id)
}
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/deploy/engine/ -run TestDriveDeployment_AdvisoryLock -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/deploy/engine/engine.go internal/deploy/engine/engine_test.go
git commit -m "feat(engine): add advisory lock to prevent concurrent deployment processing"
```

---

## Task 5: Wrap Phase Steps in Transactions

Wrap the phase activation and completion writes in `WithTx` calls so each state change is atomic.

**Files:**
- Modify: `internal/deploy/engine/engine.go`

- [ ] **Step 1: Wrap phase activation in transaction**

In `driveDeployment`, replace steps 6a and 6b (lines 208-221):

```go
		// a+b. Atomically set phase active and update deployment traffic.
		now := time.Now().UTC()
		ph.Status = models.PhaseStatusActive
		ph.StartedAt = &now
		d.TrafficPercent = ph.TrafficPercent
		d.UpdatedAt = time.Now().UTC()

		if err := e.repo.WithTx(ctx, func(tx deploy.TxRepository) error {
			if err := tx.UpdatePhase(ctx, ph); err != nil {
				return fmt.Errorf("update phase active: %w", err)
			}
			return tx.UpdateDeployment(ctx, d)
		}); err != nil {
			return fmt.Errorf("driveDeployment: activate phase tx: %w", err)
		}
```

- [ ] **Step 2: Wrap rollback in transaction**

In `triggerRollback`, wrap the deployment update and rollback record creation:

```go
func (e *Engine) triggerRollback(ctx context.Context, d *models.Deployment, h *health.DeploymentHealth) error {
	if err := d.TransitionTo(models.DeployStatusRolledBack); err != nil {
		return fmt.Errorf("triggerRollback: transition: %w", err)
	}
	d.TrafficPercent = 0

	var healthScore *float64
	if h != nil {
		score := h.Overall
		healthScore = &score
	}
	record := &models.RollbackRecord{
		ID:                 uuid.New(),
		DeploymentID:       d.ID,
		TargetDeploymentID: d.PreviousDeploymentID,
		Reason:             "health check failure",
		HealthScore:        healthScore,
		Automatic:          true,
		Strategy:           string(d.Strategy),
		StartedAt:          time.Now().UTC(),
		CreatedAt:          time.Now().UTC(),
	}

	if err := e.repo.WithTx(ctx, func(tx deploy.TxRepository) error {
		if err := tx.UpdateDeployment(ctx, d); err != nil {
			return err
		}
		return tx.CreateRollbackRecord(ctx, record)
	}); err != nil {
		return fmt.Errorf("triggerRollback: tx: %w", err)
	}

	// Publish events (best-effort, after commit)
	if err := e.publishPhaseChanged(ctx, d, nil, nil); err != nil {
		e.logger.Warn("engine: failed to publish phase_changed on rollback", "error", err)
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"deployment_id":   d.ID,
		"application_id":  d.ApplicationID,
		"environment_id":  d.EnvironmentID,
		"artifact":        d.Artifact,
		"version":         d.Version,
		"traffic_percent": 0,
		"status":          string(d.Status),
		"timestamp":       time.Now().UTC(),
	})
	if err := e.publisher.Publish(ctx, "deployments.deployment.rolled_back", payload); err != nil {
		e.logger.Warn("engine: failed to publish deployment.rolled_back", "error", err)
	}

	return nil
}
```

- [ ] **Step 3: Wrap phase completion in transaction**

Replace step 6h (marking phase passed, lines 267-272):

```go
		// h. Mark phase passed (atomic with no other writes needed).
		ph.Status = models.PhaseStatusPassed
		completedAt := time.Now().UTC()
		ph.CompletedAt = &completedAt
		if err := e.repo.WithTx(ctx, func(tx deploy.TxRepository) error {
			return tx.UpdatePhase(ctx, ph)
		}); err != nil {
			return fmt.Errorf("driveDeployment: complete phase tx: %w", err)
		}
```

- [ ] **Step 4: Wrap deployment completion in transaction**

Replace step 7 (lines 275-282):

```go
	// 7. All phases complete.
	d.TrafficPercent = 100
	if err := d.TransitionTo(models.DeployStatusCompleted); err != nil {
		return fmt.Errorf("driveDeployment: transition to completed: %w", err)
	}
	if err := e.repo.WithTx(ctx, func(tx deploy.TxRepository) error {
		return tx.UpdateDeployment(ctx, d)
	}); err != nil {
		return fmt.Errorf("driveDeployment: complete deployment tx: %w", err)
	}
```

- [ ] **Step 5: Run all engine tests**

Run: `go test ./internal/deploy/engine/ -v -timeout 15s -tags integration`
Expected: All 4+ tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/deploy/engine/engine.go
git commit -m "feat(engine): wrap phase steps in database transactions"
```

---

## Task 6: Crash Recovery — resumeInFlight and Sweep

Implement the startup recovery and periodic sweep.

**Files:**
- Modify: `internal/deploy/engine/engine.go`
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Implement resumeInFlight**

In `internal/deploy/engine/engine.go`, replace or add the `resumeInFlight` method:

```go
// resumeInFlight queries all non-terminal canary deployments and spawns a
// driveDeployment goroutine for each. The advisory lock prevents double-processing.
func (e *Engine) resumeInFlight(ctx context.Context) {
	deps, err := e.repo.ListNonTerminalDeployments(ctx)
	if err != nil {
		e.logger.Error("engine: resumeInFlight query failed", "error", err)
		return
	}
	if len(deps) > 0 {
		e.logger.Info("engine: resuming in-flight deployments", "count", len(deps))
	}
	for _, d := range deps {
		id := d.ID
		go func() {
			if err := e.driveDeployment(ctx, id); err != nil {
				e.logger.Error("engine: resume driveDeployment error", "deployment_id", id, "error", err)
			}
		}()
	}
}
```

- [ ] **Step 2: Add startSweep method**

```go
// startSweep runs resumeInFlight periodically to catch orphaned deployments.
func (e *Engine) startSweep(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.resumeInFlight(ctx)
		}
	}
}
```

- [ ] **Step 3: Update Start to call resumeInFlight and startSweep**

Modify the `Start` method to resume in-flight deployments before subscribing, and start the sweep in a goroutine:

```go
func (e *Engine) Start(ctx context.Context, subscriber MessageSubscriber) error {
	// Resume any deployments left in non-terminal state from a previous run.
	e.resumeInFlight(ctx)

	// Start periodic sweep to catch orphaned deployments.
	go e.startSweep(ctx, 60*time.Second)

	// Subscribe to new deployment events.
	if err := subscriber.Subscribe("deployments.deployment.created", func(msg []byte) {
		var payload struct {
			DeploymentID string `json:"deployment_id"`
		}
		if err := json.Unmarshal(msg, &payload); err != nil {
			e.logger.Error("engine: failed to unmarshal deployment.created", "error", err)
			return
		}
		id, err := uuid.Parse(payload.DeploymentID)
		if err != nil {
			e.logger.Error("engine: invalid deployment_id in event", "error", err)
			return
		}
		go func() {
			if err := e.driveDeployment(ctx, id); err != nil {
				e.logger.Error("engine: driveDeployment error", "deployment_id", id, "error", err)
			}
		}()
	}); err != nil {
		return fmt.Errorf("engine: subscribe failed: %w", err)
	}

	<-ctx.Done()
	return ctx.Err()
}
```

- [ ] **Step 4: Add remaining duration calculation for resumed phases**

In `driveDeployment`, in step 6d (the wait), replace the simple duration wait with a calculation that accounts for already-elapsed time when resuming an active phase:

```go
		// d. Wait remaining phase duration (accounts for crash recovery).
		if ph.Duration > 0 {
			dur := time.Duration(ph.Duration) * time.Second
			if ph.StartedAt != nil {
				elapsed := time.Since(*ph.StartedAt)
				dur -= elapsed
			}
			if dur > 0 {
				timer := time.NewTimer(dur)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				case <-timer.C:
				}
			}
			// If dur <= 0, phase duration already elapsed — proceed immediately
		}
```

- [ ] **Step 5: Run all tests**

Run: `go test ./internal/deploy/engine/ -v -timeout 15s -tags integration`
Expected: All pass.

- [ ] **Step 6: Write crash recovery test**

Add to `internal/deploy/engine/engine_test.go`:

```go
func TestResumeInFlight_PicksUpRunningDeployments(t *testing.T) {
	strategies.SetDefaultCanaryConfigForTest(strategies.CanaryConfig{
		Steps: []strategies.CanaryStep{
			{TrafficPercent: 50, Duration: 0},
			{TrafficPercent: 100, Duration: 0},
		},
		AutoPromote:       true,
		RollbackOnFailure: true,
		HealthThreshold:   0.95,
	})

	depID := uuid.New()
	repo := newMockEngineRepo()
	repo.deployments[depID] = &models.Deployment{
		ID:            depID,
		Strategy:      models.DeployStrategyCanary,
		Status:        models.DeployStatusRunning,
		Artifact:      "test:v1",
		Version:       "v1",
		ApplicationID: uuid.New(),
		EnvironmentID: uuid.New(),
	}

	pub := &mockPublisher{}
	eng := engine.New(repo, pub, nil, nil)

	// Call resumeInFlight (exposed via Start, but we can test via DriveDeploymentForTest)
	// Since the deployment is already running, driveDeployment should pick it up and complete it.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := eng.DriveDeploymentForTest(ctx, depID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d, _ := repo.GetDeployment(ctx, depID)
	if d.Status != models.DeployStatusCompleted {
		t.Errorf("expected completed, got %s", d.Status)
	}
}

func TestDriveDeployment_ResumesFromActivePhase(t *testing.T) {
	strategies.SetDefaultCanaryConfigForTest(strategies.CanaryConfig{
		Steps: []strategies.CanaryStep{
			{TrafficPercent: 25, Duration: 0},
			{TrafficPercent: 75, Duration: 0},
			{TrafficPercent: 100, Duration: 0},
		},
		AutoPromote:       true,
		RollbackOnFailure: true,
		HealthThreshold:   0.95,
	})

	depID := uuid.New()
	repo := newMockEngineRepo()
	repo.deployments[depID] = &models.Deployment{
		ID:            depID,
		Strategy:      models.DeployStrategyCanary,
		Status:        models.DeployStatusRunning,
		Artifact:      "test:v1",
		Version:       "v1",
		ApplicationID: uuid.New(),
		EnvironmentID: uuid.New(),
	}

	// Pre-create phases: first passed, second active (simulating crash mid-phase)
	startedAt := time.Now().Add(-10 * time.Minute) // started 10 min ago
	completedAt := time.Now().Add(-9 * time.Minute)
	phase1 := &models.DeploymentPhase{
		ID: uuid.New(), DeploymentID: depID, Name: "canary-25%",
		Status: models.PhaseStatusPassed, TrafficPercent: 25,
		Duration: 0, SortOrder: 0, AutoPromote: true,
		StartedAt: &startedAt, CompletedAt: &completedAt,
	}
	phase2 := &models.DeploymentPhase{
		ID: uuid.New(), DeploymentID: depID, Name: "canary-75%",
		Status: models.PhaseStatusActive, TrafficPercent: 75,
		Duration: 60, SortOrder: 1, AutoPromote: true,
		StartedAt: &startedAt, // started 10 min ago, duration was 60s — long expired
	}
	phase3 := &models.DeploymentPhase{
		ID: uuid.New(), DeploymentID: depID, Name: "canary-100%",
		Status: models.PhaseStatusPending, TrafficPercent: 100,
		Duration: 0, SortOrder: 2, AutoPromote: true,
	}
	repo.phases[depID] = []*models.DeploymentPhase{phase1, phase2, phase3}

	pub := &mockPublisher{}
	eng := engine.New(repo, pub, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := eng.DriveDeploymentForTest(ctx, depID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d, _ := repo.GetDeployment(ctx, depID)
	if d.Status != models.DeployStatusCompleted {
		t.Errorf("expected completed, got %s", d.Status)
	}
	if d.TrafficPercent != 100 {
		t.Errorf("expected 100%% traffic, got %d%%", d.TrafficPercent)
	}
}
```

- [ ] **Step 7: Run tests**

Run: `go test ./internal/deploy/engine/ -v -timeout 15s`
Expected: All pass including new recovery tests.

- [ ] **Step 8: Commit**

```bash
git add internal/deploy/engine/engine.go internal/deploy/engine/engine_test.go
git commit -m "feat(engine): add crash recovery with resumeInFlight and periodic sweep"
```

---

## Task 7: Final Verification

Run all tests, verify full build, ensure nothing is broken.

**Files:** None (verification only)

- [ ] **Step 1: Run full Go build**

Run: `cd /Users/sgamel/git/DeploySentry/.worktrees/canary-rollout-e2e && go build ./...`
Expected: Build succeeds.

- [ ] **Step 2: Run all deploy tests**

Run: `go test ./internal/deploy/... -v -timeout 30s -tags integration`
Expected: All pass.

- [ ] **Step 3: Run postgres package tests**

Run: `go test ./internal/platform/database/postgres/ -v -timeout 30s`
Expected: All pass.

- [ ] **Step 4: Run notification tests (unchanged)**

Run: `go test ./internal/notifications/ -timeout 30s`
Expected: All pass.

- [ ] **Step 5: Update spec phase**

Update `docs/superpowers/specs/2026-04-13-engine-reliability-hardening-design.md`:
Change `**Status:** Approved` to `**Status:** Complete`

- [ ] **Step 6: Commit**

```bash
git add docs/
git commit -m "docs: mark engine reliability hardening spec as complete"
```
