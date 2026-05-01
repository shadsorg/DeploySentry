# Staged Changes + Deploy Review Workflow — Implementation Plan

**Phase**: Design (open questions below) → Implementation (after sign-off)
**Date**: 2026-05-01
**Spec**: [`../specs/2026-04-30-staged-changes-and-deploy-workflow-design.md`](../specs/2026-04-30-staged-changes-and-deploy-workflow-design.md)
**Branch (proposed)**: `feature/staged-changes-foundation` (Phase A); subsequent phases get their own branches.

## Overview

Introduce a per-user staging layer for dashboard mutations. UI writes go to `staged_changes` instead of production; the dashboard overlays the user's staged rows on read; a header chrome surfaces the count; a Review page commits the selected rows in one DB transaction. SDK / CLI / webhook writes are unchanged.

This is a high-blast-radius change to commit semantics. It lands in three PRs (matching the spec's "Approach summary"), each gated so we can roll back granularly.

## Open design questions (resolve before Phase A code)

These are the spec ambiguities I need confirmed before writing code. None block planning — they shape implementation choices.

1. **Read-path overlay shape.** Per-handler patches vs. a generic Gin response-rewrite middleware. Recommend per-handler — the overlay logic is type-specific (different field paths per resource), and a generic middleware would have to deserialise/re-serialise every JSON body. Cost: more handler-side code; benefit: typed, testable, no JSON round-trip.
2. **Bypass switch for JWT requests.** Spec says `?immediate=true` is honoured for any caller. Confirm: this is intended (escape hatch for power users / debugging) and not gated by role. Default: dashboard never sets it.
3. **Feature flag.** Use `staged-changes-enabled` (release category, expires once rollout completes). Phase A wires the read+write paths but keeps every endpoint short-circuiting to direct writes when the flag is OFF, so we can land + soak before the UI flips on.
4. **Provisional-id egress.** Spec calls out leakage as a risk but doesn't specify enforcement. Proposal: a single helper `IsProvisionalID(uuid) bool` (UUIDs minted with a reserved version/variant byte, or a separate `provisional_ids` set held in `staged_changes_repo`) plus a unit test on every egress surface (audit writer, webhook payload builder, analytics emitter, SSE broadcaster) that calls a `MustNotBeProvisional(...)` guard.
5. **Permission re-check at commit.** Spec says "the commit handler registry runs the same RBAC middleware". `RequirePermission` is Gin middleware — won't run inside a transaction. Proposal: extract a `func CheckPermission(ctx, role, perm) error` from `RequirePermission` and call it inside each commit handler before it dispatches. Existing `RequirePermission` middleware remains the front-door check at the `/deploy-changes/commit` endpoint.
6. **Conflict warning trigger.** Spec says compare `old_value` to current production at commit time. Proposal: do that comparison at *render* time on the review page too, so the user sees the warning before clicking Deploy. Cheap — already loading the production rows.
7. **Cleanup cadence.** "30-day auto-discard" — implement as a cron-style sweeper (mirrors the flag retention sweep) or as a lazy delete on read? Recommend the sweeper, daily — it bounds the join cost on every read.
8. **Tests.** Audit-handler tests in `internal/auth/` use the existing test-DB harness. Mirror that for `internal/staging/`. Frontend tests: Vitest for the review page diff rendering; one Playwright smoke that stages two changes, reviews, deploys, and asserts the production state.

## Critical files / patterns to reuse

- **Registry**: mirror `internal/auth/revert_registry.go` — `RevertRegistry` is `(entity_type, action) → RevertHandler`; the commit registry is `(resource_type, action) → CommitHandler` with the same signature shape.
- **Audit writer**: `internal/auth/audit_*` — staged commits write audit rows the same way direct mutations do today; add `metadata.staged_at` per spec.
- **Diff component**: reuse `web/src/components/audit/` diff renderer (used by `OrgAuditPage`).
- **ConfirmDialog**: `web/src/components/ConfirmDialog.tsx` for "Discard all".
- **Migration template**: `migrations/060_add_flag_delete_after.up.sql` as a recent reference for naming + style.
- **Sweeper**: `internal/flags/retention_sweep.go` as the cron pattern for the 30-day cleanup.
- **Feature flag**: lifecycle/category fields enforced; release category needs `expires_at` (set ~6 months out).

## Phase A — Backend foundation

Goal: `staged_changes` table + read-overlay helpers + write-branching middleware + commit registry, all behind `staged-changes-enabled`. No UI surface yet.

### Schema (1 migration)

- [ ] `migrations/061_create_staged_changes.up.sql` — table + indexes per spec §"Schema sketch", in the `deploy` schema. `provisional_id` column, partial unique on `(user_id, org_id, resource_type, resource_id, field_path)` (NULLs treated distinct via expression index — verify with the Neon MCP `compare_database_schema` tool before merge).
- [ ] Down migration drops the table.

### Repository / service layer

- [ ] `internal/staging/repository.go` — `StagedChangesRepository` interface (`Upsert`, `ListForUser(orgID, userID)`, `ListForResource(orgID, resourceType, resourceID)`, `DeleteByIDs`, `DeleteAllForUser`, `DeleteOlderThan`).
- [ ] `internal/staging/service.go` — `StagingService` orchestrates upserts + listing + commit. Holds the commit-handler registry.
- [ ] `internal/staging/registry.go` — `CommitRegistry` mirroring `RevertRegistry`. Signature: `func(ctx, tx, row *models.StagedChange) error`. `Register(resourceType, action, handler)`.
- [ ] `internal/staging/overlay.go` — pure functions: `OverlayList[T]`, `OverlayDetail[T]` taking production data + staged rows + a per-resource patch func. Typed via generics so each call site is compile-time checked.
- [ ] `internal/staging/provisional.go` — `IsProvisional(id) bool` and `MustNotBeProvisional(...)` guards. Provisional ids carry a reserved variant byte (or are tracked in a per-process set) — pick one approach during implementation, document the choice.

### Write-branching

- [ ] `internal/staging/middleware.go` — Gin middleware that, for designated mutation routes, decides whether the write goes to `staged_changes` (UI/JWT request, flag ON, no `?immediate=true`) or passes through to the existing handler. When staging, it constructs the `StagedChange`, validates RBAC at request time, upserts, and short-circuits with 200 + the staged row.
- [ ] Bypass conditions: `auth_method != "jwt"`, OR `staged-changes-enabled` flag is off for this org, OR `?immediate=true`.

### Commit endpoint + handlers

- [ ] `POST /api/v1/orgs/:orgSlug/deploy-changes/commit` body: `{ ids: [uuid] }`. Handler opens one transaction, dispatches each row through `CommitRegistry`, writes audit log per committed row with `metadata.staged_at`, deletes the committed rows, returns the per-row commit result.
- [ ] `DELETE /api/v1/orgs/:orgSlug/deploy-changes/:id` — single discard.
- [ ] `DELETE /api/v1/orgs/:orgSlug/deploy-changes` — discard all for current user in this org.
- [ ] `GET /api/v1/orgs/:orgSlug/deploy-changes` — list current user's staged changes, joined with production for diff rendering.

### Per-resource registrations (Phase A scope = stub registry)

- [ ] Wire the registry in `cmd/api/main.go` but register **only one resource for Phase A**: `flag.toggle` (and its env-state cousin `flag_env_state.set`). Smallest end-to-end seam to prove the loop works without dragging the rest of the codebase.
- [ ] All other resources fall through to direct writes until Phase C.

### Tests

- [ ] `internal/staging/registry_test.go` — Register / dispatch / unknown-resource error.
- [ ] `internal/staging/overlay_test.go` — list/detail overlay correctness (update / delete / create / multi).
- [ ] `internal/staging/provisional_test.go` — guard fires on every egress surface (table-driven).
- [ ] `internal/staging/middleware_test.go` — branching matrix (jwt/api_key × flag-on/off × immediate-true/false).
- [ ] `internal/staging/commit_handler_test.go` — `flag.toggle` round-trip against the test DB.

### Sweeper

- [ ] `internal/staging/sweep.go` mirroring `internal/flags/retention_sweep.go`. Daily cadence, deletes `created_at < now() - 30d`. Logs counts.

### Done when (Phase A)

- Migration applies cleanly on a fresh DB and on the e2e DB.
- `go test ./internal/staging/...` clean.
- Feature flag OFF: every existing dashboard mutation still hits production directly.
- Feature flag ON for a test org: toggling a flag from the dashboard writes a `staged_changes` row instead of touching the production flag; `GET /deploy-changes` returns it; `POST /deploy-changes/commit` flips the production flag and removes the staged row.
- `staged-changes-enabled` is created in seed data with the test org pre-toggled on.

## Phase B — Header chrome + Review page

Goal: the user can see and act on staged changes without backend changes beyond what Phase A landed.

- [ ] Sticky header banner in `web/src/layouts/AppLayout.tsx` (or wherever `AppShell` lives). Polls `GET /deploy-changes` on a 30s interval + on focus + after each mutation. Hides when count is zero.
- [ ] `/orgs/:orgSlug/deploy-changes` page (`web/src/pages/DeployChangesPage.tsx`).
  - Group by resource type.
  - Per-row diff (reuse `web/src/components/audit/Diff.tsx` if present — confirm during implementation).
  - Per-row checkbox + per-row Discard.
  - Bulk Deploy Selected / Discard Selected.
  - Conflict warning banner per row when `old_value` no longer matches production.
- [ ] `web/src/api/staging.ts` — typed client for the four endpoints.
- [ ] Vitest: review-page diff rendering, conflict-warning rendering, bulk-action selection state.
- [ ] Playwright: stage flag-toggle → assert banner shows "1 pending" → open review page → click Deploy → assert production reflects the change → assert banner hides.

### Done when (Phase B)

- All five Phase B checkboxes pass.
- Lint / build / tsc clean.

## Phase C — Per-resource integration

Goal: every UI mutation surface in scope (per spec §"In scope") routes through the staging path. Each page lands as its own commit/PR for granular revert.

For each resource group, the unit of work is: register its commit handler, switch the dashboard's mutation calls to the staging path, add a Playwright assertion that the page works in both flag-on and flag-off modes.

- [ ] Feature flags — create / update / archive / restore (toggle already in Phase A).
- [ ] Targeting rules — create / update / delete / reorder.
- [ ] Per-environment rule state.
- [ ] Settings — project / app / env / monitoring links.
- [ ] Members — role changes only (invites stay immediate).
- [ ] Strategies — create / update steps.
- [ ] Strategy defaults / rollout policy.

After all of these land, flip `staged-changes-enabled` to ON by default org-wide via a one-time migration / admin script. Spec's "Done when" section is the acceptance.

## Verification (gates each PR must pass)

- [ ] `go test ./...` clean
- [ ] `cd web && npm run lint && npm run build && npx tsc --noEmit` clean
- [ ] Phase A: `make e2e-sdk-up && cd web && npx playwright test --project=sdk` 5/5 (regression check; staging is dashboard-only so SDK tests must stay green)
- [ ] Phase B / C: dedicated Playwright spec for the staging flow

## Out of scope (per spec, restated)

- Multi-user collaboration on the same batch
- Branch / merge semantics
- Pre-deploy simulation
- Two-person approval (separate spec / PR)
- CLI / SDK staging UX

## Completion Record

<!-- Fill when Phase: Complete -->
- **Branch**:
- **Committed**: No
- **Pushed**: No
- **CI Checks**:
