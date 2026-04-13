# All-Strategy Engine + Live Polling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the phase engine to drive rolling and blue-green deployments (not just canary), and add real-time polling to the deployment detail page.

**Architecture:** Add `BuildPhasesForStrategy` dispatcher that creates phase records for any strategy type. Rolling phases are evenly-spaced traffic steps; blue-green phases are deploy→health-check→switch. Remove canary-only guard from engine. Frontend polls every 5s for active deployments.

**Tech Stack:** Go 1.22, React + TypeScript, existing engine/strategy patterns

**Spec:** `docs/superpowers/specs/2026-04-13-all-strategies-and-live-polling-design.md`

---

## Task 1: Rolling and Blue-Green Config Updates

Add `BatchCount` and `AutoPromote` to `RollingConfig`, add `AutoPromote` to `BlueGreenConfig`, add default config vars and test setters matching the canary pattern.

**Files:**
- Modify: `internal/deploy/strategies/rolling.go`
- Modify: `internal/deploy/strategies/bluegreen.go`

- [ ] **Step 1: Update RollingConfig**

In `internal/deploy/strategies/rolling.go`, add `BatchCount` and `AutoPromote` fields to `RollingConfig`:

```go
type RollingConfig struct {
	// BatchCount is the number of traffic steps in the rolling deployment.
	// Each batch advances traffic by 100/BatchCount percent.
	BatchCount int `json:"batch_count"`
	// BatchSize is the number of instances to update simultaneously (legacy, unused by phase builder).
	BatchSize int `json:"batch_size"`
	// BatchDelay is the delay between successive batch updates.
	BatchDelay time.Duration `json:"batch_delay"`
	// MaxUnavailable is the maximum number of instances that can be unavailable (legacy).
	MaxUnavailable int `json:"max_unavailable"`
	// MaxSurge is the maximum number of extra instances (legacy).
	MaxSurge          int     `json:"max_surge"`
	HealthCheckURL    string  `json:"health_check_url"`
	HealthThreshold   float64 `json:"health_threshold"`
	RollbackOnFailure bool    `json:"rollback_on_failure"`
	// AutoPromote controls whether batches auto-advance after health check.
	AutoPromote bool `json:"auto_promote"`
	// TotalInstances is the total number of instances to update (legacy).
	TotalInstances int `json:"total_instances"`
}
```

- [ ] **Step 2: Add rolling default config var and test setter**

Replace the `DefaultRollingConfig` function with the package-var pattern:

```go
var defaultRollingConfig = RollingConfig{
	BatchCount:        3,
	BatchSize:         1,
	BatchDelay:        30 * time.Second,
	MaxUnavailable:    1,
	MaxSurge:          1,
	HealthThreshold:   0.95,
	RollbackOnFailure: true,
	AutoPromote:       true,
	TotalInstances:    3,
}

func DefaultRollingConfig() RollingConfig {
	return defaultRollingConfig
}

func SetDefaultRollingConfigForTest(config RollingConfig) {
	defaultRollingConfig = config
}
```

- [ ] **Step 3: Update BlueGreenConfig**

In `internal/deploy/strategies/bluegreen.go`, add `AutoPromote` to `BlueGreenConfig`:

```go
type BlueGreenConfig struct {
	WarmupDuration    time.Duration `json:"warmup_duration"`
	HealthCheckURL    string        `json:"health_check_url"`
	HealthThreshold   float64       `json:"health_threshold"`
	RollbackOnFailure bool          `json:"rollback_on_failure"`
	AutoPromote       bool          `json:"auto_promote"`
}
```

- [ ] **Step 4: Add blue-green default config var and test setter**

Replace the `DefaultBlueGreenConfig` function:

```go
var defaultBlueGreenConfig = BlueGreenConfig{
	WarmupDuration:    2 * time.Minute,
	HealthThreshold:   0.95,
	RollbackOnFailure: true,
	AutoPromote:       true,
}

func DefaultBlueGreenConfig() BlueGreenConfig {
	return defaultBlueGreenConfig
}

func SetDefaultBlueGreenConfigForTest(config BlueGreenConfig) {
	defaultBlueGreenConfig = config
}
```

- [ ] **Step 5: Verify build and tests**

