# Staged Changes + Deploy Review Workflow — Implementation Plan

**Phase**: Implementation (Phase C tail / pre-rollout)
**Date**: 2026-05-01 (last updated 2026-05-01 — staging plumbing landed; UI conversions in queue)
**Spec**: [`../specs/2026-04-30-staged-changes-and-deploy-workflow-design.md`](../specs/2026-04-30-staged-changes-and-deploy-workflow-design.md)

## Status snapshot

| Phase | Scope | State | PR(s) |
|---|---|---|---|
| A | Backend foundation: `staged_changes` table (migration 061), `internal/staging/` package (registry, repo, service, overlay, sweep, HTTP handler, provisional-id helpers), `flag.toggle` commit handler | **Merged** | #86 |
| B | `StagingHeaderBanner` + `DeployChangesPage` review page with diff / select / bulk Deploy / Discard | **Merged** | #87 |
| C-1 | `flag.update` / `archive` / `restore` backend commit handlers | Open | #89 |
| C-2 | `flag_rule.update` / `flag_rule.delete` / `flag_rule_env_state.update` / `flag_env_state.update` backend commit handlers | Open | #91 |
| C-3 | `setting.update` / `setting.delete` / `member.role_changed` backend commit handlers | Open | #92 |
| C-4 | `strategy.update` / `strategy.delete` / `strategy_default.upsert/delete` / `rollout_policy.upsert/delete` backend commit handlers | Open | #93 |
| C-5 | `useStagingEnabled` hook + `stageOrCall` helper + FlagDetailPage archive/restore UI | **Merged** | #94 |
| C-6 | FlagDetailPage per-env state UI conversion | Open | #95 |
| C-7 | FlagDetailPage rule update/delete UI conversion | Open | #96 |
| C-8 | FlagDetailPage per-rule per-env activation UI conversion | Open | #97 |
| C-9 | FlagDetailPage settings-tab UI conversion | Open | #98 |
| C-10 | MembersPage role-change UI conversion | Open | #99 |
| C-11 | StrategiesPage delete + StrategyEditor update UI conversion | Open | #100 |
| C-12 | Operator-facing `StagingModeToggle` on DeployChangesPage | Open | #101 |
| — | Provisional-id resolution + `*.create` commit handlers | Not started (design needed) | — |
| — | Real feature-flag-backed gate in `useStagingEnabled` (replace localStorage backing) | Not started | — |
| — | One-time org-wide enablement (flip `staged-changes-enabled` ON for every existing org) | Not started | — |

## Overview

Per-user staging layer for dashboard mutations. UI writes go to `staged_changes` instead of production; the dashboard overlays the user's staged rows on read; a header chrome surfaces the count; a Review page commits the selected rows in one DB transaction. SDK / CLI / webhook writes are unchanged.

## Design questions (resolved during Phase A)

