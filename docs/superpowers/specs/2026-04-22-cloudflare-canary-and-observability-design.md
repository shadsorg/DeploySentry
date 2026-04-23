# Cloudflare Traffic-Shift Canaries & Provider-Agnostic Observability

**Date:** 2026-04-22
**Status:** Design (implementation deferred)

## Overview

DeploySentry's rollout engine currently supports flag-based progressive rollouts and a sidecar/Envoy traffic-management design (see `2026-04-17-sidecar-traffic-management-design.md`). Neither covers the case where the cloud provider is a PaaS (Railway, Render, Fly, etc.) that does not expose native percentage-based traffic splitting between deployment revisions, and where flag-based rollouts are insufficient because the *binary itself* is the risk (runtime upgrade, framework bump, dep swap, full rewrite).

This design introduces two related capabilities:

1. **Edge traffic-shift canaries** — DeploySentry drives a weighted router at the edge (Cloudflare Load Balancer first, provider-agnostic interface) to split live traffic between two PaaS service slots (`stable` and `canary`) running different revisions of the same app. This is an *additional* rollout strategy — `traffic_shift` — not a replacement for flag-based rollouts.

2. **Provider-agnostic observability signals** — A generic abort-signal interface that lets rollout abort conditions consume telemetry from any observability backend (New Relic, Datadog, Grafana Cloud, CloudWatch, self-hosted Prometheus/Grafana/Loki/Tempo). Both **push** (backend alert → DeploySentry webhook) and **pull** (DeploySentry queries the backend) paths are supported, with the same semantics regardless of vendor.

Implementation is **explicitly deferred** — ship when the first concrete customer need arises. The spec exists to lock the architecture before it's built, so contributors know where these capabilities slot into the existing rollout engine.

## Goals

- Support true binary-level progressive rollouts on PaaS providers that don't expose weighted traffic routing natively.
- Keep flag-based rollouts as the default and cheapest path; `traffic_shift` is the escalation strategy.
- Define one abort-signal interface that works across SaaS and self-hosted observability stacks, so a rollout's abort conditions don't know or care which backend produced the signal.
- Prefer push (webhook) where latency matters, pull (query) where canary-vs-stable comparison is the signal.
- Fit both capabilities into the existing `internal/rollout/engine/` phase engine and `abort_conditions` model — no forked rollout path.

## Non-Goals

- **Native PaaS traffic splitting** — we are not asking Railway/Render/Fly to expose new capabilities. DeploySentry drives an external router (Cloudflare LB) that sits in front of the provider.
- **Building an observability backend** — DeploySentry does not store metrics/traces/logs. It only consumes signals from existing backends.
- **Automatic threshold tuning** — thresholds are user-defined per strategy/app, with sane defaults. No ML-driven auto-tuning.
- **Migrations off existing sidecar design** — the Envoy sidecar (`2026-04-17-sidecar-traffic-management-design.md`) stays as the self-hosted / in-VPC option. The Cloudflare path covers PaaS where a sidecar is awkward or impossible.
- **Multi-region weighted routing** — scope is single-region canary-vs-stable. Multi-region traffic management is a separate concern.

## Part 1 — Cloudflare (and friends) Traffic-Shift Canaries

### Topology

Per app per environment, two long-running PaaS services:

- `app-<env>-stable` — currently-promoted revision
- `app-<env>-canary` — rollout target

Both are registered as **origin pools** in a Cloudflare Load Balancer sitting in front of the public hostname. The LB health-checks each pool independently and steers traffic by configured weights.

```
         ┌──────────────────────────┐
DNS ───▶ │  Cloudflare Load Balancer│
         │  (weighted steering)     │
         └──────┬─────────────┬─────┘
                │ 95%         │ 5%
                ▼             ▼
         ┌────────────┐ ┌────────────┐
         │ Railway    │ │ Railway    │
         │ app-stable │ │ app-canary │
         └────────────┘ └────────────┘
```

After a successful rollout to 100%, the **stable/canary pointer swaps** — the next deploy targets the previous-stable slot, which becomes the new canary. This is the classic blue/green pointer flip, applied per app.

### Provider Abstraction

A new traffic-router provider interface lives alongside existing deploy providers:

```go
// internal/rollout/routers/router.go
type TrafficRouter interface {
    // SetWeights pushes new pool weights atomically.
    // The implementation decides how to achieve that
    // (Cloudflare: single LB update; nginx: reload).
    SetWeights(ctx context.Context, lb LBRef, weights map[PoolID]int) error

    // GetWeights reads current observed weights — used for
    // reconciliation and drift detection.
    GetWeights(ctx context.Context, lb LBRef) (map[PoolID]int, error)

    // PoolHealth returns the router's view of each pool's health.
    // Used as a gating signal before advancing weight.
    PoolHealth(ctx context.Context, lb LBRef) (map[PoolID]PoolHealth, error)
}
```

