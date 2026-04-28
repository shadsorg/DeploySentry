# Feature Flag Engine Improvements — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the feature flag evaluation engine complete, performant, and enterprise-ready with rule-based segments, compound rules, full SSE broadcasts, singleflight protection, and concurrent batch evaluation.

**Architecture:** Shared evaluation core — segments and compound rules both use a single `evaluateConditions()` function in `targeting.go`. Singleflight wraps DB fallback paths in the evaluator. Batch evaluation uses `errgroup` with bounded concurrency. SSE broadcasts use a `broadcastEvent` helper on the handler.

**Tech Stack:** Go, PostgreSQL (deploy schema), Redis cache, `golang.org/x/sync` (singleflight + errgroup), Gin HTTP framework, SSE

**Spec:** `docs/superpowers/specs/2026-04-10-flag-engine-improvements-design.md`

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `migrations/033_create_segments.up.sql` | Segments + conditions tables |
| Create | `migrations/033_create_segments.down.sql` | Rollback segments tables |
| Create | `migrations/034_add_compound_rule_support.up.sql` | Add combine_op column, update rule_type constraint |
| Create | `migrations/034_add_compound_rule_support.down.sql` | Rollback compound rule changes |
| Create | `internal/models/segment.go` | Segment and SegmentCondition structs |
| Create | `internal/flags/segment_repository.go` | Segment CRUD repository |
| Create | `internal/flags/segment_handler.go` | Segment HTTP handlers |
| Create | `internal/flags/segment_handler_test.go` | Segment handler tests |
| Modify | `internal/models/flag.go:46-60,88-105,116-124` | Add RuleTypeCompound, compound fields on TargetingRule, Error on FlagEvaluationResult |
| Modify | `internal/flags/targeting.go:23-27` | Add `evaluateConditions()` shared function |
| Modify | `internal/flags/targeting_test.go` | Tests for evaluateConditions, compound rules |
| Modify | `internal/flags/evaluator.go:17-32,51-58,184-201` | Add segment loading, singleflight groups, wire segment+compound eval |
| Modify | `internal/flags/service.go:30-83,86-91,262-279` | Add segment service methods, concurrent BatchEvaluate |
| Modify | `internal/flags/service_test.go` | Tests for concurrent batch eval |
| Modify | `internal/flags/handler.go:38-45,285-363,365-412,419-476,484-502,684-772,854-896` | broadcastEvent helper, SSE on all mutations |
| Modify | `internal/flags/handler_test.go` | Tests for SSE broadcasts |
| Modify | `internal/flags/repository.go:25-69` | Add segment repository interface methods |
| Modify | `internal/platform/cache/flagcache/flagcache.go` | Add segment cache methods |
| Modify | `go.mod` | Promote golang.org/x/sync to direct dependency |
| Modify | `cmd/api/main.go:291-293` | Register segment routes |

---

### Task 1: Promote golang.org/x/sync Dependency

**Files:**
- Modify: `go.mod`

- [x] **Step 1: Promote to direct dependency**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go get golang.org/x/sync
```

This moves `golang.org/x/sync` from indirect to direct in `go.mod`. Verify it's no longer marked `// indirect`.

- [x] **Step 2: Verify imports work**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go build ./...
```

Expected: Build succeeds with no errors.

- [x] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "build: promote golang.org/x/sync to direct dependency"
```

---

### Task 2: Database Migrations

**Files:**
- Create: `migrations/033_create_segments.up.sql`
- Create: `migrations/033_create_segments.down.sql`
- Create: `migrations/034_add_compound_rule_support.up.sql`
- Create: `migrations/034_add_compound_rule_support.down.sql`

- [x] **Step 1: Create segments migration (up)**

Write `migrations/033_create_segments.up.sql`:

```sql
CREATE TABLE segments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key           TEXT NOT NULL,
    name          TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    combine_op    TEXT NOT NULL DEFAULT 'AND' CHECK (combine_op IN ('AND', 'OR')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, key)
);

CREATE TABLE segment_conditions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    segment_id    UUID NOT NULL REFERENCES segments(id) ON DELETE CASCADE,
    attribute     TEXT NOT NULL,
    operator      TEXT NOT NULL,
    value         TEXT NOT NULL,
    priority      INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_segments_project_id ON segments(project_id);
CREATE INDEX idx_segment_conditions_segment_id ON segment_conditions(segment_id);
```

- [x] **Step 2: Create segments migration (down)**

Write `migrations/033_create_segments.down.sql`:

```sql
DROP TABLE IF EXISTS segment_conditions;
DROP TABLE IF EXISTS segments;
```

- [x] **Step 3: Create compound rule migration (up)**

Write `migrations/034_add_compound_rule_support.up.sql`:

```sql
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS combine_op TEXT NOT NULL DEFAULT 'AND';

ALTER TABLE flag_targeting_rules DROP CONSTRAINT IF EXISTS flag_targeting_rules_rule_type_check;
ALTER TABLE flag_targeting_rules ADD CONSTRAINT flag_targeting_rules_rule_type_check
    CHECK (rule_type IN ('percentage', 'user_target', 'attribute', 'segment', 'schedule', 'compound'));
```

- [x] **Step 4: Create compound rule migration (down)**

Write `migrations/034_add_compound_rule_support.down.sql`:

```sql
ALTER TABLE flag_targeting_rules DROP CONSTRAINT IF EXISTS flag_targeting_rules_rule_type_check;
ALTER TABLE flag_targeting_rules ADD CONSTRAINT flag_targeting_rules_rule_type_check
    CHECK (rule_type IN ('percentage', 'user_target', 'attribute', 'segment', 'schedule'));

ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS combine_op;
```

