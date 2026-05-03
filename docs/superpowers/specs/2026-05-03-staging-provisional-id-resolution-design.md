# Staging — Provisional-ID Resolution + `*.create` Commit Handlers

**Status**: Design
**Date**: 2026-05-03
**Parent spec**: [`2026-04-30-staged-changes-and-deploy-workflow-design.md`](./2026-04-30-staged-changes-and-deploy-workflow-design.md)
**Severity**: medium — completes the staged-changes initiative; unlocks staged creates across flag / rule / strategy

## Problem

The parent spec ships `staged_changes` rows for create actions with a `provisional_id` (a non-RFC-4122-variant UUID minted before the resource exists in production). Today `Service.Commit` refuses to dispatch any row whose `ResourceID` is nil with a clear error — there is no way to commit a staged `flag.create`, `flag_rule.create`, or `strategy.create`.

Two requirements have to be met to commit such a row:

1. The create handler must mint the real production id atomically with the production-row insert (rollback on any later batch failure must unwind the insert).
2. Other staged rows in the same Deploy batch that reference the provisional id — either as their own `resource_id` (a child create whose parent isn't created yet) or inside `new_value` JSON fields like `flag_id` — must have those references rewritten to the real id before their handlers run.

Provisional UUIDs must never reach production tables, the audit log, webhook payloads, SSE broadcasts, or analytics events. The variant-byte guard (`staging.IsProvisional`) catches leaks at egress; this design ensures no leak occurs in the first place by resolving every provisional inside the commit transaction.

## In scope

- Backend resolution algorithm + tx-aware create handlers for `flag.create`, `flag_rule.create`, `strategy.create`.
- Read-path overlay support for staged creates (synthetic rows visible in lists / detail when `?include_my_staged=true`).
- UI conversion of the three create surfaces (`FlagCreatePage`, `handleAddRule` on `FlagDetailPage`, `StrategyEditor` create branch) to call `stageOrCall` instead of writing directly.
- Client-side provisional-id minting helper (`web/src/lib/provisional.ts`) that mirrors the Go variant-byte invariant.

## Out of scope

- New create-able resource types beyond flag / rule / strategy (settings, members, env-state already only support `update`/`delete`/`role_changed`).
- UI assist for unresolved-provisional errors on the review page (auto-checking the parent when a child is selected). The backend returns a hard error; UI shows the failure banner. Auto-include is a follow-on if the manual experience proves annoying.
- Tightening batch scope below `(user, org)` (e.g. one project per Deploy). Cross-project batches stay legal, with domain-level validation surfacing inside individual handlers as today.
- Two-person approval interaction. The two-person spec interacts at the commit boundary; this spec doesn't change that boundary.

## Architecture

`Service.Commit` becomes a four-phase pipeline:

1. **Pre-flight (no tx open).** Partition the loaded batch into `creates` (rows with `ProvisionalID != nil`) and `mutations`. Build the *known-provisionals* set from creates. Walk every row's `ResourceID` and recursively walk every UUID inside `new_value` + `old_value` JSON; collect every provisional UUID seen anywhere. If any referenced provisional is not in the known set, refuse the deploy with `CommitResult.FailedReason` naming the missing parent — no tx opened, no partial work. Topo-order rows so every creator precedes every consumer; mutations with no provisional references stay in original order for determinism.

2. **Open tx + walk in topo order.** For each row: if it's a create, dispatch through `CreateRegistry`, take the returned real id, bind it into the in-memory `Resolver` map. Otherwise call `Resolver.RewriteRow(row)` first (rewrites `ResourceID` if it was provisional + known, plus `new_value` / `old_value` JSON via blanket UUID walk) then dispatch through the existing `CommitRegistry`. Every handler's first action is `staging.MustNotBeProvisional(row.ResourceID, ...)` and the same guard runs on every UUID inside the resolved JSON — belt + suspenders against a missed reference.

3. **Delete staged rows + tx.Commit.** Unchanged from today.

4. **Post-commit side effects.** Each `CreateHandler` returns an optional `postCommit func(context.Context)` closure that fires cache invalidation and NATS publish for the new resource. Closures run after `tx.Commit` succeeds, never inside the tx — broadcasting an id that just got rolled back would be incorrect. Audit-row writes (already post-commit today) keep their current shape; `entity_id` is the resolved real id.

Failure modes:

- Pre-flight error → no tx opened, batch refused, staged rows untouched.
- Mid-batch handler error → `tx.Rollback` unwinds *all* production writes including completed creates (they all rode the same tx). Staged rows untouched. Resolver state discarded.
- `tx.Commit` error → same as today; staged rows untouched.
- Post-commit hook error (cache invalidate / NATS publish failure) → logged + surfaced via the `CommitResult` error envelope; production write is **not** unwound — same policy as the existing audit-write failure path.

## New types

### `internal/staging/resolver.go`

```go
type Resolver struct {
    m map[uuid.UUID]uuid.UUID // provisional → real
}

func NewResolver() *Resolver
func (r *Resolver) Bind(provisional, real uuid.UUID)            // panics if provisional is not provisional, or real is provisional
func (r *Resolver) Lookup(id uuid.UUID) (uuid.UUID, bool)       // returns real if known
func (r *Resolver) RewriteUUIDsInJSON(raw []byte) ([]byte, error)
func (r *Resolver) RewriteRow(row *models.StagedChange) error
```

`RewriteUUIDsInJSON` walks JSON via `map[string]any` round-trip; every string value is parsed with `uuid.Parse`; on success, looked up in the resolver; on hit, replaced with the real id's string form. Non-UUID strings, numbers, nulls, bools, and structural tokens pass through untouched.

### `internal/staging/registry.go` (extended)

The existing `CommitRegistry` + `CommitHandler` stay byte-identical. Added alongside:

```go
type CreateHandler func(
    ctx context.Context,
    tx pgx.Tx,
    row *models.StagedChange,
) (realID uuid.UUID, auditAction string, postCommit func(context.Context), err error)

type CreateRegistry struct{ handlers map[string]CreateHandler }

func NewCreateRegistry() *CreateRegistry
func (r *CreateRegistry) Register(resourceType, action string, h CreateHandler)
func (r *CreateRegistry) IsCreatable(resourceType, action string) bool
func (r *CreateRegistry) Dispatch(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (uuid.UUID, string, func(context.Context), error)
```

`Service` grows one field: `creates *CreateRegistry`, wired in `NewService`.

### `internal/staging/preflight.go`

```go
type batchPlan struct {
    ordered    []*models.StagedChange  // creates first, then mutations, in dependency order
    knownProvs map[uuid.UUID]struct{}  // every provisional minted by this batch
}

type ErrUnresolvedProvisional struct {
    RowID    uuid.UUID
    ProvUUID uuid.UUID
}

func (e *ErrUnresolvedProvisional) Error() string

func planBatch(rows []*models.StagedChange) (*batchPlan, error)
```

`planBatch` errors with `*ErrUnresolvedProvisional` when any referenced UUID is provisional but not minted by this batch.

## Tx-aware service paths

Per-resource changes:

### `internal/flags/repository.go` + Postgres impl

- Add `CreateFlagTx(ctx, tx pgx.Tx, flag *models.FeatureFlag) error` — same SQL as `CreateFlag`, executed via `tx.Exec`. The pool path stays for direct (non-staged) writes.
- Same for `CreateRuleTx(ctx, tx, rule *models.TargetingRule) error`.

### `internal/flags/service.go`

- Add `CreateFlagTx(ctx, tx, flag) (realID uuid.UUID, err error)` and `AddRuleTx(ctx, tx, rule) (realID, err)`. Each: mints id if zero (real UUID, never provisional), sets timestamps, runs `flag.Validate()` / `rule.Validate()` exactly like the non-tx path, calls the tx repo method. Does **not** call cache invalidate or `publishEvent`.
- The non-tx `CreateFlag` / `AddRule` keep their public shape but compose: `pool.BeginTx → CreateFlagTx → tx.Commit → cache.Invalidate + publishEvent`. Single source of truth for validation + id minting.

### `internal/rollout/service.go`

- Same shape: `CreateStrategyTx(ctx, tx, strategy)` defers cache + event work.

### `internal/flags/staging_commit.go` + `internal/rollout/staging_commit.go`

Add three create handlers:

```go
func commitFlagCreate(svc FlagService) staging.CreateHandler
func commitFlagRuleCreate(svc FlagService) staging.CreateHandler
func commitStrategyCreate(svc rollout.Service) staging.CreateHandler
```

Each handler:

1. Unmarshals `row.NewValue` into the resource's domain struct.
2. Sets `Domain.ID = uuid.Nil` (force the tx-aware path to mint a fresh real id; ignore any id smuggled in the payload).
3. Calls `svc.<Resource>Tx(ctx, tx, &domain)` → real id.
4. Returns `(realID, "<resource>.created", postCommit, nil)` where `postCommit` invokes the resource's existing event publish + cache invalidation paths.

Audit-row build keeps the same code path; `entity_id` is the returned real id; `MustNotBeProvisional` guard at the audit-write site catches any leak.

## Read-path overlay

`internal/staging/overlay.go` already has `OverlayList[T]` / `OverlayDetail[T]`; this spec adds the `create` synthesis branch and wires three handlers.

### Synthesis branch

When applying staged rows over a list, a `<resource>.create` row produces a synthetic `*T` whose fields come from `new_value` and whose `ID` is `*row.ProvisionalID`. Updates merge field-by-field over a matching prod row; deletes filter the prod row out. The adapter is per-resource — flag / rule / strategy each declare how to materialise from `new_value` JSON.

### Wired handlers

- `internal/flags/handler.go` `ListFlags` (project scope) → overlay flag.create rows whose `new_value.project_id` matches.
- `internal/flags/handler.go` `ListRules` (flag scope) → overlay flag_rule.create rows whose `new_value.flag_id` matches the URL flag id (provisional-to-provisional or real-to-real; the resolver is not involved on read).
- `internal/rollout/handler.go` `ListStrategies` → overlay strategy.create rows.
- Detail-by-provisional-id: `GET /flags/:id` (and rule / strategy equivalents) detect via `staging.IsProvisional`, look up the user's staged create by provisional id, and return the synthesised resource from `new_value`.

`?include_my_staged=true` gating stays. Dashboard always sets it; SDK never does.

### Wire-format marker

Synthetic creates and merged updates emitted through the overlay carry an envelope field on the JSON response:

```json
{ "_staged": { "provisional_id": "...", "action": "create", "staged_at": "..." } }
```

`_staged` is absent on plain prod rows. The dashboard renders a `<StagedBadge>` component when present; SDK clients that don't request the overlay never see it.

## UI conversions

Three pages, each a one-file diff using the existing `useStagingEnabled` + `stageOrCall` hooks (the C-1...C-11 pattern).

### `web/src/lib/provisional.ts`

```ts
export function newProvisionalId(): string
```

Mints a v4 UUID, then sets the variant byte to `0xc0` to match `staging.NewProvisional` in Go. Backend `Service.Stage` validates the variant byte on stage and rejects RFC-4122 UUIDs in `provisional_id` — the dashboard owns provisional ids end-to-end.

### `web/src/pages/FlagCreatePage.tsx`

`handleSubmit` wraps the existing `flagsApi.createFlag(...)` call:

```ts
await stageOrCall({
  staged: stagingEnabled,
  orgSlug,
  stage: {
    resource_type: 'flag',
    action: 'create',
    provisional_id: newProvisionalId(),
    new_value: flagPayload,
  },
  direct: () => flagsApi.createFlag(...),
});
```

On staged success, navigate to `/orgs/:orgSlug/projects/:slug/flags/:provisionalId` so the user lands on the detail page surfaced by the read-path overlay.

### `web/src/pages/FlagDetailPage.tsx` `handleAddRule`

Same wrap. The new rule's `flag_id` payload is the URL flag id, which may itself be provisional — the JSON walker substitutes at commit.

### `web/src/pages/StrategyEditor.tsx` create branch

Same wrap.

### `web/src/components/staging/StagedBadge.tsx`

New leaf component. Renders a small "pending" marker on any list row or detail header where the API response carries `_staged`. Used by `FlagListPage`, `FlagDetailPage` (rules tab), `StrategiesPage`, and any other list that consumes overlay output.

## PR shape

Three PRs, in order:

1. **Backend resolution + tx-aware creates + read-path overlay.** `internal/staging/resolver.go`, `preflight.go`, registry extension, three create handlers, tx-aware service / repo methods, overlay create-synthesis, three list-handler wirings, three detail-by-provisional-id handlers, full unit + integration test set.
2. **Client-side provisional minting + `FlagCreatePage` conversion + `StagedBadge`.** Adds `web/src/lib/provisional.ts`, `StagedBadge` leaf component, converts `FlagCreatePage`, lights up the badge on `FlagListPage`.
3. **`handleAddRule` + `StrategyEditor` create branch.** Two more page conversions; rides on (1) and (2).

## Testing

### `internal/staging/` unit tests (no DB)

- `resolver_test.go` — `Bind` rejects non-provisional source / provisional destination; `Lookup` round-trips; `RewriteUUIDsInJSON` substitutes nested objects, arrays, mixed types, leaves non-UUID strings alone, leaves unknown UUIDs alone (real ids in payloads survive untouched).
- `preflight_test.go` — happy path (one create + dependent mutation, ordered correctly); pre-flight rejects an unresolved provisional with the expected `*ErrUnresolvedProvisional` shape; topo order produces creates first then mutations; isolated mutations preserve original input order.
- `service_test.go` — extend existing tests: full Commit with one `flag.create` + one `flag_rule.update` referencing it via JSON `flag_id`. Use a fake `CreateRegistry` that mints a deterministic real id; assert the rule handler sees the rewritten id, post-commit hooks run after `tx.Commit`, audit entries carry `entity_id = realID`, no provisional UUIDs leak.

### Postgres integration tests (gated by `DS_TEST_DATABASE_DSN`)

- `internal/flags/staging_commit_test.go` — extend with `commitFlagCreate` cases: tx-aware `CreateFlagTx` writes through tx; rollback unwinds the create when a later mutation in the batch fails (deliberate handler error → no flag row in `flags` table after rollback); cache invalidate + NATS publish fire only on success path (assert via spies).
- Same shape for `commitFlagRuleCreate` and `commitStrategyCreate`.
- New `internal/staging/service_provisional_test.go` end-to-end: stage a flag.create + dependent rule.create + dependent flag_env_state.update in one batch; commit; assert all three rows appear in production tables with the same real flag id; assert `staged_changes` is empty; assert audit log has three rows with real `entity_id`.

### Web tests (Vitest)

- `web/src/lib/provisional.test.ts` — variant byte is `0xc0` post-mint; round-trips through `JSON.stringify` / `JSON.parse`.
- `web/src/pages/FlagCreatePage.test.tsx` — staged path sends `provisional_id` + `new_value`, navigates to `/.../flags/:provisionalId` after stage; direct path unchanged.
- `web/src/pages/FlagDetailPage.test.tsx` — `handleAddRule` staged path sends `provisional_id` for the rule + carries the parent flag's id (real or provisional) in `new_value.flag_id`.
- `web/src/components/staging/StagedBadge.test.tsx` — renders for `_staged` envelope; absent otherwise.
- `DeployChangesPage.test.tsx` — extend existing test: staged creates render with their resource label using `new_value.key` / `new_value.name` since `resource_id` is null; pre-flight error from backend (unresolved provisional) renders `failed_id` + reason banner.

### Manual verification

- Stage a `flag.create` + dependent `flag_rule.create` + `flag_env_state.update`. Open Review page. Deploy. Confirm production row count delta is exactly +1 flag, +1 rule, 1 env_state row updated, with one matching real flag id across all three.
- Force a validation failure on the rule (e.g. malformed condition). Deploy. Confirm flag is **not** created in production (tx rollback worked) and the staged rows are still pending.
- Stage a `flag_rule.create` referencing a provisional flag id. *Discard* the parent `flag.create`. Deploy the rule alone. Confirm pre-flight error names the missing parent and refuses the deploy.
- Stage a flag create. Navigate to its detail page (URL has the provisional id). Confirm the page renders the synthesised flag with `StagedBadge`. Confirm SDK reads do not see it.

### Verification gates per PR

```
go vet ./...
go build ./...
go test ./... -short
cd web && npx tsc --noEmit && npm run lint && npm run build && npx prettier --check "src/**/*.{ts,tsx,css,json}" && npx vitest run
```

Postgres-gated integration tests run in CI under `DS_TEST_DATABASE_DSN`.

## Risks & callouts

- **Cache invalidation timing.** SDK readers see a brief window between `tx.Commit` and post-commit cache invalidation finishing — same window every direct write has today, no worse.
- **Cross-project batches.** A single Deploy can mix resources across projects. Domain validation inside individual handlers catches invalid cross-project references (e.g. a rule's `flag_id` belonging to another project) the same way it does for direct writes — not the resolver's job.
- **Provisional-id leak surface.** `MustNotBeProvisional` guards at audit-write site already cover the audit log. Webhook payloads, SSE broadcasts, and analytics emits should add the guard when they're touched in this work — not retroactively across all egress paths.
- **Resolver memory.** A batch may carry several creates. The resolver map is bounded by batch size, which is bounded by 30-day staging retention + practical UI usage. No worry.

## Done when

- A staged `flag.create` + dependent `flag_rule.create` deploys atomically: both rows appear in production with one matching real flag id, audit log has two rows, staged_changes table is empty.
- A staged create whose parent is missing from the deploy batch is refused with a clear pre-flight error naming the missing parent. No tx opened.
- A mid-batch handler error rolls back every create that ran earlier in the batch.
- Provisional UUIDs never appear in audit log rows, webhook payloads, or NATS broadcasts (verified by `MustNotBeProvisional` guards at every egress).
- `FlagCreatePage`, `handleAddRule`, and `StrategyEditor` create branch route through `stageOrCall` when staging is enabled.
- Lists and detail pages render staged creates (`StagedBadge`) when `?include_my_staged=true` is set; SDK responses are unchanged.
