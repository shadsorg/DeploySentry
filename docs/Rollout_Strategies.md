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
