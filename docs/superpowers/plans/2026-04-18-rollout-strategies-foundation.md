# Rollout Strategies Foundation & Templates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the schema and management surface for rollout strategy templates, defaults, and onboarding policies. After this plan, users can author/manage reusable strategies via API/CLI/YAML, pin per-scope defaults, and declare rollout-policy onboarding — but nothing consumes them yet (that's Plan 2).

**Architecture:** Three new tables in the `deploy` schema (`strategies`, `strategy_defaults`, `rollout_policies`). A new `internal/rollout/` Go package holding models/validation/repository/service/handler with a Postgres implementation under `internal/platform/database/postgres/rollout.go`. Scope inheritance resolution mirrors the existing `internal/settings/` pattern: walk app → project → org, most-specific wins. YAML import/export uses `gopkg.in/yaml.v3` (already a project dep). CLI additions use cobra per existing `cmd/cli/` conventions.

**Tech Stack:** Go 1.22+, PostgreSQL (schema: `deploy`), gin, cobra, yaml.v3, golang-migrate. Tests follow existing project style: unit tests alongside code, integration tests hit the real database per `CLAUDE.md` conventions (run via `make test`).

**Spec:** `docs/superpowers/specs/2026-04-18-configurable-rollout-strategies-design.md`

---

## File Structure

**New files:**

```
migrations/
  049_create_rollout_control_plane.up.sql
  049_create_rollout_control_plane.down.sql

internal/models/
  strategy.go             # Strategy, Step, StepApproval, StepNotify, StepAbortCondition, SignalSource
  strategy_default.go     # StrategyDefault
  rollout_policy.go       # RolloutPolicy

internal/rollout/
  validation.go           # ValidateStrategy, parseDuration, range checks
  validation_test.go
  repository.go           # StrategyRepository, StrategyDefaultRepository, RolloutPolicyRepository interfaces
  service.go              # StrategyService, StrategyDefaultService, RolloutPolicyService (all three resolvers)
  service_test.go
  resolver.go             # Inheritance walk helpers shared by all three services
  resolver_test.go
  yaml.go                 # YAML <-> Strategy conversion
  yaml_test.go
  handler.go              # HTTP handlers for strategies, strategy-defaults, rollout-policy
  handler_test.go

internal/platform/database/postgres/
  rollout.go              # Postgres implementations of the three repositories
  rollout_test.go         # Integration tests (real DB)

cmd/cli/
  strategies.go           # ds strategies {list,get,apply,delete,import,export}
  rolloutpolicy.go        # ds rollout-policy and ds strategy-defaults

docs/
  Rollout_Strategies.md   # User-facing reference
```

**Modified files:**

```
cmd/api/main.go           # Wire rollout.NewHandler(...).RegisterRoutes(api)
cmd/cli/root.go           # Register new cobra commands (strategies, rolloutpolicy, strategy-defaults)
docs/Traffic_Management_Guide.md    # Cross-reference Rollout_Strategies.md
docs/CURRENT_INITIATIVES.md          # Add initiative row (Design → Implementation)
```

**Namespace convention:**
- Go package: `internal/rollout` (sub-packages `engine`, `applicator` come in Plan 2)
- DB tables: `strategies`, `strategy_defaults`, `rollout_policies` (deploy schema)
- Route prefix: `/api/v1/orgs/:orgSlug`, `/api/v1/orgs/:orgSlug/projects/:projectSlug`, `/api/v1/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug`

---

## Task 1: Migration — rollout control plane schema

**Files:**
- Create: `migrations/049_create_rollout_control_plane.up.sql`
- Create: `migrations/049_create_rollout_control_plane.down.sql`

- [ ] **Step 1: Write the up migration**

Create `migrations/049_create_rollout_control_plane.up.sql`:

```sql
-- Strategies: reusable rollout templates scoped to org, project, or app.
CREATE TABLE strategies (
    id                              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_type                      TEXT NOT NULL CHECK (scope_type IN ('org','project','app')),
    scope_id                        UUID NOT NULL,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL DEFAULT '',
    target_type                     TEXT NOT NULL CHECK (target_type IN ('deploy','config','any')),
    steps                           JSONB NOT NULL,
    default_health_threshold        NUMERIC(4,3) NOT NULL DEFAULT 0.95 CHECK (default_health_threshold >= 0 AND default_health_threshold <= 1),
    default_rollback_on_failure     BOOLEAN NOT NULL DEFAULT TRUE,
    version                         INTEGER NOT NULL DEFAULT 1,
    is_system                       BOOLEAN NOT NULL DEFAULT FALSE,
    created_by                      UUID,
    updated_by                      UUID,
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at                      TIMESTAMPTZ,
    UNIQUE (scope_type, scope_id, name)
);
CREATE INDEX idx_strategies_scope       ON strategies(scope_type, scope_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_strategies_target_type ON strategies(target_type)          WHERE deleted_at IS NULL;

-- Strategy defaults: pins a default strategy per (scope, environment, target_type).
CREATE TABLE strategy_defaults (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_type        TEXT NOT NULL CHECK (scope_type IN ('org','project','app')),
    scope_id          UUID NOT NULL,
    environment       TEXT,   -- NULL = any environment
    target_type       TEXT CHECK (target_type IN ('deploy','config')), -- NULL = any target
    strategy_id       UUID NOT NULL REFERENCES strategies(id) ON DELETE RESTRICT,
    created_by        UUID,
    updated_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_strategy_defaults_key ON strategy_defaults (
    scope_type,
    scope_id,
    COALESCE(environment, ''),
    COALESCE(target_type, '')
);
CREATE INDEX idx_strategy_defaults_strategy ON strategy_defaults(strategy_id);

-- Rollout policies: onboarding + mandate config per scope.
CREATE TABLE rollout_policies (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_type        TEXT NOT NULL CHECK (scope_type IN ('org','project','app')),
    scope_id          UUID NOT NULL,
    environment       TEXT,   -- NULL = any environment
    target_type       TEXT CHECK (target_type IN ('deploy','config')), -- NULL = any target
    enabled           BOOLEAN NOT NULL DEFAULT FALSE,
    policy            TEXT NOT NULL DEFAULT 'off' CHECK (policy IN ('off','prompt','mandate')),
    created_by        UUID,
    updated_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_rollout_policies_key ON rollout_policies (
    scope_type,
    scope_id,
    COALESCE(environment, ''),
    COALESCE(target_type, '')
);
```

- [ ] **Step 2: Write the down migration**

Create `migrations/049_create_rollout_control_plane.down.sql`:

```sql
DROP TABLE IF EXISTS rollout_policies;
DROP TABLE IF EXISTS strategy_defaults;
DROP TABLE IF EXISTS strategies;
```

- [ ] **Step 3: Run the migration up, confirm, and roll back**

```bash
make dev-up
make migrate-up
psql "$(grep '^MIGRATE_DSN' Makefile | head -1 | cut -d= -f2-)" -c "SET search_path TO deploy; \d strategies"
make migrate-down
make migrate-up
```

Expected: `\d strategies` shows the columns and indexes from Step 1; migrate-down completes cleanly; migrate-up re-applies.

- [ ] **Step 4: Commit**

```bash
git add migrations/049_create_rollout_control_plane.up.sql migrations/049_create_rollout_control_plane.down.sql
git commit -m "feat(db): add rollout control plane schema (strategies, defaults, policies)"
```

---

## Task 2: Strategy model + Step types

**Files:**
- Create: `internal/models/strategy.go`
- Create: `internal/models/strategy_test.go`

- [ ] **Step 1: Write tests first**

Create `internal/models/strategy_test.go`:

```go
package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStrategyJSONRoundTrip(t *testing.T) {
	s := &Strategy{
		Name:        "prod-canary-standard",
		Description: "Standard prod canary",
		TargetType:  TargetTypeDeploy,
		Steps: []Step{
			{Percent: 1, MinDuration: 5 * time.Minute, MaxDuration: 30 * time.Minute, BakeTimeHealthy: 5 * time.Minute},
			{Percent: 100, MinDuration: 0, MaxDuration: 0, BakeTimeHealthy: 0},
		},
		DefaultHealthThreshold:   0.95,
		DefaultRollbackOnFailure: true,
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Strategy
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Steps[0].Percent != 1 || got.Steps[0].MinDuration != 5*time.Minute {
		t.Fatalf("roundtrip mismatch: %+v", got.Steps[0])
	}
}

func TestStepOptionalFieldsOmitEmpty(t *testing.T) {
	s := Step{Percent: 5, MinDuration: time.Minute, MaxDuration: time.Hour, BakeTimeHealthy: time.Minute}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	for _, field := range []string{"approval", "notify", "abort_conditions", "health_threshold", "signal_override"} {
		if containsField(js, field) {
			t.Errorf("expected %q to be omitted when empty: %s", field, js)
		}
	}
}

func containsField(js, name string) bool {
	return indexOf(js, `"`+name+`":`) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

```bash
go test ./internal/models/ -run TestStrategy -v
go test ./internal/models/ -run TestStep -v
```

Expected: compile error, `Strategy`/`Step`/`TargetTypeDeploy` undefined.

- [ ] **Step 3: Create the model file**

Create `internal/models/strategy.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

// TargetType identifies what a rollout applies to.
type TargetType string

const (
	TargetTypeDeploy TargetType = "deploy"
	TargetTypeConfig TargetType = "config"
	TargetTypeAny    TargetType = "any"
)

// ScopeType identifies the level at which a rollout-control entity is attached.
type ScopeType string

const (
	ScopeOrg     ScopeType = "org"
	ScopeProject ScopeType = "project"
	ScopeApp     ScopeType = "app"
)

// Strategy is a reusable rollout template.
type Strategy struct {
	ID                       uuid.UUID  `json:"id"`
	ScopeType                ScopeType  `json:"scope_type"`
	ScopeID                  uuid.UUID  `json:"scope_id"`
	Name                     string     `json:"name"`
	Description              string     `json:"description"`
	TargetType               TargetType `json:"target_type"`
	Steps                    []Step     `json:"steps"`
	DefaultHealthThreshold   float64    `json:"default_health_threshold"`
	DefaultRollbackOnFailure bool       `json:"default_rollback_on_failure"`
	Version                  int        `json:"version"`
	IsSystem                 bool       `json:"is_system"`
	CreatedBy                *uuid.UUID `json:"created_by,omitempty"`
	UpdatedBy                *uuid.UUID `json:"updated_by,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

// Step is a single phase of a strategy.
type Step struct {
	Percent          float64              `json:"percent"`
	MinDuration      time.Duration        `json:"min_duration"`
	MaxDuration      time.Duration        `json:"max_duration"`
	BakeTimeHealthy  time.Duration        `json:"bake_time_healthy"`
	HealthThreshold  *float64             `json:"health_threshold,omitempty"`
	Approval         *StepApproval        `json:"approval,omitempty"`
	Notify           *StepNotify          `json:"notify,omitempty"`
	AbortConditions  []StepAbortCondition `json:"abort_conditions,omitempty"`
	SignalOverride   *SignalSource        `json:"signal_override,omitempty"`
}

// StepApproval declares that a phase pauses at `awaiting_approval` until granted.
type StepApproval struct {
	RequiredRole string        `json:"required_role"`
	Timeout      time.Duration `json:"timeout"`
}

// StepNotify declares notification channels fired on phase entry/exit.
type StepNotify struct {
	OnEntry []string `json:"on_entry,omitempty"`
	OnExit  []string `json:"on_exit,omitempty"`
}

// StepAbortCondition is a fast-abort threshold evaluated continuously.
type StepAbortCondition struct {
	Metric    string        `json:"metric"`
	Operator  string        `json:"operator"`
	Threshold float64       `json:"threshold"`
	Window    time.Duration `json:"window"`
}

// SignalSource describes where health is read from for a rollout (or a single step).
// Kind `app_env` means "use the rollout's app+env health monitor"; other kinds
// are reserved for Plan 2.
type SignalSource struct {
	Kind string `json:"kind"`
}
```

- [ ] **Step 4: Run tests to confirm pass**

```bash
go test ./internal/models/ -run TestStrategy -v
go test ./internal/models/ -run TestStep -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/models/strategy.go internal/models/strategy_test.go
git commit -m "feat(models): add Strategy, Step, and related rollout types"
```

---

## Task 3: StrategyDefault & RolloutPolicy models

**Files:**
- Create: `internal/models/strategy_default.go`
- Create: `internal/models/rollout_policy.go`
- Create: `internal/models/rollout_policy_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/models/rollout_policy_test.go`:

```go
package models

import "testing"

func TestRolloutPolicyKind(t *testing.T) {
	cases := []struct {
		p    PolicyKind
		want string
	}{{PolicyOff, "off"}, {PolicyPrompt, "prompt"}, {PolicyMandate, "mandate"}}
	for _, c := range cases {
		if string(c.p) != c.want {
			t.Errorf("PolicyKind %v: got %q want %q", c.p, string(c.p), c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to confirm fail**

```bash
go test ./internal/models/ -run TestRolloutPolicyKind -v
```

Expected: FAIL — `PolicyKind`/`PolicyOff`/etc undefined.

- [ ] **Step 3: Create `strategy_default.go`**

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

// StrategyDefault pins a default strategy for a (scope, environment, target_type).
// Empty Environment or TargetType means wildcard.
type StrategyDefault struct {
	ID          uuid.UUID   `json:"id"`
	ScopeType   ScopeType   `json:"scope_type"`
	ScopeID     uuid.UUID   `json:"scope_id"`
	Environment *string     `json:"environment,omitempty"`
	TargetType  *TargetType `json:"target_type,omitempty"`
	StrategyID  uuid.UUID   `json:"strategy_id"`
	CreatedBy   *uuid.UUID  `json:"created_by,omitempty"`
	UpdatedBy   *uuid.UUID  `json:"updated_by,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}
```

- [ ] **Step 4: Create `rollout_policy.go`**

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

// PolicyKind controls how a scope enforces rollout usage.
type PolicyKind string

const (
	PolicyOff     PolicyKind = "off"
	PolicyPrompt  PolicyKind = "prompt"
	PolicyMandate PolicyKind = "mandate"
)

// RolloutPolicy is the onboarding + mandate row for a scope.
type RolloutPolicy struct {
	ID          uuid.UUID   `json:"id"`
	ScopeType   ScopeType   `json:"scope_type"`
	ScopeID     uuid.UUID   `json:"scope_id"`
	Environment *string     `json:"environment,omitempty"`
	TargetType  *TargetType `json:"target_type,omitempty"`
	Enabled     bool        `json:"enabled"`
	Policy      PolicyKind  `json:"policy"`
	CreatedBy   *uuid.UUID  `json:"created_by,omitempty"`
	UpdatedBy   *uuid.UUID  `json:"updated_by,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}
```

- [ ] **Step 5: Run tests, confirm pass**

```bash
go test ./internal/models/ -v
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/models/strategy_default.go internal/models/rollout_policy.go internal/models/rollout_policy_test.go
git commit -m "feat(models): add StrategyDefault and RolloutPolicy types"
```

---

## Task 4: Strategy validation

**Files:**
- Create: `internal/rollout/validation.go`
- Create: `internal/rollout/validation_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/rollout/validation_test.go`:

```go
package rollout

import (
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
)

func validStrategy() *models.Strategy {
	return &models.Strategy{
		Name:       "ok",
		TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{
			{Percent: 1, MinDuration: time.Minute, MaxDuration: 10 * time.Minute, BakeTimeHealthy: time.Minute},
			{Percent: 100, MinDuration: 0, MaxDuration: 0, BakeTimeHealthy: 0},
		},
		DefaultHealthThreshold:   0.95,
		DefaultRollbackOnFailure: true,
	}
}

func TestValidateStrategy_OK(t *testing.T) {
	if err := ValidateStrategy(validStrategy()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateStrategy_RequiresName(t *testing.T) {
	s := validStrategy()
	s.Name = ""
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStrategy_RequiresSteps(t *testing.T) {
	s := validStrategy()
	s.Steps = nil
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStrategy_PercentRange(t *testing.T) {
	s := validStrategy()
	s.Steps[0].Percent = 150
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error for percent > 100")
	}
	s.Steps[0].Percent = -1
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error for percent < 0")
	}
}

func TestValidateStrategy_StepsMonotonic(t *testing.T) {
	s := validStrategy()
	s.Steps = []models.Step{
		{Percent: 50, MinDuration: time.Minute, MaxDuration: time.Minute, BakeTimeHealthy: 0},
		{Percent: 25, MinDuration: 0, MaxDuration: 0, BakeTimeHealthy: 0},
	}
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error: percent not monotonic")
	}
}

func TestValidateStrategy_MinLessThanMax(t *testing.T) {
	s := validStrategy()
	s.Steps[0].MinDuration = 20 * time.Minute
	s.Steps[0].MaxDuration = 10 * time.Minute
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error: min > max")
	}
}

func TestValidateStrategy_BakeWithinBounds(t *testing.T) {
	s := validStrategy()
	s.Steps[0].BakeTimeHealthy = 30 * time.Minute
	s.Steps[0].MaxDuration = 10 * time.Minute
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error: bake > max")
	}
}

