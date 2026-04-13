# Engine Reliability Hardening — Design Spec

**Date:** 2026-04-13
**Status:** Approved
**Depends on:** `2026-04-13-canary-rollout-e2e-design.md` (implemented on `feature/canary-rollout-e2e`)

## Overview

Harden the phase engine (`internal/deploy/engine/`) for production use. The engine currently works correctly in the happy path but has three reliability gaps:

1. **No concurrency protection** — duplicate NATS messages or concurrent Advance() calls can spawn parallel goroutines for the same deployment, causing corrupted state
2. **No transaction boundaries** — the engine loop performs multiple writes without DB transactions; a crash mid-step leaves inconsistent state
3. **No crash recovery** — if the API server restarts, in-flight deployments are stuck forever

All three are addressed by making the database the single source of truth and using PostgreSQL's built-in concurrency primitives.

## Approach: Database-Driven State Machine

Instead of relying on in-memory goroutines as the source of truth, the database is the authority. The engine becomes a loop that reads state, does the next step, and writes the result — all in a transaction. PostgreSQL advisory locks prevent concurrent processing.

---

## Section 1: Concurrency Protection (PostgreSQL Advisory Locks)

At the start of `driveDeployment`, acquire a PostgreSQL advisory lock keyed on the deployment ID. If the lock is already held, return immediately — another goroutine is already handling it.

### How it works

```go
locked, err := repo.TryAdvisoryLock(ctx, deploymentID)
if !locked {
    return nil // another goroutine is handling this
}
defer repo.AdvisoryUnlock(ctx, deploymentID)
```

### Advisory lock properties

- **Session-scoped**: released automatically if the connection drops, preventing deadlocks on crash
- **Non-blocking**: `pg_try_advisory_lock` returns false immediately if the lock is held — we skip, not queue
- **Keyed on bigint**: use the first 8 bytes of the deployment UUID as the lock key
- **No new tables**: advisory locks are a built-in PostgreSQL feature

### Lock key derivation

```go
func advisoryLockKey(id uuid.UUID) int64 {
    b := id[:]
    return int64(binary.BigEndian.Uint64(b[:8]))
}
```

### Repository interface additions

```go
TryAdvisoryLock(ctx context.Context, deploymentID uuid.UUID) (bool, error)
AdvisoryUnlock(ctx context.Context, deploymentID uuid.UUID) error
```

Implementation:
```sql
SELECT pg_try_advisory_lock($1)   -- returns true/false
SELECT pg_advisory_unlock($1)     -- releases the lock
```

---

## Section 2: Transaction Boundaries

Wrap each phase step in a database transaction. Events are published after commit — if the publish fails, external infra polls the desired-state API and converges.

### Transaction pattern per phase iteration

```
BEGIN TX
  1. Update phase → active, set started_at
  2. Update deployment → traffic_percent
COMMIT

3. Publish phase_changed event (best-effort, outside tx)

4. Wait duration (respecting ctx cancellation)

5. Check health

BEGIN TX
  6a. If healthy + auto-promote: Update phase → passed, set completed_at
  6b. If unhealthy + rollback: Update phase → failed + Update deployment → rolled_back
  6c. If manual gate: Update deployment → paused
COMMIT

7. Publish event (best-effort, outside tx)
```

### Why publish outside the transaction

NATS publish is not transactional with Postgres. Publishing inside the tx risks sending a false event if the tx rolls back. Publishing after commit means a failed publish is recoverable — external infra polls desired-state and converges. Eventual consistency is acceptable here.

### Repository additions

```go
// WithTx executes fn inside a database transaction. If fn returns an error
// or panics, the transaction is rolled back. Otherwise it is committed.
WithTx(ctx context.Context, fn func(tx TxRepository) error) error
```

`TxRepository` is a subset of `DeployRepository` scoped to a transaction:

```go
type TxRepository interface {
    UpdateDeployment(ctx context.Context, d *models.Deployment) error
    UpdatePhase(ctx context.Context, phase *models.DeploymentPhase) error
    CreateRollbackRecord(ctx context.Context, record *models.RollbackRecord) error
}
```

The pool-based repository creates a `pgx.Tx`, wraps it in a tx-scoped repository struct, passes it to `fn`, and commits or rolls back based on the return.

### Engine usage

```go
err := e.repo.WithTx(ctx, func(tx deploy.TxRepository) error {
    if err := tx.UpdatePhase(ctx, phase); err != nil {
        return err
    }
    return tx.UpdateDeployment(ctx, deployment)
})
if err != nil {
    return fmt.Errorf("phase activation failed: %w", err)
}
// Publish event after successful commit
e.publishPhaseChanged(ctx, deployment, phase, prevDeployment)
```

---

## Section 3: Crash Recovery

On startup, query all non-terminal deployments and resume each one. Additionally, run a periodic sweep to catch any that were missed or whose goroutine died silently.

### Startup recovery

```go
func (e *Engine) resumeInFlight(ctx context.Context) error {
    deps, err := e.repo.ListNonTerminalDeployments(ctx)
    if err != nil {
        return err
    }
    for _, d := range deps {
        go e.driveDeployment(ctx, d.ID)
    }
    return nil
}
```

The advisory lock from Section 1 prevents double-processing — if two instances start simultaneously, only one acquires the lock per deployment.

### Periodic sweep

A background ticker runs every 60 seconds:

```go
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

This catches edge cases: goroutine panics, context cancellation races, advisory lock released but goroutine didn't restart. The lock ensures the sweep is safe — it only picks up deployments no one else is working on.

### New repository method

```go
ListNonTerminalDeployments(ctx context.Context) ([]*models.Deployment, error)
```

```sql
SELECT ... FROM deployments
WHERE status IN ('pending', 'running', 'paused', 'promoting')
  AND strategy = 'canary'
ORDER BY created_at ASC
```

### Stale phase detection

When resuming, the engine checks for phases that are `active` but have exceeded their duration by more than 2x. This indicates the previous goroutine died mid-phase.

The engine re-enters the phase loop from that point. The duration wait is recalculated from `started_at`:

```go
elapsed := time.Since(*phase.StartedAt)
remaining := time.Duration(phase.Duration)*time.Second - elapsed
if remaining > 0 {
    // Wait remaining time
} else {
    // Phase duration already elapsed, proceed to health check
}
```

---

## What's NOT Changing

- The `driveDeployment` loop structure (same phase iteration, same health check, same auto-promote/manual gate logic)
- The engine's public API (`Start`, `Advance`, `BuildPhases`)
- The desired-state API and webhook payloads
- The NATS subscription model (advisory locks supplement it, not replace it)
- The existing test structure (unit + integration tests remain, new tests added for concurrency/recovery)

## Files Affected

| File | Change |
|------|--------|
| `internal/deploy/engine/engine.go` | Add advisory lock acquire/release, wrap state changes in WithTx, implement resumeInFlight + startSweep, recalculate remaining duration |
| `internal/deploy/repository.go` | Add TryAdvisoryLock, AdvisoryUnlock, WithTx, TxRepository, ListNonTerminalDeployments |
| `internal/platform/database/postgres/deploy.go` | Implement advisory lock, WithTx, tx-scoped repository, ListNonTerminalDeployments |
| `internal/deploy/engine/engine_test.go` | Update mocks, add concurrency test, add recovery test |
| `cmd/api/main.go` | Start sweep goroutine alongside engine |