Run: `cd /Users/sgamel/git/DeploySentry/.worktrees/canary-rollout-e2e && go build ./... && go test ./internal/deploy/strategies/ -v -timeout 15s`
Expected: Build succeeds, all strategy tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/deploy/strategies/rolling.go internal/deploy/strategies/bluegreen.go
git commit -m "feat(strategies): add BatchCount, AutoPromote, and test config setters for rolling/blue-green"
```

---

## Task 2: Phase Builders for Rolling and Blue-Green

Create a new file with strategy-specific phase builders and a unified dispatcher.

**Files:**
- Create: `internal/deploy/engine/phases.go`
- Create: `internal/deploy/engine/phases_test.go`

- [ ] **Step 1: Write tests for all three builders**

Create `internal/deploy/engine/phases_test.go`:

```go
package engine

import (
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/deploy/strategies"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRollingPhases(t *testing.T) {
	depID := uuid.New()
	config := strategies.RollingConfig{
		BatchCount:        3,
		BatchDelay:        30 * time.Second,
		HealthThreshold:   0.95,
		RollbackOnFailure: true,
		AutoPromote:       true,
	}

	phases := BuildRollingPhases(depID, config)

	require.Len(t, phases, 3)

	assert.Equal(t, "rolling-batch-1", phases[0].Name)
	assert.Equal(t, 33, phases[0].TrafficPercent)
	assert.Equal(t, 30, phases[0].Duration)
	assert.Equal(t, 0, phases[0].SortOrder)
	assert.True(t, phases[0].AutoPromote)
	assert.Equal(t, models.PhaseStatusPending, phases[0].Status)

	assert.Equal(t, "rolling-batch-2", phases[1].Name)
	assert.Equal(t, 66, phases[1].TrafficPercent)
	assert.Equal(t, 1, phases[1].SortOrder)

	assert.Equal(t, "rolling-batch-3", phases[2].Name)
	assert.Equal(t, 100, phases[2].TrafficPercent)
	assert.Equal(t, 2, phases[2].SortOrder)
}

func TestBuildRollingPhases_SingleBatch(t *testing.T) {
	depID := uuid.New()
	config := strategies.RollingConfig{
		BatchCount: 1,
		BatchDelay: 0,
		AutoPromote: true,
	}

	phases := BuildRollingPhases(depID, config)

	require.Len(t, phases, 1)
	assert.Equal(t, 100, phases[0].TrafficPercent)
}

func TestBuildBlueGreenPhases(t *testing.T) {
	depID := uuid.New()
	config := strategies.BlueGreenConfig{
		WarmupDuration:    2 * time.Minute,
		HealthThreshold:   0.95,
		RollbackOnFailure: true,
		AutoPromote:       true,
	}

	phases := BuildBlueGreenPhases(depID, config)

	require.Len(t, phases, 3)

	assert.Equal(t, "deploy-green", phases[0].Name)
	assert.Equal(t, 0, phases[0].TrafficPercent)
	assert.Equal(t, 120, phases[0].Duration) // 2 min warmup
	assert.Equal(t, 0, phases[0].SortOrder)
	assert.True(t, phases[0].AutoPromote)

	assert.Equal(t, "health-check", phases[1].Name)
	assert.Equal(t, 0, phases[1].TrafficPercent)
	assert.Equal(t, 30, phases[1].Duration)
	assert.Equal(t, 1, phases[1].SortOrder)

	assert.Equal(t, "switch-traffic", phases[2].Name)
	assert.Equal(t, 100, phases[2].TrafficPercent)
	assert.Equal(t, 0, phases[2].Duration)
	assert.Equal(t, 2, phases[2].SortOrder)
}