1. **Read-path overlay shape** → per-handler typed patches (typed via Go generics `OverlayList[T]` / `OverlayDetail[T]`).
2. **Bypass switch for JWT requests** → `?immediate=true` honoured for any caller (escape hatch). Default: dashboard never sets it.
3. **Feature flag** → `staged-changes-enabled` (release category, expires after rollout). **Currently not wired to a real flag** — `useStagingEnabled` reads from `localStorage:ds_staging_enabled:<orgSlug>` so operators can opt in per-org. Real flag wiring is on the remaining-work list.
4. **Provisional-id egress** → non-RFC-4122 variant byte; constant-time `IsProvisional` + `MustNotBeProvisional` guard. Audit-write site enforces it today; webhook/SSE/analytics need the guard added when those egress paths are touched.
5. **Permission re-check at commit** → not yet extracted. The commit endpoint runs `RequirePermission(PermFlagRead)` at the front door; per-row re-check inside the transaction is on the remaining-work list once role changes mid-batch become a real concern.
6. **Conflict warning trigger** → render-time pairwise heuristic in `web/src/components/staging/conflicts.ts` (Phase B). Production-state lookup is Phase C tail work.
7. **Cleanup cadence** → daily sweeper (`internal/staging/sweep.go`), 30-day retention, mirrors `flags.RetentionSweeper`.
8. **Tests** → Go unit + Vitest + Playwright smoke. Playwright still pending (blocked on PR #84's migration-037 fix until that lands).

## Critical files / patterns reused

- **Registry**: `internal/staging/registry.go` mirrors `internal/auth/revert_registry.go`.
- **Audit writer**: `internal/auth/audit_*` — staged commits write audit rows the same way direct mutations do; `metadata.staged_at` is spliced into the JSON `new_value` envelope as `_staged_at` (a real metadata column would be cleaner once added).
- **Diff component**: `web/src/components/audit/AuditDiff.tsx` — reused by `StagedChangeRow`.
- **ConfirmDialog**: `web/src/components/ConfirmDialog.tsx` for "Discard all".
- **Migration template**: `migrations/060_add_flag_delete_after.up.sql` for naming + style.
- **Sweeper**: `internal/flags/retention_sweep.go`.

## Phase A — Backend foundation (Merged via PR #86)

- [x] `migrations/061_create_staged_changes.{up,down}.sql` — partial unique index uses `COALESCE(resource_id, provisional_id, sentinel)` + `COALESCE(field_path, '')` so latest-edit-wins for updates/toggles while provisional creates with distinct ids coexist.
- [x] `internal/staging/repository.go` — `Repository` interface (Upsert / List / Get / Delete / DeleteOlderThan / CountForUser).
- [x] `internal/staging/service.go` — Stage / ListForUser / Commit (transactional, all-or-nothing) / DiscardOne / DiscardAll. Builds an audit entry per committed row with `_staged_at` annotation.
- [x] `internal/staging/registry.go` — `CommitRegistry` mapping `(resource_type, action) → CommitHandler`.
- [x] `internal/staging/overlay.go` — generic `OverlayList[T]` / `OverlayDetail[T]`. Helpers in place; no list/detail handlers consume them yet (Phase C tail work).
- [x] `internal/staging/provisional.go` — `IsProvisional` / `MustNotBeProvisional`. Provisional UUIDs minted with non-RFC-4122 variant byte (constant-time bit-mask check).
- [x] HTTP handler — `GET / POST stage / POST commit / DELETE :id / DELETE all` under `/api/v1/orgs/:orgSlug/deploy-changes`.
- [x] `internal/staging/sweep.go` — daily sweeper, 30-day retention, mirrors `flags.RetentionSweeper`.
- [x] `internal/flags/staging_commit.go` registers `flag.toggle` (Phase A's only commit handler — extended in C-1/C-2).
- [x] `cmd/api/main.go` wires repo + registry + service + handler + sweeper.
- [x] Tests: provisional / registry / overlay / sweep / service unit tests + Postgres integration tests for the repo (gated by `DS_TEST_DATABASE_DSN`).
- [x] Migration semantics manually verified against a scratch postgres.

**Decision deferred from Phase A**: explicit per-route mutation-endpoint branching middleware. Approach instead: dashboard pages call `stagingApi.stage(...)` explicitly via the `stageOrCall` helper (Phase C-5+). The middleware approach in the original plan is no longer planned.

## Phase B — Header chrome + Review page (Merged via PR #87)

- [x] `web/src/api.ts` — `stagingApi` client + types.
- [x] `web/src/components/staging/StagingHeaderBanner.tsx` — sticky banner; polls every 30s + on focus; "Discard all" via `ConfirmDialog`. Mounted in `HierarchyLayout`.
- [x] `web/src/pages/DeployChangesPage.tsx` — `/orgs/:orgSlug/deploy-changes`. Groups by resource type. Bulk Select all / Select none / Discard selected / Deploy.
- [x] `web/src/components/staging/StagedChangeRow.tsx` — per-row presentation, reuses `AuditDiff` for old→new.
- [x] `web/src/components/staging/labels.ts` + `conflicts.ts` (pairwise heuristic).
- [x] Vitest: banner count + plural phrasing, page grouping + selection state, Deploy posts the right ids, partial-failure surfaces `failed_id` + reason, `computeConflicts` pairwise behavior.
- [ ] Playwright smoke — deferred (blocked on PR #84's migration-037 fix). Will land once `make e2e-sdk-up` is back to clean.

## Phase C — Per-resource backend handlers + UI conversions (in queue)

### Backend commit handlers (PRs #89, #91, #92, #93)

Each PR adds tuples to `*CommitHandlers(svc) []CommitTuple` and the `cmd/api/main.go` registration loop picks them up.

- [x] **C-1 (#89, open)** — `flag.update` / `flag.archive` / `flag.restore`.
- [x] **C-2 (#91, open)** — `flag_rule.update` / `flag_rule.delete` / `flag_rule_env_state.update` / `flag_env_state.update`.
- [x] **C-3 (#92, open)** — `setting.update` / `setting.delete` (in `internal/settings/`); `member.role_changed` (in `internal/members/` — invites + removals stay direct per spec).
- [x] **C-4 (#93, open)** — `strategy.update` / `strategy.delete` / `strategy_default.upsert/delete` / `rollout_policy.upsert/delete` (in `internal/rollout/`). Strategy update preserves optimistic concurrency by carrying `expected_version` in the payload.

### UI plumbing (PR #94 merged)

- [x] `web/src/hooks/useStagingEnabled.ts` — per-org hook backed by `localStorage:ds_staging_enabled:<orgSlug>`. Listens to native `storage` (cross-tab) + custom `ds:staging-enabled` (same-tab) events.
- [x] `web/src/hooks/stageOrCall.ts` — generic helper returning `{mode: 'staged', row}` or `{mode: 'direct', result}`.

### Per-page UI conversions (PRs #94–#100)

Pattern across all of these: import the two hooks, derive `stagingEnabled = useStagingEnabled(orgSlug)`, wrap the existing direct call in `stageOrCall({staged, orgSlug, stage:{...}, direct:() => ...})`. Optimistic local updates preserved on both branches; failures revert. One-file diff per page; no new tests (the hooks' unit tests cover dispatch).

- [x] **C-5 (#94, merged)** — FlagDetailPage `handleArchive` + `handleRestore`.
- [x] **C-6 (#95, open)** — FlagDetailPage `saveEnvState` (per-env enabled/value).
- [x] **C-7 (#96, open)** — FlagDetailPage `handleUpdateRule` + `handleDeleteRule`.
- [x] **C-8 (#97, open)** — FlagDetailPage `handleRuleEnvToggle`.
- [x] **C-9 (#98, open)** — FlagDetailPage `handleSettingsSave`.
- [x] **C-10 (#99, open)** — MembersPage `handleChangeRole`.
- [x] **C-11 (#100, open)** — StrategiesPage `handleDelete` + StrategyEditor `submit` (update branch).

### Operator-facing toggle

- [x] **C-12 (#101, open)** — `StagingModeToggle` on DeployChangesPage. Reads/writes `useStagingEnabled` / `setStagingEnabled`. Replaces the DevTools-only opt-in.

### Pages with no staging conversion (intentional)

- **SettingsPage** — `settingsApi` exists in `api.ts` but isn't consumed by any UI code. SettingsPage works through `entitiesApi`/`webhooksApi`/`notificationsApi`, none of which currently have staging commit handlers.
- **No standalone UI for `strategy_default` / `rollout_policy`** — backend handlers exist (C-4) but no dedicated page surfaces them today.

## Remaining work

### 1. Provisional-id resolution + `*.create` commit handlers

`flag.create`, `flag_rule.create`, and `strategy.create` are intentionally absent from the registry. A staged create has no `resource_id` (only a `provisional_id`), and other staged rows in the same batch may reference that placeholder (e.g., create a flag + add a rule to it before deploying).

**Needed before code:**
- Sub-plan in `docs/superpowers/plans/` proposing the rewrite algorithm — when `Service.Commit` opens its transaction and walks the rows, it must resolve every `provisional_id` to the real id minted by the create handler, then patch any subsequent row in the same batch that references it.
- Per-resource `commitFlagCreate` / `commitFlagRuleCreate` / `commitStrategyCreate` handler signatures.
- Updated `Service.Commit`: today refuses provisional rows with a clear error. Replace that with the resolution step.
- UI conversions for the corresponding `handleAddRule` / FlagCreatePage / StrategyEditor `create` branch.

### 2. Real feature-flag-backed gate

Replace the localStorage backing in `useStagingEnabled` with a check against the existing flag service for `staged-changes-enabled` (release category, expires after rollout). Hook signature stays the same; no caller changes.

**Sub-tasks:**
- Flag seed (or admin-script bootstrap) creating `staged-changes-enabled` per org.
- Wrap a `useFlag('staged-changes-enabled')` call into the per-org semantics.
- Graceful fallback when the flag doesn't yet exist (treat as off).
- The irony noted in the spec: `StagingModeToggle` should stage its own flag toggle when staging is on.

### 3. One-time org-wide enablement

After the queue drains and dogfooding settles, write a small admin script or migration that flips `staged-changes-enabled` ON for every existing org. Spec's "Done when" criteria become acceptance.

### 4. Production-state conflict detection

Today `web/src/components/staging/conflicts.ts` is a pairwise heuristic. Replace with a real production-state lookup at render time on the review page.

### 5. Phase B Playwright smoke

Add the dedicated staging Playwright spec once `make e2e-sdk-up` is back to clean (blocked on PR #84).

## Verification (gates each PR has been passing)

- `go vet ./...` clean
- `go build ./...` clean
- `go test ./... -short` clean
- `cd web && npx tsc --noEmit && npm run lint && npm run build && npx prettier --check "src/**/*.{ts,tsx,css,json}" && npx vitest run`

## Out of scope (per spec, restated)

- Multi-user collaboration on the same batch
- Branch / merge semantics
- Pre-deploy simulation
- Two-person approval (separate spec / PR)
- CLI / SDK staging UX

## Completion Record

<!-- Fill when Phase: Complete (after all queued PRs land + provisional-id work + flag wiring) -->
- **Branches**: many (one per PR); see status snapshot.
- **Committed**: A + B + C-5 plumbing on main; C-1/C-2/C-3/C-4 backend + C-6/C-7/C-8/C-9/C-10/C-11 UI + C-12 toggle in queue.
- **Pushed**: Yes (all in queue have remote branches and PRs).
- **CI Checks**: Each PR's CI green; whole-initiative completion blocked on remaining-work items 1–5.
