# Config Rollouts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the Plan 2 rollout engine to drive config/flag targeting-rule percentage changes progressively. A user editing `PUT /flags/:id/rules/:ruleId` can attach a strategy; the rollout engine shifts the rule's `percentage` field through the strategy's phases. Backward-compatible: rule edits without rollout intent keep today's immediate-apply behavior.

**Architecture:** A new `config` applicator (`internal/rollout/applicator/config/`) applies a step's percent to a targeting rule via the existing `flags.FlagService.UpdateRule`. A `router` applicator dispatches by `rollout.TargetType` so one engine can drive both deploy and config rollouts. `RolloutService.AttachConfig` and `Attacher.AttachConfig` extend the attach-side plumbing. The flag handler's `updateRule` route accepts an optional `rollout: {}` field and returns 409 when an active rollout already owns the rule.

**Tech Stack:** Go 1.22+, PostgreSQL, gin, cobra. Builds on Plans 1 + 2 (strategy templates + rollout engine).

**Spec:** `docs/superpowers/specs/2026-04-18-configurable-rollout-strategies-design.md`
**Prior plans:**
- `docs/superpowers/plans/2026-04-18-rollout-strategies-foundation.md` (Plan 1)
- `docs/superpowers/plans/2026-04-18-rollout-engine-deploy.md` (Plan 2)

---

## File Structure

**New files:**

```
internal/rollout/applicator/config/
  config.go                # ConfigApplicator: rule.percentage progressive updates
  config_test.go

internal/rollout/applicator/
  router.go                # RouterApplicator: dispatches by TargetType
  router_test.go
```

**Modified files:**

```
internal/models/rollout.go                     # Add RuleID + PreviousPercentage to RolloutTargetRef
internal/rollout/rollout_repository.go         # Add GetActiveByRule method on RolloutRepository
internal/platform/database/postgres/rollouts.go # GetActiveByRule impl
internal/rollout/rollout_service.go            # AttachConfig method
internal/rollout/rollout_service_test.go       # AttachConfig tests
internal/rollout/attacher.go                   # AttachConfig + target-type routing
internal/rollout/attacher_test.go              # AttachConfig tests
internal/flags/handler.go                      # updateRule accepts rollout field + 409
internal/flags/handler_test.go                 # new test for rollout field routing
cmd/api/main.go                                # Construct config applicator + router; rewire engine
cmd/cli/flags.go                               # rule update command gains --strategy flag
docs/Rollout_Strategies.md                     # New "Config rollouts" section
docs/Current_Initiatives.md                    # Phase → Implementation (Plans 1+2+3)
```

**Target model** (cross-cutting decision for this plan):
- Config rollout `target_ref` shape: `{"rule_id": "<uuid>", "previous_percentage": <int>}`.
  - `rule_id` identifies the targeting rule whose `percentage` field is progressively updated.
  - `previous_percentage` is captured at attach time so `Revert` can restore the pre-rollout state.
- **Strategy semantics**: each step's `percent` is the *absolute* rule percentage to set. Users pick or tune a strategy whose final step reaches the desired rollout target (usually 100). Going backwards (e.g., from 25% to 10%) is currently not protected — the applicator just sets whatever the step says.
- The existing `GetActiveByConfig(flagKey, env)` method from Plan 2 is kept for future use but this plan uses `GetActiveByRule(ruleID)` for 409 enforcement, which is more specific and avoids "two rules on one flag" collisions.

---

## Task 1: Extend `RolloutTargetRef` for rule-based config targets

**Files:**
- Modify: `internal/models/rollout.go`
- Modify: `internal/models/rollout_test.go`

- [ ] **Step 1: Read current `RolloutTargetRef`**

```bash
grep -n "type RolloutTargetRef" internal/models/rollout.go
```

Expected: three fields (DeploymentID, FlagKey, Env) — all `*string`.

- [ ] **Step 2: Add a failing test**

Append to `internal/models/rollout_test.go`:

```go
func TestRolloutTargetRef_RuleJSON(t *testing.T) {
	pp := 10
	r := Rollout{TargetType: TargetTypeConfig, TargetRef: RolloutTargetRef{
		RuleID:             ptrString("11111111-1111-1111-1111-111111111111"),
		PreviousPercentage: &pp,
	}}
	b, err := json.Marshal(r.TargetRef)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"rule_id":"11111111-1111-1111-1111-111111111111","previous_percentage":10}` {
		t.Fatalf("unexpected json: %s", string(b))
	}
}
```

- [ ] **Step 3: Run, confirm fail**

```bash
go test ./internal/models/ -run TestRolloutTargetRef_RuleJSON -v
```

Expected: undefined fields.

- [ ] **Step 4: Add fields to `RolloutTargetRef`**

In `internal/models/rollout.go`, change the struct to:

```go
// RolloutTargetRef points at the specific resource a rollout is driving.
// For TargetTypeDeploy: DeploymentID is set.
// For TargetTypeConfig: RuleID is set, and PreviousPercentage captures the
// pre-rollout value so Revert can restore it.
type RolloutTargetRef struct {
	DeploymentID       *string `json:"deployment_id,omitempty"`
	FlagKey            *string `json:"flag_key,omitempty"`
	Env                *string `json:"env,omitempty"`
	RuleID             *string `json:"rule_id,omitempty"`
	PreviousPercentage *int    `json:"previous_percentage,omitempty"`
}
```

- [ ] **Step 5: Run tests, confirm pass**

```bash
go test ./internal/models/ -v
```

- [ ] **Step 6: Commit**

```bash
git add internal/models/rollout.go internal/models/rollout_test.go
git commit -m "feat(models): extend RolloutTargetRef with rule_id + previous_percentage"
```

---

## Task 2: Add `GetActiveByRule` repository method

**Files:**
- Modify: `internal/rollout/rollout_repository.go`
- Modify: `internal/platform/database/postgres/rollouts.go`
- Modify: `internal/platform/database/postgres/rollouts_test.go`

- [ ] **Step 1: Add interface method**

In `internal/rollout/rollout_repository.go`, add to the `RolloutRepository` interface (just below `GetActiveByConfig`):

```go
	GetActiveByRule(ctx context.Context, ruleID uuid.UUID) (*models.Rollout, error)