First implementation: `CloudflareRouter` (NerdGraph-style REST against `api.cloudflare.com/client/v4/accounts/:id/load_balancers/...`). Future implementations slot in without changing the rollout engine:

- `NginxRouter` — writes upstream weights, triggers reload (useful for self-hosted).
- `EnvoyRouter` — existing sidecar xDS push (bridges the two designs).
- `AWSALBRouter` — weighted target groups.
- `GCPLBRouter` — URL map weighted backend services.

### New Rollout Strategy: `traffic_shift`

Added to the polymorphic strategy catalog from `2026-04-18-configurable-rollout-strategies-design.md`. Interpretation per target type:

| target_type | Existing strategies interpret `percent` as… | `traffic_shift` interprets `percent` as… |
|---|---|---|
| `deploy` | Instance replacement progression | Edge weight routed to canary pool |
| `config` | SDK evaluation rollout % | N/A (strategy only applies to `deploy`) |

Applicator mapping: at each step, the rollout engine calls `TrafficRouter.SetWeights(lb, {stable: 100-p, canary: p})` and then evaluates standard dwell / health / abort_conditions before promotion.

### Config Surface

```yaml
# App-level config (additive to existing app config)
traffic_router:
  provider: cloudflare
  account_id: <cloudflare_account_id>
  zone_id: <cloudflare_zone_id>
  load_balancer_id: <lb_id>
  pools:
    stable:   { pool_id: <cf_pool_id_a>, railway_service: app-prod-blue  }
    canary:   { pool_id: <cf_pool_id_b>, railway_service: app-prod-green }
  session_affinity: none | cookie | ip   # default none; canary may prefer cookie
  api_token_secret_ref: secrets://cloudflare/api_token
```

Secrets are stored in the existing settings/secrets layer, not in the config row.

### Stable/Canary Swap Semantics

- After a rollout reaches 100% canary and bakes successfully, DeploySentry writes a **pointer swap**: next deploy's `canary` slot = current `stable` slot, and vice-versa.
- The two Railway services themselves never move — only which one is labeled canary.
- Rollback pre-swap: set canary weight to 0 (and optionally redeploy the last-known-good image to canary for forensic parity).
- Rollback post-swap (i.e. after full promotion, new problem observed): treated as a new rollout targeting the other slot, not a magic "undo." Keeps the state machine honest.

### Data Model Additions

New tables in the `deploy` schema (names reflect our convention of not prefixing with `deploy.` in migrations):

- `traffic_routers(id, app_id, environment_id, provider, config_json, active, created_at, updated_at)`
- `traffic_pools(id, router_id, slot ENUM('stable','canary'), external_pool_id, backend_ref, created_at, updated_at)`
- `traffic_rollout_state(rollout_id, router_id, current_weights_json, last_applied_at, last_observed_at)`

No changes to the existing `rollouts` table — the `traffic_shift` strategy reads/writes through `traffic_rollout_state` for its target-specific state.

### Tradeoffs & Constraints (explicit)

- **Always-on dual services.** Cost floor = 2× service cost per app×environment. Recommendation: opt-in per app, default off.
- **Cloudflare LB is a paid add-on.** Weighted steering + active health checks require a plan tier above free. Confirm per-customer before proposing this path.
- **Migration discipline required.** Both versions run simultaneously during a rollout; schema changes must be backward-compatible (expand/contract). This is table stakes for any real canary system and should be enforced in the deployment preflight (static check on pending migrations).
- **Session affinity is a policy choice.** Default `none` — each request independently rolled. Customers with login state or WebSockets will want `cookie`. Document the implications (canary "stuck" users take longer to drain on rollback).
- **Observability parity is mandatory.** Canary and stable must emit identical telemetry schema, differentiated by a `deploy_slot` label — otherwise abort conditions can't compare them (Part 2).

## Part 2 — Provider-Agnostic Observability Signals

### Model

All abort-signal sources reduce to one of two shapes:

- **Push:** the observability backend fires when its own alerting thresholds are crossed. DeploySentry exposes a webhook endpoint that, on receipt, aborts (or pauses) the active rollout for the identified app/environment. The backend owns the math.
- **Pull:** DeploySentry periodically queries the backend during a rollout, applies rollout-specific thresholds (usually stricter or comparative: canary vs stable), and decides whether to abort. DeploySentry owns the math.

Both paths feed the same `abort_conditions` evaluator on a rollout. From the engine's perspective, every signal is:

```go
type AbortSignal struct {
    Source       string    // "newrelic", "datadog", "prometheus", "webhook:<name>"
    RolloutID    string
    StepID       string
    Metric       string    // "error_rate", "p95_latency_ms", "apdex", ...
    Value        float64
    Comparison   *Comparison // nil if absolute signal (webhook)
    ObservedAt   time.Time
    Severity     Severity  // warn | abort
}
```

Abort_conditions compose signals as they already do — the source is opaque.

### Pluggable Provider Interface

```go
// internal/observability/provider.go
type ObservabilityProvider interface {
    // Identifier used in config and audit logs.
    Name() string

    // Query runs a provider-specific query spec and returns a numeric
    // value. Query specs are provider-typed (NRQL for NR, PromQL for
    // Prometheus, etc.) — DeploySentry stores the string, the provider
    // parses and executes.
    Query(ctx context.Context, q QuerySpec) (QueryResult, error)

    // PostDeploymentMarker annotates the provider's UI with a rollout
    // step event, so operators see overlays on their dashboards.
    PostDeploymentMarker(ctx context.Context, m DeploymentMarker) error
}
```

First-class providers (implementation order when the feature is built):

1. **Prometheus / Grafana Cloud** — PromQL; self-hosted or managed; covers the open-source default.
2. **New Relic** — NerdGraph + NRQL.
3. **Datadog** — metrics query API.
4. **CloudWatch** — metrics GetMetricData.
5. **Generic HTTP JSON** — escape hatch for anything custom; config gives a URL template and a JSONPath to extract the value.

Each provider's config lives per-org with optional per-app override:

```yaml
observability:
  - name: prod-prom
    provider: prometheus
    url: https://prom.internal/api/v1
    auth_secret_ref: secrets://prom/basic_auth
  - name: prod-nr
    provider: newrelic
    account_id: "123456"
    api_key_secret_ref: secrets://newrelic/user_key
  - name: prod-dd
    provider: datadog
    site: datadoghq.com
    api_key_secret_ref: secrets://datadog/api_key
    app_key_secret_ref: secrets://datadog/app_key
```

### Pull Path: Rollout Queries Provider

Abort condition references a provider and a query:

```yaml
abort_conditions:
  - name: canary-error-rate-vs-stable
    provider: prod-prom
    query: |
      (
        sum(rate(http_requests_total{app="checkout",deploy_slot="canary",status=~"5.."}[2m]))
        /
        sum(rate(http_requests_total{app="checkout",deploy_slot="canary"}[2m]))
      )
      -
      (
        sum(rate(http_requests_total{app="checkout",deploy_slot="stable",status=~"5.."}[2m]))
        /
        sum(rate(http_requests_total{app="checkout",deploy_slot="stable"}[2m]))
      )
    operator: ">"
    threshold: 0.005        # canary error rate > stable + 0.5%
    evaluation_window_s: 120
    consecutive_failures: 3 # require 3 breaches before abort
    severity: abort
```

Canary-vs-stable deltas require the `deploy_slot` label (or equivalent) to be present on telemetry — see the Observability Standard below.

### Push Path: Provider Alerts → DeploySentry Webhook

Endpoint (new):

```
POST /api/v1/webhooks/observability/:provider
Headers:
  X-DeploySentry-Signature: sha256=<hmac>
  X-DeploySentry-Rollout-Id: <rollout_id>  # OR resolvable from payload
```

