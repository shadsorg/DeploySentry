# Configurable Rollout Strategies

**Date:** 2026-04-18
**Status:** Design

## Overview

DeploySentry ships three deployment strategies today (canary, blue-green, rolling) with their traffic percentages and dwell times hardcoded in Go (`internal/deploy/strategies/*.go`). Users can pick a strategy name when creating a deployment but cannot tune step percentages, durations, health thresholds, or advancement behavior. Flag/config changes apply immediately with no rollout concept at all — there's no way to progressively shift a flag value across traffic with dwell and health gating, the way a code deploy is shifted.

This design introduces:

1. **User-defined `Strategy` templates** — first-class, org/project/app-scoped, with rich step semantics (percent, min/max duration, bake-time-until-healthy, per-step health threshold, approval gates, abort conditions, notifications). Importable/exportable as YAML.
2. **A unified `Rollout` entity** — wraps any change being progressively applied: a deploy (traffic shift) or a config change (SDK rollout percentage; flags are just booleans in this system). Each rollout is tracked independently with its own phases, state, and rollback.
3. **Optional `Release` bundles** — group related rollouts (e.g., "v1.1 ship = deploy + feature flag enable") for unified UI and audit. Rollouts within a bundle are independent by default; two opt-in coordination policies allow cross-rollout pause or cascade.
4. **A generalized phase engine** — refactored from `internal/deploy/engine/` to `internal/rollout/engine/`, driving all rollouts through pluggable target applicators (deploy → Nginx/Envoy weight; config → SDK rollout %).
5. **Scope-level onboarding** — org/project/app can enable rollout control with `mandate` or `prompt` policy, preserving today's immediate-apply behavior for non-onboarded scopes (full backward compatibility).
6. **Strategy defaults matrix** — per (scope, environment, target_type) → strategy, with inheritance down the scope hierarchy.

The design deliberately treats flags and configs as the same target type (`config`), since flags are configs with boolean type in DeploySentry. Strategy interpretation is polymorphic by target type: `deploy` steps shift traffic weight; `config` steps shift SDK evaluation rollout percentage.

## Goals

- Let users define reusable rollout strategies with rich semantics (health-adaptive dwell, per-step gates, approval, fast-abort on user-defined thresholds).
- Tie a strategy to either a new version (deploy) or a config/flag change, using the same engine.
- Allow independent rollout tracking for co-shipped changes (roll back the flag without rolling back the deploy) with optional coordination policies when tighter coupling is needed.
- Guardrail onboarded scopes: mandate strategy usage or prompt the user, without breaking non-onboarded scopes.
- Provide runtime operator controls: pause, resume, rollback, promote, approve, force-promote (with required audit reason).
- Full backward compatibility: non-onboarded orgs, existing deploys, existing flag edits all continue to work unchanged.

## Non-Goals

- **Rolling out arbitrary external config**: only DeploySentry-managed values (flags + configs under `internal/flags/` / future config primitive). Out-of-system values are not in scope.
- **Retargeting in flight ("amend")**: Q7 deferred this. The 409 error exposes `amend` as a hint only; the operation itself is a follow-on.
- **Advanced runtime controls**: skip-to-step-N, split, extend-dwell. Ship the six baseline controls; add advanced later if asked.
- **Cross-org strategy sharing**: templates live in one org's scope tree. No marketplace.
- **Gate composition DSL (Q2 option D)**: each step has the named fields from Q2-C, not an arbitrary ordered list of gate primitives.
- **Automatic rollout attachment to settings writes** (`internal/settings/`): settings are out of scope for phase 1.

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Strategy semantics | Polymorphic by `target_type` | One strategy library serves both deploys and configs; each target applicator interprets `percent` and the other step fields in its own terms. |
| Step shape | Rich (percent, min/max dur, bake_time, per-step health, approval, notify, abort) | Covers the real gaps (health-adaptive dwell, fast abort) without the composition complexity of a gate list. |
| Template lifecycle | Snapshot into rollout at attach time | Mid-rollout edits to the template do not affect in-flight rollouts. Deterministic, auditable. |
| Rollout data model | Independent `Rollout` rows per target, optional `Release` grouping | Matches real ops: deploy + flag ship together but roll back independently. Release is an umbrella for UI + audit + optional coordination. |
| Release coordination default | `independent` | Matches the driving scenario (flag rolls back, deploy continues). `pause_on_sibling_abort` and `cascade_abort` are opt-in per Release. |
| Target types | `deploy` and `config` (flags = boolean configs) | Clean two-type model; one rollout applicator per type. |
| Trigger model | Opt-in by default; onboarded scopes pick `mandate` or `prompt` | Backward compatible for non-onboarded; explicit guardrail for prod. |
| Edit during rollout | 409 with action hints | Rollout owns its target until done. No surprising implicit behavior on direct writes. |
| Health signal | App+env health monitor by default; per-step override | Reuses existing health infrastructure. Override exists for "flag only touches checkout" cases. |
| Runtime controls | Six actions (pause, resume, rollback, promote, approve, force-promote) | Table-stakes + approval + force-override. Force-promote requires audit reason. |
| Template scope | Org with project/app overrides | Matches existing hierarchical patterns (settings, members, env scope). |
| Defaults | Per (scope, environment, target_type) matrix with inheritance | Enables "all prod deploys get standard canary" without per-rollout boilerplate. |
| Authoring surface | API + web form + YAML DSL | Web UI for onboarding; YAML+CLI for version-controlled platform management. |