```

- [ ] **Step 2: Add failing Postgres test**

Append to `internal/platform/database/postgres/rollouts_test.go`:

```go
func TestRolloutRepo_GetActiveByRule(t *testing.T) {
	ctx := context.Background()
	repo := NewRolloutRepo(testDB(t))

	ruleIDStr := uuid.NewString()
	prevPct := 5
	ro := &models.Rollout{
		ID:         uuid.New(),
		TargetType: models.TargetTypeConfig,
		TargetRef: models.RolloutTargetRef{
			RuleID:             &ruleIDStr,
			PreviousPercentage: &prevPct,
		},
		StrategySnapshot: models.Strategy{Name: "s", TargetType: models.TargetTypeConfig,
			Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95},
		Status: models.RolloutActive,
	}
	if err := repo.Create(ctx, ro); err != nil {
		t.Fatal(err)
	}
	ruleID, _ := uuid.Parse(ruleIDStr)
	got, err := repo.GetActiveByRule(ctx, ruleID)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got.ID != ro.ID {
		t.Fatalf("expected %v got %v", ro.ID, got.ID)
	}

	// Terminal rollout should not return.
	_ = repo.UpdateStatus(ctx, ro.ID, models.RolloutSucceeded, nil)
	if _, err := repo.GetActiveByRule(ctx, ruleID); err == nil {
		t.Fatalf("expected ErrRolloutNotFound for terminal rollout")
	}
}
```

- [ ] **Step 3: Run, confirm fail**

```bash
go test ./internal/platform/database/postgres/ -run TestRolloutRepo_GetActiveByRule -v
```

Expected: `GetActiveByRule` undefined.

- [ ] **Step 4: Implement in Postgres**

Append to `internal/platform/database/postgres/rollouts.go` (alongside `GetActiveByDeployment`):

```go
// GetActiveByRule returns the active config-rollout for a targeting rule, if any.
func (r *RolloutRepo) GetActiveByRule(ctx context.Context, ruleID uuid.UUID) (*models.Rollout, error) {
	return r.scanOne(ctx,
		`WHERE target_type='config' AND target_ref->>'rule_id'=$1
         AND status IN ('pending','active','paused','awaiting_approval')`,
		ruleID.String(),
	)
}
```

- [ ] **Step 5: Add a partial index on `target_ref->>'rule_id'` — optional but useful for production**

If row counts stay small in development, the existing `idx_rollouts_config` partial index (on flag_key+env) won't serve these queries. Rather than add a migration for a new index in this plan, rely on a full scan for now — small tables, acceptable. A follow-up index can be added later. **Skip this step unless the implementer has time and wants to add a micro-migration**.

- [ ] **Step 6: Run tests, confirm pass**

```bash
go test ./internal/platform/database/postgres/ -run TestRolloutRepo_GetActiveByRule -v
```

- [ ] **Step 7: Commit**

```bash
git add internal/rollout/rollout_repository.go internal/platform/database/postgres/rollouts.go internal/platform/database/postgres/rollouts_test.go
git commit -m "feat(rollout): GetActiveByRule repository method"
```

---

## Task 3: Config applicator

**Files:**
- Create: `internal/rollout/applicator/config/config.go`
- Create: `internal/rollout/applicator/config/config_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/rollout/applicator/config/config_test.go`:

```go
package config