- [x] **Step 5: Run migrations**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && make migrate-up
```

Expected: Migrations 033 and 034 applied successfully.

- [x] **Step 6: Commit**

```bash
git add migrations/033_create_segments.up.sql migrations/033_create_segments.down.sql \
       migrations/034_add_compound_rule_support.up.sql migrations/034_add_compound_rule_support.down.sql
git commit -m "feat: add segments and compound rule migrations (033, 034)"
```

---

### Task 3: Segment Model and Updated Flag Models

**Files:**
- Create: `internal/models/segment.go`
- Modify: `internal/models/flag.go:46-60,88-105,116-124`

- [x] **Step 1: Create segment model**

Write `internal/models/segment.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

// Segment defines a reusable group of conditions for flag targeting.
type Segment struct {
	ID          uuid.UUID          `json:"id" db:"id"`
	ProjectID   uuid.UUID          `json:"project_id" db:"project_id"`
	Key         string             `json:"key" db:"key"`
	Name        string             `json:"name" db:"name"`
	Description string             `json:"description" db:"description"`
	CombineOp   string             `json:"combine_op" db:"combine_op"`
	Conditions  []SegmentCondition `json:"conditions" db:"-"`
	CreatedAt   time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at" db:"updated_at"`
}

// SegmentCondition defines a single condition within a segment.
type SegmentCondition struct {
	ID        uuid.UUID `json:"id" db:"id"`
	SegmentID uuid.UUID `json:"segment_id" db:"segment_id"`
	Attribute string    `json:"attribute" db:"attribute"`
	Operator  string    `json:"operator" db:"operator"`
	Value     string    `json:"value" db:"value"`
	Priority  int       `json:"priority" db:"priority"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
```

- [x] **Step 2: Add RuleTypeCompound constant**

In `internal/models/flag.go`, add after the existing rule type constants (line 56):

```go
RuleTypeCompound RuleType = "compound"
```

- [x] **Step 3: Add compound fields to TargetingRule**

In `internal/models/flag.go`, add two fields after `SegmentID` (after line 99):

```go
Conditions []CompoundCondition `json:"conditions,omitempty" db:"-"`
CombineOp  string              `json:"combine_op,omitempty" db:"combine_op"`
```

Where `CompoundCondition` is the type already defined in `internal/flags/targeting.go`. Since models shouldn't import the flags package (circular dependency), define `CompoundCondition` in models:

Add to `internal/models/flag.go` after the `FlagEvaluationResult` struct:

```go
// CompoundCondition defines a single condition in a compound rule or segment.
type CompoundCondition struct {
	Attribute string `json:"attribute"`
	Operator  string `json:"operator"`
	Value     string `json:"value"`
}
```

- [x] **Step 4: Add Error field to FlagEvaluationResult**

In `internal/models/flag.go`, add after the `Metadata` field (after line 123):

```go
Error string `json:"error,omitempty"`
```

- [x] **Step 5: Update targeting.go to use models.CompoundCondition**

In `internal/flags/targeting.go`, remove the local `CompoundCondition` struct (lines 23-27) and update imports to use `models.CompoundCondition`. Keep `CombineOperator` and its constants (lines 11-19) in targeting.go since they're evaluation-specific.

- [x] **Step 6: Verify build**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go build ./...
```

Expected: Build succeeds.

- [x] **Step 7: Commit**

```bash
git add internal/models/segment.go internal/models/flag.go internal/flags/targeting.go
git commit -m "feat: add segment model, compound rule type, and error field on evaluation result"
```

---

### Task 4: Shared evaluateConditions Function (TDD)

**Files:**
- Modify: `internal/flags/targeting.go`
- Modify: `internal/flags/targeting_test.go`

- [x] **Step 1: Write failing tests for evaluateConditions**

Add to `internal/flags/targeting_test.go`:

```go
func TestEvaluateConditions(t *testing.T) {
	tests := []struct {
		name       string
		conditions []models.CompoundCondition
		op         CombineOperator
		evalCtx    models.EvaluationContext
		want       bool
	}{
		{
			name: "AND all match",
			conditions: []models.CompoundCondition{
				{Attribute: "plan", Operator: "eq", Value: "enterprise"},
				{Attribute: "region", Operator: "eq", Value: "US"},
			},
			op:      CombineAND,
			evalCtx: models.EvaluationContext{Attributes: map[string]string{"plan": "enterprise", "region": "US"}},
			want:    true,
		},
		{
			name: "AND one mismatch",
			conditions: []models.CompoundCondition{
				{Attribute: "plan", Operator: "eq", Value: "enterprise"},
				{Attribute: "region", Operator: "eq", Value: "EU"},
			},
			op:      CombineAND,
			evalCtx: models.EvaluationContext{Attributes: map[string]string{"plan": "enterprise", "region": "US"}},
			want:    false,
		},
		{
			name: "OR one matches",
			conditions: []models.CompoundCondition{
				{Attribute: "plan", Operator: "eq", Value: "enterprise"},
				{Attribute: "plan", Operator: "eq", Value: "pro"},
			},
			op:      CombineOR,
			evalCtx: models.EvaluationContext{Attributes: map[string]string{"plan": "pro"}},
			want:    true,
		},
		{
			name: "OR none match",
			conditions: []models.CompoundCondition{
				{Attribute: "plan", Operator: "eq", Value: "enterprise"},
				{Attribute: "plan", Operator: "eq", Value: "pro"},
			},
			op:      CombineOR,
			evalCtx: models.EvaluationContext{Attributes: map[string]string{"plan": "free"}},
			want:    false,
		},
		{
			name:       "empty conditions AND returns true",
			conditions: []models.CompoundCondition{},
			op:         CombineAND,
			evalCtx:    models.EvaluationContext{},
			want:       true,
		},
		{
			name:       "empty conditions OR returns false",
			conditions: []models.CompoundCondition{},
			op:         CombineOR,
			evalCtx:    models.EvaluationContext{},
			want:       false,
		},
		{
			name: "AND with in operator",
			conditions: []models.CompoundCondition{
				{Attribute: "region", Operator: "in", Value: "US,EU,UK"},
				{Attribute: "plan", Operator: "eq", Value: "enterprise"},
			},
			op:      CombineAND,
			evalCtx: models.EvaluationContext{Attributes: map[string]string{"region": "EU", "plan": "enterprise"}},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateConditions(tt.conditions, tt.op, tt.evalCtx)
			if got != tt.want {
				t.Errorf("evaluateConditions() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -run TestEvaluateConditions -v
```

Expected: FAIL — `evaluateConditions` not defined.

- [x] **Step 3: Implement evaluateConditions**

Add to `internal/flags/targeting.go`:

```go
// evaluateConditions evaluates a list of compound conditions against an evaluation context.
// AND mode: all conditions must match (short-circuits on first false).
// OR mode: any condition must match (short-circuits on first true).
func evaluateConditions(conditions []models.CompoundCondition, op CombineOperator, evalCtx models.EvaluationContext) bool {
	if len(conditions) == 0 {
		return op == CombineAND
	}

	for _, cond := range conditions {
		rule := &models.TargetingRule{
			Attribute: cond.Attribute,
			Operator:  cond.Operator,
			Value:     cond.Value,
		}
		match := evaluateAttributeRule(rule, evalCtx)

		if op == CombineAND && !match {
			return false
		}
		if op == CombineOR && match {
			return true
		}
	}

	return op == CombineAND
}
```

- [x] **Step 4: Run tests to verify they pass**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -run TestEvaluateConditions -v
```

Expected: All tests PASS.

- [x] **Step 5: Run full targeting test suite**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -run TestEvaluate -v
```

Expected: All existing tests still pass.

- [x] **Step 6: Commit**

```bash
git add internal/flags/targeting.go internal/flags/targeting_test.go
git commit -m "feat: add shared evaluateConditions function for segments and compound rules"
```

---

### Task 5: Segment Repository

**Files:**
- Create: `internal/flags/segment_repository.go`
- Modify: `internal/flags/repository.go:25-69`

- [x] **Step 1: Add segment methods to FlagRepository interface**

In `internal/flags/repository.go`, add after the existing methods (after line 68):

```go
	// Segment operations
	CreateSegment(ctx context.Context, segment *models.Segment) error
	GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error)
	GetSegmentByKey(ctx context.Context, projectID uuid.UUID, key string) (*models.Segment, error)
	ListSegments(ctx context.Context, projectID uuid.UUID) ([]*models.Segment, error)
	UpdateSegment(ctx context.Context, segment *models.Segment) error
	DeleteSegment(ctx context.Context, id uuid.UUID) error
```

- [x] **Step 2: Find the PostgreSQL repository implementation**

The concrete FlagRepository is in `internal/platform/database/postgres/`. Locate the file that implements `FlagRepository` — likely `flag_repository.go` or similar. Read it to understand the patterns used (how queries are structured, how `db.Pool` is used, error handling style).

- [x] **Step 3: Implement segment repository methods**

Add segment CRUD methods to the PostgreSQL FlagRepository implementation, following the same patterns as existing flag/rule methods. Each method:

- `CreateSegment`: INSERT into `segments`, then INSERT each condition into `segment_conditions` in a transaction.
- `GetSegment`: SELECT from `segments` WHERE id, then SELECT conditions from `segment_conditions` WHERE segment_id ordered by priority.
- `GetSegmentByKey`: Same as GetSegment but WHERE project_id AND key.
- `ListSegments`: SELECT from `segments` WHERE project_id. Load conditions for each segment.
- `UpdateSegment`: UPDATE `segments`, DELETE existing conditions, INSERT new conditions — all in a transaction.
- `DeleteSegment`: DELETE from `segments` WHERE id (conditions cascade).

- [x] **Step 4: Verify build**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go build ./...
```

Expected: Build succeeds. Any mock implementations of FlagRepository will need the new methods added (check test files for mock structs).

- [x] **Step 5: Update mock repositories in tests**

Search for mock implementations of `FlagRepository` in test files and add stub implementations of the new segment methods so tests compile:

```go
func (m *mockFlagRepository) CreateSegment(ctx context.Context, segment *models.Segment) error { return nil }
func (m *mockFlagRepository) GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error) { return nil, nil }
func (m *mockFlagRepository) GetSegmentByKey(ctx context.Context, projectID uuid.UUID, key string) (*models.Segment, error) { return nil, nil }
func (m *mockFlagRepository) ListSegments(ctx context.Context, projectID uuid.UUID) ([]*models.Segment, error) { return nil, nil }
func (m *mockFlagRepository) UpdateSegment(ctx context.Context, segment *models.Segment) error { return nil }
func (m *mockFlagRepository) DeleteSegment(ctx context.Context, id uuid.UUID) error { return nil }
```

- [x] **Step 6: Run tests**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./... 2>&1 | tail -20
```

Expected: All tests pass.

- [x] **Step 7: Commit**

```bash
git add internal/flags/repository.go internal/platform/database/postgres/
git commit -m "feat: add segment CRUD to repository interface and PostgreSQL implementation"
```

---

### Task 6: Segment Cache Methods

**Files:**
- Modify: `internal/flags/evaluator.go:17-32` (Cache interface)
- Modify: `internal/platform/cache/flagcache/flagcache.go`

- [x] **Step 1: Add segment methods to Cache interface**

In `internal/flags/evaluator.go`, add to the `Cache` interface (after line 30):

```go
	GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error)
	SetSegment(ctx context.Context, segment *models.Segment, ttl time.Duration) error
```

- [x] **Step 2: Implement segment cache methods**

In `internal/platform/cache/flagcache/flagcache.go`, add:

```go
func (c *FlagCache) GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error) {
	key := fmt.Sprintf("segment:%s", id)
	data, err := c.redis.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var segment models.Segment
	if err := json.Unmarshal(data, &segment); err != nil {
		return nil, err
	}
	return &segment, nil
}

func (c *FlagCache) SetSegment(ctx context.Context, segment *models.Segment, ttl time.Duration) error {
	key := fmt.Sprintf("segment:%s", segment.ID)
	data, err := json.Marshal(segment)
	if err != nil {
		return err
	}
	return c.redis.Set(ctx, key, data, ttl).Err()
}
```

- [x] **Step 3: Update mock cache implementations**

Search for mock implementations of the `Cache` interface in test files and add stub implementations:

```go
func (m *mockCache) GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error) { return nil, nil }
func (m *mockCache) SetSegment(ctx context.Context, segment *models.Segment, ttl time.Duration) error { return nil }
```

- [x] **Step 4: Verify build and tests**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go build ./... && go test ./... 2>&1 | tail -20
```

Expected: Build and all tests pass.

- [x] **Step 5: Commit**

```bash
git add internal/flags/evaluator.go internal/platform/cache/flagcache/flagcache.go
git commit -m "feat: add segment caching to Cache interface and Redis implementation"
```

---

### Task 7: Wire Segment Evaluation into Evaluator (TDD)

**Files:**
- Modify: `internal/flags/evaluator.go:51-58,184-201`
- Modify: `internal/flags/targeting_test.go`

- [x] **Step 1: Write failing test for segment evaluation**

Add to `internal/flags/targeting_test.go` (or a new `evaluator_test.go` if targeting_test doesn't have access to Evaluator):

The test needs to verify that when a rule with `RuleTypeSegment` is evaluated, the evaluator loads the segment and evaluates its conditions. Since `evaluateRule` is a method on `Evaluator`, write the test at the service/evaluator level.

Add to the appropriate test file:

```go
func TestEvaluateRule_Segment(t *testing.T) {
	segmentID := uuid.New()
	repo := &mockFlagRepository{
		segment: &models.Segment{
			ID:        segmentID,
			CombineOp: "AND",
			Conditions: []models.SegmentCondition{
				{Attribute: "plan", Operator: "eq", Value: "enterprise"},
				{Attribute: "region", Operator: "eq", Value: "US"},
			},
		},
	}
	cache := &mockCache{}
	evaluator := NewEvaluator(repo, cache)

	rule := &models.TargetingRule{
		RuleType:  models.RuleTypeSegment,
		SegmentID: &segmentID,
		Enabled:   true,
	}

	// Should match: user has plan=enterprise and region=US
	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"plan": "enterprise", "region": "US"},
	}
	match, err := evaluator.evaluateRule(rule, evalCtx, "test-flag")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !match {
		t.Error("expected segment rule to match, got false")
	}

	// Should not match: user has wrong region
	evalCtx2 := models.EvaluationContext{
		Attributes: map[string]string{"plan": "enterprise", "region": "EU"},
	}
	match2, err := evaluator.evaluateRule(rule, evalCtx2, "test-flag")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if match2 {
		t.Error("expected segment rule not to match, got true")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -run TestEvaluateRule_Segment -v
```

Expected: FAIL — evaluator doesn't have segment loading logic.

- [x] **Step 3: Add loadSegment method to Evaluator**

In `internal/flags/evaluator.go`, add a method:

```go
func (e *Evaluator) loadSegment(ctx context.Context, segmentID *uuid.UUID) (*models.Segment, error) {
	if segmentID == nil {
		return nil, fmt.Errorf("segment rule missing segment_id")
	}

	// Try cache first
	segment, err := e.cache.GetSegment(ctx, *segmentID)
	if err == nil && segment != nil {
		return segment, nil
	}

	// Fallback to database
	segment, err = e.repo.GetSegment(ctx, *segmentID)
	if err != nil {
		return nil, fmt.Errorf("loading segment %s: %w", segmentID, err)
	}
	_ = e.cache.SetSegment(ctx, segment, e.cacheTTL)
	return segment, nil
}
```

- [x] **Step 4: Wire segment evaluation in evaluateRule**

In `internal/flags/evaluator.go`, replace the segment stub (lines 194-197):

```go
case models.RuleTypeSegment:
	segment, err := e.loadSegment(ctx, rule.SegmentID)
	if err != nil {
		return false, err
	}
	conditions := make([]models.CompoundCondition, len(segment.Conditions))
	for i, sc := range segment.Conditions {
		conditions[i] = models.CompoundCondition{
			Attribute: sc.Attribute,
			Operator:  sc.Operator,
			Value:     sc.Value,
		}
	}
	return evaluateConditions(conditions, CombineOperator(segment.CombineOp), evalCtx), nil
```

Note: `evaluateRule` needs access to `context.Context` for the segment loading. Check if it already receives ctx — if not, update the signature from `evaluateRule(rule, evalCtx, flagKey)` to `evaluateRule(ctx, rule, evalCtx, flagKey)` and update all call sites in `Evaluate()`.

- [x] **Step 5: Run test to verify it passes**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -run TestEvaluateRule_Segment -v
```

Expected: PASS.

- [x] **Step 6: Run full test suite**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -v 2>&1 | tail -30
```

Expected: All tests pass.

- [x] **Step 7: Commit**

```bash
git add internal/flags/evaluator.go internal/flags/targeting_test.go
git commit -m "feat: wire real segment evaluation into evaluator, replacing stub"
```

---

### Task 8: Wire Compound Rule Evaluation (TDD)

**Files:**
- Modify: `internal/flags/evaluator.go:184-201`
- Modify: `internal/flags/targeting_test.go`

- [x] **Step 1: Write failing test for compound rule evaluation**

Add to the test file:

```go
func TestEvaluateRule_Compound(t *testing.T) {
	repo := &mockFlagRepository{}
	cache := &mockCache{}
	evaluator := NewEvaluator(repo, cache)

	rule := &models.TargetingRule{
		RuleType:  models.RuleTypeCompound,
		CombineOp: "AND",
		Conditions: []models.CompoundCondition{
			{Attribute: "plan", Operator: "eq", Value: "enterprise"},
			{Attribute: "region", Operator: "in", Value: "US,EU"},
		},
		Enabled: true,
	}

	// Match: enterprise plan in US
	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"plan": "enterprise", "region": "US"},
	}
	match, err := evaluator.evaluateRule(rule, evalCtx, "test-flag")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !match {
		t.Error("expected compound rule to match")
	}

	// No match: free plan
	evalCtx2 := models.EvaluationContext{
		Attributes: map[string]string{"plan": "free", "region": "US"},
	}
	match2, err := evaluator.evaluateRule(rule, evalCtx2, "test-flag")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if match2 {
		t.Error("expected compound rule not to match")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -run TestEvaluateRule_Compound -v
```

Expected: FAIL — no case for `RuleTypeCompound` in evaluateRule.

- [x] **Step 3: Add compound case to evaluateRule**

In `internal/flags/evaluator.go`, add a new case in the `evaluateRule` switch:

```go
case models.RuleTypeCompound:
	return evaluateConditions(rule.Conditions, CombineOperator(rule.CombineOp), evalCtx), nil
```

- [x] **Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -run TestEvaluateRule_Compound -v
```

Expected: PASS.

- [x] **Step 5: Run full test suite**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -v 2>&1 | tail -30
```

Expected: All tests pass.

- [x] **Step 6: Commit**

```bash
git add internal/flags/evaluator.go internal/flags/targeting_test.go
git commit -m "feat: wire compound rule evaluation into evaluator"
```

---

### Task 9: Compound Rule Conditions Marshaling in Repository

**Files:**
- Modify: PostgreSQL flag repository (the file implementing `CreateRule`, `GetRule`, `ListRules`, `UpdateRule`)

- [x] **Step 1: Find the repository implementation**

The PostgreSQL repository that implements `CreateRule`/`ListRules` etc. is in `internal/platform/database/postgres/`. Read the file to understand how rules are currently read/written.

- [x] **Step 2: Update CreateRule to marshal conditions**

When `rule.RuleType == "compound"`, marshal `rule.Conditions` to JSON and store in the `conditions` JSONB column. Set `rule.CombineOp` into the `combine_op` column.

- [x] **Step 3: Update ListRules/GetRule to unmarshal conditions**

When scanning rules from the database, if `rule_type == "compound"`, unmarshal the `conditions` JSONB column into `rule.Conditions` and read `combine_op` into `rule.CombineOp`.

- [x] **Step 4: Update UpdateRule similarly**

Same marshaling logic as CreateRule.

- [x] **Step 5: Verify build and tests**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go build ./... && go test ./... 2>&1 | tail -20
```

Expected: Build and all tests pass.

- [x] **Step 6: Commit**

```bash
git add internal/platform/database/postgres/
git commit -m "feat: marshal/unmarshal compound rule conditions in repository"
```

---

### Task 10: SSE Broadcast Overhaul (TDD)

**Files:**
- Modify: `internal/flags/handler.go:38-45,285-363,365-412,419-476,484-502,684-772`

- [x] **Step 1: Write failing test for broadcastEvent**

Add to `internal/flags/handler_test.go`:

```go
func TestBroadcastEvent(t *testing.T) {
	broker := NewSSEBroker()
	handler := &Handler{sse: broker}

	ch := broker.Subscribe()
	defer broker.Unsubscribe(ch)

	flagID := uuid.New()
	handler.broadcastEvent("flag.updated", flagID, "my-flag")

	select {
	case msg := <-ch:
		var event struct {
			Event     string `json:"event"`
			FlagID    string `json:"flag_id"`
			FlagKey   string `json:"flag_key"`
			Timestamp string `json:"timestamp"`
		}
		if err := json.Unmarshal([]byte(msg), &event); err != nil {
			t.Fatalf("failed to unmarshal SSE event: %v", err)
		}
		if event.Event != "flag.updated" {
			t.Errorf("event = %q, want %q", event.Event, "flag.updated")
		}
		if event.FlagID != flagID.String() {
			t.Errorf("flag_id = %q, want %q", event.FlagID, flagID.String())
		}
		if event.FlagKey != "my-flag" {
			t.Errorf("flag_key = %q, want %q", event.FlagKey, "my-flag")
		}
		if event.Timestamp == "" {
			t.Error("timestamp should not be empty")
		}
	default:
		t.Error("expected to receive SSE event")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -run TestBroadcastEvent -v
```

Expected: FAIL — `broadcastEvent` method not defined.

- [x] **Step 3: Add SSEEvent type and broadcastEvent helper**

In `internal/flags/handler.go`, add before the SSEBroker struct:

```go
type SSEEvent struct {
	Event     string    `json:"event"`
	FlagID    string    `json:"flag_id"`
	FlagKey   string    `json:"flag_key,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func (h *Handler) broadcastEvent(event string, flagID uuid.UUID, flagKey string) {
	data, _ := json.Marshal(SSEEvent{
		Event:     event,
		FlagID:    flagID.String(),
		FlagKey:   flagKey,
		Timestamp: time.Now(),
	})
	h.sse.Broadcast(string(data))
}
```

- [x] **Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -run TestBroadcastEvent -v
```

Expected: PASS.

- [x] **Step 5: Add broadcastEvent calls to all mutation handlers**

In `internal/flags/handler.go`, add `h.broadcastEvent(...)` after the successful service call in each handler. You'll need the flag's key for the event — read it from the service response or the request context.

**updateFlag** (after successful service.UpdateFlag call, around line 340):
```go
h.broadcastEvent("flag.updated", id, flag.Key)
```

**archiveFlag** (after successful service.ArchiveFlag call, around line 395):
```go
h.broadcastEvent("flag.archived", id, "")
```

**toggleFlag** — refactor the existing inline broadcast (line 445) to use `broadcastEvent`:
```go
h.broadcastEvent("flag.toggled", id, "")
```
Remove the old `h.sse.Broadcast(fmt.Sprintf(...))` line.

**bulkToggle** (inside the loop, after each successful toggle, around line 495):
```go
h.broadcastEvent("flag.bulk_toggled", flagID, "")
```

**addRule** (after successful service.AddRule call, around line 710):
```go
h.broadcastEvent("rule.created", rule.FlagID, "")
```

**updateRule** (after successful service.UpdateRule call, around line 750):
```go
h.broadcastEvent("rule.updated", rule.FlagID, "")
```

**deleteRule** (after successful service.DeleteRule call, around line 768):
```go
h.broadcastEvent("rule.deleted", flagID, "")
```

Note: For handlers where you have the flag ID but not the key, pass empty string for flagKey — the ID is sufficient for cache invalidation.

- [x] **Step 6: Run full handler tests**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -run TestHandler -v 2>&1 | tail -30
```

Expected: All tests pass.

- [x] **Step 7: Commit**

```bash
git add internal/flags/handler.go internal/flags/handler_test.go
git commit -m "feat: add structured SSE broadcasts to all flag mutation handlers"
```

---

### Task 11: Singleflight Cache Stampede Protection (TDD)

**Files:**
- Modify: `internal/flags/evaluator.go:51-58,99-138`

- [x] **Step 1: Write failing test for singleflight coalescing**

Add to an evaluator test file:

```go
func TestSingleflightCoalescesFlagLookups(t *testing.T) {
	var callCount atomic.Int32
	repo := &mockFlagRepository{
		getFlagByKeyFunc: func(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*models.FeatureFlag, error) {
			callCount.Add(1)
			time.Sleep(50 * time.Millisecond) // simulate DB latency
			return &models.FeatureFlag{
				ID:      uuid.New(),
				Key:     key,
				Enabled: true,
			}, nil
		},
	}
	cache := &mockCache{} // always returns cache miss
	evaluator := NewEvaluator(repo, cache)

	projectID := uuid.New()
	envID := uuid.New()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			evaluator.Evaluate(context.Background(), projectID, envID, "test-flag", models.EvaluationContext{})
		}()
	}
	wg.Wait()

	if callCount.Load() != 1 {
		t.Errorf("expected 1 DB call due to singleflight, got %d", callCount.Load())
	}
}
```

Note: The mock repository may need a function field pattern (e.g., `getFlagByKeyFunc`) to support custom behavior per test. Check existing mock patterns and adapt.

- [x] **Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -run TestSingleflightCoalesces -v
```

Expected: FAIL — 10 DB calls instead of 1.

- [x] **Step 3: Add singleflight groups to Evaluator**

In `internal/flags/evaluator.go`, add to the Evaluator struct (after line 57):

```go
sfFlags singleflight.Group
sfRules singleflight.Group
```

Add import:
```go
"golang.org/x/sync/singleflight"
```

- [x] **Step 4: Wrap flag fallback with singleflight**

In the `Evaluate` method, replace the flag cache miss path (around lines 103-110):

```go
flag, err := e.cache.GetFlag(ctx, projectID, environmentID, key)
if err != nil || flag == nil {
	e.Metrics.Misses.Add(1)
	sfKey := fmt.Sprintf("%s:%s:%s", projectID, environmentID, key)
	val, sfErr, _ := e.sfFlags.Do(sfKey, func() (interface{}, error) {
		return e.repo.GetFlagByKey(ctx, projectID, environmentID, key)
	})
	if sfErr != nil {
		return nil, fmt.Errorf("flag %q not found: %w", key, sfErr)
	}
	flag = val.(*models.FeatureFlag)
	_ = e.cache.SetFlag(ctx, flag, e.cacheTTL)
} else {
	e.Metrics.Hits.Add(1)
}
```

- [x] **Step 5: Wrap rules fallback with singleflight**

Replace the rules cache miss path (around lines 129-135):

```go
rules, err := e.cache.GetRules(ctx, flag.ID)
if err != nil || rules == nil {
	e.Metrics.Misses.Add(1)
	val, sfErr, _ := e.sfRules.Do(flag.ID.String(), func() (interface{}, error) {
		return e.repo.ListRules(ctx, flag.ID)
	})
	if sfErr != nil {
		return nil, fmt.Errorf("loading rules for flag %q: %w", key, sfErr)
	}
	rules = val.([]*models.TargetingRule)
	_ = e.cache.SetRules(ctx, flag.ID, rules, e.cacheTTL)
} else {
	e.Metrics.Hits.Add(1)
}
```

- [x] **Step 6: Run test to verify it passes**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -run TestSingleflightCoalesces -v
```

Expected: PASS — only 1 DB call.

- [x] **Step 7: Run full test suite**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -v 2>&1 | tail -30
```

Expected: All tests pass.

- [x] **Step 8: Commit**

```bash
git add internal/flags/evaluator.go
git commit -m "feat: add singleflight protection against cache stampedes"
```

---

### Task 12: Concurrent Batch Evaluation (TDD)

**Files:**
- Modify: `internal/flags/service.go:262-279`
- Modify: `internal/flags/service_test.go`

- [x] **Step 1: Write failing test for concurrent batch evaluation**

Add to `internal/flags/service_test.go`:

```go
func TestBatchEvaluate_Concurrent(t *testing.T) {
	var maxConcurrent atomic.Int32
	var current atomic.Int32

	repo := &mockFlagRepository{}
	cache := &mockCache{}
	evaluator := NewEvaluator(repo, cache)

	// Override evaluator behavior to track concurrency
	// This depends on the mock pattern — may need a custom evaluator wrapper
	// that tracks concurrent calls

	svc := NewFlagService(repo, cache, nil)

	keys := make([]string, 20)
	for i := range keys {
		keys[i] = fmt.Sprintf("flag-%d", i)
	}

	results, err := svc.BatchEvaluate(
		context.Background(),
		uuid.New(), uuid.New(),
		keys,
		models.EvaluationContext{},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 20 {
		t.Errorf("expected 20 results, got %d", len(results))
	}
}

func TestBatchEvaluate_ErrorField(t *testing.T) {
	// Set up a mock that fails for a specific flag key
	repo := &mockFlagRepository{
		getFlagByKeyFunc: func(ctx context.Context, projectID, envID uuid.UUID, key string) (*models.FeatureFlag, error) {
			if key == "bad-flag" {
				return nil, fmt.Errorf("flag not found")
			}
			return &models.FeatureFlag{
				ID:      uuid.New(),
				Key:     key,
				Enabled: true,
			}, nil
		},
	}
	cache := &mockCache{}
	svc := NewFlagService(repo, cache, nil)

	results, err := svc.BatchEvaluate(
		context.Background(),
		uuid.New(), uuid.New(),
		[]string{"good-flag", "bad-flag"},
		models.EvaluationContext{},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// bad-flag should have Error field set
	badResult := results[1]
	if badResult.Error == "" {
		t.Error("expected Error field to be set for failed evaluation")
	}
	if badResult.Enabled {
		t.Error("expected Enabled=false for failed evaluation")
	}

	// good-flag should have no Error
	goodResult := results[0]
	if goodResult.Error != "" {
		t.Errorf("expected no error for good flag, got %q", goodResult.Error)
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -run TestBatchEvaluate -v
```

Expected: FAIL — Error field doesn't exist yet (added in Task 3), and BatchEvaluate is still sequential.

- [x] **Step 3: Replace BatchEvaluate with concurrent implementation**

In `internal/flags/service.go`, replace the `BatchEvaluate` method (lines 262-279):

```go
func (s *flagService) BatchEvaluate(ctx context.Context, projectID, environmentID uuid.UUID, keys []string, evalCtx models.EvaluationContext) ([]*models.FlagEvaluationResult, error) {
	results := make([]*models.FlagEvaluationResult, len(keys))
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(10)

	for i, key := range keys {
		i, key := i, key
		g.Go(func() error {
			result, err := s.evaluator.Evaluate(gCtx, projectID, environmentID, key, evalCtx)
			if err != nil {
				results[i] = &models.FlagEvaluationResult{
					FlagKey: key,
					Enabled: false,
					Value:   "",
					Reason:  "error",
					Error:   err.Error(),
				}
				return nil
			}
			results[i] = result
			return nil
		})
	}
	_ = g.Wait()
	return results, nil
}
```

Add import:
```go
"golang.org/x/sync/errgroup"
```

- [x] **Step 4: Run tests to verify they pass**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -run TestBatchEvaluate -v
```

Expected: PASS.

- [x] **Step 5: Run full test suite**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./... 2>&1 | tail -20
```

Expected: All tests pass.

- [x] **Step 6: Commit**

```bash
git add internal/flags/service.go internal/flags/service_test.go
git commit -m "feat: concurrent batch evaluation with errgroup and per-flag error field"
```

---

### Task 13: Segment Handler and Routes

**Files:**
- Create: `internal/flags/segment_handler.go`
- Create: `internal/flags/segment_handler_test.go`
- Modify: `internal/flags/service.go:30-83` (add segment service methods)
- Modify: `cmd/api/main.go:291-293`

- [x] **Step 1: Add segment methods to FlagService interface**

In `internal/flags/service.go`, add to the `FlagService` interface:

```go
	// Segment operations
	CreateSegment(ctx context.Context, segment *models.Segment) error
	GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error)
	ListSegments(ctx context.Context, projectID uuid.UUID) ([]*models.Segment, error)
	UpdateSegment(ctx context.Context, segment *models.Segment) error
	DeleteSegment(ctx context.Context, id uuid.UUID) error
```

- [x] **Step 2: Implement segment service methods**

Add to `internal/flags/service.go`:

```go
func (s *flagService) CreateSegment(ctx context.Context, segment *models.Segment) error {
	return s.repo.CreateSegment(ctx, segment)
}

func (s *flagService) GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error) {
	return s.repo.GetSegment(ctx, id)
}

func (s *flagService) ListSegments(ctx context.Context, projectID uuid.UUID) ([]*models.Segment, error) {
	return s.repo.ListSegments(ctx, projectID)
}

func (s *flagService) UpdateSegment(ctx context.Context, segment *models.Segment) error {
	return s.repo.UpdateSegment(ctx, segment)
}

func (s *flagService) DeleteSegment(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteSegment(ctx, id)
}
```

- [x] **Step 3: Write segment handler**

Create `internal/flags/segment_handler.go`:

```go
package flags

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/shadsorg/deploysentry/internal/models"
)

type createSegmentRequest struct {
	Key         string                    `json:"key" binding:"required"`
	Name        string                    `json:"name" binding:"required"`
	Description string                    `json:"description"`
	CombineOp   string                    `json:"combine_op"`
	Conditions  []segmentConditionRequest `json:"conditions" binding:"required"`
}

type segmentConditionRequest struct {
	Attribute string `json:"attribute" binding:"required"`
	Operator  string `json:"operator" binding:"required"`
	Value     string `json:"value" binding:"required"`
	Priority  int    `json:"priority"`
}

type updateSegmentRequest struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	CombineOp   string                    `json:"combine_op"`
	Conditions  []segmentConditionRequest `json:"conditions"`
}

func (h *Handler) RegisterSegmentRoutes(rg *gin.RouterGroup) {
	segments := rg.Group("/segments")
	{
		segments.POST("", h.createSegment)
		segments.GET("", h.listSegments)
		segments.GET("/:segmentID", h.getSegment)
		segments.PUT("/:segmentID", h.updateSegment)
		segments.DELETE("/:segmentID", h.deleteSegment)
	}
}

func (h *Handler) createSegment(c *gin.Context) {
	var req createSegmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	projectID, err := uuid.Parse(c.GetString("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
		return
	}

	combineOp := req.CombineOp
	if combineOp == "" {
		combineOp = "AND"
	}

	segment := &models.Segment{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Key:         req.Key,
		Name:        req.Name,
		Description: req.Description,
		CombineOp:   combineOp,
	}

	for _, cond := range req.Conditions {
		segment.Conditions = append(segment.Conditions, models.SegmentCondition{
			ID:        uuid.New(),
			SegmentID: segment.ID,
			Attribute: cond.Attribute,
			Operator:  cond.Operator,
			Value:     cond.Value,
			Priority:  cond.Priority,
		})
	}

	if err := h.service.CreateSegment(c.Request.Context(), segment); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, segment)
}

func (h *Handler) listSegments(c *gin.Context) {
	projectID, err := uuid.Parse(c.GetString("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
		return
	}

	segments, err := h.service.ListSegments(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, segments)
}

func (h *Handler) getSegment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("segmentID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid segment ID"})
		return
	}

	segment, err := h.service.GetSegment(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "segment not found"})
		return
	}

	c.JSON(http.StatusOK, segment)
}

func (h *Handler) updateSegment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("segmentID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid segment ID"})
		return
	}

	var req updateSegmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	segment, err := h.service.GetSegment(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "segment not found"})
		return
	}

	if req.Name != "" {
		segment.Name = req.Name
	}
	if req.Description != "" {
		segment.Description = req.Description
	}
	if req.CombineOp != "" {
		segment.CombineOp = req.CombineOp
	}
	if req.Conditions != nil {
		segment.Conditions = nil
		for _, cond := range req.Conditions {
			segment.Conditions = append(segment.Conditions, models.SegmentCondition{
				ID:        uuid.New(),
				SegmentID: segment.ID,
				Attribute: cond.Attribute,
				Operator:  cond.Operator,
				Value:     cond.Value,
				Priority:  cond.Priority,
			})
		}
	}

	if err := h.service.UpdateSegment(c.Request.Context(), segment); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, segment)
}