func TestValidateStrategy_HealthThresholdRange(t *testing.T) {
	s := validStrategy()
	th := 1.5
	s.Steps[0].HealthThreshold = &th
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error for health threshold > 1")
	}
}

func TestValidateStrategy_AbortConditionOperators(t *testing.T) {
	s := validStrategy()
	s.Steps[0].AbortConditions = []models.StepAbortCondition{
		{Metric: "error_rate", Operator: "?", Threshold: 0.01, Window: time.Minute},
	}
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error for bogus operator")
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/ -run TestValidate -v
```

Expected: package does not exist.

- [ ] **Step 3: Implement validation**

Create `internal/rollout/validation.go`:

```go
// Package rollout provides management for rollout strategy templates,
// scope defaults, and onboarding policies. The rollout execution engine
// lives in sub-packages added in a later phase.
package rollout

import (
	"fmt"

	"github.com/deploysentry/deploysentry/internal/models"
)

var validAbortOperators = map[string]bool{
	">":  true,
	">=": true,
	"<":  true,
	"<=": true,
	"==": true,
	"!=": true,
}

// ValidateStrategy returns a non-nil error when the Strategy is not
// acceptable for persistence. It checks required fields, value ranges,
// step monotonicity (percent increases across steps), and per-step
// duration/threshold constraints.
func ValidateStrategy(s *models.Strategy) error {
	if s == nil {
		return fmt.Errorf("strategy is nil")
	}
	if s.Name == "" {
		return fmt.Errorf("name is required")
	}
	switch s.TargetType {
	case models.TargetTypeDeploy, models.TargetTypeConfig, models.TargetTypeAny:
	default:
		return fmt.Errorf("invalid target_type: %q", s.TargetType)
	}
	if s.DefaultHealthThreshold < 0 || s.DefaultHealthThreshold > 1 {
		return fmt.Errorf("default_health_threshold must be in [0,1], got %g", s.DefaultHealthThreshold)
	}
	if len(s.Steps) == 0 {
		return fmt.Errorf("at least one step is required")
	}
	var prevPercent float64 = -1
	for i, st := range s.Steps {
		if st.Percent < 0 || st.Percent > 100 {
			return fmt.Errorf("step %d: percent must be in [0,100], got %g", i, st.Percent)
		}
		if st.Percent < prevPercent {
			return fmt.Errorf("step %d: percent %g < previous %g (steps must be non-decreasing)", i, st.Percent, prevPercent)
		}
		prevPercent = st.Percent
		if st.MinDuration < 0 || st.MaxDuration < 0 || st.BakeTimeHealthy < 0 {
			return fmt.Errorf("step %d: negative durations are not allowed", i)
		}
		if st.MinDuration > st.MaxDuration {
			return fmt.Errorf("step %d: min_duration %s > max_duration %s", i, st.MinDuration, st.MaxDuration)
		}
		if st.BakeTimeHealthy > st.MaxDuration && st.MaxDuration > 0 {
			return fmt.Errorf("step %d: bake_time_healthy %s > max_duration %s", i, st.BakeTimeHealthy, st.MaxDuration)
		}
		if st.HealthThreshold != nil && (*st.HealthThreshold < 0 || *st.HealthThreshold > 1) {
			return fmt.Errorf("step %d: health_threshold must be in [0,1]", i)
		}
		if st.Approval != nil && st.Approval.Timeout < 0 {
			return fmt.Errorf("step %d: approval.timeout must be non-negative", i)
		}
		for j, ac := range st.AbortConditions {
			if ac.Metric == "" {
				return fmt.Errorf("step %d abort[%d]: metric is required", i, j)
			}
			if !validAbortOperators[ac.Operator] {
				return fmt.Errorf("step %d abort[%d]: invalid operator %q", i, j, ac.Operator)
			}
			if ac.Window <= 0 {
				return fmt.Errorf("step %d abort[%d]: window must be positive", i, j)
			}
		}
	}
	if s.Steps[len(s.Steps)-1].Percent != 100 {
		return fmt.Errorf("final step must reach percent=100")
	}
	return nil
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/rollout/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/validation.go internal/rollout/validation_test.go
git commit -m "feat(rollout): strategy validation (ranges, durations, monotonic percent)"
```

---

## Task 5: Strategy repository interface + Postgres implementation

**Files:**
- Create: `internal/rollout/repository.go` (interfaces only for all three)
- Create: `internal/platform/database/postgres/rollout.go` (Strategy impl in this task)
- Create: `internal/platform/database/postgres/rollout_test.go` (Strategy tests in this task)

- [ ] **Step 1: Create the three repository interfaces**

Create `internal/rollout/repository.go`:

```go
package rollout

import (
	"context"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// StrategyRepository persists strategy templates.
type StrategyRepository interface {
	Create(ctx context.Context, s *models.Strategy) error
	Get(ctx context.Context, id uuid.UUID) (*models.Strategy, error)
	GetByName(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID, name string) (*models.Strategy, error)
	ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.Strategy, error)
	ListByAnyScope(ctx context.Context, scopeIDs []ScopeRef) ([]*models.Strategy, error)
	Update(ctx context.Context, s *models.Strategy, expectedVersion int) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	IsReferenced(ctx context.Context, id uuid.UUID) (bool, error) // true if any strategy_defaults row references it
}

// StrategyDefaultRepository persists (scope, env, target_type) → strategy defaults.
type StrategyDefaultRepository interface {
	Upsert(ctx context.Context, d *models.StrategyDefault) error
	ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.StrategyDefault, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByKey(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID, env *string, target *models.TargetType) error
}

// RolloutPolicyRepository persists onboarding + mandate policies per scope.
type RolloutPolicyRepository interface {
	Upsert(ctx context.Context, p *models.RolloutPolicy) error
	ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.RolloutPolicy, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// ScopeRef is a scope-type + scope-id pair used in multi-scope lookups.
type ScopeRef struct {
	Type models.ScopeType
	ID   uuid.UUID
}
```

- [ ] **Step 2: Write failing integration tests**

Create `internal/platform/database/postgres/rollout_test.go`:

```go
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// newStrategyRepoForTest returns a StrategyRepo backed by the project's
// standard test database (same helper every other postgres_test file uses).
// See helpers.go for testDB().
func newStrategyRepoForTest(t *testing.T) *StrategyRepo {
	t.Helper()
	return NewStrategyRepo(testDB(t))
}

func sampleStrategy(scopeID uuid.UUID) *models.Strategy {
	return &models.Strategy{
		ScopeType:                models.ScopeOrg,
		ScopeID:                  scopeID,
		Name:                     "canary-" + uuid.NewString()[:8],
		Description:              "t",
		TargetType:               models.TargetTypeDeploy,
		Steps:                    []models.Step{{Percent: 1, MinDuration: time.Minute, MaxDuration: time.Minute, BakeTimeHealthy: 0}, {Percent: 100}},
		DefaultHealthThreshold:   0.95,
		DefaultRollbackOnFailure: true,
	}
}

func TestStrategyRepo_CreateGet(t *testing.T) {
	ctx := context.Background()
	repo := newStrategyRepoForTest(t)
	orgID := uuid.New()
	s := sampleStrategy(orgID)

	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.GetByName(ctx, models.ScopeOrg, orgID, s.Name)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != s.ID || got.Name != s.Name || len(got.Steps) != 2 {
		t.Fatalf("mismatch: %+v", got)
	}
}

func TestStrategyRepo_UniqueNamePerScope(t *testing.T) {
	ctx := context.Background()
	repo := newStrategyRepoForTest(t)
	orgID := uuid.New()
	s1 := sampleStrategy(orgID)
	if err := repo.Create(ctx, s1); err != nil {
		t.Fatal(err)
	}
	s2 := sampleStrategy(orgID)
	s2.Name = s1.Name
	if err := repo.Create(ctx, s2); err == nil {
		t.Fatalf("expected unique violation")
	}
}

func TestStrategyRepo_UpdateOptimisticVersion(t *testing.T) {
	ctx := context.Background()
	repo := newStrategyRepoForTest(t)
	orgID := uuid.New()
	s := sampleStrategy(orgID)
	if err := repo.Create(ctx, s); err != nil {
		t.Fatal(err)
	}
	s.Description = "updated"
	if err := repo.Update(ctx, s, 1); err != nil {
		t.Fatalf("update v1: %v", err)
	}
	if err := repo.Update(ctx, s, 1); err == nil {
		t.Fatalf("expected version conflict")
	}
}

func TestStrategyRepo_SoftDelete(t *testing.T) {
	ctx := context.Background()
	repo := newStrategyRepoForTest(t)
	orgID := uuid.New()
	s := sampleStrategy(orgID)
	if err := repo.Create(ctx, s); err != nil {
		t.Fatal(err)
	}
	if err := repo.SoftDelete(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByName(ctx, models.ScopeOrg, orgID, s.Name); err == nil {
		t.Fatalf("expected not-found after soft delete")
	}
}
```

- [ ] **Step 3: Run tests, confirm fail**

```bash
make dev-up
make migrate-up
go test ./internal/platform/database/postgres/ -run TestStrategyRepo -v
```

Expected: compile error, `StrategyRepo` undefined.

- [ ] **Step 4: Implement the Postgres repo**

Create `internal/platform/database/postgres/rollout.go`:

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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StrategyRepo is a Postgres-backed rollout.StrategyRepository.
type StrategyRepo struct {
	db *pgxpool.Pool
}

// NewStrategyRepo returns a new StrategyRepo.
func NewStrategyRepo(db *pgxpool.Pool) *StrategyRepo {
	return &StrategyRepo{db: db}
}

var _ rollout.StrategyRepository = (*StrategyRepo)(nil)

// ErrVersionConflict is returned when Update is called with an expectedVersion
// that does not match the row's current version.
var ErrVersionConflict = errors.New("version conflict")

// ErrStrategyNotFound is returned when a strategy lookup fails.
var ErrStrategyNotFound = errors.New("strategy not found")

func (r *StrategyRepo) Create(ctx context.Context, s *models.Strategy) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	now := time.Now().UTC()
	s.CreatedAt, s.UpdatedAt = now, now
	s.Version = 1
	stepsJSON, err := json.Marshal(s.Steps)
	if err != nil {
		return fmt.Errorf("marshal steps: %w", err)
	}
	_, err = r.db.Exec(ctx, `
        INSERT INTO strategies (
            id, scope_type, scope_id, name, description, target_type, steps,
            default_health_threshold, default_rollback_on_failure,
            version, is_system, created_by, updated_by, created_at, updated_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		s.ID, s.ScopeType, s.ScopeID, s.Name, s.Description, s.TargetType, stepsJSON,
		s.DefaultHealthThreshold, s.DefaultRollbackOnFailure,
		s.Version, s.IsSystem, s.CreatedBy, s.UpdatedBy, s.CreatedAt, s.UpdatedAt,
	)
	return err
}

func (r *StrategyRepo) Get(ctx context.Context, id uuid.UUID) (*models.Strategy, error) {
	return r.scanOne(ctx, `WHERE id=$1 AND deleted_at IS NULL`, id)
}

func (r *StrategyRepo) GetByName(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID, name string) (*models.Strategy, error) {
	return r.scanOne(ctx, `WHERE scope_type=$1 AND scope_id=$2 AND name=$3 AND deleted_at IS NULL`, scopeType, scopeID, name)
}

func (r *StrategyRepo) ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.Strategy, error) {
	return r.scanMany(ctx, `WHERE scope_type=$1 AND scope_id=$2 AND deleted_at IS NULL ORDER BY name`, scopeType, scopeID)
}

func (r *StrategyRepo) ListByAnyScope(ctx context.Context, refs []rollout.ScopeRef) ([]*models.Strategy, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	types := make([]string, 0, len(refs))
	ids := make([]uuid.UUID, 0, len(refs))
	for _, r := range refs {
		types = append(types, string(r.Type))
		ids = append(ids, r.ID)
	}
	return r.scanMany(ctx, `
        WHERE (scope_type, scope_id) IN (SELECT unnest($1::text[]), unnest($2::uuid[]))
          AND deleted_at IS NULL
        ORDER BY scope_type, name`, types, ids)
}

func (r *StrategyRepo) Update(ctx context.Context, s *models.Strategy, expected int) error {
	stepsJSON, err := json.Marshal(s.Steps)
	if err != nil {
		return fmt.Errorf("marshal steps: %w", err)
	}
	s.UpdatedAt = time.Now().UTC()
	tag, err := r.db.Exec(ctx, `
        UPDATE strategies SET
            description=$1, target_type=$2, steps=$3,
            default_health_threshold=$4, default_rollback_on_failure=$5,
            updated_by=$6, updated_at=$7, version=version+1
        WHERE id=$8 AND version=$9 AND deleted_at IS NULL`,
		s.Description, s.TargetType, stepsJSON,
		s.DefaultHealthThreshold, s.DefaultRollbackOnFailure,
		s.UpdatedBy, s.UpdatedAt, s.ID, expected,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrVersionConflict
	}
	s.Version = expected + 1
	return nil
}

func (r *StrategyRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `UPDATE strategies SET deleted_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrStrategyNotFound
	}
	return nil
}

func (r *StrategyRepo) IsReferenced(ctx context.Context, id uuid.UUID) (bool, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM strategy_defaults WHERE strategy_id=$1`, id).Scan(&n)
	return n > 0, err
}

func (r *StrategyRepo) scanOne(ctx context.Context, where string, args ...any) (*models.Strategy, error) {
	rows, err := r.db.Query(ctx, selectStrategyCols+" FROM strategies "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanStrategies(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrStrategyNotFound
	}
	return list[0], nil
}

func (r *StrategyRepo) scanMany(ctx context.Context, where string, args ...any) ([]*models.Strategy, error) {
	rows, err := r.db.Query(ctx, selectStrategyCols+" FROM strategies "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStrategies(rows)
}

const selectStrategyCols = `SELECT
    id, scope_type, scope_id, name, description, target_type, steps,
    default_health_threshold, default_rollback_on_failure,
    version, is_system, created_by, updated_by, created_at, updated_at`

func scanStrategies(rows pgx.Rows) ([]*models.Strategy, error) {
	var out []*models.Strategy
	for rows.Next() {
		var s models.Strategy
		var stepsJSON []byte
		var createdBy, updatedBy sql.Null[uuid.UUID]
		if err := rows.Scan(
			&s.ID, &s.ScopeType, &s.ScopeID, &s.Name, &s.Description, &s.TargetType, &stepsJSON,
			&s.DefaultHealthThreshold, &s.DefaultRollbackOnFailure,
			&s.Version, &s.IsSystem, &createdBy, &updatedBy, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(stepsJSON, &s.Steps); err != nil {
			return nil, fmt.Errorf("decode steps: %w", err)
		}
		if createdBy.Valid {
			id := createdBy.V
			s.CreatedBy = &id
		}
		if updatedBy.Valid {
			id := updatedBy.V
			s.UpdatedBy = &id
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}
```

- [ ] **Step 5: Run tests, confirm pass**

```bash
go test ./internal/platform/database/postgres/ -run TestStrategyRepo -v
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/rollout/repository.go internal/platform/database/postgres/rollout.go internal/platform/database/postgres/rollout_test.go
git commit -m "feat(rollout): StrategyRepository + Postgres implementation"
```

---

## Task 6: StrategyDefault Postgres repository

**Files:**
- Modify: `internal/platform/database/postgres/rollout.go`
- Modify: `internal/platform/database/postgres/rollout_test.go`

- [ ] **Step 1: Add failing integration tests**

Append to `rollout_test.go`:

```go
func TestStrategyDefaultsRepo_Upsert(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	srepo := NewStrategyRepo(db)
	drepo := NewStrategyDefaultsRepo(db)
	orgID := uuid.New()
	s := sampleStrategy(orgID)
	if err := srepo.Create(ctx, s); err != nil {
		t.Fatal(err)
	}
	env := "prod"
	tt := models.TargetTypeDeploy
	d := &models.StrategyDefault{ScopeType: models.ScopeOrg, ScopeID: orgID, Environment: &env, TargetType: &tt, StrategyID: s.ID}
	if err := drepo.Upsert(ctx, d); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// Upsert again replaces
	s2 := sampleStrategy(orgID)
	if err := srepo.Create(ctx, s2); err != nil {
		t.Fatal(err)
	}
	d.StrategyID = s2.ID
	if err := drepo.Upsert(ctx, d); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := drepo.ListByScope(ctx, models.ScopeOrg, orgID)
	if err != nil || len(got) != 1 || got[0].StrategyID != s2.ID {
		t.Fatalf("list: err=%v, got=%+v", err, got)
	}
}

func TestStrategyDefaultsRepo_WildcardKeyUniqueness(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	srepo := NewStrategyRepo(db)
	drepo := NewStrategyDefaultsRepo(db)
	orgID := uuid.New()
	s := sampleStrategy(orgID)
	if err := srepo.Create(ctx, s); err != nil {
		t.Fatal(err)
	}
	// Two rows: (nil env, deploy target) and (nil env, nil target) must coexist.
	tt := models.TargetTypeDeploy
	if err := drepo.Upsert(ctx, &models.StrategyDefault{ScopeType: models.ScopeOrg, ScopeID: orgID, TargetType: &tt, StrategyID: s.ID}); err != nil {
		t.Fatal(err)
	}
	if err := drepo.Upsert(ctx, &models.StrategyDefault{ScopeType: models.ScopeOrg, ScopeID: orgID, StrategyID: s.ID}); err != nil {
		t.Fatal(err)
	}
	rows, err := drepo.ListByScope(ctx, models.ScopeOrg, orgID)
	if err != nil || len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d (err=%v)", len(rows), err)
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/platform/database/postgres/ -run TestStrategyDefaultsRepo -v
```

Expected: compile error.

- [ ] **Step 3: Implement `StrategyDefaultsRepo`**

Append to `internal/platform/database/postgres/rollout.go`:

```go
// StrategyDefaultsRepo is a Postgres-backed rollout.StrategyDefaultRepository.
type StrategyDefaultsRepo struct {
	db *pgxpool.Pool
}

// NewStrategyDefaultsRepo returns a new StrategyDefaultsRepo.
func NewStrategyDefaultsRepo(db *pgxpool.Pool) *StrategyDefaultsRepo {
	return &StrategyDefaultsRepo{db: db}
}

var _ rollout.StrategyDefaultRepository = (*StrategyDefaultsRepo)(nil)

func (r *StrategyDefaultsRepo) Upsert(ctx context.Context, d *models.StrategyDefault) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	now := time.Now().UTC()
	d.UpdatedAt = now
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	envStr := ""
	if d.Environment != nil {
		envStr = *d.Environment
	}
	ttStr := ""
	if d.TargetType != nil {
		ttStr = string(*d.TargetType)
	}
	_, err := r.db.Exec(ctx, `
        INSERT INTO strategy_defaults (id, scope_type, scope_id, environment, target_type, strategy_id, created_by, updated_by, created_at, updated_at)
        VALUES ($1,$2,$3, NULLIF($4,''), NULLIF($5,''), $6, $7, $8, $9, $10)
        ON CONFLICT (scope_type, scope_id, COALESCE(environment,''), COALESCE(target_type,''))
        DO UPDATE SET strategy_id=EXCLUDED.strategy_id, updated_by=EXCLUDED.updated_by, updated_at=EXCLUDED.updated_at`,
		d.ID, d.ScopeType, d.ScopeID, envStr, ttStr, d.StrategyID, d.CreatedBy, d.UpdatedBy, d.CreatedAt, d.UpdatedAt,
	)
	return err
}

func (r *StrategyDefaultsRepo) ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.StrategyDefault, error) {
	rows, err := r.db.Query(ctx, `
        SELECT id, scope_type, scope_id, environment, target_type, strategy_id, created_by, updated_by, created_at, updated_at
        FROM strategy_defaults WHERE scope_type=$1 AND scope_id=$2 ORDER BY COALESCE(environment,''), COALESCE(target_type,'')`,
		scopeType, scopeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.StrategyDefault
	for rows.Next() {
		var d models.StrategyDefault
		var env, tt sql.NullString
		var createdBy, updatedBy sql.Null[uuid.UUID]
		if err := rows.Scan(&d.ID, &d.ScopeType, &d.ScopeID, &env, &tt, &d.StrategyID, &createdBy, &updatedBy, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		if env.Valid {
			e := env.String
			d.Environment = &e
		}
		if tt.Valid {
			t := models.TargetType(tt.String)
			d.TargetType = &t
		}
		if createdBy.Valid {
			id := createdBy.V
			d.CreatedBy = &id
		}
		if updatedBy.Valid {
			id := updatedBy.V
			d.UpdatedBy = &id
		}
		out = append(out, &d)
	}
	return out, rows.Err()
}

func (r *StrategyDefaultsRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM strategy_defaults WHERE id=$1`, id)
	return err
}

func (r *StrategyDefaultsRepo) DeleteByKey(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID, env *string, target *models.TargetType) error {
	envStr, ttStr := "", ""
	if env != nil {
		envStr = *env
	}
	if target != nil {
		ttStr = string(*target)
	}
	_, err := r.db.Exec(ctx, `
        DELETE FROM strategy_defaults
        WHERE scope_type=$1 AND scope_id=$2 AND COALESCE(environment,'')=$3 AND COALESCE(target_type,'')=$4`,
		scopeType, scopeID, envStr, ttStr,
	)
	return err
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/platform/database/postgres/ -run TestStrategyDefaults -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/platform/database/postgres/rollout.go internal/platform/database/postgres/rollout_test.go
git commit -m "feat(rollout): StrategyDefaultsRepo Postgres implementation"
```

---

## Task 7: RolloutPolicy Postgres repository

**Files:**
- Modify: `internal/platform/database/postgres/rollout.go`
- Modify: `internal/platform/database/postgres/rollout_test.go`

- [ ] **Step 1: Add failing integration test**

Append to `rollout_test.go`:

```go
func TestRolloutPolicyRepo_Upsert(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	repo := NewRolloutPolicyRepo(db)
	orgID := uuid.New()
	p := &models.RolloutPolicy{ScopeType: models.ScopeOrg, ScopeID: orgID, Enabled: true, Policy: models.PolicyPrompt}
	if err := repo.Upsert(ctx, p); err != nil {
		t.Fatal(err)
	}
	p.Policy = models.PolicyMandate
	if err := repo.Upsert(ctx, p); err != nil {
		t.Fatal(err)
	}
	rows, err := repo.ListByScope(ctx, models.ScopeOrg, orgID)
	if err != nil || len(rows) != 1 || rows[0].Policy != models.PolicyMandate {
		t.Fatalf("list: err=%v rows=%+v", err, rows)
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/platform/database/postgres/ -run TestRolloutPolicyRepo -v
```

Expected: compile error.

- [ ] **Step 3: Implement `RolloutPolicyRepo`**

Append to `internal/platform/database/postgres/rollout.go`:

```go
// RolloutPolicyRepo is a Postgres-backed rollout.RolloutPolicyRepository.
type RolloutPolicyRepo struct {
	db *pgxpool.Pool
}

// NewRolloutPolicyRepo returns a new RolloutPolicyRepo.
func NewRolloutPolicyRepo(db *pgxpool.Pool) *RolloutPolicyRepo {
	return &RolloutPolicyRepo{db: db}
}

var _ rollout.RolloutPolicyRepository = (*RolloutPolicyRepo)(nil)

func (r *RolloutPolicyRepo) Upsert(ctx context.Context, p *models.RolloutPolicy) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now().UTC()
	p.UpdatedAt = now
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	envStr := ""
	if p.Environment != nil {
		envStr = *p.Environment
	}
	ttStr := ""
	if p.TargetType != nil {
		ttStr = string(*p.TargetType)
	}
	_, err := r.db.Exec(ctx, `
        INSERT INTO rollout_policies (id, scope_type, scope_id, environment, target_type, enabled, policy, created_by, updated_by, created_at, updated_at)
        VALUES ($1,$2,$3, NULLIF($4,''), NULLIF($5,''), $6, $7, $8, $9, $10, $11)
        ON CONFLICT (scope_type, scope_id, COALESCE(environment,''), COALESCE(target_type,''))
        DO UPDATE SET enabled=EXCLUDED.enabled, policy=EXCLUDED.policy, updated_by=EXCLUDED.updated_by, updated_at=EXCLUDED.updated_at`,
		p.ID, p.ScopeType, p.ScopeID, envStr, ttStr, p.Enabled, p.Policy, p.CreatedBy, p.UpdatedBy, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (r *RolloutPolicyRepo) ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.RolloutPolicy, error) {
	rows, err := r.db.Query(ctx, `
        SELECT id, scope_type, scope_id, environment, target_type, enabled, policy, created_by, updated_by, created_at, updated_at
        FROM rollout_policies WHERE scope_type=$1 AND scope_id=$2 ORDER BY COALESCE(environment,''), COALESCE(target_type,'')`,
		scopeType, scopeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.RolloutPolicy
	for rows.Next() {
		var p models.RolloutPolicy
		var env, tt sql.NullString
		var createdBy, updatedBy sql.Null[uuid.UUID]
		if err := rows.Scan(&p.ID, &p.ScopeType, &p.ScopeID, &env, &tt, &p.Enabled, &p.Policy, &createdBy, &updatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if env.Valid {
			e := env.String
			p.Environment = &e
		}
		if tt.Valid {
			t := models.TargetType(tt.String)
			p.TargetType = &t
		}
		if createdBy.Valid {
			id := createdBy.V
			p.CreatedBy = &id
		}
		if updatedBy.Valid {
			id := updatedBy.V
			p.UpdatedBy = &id
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

func (r *RolloutPolicyRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM rollout_policies WHERE id=$1`, id)
	return err
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/platform/database/postgres/ -run TestRolloutPolicyRepo -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/platform/database/postgres/rollout.go internal/platform/database/postgres/rollout_test.go
git commit -m "feat(rollout): RolloutPolicyRepo Postgres implementation"
```

---

## Task 8: Scope inheritance resolver

**Files:**
- Create: `internal/rollout/resolver.go`
- Create: `internal/rollout/resolver_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/rollout/resolver_test.go`:

```go
package rollout

import (
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

func TestAncestorScopes_App(t *testing.T) {
	orgID, projID, appID := uuid.New(), uuid.New(), uuid.New()
	got := AncestorScopes(ScopeRef{Type: models.ScopeApp, ID: appID}, &projID, &orgID)
	want := []ScopeRef{{models.ScopeApp, appID}, {models.ScopeProject, projID}, {models.ScopeOrg, orgID}}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pos %d: got %v want %v", i, got[i], want[i])
		}
	}
}

func TestAncestorScopes_Org(t *testing.T) {
	orgID := uuid.New()
	got := AncestorScopes(ScopeRef{Type: models.ScopeOrg, ID: orgID}, nil, nil)
	if len(got) != 1 || got[0].Type != models.ScopeOrg {
		t.Fatalf("got %v", got)
	}
}

func TestResolveDefault_MatchesMostSpecificKey(t *testing.T) {
	orgID, projID := uuid.New(), uuid.New()
	env := "prod"
	tt := models.TargetTypeDeploy
	rows := []*models.StrategyDefault{
		{ScopeType: models.ScopeOrg, ScopeID: orgID, StrategyID: uuid.New()},                                // (org, any, any)
		{ScopeType: models.ScopeOrg, ScopeID: orgID, TargetType: &tt, StrategyID: uuid.New()},               // (org, any, deploy)
		{ScopeType: models.ScopeProject, ScopeID: projID, Environment: &env, TargetType: &tt, StrategyID: uuid.New()}, // (project, prod, deploy)
	}
	ancestors := []ScopeRef{{models.ScopeProject, projID}, {models.ScopeOrg, orgID}}
	got := ResolveDefault(rows, ancestors, &env, &tt)
	if got == nil || got.ScopeType != models.ScopeProject {
		t.Fatalf("expected project match, got %+v", got)
	}
}

func TestResolveDefault_FallsThroughToWildcard(t *testing.T) {
	orgID := uuid.New()
	tt := models.TargetTypeDeploy
	rows := []*models.StrategyDefault{
		{ScopeType: models.ScopeOrg, ScopeID: orgID, StrategyID: uuid.New()}, // (org, any, any)
	}
	env := "prod"
	ancestors := []ScopeRef{{models.ScopeOrg, orgID}}
	got := ResolveDefault(rows, ancestors, &env, &tt)
	if got == nil {
		t.Fatalf("expected wildcard fallback")
	}
}

func TestResolveDefault_NoMatch(t *testing.T) {
	ancestors := []ScopeRef{{models.ScopeOrg, uuid.New()}}
	env := "prod"
	tt := models.TargetTypeDeploy
	got := ResolveDefault(nil, ancestors, &env, &tt)
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/ -run TestAncestorScopes -v
go test ./internal/rollout/ -run TestResolveDefault -v
```

Expected: compile error.

- [ ] **Step 3: Implement resolver**

Create `internal/rollout/resolver.go`:

```go
package rollout

import (
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// AncestorScopes returns scope refs from most-specific to least-specific for
// a given leaf scope. Parents passed as nil are skipped; pass nil for both to
// walk an org-only leaf.
func AncestorScopes(leaf ScopeRef, projectID, orgID *uuid.UUID) []ScopeRef {
	switch leaf.Type {
	case models.ScopeApp:
		out := []ScopeRef{leaf}
		if projectID != nil {
			out = append(out, ScopeRef{models.ScopeProject, *projectID})
		}
		if orgID != nil {
			out = append(out, ScopeRef{models.ScopeOrg, *orgID})
		}
		return out
	case models.ScopeProject:
		out := []ScopeRef{leaf}
		if orgID != nil {
			out = append(out, ScopeRef{models.ScopeOrg, *orgID})
		}
		return out
	case models.ScopeOrg:
		return []ScopeRef{leaf}
	}
	return nil
}

// ResolveDefault walks ancestors most-specific → least-specific, within each
// scope trying key patterns in priority order (env+target, env+any, any+target, any+any).
// First match wins.
func ResolveDefault(rows []*models.StrategyDefault, ancestors []ScopeRef, env *string, target *models.TargetType) *models.StrategyDefault {
	bucket := map[ScopeRef][]*models.StrategyDefault{}
	for _, r := range rows {
		ref := ScopeRef{r.ScopeType, r.ScopeID}
		bucket[ref] = append(bucket[ref], r)
	}
	patterns := []struct {
		matchEnv    bool
		matchTarget bool
	}{{true, true}, {true, false}, {false, true}, {false, false}}
	for _, anc := range ancestors {
		for _, pat := range patterns {
			for _, row := range bucket[anc] {
				if pat.matchEnv {
					if row.Environment == nil || env == nil || *row.Environment != *env {
						continue
					}
				} else if row.Environment != nil {
					continue
				}
				if pat.matchTarget {
					if row.TargetType == nil || target == nil || *row.TargetType != *target {
						continue
					}
				} else if row.TargetType != nil {
					continue
				}
				return row
			}
		}
	}
	return nil
}

// ResolvePolicy walks ancestors most-specific → least-specific and returns the
// first matching policy. Same key-pattern priority as ResolveDefault.
func ResolvePolicy(rows []*models.RolloutPolicy, ancestors []ScopeRef, env *string, target *models.TargetType) *models.RolloutPolicy {
	bucket := map[ScopeRef][]*models.RolloutPolicy{}
	for _, r := range rows {
		ref := ScopeRef{r.ScopeType, r.ScopeID}
		bucket[ref] = append(bucket[ref], r)
	}
	patterns := []struct {
		matchEnv    bool
		matchTarget bool
	}{{true, true}, {true, false}, {false, true}, {false, false}}
	for _, anc := range ancestors {
		for _, pat := range patterns {
			for _, row := range bucket[anc] {
				if pat.matchEnv {
					if row.Environment == nil || env == nil || *row.Environment != *env {
						continue
					}
				} else if row.Environment != nil {
					continue
				}
				if pat.matchTarget {
					if row.TargetType == nil || target == nil || *row.TargetType != *target {
						continue
					}
				} else if row.TargetType != nil {
					continue
				}
				return row
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/rollout/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/resolver.go internal/rollout/resolver_test.go
git commit -m "feat(rollout): scope inheritance resolver for defaults and policies"
```

---

## Task 9: Strategy / StrategyDefault / RolloutPolicy services

**Files:**
- Create: `internal/rollout/service.go`
- Create: `internal/rollout/service_test.go`

- [ ] **Step 1: Write failing tests (table-driven, mock repos)**

Create `internal/rollout/service_test.go`:

```go
package rollout

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// --- in-memory fake repos ---

type fakeStratRepo struct{ rows map[uuid.UUID]*models.Strategy }

func newFakeStratRepo() *fakeStratRepo { return &fakeStratRepo{rows: map[uuid.UUID]*models.Strategy{}} }

func (f *fakeStratRepo) Create(_ context.Context, s *models.Strategy) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	for _, existing := range f.rows {
		if existing.ScopeType == s.ScopeType && existing.ScopeID == s.ScopeID && existing.Name == s.Name {
			return errors.New("dup")
		}
	}
	s.Version = 1
	f.rows[s.ID] = s
	return nil
}
func (f *fakeStratRepo) Get(_ context.Context, id uuid.UUID) (*models.Strategy, error) {
	if r, ok := f.rows[id]; ok {
		return r, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeStratRepo) GetByName(_ context.Context, st models.ScopeType, sid uuid.UUID, name string) (*models.Strategy, error) {
	for _, r := range f.rows {
		if r.ScopeType == st && r.ScopeID == sid && r.Name == name {
			return r, nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeStratRepo) ListByScope(_ context.Context, st models.ScopeType, sid uuid.UUID) ([]*models.Strategy, error) {
	var out []*models.Strategy
	for _, r := range f.rows {
		if r.ScopeType == st && r.ScopeID == sid {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeStratRepo) ListByAnyScope(_ context.Context, refs []ScopeRef) ([]*models.Strategy, error) {
	refset := map[ScopeRef]bool{}
	for _, r := range refs {
		refset[r] = true
	}
	var out []*models.Strategy
	for _, r := range f.rows {
		if refset[ScopeRef{r.ScopeType, r.ScopeID}] {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeStratRepo) Update(_ context.Context, s *models.Strategy, expected int) error {
	cur, ok := f.rows[s.ID]
	if !ok {
		return errors.New("not found")
	}
	if cur.Version != expected {
		return errors.New("version conflict")
	}
	cur.Description, cur.Steps, cur.Version = s.Description, s.Steps, cur.Version+1
	return nil
}
func (f *fakeStratRepo) SoftDelete(_ context.Context, id uuid.UUID) error { delete(f.rows, id); return nil }
func (f *fakeStratRepo) IsReferenced(_ context.Context, id uuid.UUID) (bool, error) { return false, nil }

// --- tests ---

func TestStrategyService_CreateValidates(t *testing.T) {
	svc := NewStrategyService(newFakeStratRepo(), nil)
	bad := &models.Strategy{Name: "bad", TargetType: models.TargetTypeDeploy} // no steps
	if err := svc.Create(context.Background(), bad); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestStrategyService_EffectiveList_Inheritance(t *testing.T) {
	repo := newFakeStratRepo()
	orgID, projID := uuid.New(), uuid.New()
	orgStrat := &models.Strategy{ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "std",
		TargetType: models.TargetTypeDeploy,
		Steps:      []models.Step{{Percent: 100}},
		DefaultHealthThreshold: 0.95, DefaultRollbackOnFailure: true}
	if err := repo.Create(context.Background(), orgStrat); err != nil {
		t.Fatal(err)
	}
	projStrat := &models.Strategy{ScopeType: models.ScopeProject, ScopeID: projID, Name: "std", // shadows org
		TargetType: models.TargetTypeDeploy, Steps: []models.Step{{Percent: 100}},
		DefaultHealthThreshold: 0.95, DefaultRollbackOnFailure: true}
	if err := repo.Create(context.Background(), projStrat); err != nil {
		t.Fatal(err)
	}

	svc := NewStrategyService(repo, nil)
	eff, err := svc.EffectiveList(context.Background(), ScopeRef{models.ScopeProject, projID}, &projID, &orgID)
	if err != nil {
		t.Fatal(err)
	}
	if len(eff) != 1 || eff[0].Strategy.ID != projStrat.ID {
		t.Fatalf("expected project to shadow org: %+v", eff)
	}
	if eff[0].OriginScope.Type != models.ScopeProject {
		t.Fatalf("origin_scope mismatch: %+v", eff[0].OriginScope)
	}
}

func TestStrategyService_Delete_BlockedIfReferenced(t *testing.T) {
	r := newFakeStratRepo()
	// Force IsReferenced to return true.
	rw := &refsTrue{fakeStratRepo: r}
	svc := NewStrategyService(rw, nil)
	orgID := uuid.New()
	s := &models.Strategy{ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "x", TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	if err := r.Create(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(context.Background(), s.ID); err == nil {
		t.Fatalf("expected reference-protection error")
	}
}

func TestStrategyService_Delete_BlockedIfSystem(t *testing.T) {
	r := newFakeStratRepo()
	svc := NewStrategyService(r, nil)
	orgID := uuid.New()
	s := &models.Strategy{ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "x", TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95, IsSystem: true}
	if err := r.Create(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(context.Background(), s.ID); err == nil {
		t.Fatalf("expected system-protection error")
	}
}

// refsTrue wraps fakeStratRepo to force IsReferenced=true.
type refsTrue struct{ *fakeStratRepo }

func (r *refsTrue) IsReferenced(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil }

var _ = time.Now // silence unused
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/ -run TestStrategyService -v
```

Expected: compile error.

- [ ] **Step 3: Implement services**

Create `internal/rollout/service.go`:

```go
package rollout

import (
	"context"
	"errors"
	"fmt"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// EffectiveStrategy is a strategy + metadata about where it came from.
type EffectiveStrategy struct {
	Strategy    *models.Strategy `json:"strategy"`
	OriginScope ScopeRef         `json:"origin_scope"`
	IsInherited bool             `json:"is_inherited"`
}

// AuditLogger is the minimal interface the services need to record audit trails.
// Concrete implementation wired at cmd/api level. A no-op is acceptable in tests.
type AuditLogger interface {
	Log(ctx context.Context, action string, actorID uuid.UUID, payload map[string]any) error
}

// StrategyService provides template CRUD + inheritance.
type StrategyService struct {
	repo  StrategyRepository
	audit AuditLogger
}

// NewStrategyService builds a StrategyService.
func NewStrategyService(repo StrategyRepository, audit AuditLogger) *StrategyService {
	return &StrategyService{repo: repo, audit: audit}
}

// ErrSystemStrategyImmutable is returned when a system template's delete/update is attempted.
var ErrSystemStrategyImmutable = errors.New("system strategy cannot be modified or deleted")

// ErrStrategyInUse is returned when Delete is blocked by a strategy_defaults reference.
var ErrStrategyInUse = errors.New("strategy is referenced by a default assignment")

// Create validates and persists a new strategy.
func (s *StrategyService) Create(ctx context.Context, st *models.Strategy) error {
	if err := ValidateStrategy(st); err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	if err := s.repo.Create(ctx, st); err != nil {
		return fmt.Errorf("create: %w", err)
	}
	return nil
}

// Update applies changes if the expected version matches the DB row.
func (s *StrategyService) Update(ctx context.Context, st *models.Strategy, expectedVersion int) error {
	existing, err := s.repo.Get(ctx, st.ID)
	if err != nil {
		return err
	}
	if existing.IsSystem {
		return ErrSystemStrategyImmutable
	}
	if err := ValidateStrategy(st); err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	return s.repo.Update(ctx, st, expectedVersion)
}

// Delete blocks on system templates or referenced strategies, then soft-deletes.
func (s *StrategyService) Delete(ctx context.Context, id uuid.UUID) error {
	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if existing.IsSystem {
		return ErrSystemStrategyImmutable
	}
	used, err := s.repo.IsReferenced(ctx, id)
	if err != nil {
		return err
	}
	if used {
		return ErrStrategyInUse
	}
	return s.repo.SoftDelete(ctx, id)
}

// EffectiveList returns all strategies visible at the leaf scope, including
// inherited ones. Names in more specific scopes shadow less-specific scopes.
func (s *StrategyService) EffectiveList(ctx context.Context, leaf ScopeRef, projectID, orgID *uuid.UUID) ([]*EffectiveStrategy, error) {
	ancestors := AncestorScopes(leaf, projectID, orgID)
	rows, err := s.repo.ListByAnyScope(ctx, ancestors)
	if err != nil {
		return nil, err
	}
	// Most-specific first in `ancestors`. Keep first occurrence per name.
	seen := map[string]bool{}
	byScope := map[ScopeRef][]*models.Strategy{}
	for _, r := range rows {
		byScope[ScopeRef{r.ScopeType, r.ScopeID}] = append(byScope[ScopeRef{r.ScopeType, r.ScopeID}], r)
	}
	var out []*EffectiveStrategy
	for _, anc := range ancestors {
		for _, r := range byScope[anc] {
			if seen[r.Name] {
				continue
			}
			seen[r.Name] = true
			out = append(out, &EffectiveStrategy{Strategy: r, OriginScope: anc, IsInherited: anc != leaf})
		}
	}
	return out, nil
}

// Get returns a single strategy by ID.
func (s *StrategyService) Get(ctx context.Context, id uuid.UUID) (*models.Strategy, error) {
	return s.repo.Get(ctx, id)
}

// GetByName returns a strategy by (scope, name), not searching ancestors.
func (s *StrategyService) GetByName(ctx context.Context, st models.ScopeType, sid uuid.UUID, name string) (*models.Strategy, error) {
	return s.repo.GetByName(ctx, st, sid, name)
}

// StrategyDefaultService encapsulates defaults CRUD + scope-inheritance resolution.
type StrategyDefaultService struct {
	repo StrategyDefaultRepository
}

// NewStrategyDefaultService builds a StrategyDefaultService.
func NewStrategyDefaultService(repo StrategyDefaultRepository) *StrategyDefaultService {
	return &StrategyDefaultService{repo: repo}
}

// Upsert writes the default row.
func (s *StrategyDefaultService) Upsert(ctx context.Context, d *models.StrategyDefault) error {
	return s.repo.Upsert(ctx, d)
}

// List returns rows defined directly on the scope (no inheritance).
func (s *StrategyDefaultService) List(ctx context.Context, st models.ScopeType, sid uuid.UUID) ([]*models.StrategyDefault, error) {
	return s.repo.ListByScope(ctx, st, sid)
}

// Resolve walks ancestors and returns the most-specific matching default.
func (s *StrategyDefaultService) Resolve(ctx context.Context, leaf ScopeRef, projectID, orgID *uuid.UUID, env *string, target *models.TargetType) (*models.StrategyDefault, error) {
	ancestors := AncestorScopes(leaf, projectID, orgID)
	var allRows []*models.StrategyDefault
	for _, anc := range ancestors {
		rows, err := s.repo.ListByScope(ctx, anc.Type, anc.ID)
		if err != nil {
			return nil, err
		}
		allRows = append(allRows, rows...)
	}
	return ResolveDefault(allRows, ancestors, env, target), nil
}

// Delete removes a default row by ID.
func (s *StrategyDefaultService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// RolloutPolicyService encapsulates policy CRUD + scope-inheritance resolution.
type RolloutPolicyService struct {
	repo RolloutPolicyRepository
}

// NewRolloutPolicyService builds a RolloutPolicyService.
func NewRolloutPolicyService(repo RolloutPolicyRepository) *RolloutPolicyService {
	return &RolloutPolicyService{repo: repo}
}

// Upsert writes the policy row.
func (s *RolloutPolicyService) Upsert(ctx context.Context, p *models.RolloutPolicy) error {
	return s.repo.Upsert(ctx, p)
}

// List returns rows defined directly on the scope (no inheritance).
func (s *RolloutPolicyService) List(ctx context.Context, st models.ScopeType, sid uuid.UUID) ([]*models.RolloutPolicy, error) {
	return s.repo.ListByScope(ctx, st, sid)
}

// Resolve walks ancestors and returns the most-specific matching policy.
// If no row matches, returns nil (caller treats as "off" = immediate-apply).
func (s *RolloutPolicyService) Resolve(ctx context.Context, leaf ScopeRef, projectID, orgID *uuid.UUID, env *string, target *models.TargetType) (*models.RolloutPolicy, error) {
	ancestors := AncestorScopes(leaf, projectID, orgID)
	var allRows []*models.RolloutPolicy
	for _, anc := range ancestors {
		rows, err := s.repo.ListByScope(ctx, anc.Type, anc.ID)
		if err != nil {
			return nil, err
		}
		allRows = append(allRows, rows...)
	}
	return ResolvePolicy(allRows, ancestors, env, target), nil
}

// Delete removes a policy row by ID.
func (s *RolloutPolicyService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/rollout/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/service.go internal/rollout/service_test.go
git commit -m "feat(rollout): services for strategies, defaults, policies with inheritance"
```

---

## Task 10: YAML import/export converter

**Files:**
- Create: `internal/rollout/yaml.go`
- Create: `internal/rollout/yaml_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/rollout/yaml_test.go`:

```go
package rollout

import (
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
)

func TestYAML_RoundTrip(t *testing.T) {
	s := &models.Strategy{
		Name: "prod-canary-standard", Description: "d",
		TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{
			{Percent: 1, MinDuration: 5 * time.Minute, MaxDuration: 30 * time.Minute, BakeTimeHealthy: 5 * time.Minute,
				AbortConditions: []models.StepAbortCondition{{Metric: "error_rate", Operator: ">", Threshold: 0.02, Window: time.Minute}}},
			{Percent: 100, MinDuration: 0, MaxDuration: 0, BakeTimeHealthy: 0},
		},
		DefaultHealthThreshold: 0.95, DefaultRollbackOnFailure: true,
	}
	b, err := MarshalStrategyYAML(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := UnmarshalStrategyYAML(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != s.Name || len(got.Steps) != 2 || got.Steps[0].MinDuration != 5*time.Minute ||
		got.Steps[0].AbortConditions[0].Metric != "error_rate" {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
}

func TestYAML_DurationStrings(t *testing.T) {
	y := []byte(`
name: s
target_type: deploy
default_health_threshold: 0.95
default_rollback_on_failure: true
steps:
  - percent: 1
    min_duration: 5m
    max_duration: 1h
    bake_time_healthy: 10s
  - percent: 100
    min_duration: 0s
    max_duration: 0s
    bake_time_healthy: 0s
`)
	got, err := UnmarshalStrategyYAML(y)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Steps[0].MinDuration != 5*time.Minute || got.Steps[0].MaxDuration != time.Hour {
		t.Fatalf("duration parse: %+v", got.Steps[0])
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/ -run TestYAML -v
```

Expected: compile error — `MarshalStrategyYAML` undefined.

- [ ] **Step 3: Implement YAML converter**

Create `internal/rollout/yaml.go`:

```go
package rollout

import (
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"gopkg.in/yaml.v3"
)

// yamlStrategy is the on-disk shape; durations are strings.
type yamlStrategy struct {
	Name                     string     `yaml:"name"`
	Description              string     `yaml:"description,omitempty"`
	TargetType               string     `yaml:"target_type"`
	DefaultHealthThreshold   float64    `yaml:"default_health_threshold"`
	DefaultRollbackOnFailure bool       `yaml:"default_rollback_on_failure"`
	Steps                    []yamlStep `yaml:"steps"`
}

type yamlStep struct {
	Percent          float64               `yaml:"percent"`
	MinDuration      string                `yaml:"min_duration"`
	MaxDuration      string                `yaml:"max_duration"`
	BakeTimeHealthy  string                `yaml:"bake_time_healthy"`
	HealthThreshold  *float64              `yaml:"health_threshold,omitempty"`
	Approval         *yamlApproval         `yaml:"approval,omitempty"`
	Notify           *yamlNotify           `yaml:"notify,omitempty"`
	AbortConditions  []yamlAbortCondition  `yaml:"abort_conditions,omitempty"`
	SignalOverride   *yamlSignalSource     `yaml:"signal_override,omitempty"`
}

type yamlApproval struct {
	RequiredRole string `yaml:"required_role"`
	Timeout      string `yaml:"timeout"`
}

type yamlNotify struct {
	OnEntry []string `yaml:"on_entry,omitempty"`
	OnExit  []string `yaml:"on_exit,omitempty"`
}

type yamlAbortCondition struct {
	Metric    string  `yaml:"metric"`
	Operator  string  `yaml:"operator"`
	Threshold float64 `yaml:"threshold"`
	Window    string  `yaml:"window"`
}

type yamlSignalSource struct {
	Kind string `yaml:"kind"`
}

// MarshalStrategyYAML converts a strategy to YAML bytes.
func MarshalStrategyYAML(s *models.Strategy) ([]byte, error) {
	y := yamlStrategy{
		Name: s.Name, Description: s.Description,
		TargetType:               string(s.TargetType),
		DefaultHealthThreshold:   s.DefaultHealthThreshold,
		DefaultRollbackOnFailure: s.DefaultRollbackOnFailure,
	}
	for _, st := range s.Steps {
		ys := yamlStep{
			Percent:         st.Percent,
			MinDuration:     st.MinDuration.String(),
			MaxDuration:     st.MaxDuration.String(),
			BakeTimeHealthy: st.BakeTimeHealthy.String(),
			HealthThreshold: st.HealthThreshold,
		}
		if st.Approval != nil {
			ys.Approval = &yamlApproval{RequiredRole: st.Approval.RequiredRole, Timeout: st.Approval.Timeout.String()}
		}
		if st.Notify != nil {
			ys.Notify = &yamlNotify{OnEntry: st.Notify.OnEntry, OnExit: st.Notify.OnExit}
		}
		for _, ac := range st.AbortConditions {
			ys.AbortConditions = append(ys.AbortConditions, yamlAbortCondition{
				Metric: ac.Metric, Operator: ac.Operator, Threshold: ac.Threshold, Window: ac.Window.String(),
			})
		}
		if st.SignalOverride != nil {
			ys.SignalOverride = &yamlSignalSource{Kind: st.SignalOverride.Kind}
		}
		y.Steps = append(y.Steps, ys)
	}
	return yaml.Marshal(y)
}

// UnmarshalStrategyYAML parses YAML bytes into a strategy (scope/ID left empty).
func UnmarshalStrategyYAML(data []byte) (*models.Strategy, error) {
	var y yamlStrategy
	if err := yaml.Unmarshal(data, &y); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	s := &models.Strategy{
		Name: y.Name, Description: y.Description,
		TargetType:               models.TargetType(y.TargetType),
		DefaultHealthThreshold:   y.DefaultHealthThreshold,
		DefaultRollbackOnFailure: y.DefaultRollbackOnFailure,
	}
	for i, ys := range y.Steps {
		st, err := yamlStepToStep(ys, i)
		if err != nil {
			return nil, err
		}
		s.Steps = append(s.Steps, st)
	}
	return s, nil
}

func yamlStepToStep(ys yamlStep, idx int) (models.Step, error) {
	minD, err := parseDur(ys.MinDuration, "min_duration", idx)
	if err != nil {
		return models.Step{}, err
	}
	maxD, err := parseDur(ys.MaxDuration, "max_duration", idx)
	if err != nil {
		return models.Step{}, err
	}
	bake, err := parseDur(ys.BakeTimeHealthy, "bake_time_healthy", idx)
	if err != nil {
		return models.Step{}, err
	}
	st := models.Step{
		Percent: ys.Percent, MinDuration: minD, MaxDuration: maxD, BakeTimeHealthy: bake,
		HealthThreshold: ys.HealthThreshold,
	}
	if ys.Approval != nil {
		d, err := parseDur(ys.Approval.Timeout, "approval.timeout", idx)
		if err != nil {
			return models.Step{}, err
		}
		st.Approval = &models.StepApproval{RequiredRole: ys.Approval.RequiredRole, Timeout: d}
	}
	if ys.Notify != nil {
		st.Notify = &models.StepNotify{OnEntry: ys.Notify.OnEntry, OnExit: ys.Notify.OnExit}
	}
	for j, ac := range ys.AbortConditions {
		w, err := parseDur(ac.Window, fmt.Sprintf("abort_conditions[%d].window", j), idx)
		if err != nil {
			return models.Step{}, err
		}
		st.AbortConditions = append(st.AbortConditions, models.StepAbortCondition{
			Metric: ac.Metric, Operator: ac.Operator, Threshold: ac.Threshold, Window: w,
		})
	}
	if ys.SignalOverride != nil {
		st.SignalOverride = &models.SignalSource{Kind: ys.SignalOverride.Kind}
	}
	return st, nil
}

func parseDur(s, field string, idx int) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("step %d: %s: %w", idx, field, err)
	}
	return d, nil
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/rollout/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/yaml.go internal/rollout/yaml_test.go
git commit -m "feat(rollout): YAML import/export for strategy templates"
```

---

## Task 11: HTTP handler — strategies CRUD + YAML

**Files:**
- Create: `internal/rollout/handler.go`
- Create: `internal/rollout/handler_test.go`

- [ ] **Step 1: Write failing handler tests (table-driven httptest)**

Create `internal/rollout/handler_test.go`:

```go
package rollout

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// newTestHandler wires a handler with fake repos for unit tests.
func newTestHandler(t *testing.T) (*Handler, *fakeStratRepo, uuid.UUID) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	stratRepo := newFakeStratRepo()
	defaultsRepo := newFakeDefaultsRepo()
	policyRepo := newFakePolicyRepo()
	resolver := &fakeOrgResolver{orgID: uuid.New()}
	h := NewHandler(
		NewStrategyService(stratRepo, nil),
		NewStrategyDefaultService(defaultsRepo),
		NewRolloutPolicyService(policyRepo),
		resolver,
	)
	return h, stratRepo, resolver.orgID
}

type fakeOrgResolver struct{ orgID uuid.UUID }

func (f *fakeOrgResolver) ResolveOrg(c *gin.Context) (uuid.UUID, error)       { return f.orgID, nil }
func (f *fakeOrgResolver) ResolveProject(c *gin.Context) (uuid.UUID, uuid.UUID, error) {
	return f.orgID, uuid.New(), nil
}
func (f *fakeOrgResolver) ResolveApp(c *gin.Context) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	return f.orgID, uuid.New(), uuid.New(), nil
}

func TestHandler_CreateStrategy_OK(t *testing.T) {
	h, _, _ := newTestHandler(t)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	body, _ := json.Marshal(map[string]any{
		"name": "std", "target_type": "deploy",
		"default_health_threshold": 0.95, "default_rollback_on_failure": true,
		"steps": []map[string]any{
			{"percent": 1, "min_duration": int64(time.Minute), "max_duration": int64(time.Minute), "bake_time_healthy": 0},
			{"percent": 100, "min_duration": 0, "max_duration": 0, "bake_time_healthy": 0},
		},
	})
	req := httptest.NewRequest("POST", "/api/v1/orgs/acme/strategies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandler_CreateStrategy_ValidationError(t *testing.T) {
	h, _, _ := newTestHandler(t)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	body, _ := json.Marshal(map[string]any{"name": "", "target_type": "deploy"})
	req := httptest.NewRequest("POST", "/api/v1/orgs/acme/strategies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestHandler_List_ReturnsEffective(t *testing.T) {
	h, repo, orgID := newTestHandler(t)
	_ = repo.Create(nil, &models.Strategy{
		ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "std", TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95, DefaultRollbackOnFailure: true,
	})
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	req := httptest.NewRequest("GET", "/api/v1/orgs/acme/strategies", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []struct {
			Strategy    models.Strategy `json:"strategy"`
			OriginScope ScopeRef        `json:"origin_scope"`
		} `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Items) != 1 || resp.Items[0].Strategy.Name != "std" {
		t.Fatalf("unexpected list: %s", w.Body.String())
	}
}

// fakes for defaults + policies (unused in strategy tests but needed by NewHandler)

type fakeDefaultsRepo struct{ rows []*models.StrategyDefault }

func newFakeDefaultsRepo() *fakeDefaultsRepo { return &fakeDefaultsRepo{} }
func (f *fakeDefaultsRepo) Upsert(_ ctxType, d *models.StrategyDefault) error {
	for _, r := range f.rows {
		if r.ScopeType == d.ScopeType && r.ScopeID == d.ScopeID &&
			ptrEq(r.Environment, d.Environment) && ttPtrEq(r.TargetType, d.TargetType) {
			r.StrategyID = d.StrategyID
			return nil
		}
	}
	d.ID = uuid.New()
	f.rows = append(f.rows, d)
	return nil
}
func (f *fakeDefaultsRepo) ListByScope(_ ctxType, st models.ScopeType, sid uuid.UUID) ([]*models.StrategyDefault, error) {
	var out []*models.StrategyDefault
	for _, r := range f.rows {
		if r.ScopeType == st && r.ScopeID == sid {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeDefaultsRepo) Delete(_ ctxType, id uuid.UUID) error {
	for i, r := range f.rows {
		if r.ID == id {
			f.rows = append(f.rows[:i], f.rows[i+1:]...)
			return nil
		}
	}
	return nil
}
func (f *fakeDefaultsRepo) DeleteByKey(_ ctxType, st models.ScopeType, sid uuid.UUID, env *string, tt *models.TargetType) error {
	for i, r := range f.rows {
		if r.ScopeType == st && r.ScopeID == sid && ptrEq(r.Environment, env) && ttPtrEq(r.TargetType, tt) {
			f.rows = append(f.rows[:i], f.rows[i+1:]...)
			return nil
		}
	}
	return nil
}

type fakePolicyRepo struct{ rows []*models.RolloutPolicy }

func newFakePolicyRepo() *fakePolicyRepo { return &fakePolicyRepo{} }
func (f *fakePolicyRepo) Upsert(_ ctxType, p *models.RolloutPolicy) error {
	for _, r := range f.rows {
		if r.ScopeType == p.ScopeType && r.ScopeID == p.ScopeID &&
			ptrEq(r.Environment, p.Environment) && ttPtrEq(r.TargetType, p.TargetType) {
			r.Enabled, r.Policy = p.Enabled, p.Policy
			return nil
		}
	}
	p.ID = uuid.New()
	f.rows = append(f.rows, p)
	return nil
}
func (f *fakePolicyRepo) ListByScope(_ ctxType, st models.ScopeType, sid uuid.UUID) ([]*models.RolloutPolicy, error) {
	var out []*models.RolloutPolicy
	for _, r := range f.rows {
		if r.ScopeType == st && r.ScopeID == sid {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakePolicyRepo) Delete(_ ctxType, id uuid.UUID) error {
	for i, r := range f.rows {
		if r.ID == id {
			f.rows = append(f.rows[:i], f.rows[i+1:]...)
			return nil
		}
	}
	return nil
}

type ctxType = any // tests do not need a real context

func ptrEq(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
func ttPtrEq(a, b *models.TargetType) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
```

> **Note**: the fake repos above use `context.Context` as `any` purely to keep the test-only signature terse. Real repos keep the `context.Context` parameter. When implementing, the handler signature uses real `context.Context`; the fakes compile because the interface methods accept `context.Context` which is assignable to `any`.

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/rollout/ -run TestHandler -v
```

Expected: compile error — `Handler` undefined.

- [ ] **Step 3: Implement the handler**

Create `internal/rollout/handler.go`:

```go
package rollout

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ScopeResolver resolves URL slugs (orgSlug, projectSlug, appSlug) to scope IDs.
// The concrete implementation wraps the existing entities package at cmd/api wiring.
type ScopeResolver interface {
	ResolveOrg(c *gin.Context) (orgID uuid.UUID, err error)
	ResolveProject(c *gin.Context) (orgID, projectID uuid.UUID, err error)
	ResolveApp(c *gin.Context) (orgID, projectID, appID uuid.UUID, err error)
}

// Handler serves strategies, strategy-defaults, and rollout-policy endpoints.
type Handler struct {
	strategies *StrategyService
	defaults   *StrategyDefaultService
	policies   *RolloutPolicyService
	scope      ScopeResolver
}

// NewHandler builds a Handler.
func NewHandler(strat *StrategyService, defs *StrategyDefaultService, pols *RolloutPolicyService, scope ScopeResolver) *Handler {
	return &Handler{strategies: strat, defaults: defs, policies: pols, scope: scope}
}

// RegisterRoutes attaches all rollout endpoints to the given group.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	// Org scope
	org := api.Group("/orgs/:orgSlug")
	h.registerStrategyRoutes(org, "org")
	h.registerPolicyRoutes(org, "org")
	h.registerDefaultsRoutes(org, "org")
	// Project scope
	proj := api.Group("/orgs/:orgSlug/projects/:projectSlug")
	h.registerStrategyRoutes(proj, "project")
	h.registerPolicyRoutes(proj, "project")
	h.registerDefaultsRoutes(proj, "project")
	// App scope
	app := api.Group("/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug")
	h.registerStrategyRoutes(app, "app")
	h.registerPolicyRoutes(app, "app")
	h.registerDefaultsRoutes(app, "app")
}

func (h *Handler) registerStrategyRoutes(g *gin.RouterGroup, scope string) {
	g.GET("/strategies", h.listStrategies(scope))
	g.POST("/strategies", h.createStrategy(scope))
	g.GET("/strategies/:name", h.getStrategy(scope))
	g.PUT("/strategies/:name", h.updateStrategy(scope))
	g.DELETE("/strategies/:name", h.deleteStrategy(scope))
	g.POST("/strategies/import", h.importStrategy(scope))
	g.GET("/strategies/:name/export", h.exportStrategy(scope))
}

func (h *Handler) registerPolicyRoutes(g *gin.RouterGroup, scope string) {
	g.GET("/rollout-policy", h.listPolicies(scope))
	g.PUT("/rollout-policy", h.putPolicy(scope))
}

func (h *Handler) registerDefaultsRoutes(g *gin.RouterGroup, scope string) {
	g.GET("/strategy-defaults", h.listDefaults(scope))
	g.PUT("/strategy-defaults", h.putDefault(scope))
	g.DELETE("/strategy-defaults/:id", h.deleteDefault(scope))
}

// --- helpers ---

func (h *Handler) resolveLeaf(c *gin.Context, scope string) (ScopeRef, *uuid.UUID, *uuid.UUID, error) {
	switch scope {
	case "org":
		orgID, err := h.scope.ResolveOrg(c)
		if err != nil {
			return ScopeRef{}, nil, nil, err
		}
		return ScopeRef{models.ScopeOrg, orgID}, nil, nil, nil
	case "project":
		orgID, projID, err := h.scope.ResolveProject(c)
		if err != nil {
			return ScopeRef{}, nil, nil, err
		}
		return ScopeRef{models.ScopeProject, projID}, &projID, &orgID, nil
	case "app":
		orgID, projID, appID, err := h.scope.ResolveApp(c)
		if err != nil {
			return ScopeRef{}, nil, nil, err
		}
		return ScopeRef{models.ScopeApp, appID}, &projID, &orgID, nil
	}
	return ScopeRef{}, nil, nil, errors.New("unknown scope")
}

// --- strategies handlers ---

type createStrategyReq struct {
	Name                     string        `json:"name"`
	Description              string        `json:"description"`
	TargetType               string        `json:"target_type"`
	Steps                    []models.Step `json:"steps"`
	DefaultHealthThreshold   float64       `json:"default_health_threshold"`
	DefaultRollbackOnFailure bool          `json:"default_rollback_on_failure"`
}

func (h *Handler) createStrategy(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var req createStrategyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		s := &models.Strategy{
			ScopeType: leaf.Type, ScopeID: leaf.ID,
			Name: req.Name, Description: req.Description,
			TargetType:               models.TargetType(req.TargetType),
			Steps:                    req.Steps,
			DefaultHealthThreshold:   req.DefaultHealthThreshold,
			DefaultRollbackOnFailure: req.DefaultRollbackOnFailure,
		}
		if err := h.strategies.Create(c.Request.Context(), s); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, s)
	}
}

func (h *Handler) listStrategies(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, projID, orgID, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		items, err := h.strategies.EffectiveList(c.Request.Context(), leaf, projID, orgID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	}
}

func (h *Handler) getStrategy(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		s, err := h.strategies.GetByName(c.Request.Context(), leaf.Type, leaf.ID, c.Param("name"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, s)
	}
}

type updateStrategyReq struct {
	Description              string        `json:"description"`
	TargetType               string        `json:"target_type"`
	Steps                    []models.Step `json:"steps"`
	DefaultHealthThreshold   float64       `json:"default_health_threshold"`
	DefaultRollbackOnFailure bool          `json:"default_rollback_on_failure"`
	ExpectedVersion          int           `json:"expected_version"`
}

func (h *Handler) updateStrategy(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		existing, err := h.strategies.GetByName(c.Request.Context(), leaf.Type, leaf.ID, c.Param("name"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		var req updateStrategyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		existing.Description = req.Description
		existing.TargetType = models.TargetType(req.TargetType)
		existing.Steps = req.Steps
		existing.DefaultHealthThreshold = req.DefaultHealthThreshold
		existing.DefaultRollbackOnFailure = req.DefaultRollbackOnFailure
		if err := h.strategies.Update(c.Request.Context(), existing, req.ExpectedVersion); err != nil {
			if errors.Is(err, ErrSystemStrategyImmutable) {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, existing)
	}
}

func (h *Handler) deleteStrategy(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		existing, err := h.strategies.GetByName(c.Request.Context(), leaf.Type, leaf.ID, c.Param("name"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err := h.strategies.Delete(c.Request.Context(), existing.ID); err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, ErrSystemStrategyImmutable) {
				status = http.StatusForbidden
			} else if errors.Is(err, ErrStrategyInUse) {
				status = http.StatusConflict
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func (h *Handler) importStrategy(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		s, err := UnmarshalStrategyYAML(body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		s.ScopeType, s.ScopeID = leaf.Type, leaf.ID
		existing, err := h.strategies.GetByName(c.Request.Context(), leaf.Type, leaf.ID, s.Name)
		if err == nil && existing != nil {
			// Update path: carry over existing ID+version.
			existing.Description = s.Description
			existing.TargetType = s.TargetType
			existing.Steps = s.Steps
			existing.DefaultHealthThreshold = s.DefaultHealthThreshold
			existing.DefaultRollbackOnFailure = s.DefaultRollbackOnFailure
			if err := h.strategies.Update(c.Request.Context(), existing, existing.Version); err != nil {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, existing)
			return
		}
		if err := h.strategies.Create(c.Request.Context(), s); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, s)
	}
}

func (h *Handler) exportStrategy(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		s, err := h.strategies.GetByName(c.Request.Context(), leaf.Type, leaf.ID, c.Param("name"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		b, err := MarshalStrategyYAML(s)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Data(http.StatusOK, "application/yaml", b)
	}
}

// --- policies handlers ---

type putPolicyReq struct {
	Environment *string `json:"environment,omitempty"`
	TargetType  *string `json:"target_type,omitempty"`
	Enabled     bool    `json:"enabled"`
	Policy      string  `json:"policy"`
}

func (h *Handler) listPolicies(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		rows, err := h.policies.List(c.Request.Context(), leaf.Type, leaf.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": rows})
	}
}

func (h *Handler) putPolicy(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var req putPolicyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		pk := models.PolicyKind(req.Policy)
		switch pk {
		case models.PolicyOff, models.PolicyPrompt, models.PolicyMandate:
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "policy must be off, prompt, or mandate"})
			return
		}
		var ttPtr *models.TargetType
		if req.TargetType != nil {
			tt := models.TargetType(*req.TargetType)
			ttPtr = &tt
		}
		p := &models.RolloutPolicy{
			ScopeType: leaf.Type, ScopeID: leaf.ID,
			Environment: req.Environment, TargetType: ttPtr,
			Enabled: req.Enabled, Policy: pk,
		}
		if err := h.policies.Upsert(c.Request.Context(), p); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, p)
	}
}

// --- defaults handlers ---

type putDefaultReq struct {
	Environment  *string   `json:"environment,omitempty"`
	TargetType   *string   `json:"target_type,omitempty"`
	StrategyName string    `json:"strategy_name"` // friendlier than StrategyID in write path
	StrategyID   uuid.UUID `json:"strategy_id,omitempty"`
}

func (h *Handler) listDefaults(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		rows, err := h.defaults.List(c.Request.Context(), leaf.Type, leaf.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": rows})
	}
}

func (h *Handler) putDefault(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var req putDefaultReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		sid := req.StrategyID
		if sid == uuid.Nil {
			s, err := h.strategies.GetByName(c.Request.Context(), leaf.Type, leaf.ID, req.StrategyName)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "strategy not found in scope"})
				return
			}
			sid = s.ID
		}
		var ttPtr *models.TargetType
		if req.TargetType != nil {
			tt := models.TargetType(*req.TargetType)
			ttPtr = &tt
		}
		d := &models.StrategyDefault{
			ScopeType: leaf.Type, ScopeID: leaf.ID,
			Environment: req.Environment, TargetType: ttPtr,
			StrategyID: sid,
		}
		if err := h.defaults.Upsert(c.Request.Context(), d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, d)
	}
}

func (h *Handler) deleteDefault(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		if err := h.defaults.Delete(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// unused placeholder to avoid import lint on context (used by wrapper packages);
// all handlers route through c.Request.Context() directly.
var _ = context.Background
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/rollout/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/handler.go internal/rollout/handler_test.go
git commit -m "feat(rollout): HTTP handler for strategies, defaults, policies"
```

---

## Task 12: Wire routes in cmd/api

**Files:**
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Read the surrounding wiring to pick insertion point**

```bash
grep -n "entities.NewHandler\|settings.NewHandler\|rollout" cmd/api/main.go | head -5
```

Expected: existing lines showing where other handlers are registered; no `rollout` matches yet.

- [ ] **Step 2: Add the import**

In `cmd/api/main.go`, add to the import block (alphabetical placement within the `deploysentry/internal/...` group):

```go
"github.com/deploysentry/deploysentry/internal/rollout"
```

- [ ] **Step 3: Construct the services and resolver, register routes**

Find the block that registers other handlers (after `members.NewHandler(...)` and before the rollback wiring block) and insert:

```go
// Rollout control plane: strategies, defaults, onboarding policies.
strategyRepo := postgres.NewStrategyRepo(db.Pool)
strategyDefRepo := postgres.NewStrategyDefaultsRepo(db.Pool)
rolloutPolicyRepo := postgres.NewRolloutPolicyRepo(db.Pool)
rolloutScopeResolver := &rolloutScopeAdapter{entities: entityRepo, envs: envRepo}
rollout.NewHandler(
    rollout.NewStrategyService(strategyRepo, nil),
    rollout.NewStrategyDefaultService(strategyDefRepo),
    rollout.NewRolloutPolicyService(rolloutPolicyRepo),
    rolloutScopeResolver,
).RegisterRoutes(api)
```

- [ ] **Step 4: Add the slug→ID adapter**

Append to the bottom of `cmd/api/main.go` (near the existing `deployServiceRollbackExecutor` adapter):

```go
// rolloutScopeAdapter bridges rollout.ScopeResolver to the existing entity and
// environment repositories. It reads path params set by gin.
type rolloutScopeAdapter struct {
    entities entities.EntityRepository
    envs     environments.EnvironmentRepository // or the actual env repo type in this codebase
}

func (a *rolloutScopeAdapter) ResolveOrg(c *gin.Context) (uuid.UUID, error) {
    org, err := a.entities.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
    if err != nil {
        return uuid.Nil, err
    }
    return org.ID, nil
}
func (a *rolloutScopeAdapter) ResolveProject(c *gin.Context) (uuid.UUID, uuid.UUID, error) {
    orgID, err := a.ResolveOrg(c)
    if err != nil {
        return uuid.Nil, uuid.Nil, err
    }
    proj, err := a.entities.GetProjectBySlug(c.Request.Context(), orgID, c.Param("projectSlug"))
    if err != nil {
        return uuid.Nil, uuid.Nil, err
    }
    return orgID, proj.ID, nil
}
func (a *rolloutScopeAdapter) ResolveApp(c *gin.Context) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
    orgID, projID, err := a.ResolveProject(c)
    if err != nil {
        return uuid.Nil, uuid.Nil, uuid.Nil, err
    }
    app, err := a.entities.GetAppBySlug(c.Request.Context(), projID, c.Param("appSlug"))
    if err != nil {
        return uuid.Nil, uuid.Nil, uuid.Nil, err
    }
    return orgID, projID, app.ID, nil
}
```

> **Note**: if the exact method names on `entities.EntityRepository` differ (e.g., `GetOrgBySlug` is named `OrgBySlug`), match the existing repo — do not rename repo methods. Run `grep -n "GetOrgBySlug\|OrgBySlug\|ProjectBySlug" internal/entities/` if unsure.

- [ ] **Step 5: Build and boot to verify routes print**

```bash
go build ./cmd/api
./api 2>&1 | head -200 | grep -E "ROUTE:.*(strategies|rollout-policy|strategy-defaults)"
# Ctrl-C once verified
```

Expected: 30+ `ROUTE:` lines at three scopes (org/project/app) for strategies/rollout-policy/strategy-defaults.

- [ ] **Step 6: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat(api): wire rollout strategies, defaults, and policy routes"
```

---

## Task 13: CLI — `ds strategies`

**Files:**
- Create: `cmd/cli/strategies.go`
- Modify: `cmd/cli/root.go`

- [ ] **Step 1: Read existing CLI conventions**

```bash
head -40 cmd/cli/flags.go
grep -n "rootCmd.AddCommand" cmd/cli/root.go
```

Expected: existing pattern — cobra commands created in their own file, registered in `root.go` via `rootCmd.AddCommand(...)`.

- [ ] **Step 2: Create the strategies command file**

Create `cmd/cli/strategies.go`:

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var strategiesCmd = &cobra.Command{
	Use:     "strategies",
	Aliases: []string{"strategy", "strat"},
	Short:   "Manage rollout strategy templates",
}

var strategiesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List strategies visible at the current scope (org/project/app)",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath(cmd)
		if err != nil {
			return err
		}
		body, err := apiGET(path + "/strategies")
		if err != nil {
			return err
		}
		var resp struct {
			Items []struct {
				Strategy struct {
					Name       string `json:"name"`
					TargetType string `json:"target_type"`
					Version    int    `json:"version"`
				} `json:"strategy"`
				OriginScope struct {
					Type string `json:"type"`
				} `json:"origin_scope"`
				IsInherited bool `json:"is_inherited"`
			} `json:"items"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return err
		}
		fmt.Printf("%-30s %-10s %-8s %-12s\n", "NAME", "TARGET", "VERSION", "ORIGIN")
		for _, it := range resp.Items {
			origin := it.OriginScope.Type
			if it.IsInherited {
				origin += " (inh)"
			}
			fmt.Printf("%-30s %-10s %-8d %-12s\n", it.Strategy.Name, it.Strategy.TargetType, it.Strategy.Version, origin)
		}
		return nil
	},
}

var strategiesGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Fetch a strategy by name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath(cmd)
		if err != nil {
			return err
		}
		body, err := apiGET(path + "/strategies/" + args[0])
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

var strategiesApplyCmd = &cobra.Command{
	Use:   "apply -f <file.yaml>",
	Short: "Create or update a strategy from YAML",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath(cmd)
		if err != nil {
			return err
		}
		file, _ := cmd.Flags().GetString("file")
		if file == "" {
			return fmt.Errorf("--file is required")
		}
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		body, err := apiPOSTRaw(path+"/strategies/import", "application/yaml", b)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	},
}

var strategiesDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a strategy (blocked if referenced)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath(cmd)
		if err != nil {
			return err
		}
		return apiDELETE(path + "/strategies/" + args[0])
	},
}

var strategiesExportCmd = &cobra.Command{
	Use:   "export <name>",
	Short: "Export a strategy as YAML",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath(cmd)
		if err != nil {
			return err
		}
		body, err := apiGET(path + "/strategies/" + args[0] + "/export")
		if err != nil {
			return err
		}
		out, _ := cmd.Flags().GetString("out")
		if out == "" {
			fmt.Print(string(body))
			return nil
		}
		return os.WriteFile(out, body, 0o644)
	},
}

// strategyScopePath returns the URL prefix for the current scope flags.
// Flags: --org, --project, --app. If only --org is set, scope is org.
// If --project is set, scope is project. If --app is set, scope is app.
func strategyScopePath(cmd *cobra.Command) (string, error) {
	org, _ := cmd.Flags().GetString("org")
	proj, _ := cmd.Flags().GetString("project")
	app, _ := cmd.Flags().GetString("app")
	if org == "" {
		return "", fmt.Errorf("--org is required")
	}
	path := "/api/v1/orgs/" + org
	if proj != "" {
		path += "/projects/" + proj
	}
	if app != "" {
		if proj == "" {
			return "", fmt.Errorf("--project is required when --app is given")
		}
		path += "/apps/" + app
	}
	return path, nil
}

func init() {
	for _, c := range []*cobra.Command{strategiesListCmd, strategiesGetCmd, strategiesApplyCmd, strategiesDeleteCmd, strategiesExportCmd} {
		c.Flags().String("org", "", "Organization slug (required)")
		c.Flags().String("project", "", "Project slug (optional, for project scope)")
		c.Flags().String("app", "", "Application slug (optional, for app scope)")
	}
	strategiesApplyCmd.Flags().StringP("file", "f", "", "YAML file to apply")
	strategiesExportCmd.Flags().String("out", "", "Write to file (default: stdout)")
	strategiesCmd.AddCommand(strategiesListCmd, strategiesGetCmd, strategiesApplyCmd, strategiesDeleteCmd, strategiesExportCmd)
}
```

- [ ] **Step 3: Register in `root.go`**

Find the `rootCmd.AddCommand(...)` block in `cmd/cli/root.go` and append:

```go
rootCmd.AddCommand(strategiesCmd)
```

- [ ] **Step 4: Confirm `apiGET`, `apiPOSTRaw`, `apiDELETE` helpers exist**

```bash
grep -n "func apiGET\|func apiPOSTRaw\|func apiDELETE" cmd/cli/client.go
```

Expected: helpers defined. If any helper is missing, add it to `client.go` following the pattern of existing helpers like `apiPOST`. For example:

```go
func apiPOSTRaw(path, contentType string, body []byte) ([]byte, error) {
    req, err := http.NewRequest("POST", apiURL()+path, bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", contentType)
    return doRequest(req)
}

func apiDELETE(path string) error {
    req, err := http.NewRequest("DELETE", apiURL()+path, nil)
    if err != nil {
        return err
    }
    _, err = doRequest(req)
    return err
}
```

- [ ] **Step 5: Build and smoke test**

```bash
go build ./cmd/cli
./cli strategies list --help
./cli strategies list --org acme
```

Expected: help text prints; the second call hits the API (will 404 or list depending on auth/test data).

- [ ] **Step 6: Commit**

```bash
git add cmd/cli/strategies.go cmd/cli/root.go cmd/cli/client.go
git commit -m "feat(cli): ds strategies {list,get,apply,delete,export}"
```

---

## Task 14: CLI — `ds rollout-policy` and `ds strategy-defaults`

**Files:**
- Create: `cmd/cli/rolloutpolicy.go`
- Modify: `cmd/cli/root.go`

- [ ] **Step 1: Create the commands file**

Create `cmd/cli/rolloutpolicy.go`:

```go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var rolloutPolicyCmd = &cobra.Command{
	Use:     "rollout-policy",
	Aliases: []string{"rp"},
	Short:   "Manage rollout onboarding + mandate policy per scope",
}

var rolloutPolicyGetCmd = &cobra.Command{
	Use:   "get",
	Short: "List policy rows defined at the scope",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath(cmd)
		if err != nil {
			return err
		}
		body, err := apiGET(path + "/rollout-policy")
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	},
}

var rolloutPolicySetCmd = &cobra.Command{
	Use:   "set",
	Short: "Upsert a policy row (off|prompt|mandate) for a scope",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath(cmd)
		if err != nil {
			return err
		}
		policy, _ := cmd.Flags().GetString("policy")
		enabled, _ := cmd.Flags().GetBool("enabled")
		env, _ := cmd.Flags().GetString("env")
		target, _ := cmd.Flags().GetString("target")
		payload := map[string]any{"enabled": enabled, "policy": policy}
		if env != "" {
			payload["environment"] = env
		}
		if target != "" {
			payload["target_type"] = target
		}
		b, _ := json.Marshal(payload)
		body, err := apiPUT(path+"/rollout-policy", b)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	},
}

var strategyDefaultsCmd = &cobra.Command{
	Use:     "strategy-defaults",
	Aliases: []string{"sd"},
	Short:   "Manage default strategy assignments per (scope, env, target)",
}

var strategyDefaultsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List default strategy rows defined at the scope",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath(cmd)
		if err != nil {
			return err
		}
		body, err := apiGET(path + "/strategy-defaults")
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	},
}

var strategyDefaultsSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Upsert a default strategy assignment",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath(cmd)
		if err != nil {
			return err
		}
		strategy, _ := cmd.Flags().GetString("strategy")
		env, _ := cmd.Flags().GetString("env")
		target, _ := cmd.Flags().GetString("target")
		payload := map[string]any{"strategy_name": strategy}
		if env != "" {
			payload["environment"] = env
		}
		if target != "" {
			payload["target_type"] = target
		}
		b, _ := json.Marshal(payload)
		body, err := apiPUT(path+"/strategy-defaults", b)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	},
}

func init() {
	for _, c := range []*cobra.Command{rolloutPolicyGetCmd, rolloutPolicySetCmd, strategyDefaultsListCmd, strategyDefaultsSetCmd} {
		c.Flags().String("org", "", "Organization slug (required)")
		c.Flags().String("project", "", "Project slug (optional)")
		c.Flags().String("app", "", "Application slug (optional)")
	}
	rolloutPolicySetCmd.Flags().String("policy", "off", "Policy: off|prompt|mandate")
	rolloutPolicySetCmd.Flags().Bool("enabled", true, "Enable rollout control on this scope")
	rolloutPolicySetCmd.Flags().String("env", "", "Environment name (optional narrowing)")
	rolloutPolicySetCmd.Flags().String("target", "", "deploy|config (optional narrowing)")

	strategyDefaultsSetCmd.Flags().String("strategy", "", "Strategy name (required)")
	strategyDefaultsSetCmd.Flags().String("env", "", "Environment name (optional)")
	strategyDefaultsSetCmd.Flags().String("target", "", "deploy|config (optional)")
	_ = strategyDefaultsSetCmd.MarkFlagRequired("strategy")

	rolloutPolicyCmd.AddCommand(rolloutPolicyGetCmd, rolloutPolicySetCmd)
	strategyDefaultsCmd.AddCommand(strategyDefaultsListCmd, strategyDefaultsSetCmd)
}
```

- [ ] **Step 2: Register in `root.go`**

Append to the same `AddCommand` block in `cmd/cli/root.go`:

```go
rootCmd.AddCommand(rolloutPolicyCmd, strategyDefaultsCmd)
```

- [ ] **Step 3: Confirm `apiPUT` helper exists**

```bash
grep -n "func apiPUT" cmd/cli/client.go
```

If missing, add next to `apiPOST`:

```go
func apiPUT(path string, body []byte) ([]byte, error) {
    req, err := http.NewRequest("PUT", apiURL()+path, bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")
    return doRequest(req)
}
```

- [ ] **Step 4: Build and smoke test**

```bash
go build ./cmd/cli
./cli rollout-policy --help
./cli strategy-defaults --help
```

Expected: help text for both.

- [ ] **Step 5: Commit**

```bash
git add cmd/cli/rolloutpolicy.go cmd/cli/root.go cmd/cli/client.go
git commit -m "feat(cli): ds rollout-policy and ds strategy-defaults"
```

---

## Task 15: Documentation

**Files:**
- Create: `docs/Rollout_Strategies.md`
- Modify: `docs/Traffic_Management_Guide.md`
- Modify: `docs/CURRENT_INITIATIVES.md`

- [ ] **Step 1: Create the user-facing reference**

Create `docs/Rollout_Strategies.md`:

```markdown
# Rollout Strategies

**Phase**: Implementation

## Overview

Rollout strategies are reusable templates describing how a change should be applied progressively — the traffic/rollout percentage at each step, how long each step holds, what conditions promote or abort it. Strategies can be attached to a deployment or a flag/config change (coming in a later plan). This document covers the template management surface added in the foundation plan.

## Scope hierarchy

Strategies can be defined at three scopes: **org**, **project**, or **app**. When listing strategies for a scope, ancestors contribute too — an app sees its own + its project's + its org's strategies. A name defined at a more specific scope shadows the ancestor's.

## Step fields

Each step in a strategy is an object with the following fields:

- `percent` (0–100, required) — traffic weight (deploy) or rollout % (config).
- `min_duration` (duration, required) — phase cannot advance before this.
- `max_duration` (duration, required) — if still unhealthy at max, rollback.
- `bake_time_healthy` (duration, required) — advance once healthy for this long.
- `health_threshold` (0–1, optional) — override strategy default for this phase.
- `approval` (optional) — `{ required_role, timeout }`; pauses until granted.
- `notify` (optional) — `{ on_entry: [channel], on_exit: [channel] }`.
- `abort_conditions` (optional) — list of `{ metric, operator, threshold, window }` rules that roll back immediately if tripped.

Durations use Go's `time.ParseDuration` syntax: `30s`, `5m`, `1h`.

## CLI examples

```
ds strategies list --org acme
ds strategies apply --org acme -f ./prod-canary.yaml
ds strategies export --org acme prod-canary --out ./prod-canary.yaml
```

## Policy & defaults

```
ds rollout-policy set --org acme --enabled --policy prompt
ds strategy-defaults set --org acme --env prod --target deploy --strategy prod-canary
```

Policy `prompt` means the UI/CLI will prompt for a strategy on changes. `mandate` requires one. `off` keeps immediate-apply behavior (backward compatible).

## YAML example

```yaml
name: prod-canary-standard
target_type: deploy
default_health_threshold: 0.95
default_rollback_on_failure: true
steps:
  - percent: 1
    min_duration: 5m
    max_duration: 30m
    bake_time_healthy: 5m
  - percent: 25
    min_duration: 10m
    max_duration: 1h
    bake_time_healthy: 10m
  - percent: 100
    min_duration: 0s
    max_duration: 0s
    bake_time_healthy: 0s
```

## What's coming

The foundation plan ships template management only. Future plans wire strategies into live deploy and config rollouts, add runtime controls (pause/resume/promote/rollback/approve/force-promote), add Release bundles, and build the web UI.

See `docs/superpowers/specs/2026-04-18-configurable-rollout-strategies-design.md` for the full design.
```

- [ ] **Step 2: Cross-link from the traffic guide**

In `docs/Traffic_Management_Guide.md`, add a "Related" bullet near the top (after the overview paragraph):

```markdown
- [Rollout Strategies](./Rollout_Strategies.md) — reusable templates that describe how traffic is shifted step by step during a deploy or config rollout.
```

- [ ] **Step 3: Update `CURRENT_INITIATIVES.md`**

Add a row to the table in `docs/CURRENT_INITIATIVES.md`:

```markdown
| Configurable Rollout Strategies (Foundation) | Implementation | [spec](./superpowers/specs/2026-04-18-configurable-rollout-strategies-design.md) / [plan](./superpowers/plans/2026-04-18-rollout-strategies-foundation.md) |
```

- [ ] **Step 4: Run full test suite**

```bash
make test
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add docs/Rollout_Strategies.md docs/Traffic_Management_Guide.md docs/CURRENT_INITIATIVES.md
git commit -m "docs: add Rollout_Strategies reference and initiative entry"
```

---

## Definition of Done

- All 15 tasks committed individually.
- `make test` green.
- `go build ./...` clean.
- New routes visible in startup `ROUTE:` log.
- CLI smoke tests successful: `ds strategies list`, `ds rollout-policy get`, `ds strategy-defaults list` — each reaches the API with `--org <slug>` and returns expected shape.
- `docs/Rollout_Strategies.md` created; `CURRENT_INITIATIVES.md` updated.
- No code references rollout engine / applicator / rollout entity (those are Plan 2).

## Not in scope (deferred to Plan 2)

- `internal/rollout/engine/` — phase engine loop.
- `internal/rollout/applicator/` — deploy and config applicators.
- `rollouts`, `rollout_phases`, `rollout_events`, `releases` tables.
- Policy enforcement in deploy/flag PUT paths.
- System strategy seeding.
- Runtime operator control endpoints.

## Self-Review Notes

- Spec coverage: this plan covers spec sections "Data Model: strategies / strategy_defaults / rollout_policies" (subset), "Strategy Schema", "API: strategies + policies + defaults (subset)", "CLI: ds strategies + ds rollout-policy + ds strategy-defaults". Spec sections deferred: rollouts/phases/events/releases tables, phase engine, applicators, 409 on extended deploy/flag endpoints, six runtime controls, system strategy seeding, web UI.
- Type consistency: `Strategy`, `Step`, `TargetType`, `ScopeType`, `PolicyKind` names are consistent across all tasks. `StrategyRepository` (interface) vs `StrategyRepo` (Postgres struct) follows the codebase's existing `FlagRepository` / `FlagRepo` convention.
- Placeholder scan: no TBD/TODO/"handle edge cases" phrasing remaining. One `// Note` comment in Task 12 points to checking actual method names on the entities repo — this is a legitimate codebase reality, not a planning placeholder.