## Data Model

New tables in the `deploy` schema:

### `strategies`

Reusable template. Scoped to org, project, or app.

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `scope_type` | text | `org` / `project` / `app` |
| `scope_id` | uuid | |
| `name` | text | Unique per `(scope_type, scope_id)` |
| `description` | text | |
| `target_type` | text | `deploy` / `config` / `any` |
| `steps` | jsonb | Ordered array (see Strategy Schema) |
| `default_health_threshold` | numeric | e.g., 0.95 |
| `default_rollback_on_failure` | bool | |
| `version` | int | Optimistic concurrency; incremented on PUT |
| `is_system` | bool | Seeded system templates cannot be deleted |
| `created_by`, `updated_by`, `created_at`, `updated_at` | | |

### `strategy_defaults`

Pins a default strategy per (scope, environment, target_type). Inheritance: most-specific scope wins.

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `scope_type`, `scope_id` | | |
| `environment` | text NULL | NULL = any environment |
| `target_type` | text NULL | NULL = any target |
| `strategy_id` | uuid FK → strategies | |
| Unique index: `(scope_type, scope_id, coalesce(environment,''), coalesce(target_type,''))` |

### `rollout_policies`

Onboarding + mandate policy per scope.

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `scope_type`, `scope_id` | | |
| `environment` | text NULL | Optional env-specific override |
| `target_type` | text NULL | Optional target-specific override |
| `enabled` | bool | |
| `policy` | text | `mandate` / `prompt` / `off` |
| Unique index: `(scope_type, scope_id, coalesce(environment,''), coalesce(target_type,''))` |

### `rollouts`

One row per in-flight or historical rollout.

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `release_id` | uuid NULL FK → releases | |
| `target_type` | text | `deploy` / `config` |
| `target_ref` | jsonb | `{"deployment_id": "..."}` or `{"flag_key":"...", "env":"..."}` |
| `strategy_snapshot` | jsonb | Resolved template + overrides, frozen at attach |
| `signal_source` | jsonb | Default app+env health; may be overridden per step (in snapshot) |
| `status` | text | `pending` / `active` / `paused` / `awaiting_approval` / `succeeded` / `rolled_back` / `aborted` / `superseded` |
| `current_phase_index` | int | Cached pointer; truth is `rollout_phases` |
| `current_phase_started_at` | timestamptz | |
| `last_healthy_since` | timestamptz NULL | Tracks bake-time-healthy |
| `rollback_reason` | text NULL | |
| `created_by`, `created_at`, `completed_at` | | |
| Index: `(status)`, `(release_id)`, `((target_ref->>'deployment_id'))`, `((target_ref->>'flag_key'), (target_ref->>'env'))` |

### `rollout_phases`

Per-phase ledger (truth over time).

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `rollout_id` | uuid FK | |
| `phase_index` | int | |
| `step_snapshot` | jsonb | The step as resolved at attach |
| `status` | text | `pending` / `active` / `awaiting_approval` / `passed` / `failed` / `rolled_back` |
| `entered_at`, `exited_at` | | |
| `applied_percent` | numeric | |
| `health_score_at_exit` | numeric NULL | |
| `notes` | text | |
| Unique: `(rollout_id, phase_index)` |