func TestBuildPhasesForStrategy_Dispatches(t *testing.T) {
	depID := uuid.New()

	canaryPhases := BuildPhasesForStrategy(depID, models.DeployStrategyCanary)
	assert.Greater(t, len(canaryPhases), 0)
	assert.Contains(t, canaryPhases[0].Name, "canary-")

	rollingPhases := BuildPhasesForStrategy(depID, models.DeployStrategyRolling)
	assert.Greater(t, len(rollingPhases), 0)
	assert.Contains(t, rollingPhases[0].Name, "rolling-")

	bgPhases := BuildPhasesForStrategy(depID, models.DeployStrategyBlueGreen)
	assert.Len(t, bgPhases, 3)
	assert.Equal(t, "deploy-green", bgPhases[0].Name)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/deploy/engine/ -run TestBuildRolling -v`
Expected: FAIL — `BuildRollingPhases` undefined.

- [ ] **Step 3: Implement phases.go**

Create `internal/deploy/engine/phases.go`:

```go
package engine

import (
	"fmt"

	"github.com/deploysentry/deploysentry/internal/deploy/strategies"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// BuildPhasesForStrategy creates DeploymentPhase records for any deployment
// strategy. It dispatches to the appropriate strategy-specific builder.
func BuildPhasesForStrategy(deploymentID uuid.UUID, strategy models.DeployStrategyType) []*models.DeploymentPhase {
	switch strategy {
	case models.DeployStrategyCanary:
		return BuildPhases(deploymentID, strategies.DefaultCanaryConfig())
	case models.DeployStrategyRolling:
		return BuildRollingPhases(deploymentID, strategies.DefaultRollingConfig())
	case models.DeployStrategyBlueGreen:
		return BuildBlueGreenPhases(deploymentID, strategies.DefaultBlueGreenConfig())
	default:
		return nil
	}
}

// BuildRollingPhases creates DeploymentPhase records for a rolling deployment.
// Traffic is divided evenly across BatchCount steps.
func BuildRollingPhases(deploymentID uuid.UUID, config strategies.RollingConfig) []*models.DeploymentPhase {
	count := config.BatchCount
	if count <= 0 {
		count = 3
	}

	phases := make([]*models.DeploymentPhase, 0, count)
	for i := 1; i <= count; i++ {
		trafficPercent := (i * 100) / count
		phases = append(phases, &models.DeploymentPhase{
			ID:             uuid.New(),
			DeploymentID:   deploymentID,
			Name:           fmt.Sprintf("rolling-batch-%d", i),
			Status:         models.PhaseStatusPending,
			TrafficPercent: trafficPercent,
			Duration:       int(config.BatchDelay.Seconds()),
			SortOrder:      i - 1,
			AutoPromote:    config.AutoPromote,
		})
	}
	return phases
}

// BuildBlueGreenPhases creates three fixed DeploymentPhase records for a
// blue-green deployment: deploy-green (warmup), health-check, switch-traffic.
func BuildBlueGreenPhases(deploymentID uuid.UUID, config strategies.BlueGreenConfig) []*models.DeploymentPhase {
	return []*models.DeploymentPhase{
		{
			ID:             uuid.New(),
			DeploymentID:   deploymentID,
			Name:           "deploy-green",
			Status:         models.PhaseStatusPending,
			TrafficPercent: 0,
			Duration:       int(config.WarmupDuration.Seconds()),
			SortOrder:      0,
			AutoPromote:    config.AutoPromote,
		},
		{
			ID:             uuid.New(),
			DeploymentID:   deploymentID,
			Name:           "health-check",
			Status:         models.PhaseStatusPending,
			TrafficPercent: 0,
			Duration:       30, // 30 second health check window
			SortOrder:      1,
			AutoPromote:    config.AutoPromote,
		},
		{
			ID:             uuid.New(),
			DeploymentID:   deploymentID,
			Name:           "switch-traffic",
			Status:         models.PhaseStatusPending,
			TrafficPercent: 100,
			Duration:       0,
			SortOrder:      2,
			AutoPromote:    config.AutoPromote,
		},
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/deploy/engine/ -run "TestBuildRolling|TestBuildBlueGreen|TestBuildPhasesForStrategy" -v`
Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/deploy/engine/phases.go internal/deploy/engine/phases_test.go
git commit -m "feat(engine): add rolling and blue-green phase builders with BuildPhasesForStrategy"
```

---

## Task 3: Remove Canary-Only Guard from Engine

Update `driveDeployment` to handle all strategies using `BuildPhasesForStrategy`.

**Files:**
- Modify: `internal/deploy/engine/engine.go`
- Modify: `internal/deploy/engine/engine_test.go`
- Modify: `internal/platform/database/postgres/deploy.go` (remove canary filter from ListNonTerminalDeployments)

- [ ] **Step 1: Remove canary-only guard**

In `internal/deploy/engine/engine.go`, find and remove these lines (around line 214-217):

```go
	// 2. Only process canary deployments.
	if d.Strategy != models.DeployStrategyCanary {
		return nil
	}
```

- [ ] **Step 2: Replace canary-specific phase creation with BuildPhasesForStrategy**

In `driveDeployment`, find the phase creation block (around line 239-247):

```go
	if len(phases) == 0 {
		config := strategies.DefaultCanaryConfig()
		phases = BuildPhases(deploymentID, config)
		for _, ph := range phases {
```

Replace with:

```go
	if len(phases) == 0 {
		phases = BuildPhasesForStrategy(deploymentID, d.Strategy)
		if len(phases) == 0 {
			e.logger.Warn("engine: no phases generated for strategy", "strategy", d.Strategy, "deployment_id", deploymentID)
			return nil
		}
		for _, ph := range phases {
```

- [ ] **Step 3: Remove unused canary config variable**

In `driveDeployment`, find and remove this line (around line 262, after the phase loading):

```go
	config := strategies.DefaultCanaryConfig()
```

The `config.RollbackOnFailure` reference in the health check section needs to be replaced. Since each strategy has `RollbackOnFailure`, look up the value based on strategy type. The simplest approach — always rollback on failure when health monitor detects issues (the per-strategy config was only used for this one boolean):

Replace:
```go
			if config.RollbackOnFailure {
```

With:
```go
			if true { // Always rollback on health failure; per-strategy config was redundant
```

Actually, that's ugly. Better: extract the rollback flag:

```go
	// Determine rollback-on-failure from strategy config.
	rollbackOnFailure := true // default
	switch d.Strategy {
	case models.DeployStrategyCanary:
		rollbackOnFailure = strategies.DefaultCanaryConfig().RollbackOnFailure
	case models.DeployStrategyRolling:
		rollbackOnFailure = strategies.DefaultRollingConfig().RollbackOnFailure
	case models.DeployStrategyBlueGreen:
		rollbackOnFailure = strategies.DefaultBlueGreenConfig().RollbackOnFailure
	}
```

Place this after the phases are loaded, before the phase iteration loop. Then the existing `if config.RollbackOnFailure {` becomes `if rollbackOnFailure {`.

- [ ] **Step 4: Write rolling happy path test**

Add to `internal/deploy/engine/engine_test.go`:

```go
func TestDriveDeployment_RollingHappyPath(t *testing.T) {
	strategies.SetDefaultRollingConfigForTest(strategies.RollingConfig{
		BatchCount:        2,
		BatchDelay:        0,
		HealthThreshold:   0.95,
		RollbackOnFailure: true,
		AutoPromote:       true,
	})
	defer strategies.SetDefaultRollingConfigForTest(strategies.DefaultRollingConfig())

	depID := uuid.New()
	repo := newMockEngineRepo()
	repo.deployments[depID] = &models.Deployment{
		ID:            depID,
		Strategy:      models.DeployStrategyRolling,
		Status:        models.DeployStatusPending,
		Artifact:      "test:v2",
		Version:       "v2",
		ApplicationID: uuid.New(),
		EnvironmentID: uuid.New(),
	}

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

	// Should have rolling-batch-1 and rolling-batch-2 phases
	phases := repo.phases[depID]
	if len(phases) != 2 {
		t.Fatalf("expected 2 phases, got %d", len(phases))
	}
	if phases[0].Name != "rolling-batch-1" {
		t.Errorf("expected rolling-batch-1, got %s", phases[0].Name)
	}
	for _, p := range phases {
		if p.Status != models.PhaseStatusPassed {
			t.Errorf("phase %s should be passed, got %s", p.Name, p.Status)
		}
	}
}
```

- [ ] **Step 5: Write blue-green happy path test**

Add to `internal/deploy/engine/engine_test.go`:

```go
func TestDriveDeployment_BlueGreenHappyPath(t *testing.T) {
	strategies.SetDefaultBlueGreenConfigForTest(strategies.BlueGreenConfig{
		WarmupDuration:    0,
		HealthThreshold:   0.95,
		RollbackOnFailure: true,
		AutoPromote:       true,
	})
	defer strategies.SetDefaultBlueGreenConfigForTest(strategies.DefaultBlueGreenConfig())

	depID := uuid.New()
	repo := newMockEngineRepo()
	repo.deployments[depID] = &models.Deployment{
		ID:            depID,
		Strategy:      models.DeployStrategyBlueGreen,
		Status:        models.DeployStatusPending,
		Artifact:      "test:v2",
		Version:       "v2",
		ApplicationID: uuid.New(),
		EnvironmentID: uuid.New(),
	}

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

	// Should have 3 fixed phases
	phases := repo.phases[depID]
	if len(phases) != 3 {
		t.Fatalf("expected 3 phases, got %d", len(phases))
	}
	if phases[0].Name != "deploy-green" {
		t.Errorf("expected deploy-green, got %s", phases[0].Name)
	}
	if phases[1].Name != "health-check" {
		t.Errorf("expected health-check, got %s", phases[1].Name)
	}
	if phases[2].Name != "switch-traffic" {
		t.Errorf("expected switch-traffic, got %s", phases[2].Name)
	}
	for _, p := range phases {
		if p.Status != models.PhaseStatusPassed {
			t.Errorf("phase %s should be passed, got %s", p.Name, p.Status)
		}
	}
}
```

- [ ] **Step 6: Also update ListNonTerminalDeployments mock**

The `mockEngineRepo.ListNonTerminalDeployments` currently filters by `DeployStrategyCanary`. Remove that filter so it returns all non-terminal deployments:

```go
func (r *mockEngineRepo) ListNonTerminalDeployments(_ context.Context) ([]*models.Deployment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*models.Deployment
	for _, d := range r.deployments {
		if !d.IsTerminal() {
			copy := *d
			result = append(result, &copy)
		}
	}
	return result, nil
}
```

Also update the postgres query in `internal/platform/database/postgres/deploy.go` — remove the `AND strategy = 'canary'` filter:

```sql
WHERE status IN ('pending', 'running', 'paused', 'promoting')
ORDER BY created_at ASC
```

- [ ] **Step 7: Run all engine tests**

Run: `go test ./internal/deploy/engine/ -v -timeout 15s -tags integration`
Expected: All tests pass including the two new strategy tests.

- [ ] **Step 8: Commit**

```bash
git add internal/deploy/engine/engine.go internal/deploy/engine/engine_test.go internal/platform/database/postgres/deploy.go
git commit -m "feat(engine): support rolling and blue-green strategies, remove canary-only guard"
```

---

## Task 4: Frontend Live Polling

Add interval-based polling for active deployments on the DeploymentDetailPage.

**Files:**
- Modify: `web/src/pages/DeploymentDetailPage.tsx`

- [ ] **Step 1: Read current DeploymentDetailPage**

Read `web/src/pages/DeploymentDetailPage.tsx` to understand the current useEffect structure and state variables.

- [ ] **Step 2: Add polling useEffect**

Add a second `useEffect` after the existing fetch effect. This one sets up an interval when the deployment is active:

```tsx
  // Poll for updates while deployment is active
  useEffect(() => {
    if (!id || !dep) return;

    const terminalStatuses = ['completed', 'failed', 'rolled_back', 'cancelled'];
    if (terminalStatuses.includes(dep.status)) return;

    const interval = setInterval(() => {
      deploymentsApi.get(id).then(setDep).catch(() => {});
      deploymentsApi.phases(id).then((data) => setPhases(data.phases || [])).catch(() => {});
    }, 5000);

    return () => clearInterval(interval);
  }, [id, dep?.status]);
```

Place this after the existing `useEffect` that does the initial fetch (which sets `loading`/`error`/`dep`).

Important: The `.catch(() => {})` on each call prevents unhandled promise rejections from crashing the poll loop. If the API is temporarily unavailable, the next poll will retry.

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/.worktrees/canary-rollout-e2e/web && npx tsc --noEmit 2>&1 | grep -v "TS2688\|Cannot find type"`
Expected: No errors from DeploymentDetailPage.

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/DeploymentDetailPage.tsx
git commit -m "feat(web): add live polling for active deployments on detail page"
```

---

## Task 5: Final Verification

Run all tests, verify full build, update spec.

**Files:** None (verification only)

- [ ] **Step 1: Full Go build**

Run: `cd /Users/sgamel/git/DeploySentry/.worktrees/canary-rollout-e2e && go build ./...`
Expected: Build succeeds.

- [ ] **Step 2: Run all deploy tests**

Run: `go test ./internal/deploy/... -v -timeout 30s -tags integration`
Expected: All pass.

- [ ] **Step 3: Run strategy tests**

Run: `go test ./internal/deploy/strategies/ -v -timeout 15s`
Expected: All pass.

- [ ] **Step 4: Run postgres tests**

Run: `go test ./internal/platform/database/postgres/ -v -timeout 15s`
Expected: All pass.

- [ ] **Step 5: Mark spec as complete**

In `docs/superpowers/specs/2026-04-13-all-strategies-and-live-polling-design.md`, change `**Status:** Approved` to `**Status:** Complete`.

- [ ] **Step 6: Commit and push**

```bash
git add docs/
git commit -m "docs: mark all-strategy and live polling spec as complete"
git push
```
