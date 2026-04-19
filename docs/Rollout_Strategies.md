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

## Attaching a strategy to a deploy

```
ds deploy create --org acme --project api --app web \
  --artifact ./build.tgz --version v1.2.3 \
  --strategy prod-canary-standard
```

If your scope is onboarded with `policy=mandate` and no `--strategy` is given, the CLI resolves the default assignment matrix and attaches it automatically. If no default resolves, the server returns 400 with a hint.

## Runtime operator controls

Once a rollout is in flight, six operator actions are available:

| Command | Meaning | Notes |
|---|---|---|
| `pause` | Freeze at current phase | Reason optional |
| `resume` | Unfreeze a paused rollout | Reason optional |
| `promote` | Skip remaining dwell on current phase | Health still checked before advancing |
| `approve` | Grant a manual approval gate | Reason optional |
| `rollback` | Revert the target to prior value | **Reason required** |
| `force-promote` | Advance even if unhealthy | **Reason required** (audit) |

```
ds rollouts list --org acme --status active
ds rollouts get <id> --org acme
ds rollouts rollback <id> --org acme --reason "p99 regression"
ds rollouts force-promote <id> --org acme --reason "manual override, verified in staging"
```

## System strategies

Every org is seeded with three system templates: `system-canary` (the default for deploys), `system-blue-green`, and `system-rolling`. They cannot be deleted and their values match what earlier versions of DeploySentry had hardcoded. Customize by copying (`ds strategies export … | ds strategies apply -f -`) under a new name.

## Current limitations (Plan 2)

- **Abort conditions** currently only support the `score` metric (overall health score). Richer metrics (error_rate, latency_p99_ms, latency_p50_ms, request_rate) are defined in the HealthScore struct but `internal/health/DeploymentHealth` does not yet expose them — those abort conditions will not trip until the health package is extended. Use `score` comparisons as the reliable abort signal for now.
- **HealthMonitor is not wired up in cmd/api/main.go** (neither Plan 1 nor Plan 2 introduced it). The rollout engine uses a no-op health reader that always returns healthy, so rollouts currently advance on time alone — matching the existing legacy deploy engine's behavior. Wiring up a real HealthMonitor is a future infrastructure task.
- **Config rollouts** (flag/config value rollouts via SDK) come in Plan 3.
- **Releases** (bundling related rollouts with coordination policies) come in Plan 4.
- **Web UI** comes in Plan 5.

## What's coming

The foundation plan ships template management only. Future plans wire strategies into live deploy and config rollouts, add runtime controls (pause/resume/promote/rollback/approve/force-promote), add Release bundles, and build the web UI.

See `docs/superpowers/specs/2026-04-18-configurable-rollout-strategies-design.md` for the full design.

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

### Revert and previous percentage

When a config rollout rolls back, the rule's percentage is restored to its pre-rollout value (captured at attach time). This preserves whatever partial rollout state existed before the user started the new rollout.

### Current limitations (Plan 3)

- Config rollouts use a no-op health reader — they advance on time alone. Wiring a real HealthMonitor is future work.
- A rule whose `percentage` decreases during a rollout (e.g., from 25% → 1% as the first step) is allowed; the engine treats the strategy's step values as absolute. Users should pick strategies whose first step matches or exceeds the current percentage.
- Only the `percentage` field of a rule is rolled out. Changing the rule's `Value` or type requires a direct edit (no progressive rollout of `Value` yet).
- Config rollouts advance by percentage only — no SDK-side bucket hashing logic changed; evaluation uses whatever the existing rule's percentage field is.

## Rollout groups

A **Rollout group** bundles related rollouts (e.g., a deploy + its associated flag enable) under one umbrella. Groups are optional — rollouts without a group behave exactly as before.

**Naming note:** "Rollout group" (not "Release") avoids collision with the existing `releases` concept in this codebase (which tracks version/commit metadata).

### Why group rollouts?

For UI: a single bundle view for "v1.2 shipped these 3 rollouts" instead of scattered entries.

For coordination: set the group's `coordination_policy` so that if any member rollout rolls back, siblings react:

| Policy | Effect on siblings |
|---|---|
| `independent` *(default)* | No effect. Siblings keep running. |
| `pause_on_sibling_abort` | Active siblings are paused. Operator decides next. |
| `cascade_abort` | Active/paused siblings are rolled back with reason `sibling_aborted:<id>`. |

### CLI

```
ds rollout-groups create --org acme --name "v1.2 ship" --policy pause_on_sibling_abort
ds rollout-groups attach <group-id> --org acme --rollout <rollout-id>
ds rollout-groups list --org acme
ds rollout-groups get <group-id> --org acme
```

### Attaching at rollout creation

Both deploy rollouts and config rollouts accept `release_id` in their attach request body — a group ID (column name preserved from Plan 2):

```json
{ "rollout": { "strategy_name": "prod-canary", "release_id": "<group-uuid>" } }
```

### Coordination limitations

- Coordination fires on `rollouts.rollout.rolled_back` NATS events. Rollbacks that happened before Plan 4 shipped are not replayed.
- Sibling rollback via `cascade_abort` calls `RolloutService.Rollback` which updates state but does NOT trigger applicator `Revert`. The engine performs Revert only on its own abort path; manually-triggered rollbacks (operator or cascade) leave traffic/rule unchanged in place. A follow-up should wire manual rollback to also call Revert.