- HMAC secret is per-webhook, rotated via the existing API keys / secrets surface.
- Payload schema is provider-specific (we don't try to normalize at receive time); a provider adapter maps it to an `AbortSignal` and delivers it to the active rollout for the named app/environment.
- If no active rollout, the signal is logged and ignored (noisy alerts during steady state shouldn't cause false actions).
- Idempotency key per signal prevents duplicate aborts from alert re-fires.

### Deployment Markers (bi-directional context)

On every rollout step transition, DeploySentry calls `PostDeploymentMarker` on all configured providers for that app. Benefits:

- Provider dashboards overlay rollout events on their charts automatically — operators don't have to cross-reference timestamps.
- Post-incident analysis gets rollout boundaries for free in the observability UI.
- Works even for providers we don't *query* (e.g. a customer uses Datadog for dashboards only, Prometheus for abort gating) — markers are cheap one-way pushes.

Providers without marker APIs fall back to no-op; the engine doesn't fail on marker errors.

### Self-Hosted Specifics (Grafana / Prometheus / Loki / Tempo)

- **Prometheus:** PromQL over `/api/v1/query`. Auth: basic / bearer / mTLS — handled by the generic HTTP client in the provider. No push equivalent; use Alertmanager webhook (below).
- **Alertmanager:** first-class push receiver. Alertmanager's webhook payload is well-defined; the `alertmanager` provider adapter maps it directly. Customers can point existing Alertmanager routes at DeploySentry without rewriting alert rules.
- **Grafana:** two touchpoints — (1) Grafana Alerting webhook → push path, (2) Grafana data source proxy → *not* used; we query the underlying source directly (Prometheus, Loki) to avoid coupling to Grafana's UI layer.
- **Loki:** LogQL over `/loki/api/v1/query_range` for log-rate abort conditions ("error log lines per minute on canary"). Wire it in as a distinct provider even though it's Grafana Labs — it's a different query language and different metric shape.
- **Tempo:** out of scope for abort signals (trace data is too sparse for rate-based gating). May be used for marker context in the future.

### Observability Standard (mandatory labels/tags)

For canary-vs-stable comparison to work on *any* provider, every deployed service must emit:

| Label/tag | Required values | Purpose |
|---|---|---|
| `app` | app slug | identifies the service |
| `environment` | env slug | separates prod / staging / etc |
| `deploy_slot` | `stable` \| `canary` | enables weighted-rollout comparisons |
| `version` | semver or commit SHA | post-hoc correlation |

DeploySentry injects these as environment variables on the deployed container (`DS_APP`, `DS_ENVIRONMENT`, `DS_DEPLOY_SLOT`, `DS_VERSION`). SDK wrappers and auto-instrumentation adapters are expected to read these and attach them to emitted telemetry. A preflight check (part of deployment onboarding) verifies that at least one sample with the required labels lands in the configured provider before the first `traffic_shift` rollout is allowed.

### Threshold Ownership

- **Steady-state SLO thresholds** live in the observability provider (alert policies). They exist whether or not a rollout is in progress, and fire into DeploySentry via the push path → hard abort.
- **Rollout-specific thresholds** live in DeploySentry strategy definitions (pull path). These are stricter / comparative / short-window. They only evaluate while a rollout is in progress.

Dual-signal policy: a rollout with `require_dual_signal: true` will only abort on (a) a provider webhook alert OR (b) `N` consecutive failed pull evaluations. Single-datapoint flaps don't trigger aborts.

## Rollout of the Design (if/when we implement)

Suggested phasing, each independently shippable:

1. **Observability pull path + providers (Prometheus, New Relic).** Adds `ObservabilityProvider` interface, two implementations, `abort_conditions` can reference a provider+query. Zero UI changes beyond strategy editor fields — orthogonal to traffic shift.
2. **Observability push path + deployment markers.** Webhook endpoint, provider adapters for Alertmanager + New Relic. Markers for all configured providers. Still orthogonal.
3. **Datadog, CloudWatch, Loki, generic-HTTP providers.** Additive.
4. **TrafficRouter interface + CloudflareRouter.** New tables, new provider, weight push/pull, pointer-swap logic. No rollout strategy yet — infra only.
5. **`traffic_shift` rollout strategy.** Wires the router into the existing rollout engine. Gated behind a per-app opt-in flag.
6. **Preflight checks** — observability label presence, migration compatibility, dual-service health — blocking the first `traffic_shift` rollout until satisfied.

Each phase is shippable on its own and leaves the system in a useful state. Phases 1–3 are valuable even if `traffic_shift` is never built.

## Open Questions (defer-and-return)

- **Canary traffic origin:** should we prefer cookie-based affinity or pure random per-request on Cloudflare by default? Affects user experience during rollback; tradeoff depends on app type. Defer the default until we have a second customer's use case.
- **Provider marker verbosity:** every step, or only promotion/abort? Fine-grained markers are great for forensics, noisy for dashboards. Recommend configurable per-provider, default to promotion/abort only.
- **Webhook-to-rollout resolution:** trust the `X-DeploySentry-Rollout-Id` header, or always resolve via `{app, environment}` on the payload? Header is simpler but requires the alerting backend to template it in. Supporting both is low effort and low risk.
- **Cost modeling surface:** should DeploySentry expose "you are paying $X/mo extra for canary capacity on this app" once `traffic_shift` is active? Useful nudge, but out of initial scope.

## Related Designs

- `2026-04-17-sidecar-traffic-management-design.md` — Envoy sidecar path for self-hosted / in-VPC traffic splitting. Same `TrafficRouter` interface should cover both; the sidecar is one more implementation.
- `2026-04-18-configurable-rollout-strategies-design.md` — polymorphic strategy model this spec extends with `traffic_shift`.
- `Deploy_Monitoring_Setup.md` — current (pre-provider-agnostic) health monitoring. The observability layer in this spec generalizes that into pluggable sources.

## Status & Next Actions

- **Implementation deferred.** No scheduled work. Revisit when a real customer need pushes this past flag-based rollouts.
- **Spec review:** open for comment; merge as Design. Flip to Implementation only when a plan file is created under `docs/superpowers/plans/`.
