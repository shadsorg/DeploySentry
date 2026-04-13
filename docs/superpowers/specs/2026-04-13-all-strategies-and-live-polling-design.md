# All-Strategy Engine Support + Live Polling — Design Spec

**Date:** 2026-04-13
**Status:** Complete
**Depends on:** `2026-04-13-canary-rollout-e2e-design.md`, `2026-04-13-engine-reliability-hardening-design.md`

## Overview

Extend the phase engine to drive rolling and blue-green deployments (not just canary), and add live polling to the frontend so users see deployment progress in real time.

Two categories of work:
1. **All-strategy engine support** — unified phase builder, rolling/blue-green phase generation, remove canary-only filter
2. **Frontend live polling** — interval-based polling for active deployments

---

## Section 1: Unified Phase Builder

A single entry point that builds phases for any strategy type:

```go
func BuildPhasesForStrategy(deploymentID uuid.UUID, strategy models.DeployStrategyType) []*models.DeploymentPhase
```

Dispatches to strategy-specific builders:

### Canary (unchanged)

Default phases: `[1%, 5%, 25%, 50%, 100%]` from `DefaultCanaryConfig()`. Named `canary-1%`, `canary-5%`, etc. Each phase has an observation duration and optional health gate.

### Rolling (new)

Evenly-spaced traffic steps based on batch count. Default 3 batches produces `[33%, 66%, 100%]`. Each phase has a `BatchDelay` observation window with health check. Named `rolling-batch-1`, `rolling-batch-2`, `rolling-batch-3`.

Phase generation:
```
for i := 1; i <= batchCount; i++ {
    trafficPercent = (i * 100) / batchCount
    phase: name="rolling-batch-{i}", traffic=trafficPercent, duration=batchDelay
}
```

### Blue-Green (new)

Three fixed phases:
1. `deploy-green` — 0% traffic, duration = warmup period (default 2 min). External infra deploys new version to the inactive environment during this window.
2. `health-check` — 0% traffic, duration = 30s. Engine checks health of the green environment before switching.
3. `switch-traffic` — 100% traffic, duration = 0. Atomic cutover published via desired-state webhook.

### Engine change

Remove the `if d.Strategy != models.DeployStrategyCanary { return nil }` guard in `driveDeployment`. Replace the canary-specific `BuildPhases(deploymentID, config)` call with `BuildPhasesForStrategy(deploymentID, d.Strategy)`.

The engine loop itself doesn't change — it iterates phases, waits durations, checks health, advances or pauses. The strategy only affects which phases are created.

---

## Section 2: Strategy-Specific Default Configs

### Rolling config simplification

Replace instance-based fields with `BatchCount` for the declarative model:

```go
type RollingConfig struct {
    BatchCount        int           // number of traffic steps (default 3)
    BatchDelay        time.Duration // observation window per batch (default 30s)
    HealthThreshold   float64       // minimum health score (default 0.95)
    RollbackOnFailure bool          // auto-rollback on health failure (default true)
    AutoPromote       bool          // auto-advance between batches (default true)
}
```

The old instance-based fields (`TotalInstances`, `BatchSize`, `MaxUnavailable`, `MaxSurge`) remain in the struct for backward compatibility with the existing `RollingStrategy.Execute` method, but `BatchCount` is what the phase builder uses.

Default: `BatchCount=3, BatchDelay=30s, HealthThreshold=0.95, RollbackOnFailure=true, AutoPromote=true`

### Blue-green config

No changes to the struct. Add `AutoPromote` field:

```go
type BlueGreenConfig struct {
    WarmupDuration    time.Duration // warmup before health check (default 2 min)
    HealthThreshold   float64       // minimum health score (default 0.95)
    RollbackOnFailure bool          // auto-rollback on health failure (default true)
    AutoPromote       bool          // auto-advance through phases (default true)
}
```

### Test helpers

Both configs get the same pattern as canary:
- Package variable: `var defaultRollingConfig = RollingConfig{...}`
- Getter: `func DefaultRollingConfig() RollingConfig`
- Test setter: `func SetDefaultRollingConfigForTest(config RollingConfig)`

Same for blue-green.

---

## Section 3: Frontend Live Polling

Poll every 5 seconds when the deployment is in a non-terminal state. Stop when it reaches a terminal state.

```typescript
useEffect(() => {
  if (!id) return;
  const isActive = dep && !['completed', 'failed', 'rolled_back', 'cancelled'].includes(dep.status);
  if (!isActive) return;

  const interval = setInterval(() => {
    deploymentsApi.get(id).then(setDep);
    deploymentsApi.phases(id).then((data) => setPhases(data.phases || []));
  }, 5000);

  return () => clearInterval(interval);
}, [id, dep?.status]);
```

Properties:
- Only polls when deployment is active (pending, running, paused, promoting)
- Stops automatically when status transitions to terminal
- Refreshes both deployment data and phase list
- 5 second interval — fast enough to feel live, light enough on the API
- Proper cleanup on unmount or status change
- No SSE/WebSocket — polling is simpler and the API already exists

---

## Files Affected

| File | Change |
|------|--------|
| `internal/deploy/engine/engine.go` | Remove canary-only guard, use `BuildPhasesForStrategy` |
| `internal/deploy/engine/phases.go` | New file: `BuildPhasesForStrategy`, `BuildRollingPhases`, `BuildBlueGreenPhases` |
| `internal/deploy/engine/phases_test.go` | New file: tests for all three phase builders |
| `internal/deploy/strategies/rolling.go` | Add `BatchCount`, `AutoPromote`, default config var, test setter |
| `internal/deploy/strategies/bluegreen.go` | Add `AutoPromote`, default config var, test setter |
| `web/src/pages/DeploymentDetailPage.tsx` | Add polling useEffect |
| `internal/deploy/engine/engine_test.go` | Add rolling and blue-green happy path tests |

## What's NOT Changing

- The engine loop itself (phase iteration, health checks, manual gates, transactions, advisory locks)
- The desired-state API and webhook payloads
- The existing canary phase builder and config
- The `RollingStrategy.Execute` and `BlueGreenStrategy.Execute` methods (they still exist for direct use; the engine uses the phase builder instead)
- The ConfirmDialog, action buttons, or any other frontend beyond the polling addition