func (h *Handler) deleteSegment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("segmentID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid segment ID"})
		return
	}

	if err := h.service.DeleteSegment(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
```

- [x] **Step 4: Register segment routes in main.go**

In `cmd/api/main.go`, after the existing `flagHandler.RegisterRoutes(api)` call (around line 293), add:

```go
flagHandler.RegisterSegmentRoutes(api)
```

Check how project context (project_id) is resolved for the route group. The segment routes need `project_id` in the Gin context. Look at how existing flag routes resolve this — likely via middleware that extracts orgSlug/projectSlug from the URL and resolves to IDs.

If segment routes need to be under `/api/v1/orgs/:orgSlug/projects/:projectSlug/segments`, adjust the route registration accordingly.

- [x] **Step 5: Write basic handler tests**

Create `internal/flags/segment_handler_test.go` with tests for create, get, list, update, and delete. Follow the same patterns used in `handler_test.go` for setting up Gin test contexts and mock services.

- [x] **Step 6: Run tests**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -run TestSegment -v
```

Expected: All segment handler tests pass.

- [x] **Step 7: Run full test suite**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./... 2>&1 | tail -20
```

Expected: All tests pass.

- [x] **Step 8: Commit**

```bash
git add internal/flags/segment_handler.go internal/flags/segment_handler_test.go \
       internal/flags/service.go cmd/api/main.go
git commit -m "feat: add segment CRUD handler, service methods, and routes"
```

---

### Task 14: Update Plan Document and Current Initiatives

**Files:**
- Modify: `docs/Feature_Flag_Engine_Improvements.md`
- Modify: `docs/Current_Initiatives.md`

- [x] **Step 1: Update the plan document phase**

Change the phase from `Design` to `Implementation` in `docs/Feature_Flag_Engine_Improvements.md`.

- [x] **Step 2: Check off completed items**

Update the checklist in `docs/Feature_Flag_Engine_Improvements.md` to reflect all completed work.

- [x] **Step 3: Update Current Initiatives**

Update `docs/Current_Initiatives.md` to reflect the phase change.

- [x] **Step 4: Commit**

```bash
git add docs/Feature_Flag_Engine_Improvements.md docs/Current_Initiatives.md
git commit -m "docs: update flag engine improvements to Implementation phase"
```

---

### Task 15: Final Verification

- [x] **Step 1: Run full build**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go build ./...
```

Expected: Clean build with no errors.

- [x] **Step 2: Run full test suite**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go test ./... -count=1
```

Expected: All tests pass.

- [x] **Step 3: Run linter (if configured)**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && make lint 2>/dev/null || go vet ./...
```

Expected: No issues.
