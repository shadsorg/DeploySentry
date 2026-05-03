# Staging Provisional-ID Resolution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow staged `flag.create`, `flag_rule.create`, and `strategy.create` rows to deploy atomically, with provisional UUIDs rewritten to real ids across the batch and the resulting creates surfaced through the read-path overlay before commit.

**Architecture:** `Service.Commit` becomes a four-phase pipeline (preflight → tx + topo-ordered dispatch with resolver-driven JSON rewrite → tx.Commit → post-commit hooks). New `CreateRegistry` + `CreateHandler` shape sits alongside the unchanged `CommitRegistry`. Tx-aware `Create*Tx` paths are added on `internal/flags/` and `internal/rollout/` services so create handlers can ride the staging tx for true rollback. Read-path overlay learns to synthesise create rows; lists and detail handlers gain `?include_my_staged=true` overlay calls plus a `_staged` envelope marker. Three UI surfaces (`FlagCreatePage`, `handleAddRule`, `StrategyEditor`) route through the existing `stageOrCall` helper using a new client-side `newProvisionalId()` helper that mirrors the Go variant-byte invariant.

**Tech Stack:** Go 1.22 + pgx/v5, PostgreSQL 14 with `deploy` schema, React 18 + Vite + TypeScript, Vitest.

**Spec:** [`../specs/2026-05-03-staging-provisional-id-resolution-design.md`](../specs/2026-05-03-staging-provisional-id-resolution-design.md)

---

## File Structure

### PR 1 — Backend resolution + read-path overlay

**Create:**
- `internal/staging/resolver.go` — `Resolver` type, `RewriteUUIDsInJSON`, `RewriteRow`.
- `internal/staging/resolver_test.go`
- `internal/staging/preflight.go` — `planBatch`, `ErrUnresolvedProvisional`.
- `internal/staging/preflight_test.go`
- `internal/staging/service_provisional_test.go` — end-to-end commit with creates.

**Modify:**
- `internal/staging/registry.go` — add `CreateHandler`, `CreateRegistry`.
- `internal/staging/registry_test.go` — add `CreateRegistry` tests.
- `internal/staging/service.go` — `Service` carries `*CreateRegistry`; `Commit` pipeline rewrite; `Stage` validates provisional variant byte.
- `internal/staging/service_test.go` — extend with provisional cases.
- `internal/staging/overlay.go` — already supports `synth`; add a list helper for the `_staged` envelope marker (purely additive `OverlayListMarked`).
- `internal/staging/overlay_test.go`
- `internal/flags/repository.go` — add `CreateFlagTx`, `CreateRuleTx` to interface.
- `internal/platform/database/postgres/flags.go` — implement `CreateFlagTx`, `CreateRuleTx`.
- `internal/flags/service.go` — interface adds `CreateFlagTx`, `AddRuleTx`; existing `CreateFlag`/`AddRule` compose them.
- `internal/flags/staging_commit.go` — register `commitFlagCreate`, `commitFlagRuleCreate`.
- `internal/flags/staging_commit_test.go`
- `internal/flags/handler.go` — `ListFlags` + `ListRules` apply the overlay when `include_my_staged=true`; `GetFlag` detail-by-provisional branch.
- `internal/rollout/service.go` — add `CreateStrategyTx`.
- `internal/rollout/staging_commit.go` — register `commitStrategyCreate`.
- `internal/rollout/staging_commit_test.go`
- `internal/rollout/handler.go` — `ListStrategies` overlay + detail-by-provisional.
- `cmd/api/main.go` — wire `CreateRegistry`, register the three create handlers, pass to `staging.NewService`.

### PR 2 — Client provisional minting + FlagCreatePage + StagedBadge

**Create:**
- `web/src/lib/provisional.ts` — `newProvisionalId()`.
- `web/src/lib/provisional.test.ts`
- `web/src/components/staging/StagedBadge.tsx` — leaf component reading `_staged` envelope.
- `web/src/components/staging/StagedBadge.test.tsx`

**Modify:**
- `web/src/api.ts` — `stagingApi.stage` payload type carries `provisional_id`; flag/strategy list responses type the optional `_staged` envelope.
- `web/src/pages/FlagCreatePage.tsx` — `handleSubmit` routes through `stageOrCall`.
- `web/src/pages/FlagCreatePage.test.tsx`
- `web/src/pages/FlagListPage.tsx` — render `<StagedBadge>` on rows whose response carries `_staged`.

### PR 3 — handleAddRule + StrategyEditor create

**Modify:**
- `web/src/pages/FlagDetailPage.tsx` — `handleAddRule` routes through `stageOrCall`.
- `web/src/pages/FlagDetailPage.test.tsx` — extend rule-add tests.
- `web/src/pages/StrategyEditor.tsx` — create branch routes through `stageOrCall`.
- `web/src/pages/StrategyEditor.test.tsx`
- `web/src/pages/StrategiesPage.tsx` — render `<StagedBadge>` on rows whose response carries `_staged`.

---

## PR 1 — Backend Resolution + Read-Path Overlay

### Task 1: Resolver type + tests

**Files:**
- Create: `internal/staging/resolver.go`
- Create: `internal/staging/resolver_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/staging/resolver_test.go
package staging

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
)

func TestResolverBindRejectsNonProvisionalSource(t *testing.T) {
	r := NewResolver()
	defer func() {
		if recover() == nil {
			t.Fatal("Bind should panic when source is not provisional")
		}
	}()
	r.Bind(uuid.New(), uuid.New())
}

func TestResolverBindRejectsProvisionalDestination(t *testing.T) {
	r := NewResolver()
	defer func() {
		if recover() == nil {
			t.Fatal("Bind should panic when destination is provisional")
		}
	}()
	r.Bind(NewProvisional(), NewProvisional())
}

func TestResolverLookupRoundTrip(t *testing.T) {
	r := NewResolver()
	prov := NewProvisional()
	real := uuid.New()
	r.Bind(prov, real)

	got, ok := r.Lookup(prov)
	if !ok || got != real {
		t.Fatalf("Lookup(prov) = (%v,%v), want (%v,true)", got, ok, real)
	}
	if _, ok := r.Lookup(uuid.New()); ok {
		t.Fatal("Lookup of unknown UUID should return ok=false")
	}
}

func TestRewriteUUIDsInJSONSubstitutesNested(t *testing.T) {
	r := NewResolver()
	prov := NewProvisional()
	real := uuid.New()
	r.Bind(prov, real)

	in := []byte(`{"flag_id":"` + prov.String() + `","nested":{"id":"` + prov.String() + `"},"arr":["` + prov.String() + `","not-a-uuid"]}`)
	out, err := r.RewriteUUIDsInJSON(in)
	if err != nil {
		t.Fatalf("RewriteUUIDsInJSON: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if got["flag_id"] != real.String() {
		t.Errorf("top-level not rewritten: %v", got["flag_id"])
	}
	if got["nested"].(map[string]any)["id"] != real.String() {
		t.Errorf("nested not rewritten: %v", got["nested"])
	}
	arr := got["arr"].([]any)
	if arr[0] != real.String() || arr[1] != "not-a-uuid" {
		t.Errorf("array rewrite mismatch: %v", arr)
	}
}

func TestRewriteUUIDsInJSONLeavesUnknownUUIDsAlone(t *testing.T) {
	r := NewResolver()
	r.Bind(NewProvisional(), uuid.New()) // bind something else
	other := uuid.New()
	in := []byte(`{"flag_id":"` + other.String() + `"}`)
	out, err := r.RewriteUUIDsInJSON(in)
	if err != nil {
		t.Fatalf("RewriteUUIDsInJSON: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	if got["flag_id"] != other.String() {
		t.Errorf("unknown UUID was rewritten: %v", got["flag_id"])
	}
}

func TestRewriteRowResolvesResourceIDAndJSON(t *testing.T) {
	r := NewResolver()
	prov := NewProvisional()
	real := uuid.New()
	r.Bind(prov, real)

	row := &models.StagedChange{
		ResourceID: &prov,
		NewValue:   []byte(`{"flag_id":"` + prov.String() + `"}`),
	}
	if err := r.RewriteRow(row); err != nil {
		t.Fatalf("RewriteRow: %v", err)
	}
	if row.ResourceID == nil || *row.ResourceID != real {
		t.Errorf("ResourceID not rewritten: %v", row.ResourceID)
	}
	var got map[string]any
	_ = json.Unmarshal(row.NewValue, &got)
	if got["flag_id"] != real.String() {
		t.Errorf("NewValue not rewritten: %v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/staging/ -run TestResolver -run TestRewrite -v`
Expected: FAIL with `undefined: NewResolver` and friends.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/staging/resolver.go
package staging

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
)

// Resolver maps provisional UUIDs minted during staging to the real UUIDs
// produced by the create handlers at commit time. Used inside Service.Commit
// to rewrite ResourceID + new_value/old_value JSON references on every
// dependent row before its handler runs.
type Resolver struct {
	m map[uuid.UUID]uuid.UUID
}

func NewResolver() *Resolver { return &Resolver{m: map[uuid.UUID]uuid.UUID{}} }

// Bind records that a provisional id resolves to a real id. Both invariants
// are checked: source must be provisional; destination must not be.
func (r *Resolver) Bind(provisional, real uuid.UUID) {
	if !IsProvisional(provisional) {
		panic(fmt.Sprintf("staging.Resolver.Bind: source %s is not provisional", provisional))
	}
	if IsProvisional(real) {
		panic(fmt.Sprintf("staging.Resolver.Bind: destination %s is provisional", real))
	}
	r.m[provisional] = real
}

// Lookup returns the real id bound to a provisional id, plus an ok flag.
func (r *Resolver) Lookup(id uuid.UUID) (uuid.UUID, bool) {
	real, ok := r.m[id]
	return real, ok
}

// RewriteUUIDsInJSON walks raw JSON and substitutes any string value that
// parses as a UUID and is bound in the resolver with the real id's string.
// Non-UUID strings, numbers, nulls, bools, and structural tokens pass
// through. Returns the rewritten JSON; input is not mutated.
func (r *Resolver) RewriteUUIDsInJSON(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("RewriteUUIDsInJSON: parse: %w", err)
	}
	walked := r.walk(v)
	out, err := json.Marshal(walked)
	if err != nil {
		return nil, fmt.Errorf("RewriteUUIDsInJSON: marshal: %w", err)
	}
	return out, nil
}