### `rollout_events`

Audit trail.

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `rollout_id` | uuid FK | |
| `event_type` | text | `attached` / `phase_entered` / `phase_exited` / `paused` / `resumed` / `promoted` / `force_promoted` / `approved` / `abort_condition_tripped` / `rollback_triggered` / `completed` / `superseded` |
| `actor_type` | text | `user` / `system` |
| `actor_id` | uuid NULL | |
| `reason` | text NULL | Required for `force_promoted` and `rollback_triggered` (operator action) |
| `payload` | jsonb | Event-specific data |
| `occurred_at` | timestamptz | |
| Index: `(rollout_id, occurred_at)` |

### `releases`

Optional bundle.

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `scope_type`, `scope_id` | | |
| `name` | text | |
| `description` | text | |
| `coordination_policy` | text | `independent` / `pause_on_sibling_abort` / `cascade_abort` |
| `created_by`, `created_at` | | |
| Note: Release `status` is derived from member rollouts at read time (not persisted) |

Existing `deployments` table is **not modified**. Rollouts reference a deployment via `target_ref.deployment_id`.

## Strategy Schema

YAML representation (canonical for import/export; jsonb in DB matches shape):

```yaml
name: prod-canary-standard
scope: org/acme
target_type: deploy            # deploy | config | any
default_health_threshold: 0.95
default_rollback_on_failure: true
steps:
  - percent: 1
    min_duration: 5m
    max_duration: 30m
    bake_time_healthy: 5m
    abort_conditions:
      - metric: error_rate
        operator: ">"
        threshold: 0.02
        window: 60s
  - percent: 5
    min_duration: 5m
    max_duration: 1h
    bake_time_healthy: 10m
    health_threshold: 0.98
    approval:
      required_role: sre_oncall
      timeout: 2h
    notify:
      on_entry: [slack:#releases]
      on_exit:  [slack:#releases]
  - { percent: 25, min_duration: 10m, max_duration: 2h, bake_time_healthy: 10m }
  - { percent: 50, min_duration: 10m, max_duration: 2h, bake_time_healthy: 10m }
  - { percent: 100, min_duration: 0, max_duration: 0, bake_time_healthy: 0 }
```

### Step field semantics

| Field | Type | Required | Meaning |
|---|---|---|---|
| `percent` | 0-100 | yes | Target-dependent. Deploy: upstream weight. Config: SDK rollout %. |
| `min_duration` | duration | yes | Phase cannot advance before this elapses. |
| `max_duration` | duration | yes | If still unhealthy at max, rollback. Set equal to `min_duration` for strict fixed-time phases. `0` means instant (terminal phase). |
| `bake_time_healthy` | duration | yes | Advance once signal has been healthy for this long (and `min_duration` has elapsed). `0` means advance at `min_duration`. |
| `health_threshold` | 0-1 | no | Override strategy default for this phase. |
| `approval` | object | no | `{ required_role, timeout }`. Phase transitions to `awaiting_approval`. |
| `notify` | object | no | `{ on_entry: [channel], on_exit: [channel] }`. Channels reference existing notification channels. |
| `abort_conditions` | array | no | Each: `{ metric, operator, threshold, window }`. If ANY trips, rollback immediately (skips dwell). |
| `signal_override` | object | no | Override `signal_source` for this phase only. |

### Target interpretation

- **`deploy`**: `percent` → Nginx upstream weight or Envoy xDS cluster weight via the existing consumer/agent path. `target_ref.deployment_id` points at the existing `deployments` row.
- **`config`**: `percent` → SDK rollout percentage for `(flag_key, env)`. The existing SDK bucket-hash logic already supports this; we just update the persisted value step by step. Flags are configs with boolean type; configs can be any type (`boolean`, `string`, `number`, `json`).

## Rollout Lifecycle

The phase engine lives at `internal/rollout/engine/` (generalized from `internal/deploy/engine/engine.go`). It subscribes to the `rollouts.rollout.created` NATS subject. Per rollout:

```
1. Load rollout + strategy_snapshot + signal_source
2. For each step in order:
     a. Mark phase Active; call target applicator to apply `percent`
     b. Fire notify.on_entry; emit phase_entered event
     c. Loop with short sleep until promoted:
        - Evaluate abort_conditions        → if tripped → rollback (skip dwell)
        - If approval.required and ungranted → status = awaiting_approval; pause
        - Get current health score from signal source (or step override)
        - If healthy ≥ threshold, advance last_healthy_since
        - If elapsed ≥ min_duration AND (now - last_healthy_since) ≥ bake_time_healthy
             → promote
        - If elapsed ≥ max_duration AND not healthy
             → rollback (unhealthy timeout)
     d. Fire notify.on_exit; mark phase Passed; emit phase_exited event
3. On completion → Succeeded; publish rollout.completed
```

### Applicator interface

```go
package applicator

type Applicator interface {
    Apply(ctx context.Context, rollout *Rollout, step Step) error
    Revert(ctx context.Context, rollout *Rollout) error
    CurrentSignal(ctx context.Context, rollout *Rollout, overrides *SignalSource) (HealthScore, error)
}
```

Two implementations:

- `internal/rollout/applicator/deploy/` — wraps existing traffic-percent update logic from `internal/deploy/`; continues publishing the `deployments.deployment.phase_changed` NATS subject so existing consumers (Nginx, xDS agent) need no changes.
- `internal/rollout/applicator/config/` — writes the rollout percentage for `(flag_key, env)` to the flags/config store; SDK already supports bucket-hash rollout.

### Abort conditions

Evaluated on a short timer (~5s) by the engine during each phase's loop. Metrics available: `error_rate`, `latency_p99`, `latency_p50`, `request_rate`, any custom metric registered in the health monitor. First trip wins; the event is logged with the tripping condition and current metric value.

### Approval

When a phase has `approval.required_role`, the engine transitions the rollout to `awaiting_approval` on phase entry (after applying percent), fires `notify.on_entry`, and waits. A user in the required role calls `/rollouts/:id/approve` to unblock. If `approval.timeout` elapses, rollback is triggered.

### Release coordination

On rollout transition to `rolled_back`, the engine checks `release.coordination_policy`:

- `independent` — no-op.
- `pause_on_sibling_abort` — active siblings → `paused` (reason: `sibling_aborted:{id}`).
- `cascade_abort` — active siblings begin rollback.

Transitions go through `rollout_events` for full audit.

### Runtime controls

Six operator actions:

| Action | Endpoint | Effect | Audit |
|---|---|---|---|
| pause | POST `/rollouts/:id/pause` | Freeze; dwell timer suspended | actor, timestamp |
| resume | POST `/rollouts/:id/resume` | Unfreeze | actor, timestamp |
| rollback | POST `/rollouts/:id/rollback` | Revert target, fire notifications | actor, timestamp, **reason required** |
| promote | POST `/rollouts/:id/promote` | Skip remaining dwell on current phase; advance if healthy | actor, timestamp |
| approve | POST `/rollouts/:id/approve` | Grant approval on an `awaiting_approval` phase | actor, timestamp |
| force-promote | POST `/rollouts/:id/force-promote` | Advance even if unhealthy | actor, timestamp, **reason required** |

Force-promote and rollback require a non-empty `reason` string in the request body. API rejects empty/missing reason with 400.

### Events and notifications

All state transitions persist a `rollout_events` row AND publish a NATS event on `rollouts.rollout.{event_type}`. The web UI consumes these via SSE on `/rollouts/:id/events` for live updates. Notification channels (from step `notify`) fire through the existing `internal/notifications/` package.

## Onboarding, Policy, & Defaults

### Onboarding (`rollout_policies`)

At org/project/app scope — optionally further refined by environment or target_type — an admin sets `enabled=true` plus a `policy`:

- `policy=off` (or row absent): original immediate-apply behavior. Zero behavior change.
- `policy=prompt`: API and web UI prompt the user to attach a strategy; CLI requires `--strategy` or `--apply-immediately`. On API calls without rollout info, a 422 with `{ hint: "attach_strategy", available_defaults: [...] }` is returned.
- `policy=mandate`: every qualifying change must resolve to a strategy (explicit or via defaults). Missing resolution → 400.

Policy resolution walks scope most-specific first (app → project → org), then matches on `(environment, target_type)` with NULL wildcards.

### Default strategy resolution

When a change requests a strategy implicitly (no `strategy_id`, no overrides):

1. Walk scopes most-specific → least-specific: app → project → org.
2. Within each scope, match in priority order:
   1. `(environment, target_type)` exact
   2. `(environment, NULL)` — any target
   3. `(NULL, target_type)` — any env
   4. `(NULL, NULL)` — wildcard
