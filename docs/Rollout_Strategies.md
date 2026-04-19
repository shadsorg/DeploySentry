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