import (
	"context"
	"errors"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

type stubRuleUpdater struct {
	calls []int // percentages applied, in order
	err   error
}

func (s *stubRuleUpdater) UpdateRulePercentage(_ context.Context, _ uuid.UUID, pct int) error {
	s.calls = append(s.calls, pct)
	return s.err
}

func makeRolloutWithRule(ruleID string, prev int) *models.Rollout {
	p := prev
	return &models.Rollout{
		TargetType: models.TargetTypeConfig,
		TargetRef: models.RolloutTargetRef{
			RuleID:             &ruleID,
			PreviousPercentage: &p,
		},
	}
}

func TestConfigApplicator_Apply_SetsPercentage(t *testing.T) {
	u := &stubRuleUpdater{}
	app := NewApplicator(u)
	ro := makeRolloutWithRule(uuid.NewString(), 0)
	if err := app.Apply(context.Background(), ro, models.Step{Percent: 25}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(u.calls) != 1 || u.calls[0] != 25 {
		t.Fatalf("expected [25], got %v", u.calls)
	}
}

func TestConfigApplicator_Apply_MissingRuleID(t *testing.T) {
	app := NewApplicator(&stubRuleUpdater{})
	ro := &models.Rollout{TargetType: models.TargetTypeConfig}
	if err := app.Apply(context.Background(), ro, models.Step{Percent: 10}); err == nil {
		t.Fatalf("expected error on missing rule_id")
	}
}

func TestConfigApplicator_Revert_RestoresPrevious(t *testing.T) {
	u := &stubRuleUpdater{}
	app := NewApplicator(u)
	ro := makeRolloutWithRule(uuid.NewString(), 7)
	if err := app.Revert(context.Background(), ro); err != nil {
		t.Fatal(err)
	}
	if len(u.calls) != 1 || u.calls[0] != 7 {
		t.Fatalf("expected revert to 7, got %v", u.calls)
	}
}

func TestConfigApplicator_Revert_DefaultsToZero(t *testing.T) {
	u := &stubRuleUpdater{}
	app := NewApplicator(u)
	// No PreviousPercentage set → Revert should set to 0.
	ro := &models.Rollout{TargetType: models.TargetTypeConfig, TargetRef: models.RolloutTargetRef{RuleID: func() *string { s := uuid.NewString(); return &s }()}}
	if err := app.Revert(context.Background(), ro); err != nil {
		t.Fatal(err)
	}
	if len(u.calls) != 1 || u.calls[0] != 0 {
		t.Fatalf("expected revert to 0, got %v", u.calls)
	}
}

func TestConfigApplicator_CurrentSignal_AlwaysHealthy(t *testing.T) {
	app := NewApplicator(&stubRuleUpdater{})
	ro := makeRolloutWithRule(uuid.NewString(), 0)
	s, err := app.CurrentSignal(context.Background(), ro, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Score != 1.0 {
		t.Fatalf("expected healthy noop signal (1.0), got %g", s.Score)
	}
}

func TestConfigApplicator_Apply_ErrorPropagates(t *testing.T) {
	u := &stubRuleUpdater{err: errors.New("db fail")}
	app := NewApplicator(u)
	ro := makeRolloutWithRule(uuid.NewString(), 0)
	if err := app.Apply(context.Background(), ro, models.Step{Percent: 50}); err == nil {
		t.Fatalf("expected error propagation")
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/applicator/config/ -v
```

Expected: package does not exist.

- [ ] **Step 3: Implement**

Create `internal/rollout/applicator/config/config.go`:

```go
// Package config implements the config-target Applicator: bridges rollout phase
// progression to flag targeting-rule percentage updates.
package config

import (
	"context"
	"errors"
	"math"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/rollout/applicator"
	"github.com/google/uuid"
)

// RuleUpdater is the subset of the flag service the applicator depends on.
// Implemented by a small adapter in cmd/api wiring that calls
// flags.FlagService.UpdateRule with the new percentage.
type RuleUpdater interface {
	UpdateRulePercentage(ctx context.Context, ruleID uuid.UUID, percentage int) error
}

// Applicator implements applicator.Applicator for config targets.
type Applicator struct {
	updater RuleUpdater
}

// NewApplicator builds a config applicator.
func NewApplicator(u RuleUpdater) *Applicator { return &Applicator{updater: u} }

var _ applicator.Applicator = (*Applicator)(nil)

// ErrMissingRuleID is returned when a config rollout lacks a rule_id.
var ErrMissingRuleID = errors.New("config rollout missing rule_id in target_ref")

func (a *Applicator) ruleID(ro *models.Rollout) (uuid.UUID, error) {
	if ro.TargetRef.RuleID == nil {
		return uuid.Nil, ErrMissingRuleID
	}
	return uuid.Parse(*ro.TargetRef.RuleID)
}

// Apply sets the targeting rule's percentage to step.Percent (rounded to int 0-100).
func (a *Applicator) Apply(ctx context.Context, ro *models.Rollout, step models.Step) error {
	ruleID, err := a.ruleID(ro)
	if err != nil {
		return err
	}
	pct := int(math.Round(step.Percent))
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return a.updater.UpdateRulePercentage(ctx, ruleID, pct)
}

// Revert restores the rule's previous percentage (from target_ref.PreviousPercentage)
// or sets to 0 if not captured.
func (a *Applicator) Revert(ctx context.Context, ro *models.Rollout) error {
	ruleID, err := a.ruleID(ro)
	if err != nil {
		return err
	}
	pct := 0
	if ro.TargetRef.PreviousPercentage != nil {
		pct = *ro.TargetRef.PreviousPercentage
	}
	return a.updater.UpdateRulePercentage(ctx, ruleID, pct)
}

// CurrentSignal returns a constant healthy signal — config rollouts advance on
// time alone in Plan 3. Future work may wire an app+env health reader here.
func (a *Applicator) CurrentSignal(_ context.Context, _ *models.Rollout, _ *models.SignalSource) (applicator.HealthScore, error) {
	return applicator.HealthScore{Score: 1.0}, nil
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/rollout/applicator/config/ -v
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/applicator/config/
git commit -m "feat(rollout): config applicator for targeting rule percentage rollouts"
```

---

## Task 4: Router applicator

**Files:**
- Create: `internal/rollout/applicator/router.go`
- Create: `internal/rollout/applicator/router_test.go`

The router wraps two inner applicators and dispatches by `rollout.TargetType`, so one `engine.Engine` instance can drive both deploy and config rollouts.

- [ ] **Step 1: Write failing tests**

Create `internal/rollout/applicator/router_test.go`:

```go
package applicator

import (
	"context"
	"errors"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
)

type stubInner struct {
	name   string
	calls  int
	err    error
	signal HealthScore
}

func (s *stubInner) Apply(_ context.Context, _ *models.Rollout, _ models.Step) error {
	s.calls++
	return s.err
}
func (s *stubInner) Revert(_ context.Context, _ *models.Rollout) error { s.calls++; return s.err }
func (s *stubInner) CurrentSignal(_ context.Context, _ *models.Rollout, _ *models.SignalSource) (HealthScore, error) {
	return s.signal, s.err
}

func TestRouter_Apply_DispatchesByTargetType(t *testing.T) {
	deployInner := &stubInner{name: "deploy"}
	configInner := &stubInner{name: "config"}
	r := NewRouter(deployInner, configInner)

	dep := &models.Rollout{TargetType: models.TargetTypeDeploy}
	cfg := &models.Rollout{TargetType: models.TargetTypeConfig}
	if err := r.Apply(context.Background(), dep, models.Step{Percent: 50}); err != nil {
		t.Fatal(err)
	}
	if err := r.Apply(context.Background(), cfg, models.Step{Percent: 75}); err != nil {
		t.Fatal(err)
	}
	if deployInner.calls != 1 {
		t.Fatalf("deploy called %d", deployInner.calls)
	}
	if configInner.calls != 1 {
		t.Fatalf("config called %d", configInner.calls)
	}
}

func TestRouter_UnknownTargetType_Errors(t *testing.T) {
	r := NewRouter(&stubInner{}, &stubInner{})
	ro := &models.Rollout{TargetType: models.TargetType("mystery")}
	if err := r.Apply(context.Background(), ro, models.Step{}); err == nil {
		t.Fatalf("expected error for unknown target_type")
	}
}

func TestRouter_NilInner_Errors(t *testing.T) {
	r := NewRouter(nil, &stubInner{})
	ro := &models.Rollout{TargetType: models.TargetTypeDeploy}
	if err := r.Apply(context.Background(), ro, models.Step{}); err == nil {
		t.Fatalf("expected error when deploy applicator not provided")
	}
}

func TestRouter_CurrentSignal_Dispatches(t *testing.T) {
	deployInner := &stubInner{signal: HealthScore{Score: 0.8}}
	configInner := &stubInner{signal: HealthScore{Score: 0.9}}
	r := NewRouter(deployInner, configInner)

	dep := &models.Rollout{TargetType: models.TargetTypeDeploy}
	cfg := &models.Rollout{TargetType: models.TargetTypeConfig}
	ds, _ := r.CurrentSignal(context.Background(), dep, nil)
	cs, _ := r.CurrentSignal(context.Background(), cfg, nil)
	if ds.Score != 0.8 || cs.Score != 0.9 {
		t.Fatalf("mis-dispatched signals: deploy=%g config=%g", ds.Score, cs.Score)
	}
}

func TestRouter_Errors_FromInner(t *testing.T) {
	deployInner := &stubInner{err: errors.New("x")}
	r := NewRouter(deployInner, &stubInner{})
	if err := r.Apply(context.Background(), &models.Rollout{TargetType: models.TargetTypeDeploy}, models.Step{}); err == nil {
		t.Fatalf("expected inner error to propagate")
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/applicator/ -v
```

- [ ] **Step 3: Implement**

Create `internal/rollout/applicator/router.go`:

```go
package applicator

import (
	"context"
	"errors"
	"fmt"

	"github.com/deploysentry/deploysentry/internal/models"
)

// Router dispatches Applicator calls to an inner applicator chosen by
// rollout.TargetType. Used by cmd/api wiring so one engine can drive deploy
// and config rollouts from the same goroutine.
type Router struct {
	deploy Applicator
	config Applicator
}

// NewRouter builds a Router. Either inner may be nil at construction time, but
// calls with a matching TargetType on a nil inner will return ErrNoApplicator.
func NewRouter(deploy, config Applicator) *Router {
	return &Router{deploy: deploy, config: config}
}

var _ Applicator = (*Router)(nil)

// ErrNoApplicator is returned when no inner applicator is registered for the
// rollout's TargetType.
var ErrNoApplicator = errors.New("no applicator registered for target_type")

func (r *Router) pick(ro *models.Rollout) (Applicator, error) {
	switch ro.TargetType {
	case models.TargetTypeDeploy:
		if r.deploy == nil {
			return nil, fmt.Errorf("%w: deploy", ErrNoApplicator)
		}
		return r.deploy, nil
	case models.TargetTypeConfig:
		if r.config == nil {
			return nil, fmt.Errorf("%w: config", ErrNoApplicator)
		}
		return r.config, nil
	}
	return nil, fmt.Errorf("%w: %q", ErrNoApplicator, string(ro.TargetType))
}

// Apply dispatches to the target-type applicator.
func (r *Router) Apply(ctx context.Context, ro *models.Rollout, step models.Step) error {
	a, err := r.pick(ro)
	if err != nil {
		return err
	}
	return a.Apply(ctx, ro, step)
}

// Revert dispatches to the target-type applicator.
func (r *Router) Revert(ctx context.Context, ro *models.Rollout) error {
	a, err := r.pick(ro)
	if err != nil {
		return err
	}
	return a.Revert(ctx, ro)
}

// CurrentSignal dispatches to the target-type applicator.
func (r *Router) CurrentSignal(ctx context.Context, ro *models.Rollout, override *models.SignalSource) (HealthScore, error) {
	a, err := r.pick(ro)
	if err != nil {
		return HealthScore{}, err
	}
	return a.CurrentSignal(ctx, ro, override)
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/rollout/applicator/ -v
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/applicator/router.go internal/rollout/applicator/router_test.go
git commit -m "feat(rollout): router applicator dispatching by target_type"
```

---

## Task 5: `RolloutService.AttachConfig`

**Files:**
- Modify: `internal/rollout/rollout_service.go`
- Modify: `internal/rollout/rollout_service_test.go`

- [ ] **Step 1: Add failing test**

Append to `internal/rollout/rollout_service_test.go`:

```go
func TestRolloutService_AttachConfig_CreatesRollout(t *testing.T) {
	svc := newTestService()
	snap := &models.Strategy{Name: "c", TargetType: models.TargetTypeConfig, Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	ruleID := uuid.New()
	ro, err := svc.AttachConfig(context.Background(), ruleID, 15, snap, nil, nil)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if ro.TargetType != models.TargetTypeConfig || ro.TargetRef.RuleID == nil || *ro.TargetRef.RuleID != ruleID.String() {
		t.Fatalf("wrong rollout: %+v", ro)
	}
	if ro.TargetRef.PreviousPercentage == nil || *ro.TargetRef.PreviousPercentage != 15 {
		t.Fatalf("expected previous_percentage=15, got %+v", ro.TargetRef.PreviousPercentage)
	}
	if ro.Status != models.RolloutPending {
		t.Fatalf("expected pending, got %s", ro.Status)
	}
}

func TestRolloutService_AttachConfig_RejectsIfRuleActive(t *testing.T) {
	svc := newTestService()
	snap := &models.Strategy{Name: "c", TargetType: models.TargetTypeConfig, Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	ruleID := uuid.New()
	if _, err := svc.AttachConfig(context.Background(), ruleID, 0, snap, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AttachConfig(context.Background(), ruleID, 0, snap, nil, nil); err == nil {
		t.Fatalf("expected ErrAlreadyActiveOnTarget on second attach")
	}
}
```

(Also add `GetActiveByRule` to `fakeRolloutRepo` in this test file if not already present — it'll be needed. Mirror the existing `GetActiveByDeployment` stub.)

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/ -run TestRolloutService_AttachConfig -v
```

- [ ] **Step 3: Extend the fake repo**

In `rollout_service_test.go`, add to `fakeRolloutRepo`:

```go
func (f *fakeRolloutRepo) GetActiveByRule(_ context.Context, ruleID uuid.UUID) (*models.Rollout, error) {
	for _, r := range f.rows {
		if r.TargetRef.RuleID != nil && *r.TargetRef.RuleID == ruleID.String() && !r.IsTerminal() {
			return r, nil
		}
	}
	return nil, errors.New("not found")
}
```

- [ ] **Step 4: Implement `AttachConfig`**

In `internal/rollout/rollout_service.go`, add:

```go
// AttachConfig creates a pending Rollout for a targeting rule. It captures the
// current (pre-rollout) percentage so Revert can restore it. Returns
// ErrAlreadyActiveOnTarget if a rollout already owns this rule.
func (s *RolloutService) AttachConfig(ctx context.Context, ruleID uuid.UUID, previousPct int, snapshot *models.Strategy, releaseID *uuid.UUID, createdBy *uuid.UUID) (*models.Rollout, error) {
	if existing, _ := s.rollouts.GetActiveByRule(ctx, ruleID); existing != nil {
		return existing, ErrAlreadyActiveOnTarget
	}
	ref := ruleID.String()
	prev := previousPct
	ro := &models.Rollout{
		ReleaseID:        releaseID,
		TargetType:       models.TargetTypeConfig,
		TargetRef:        models.RolloutTargetRef{RuleID: &ref, PreviousPercentage: &prev},
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

// GetActiveByRule is a pass-through for handler 409 checks on config rollouts.
func (s *RolloutService) GetActiveByRule(ctx context.Context, ruleID uuid.UUID) (*models.Rollout, error) {
	return s.rollouts.GetActiveByRule(ctx, ruleID)
}
```

- [ ] **Step 5: Run tests, confirm pass**

```bash
go test ./internal/rollout/ -v
```

- [ ] **Step 6: Commit**

```bash
git add internal/rollout/rollout_service.go internal/rollout/rollout_service_test.go
git commit -m "feat(rollout): RolloutService.AttachConfig + GetActiveByRule"
```

---

## Task 6: Attacher — `AttachConfig`

**Files:**
- Modify: `internal/rollout/attacher.go`
- Modify: `internal/rollout/attacher_test.go`

- [ ] **Step 1: Add failing test**

Append to `internal/rollout/attacher_test.go`:

```go
func TestAttacher_AttachConfig_ExplicitStrategyName(t *testing.T) {
	stratRepo := newFakeStratRepo()
	orgID := uuid.New()
	tmpl := &models.Strategy{
		ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "slow-roll",
		TargetType: models.TargetTypeConfig,
		Steps:      []models.Step{{Percent: 100}},
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

	ruleID := uuid.New()
	projID := uuid.New()
	appID := uuid.New()
	err := attacher.AttachConfig(context.Background(), ruleID, 5, &AttachIntent{
		StrategyName: "slow-roll",
		Leaf:         ScopeRef{Type: models.ScopeApp, ID: appID},
		ProjectID:    &projID,
		OrgID:        &orgID,
	}, uuid.New())
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if len(rolloutRepo.rows) != 1 {
		t.Fatalf("expected 1 rollout created, got %d", len(rolloutRepo.rows))
	}
	for _, r := range rolloutRepo.rows {
		if r.TargetType != models.TargetTypeConfig {
			t.Fatalf("expected config rollout, got %s", r.TargetType)
		}
		if r.TargetRef.PreviousPercentage == nil || *r.TargetRef.PreviousPercentage != 5 {
			t.Fatalf("expected previous_percentage=5")
		}
	}
}

func TestAttacher_AttachConfig_MandateWithoutStrategy_Errors(t *testing.T) {
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

	ruleID := uuid.New()
	projID := uuid.New()
	appID := uuid.New()
	if err := attacher.AttachConfig(context.Background(), ruleID, 0, &AttachIntent{
		Leaf:      ScopeRef{Type: models.ScopeApp, ID: appID},
		ProjectID: &projID,
		OrgID:     &orgID,
	}, uuid.New()); err == nil {
		t.Fatalf("expected mandate error")
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/ -run TestAttacher_AttachConfig -v
```

- [ ] **Step 3: Implement `AttachConfig`**

In `internal/rollout/attacher.go`, add after `AttachDeploy`:

```go
// AttachConfig attaches a rollout to a targeting rule. previousPercentage is
// captured so Revert can restore the pre-rollout value.
func (a *Attacher) AttachConfig(ctx context.Context, ruleID uuid.UUID, previousPercentage int, intent *AttachIntent, actor uuid.UUID) error {
	target := models.TargetTypeConfig

	// Resolve explicit strategy first.
	var tmpl *models.Strategy
	if intent.StrategyID != nil {
		got, err := a.strategies.Get(ctx, *intent.StrategyID)
		if err != nil {
			return fmt.Errorf("strategy not found by id: %w", err)
		}
		tmpl = got
	} else if intent.StrategyName != "" {
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

	if tmpl == nil {
		def, err := a.defaults.Resolve(ctx, intent.Leaf, intent.ProjectID, intent.OrgID, intent.Environment, &target)
		if err == nil && def != nil {
			got, err := a.strategies.Get(ctx, def.StrategyID)
			if err == nil {
				tmpl = got
			}
		}
	}

	policy, _ := a.policies.Resolve(ctx, intent.Leaf, intent.ProjectID, intent.OrgID, intent.Environment, &target)
	if policy != nil && policy.Enabled && policy.Policy == models.PolicyMandate && tmpl == nil {
		return ErrMandateWithoutStrategy
	}
	if tmpl == nil {
		return nil // no mandate, no strategy → fall through to immediate apply
	}

	var overrides *StrategyOverrides
	if len(intent.Overrides) > 0 {
		var o StrategyOverrides
		if err := json.Unmarshal(intent.Overrides, &o); err != nil {
			return fmt.Errorf("overrides invalid: %w", err)
		}
		overrides = &o
	}
	snap := BuildSnapshot(tmpl, overrides)

	_, err := a.rollouts.AttachConfig(ctx, ruleID, previousPercentage, snap, intent.ReleaseID, &actor)
	return err
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/rollout/ -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/attacher.go internal/rollout/attacher_test.go
git commit -m "feat(rollout): Attacher.AttachConfig for targeting rule rollouts"
```

---

## Task 7: Flag handler — accept `rollout: {}` on PUT rule + 409

**Files:**
- Modify: `internal/flags/handler.go`
- Modify: `internal/flags/handler_test.go`

- [ ] **Step 1: Read current handler**

```bash
grep -n "updateRule\|updateRuleReq\|type.*Request" internal/flags/handler.go | head -10
```

Identify the existing request DTO (likely `updateRuleRequest` or similar inline anonymous struct).

- [ ] **Step 2: Add the rollout attacher interface + types**

Near the top of `internal/flags/handler.go`, add:

```go
// RolloutAttacher is implemented by the rollout package and injected at wiring time.
// A nil attacher means rollouts are not enabled; rule edits apply immediately.
type RolloutAttacher interface {
	AttachFromRuleRequest(ctx context.Context, rule *models.TargetingRule, previousPercentage int, req *RolloutAttachRequest, actor uuid.UUID) error
}

// RolloutAttachRequest is the client-supplied rollout intent on a rule edit.
type RolloutAttachRequest struct {
	StrategyName     string          `json:"strategy_name,omitempty"`
	StrategyID       *uuid.UUID      `json:"strategy_id,omitempty"`
	Overrides        json.RawMessage `json:"overrides,omitempty"`
	ReleaseID        *uuid.UUID      `json:"release_id,omitempty"`
	ApplyImmediately bool            `json:"apply_immediately,omitempty"`
}

// ErrRolloutInProgress is returned by the attacher when the target rule already
// has an active rollout (client should abort/wait/amend).
var ErrRolloutInProgress = errors.New("rollout in progress on this rule")
```

Add `rollouts RolloutAttacher` field to the `Handler` struct. Add an additive constructor `NewHandlerWithRollouts(existing args..., rollouts RolloutAttacher) *Handler` that delegates to `NewHandler` then sets the field.

- [ ] **Step 3: Extend the rule-update request DTO**

Locate the existing update-rule request DTO (likely near `updateRule` method). Add a `Rollout *RolloutAttachRequest json:"rollout,omitempty"` field.

- [ ] **Step 4: Thread the attach call in `updateRule`**

Before the `h.service.UpdateRule(...)` call, check if a rollout is requested. If yes, look up the current rule to capture `previousPercentage`, then delegate to the attacher (do NOT call UpdateRule directly — the rollout engine will).

Rough shape (adapt to real code):

```go
// Load the current rule to capture its current percentage for Revert.
currentRule, err := h.service.GetRule(c.Request.Context(), ruleID)
if err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
    return
}

if h.rollouts != nil && req.Rollout != nil && !req.Rollout.ApplyImmediately {
    prev := 0
    if currentRule.Percentage != nil {
        prev = *currentRule.Percentage
    }
    // Note: the rule in `rule` (built above) carries the new target percentage
    // the user wants to reach. We pass that through AttachRequest and the
    // attacher creates a rollout whose strategy_snapshot's final step matches it.
    actor := actorFromFlagContext(c)
    if err := h.rollouts.AttachFromRuleRequest(c.Request.Context(), rule, prev, req.Rollout, actor); err != nil {
        if errors.Is(err, ErrRolloutInProgress) {
            c.JSON(http.StatusConflict, gin.H{
                "error":   "rollout_in_progress",
                "options": []string{"abort", "wait", "amend"},
            })
            return
        }
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusAccepted, rule) // 202 — rollout created, rule not yet updated
    return
}

// No rollout intent → fall through to existing immediate-apply path.
```

`actorFromFlagContext`: if not already present, add the same helper pattern the deploy package uses (reads `user_id` from gin context).

- [ ] **Step 5: Add a handler test**

Add to `internal/flags/handler_test.go`:

```go
type fakeRolloutAttacher struct {
	called bool
	lastPrev int
}

func (f *fakeRolloutAttacher) AttachFromRuleRequest(_ context.Context, _ *models.TargetingRule, prev int, _ *flags.RolloutAttachRequest, _ uuid.UUID) error {
	f.called = true
	f.lastPrev = prev
	return nil
}

func TestUpdateRule_WithRolloutField_AttacherCalled(t *testing.T) {
	// Construct the handler using whatever setup helper the existing tests use.
	// Attach a fakeRolloutAttacher.
	// POST (actually PUT) to /flags/:id/rules/:ruleId with body including "rollout":{"strategy_name":"x"}.
	// Assert 202 (not 200) and fakeRolloutAttacher.called == true.
	// Match the existing test pattern. If the existing fake rule repo needs GetRule, add it.
}
```

If the existing test infrastructure is too complex, a minimal test that confirms:
- Calls into `AttachFromRuleRequest` when `rollout: {}` is present
- Returns 202
- Does NOT call `UpdateRule` on the service (check the service mock)
is sufficient.

- [ ] **Step 6: Run tests, confirm pass**

```bash
go test ./internal/flags/ -run TestUpdateRule -v -count=1
```

- [ ] **Step 7: Commit**

```bash
git add internal/flags/handler.go internal/flags/handler_test.go
git commit -m "feat(flags): accept rollout field on PUT rule + 409 on active rollout"
```

---

## Task 8: Config attacher adapter + wire everything in cmd/api

**Files:**
- Modify: `cmd/api/main.go`

The deploy-side already has a `deployRolloutAttacherAdapter` (Plan 2 Task 14). For config, add `flagRolloutAttacherAdapter` that satisfies `flags.RolloutAttacher`.

The engine needs a Router applicator instead of the current deploy-only applicator.

A `RuleUpdater` adapter bridges `rollout/applicator/config` to `flags.FlagService.UpdateRule`.

- [ ] **Step 1: Add the RuleUpdater adapter at the bottom of main.go**

```go
// flagRuleUpdater adapts flags.FlagService to applicatorconfig.RuleUpdater.
type flagRuleUpdater struct{ svc flags.FlagService }

func (u *flagRuleUpdater) UpdateRulePercentage(ctx context.Context, ruleID uuid.UUID, pct int) error {
	rule, err := u.svc.GetRule(ctx, ruleID)
	if err != nil {
		return err
	}
	p := pct
	rule.Percentage = &p
	return u.svc.UpdateRule(ctx, rule)
}
```

(Add any needed import aliases — e.g., `applicatorconfig "github.com/deploysentry/deploysentry/internal/rollout/applicator/config"`.)

- [ ] **Step 2: Add the flag attacher adapter**

```go
// flagRolloutAttacherAdapter satisfies flags.RolloutAttacher by delegating to
// rollout.Attacher.AttachConfig with scope context drawn from the rule's flag.
type flagRolloutAttacherAdapter struct {
	attacher *rollout.Attacher
	flagSvc  flags.FlagService
	entities entities.EntityRepository
}

func (a *flagRolloutAttacherAdapter) AttachFromRuleRequest(ctx context.Context, rule *models.TargetingRule, prev int, req *flags.RolloutAttachRequest, actor uuid.UUID) error {
	// Resolve scope ancestors by walking: rule → flag → application → project → org.
	flag, err := a.flagSvc.GetFlag(ctx, rule.FlagID)
	if err != nil {
		return err
	}
	var orgID, projID *uuid.UUID
	var envName *string
	var leaf rollout.ScopeRef
	if flag.ApplicationID != nil {
		appID := *flag.ApplicationID
		leaf = rollout.ScopeRef{Type: models.ScopeApp, ID: appID}
		if app, _ := a.entities.GetApplication(ctx, appID); app != nil {
			projID = &app.ProjectID
			if proj, _ := a.entities.GetProject(ctx, app.ProjectID); proj != nil {
				orgID = &proj.OrganizationID
			}
		}
	} else {
		leaf = rollout.ScopeRef{Type: models.ScopeProject, ID: flag.ProjectID}
		if proj, _ := a.entities.GetProject(ctx, flag.ProjectID); proj != nil {
			orgID = &proj.OrganizationID
		}
	}
	if flag.EnvironmentID != nil {
		if env, _ := a.entities.GetEnvironment(ctx, *flag.EnvironmentID); env != nil {
			name := env.Name // or env.Slug — match whichever the existing adapter uses
			envName = &name
		}
	}

	intent := &rollout.AttachIntent{
		StrategyID:   req.StrategyID,
		StrategyName: req.StrategyName,
		Overrides:    req.Overrides,
		ReleaseID:    req.ReleaseID,
		Leaf:         leaf,
		ProjectID:    projID,
		OrgID:        orgID,
		Environment:  envName,
	}
	return a.attacher.AttachConfig(ctx, rule.ID, prev, intent, actor)
}
```

- [ ] **Step 3: Rewire engine with router applicator**

Find the existing block wiring the rollout engine (from Plan 2 Task 14):

```go
deployApp := applicatordeploy.NewApplicator(trafficSetter, &noopHealthReader{})
// ...engine := rolloutengine.New(engineRepos, deployApp, natsPublisher, ...)
```

Replace the applicator construction with:

```go
deployApp := applicatordeploy.NewApplicator(trafficSetter, &noopHealthReader{})
configApp := applicatorconfig.NewApplicator(&flagRuleUpdater{svc: flagService})
routerApp := applicator.NewRouter(deployApp, configApp)
rolloutExecEngine := rolloutengine.New(engineRepos, routerApp, natsPublisher, rolloutengine.EngineOptions{
    PollInterval: 2 * time.Second,
    Logger:       logger,
})
```

Add imports as needed:

```go
"github.com/deploysentry/deploysentry/internal/rollout/applicator"
applicatorconfig "github.com/deploysentry/deploysentry/internal/rollout/applicator/config"
```

- [ ] **Step 4: Rewire flag handler with attacher**

Find the existing `flags.NewHandler(...)` call and replace with `flags.NewHandlerWithRollouts(existing args, flagRolloutAttacher)`:

```go
flagRolloutAttacher := &flagRolloutAttacherAdapter{
    attacher: rolloutAttacher, // already constructed for Plan 2 Task 14
    flagSvc:  flagService,
    entities: entityRepo,
}
flagHandler := flags.NewHandlerWithRollouts(flagService, rbacChecker, webhookService, analyticsService, entityRepo, envRepo, auditRepo, flagRolloutAttacher)
flagHandler.SetRatingService(ratingService)
flagHandler.RegisterRoutes(api)
flagHandler.RegisterSegmentRoutes(api)
```

- [ ] **Step 5: Build and verify**

```bash
go build ./cmd/api
./api 2>&1 | head -500 | grep -E "ROUTE:.*/rules"
```

Expected: existing rule routes print; no new routes (Plan 3 doesn't add endpoints, only extends behavior).

- [ ] **Step 6: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat(api): wire config applicator, router, and flag rollout attacher"
```

---

## Task 9: Docs + initiative update

**Files:**
- Modify: `docs/Rollout_Strategies.md`
- Modify: `docs/Current_Initiatives.md`

- [ ] **Step 1: Append "Config rollouts" section**

Append to `docs/Rollout_Strategies.md`:

```markdown
## Config rollouts

Flag and config changes can be rolled out progressively through the same strategy system. A config rollout targets a specific targeting rule — the engine advances the rule's `percentage` field through the strategy's phases.

### Attach via API

```
PUT /api/v1/flags/:id/rules/:ruleId
Content-Type: application/json

{
  "rule_type": "percentage",
  "value": "true",
  "percentage": 100,
  "rollout": { "strategy_name": "slow-canary" }
}
```

When the request body includes a `rollout` block, the server returns **202 Accepted** and does NOT apply the rule change immediately. Instead, a Rollout is created and the engine drives the rule's percentage through the strategy's phases.

### 409 on active rollout

If a rule already has an active rollout, the rule-edit request returns 409:

```json
{ "error": "rollout_in_progress", "options": ["abort", "wait", "amend"] }
```

The client should either wait, call `POST /rollouts/:id/rollback` to abort the existing one, or (future) `amend` its target value.

### CLI

```
ds flags update <flag-key> --rule-id <id> --percentage 100 --strategy slow-canary
```

### Current limitations (Plan 3)

- Config rollouts use a no-op health reader and advance on time only — same behavior as deploy rollouts when HealthMonitor is not wired.
- A rule whose `percentage` decreases during a rollout (e.g., from 25% → 1% as the first step) is allowed; the engine treats the strategy's step values as absolute. Users should pick strategies whose first step matches or exceeds the current percentage.
- Only the `percentage` field of a rule is rolled out. Changing the rule's `Value` or type requires a direct edit (no progressive rollout of `Value` yet).
```

- [ ] **Step 2: Update initiatives**

In `docs/Current_Initiatives.md`, update the Configurable Rollout Strategies row:
- Phase: `Implementation (Plans 1+2+3)`
- Add Plan 3 link to the plan/spec column
- Notes: `Plans 1 (templates) + 2 (engine + deploy) + 3 (config rollouts) complete. Plans 4–5 (releases, UI) to follow.`

Bump `> Last updated:` to today's date.

- [ ] **Step 3: Run tests**

```bash
go build ./...
go test ./internal/rollout/... ./internal/models/ ./internal/platform/database/postgres/ ./internal/flags/ -count=1
```

All should pass (ignore pre-existing `TestRollback*` failures in `internal/deploy/` if they still exist).

- [ ] **Step 4: Commit**

```bash
git add docs/Rollout_Strategies.md docs/Current_Initiatives.md
git commit -m "docs: Plan 3 config rollouts section + initiative update"
```

---

## Definition of Done

- All 9 tasks committed individually on branch `feature/rollout-config-integration`.
- `go build ./...` clean.
- Tests pass: rollout + models + postgres + flags packages green.
- `PUT /flags/:id/rules/:ruleId` with `rollout: {strategy_name: "..."}` returns 202 and creates a Rollout that drives the rule's percentage.
- Same endpoint without `rollout: {}` keeps today's immediate-apply behavior (verified by existing flag handler tests continuing to pass).
- 409 returned when a rule already has an active rollout.
- `docs/Rollout_Strategies.md` has the Config rollouts section.

## Not in scope (deferred)

- Rollout percentage going backward protection (preventing e.g. 25% → 5% mid-rollout).
- Rolling out changes to a rule's `Value` (only `percentage` supported in Plan 3).
- Rolling out flag-level state (e.g., `FlagEnvironmentState.Enabled` or `Value`) — needs separate design.
- Health signals for config rollouts (currently no-op; rollouts advance on time only).
- Releases + coordination (Plan 4).
- Web UI (Plan 5).

## Self-Review Notes

- **Spec coverage**: covers spec section "Config Rollouts" at the level specified — rule-based percentage progressive advancement, 409 on collision, policy enforcement for config target type, CLI hook, docs.
- **Type consistency**: `RolloutTargetRef.RuleID` added Task 1, used consistently in Tasks 2, 3, 5. `RuleUpdater` interface from Task 3 is consumed by Task 8's adapter. `RolloutAttacher` in Task 7 has `AttachFromRuleRequest` signature used by the Task 8 adapter.
- **Placeholder scan**: Task 7 Step 5 contains a "minimal test that confirms" description rather than complete test code — this is intentional because the exact handler test infrastructure in `internal/flags/handler_test.go` is large and mocking it verbatim would duplicate the plan unreasonably. The implementer is expected to match the existing mock patterns in that file. If this triggers a placeholder policy issue, the implementer should expand the test following the existing pattern for `TestUpdateRule`-style tests.