3. First hit across the combined walk wins.
4. If `policy=mandate` and no default resolves, return 400 with hint to set one.

### Template inheritance

Listing templates for scope S returns templates from S + ancestors. Name collisions shadow less-specific. The returned DTO includes `origin_scope` metadata so the UI can surface "inherited from org" vs "defined here".

## API Surface

All endpoints live under the existing hierarchical paths (org/project/app). Listed once here as `/...` for brevity.

### Strategies (CRUD + YAML round-trip)

- `GET    /api/v1/.../strategies` — list (includes inherited)
- `POST   /api/v1/.../strategies` — create
- `GET    /api/v1/.../strategies/:name` — get (resolves inherited)
- `PUT    /api/v1/.../strategies/:name` — update (requires `version` for optimistic concurrency)
- `DELETE /api/v1/.../strategies/:name` — blocked if referenced by any non-terminal rollout; system strategies cannot be deleted
- `POST   /api/v1/.../strategies/import` — YAML body; creates or updates
- `GET    /api/v1/.../strategies/:name/export` — YAML body

### Policies & defaults

- `GET /PUT /api/v1/.../rollout-policy`
- `GET /PUT /api/v1/.../strategy-defaults` (matrix editor; full replace)

### Rollouts

- `GET  /api/v1/.../rollouts` — list, filter by target_type, status, release_id, date range
- `GET  /api/v1/.../rollouts/:id` — detail with phases
- `POST /api/v1/.../rollouts/:id/pause`
- `POST /api/v1/.../rollouts/:id/resume`
- `POST /api/v1/.../rollouts/:id/rollback` — body: `{ reason: string }` (required)
- `POST /api/v1/.../rollouts/:id/promote`
- `POST /api/v1/.../rollouts/:id/approve`
- `POST /api/v1/.../rollouts/:id/force-promote` — body: `{ reason: string }` (required)
- `GET  /api/v1/.../rollouts/:id/events` — SSE stream

### Releases

- `GET  /api/v1/.../releases`
- `POST /api/v1/.../releases` — create empty bundle
- `GET  /api/v1/.../releases/:id` — with member rollouts
- `PUT  /api/v1/.../releases/:id` — edit name/description/coordination_policy
- `POST /api/v1/.../releases/:id/attach` — body: `{ rollout_id }`

### Extended existing endpoints

- `POST /api/v1/.../deployments` — accepts optional `rollout: { strategy_id?, overrides?, release_id? }`
- `PUT  /api/v1/.../flags/:key/rules` — accepts the same

Both return **409 Conflict** when an active rollout already owns the target:

```json
{
  "error": "rollout_in_progress",
  "active_rollout_id": "uuid",
  "options": ["abort", "wait", "amend"]
}
```

(`amend` is listed as a hint; the actual endpoint is not shipped in phase 1.)

## CLI

New subcommand groups under `cmd/cli/`:

- `ds strategies {list, get, apply, delete, import, export}` — YAML round-trip via `--file`/`-f`
- `ds rollouts {list, get, pause, resume, rollback, promote, approve, force-promote, watch}`
- `ds releases {list, get, create, attach}`

Extensions to existing subcommands:

- `ds deploy` gains `--strategy <name>`, `--release <id|name>`, `--apply-immediately`
- `ds flags set-rule` gains the same

`rollback` and `force-promote` require `--reason "<text>"`; CLI errors with clear message if missing.

## Web UI

New pages under `web/src/pages/`:

- **StrategiesPage** — list + create/edit. Editor has two tabs: structured step builder and YAML.
- **RolloutsPage** — list with status badges and mini timeline per row; filters for target_type, status, release.
- **RolloutDetailPage** — phase progression timeline, live signal graph, six action buttons, event log. Force-promote and rollback open a modal requiring a reason before submit.
- **ReleasesPage** + **ReleaseDetailPage** — grouped view of member rollouts; overlaid timeline.
- **Policy & Defaults** — new tabs under each scope's Settings page (org / project / app).

Existing pages extend:

- Deployment create form: strategy picker (populated from inherited templates) + optional release picker.
- Flag rule edit: onboard-aware — if onboarded and policy=prompt, show strategy picker with "apply immediately" alternative; if policy=mandate, strategy picker is required.
- Dashboard homepage: new "Active rollouts" card.