func (r *Resolver) walk(v any) any {
	switch t := v.(type) {
	case string:
		if u, err := uuid.Parse(t); err == nil {
			if real, ok := r.m[u]; ok {
				return real.String()
			}
		}
		return t
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = r.walk(x)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, x := range t {
			out[k] = r.walk(x)
		}
		return out
	default:
		return v
	}
}

// RewriteRow rewrites every resolvable provisional reference on a staged row:
// its ResourceID column (when provisional + bound) and its NewValue + OldValue
// JSON. Mutates row in place; returns error if JSON parse fails.
func (r *Resolver) RewriteRow(row *models.StagedChange) error {
	if row.ResourceID != nil && IsProvisional(*row.ResourceID) {
		if real, ok := r.m[*row.ResourceID]; ok {
			row.ResourceID = &real
		}
	}
	if len(row.NewValue) > 0 {
		out, err := r.RewriteUUIDsInJSON(row.NewValue)
		if err != nil {
			return fmt.Errorf("RewriteRow new_value: %w", err)
		}
		row.NewValue = out
	}
	if len(row.OldValue) > 0 {
		out, err := r.RewriteUUIDsInJSON(row.OldValue)
		if err != nil {
			return fmt.Errorf("RewriteRow old_value: %w", err)
		}
		row.OldValue = out
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/staging/ -run TestResolver -run TestRewrite -v`
Expected: PASS for all five tests.

- [ ] **Step 5: Commit**

```bash
git add internal/staging/resolver.go internal/staging/resolver_test.go
git commit -m "feat(staging): Resolver + JSON UUID rewrite"
```

---

### Task 2: Pre-flight planner + tests

**Files:**
- Create: `internal/staging/preflight.go`
- Create: `internal/staging/preflight_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/staging/preflight_test.go
package staging

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
)

func TestPlanBatchHappyPathOrdersCreatesFirst(t *testing.T) {
	provFlag := NewProvisional()
	rowCreateFlag := &models.StagedChange{
		ID:            uuid.New(),
		ResourceType:  "flag",
		Action:        "create",
		ProvisionalID: &provFlag,
		NewValue:      []byte(`{"key":"x"}`),
	}
	realRule := uuid.New()
	rowMutateRule := &models.StagedChange{
		ID:           uuid.New(),
		ResourceType: "flag_rule",
		Action:       "update",
		ResourceID:   &realRule,
		NewValue:     []byte(`{"flag_id":"` + provFlag.String() + `"}`),
	}
	plan, err := planBatch([]*models.StagedChange{rowMutateRule, rowCreateFlag})
	if err != nil {
		t.Fatalf("planBatch: %v", err)
	}
	if len(plan.ordered) != 2 {
		t.Fatalf("ordered len = %d, want 2", len(plan.ordered))
	}
	if plan.ordered[0].Action != "create" {
		t.Errorf("first row should be create, got %v", plan.ordered[0].Action)
	}
	if _, ok := plan.knownProvs[provFlag]; !ok {
		t.Errorf("knownProvs missing flag provisional")
	}
}

func TestPlanBatchRejectsUnresolvedProvisional(t *testing.T) {
	dangling := NewProvisional()
	row := &models.StagedChange{
		ID:           uuid.New(),
		ResourceType: "flag_rule",
		Action:       "update",
		ResourceID:   ptrUUID(uuid.New()),
		NewValue:     []byte(`{"flag_id":"` + dangling.String() + `"}`),
	}
	_, err := planBatch([]*models.StagedChange{row})
	var unresolved *ErrUnresolvedProvisional
	if !errors.As(err, &unresolved) {
		t.Fatalf("expected *ErrUnresolvedProvisional, got %v", err)
	}
	if unresolved.ProvUUID != dangling {
		t.Errorf("ProvUUID mismatch: got %v want %v", unresolved.ProvUUID, dangling)
	}
}

func TestPlanBatchPreservesOrderForIsolatedMutations(t *testing.T) {
	a := &models.StagedChange{ID: uuid.New(), ResourceType: "flag", Action: "toggle", ResourceID: ptrUUID(uuid.New())}
	b := &models.StagedChange{ID: uuid.New(), ResourceType: "flag", Action: "update", ResourceID: ptrUUID(uuid.New())}
	plan, err := planBatch([]*models.StagedChange{a, b})
	if err != nil {
		t.Fatalf("planBatch: %v", err)
	}
	if plan.ordered[0] != a || plan.ordered[1] != b {
		t.Fatalf("input order not preserved")
	}
}

func ptrUUID(u uuid.UUID) *uuid.UUID { return &u }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/staging/ -run TestPlanBatch -v`
Expected: FAIL with undefined `planBatch`, `ErrUnresolvedProvisional`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/staging/preflight.go
package staging

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
)

// batchPlan is the output of planBatch: rows in topo order (creates first,
// then mutations in dependency order, with the original input order preserved
// among rows that have no provisional dependency), plus the set of every
// provisional id minted by this batch.
type batchPlan struct {
	ordered    []*models.StagedChange
	knownProvs map[uuid.UUID]struct{}
}

// ErrUnresolvedProvisional is returned by planBatch when a row references a
// provisional UUID that is not minted by any row in the same batch. The
// commit endpoint surfaces it as CommitResult.FailedReason.
type ErrUnresolvedProvisional struct {
	RowID    uuid.UUID
	ProvUUID uuid.UUID
}

func (e *ErrUnresolvedProvisional) Error() string {
	return fmt.Sprintf(
		"row %s references provisional %s which is not in this deploy batch",
		e.RowID, e.ProvUUID,
	)
}

// planBatch partitions rows into creates + mutations, validates that every
// provisional reference is satisfied by a create in the same batch, and
// returns rows in dependency order. Cycle is impossible by construction
// because provisional ids only flow create → consumer.
func planBatch(rows []*models.StagedChange) (*batchPlan, error) {
	known := make(map[uuid.UUID]struct{})
	creates := make([]*models.StagedChange, 0)
	mutations := make([]*models.StagedChange, 0)
	for _, r := range rows {
		if r.ProvisionalID != nil {
			known[*r.ProvisionalID] = struct{}{}
			creates = append(creates, r)
		} else {
			mutations = append(mutations, r)
		}
	}

	for _, r := range rows {
		refs, err := collectProvisionals(r)
		if err != nil {
			return nil, fmt.Errorf("planBatch: scan row %s: %w", r.ID, err)
		}
		for ref := range refs {
			// A create row's own ProvisionalID is "known" — skip it.
			if r.ProvisionalID != nil && *r.ProvisionalID == ref {
				continue
			}
			if _, ok := known[ref]; !ok {
				return nil, &ErrUnresolvedProvisional{RowID: r.ID, ProvUUID: ref}
			}
		}
	}

	ordered := make([]*models.StagedChange, 0, len(rows))
	ordered = append(ordered, creates...)
	ordered = append(ordered, mutations...)
	return &batchPlan{ordered: ordered, knownProvs: known}, nil
}

// collectProvisionals walks ResourceID + new_value + old_value of a row and
// returns every UUID that has the provisional variant byte. Other UUIDs and
// non-UUID strings are ignored.
func collectProvisionals(row *models.StagedChange) (map[uuid.UUID]struct{}, error) {
	out := make(map[uuid.UUID]struct{})
	if row.ResourceID != nil && IsProvisional(*row.ResourceID) {
		out[*row.ResourceID] = struct{}{}
	}
	for _, raw := range [][]byte{row.NewValue, row.OldValue} {
		if len(raw) == 0 {
			continue
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
		walkProvisionals(v, out)
	}
	return out, nil
}

func walkProvisionals(v any, out map[uuid.UUID]struct{}) {
	switch t := v.(type) {
	case string:
		if u, err := uuid.Parse(t); err == nil && IsProvisional(u) {
			out[u] = struct{}{}
		}
	case []any:
		for _, x := range t {
			walkProvisionals(x, out)
		}
	case map[string]any:
		for _, x := range t {
			walkProvisionals(x, out)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/staging/ -run TestPlanBatch -v`
Expected: PASS for all three tests.

- [ ] **Step 5: Commit**

```bash
git add internal/staging/preflight.go internal/staging/preflight_test.go
git commit -m "feat(staging): pre-flight planner + ErrUnresolvedProvisional"
```

---

### Task 3: CreateRegistry + tests

**Files:**
- Modify: `internal/staging/registry.go`
- Modify: `internal/staging/registry_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/staging/registry_test.go`:

```go
func TestCreateRegistryDispatchReturnsRealID(t *testing.T) {
	r := NewCreateRegistry()
	wantReal := uuid.New()
	called := false
	r.Register("flag", "create", func(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (uuid.UUID, string, func(context.Context), error) {
		called = true
		return wantReal, "flag.created", nil, nil
	})
	row := &models.StagedChange{ResourceType: "flag", Action: "create"}
	gotReal, audit, hook, err := r.Dispatch(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if !called {
		t.Fatal("handler not invoked")
	}
	if gotReal != wantReal {
		t.Errorf("realID mismatch: got %v want %v", gotReal, wantReal)
	}
	if audit != "flag.created" {
		t.Errorf("audit mismatch: %v", audit)
	}
	if hook != nil {
		t.Errorf("expected nil postCommit hook")
	}
}

func TestCreateRegistryIsCreatable(t *testing.T) {
	r := NewCreateRegistry()
	if r.IsCreatable("flag", "create") {
		t.Fatal("empty registry should not report flag.create creatable")
	}
	r.Register("flag", "create", func(context.Context, pgx.Tx, *models.StagedChange) (uuid.UUID, string, func(context.Context), error) {
		return uuid.Nil, "", nil, nil
	})
	if !r.IsCreatable("flag", "create") {
		t.Fatal("registered key should be creatable")
	}
}

func TestCreateRegistryDispatchUnknownErrors(t *testing.T) {
	r := NewCreateRegistry()
	row := &models.StagedChange{ResourceType: "flag", Action: "create"}
	_, _, _, err := r.Dispatch(context.Background(), nil, row)
	if !errors.Is(err, ErrNoCreateHandler) {
		t.Errorf("expected ErrNoCreateHandler, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/staging/ -run TestCreateRegistry -v`
Expected: FAIL with `undefined: NewCreateRegistry`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/staging/registry.go`:

```go
// CreateHandler applies a staged CREATE row to its production table inside
// an open pgx transaction. It returns the real UUID minted for the new
// resource (so the staging service can bind provisional → real in the
// resolver), the audit-log action string, an optional post-commit hook for
// cache invalidation / event publish (run only after tx.Commit succeeds),
// and an error.
type CreateHandler func(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (realID uuid.UUID, auditAction string, postCommit func(context.Context), err error)

// ErrNoCreateHandler is returned by CreateRegistry.Dispatch when no create
// handler is registered for the (resource_type, action) pair.
var ErrNoCreateHandler = errors.New("staging: no create handler registered")

// CreateRegistry maps (resource_type, action) → CreateHandler. Sits beside
// the existing CommitRegistry; Service.Commit dispatches to whichever fits
// based on row.ProvisionalID being non-nil.
type CreateRegistry struct {
	handlers map[string]CreateHandler
}

// NewCreateRegistry builds an empty registry.
func NewCreateRegistry() *CreateRegistry { return &CreateRegistry{handlers: map[string]CreateHandler{}} }

// Register installs h for the given (resource_type, action) pair.
func (r *CreateRegistry) Register(resourceType, action string, h CreateHandler) {
	r.handlers[commitKey(resourceType, action)] = h
}

// IsCreatable reports whether a create handler is registered for the pair.
func (r *CreateRegistry) IsCreatable(resourceType, action string) bool {
	_, ok := r.handlers[commitKey(resourceType, action)]
	return ok
}

// Dispatch runs the create handler for row.
func (r *CreateRegistry) Dispatch(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (uuid.UUID, string, func(context.Context), error) {
	h, ok := r.handlers[commitKey(row.ResourceType, row.Action)]
	if !ok {
		return uuid.Nil, "", nil, fmt.Errorf("%w for %s.%s", ErrNoCreateHandler, row.ResourceType, row.Action)
	}
	return h(ctx, tx, row)
}
```

Add the import for `uuid` if not already present:

```go
import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shadsorg/deploysentry/internal/models"
)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/staging/ -v`
Expected: PASS for the three new tests + all existing.

- [ ] **Step 5: Commit**

```bash
git add internal/staging/registry.go internal/staging/registry_test.go
git commit -m "feat(staging): CreateRegistry + CreateHandler shape"
```

---

### Task 4: Tx-aware FlagRepository.CreateFlagTx + CreateRuleTx

**Files:**
- Modify: `internal/flags/repository.go`
- Modify: `internal/platform/database/postgres/flags.go`

- [ ] **Step 1: Add interface methods**

In `internal/flags/repository.go`, add to the `FlagRepository` interface (after the existing `CreateFlag`):

```go
// CreateFlagTx persists a new feature flag through an open transaction.
// Used by the staging service so the create rides the same tx as the
// rest of the deploy batch — a later handler error rolls the create back.
CreateFlagTx(ctx context.Context, tx pgx.Tx, flag *models.FeatureFlag) error
```

And next to the existing `CreateRule`:

```go
// CreateRuleTx persists a new targeting rule through an open transaction.
// Same staging-tx rationale as CreateFlagTx.
CreateRuleTx(ctx context.Context, tx pgx.Tx, rule *models.TargetingRule) error
```

Add `pgx` import if missing (verify `import "github.com/jackc/pgx/v5"` is present).

- [ ] **Step 2: Implement CreateFlagTx in Postgres**

In `internal/platform/database/postgres/flags.go`, factor the existing `CreateFlag` query into a helper that takes an `Execer` so both `pool.Exec` and `tx.Exec` paths share the SQL. Replace the existing `CreateFlag` (around line 183) with:

```go
type flagExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (r *FlagRepository) CreateFlag(ctx context.Context, flag *models.FeatureFlag) error {
	return r.createFlagOn(ctx, r.pool, flag)
}

func (r *FlagRepository) CreateFlagTx(ctx context.Context, tx pgx.Tx, flag *models.FeatureFlag) error {
	return r.createFlagOn(ctx, tx, flag)
}

func (r *FlagRepository) createFlagOn(ctx context.Context, ex flagExecer, flag *models.FeatureFlag) error {
	if flag.ID == uuid.Nil {
		flag.ID = uuid.New()
	}
	now := time.Now().UTC()
	flag.CreatedAt = now
	flag.UpdatedAt = now

	defaultValueJSON := flag.DefaultValue
	if defaultValueJSON == "" {
		defaultValueJSON = "null"
	}

	const q = `
		INSERT INTO feature_flags
			(id, project_id, environment_id, key, name, description, flag_type,
			 default_value, enabled, tags, created_by, created_at, updated_at,
			 category, purpose, owners, is_permanent, expires_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7,
			 $8, $9, $10, $11, $12, $13,
			 $14, $15, $16, $17, $18)`

	_, err := ex.Exec(ctx, q,
		flag.ID,
		flag.ProjectID,
		flag.EnvironmentID,
		flag.Key,
		flag.Name,
		flag.Description,
		flag.FlagType,
		[]byte(defaultValueJSON),
		flag.Enabled,
		flag.Tags,
		flag.CreatedBy,
		flag.CreatedAt,
		flag.UpdatedAt,
		flag.Category,
		flag.Purpose,
		flag.Owners,
		flag.IsPermanent,
		flag.ExpiresAt,
	)
	return err
}
```

Add the import for `pgconn` and `pgx` near the top of the file (verify `"github.com/jackc/pgx/v5/pgconn"` and `"github.com/jackc/pgx/v5"` exist).

- [ ] **Step 3: Implement CreateRuleTx in Postgres**

Same shape, applied to the existing `CreateRule` (around line 619). Replace it with `CreateRule` + `CreateRuleTx` + `createRuleOn(ctx, ex, rule)`. The body is byte-identical to the existing function except `r.pool.Exec(ctx, ...)` becomes `ex.Exec(ctx, ...)`.

- [ ] **Step 4: Run build + existing tests**

Run: `go build ./... && go test ./internal/flags/... -short -v`
Expected: build clean, existing tests pass (no behavior change for non-tx callers).

- [ ] **Step 5: Commit**

```bash
git add internal/flags/repository.go internal/platform/database/postgres/flags.go
git commit -m "feat(flags): CreateFlagTx / CreateRuleTx for staging-tx commits"
```

---

### Task 5: Tx-aware FlagService.CreateFlagTx + AddRuleTx

**Files:**
- Modify: `internal/flags/service.go`

- [ ] **Step 1: Add interface methods**

In `internal/flags/service.go`, add to the `FlagService` interface (right after `CreateFlag`):

```go
// CreateFlagTx mints a fresh real id, runs validation, and writes through
// the supplied tx. Cache invalidation + NATS publish are deferred — the
// caller (staging service) runs them post-tx.Commit via the post-commit hook.
CreateFlagTx(ctx context.Context, tx pgx.Tx, flag *models.FeatureFlag) (uuid.UUID, error)
```

And next to `AddRule`:

```go
// AddRuleTx is the tx-aware twin of AddRule. Same caching/event deferral.
AddRuleTx(ctx context.Context, tx pgx.Tx, rule *models.TargetingRule) (uuid.UUID, error)
```

- [ ] **Step 2: Implement on flagService**

Replace the existing `CreateFlag` body (around line 207) so it composes the tx version, and add the tx version:

```go
func (s *flagService) CreateFlag(ctx context.Context, flag *models.FeatureFlag) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("CreateFlag begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := s.CreateFlagTx(ctx, tx, flag); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("CreateFlag commit tx: %w", err)
	}
	s.publishEvent(ctx, "created", flag)
	return nil
}

func (s *flagService) CreateFlagTx(ctx context.Context, tx pgx.Tx, flag *models.FeatureFlag) (uuid.UUID, error) {
	flag.ID = uuid.New() // always mint fresh; ignore any id smuggled in
	now := time.Now().UTC()
	flag.CreatedAt = now
	flag.UpdatedAt = now

	if err := flag.Validate(); err != nil {
		return uuid.Nil, fmt.Errorf("validation failed: %w", err)
	}
	if err := s.repo.CreateFlagTx(ctx, tx, flag); err != nil {
		return uuid.Nil, fmt.Errorf("creating flag: %w", err)
	}
	return flag.ID, nil
}
```

Apply the same shape to `AddRule` / `AddRuleTx` (around line 393). The cache invalidate stays in the non-tx caller (`AddRule` calls `s.cache.Invalidate(ctx, rule.FlagID)` post-`tx.Commit`).

The `flagService` struct already has access to a connection pool via the existing repo — but the service may not currently hold the pool directly. Check the constructor: if it doesn't have `*pgxpool.Pool`, add a `pool *pgxpool.Pool` field and accept it in `NewFlagService`. Verify call sites in `cmd/api/main.go` are updated. (If existing service shape already wraps a tx-capable repo, prefer extending the repo shape instead.)

- [ ] **Step 3: Run build**

Run: `go build ./...`
Expected: clean. If `flagService` lacks pool access, fix wiring as above.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/flags/... -short -v`
Expected: existing tests pass; `CreateFlag` behavior unchanged from caller's POV.

- [ ] **Step 5: Commit**

```bash
git add internal/flags/service.go cmd/api/main.go
git commit -m "feat(flags): tx-aware CreateFlagTx / AddRuleTx; non-tx callers compose"
```

---

### Task 6: Tx-aware rollout.Service.CreateStrategyTx

**Files:**
- Modify: `internal/rollout/service.go`
- Modify: `internal/rollout/repository.go` (or postgres impl) — same CreateStrategy → CreateStrategyTx split.

- [ ] **Step 1: Add interface method**

In `internal/rollout/service.go`'s `Service` interface, after the existing `CreateStrategy`:

```go
// CreateStrategyTx is the tx-aware twin of CreateStrategy used by the staging
// service. Mints a real id, runs validation, defers cache + event work to the
// staging post-commit hook.
CreateStrategyTx(ctx context.Context, tx pgx.Tx, strategy *models.RolloutStrategy) (uuid.UUID, error)
```

- [ ] **Step 2: Mirror Task 4–5 pattern**

Add `CreateStrategyTx` on the repo impl (postgres), have the existing `CreateStrategy` compose it via `pool.BeginTx → CreateStrategyTx → tx.Commit`, defer side effects post-commit.

- [ ] **Step 3: Run build + tests**

Run: `go build ./... && go test ./internal/rollout/... -short -v`
Expected: clean + pass.

- [ ] **Step 4: Commit**

```bash
git add internal/rollout/service.go internal/rollout/repository.go internal/platform/database/postgres/rollout.go
git commit -m "feat(rollout): CreateStrategyTx for staging-tx commits"
```

---

### Task 7: commitFlagCreate handler + test

**Files:**
- Modify: `internal/flags/staging_commit.go`
- Modify: `internal/flags/staging_commit_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/flags/staging_commit_test.go`:

```go
func TestCommitFlagCreateMintsRealIDAndDefersHook(t *testing.T) {
	calls := []string{}
	svc := &fakeFlagService{
		createFlagTx: func(ctx context.Context, tx pgx.Tx, flag *models.FeatureFlag) (uuid.UUID, error) {
			calls = append(calls, "createFlagTx")
			real := uuid.New()
			flag.ID = real
			return real, nil
		},
		invalidate: func(ctx context.Context, flagID uuid.UUID) error {
			calls = append(calls, "invalidate")
			return nil
		},
		publish: func(ctx context.Context, eventType string, flag *models.FeatureFlag) {
			calls = append(calls, "publish:"+eventType)
		},
	}
	prov := staging.NewProvisional()
	row := &models.StagedChange{
		ResourceType:  "flag",
		Action:        "create",
		ProvisionalID: &prov,
		NewValue:      []byte(`{"key":"new","name":"New","project_id":"` + uuid.New().String() + `"}`),
	}
	h := commitFlagCreate(svc)
	realID, action, hook, err := h(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if realID == uuid.Nil || staging.IsProvisional(realID) {
		t.Fatalf("realID invalid: %v", realID)
	}
	if action != "flag.created" {
		t.Errorf("action: %v", action)
	}
	if hook == nil {
		t.Fatal("post-commit hook should be non-nil for create handler")
	}
	if got := strings.Join(calls, ","); got != "createFlagTx" {
		t.Errorf("hooks fired inside handler: %v", got)
	}
	hook(context.Background())
	got := strings.Join(calls, ",")
	if !strings.Contains(got, "publish:created") {
		t.Errorf("post-commit hook did not publish: %v", got)
	}
}
```

(The existing `fakeFlagService` in the test file may need new fields — `createFlagTx`, `invalidate`, `publish` — added to its struct.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/flags/ -run TestCommitFlagCreate -v`
Expected: FAIL — `commitFlagCreate` undefined.

- [ ] **Step 3: Write minimal implementation**

In `internal/flags/staging_commit.go`, replace the existing `FlagCommitHandlers` returning `[]CommitTuple` so the file also exposes a sibling `FlagCreateHandlers` slice — add a new tuple type to keep wiring uniform:

```go
type CreateTuple struct {
	ResourceType string
	Action       string
	Handler      staging.CreateHandler
}

func FlagCreateHandlers(svc FlagService) []CreateTuple {
	return []CreateTuple{
		{ResourceType: "flag", Action: "create", Handler: commitFlagCreate(svc)},
		{ResourceType: "flag_rule", Action: "create", Handler: commitFlagRuleCreate(svc)},
	}
}

func commitFlagCreate(svc FlagService) staging.CreateHandler {
	return func(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (uuid.UUID, string, func(context.Context), error) {
		if len(row.NewValue) == 0 {
			return uuid.Nil, "", nil, fmt.Errorf("flag.create commit: new_value required")
		}
		var flag models.FeatureFlag
		if err := json.Unmarshal(row.NewValue, &flag); err != nil {
			return uuid.Nil, "", nil, fmt.Errorf("flag.create commit: parse new_value: %w", err)
		}
		flag.ID = uuid.Nil // force fresh id; staging owns provisional, never the real
		realID, err := svc.CreateFlagTx(ctx, tx, &flag)
		if err != nil {
			return uuid.Nil, "", nil, fmt.Errorf("flag.create commit: %w", err)
		}
		// Capture for the post-commit hook so the closure doesn't reference
		// row state that could have been rewritten by a later resolver pass.
		hook := func(hookCtx context.Context) {
			svc.PublishCreated(hookCtx, &flag) // see Step 4
		}
		return realID, "flag.created", hook, nil
	}
}
```

- [ ] **Step 4: Expose post-commit publish hook on the service**

Either: (a) export `PublishCreated(ctx, *FeatureFlag)` on `FlagService` that wraps the existing internal `publishEvent(ctx, "created", flag)` for use by the staging hook, or (b) inline the publish call directly in the handler if `flagService.publishEvent` is already package-visible. Pick whichever matches the repo's idiom. The new method goes alongside other `FlagService` methods.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/flags/ -run TestCommitFlagCreate -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/flags/staging_commit.go internal/flags/staging_commit_test.go internal/flags/service.go
git commit -m "feat(flags): commitFlagCreate handler + post-commit publish hook"
```

---

### Task 8: commitFlagRuleCreate handler + test

**Files:**
- Modify: `internal/flags/staging_commit.go`
- Modify: `internal/flags/staging_commit_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestCommitFlagRuleCreateMintsRealID(t *testing.T) {
	wantReal := uuid.New()
	flagID := uuid.New()
	svc := &fakeFlagService{
		addRuleTx: func(ctx context.Context, tx pgx.Tx, rule *models.TargetingRule) (uuid.UUID, error) {
			rule.ID = wantReal
			return wantReal, nil
		},
		invalidate: func(ctx context.Context, fID uuid.UUID) error {
			if fID != flagID {
				t.Errorf("invalidate target mismatch: %v vs %v", fID, flagID)
			}
			return nil
		},
	}
	prov := staging.NewProvisional()
	row := &models.StagedChange{
		ResourceType:  "flag_rule",
		Action:        "create",
		ProvisionalID: &prov,
		NewValue:      []byte(`{"flag_id":"` + flagID.String() + `","priority":1}`),
	}
	realID, action, hook, err := commitFlagRuleCreate(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if realID != wantReal {
		t.Errorf("realID: got %v want %v", realID, wantReal)
	}
	if action != "flag.rule.created" {
		t.Errorf("action: %v", action)
	}
	hook(context.Background()) // assert no panic; cache invalidate fires
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/flags/ -run TestCommitFlagRuleCreate -v`
Expected: FAIL — `commitFlagRuleCreate` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
func commitFlagRuleCreate(svc FlagService) staging.CreateHandler {
	return func(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (uuid.UUID, string, func(context.Context), error) {
		if len(row.NewValue) == 0 {
			return uuid.Nil, "", nil, fmt.Errorf("flag_rule.create commit: new_value required")
		}
		var rule models.TargetingRule
		if err := json.Unmarshal(row.NewValue, &rule); err != nil {
			return uuid.Nil, "", nil, fmt.Errorf("flag_rule.create commit: parse new_value: %w", err)
		}
		if rule.FlagID == uuid.Nil {
			return uuid.Nil, "", nil, fmt.Errorf("flag_rule.create commit: flag_id required in new_value")
		}
		// flag_id may be provisional at the moment the row reaches the handler
		// only when a *.create row precedes it AND the resolver hasn't yet
		// rewritten — Service.Commit always runs RewriteRow before this
		// handler so we can assert non-provisional here.
		staging.MustNotBeProvisional(rule.FlagID, "flag_rule.create.flag_id")
		rule.ID = uuid.Nil
		realID, err := svc.AddRuleTx(ctx, tx, &rule)
		if err != nil {
			return uuid.Nil, "", nil, fmt.Errorf("flag_rule.create commit: %w", err)
		}
		flagID := rule.FlagID
		hook := func(hookCtx context.Context) {
			_ = svc.InvalidateFlagCache(hookCtx, flagID)
		}
		return realID, "flag.rule.created", hook, nil
	}
}
```

Add `InvalidateFlagCache(ctx, flagID)` to `FlagService` if not already exposed — it's a thin wrapper around the existing `s.cache.Invalidate`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/flags/ -run TestCommitFlagRuleCreate -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/flags/staging_commit.go internal/flags/staging_commit_test.go internal/flags/service.go
git commit -m "feat(flags): commitFlagRuleCreate handler"
```

---

### Task 9: commitStrategyCreate handler + test

**Files:**
- Modify: `internal/rollout/staging_commit.go`
- Modify: `internal/rollout/staging_commit_test.go`

- [ ] **Step 1: Write the failing test**

Mirror Task 7's shape: a fake `rollout.Service` whose `CreateStrategyTx` mints a deterministic id; the handler returns `(realID, "strategy.created", postCommit, nil)`; post-commit hook fires the existing strategy event publish.

```go
func TestCommitStrategyCreateMintsRealID(t *testing.T) {
	wantReal := uuid.New()
	svc := &fakeRolloutService{
		createStrategyTx: func(ctx context.Context, tx pgx.Tx, s *models.RolloutStrategy) (uuid.UUID, error) {
			s.ID = wantReal
			return wantReal, nil
		},
	}
	prov := staging.NewProvisional()
	row := &models.StagedChange{
		ResourceType:  "strategy",
		Action:        "create",
		ProvisionalID: &prov,
		NewValue:      []byte(`{"name":"canary","steps":[]}`),
	}
	realID, action, hook, err := commitStrategyCreate(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if realID != wantReal {
		t.Errorf("realID: got %v want %v", realID, wantReal)
	}
	if action != "strategy.created" {
		t.Errorf("action: %v", action)
	}
	if hook == nil {
		t.Fatal("hook should be non-nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/rollout/ -run TestCommitStrategyCreate -v`
Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**

In `internal/rollout/staging_commit.go`, add `CreateTuple` mirror of the flag side, expose `StrategyCreateHandlers(svc) []CreateTuple` returning `{ResourceType: "strategy", Action: "create", Handler: commitStrategyCreate(svc)}`. Implement `commitStrategyCreate` with the same skeleton: unmarshal, force fresh id, `svc.CreateStrategyTx(ctx, tx, &strategy)`, return real id + post-commit hook that publishes the create event.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/rollout/ -run TestCommitStrategyCreate -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/rollout/staging_commit.go internal/rollout/staging_commit_test.go internal/rollout/service.go
git commit -m "feat(rollout): commitStrategyCreate handler"
```

---

### Task 10: Service.Commit pipeline rewrite

**Files:**
- Modify: `internal/staging/service.go`
- Modify: `internal/staging/service_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/staging/service_test.go`:

```go
func TestCommitResolvesProvisionalAcrossBatch(t *testing.T) {
	pool := newTestPool(t) // existing helper
	repo := newRepo(pool)
	commitReg := NewCommitRegistry()
	createReg := NewCreateRegistry()

	wantRealFlag := uuid.New()
	createReg.Register("flag", "create", func(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (uuid.UUID, string, func(context.Context), error) {
		return wantRealFlag, "flag.created", nil, nil
	})
	var seenFlagID uuid.UUID
	commitReg.Register("flag_rule", "update", func(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (string, error) {
		var p map[string]any
		_ = json.Unmarshal(row.NewValue, &p)
		seenFlagID, _ = uuid.Parse(p["flag_id"].(string))
		return "flag.rule.updated", nil
	})

	svc := NewService(repo, commitReg, createReg, pool, nil)

	prov := NewProvisional()
	user, org := uuid.New(), uuid.New()
	createRow := &models.StagedChange{ID: uuid.New(), UserID: user, OrgID: org, ResourceType: "flag", Action: "create", ProvisionalID: &prov, NewValue: []byte(`{}`)}
	mutateRow := &models.StagedChange{ID: uuid.New(), UserID: user, OrgID: org, ResourceType: "flag_rule", Action: "update", ResourceID: ptrUUID(uuid.New()), NewValue: []byte(`{"flag_id":"` + prov.String() + `"}`)}
	if err := repo.Upsert(ctx, createRow); err != nil { t.Fatal(err) }
	if err := repo.Upsert(ctx, mutateRow); err != nil { t.Fatal(err) }

	res, err := svc.Commit(ctx, user, org, user, []uuid.UUID{createRow.ID, mutateRow.ID})
	if err != nil { t.Fatal(err) }
	if len(res.CommittedIDs) != 2 { t.Fatalf("committed: %v", res.CommittedIDs) }
	if seenFlagID != wantRealFlag {
		t.Errorf("rule handler saw flag_id %v, want resolved %v", seenFlagID, wantRealFlag)
	}
}

func TestCommitRefusesUnresolvedProvisional(t *testing.T) {
	// Stage a flag_rule.update referencing a provisional that's not in batch;
	// expect FailedReason naming the missing parent.
	// (Setup mirrors above with only the mutate row; assert FailedReason
	//  contains "is not in this deploy batch".)
}
```

- [ ] **Step 2: Rewrite Service.Commit**

Replace the existing `Commit` body in `internal/staging/service.go`:

```go
type Service struct {
	repo    Repository
	reg     *CommitRegistry
	creates *CreateRegistry
	pool    *pgxpool.Pool
	audit   AuditWriter
}

func NewService(repo Repository, reg *CommitRegistry, creates *CreateRegistry, pool *pgxpool.Pool, audit AuditWriter) *Service {
	return &Service{repo: repo, reg: reg, creates: creates, pool: pool, audit: audit}
}

func (s *Service) Commit(ctx context.Context, userID, orgID, actorID uuid.UUID, ids []uuid.UUID) (*CommitResult, error) {
	if len(ids) == 0 {
		return &CommitResult{}, nil
	}
	rows, err := s.repo.GetByIDs(ctx, userID, orgID, ids)
	if err != nil {
		return nil, fmt.Errorf("staging.Commit: load rows: %w", err)
	}
	if len(rows) != len(ids) {
		return nil, fmt.Errorf("staging.Commit: %d of %d rows not found or not owned by user", len(ids)-len(rows), len(ids))
	}

	plan, err := planBatch(rows)
	if err != nil {
		var unresolved *ErrUnresolvedProvisional
		if errors.As(err, &unresolved) {
			rid := unresolved.RowID
			return &CommitResult{FailedID: &rid, FailedReason: unresolved.Error()}, nil
		}
		return nil, fmt.Errorf("staging.Commit: preflight: %w", err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("staging.Commit: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	resolver := NewResolver()
	committed := make([]uuid.UUID, 0, len(plan.ordered))
	auditEntries := make([]*models.AuditLogEntry, 0, len(plan.ordered))
	postCommitHooks := make([]func(context.Context), 0)

	for _, r := range plan.ordered {
		if r.ProvisionalID != nil && s.creates.IsCreatable(r.ResourceType, r.Action) {
			realID, auditAction, hook, err := s.creates.Dispatch(ctx, tx, r)
			if err != nil {
				return &CommitResult{CommittedIDs: committed, FailedID: &r.ID, FailedReason: err.Error()}, nil
			}
			resolver.Bind(*r.ProvisionalID, realID)
			if hook != nil {
				postCommitHooks = append(postCommitHooks, hook)
			}
			entry := buildAuditEntry(r, actorID, auditAction)
			entry.EntityID = realID
			MustNotBeProvisional(entry.EntityID, "audit_log.entity_id (create)")
			auditEntries = append(auditEntries, entry)
			committed = append(committed, r.ID)
			continue
		}

		if err := resolver.RewriteRow(r); err != nil {
			return &CommitResult{CommittedIDs: committed, FailedID: &r.ID, FailedReason: err.Error()}, nil
		}
		auditAction, err := s.reg.Dispatch(ctx, tx, r)
		if err != nil {
			return &CommitResult{CommittedIDs: committed, FailedID: &r.ID, FailedReason: err.Error()}, nil
		}
		entry := buildAuditEntry(r, actorID, auditAction)
		MustNotBeProvisional(entry.EntityID, "audit_log.entity_id")
		auditEntries = append(auditEntries, entry)
		committed = append(committed, r.ID)
	}

	if err := s.repo.DeleteByIDsTx(ctx, tx, userID, orgID, committed); err != nil {
		return nil, fmt.Errorf("staging.Commit: delete staged rows: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("staging.Commit: tx commit: %w", err)
	}

	for _, hook := range postCommitHooks {
		hook(ctx)
	}

	if s.audit != nil {
		for _, e := range auditEntries {
			if writeErr := s.audit.WriteAuditLog(ctx, e); writeErr != nil {
				err = errors.Join(err, fmt.Errorf("audit row for %s: %w", e.EntityID, writeErr))
			}
		}
	}
	return &CommitResult{CommittedIDs: committed}, err
}
```

Drop the existing "Phase A no commit handler" branch and replace with the create-registry branch above.

- [ ] **Step 3: Add Stage validation for provisional variant byte**

In the same file's `Stage` method, after the existing nil checks:

```go
if row.ProvisionalID != nil && !IsProvisional(*row.ProvisionalID) {
	return errors.New("staging.Stage: provisional_id must use the provisional UUID variant byte")
}
if row.ResourceID != nil && IsProvisional(*row.ResourceID) {
	// ResourceID may carry a provisional only when it references another
	// staged create's provisional id (e.g. a future patch). For now reject
	// it — the dashboard should not be sending provisional resource_ids.
	return errors.New("staging.Stage: resource_id must not be provisional")
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/staging/ -v`
Expected: all existing + new tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/staging/service.go internal/staging/service_test.go
git commit -m "feat(staging): four-phase Commit pipeline with provisional resolution"
```

---

### Task 11: Wire CreateRegistry in cmd/api/main.go

**Files:**
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Update wiring**

Around line 277 (the existing `stagingRegistry := staging.NewCommitRegistry()` block), add:

```go
stagingRegistry := staging.NewCommitRegistry()
stagingCreateRegistry := staging.NewCreateRegistry()
for _, t := range flags.FlagCommitHandlers(flagService) {
	stagingRegistry.Register(t.ResourceType, t.Action, t.Handler)
}
for _, t := range flags.FlagCreateHandlers(flagService) {
	stagingCreateRegistry.Register(t.ResourceType, t.Action, t.Handler)
}
stagingService := staging.NewService(stagedChangesRepo, stagingRegistry, stagingCreateRegistry, db.Pool, auditRepo)
```

After `rolloutService` (or wherever it's constructed), register strategy creates:

```go
for _, t := range rollout.StrategyCreateHandlers(rolloutService) {
	stagingCreateRegistry.Register(t.ResourceType, t.Action, t.Handler)
}
```

- [ ] **Step 2: Run build**

Run: `go build ./...`
Expected: clean.

- [ ] **Step 3: Run all unit tests**

Run: `go test ./... -short`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat(staging): wire CreateRegistry + flag.create / flag_rule.create / strategy.create handlers"
```

---

### Task 12: Read-overlay marker emission

**Files:**
- Modify: `internal/staging/overlay.go`
- Modify: `internal/staging/overlay_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestOverlayListMarkedAttachesEnvelopeForCreatesAndUpdates(t *testing.T) {
	type flagDTO struct {
		ID     uuid.UUID `json:"id"`
		Staged *Marker   `json:"_staged,omitempty"`
		Key    string    `json:"key"`
	}
	prov := NewProvisional()
	staged := []*models.StagedChange{
		{ID: uuid.New(), Action: "create", ProvisionalID: &prov, NewValue: []byte(`{"key":"new"}`)},
	}
	got := OverlayListMarked(
		[]flagDTO{},
		staged,
		func(f flagDTO) uuid.UUID { return f.ID },
		func(f flagDTO, s *models.StagedChange) (flagDTO, bool) { return f, false },
		func(s *models.StagedChange) (flagDTO, bool) { return flagDTO{ID: *s.ProvisionalID, Key: "new"}, true },
		func(f *flagDTO, m Marker) { f.Staged = &m },
	)
	if len(got) != 1 || got[0].Staged == nil || got[0].Staged.Action != "create" {
		t.Errorf("expected synthetic flag with _staged marker, got %+v", got)
	}
}
```

- [ ] **Step 2: Add Marker type + OverlayListMarked**

Append to `internal/staging/overlay.go`:

```go
// Marker is the wire-format envelope attached to overlay-emitted resources.
// Dashboard renders <StagedBadge> when present; SDK responses don't carry
// the include_my_staged param so they never see it.
type Marker struct {
	ProvisionalID *uuid.UUID `json:"provisional_id,omitempty"`
	Action        string     `json:"action"`
	StagedAt      time.Time  `json:"staged_at"`
}

// SetMarkerFunc lets a typed DTO carry the envelope. Each handler defines
// how to splice Marker into its response struct (typically by adding a
// `Staged *Marker `json:"_staged,omitempty"`` field).
type SetMarkerFunc[T any] func(target *T, m Marker)

// OverlayListMarked behaves like OverlayList but additionally tags every
// emitted row that came from a staged change with a Marker via setMarker.
func OverlayListMarked[T any](
	production []T,
	staged []*models.StagedChange,
	id ResourceID[T],
	apply ApplyFunc[T],
	synth SyntheticFunc[T],
	setMarker SetMarkerFunc[T],
) []T {
	if len(staged) == 0 {
		return production
	}
	updatesByID := make(map[uuid.UUID][]*models.StagedChange, len(staged))
	creates := make([]*models.StagedChange, 0)
	for _, s := range staged {
		switch s.Action {
		case "create":
			creates = append(creates, s)
		default:
			if s.ResourceID != nil {
				updatesByID[*s.ResourceID] = append(updatesByID[*s.ResourceID], s)
			}
		}
	}

	out := make([]T, 0, len(production)+len(creates))
	for _, p := range production {
		pid := id(p)
		patched := p
		dropped := false
		var lastApplied *models.StagedChange
		for _, s := range updatesByID[pid] {
			next, drop := apply(patched, s)
			if drop {
				dropped = true
				break
			}
			patched = next
			lastApplied = s
		}
		if !dropped {
			if lastApplied != nil && setMarker != nil {
				setMarker(&patched, Marker{Action: lastApplied.Action, StagedAt: lastApplied.CreatedAt})
			}
			out = append(out, patched)
		}
	}

	for _, s := range creates {
		v, ok := synth(s)
		if !ok {
			continue
		}
		if setMarker != nil {
			setMarker(&v, Marker{ProvisionalID: s.ProvisionalID, Action: "create", StagedAt: s.CreatedAt})
		}
		out = append(out, v)
	}

	return out
}
```

Add `"time"` import.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/staging/ -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/staging/overlay.go internal/staging/overlay_test.go
git commit -m "feat(staging): OverlayListMarked + Marker envelope"
```

---

### Task 13: Wire overlay on flag/rule/strategy list handlers

**Files:**
- Modify: `internal/flags/handler.go` (`ListFlags`, `ListRules`)
- Modify: `internal/rollout/handler.go` (`ListStrategies`)
- Modify: `internal/models/feature_flag.go` and `internal/models/targeting_rule.go` and `internal/models/rollout_strategy.go` — add `Staged *staging.Marker `json:"_staged,omitempty"`` (or use a wrapper DTO if you don't want the model to import staging).

- [ ] **Step 1: Add envelope field**

Pick one approach. The cleaner option is a per-handler response DTO that wraps the model with `_staged`:

```go
// internal/flags/handler.go
type flagWithMarker struct {
	*models.FeatureFlag
	Staged *staging.Marker `json:"_staged,omitempty"`
}
```

(Avoids cyclic imports between models and staging; keeps the wire shape additive.)

- [ ] **Step 2: Apply overlay in ListFlags**

In `ListFlags`, after loading the production rows from `s.flags.ListFlags(ctx, projectID, opts)`:

```go
includeStaged := c.Query("include_my_staged") == "true"
if !includeStaged {
	c.JSON(http.StatusOK, flagsResp)
	return
}

userID := auth.UserIDFromCtx(c)
orgID := orgIDFromCtx(c) // via existing middleware
stagedRows, err := h.staging.ListForResource(c.Request.Context(), userID, orgID, "flag")
if err != nil {
	// log + degrade: return prod rows only
}

merged := staging.OverlayListMarked(
	wrap(flags), // []flagWithMarker
	stagedRows,
	func(f flagWithMarker) uuid.UUID { return f.ID },
	flagApplyFunc(projectID),    // patches whole-flag updates / archives in place
	flagSynthFunc(projectID),    // builds a synthetic flagWithMarker from row.NewValue, ID = *row.ProvisionalID
	func(f *flagWithMarker, m staging.Marker) { f.Staged = &m },
)
c.JSON(http.StatusOK, merged)
```

Implement `flagApplyFunc(projectID)` and `flagSynthFunc(projectID)`:

- `flagApplyFunc` switches on `staged.Action`:
  - `archive` / `restore` / `update` / `toggle`: deserialize `staged.NewValue` into a partial / whole `FeatureFlag`, merge fields onto `production`. For `archive`, set `ArchivedAt = now-ish`; for `restore`, clear it.
- `flagSynthFunc` synthesizes a flag from `staged.NewValue`, validates `project_id` matches; returns ok=false to filter staged creates that belong to a different project.

- [ ] **Step 3: Apply overlay in ListRules**

Same pattern, scoped by parent flag id from URL. Synth function checks `staged.NewValue.flag_id` matches the URL flag id (or matches a provisional flag whose detail is also showing, when the user is on a provisional flag's detail page — see Task 14).

- [ ] **Step 4: Apply overlay in ListStrategies**

Same pattern, no parent scope to filter on.

- [ ] **Step 5: Run build + tests**

Run: `go build ./... && go test ./internal/flags/... ./internal/rollout/... -short -v`
Expected: clean + pass.

- [ ] **Step 6: Commit**

```bash
git add internal/flags/handler.go internal/rollout/handler.go
git commit -m "feat(staging): overlay staged creates/updates on list endpoints"
```

---

### Task 14: Detail-by-provisional-id handlers

**Files:**
- Modify: `internal/flags/handler.go` (`GetFlag`, `GetRule`)
- Modify: `internal/rollout/handler.go` (`GetStrategy`)

- [ ] **Step 1: Add provisional branch to GetFlag**

In `GetFlag`:

```go
flagID, err := uuid.Parse(c.Param("id"))
if err != nil { ... }

if staging.IsProvisional(flagID) && c.Query("include_my_staged") == "true" {
	row, err := h.staging.GetProvisionalCreate(c.Request.Context(), userID, orgID, "flag", flagID)
	if err != nil || row == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "flag not found"})
		return
	}
	var synth models.FeatureFlag
	_ = json.Unmarshal(row.NewValue, &synth)
	synth.ID = flagID
	resp := flagWithMarker{
		FeatureFlag: &synth,
		Staged:      &staging.Marker{ProvisionalID: row.ProvisionalID, Action: "create", StagedAt: row.CreatedAt},
	}
	c.JSON(http.StatusOK, resp)
	return
}
// existing real-id path follows unchanged
```

- [ ] **Step 2: Add `GetProvisionalCreate` to staging.Service + Repository**

Add to `internal/staging/repository.go` interface:

```go
// GetProvisionalCreate returns the user's staged CREATE row for the given
// resource type whose provisional_id matches. Returns nil (no error) if none.
GetProvisionalCreate(ctx context.Context, userID, orgID uuid.UUID, resourceType string, provisionalID uuid.UUID) (*models.StagedChange, error)
```

Implement on the postgres repo:

```sql
SELECT * FROM staged_changes
WHERE user_id = $1 AND org_id = $2 AND resource_type = $3 AND provisional_id = $4 AND action = 'create'
LIMIT 1;
```

Pass through on `Service.GetProvisionalCreate(ctx, userID, orgID, resourceType, provisionalID)`.

- [ ] **Step 3: Mirror for GetRule and GetStrategy**

Same skeleton.

- [ ] **Step 4: Add tests**

`internal/flags/handler_test.go`:

```go
func TestGetFlagByProvisionalReturnsSynthesised(t *testing.T) {
	// stage a flag.create with provisional id; GET /flags/<provId>?include_my_staged=true;
	// expect 200 with the synth body and _staged marker.
}

func TestGetFlagByProvisionalRefusesWithoutQueryParam(t *testing.T) {
	// expect 404 when include_my_staged is missing — staging is opt-in.
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/flags/... ./internal/staging/... -short -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/flags/handler.go internal/rollout/handler.go internal/staging/repository.go internal/staging/service.go internal/platform/database/postgres/staging.go
git commit -m "feat(staging): detail-by-provisional-id for flag / rule / strategy"
```

---

### Task 15: End-to-end Postgres integration test

**Files:**
- Create: `internal/staging/service_provisional_test.go`

- [ ] **Step 1: Write the integration test**

```go
//go:build integration

package staging_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/staging"
	stagingdb "github.com/shadsorg/deploysentry/internal/platform/database/postgres"
)

func TestE2EFlagCreatePlusDependentRuleCreatePlusEnvStateUpdate(t *testing.T) {
	pool := openTestPool(t) // gated by DS_TEST_DATABASE_DSN
	ctx := context.Background()

	// Build a real wiring: real flag service, real rollout service, both
	// pointing at the same pool. Stage three rows. Commit. Assert.
	// (Use the same setup helpers internal/staging/service_test.go uses
	//  for the simpler integration tests that ship today.)

	user, org := uuid.New(), uuid.New()
	provFlag := staging.NewProvisional()
	provRule := staging.NewProvisional()
	envID := uuid.New()

	flagPayload, _ := json.Marshal(map[string]any{
		"key":            "test-flag",
		"name":           "Test Flag",
		"project_id":     uuid.New().String(),
		"environment_id": envID.String(),
		"flag_type":      "boolean",
		"default_value":  "false",
	})
	rulePayload, _ := json.Marshal(map[string]any{
		"flag_id":  provFlag.String(),
		"priority": 1,
	})
	stateRow := &models.StagedChange{
		ID:           uuid.New(),
		UserID:       user, OrgID: org,
		ResourceType: "flag_env_state",
		Action:       "update",
		ResourceID:   &provFlag,
		NewValue:     []byte(`{"environment_id":"` + envID.String() + `","enabled":true}`),
	}
	flagRow := &models.StagedChange{ID: uuid.New(), UserID: user, OrgID: org, ResourceType: "flag", Action: "create", ProvisionalID: &provFlag, NewValue: flagPayload}
	ruleRow := &models.StagedChange{ID: uuid.New(), UserID: user, OrgID: org, ResourceType: "flag_rule", Action: "create", ProvisionalID: &provRule, NewValue: rulePayload}

	for _, r := range []*models.StagedChange{flagRow, ruleRow, stateRow} {
		if err := stagingService.Stage(ctx, r); err != nil { t.Fatalf("stage: %v", err) }
	}

	res, err := stagingService.Commit(ctx, user, org, user, []uuid.UUID{flagRow.ID, ruleRow.ID, stateRow.ID})
	if err != nil { t.Fatalf("commit: %v", err) }
	if len(res.CommittedIDs) != 3 { t.Fatalf("committed=%v", res.CommittedIDs) }

	// Assert: production flags table has one new row; its id is real (not provisional).
	// flag_targeting_rules has one new row whose flag_id == that real id.
	// flag_environment_states has the env row updated.
	// staged_changes is empty for (user, org).
}

func TestE2EUnresolvedProvisionalRefusesAtPreflight(t *testing.T) {
	// Stage only the rule.create referencing a provisional flag id; commit;
	// expect FailedReason with "is not in this deploy batch".
}

func TestE2EMidBatchFailureRollsBackCreate(t *testing.T) {
	// Stage flag.create + flag_rule.update with deliberately invalid payload;
	// commit; expect tx rollback; assert no row in feature_flags.
}
```

- [ ] **Step 2: Run integration test (gated)**

Run: `DS_TEST_DATABASE_DSN=$DS_TEST_DATABASE_DSN go test -tags=integration ./internal/staging/ -run TestE2E -v`
Expected: PASS for all three. If `DS_TEST_DATABASE_DSN` is not set, tests skip (use `t.Skip` if env var missing — match the existing pattern in `staging_test.go`).

- [ ] **Step 3: Commit**

```bash
git add internal/staging/service_provisional_test.go
git commit -m "test(staging): E2E provisional-id resolution integration tests"
```

---

### Task 16: PR 1 verification + open PR

- [ ] **Step 1: Run full verification gates**

```
go vet ./...
go build ./...
go test ./... -short
```

Expected: all clean.

- [ ] **Step 2: Open PR**

```bash
git push -u origin feature/staging-provisional-id-spec
gh pr create --title "feat(staging): provisional-id resolution + *.create commit handlers" --body "$(cat <<'EOF'
## Summary
- Adds `Resolver` + pre-flight planner + `CreateRegistry` to `internal/staging/`.
- Adds tx-aware `CreateFlagTx` / `AddRuleTx` / `CreateStrategyTx` paths so create handlers can ride the staging tx (true rollback on mid-batch failure).
- Adds three create commit handlers: `flag.create`, `flag_rule.create`, `strategy.create`.
- Read-path overlay learns to synthesise create rows; lists + detail handlers gain `_staged` envelope marker emission.
- Pre-flight refuses any deploy batch that references a provisional id whose creator is not in the batch.

Spec: `docs/superpowers/specs/2026-05-03-staging-provisional-id-resolution-design.md`

## Test plan
- [ ] Unit tests pass: `go test ./internal/staging/... ./internal/flags/... ./internal/rollout/... -short -v`
- [ ] E2E integration tests pass: `DS_TEST_DATABASE_DSN=... go test -tags=integration ./internal/staging/ -run TestE2E -v`
- [ ] Manual: stage a flag + dependent rule + env-state update; confirm one Deploy creates all three with the same real flag id.
- [ ] Manual: discard a parent create, deploy the dependent — confirm pre-flight error names the missing parent.
- [ ] Manual: force a rule validation failure — confirm flag is NOT in production after rollback.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## PR 2 — Client Provisional Minting + FlagCreatePage + StagedBadge

### Task 17: Client `newProvisionalId()` + tests

**Files:**
- Create: `web/src/lib/provisional.ts`
- Create: `web/src/lib/provisional.test.ts`

- [ ] **Step 1: Branch + write the failing test**

```bash
git checkout main
git pull
git checkout -b feature/staging-provisional-client
```

```ts
// web/src/lib/provisional.test.ts
import { describe, it, expect } from 'vitest';
import { newProvisionalId, isProvisionalId } from './provisional';

describe('newProvisionalId', () => {
  it('mints a UUID with the provisional variant byte (0xc0)', () => {
    const id = newProvisionalId();
    // UUID format check
    expect(id).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
    // Variant byte: char 19 is the high nibble of byte 8 → must be 'c' or 'd'
    expect(id.charAt(19)).toMatch(/[cd]/);
    expect(isProvisionalId(id)).toBe(true);
  });

  it('round-trips through JSON.stringify / parse unchanged', () => {
    const id = newProvisionalId();
    expect(JSON.parse(JSON.stringify({ id })).id).toBe(id);
  });

  it('isProvisionalId returns false for an RFC-4122 UUID', () => {
    expect(isProvisionalId('550e8400-e29b-41d4-a716-446655440000')).toBe(false);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run src/lib/provisional.test.ts`
Expected: FAIL — file not found.

- [ ] **Step 3: Write implementation**

```ts
// web/src/lib/provisional.ts

// Mints a provisional UUID with the same variant-byte invariant as Go's
// staging.NewProvisional (byte 8 has its top two bits set to 11). Backend
// Service.Stage rejects any provisional_id that doesn't match this shape.
export function newProvisionalId(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  // Set version to 4 (top nibble of byte 6)
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  // Set the provisional variant: top two bits of byte 8 = 11
  bytes[8] = (bytes[8] & 0x3f) | 0xc0;
  return formatUUID(bytes);
}

export function isProvisionalId(id: string): boolean {
  // Char index 19 maps to the high nibble of byte 8 in canonical form.
  // Provisional variant means that nibble is 1100 (c) or 1101 (d).
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}[cd][0-9a-f]{3}-[0-9a-f]{12}$/.test(id);
}

function formatUUID(b: Uint8Array): string {
  const hex = Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20, 32)}`;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && npx vitest run src/lib/provisional.test.ts`
Expected: PASS for all three tests.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/provisional.ts web/src/lib/provisional.test.ts
git commit -m "feat(web): newProvisionalId() with variant-byte invariant"
```

---

### Task 18: StagedBadge component + test

**Files:**
- Create: `web/src/components/staging/StagedBadge.tsx`
- Create: `web/src/components/staging/StagedBadge.test.tsx`

- [ ] **Step 1: Write failing test**

```tsx
// web/src/components/staging/StagedBadge.test.tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StagedBadge } from './StagedBadge';

describe('StagedBadge', () => {
  it('renders "pending create" when action is create', () => {
    render(<StagedBadge marker={{ action: 'create', staged_at: '2026-05-03T10:00:00Z' }} />);
    expect(screen.getByText(/pending create/i)).toBeInTheDocument();
  });

  it('renders "pending update" when action is update', () => {
    render(<StagedBadge marker={{ action: 'update', staged_at: '2026-05-03T10:00:00Z' }} />);
    expect(screen.getByText(/pending update/i)).toBeInTheDocument();
  });

  it('renders nothing when marker is null', () => {
    const { container } = render(<StagedBadge marker={null} />);
    expect(container.firstChild).toBeNull();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run src/components/staging/StagedBadge.test.tsx`
Expected: FAIL.

- [ ] **Step 3: Write component**

```tsx
// web/src/components/staging/StagedBadge.tsx
export type StagedMarker = {
  provisional_id?: string;
  action: string;
  staged_at: string;
};

export function StagedBadge({ marker }: { marker: StagedMarker | null | undefined }) {
  if (!marker) return null;
  return (
    <span
      className="inline-flex items-center rounded-md bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800"
      title={`Staged ${marker.action} at ${new Date(marker.staged_at).toLocaleString()}`}
    >
      pending {marker.action}
    </span>
  );
}
```

- [ ] **Step 4: Run test**

Run: `cd web && npx vitest run src/components/staging/StagedBadge.test.tsx`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/staging/StagedBadge.tsx web/src/components/staging/StagedBadge.test.tsx
git commit -m "feat(web): StagedBadge leaf component"
```

---

### Task 19: Convert FlagCreatePage to stageOrCall

**Files:**
- Modify: `web/src/api.ts` (extend types)
- Modify: `web/src/pages/FlagCreatePage.tsx`
- Modify: `web/src/pages/FlagCreatePage.test.tsx`

- [ ] **Step 1: Add provisional_id to stagingApi.stage payload**

In `web/src/api.ts`, find the `stagingApi.stage` request type. Add (if not already present):

```ts
export type StageRequest = {
  resource_type: string;
  resource_id?: string;
  provisional_id?: string;
  action: string;
  field_path?: string;
  old_value?: unknown;
  new_value?: unknown;
};
```

Add `_staged?: StagedMarker` (importing the type from `components/staging/StagedBadge`) to the flag list response item type.

- [ ] **Step 2: Write failing test**

```tsx
// web/src/pages/FlagCreatePage.test.tsx — extend
import { newProvisionalId, isProvisionalId } from '@/lib/provisional';

it('routes through stageOrCall when staging is enabled and navigates to provisional detail', async () => {
  setStagingEnabled(orgSlug, true);
  const stageSpy = vi.spyOn(stagingApi, 'stage').mockResolvedValue({ id: 'staged-row-id' } as never);
  const navigate = vi.fn();
  // mount with router; submit form
  // assert stageSpy called with provisional_id matching isProvisionalId
  // assert navigate called with /orgs/<slug>/projects/<slug>/flags/<that-provisional-id>
});

it('routes directly when staging is disabled', async () => {
  setStagingEnabled(orgSlug, false);
  const createSpy = vi.spyOn(flagsApi, 'createFlag').mockResolvedValue({ id: 'real-id' } as never);
  // submit form
  // assert createSpy called; stageSpy NOT called; navigate to real id detail
});
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd web && npx vitest run src/pages/FlagCreatePage.test.tsx`
Expected: FAIL.

- [ ] **Step 4: Convert handleSubmit**

In `FlagCreatePage.tsx`, import the hooks:

```ts
import { useStagingEnabled } from '@/hooks/useStagingEnabled';
import { stageOrCall } from '@/hooks/stageOrCall';
import { newProvisionalId } from '@/lib/provisional';
```

Replace the existing `handleSubmit`'s mutation:

```ts
const stagingEnabled = useStagingEnabled(orgSlug);

const handleSubmit = async (e: React.FormEvent) => {
  e.preventDefault();
  // ... existing field validation ...

  const provisionalId = newProvisionalId();
  const result = await stageOrCall({
    staged: stagingEnabled,
    orgSlug,
    stage: {
      resource_type: 'flag',
      action: 'create',
      provisional_id: provisionalId,
      new_value: flagPayload,
    },
    direct: () => flagsApi.createFlag(flagPayload),
  });

  if (result.mode === 'staged') {
    navigate(`/orgs/${orgSlug}/projects/${projectSlug}/flags/${provisionalId}`);
  } else {
    navigate(`/orgs/${orgSlug}/projects/${projectSlug}/flags/${result.result.id}`);
  }
};
```

- [ ] **Step 5: Run test**

Run: `cd web && npx vitest run src/pages/FlagCreatePage.test.tsx`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/src/api.ts web/src/pages/FlagCreatePage.tsx web/src/pages/FlagCreatePage.test.tsx
git commit -m "feat(web): route flag.create through staging when enabled"
```

---

### Task 20: Show StagedBadge on FlagListPage

**Files:**
- Modify: `web/src/pages/FlagListPage.tsx`

- [ ] **Step 1: Render badge**

In the row JSX, after the flag name:

```tsx
<StagedBadge marker={flag._staged ?? null} />
```

Update the API call in this page to set `?include_my_staged=true` when `useStagingEnabled(orgSlug)` is true.

- [ ] **Step 2: Run lint + build**

Run: `cd web && npx tsc --noEmit && npm run lint && npm run build`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/FlagListPage.tsx
git commit -m "feat(web): show StagedBadge on FlagListPage"
```

---

### Task 21: PR 2 verification + open PR

- [ ] **Step 1: Run full web gates**

```
cd web && npx tsc --noEmit && npm run lint && npm run build && npx prettier --check "src/**/*.{ts,tsx,css,json}" && npx vitest run
```

Expected: all clean.

- [ ] **Step 2: Open PR**

```bash
git push -u origin feature/staging-provisional-client
gh pr create --title "feat(web): client-side provisional-id minting + FlagCreatePage staging conversion" --body "$(cat <<'EOF'
## Summary
- Adds `web/src/lib/provisional.ts` with `newProvisionalId()` mirroring the Go variant-byte invariant.
- Adds `StagedBadge` leaf component reading the `_staged` envelope from API responses.
- Routes `FlagCreatePage` through `stageOrCall` when staging is enabled; navigates to detail-by-provisional-id after stage.
- Wires `_staged` rendering on `FlagListPage`.

Builds on PR for backend resolution.

## Test plan
- [ ] Vitest pass.
- [ ] Manual: with staging on, create a flag — confirm no row appears in production tables; the new flag shows up in the list with the pending badge; the detail page renders the synthesised flag.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## PR 3 — handleAddRule + StrategyEditor Create

### Task 22: Convert handleAddRule on FlagDetailPage

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`
- Modify: `web/src/pages/FlagDetailPage.test.tsx`

- [ ] **Step 1: Branch**

```bash
git checkout main && git pull
git checkout -b feature/staging-add-rule-strategy-create
```

- [ ] **Step 2: Write failing test**

```tsx
it('routes handleAddRule through staging when enabled, carrying parent flag id', async () => {
  setStagingEnabled(orgSlug, true);
  const stageSpy = vi.spyOn(stagingApi, 'stage').mockResolvedValue({ id: 'staged' } as never);
  // render FlagDetailPage for a real flag id; click "Add rule"; submit
  expect(stageSpy).toHaveBeenCalledWith(
    orgSlug,
    expect.objectContaining({
      resource_type: 'flag_rule',
      action: 'create',
      provisional_id: expect.stringMatching(/[cd][0-9a-f]{3}-/), // variant byte
      new_value: expect.objectContaining({ flag_id: realFlagId }),
    }),
  );
});

it('also works when parent flag id is itself provisional', async () => {
  // navigate to /orgs/.../flags/<provisionalFlagId>; same assertion but flag_id is provisional
});
```

- [ ] **Step 3: Run test**

Run: `cd web && npx vitest run src/pages/FlagDetailPage.test.tsx`
Expected: FAIL.

- [ ] **Step 4: Convert handleAddRule**

```ts
const stagingEnabled = useStagingEnabled(orgSlug);

const handleAddRule = async (rule: NewRulePayload) => {
  const provisionalId = newProvisionalId();
  const payload = { ...rule, flag_id: flagId }; // flagId from URL (real or provisional)
  const result = await stageOrCall({
    staged: stagingEnabled,
    orgSlug,
    stage: {
      resource_type: 'flag_rule',
      action: 'create',
      provisional_id: provisionalId,
      new_value: payload,
    },
    direct: () => flagsApi.addRule(payload),
  });
  if (result.mode === 'staged') {
    setRules((prev) => [...prev, { ...payload, id: provisionalId, _staged: { provisional_id: provisionalId, action: 'create', staged_at: new Date().toISOString() } }]);
  } else {
    setRules((prev) => [...prev, result.result]);
  }
};
```

- [ ] **Step 5: Run test**

Run: `cd web && npx vitest run src/pages/FlagDetailPage.test.tsx`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/FlagDetailPage.tsx web/src/pages/FlagDetailPage.test.tsx
git commit -m "feat(web): route flag_rule.create through staging when enabled"
```

---

### Task 23: Convert StrategyEditor create branch

**Files:**
- Modify: `web/src/pages/StrategyEditor.tsx`
- Modify: `web/src/pages/StrategyEditor.test.tsx`

- [ ] **Step 1: Write failing test**

```tsx
it('routes strategy.create through staging when enabled', async () => {
  setStagingEnabled(orgSlug, true);
  const stageSpy = vi.spyOn(stagingApi, 'stage').mockResolvedValue({ id: 'staged' } as never);
  // render StrategyEditor in create mode; submit
  expect(stageSpy).toHaveBeenCalledWith(
    orgSlug,
    expect.objectContaining({
      resource_type: 'strategy',
      action: 'create',
      provisional_id: expect.any(String),
      new_value: expect.any(Object),
    }),
  );
});
```

- [ ] **Step 2: Convert submit (create branch)**

```ts
const stagingEnabled = useStagingEnabled(orgSlug);

const submit = async (data: StrategyPayload) => {
  if (mode === 'create') {
    const provisionalId = newProvisionalId();
    const result = await stageOrCall({
      staged: stagingEnabled,
      orgSlug,
      stage: { resource_type: 'strategy', action: 'create', provisional_id: provisionalId, new_value: data },
      direct: () => rolloutApi.createStrategy(data),
    });
    if (result.mode === 'staged') {
      navigate(`/orgs/${orgSlug}/strategies/${provisionalId}`);
    } else {
      navigate(`/orgs/${orgSlug}/strategies/${result.result.id}`);
    }
    return;
  }
  // existing update branch unchanged (already converted in C-11)
};
```

- [ ] **Step 3: Run test**

Run: `cd web && npx vitest run src/pages/StrategyEditor.test.tsx`
Expected: PASS.

- [ ] **Step 4: Render StagedBadge on StrategiesPage**

In `web/src/pages/StrategiesPage.tsx`, render `<StagedBadge marker={strategy._staged ?? null} />` next to the strategy name. Set `?include_my_staged=true` when staging is enabled.

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/StrategyEditor.tsx web/src/pages/StrategyEditor.test.tsx web/src/pages/StrategiesPage.tsx
git commit -m "feat(web): route strategy.create through staging when enabled"
```

---

### Task 24: PR 3 verification + open PR

- [ ] **Step 1: Run full web gates**

```
cd web && npx tsc --noEmit && npm run lint && npm run build && npx prettier --check "src/**/*.{ts,tsx,css,json}" && npx vitest run
```

Expected: clean.

- [ ] **Step 2: Open PR**

```bash
git push -u origin feature/staging-add-rule-strategy-create
gh pr create --title "feat(web): route flag_rule.create + strategy.create through staging when enabled" --body "$(cat <<'EOF'
## Summary
- Routes `handleAddRule` (FlagDetailPage) through `stageOrCall`.
- Routes `StrategyEditor` create branch through `stageOrCall`.
- Renders `StagedBadge` on `StrategiesPage`.

Closes the deferred provisional-id work in the staged-changes initiative when shipped together with the backend + FlagCreatePage PRs.

## Test plan
- [ ] Vitest pass.
- [ ] Manual: with staging on, create a strategy — confirm pending badge on list, deploy succeeds, real strategy lands.
- [ ] Manual: with staging on, create a flag, then add a rule to that flag from its detail page (URL is provisional) — deploy both rows; confirm the rule's flag_id matches the new real flag id in production.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Self-Review Checklist Coverage

- **Spec § Architecture (4-phase pipeline)**: Task 10 (Service.Commit rewrite) + Task 2 (preflight).
- **Spec § New types (Resolver, CreateRegistry, planBatch)**: Tasks 1, 3, 2.
- **Spec § Tx-aware service paths**: Tasks 4, 5, 6.
- **Spec § create handlers (flag, flag_rule, strategy)**: Tasks 7, 8, 9.
- **Spec § read-path overlay (synth + marker)**: Tasks 12, 13.
- **Spec § detail-by-provisional-id**: Task 14.
- **Spec § wire format `_staged` envelope**: Task 12 + 13.
- **Spec § client-side `newProvisionalId`**: Task 17.
- **Spec § FlagCreatePage / handleAddRule / StrategyEditor conversions**: Tasks 19, 22, 23.
- **Spec § StagedBadge**: Task 18, mounted in Tasks 20, 22, 23.
- **Spec § Service.Stage validates variant byte**: Task 10 step 3.
- **Spec § Done-when criteria**: covered by Task 15 E2E + Task 16/21/24 manual verification checklists.

## Verification Gates (each PR)

```
go vet ./...
go build ./...
go test ./... -short
cd web && npx tsc --noEmit && npm run lint && npm run build && npx prettier --check "src/**/*.{ts,tsx,css,json}" && npx vitest run
```

Postgres-gated integration tests run under `DS_TEST_DATABASE_DSN=...`.
