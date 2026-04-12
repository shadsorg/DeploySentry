# Feature Flag Engine Improvements — Design Spec

**Date:** 2026-04-10
**Status:** Approved
**Approach:** Shared Evaluation Core — segments and compound rules share a single `evaluateConditions` function

## Overview

Five targeted improvements to make the feature flag evaluation engine more complete, performant, and enterprise-ready:

1. Rule-based segment evaluation (replacing the stub)
2. Compound rule support via shared evaluation core
3. SSE broadcast overhaul for all flag mutations
4. Singleflight cache stampede protection
5. Batch evaluation concurrency with error distinction

## 1. Rule-Based Segments

### Data Model

New migration `033_create_segments.up.sql`:

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
```

### Go Model

```go
type Segment struct {
    ID          uuid.UUID           `json:"id" db:"id"`
    ProjectID   uuid.UUID           `json:"project_id" db:"project_id"`
    Key         string              `json:"key" db:"key"`
    Name        string              `json:"name" db:"name"`
    Description string              `json:"description" db:"description"`
    CombineOp   CombineOperator     `json:"combine_op" db:"combine_op"`
    Conditions  []CompoundCondition `json:"conditions" db:"-"`
    CreatedAt   time.Time           `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time           `json:"updated_at" db:"updated_at"`
}
```

`CompoundCondition` and `CombineOperator` types already exist in `internal/flags/targeting.go`.

### Shared Evaluation Function

New function in `targeting.go`:

```go
func evaluateConditions(conditions []CompoundCondition, op CombineOperator, ctx models.EvaluationContext) bool
```

- Iterates conditions, applies attribute matching logic per condition (reuses existing operator logic from `evaluateAttributeRule`)
- `AND` mode: all conditions must match (short-circuits on first `false`)
- `OR` mode: any condition must match (short-circuits on first `true`)

### Evaluator Integration

In `evaluateRule` (`evaluator.go:194-197`), replace the stub:

```go
case models.RuleTypeSegment:
    segment, err := e.loadSegment(ctx, rule.SegmentID)
    if err != nil { return false, err }
    return evaluateConditions(segment.Conditions, segment.CombineOp, evalCtx), nil
```

Segments are cached with the same TTL pattern as flags and rules. `loadSegment` checks cache first, falls back to the segment repository.

### Segment Repository

New `internal/flags/segment_repository.go`:

- `CreateSegment(ctx, segment) error`
- `GetSegment(ctx, id) (*Segment, error)`
- `GetSegmentByKey(ctx, projectID, key) (*Segment, error)`
- `ListSegments(ctx, projectID) ([]Segment, error)`
- `UpdateSegment(ctx, segment) error`
- `DeleteSegment(ctx, id) error`

Conditions are loaded eagerly with the segment (joined query or second query).

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/orgs/:orgSlug/projects/:projectSlug/segments` | Create segment with conditions |
| GET | `/api/v1/orgs/:orgSlug/projects/:projectSlug/segments` | List segments |
| GET | `.../segments/:segmentID` | Get segment with conditions |
| PUT | `.../segments/:segmentID` | Update segment (name, description, combine_op, conditions) |
| DELETE | `.../segments/:segmentID` | Delete segment |

## 2. Compound Rules

### Model Changes

Add fields to `TargetingRule` in `internal/models/flag.go`:

```go
type TargetingRule struct {
    // ... existing fields ...
    Conditions  []CompoundCondition `json:"conditions,omitempty" db:"-"`
    CombineOp   CombineOperator     `json:"combine_op,omitempty" db:"combine_op"`
}
```

The `conditions` JSONB column already exists in `flag_targeting_rules`. New migration `034_add_rule_combine_op.up.sql`:

```sql
ALTER TABLE flag_targeting_rules ADD COLUMN combine_op TEXT NOT NULL DEFAULT 'AND';
```

### New Rule Type

Add `RuleTypeCompound = "compound"` to the rule type constants. Update the DB check constraint:

```sql
ALTER TABLE flag_targeting_rules DROP CONSTRAINT IF EXISTS flag_targeting_rules_rule_type_check;
ALTER TABLE flag_targeting_rules ADD CONSTRAINT flag_targeting_rules_rule_type_check
    CHECK (rule_type IN ('percentage', 'user_target', 'attribute', 'segment', 'schedule', 'compound'));
```

### Evaluation

In `evaluateRule`:

```go
case models.RuleTypeCompound:
    return evaluateConditions(rule.Conditions, rule.CombineOp, evalCtx), nil
```

Same shared function as segments — no new evaluation logic.

### API

No new endpoints. Existing `addRule` and `updateRule` accept the new fields:

```json
{
  "rule_type": "compound",
  "conditions": [
    {"attribute": "plan", "operator": "eq", "value": "enterprise"},
    {"attribute": "region", "operator": "in", "value": "US,EU"}
  ],
  "combine_op": "AND",
  "value": "variant-a"
}
```

Repository layer needs to marshal/unmarshal `conditions` JSONB when reading/writing compound rules.

## 3. SSE Broadcast Overhaul

### Structured Event Envelope

```go
type SSEEvent struct {
    Event     string    `json:"event"`
    FlagID    string    `json:"flag_id"`
    FlagKey   string    `json:"flag_key,omitempty"`
    Timestamp time.Time `json:"timestamp"`
}
```

### Event Types

| Event | Trigger |
|-------|---------|
| `flag.toggled` | `toggleFlag` (existing) |
| `flag.updated` | `updateFlag` |
| `flag.archived` | `archiveFlag` |
| `flag.bulk_toggled` | `bulkToggle` (one event per flag) |
| `rule.created` | `addRule` |
| `rule.updated` | `updateRule` |
| `rule.deleted` | `deleteRule` |

### Helper Method

```go
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

Refactor existing `toggleFlag` broadcast to use this helper for consistency.

### Handler Changes

| Handler | Location | Add After Service Call |
|---------|----------|----------------------|
| `updateFlag` | `handler.go:~285` | `h.broadcastEvent("flag.updated", ...)` |
| `archiveFlag` | `handler.go:~365` | `h.broadcastEvent("flag.archived", ...)` |
| `bulkToggle` | `handler.go:~484` | `h.broadcastEvent("flag.bulk_toggled", ...)` per flag |
| `addRule` | `handler.go:~684` | `h.broadcastEvent("rule.created", ...)` |
| `updateRule` | `handler.go:~718` | `h.broadcastEvent("rule.updated", ...)` |
| `deleteRule` | `handler.go:~759` | `h.broadcastEvent("rule.deleted", ...)` |

### SDK Behavior

SDKs receiving events should invalidate their local cache for the affected flag (identified by `flag_key`) and re-evaluate. This enables targeted invalidation rather than full cache flush.

## 4. Singleflight Cache Stampede Protection

### Dependency

Add `golang.org/x/sync` module (also used by errgroup in Section 5).

### Evaluator Changes

Add two `singleflight.Group` fields to `Evaluator`:

```go
type Evaluator struct {
    // ... existing fields ...
    sfFlags singleflight.Group
    sfRules singleflight.Group
}
```

Two groups to avoid key space collisions between flag and rule lookups.

### Flag Fallback Path (`evaluator.go:~103-110`)

```go
flag, err := e.cache.GetFlag(ctx, projectID, environmentID, key)
if err != nil || flag == nil {
    e.Metrics.Misses.Add(1)
    sfKey := fmt.Sprintf("%s:%s:%s", projectID, environmentID, key)
    val, err, _ := e.sfFlags.Do(sfKey, func() (interface{}, error) {
        return e.repo.GetFlagByKey(ctx, projectID, environmentID, key)
    })
    if err != nil {
        return nil, fmt.Errorf("flag %q not found: %w", key, err)
    }
    flag = val.(*models.FeatureFlag)
    _ = e.cache.SetFlag(ctx, flag, e.cacheTTL)
}
```

### Rules Fallback Path (`evaluator.go:~129-135`)

Same pattern using `e.sfRules` with `flag.ID.String()` as the key.

### Behavior

- First caller for a key executes the DB query
- Concurrent callers for the same key block and receive the shared result
- Key is removed after the call completes — next request proceeds normally
- Cache is populated by the first caller, so subsequent requests hit cache
- No configuration needed — purely internal optimization

## 5. Batch Evaluation Concurrency

### Implementation

Replace sequential loop in `service.go:262-279` with `errgroup`:

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

### Design Decisions

- **Concurrency limit: 10** — prevents goroutine explosion for large batches; reasonable given typical DB connection pool sizes
- **Pre-allocated slice** — each goroutine writes to its own index, no mutex needed
- **Non-fatal errors** — same graceful degradation as today, errors never cancel sibling evaluations
- **No config knob** — internal implementation detail, one-line change if tuning needed later

### Per-Flag Error Distinction

Add `Error` field to `FlagEvaluationResult` in `internal/models/flag.go`:

```go
type FlagEvaluationResult struct {
    // ... existing fields ...
    Error string `json:"error,omitempty"`
}
```

This lets SDKs distinguish:
- Legitimately disabled: `Enabled: false, Error: ""`
- Evaluation failure: `Enabled: false, Error: "flag not found"`

### Dependency

`errgroup` is in the same `golang.org/x/sync` module added for singleflight.

## Files Affected

| Area | Files |
|------|-------|
| Migrations | `migrations/033_create_segments.{up,down}.sql`, `034_add_rule_combine_op.{up,down}.sql` |
| Models | `internal/models/flag.go` (TargetingRule, FlagEvaluationResult) |
| Segment model | `internal/models/segment.go` (new) |
| Segment repo | `internal/flags/segment_repository.go` (new) |
| Segment handler | `internal/flags/segment_handler.go` (new) |
| Evaluation | `internal/flags/evaluator.go`, `internal/flags/targeting.go` |
| Service | `internal/flags/service.go` |
| Handler | `internal/flags/handler.go` |
| Cache | `internal/flags/cache.go` (add segment caching methods) |
| Dependencies | `go.mod` — add `golang.org/x/sync` |

## Testing Strategy

- Unit tests for `evaluateConditions` with AND/OR combinations and edge cases
- Unit tests for segment evaluation through `evaluateRule`
- Unit tests for compound rule evaluation through `evaluateRule`
- Integration tests for segment CRUD API endpoints
- Tests verifying singleflight coalesces concurrent DB queries
- Benchmarks for BatchEvaluate comparing sequential vs concurrent
- Integration tests verifying SSE clients receive events for all mutation types
- Tests for per-flag error distinction in batch results