## Migration & Backward Compatibility

- **Existing `deployments` table unchanged.** `rollouts` references deployments via `target_ref.deployment_id`.
- **Hardcoded strategies seeded as system templates.** At migration time, create `is_system=true` strategies in every existing org named `system-canary`, `system-blue-green`, `system-rolling` with the current hardcoded values from `internal/deploy/strategies/*.go`. Insert default `strategy_defaults` rows pointing at `system-canary` for `(any, deploy)` so existing behavior is preserved without onboarding.
- **Non-onboarded orgs:** `rollout_policies` row absent → API routes behave as today. Internally the phase engine still runs, but through the seeded system strategy; outcome is identical.
- **NATS subjects:** `deployments.deployment.*` continues to be emitted by the deploy applicator. New `rollouts.rollout.*` subjects added in parallel. Existing consumer (`cmd/consumer/`) and agent (`cmd/agent/`) need no changes.
- **Old `internal/deploy/strategies/` package** retained as the implementation body of the three seeded system strategies in phase 1; deprecated once templates can fully replace it in a follow-up.
- **`internal/deploy/engine/`** becomes a thin adapter over `internal/rollout/engine/`, keeping its package path for existing imports. Phase-engine internals move to the rollout package.

## Testing Strategy

- **Unit:**
  - Strategy schema validation (valid ranges, required fields, duration parsing)
  - Template + override merge producing the expected snapshot
  - Defaults resolution matrix (every inheritance path)
  - Phase state machine transitions (all legal + illegal source→target pairs)
  - Abort-condition evaluator
  - Policy resolution walker

- **Integration (real DB per project conventions):**
  - Full rollout lifecycle with stubbed signal source: healthy → promotes; unhealthy → rollback at `max_duration`; abort trips → immediate rollback.
  - Concurrent-edit 409 returned when POST/PUT collides with an active rollout.
  - Release coordination: each of `independent`, `pause_on_sibling_abort`, `cascade_abort` under a sibling rollback.
  - Force-promote with missing reason → 400; with reason → advances and logs event.
  - Policy mandate with no default → 400; with default → uses default.

- **End-to-end:**
  - Real deploy rollout through a full strategy against a test Nginx/agent consumer.
  - Real flag/config rollout updating SDK rollout % across steps.
  - Multi-rollout Release with each coordination mode.
  - Template inheritance across org → project → app.

- **Regression:**
  - Existing deploy engine tests continue to pass under the refactored adapter with seeded system strategies.
  - CLI/API test suites for existing deploy and flag flows unchanged (verifies zero breaking change for non-onboarded orgs).

## Open Questions / Deferred

- **Amend / retarget**: surfaced as a 409 hint; endpoint not shipped.
- **Settings rollouts** (`internal/settings/`): out of scope phase 1.
- **Advanced runtime controls** (skip-to-step, split, extend-dwell): deferred.
- **Metric source for custom abort conditions**: phase 1 uses only metrics the existing health monitor produces. External metric sources (Prometheus URL, custom pushgateway) deferred.
- **Notification payload schema**: reuse existing `internal/notifications/` channel contract; detailed per-event payload design happens at plan time.

## Rollout Plan (High-Level Phases)

1. **Schema + entities** — migrations for all new tables; Go models.
2. **Strategy CRUD + templates** — repository, service, handler, YAML import/export, inheritance resolver.
3. **Phase engine generalization** — move `internal/deploy/engine/` → `internal/rollout/engine/`; applicator interface; deploy applicator wrapping existing behavior; regression green.
4. **Rollout entity + lifecycle** — create/attach, phase advancement, abort conditions, approval, 6 runtime controls.
5. **Config applicator** — SDK rollout % driver.
6. **Onboarding + policy + defaults** — `rollout_policies`, `strategy_defaults`, resolver, mandate/prompt enforcement.
7. **Releases + coordination** — bundle entity, coordination policies.
8. **CLI** — new subcommand groups + `--strategy`/`--release` on existing commands.
9. **Web UI** — Strategies, Rollouts, Releases pages; extend deploy create and flag rule edit.
10. **Seed + migrate existing orgs** — system strategies, default assignments.
11. **Docs** — update `Traffic_Management_Guide.md` and add `Rollout_Strategies.md`.
