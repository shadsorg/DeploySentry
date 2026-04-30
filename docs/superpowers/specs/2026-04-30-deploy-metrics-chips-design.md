# Deploy Metrics Chips on Deployment List Rows (Future)

**Status**: Design (future — not scheduled)
**Date**: 2026-04-30
**Origin**: UI audit §4 (`deployment-history.html` mockup)

## Goal

Surface a "how is this deploy performing?" indicator next to each deployment row on `DeploymentsPage` and `OrgDeploymentsPage`. The mockup shows two chips per row: latency and error rate. Today neither is computed or stored anywhere queryable from the list endpoint.

## Data sources

Two cases by deploy type:

1. **Rollout-driven deploy** (deploy ran through a configurable rollout strategy with health gates) — metrics already collected during phase advancement to drive abort conditions. Source: `internal/rollout/applicator/health_*` paths (Prometheus / Datadog / Sentry / signal source). Snapshot the per-phase `HealthScore` at completion onto the deployment row.
2. **Non-rollout deploy** (legacy / record-mode / one-shot deploy with no health gates) — no metrics collected during the deploy itself. Approach:
   - Compute a **pre-deploy baseline** as the previous 24h average (per env, per app) of the same metrics from whichever `HealthConfig` source is wired.
   - Schedule three follow-up snapshots **at T+1h, T+4h, T+24h** after the deploy completes. Each snapshot records `(p95_latency, error_rate, sample_count)` plus the trend vs baseline.
   - The chip displays "n/a" until the first snapshot lands; "?" while waiting; and the most recent value with an arrow indicator (↗ / ↘) once snapshots arrive.

The UI placeholder until this ships: render `?` chips so the visual treatment is locked in but the wiring is no-op.

## Schema sketch

```
CREATE TABLE deployment_metrics (
  deployment_id  UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
  observed_at    TIMESTAMPTZ NOT NULL,
  source         TEXT NOT NULL,           -- 'rollout' | 't+1h' | 't+4h' | 't+24h' | 'baseline'
  p95_latency_ms NUMERIC,
  error_rate     NUMERIC,
  sample_count   BIGINT,
  PRIMARY KEY (deployment_id, observed_at, source)
);
```

The list endpoint joins to the **most recent** snapshot per deploy and includes `latest_metrics` + `baseline` on each row. Two computed deltas (`latency_delta_pct`, `error_rate_delta_pct`) drive chip color: green if better than baseline, neutral if within ±5%, warn if worse.

## Worker

A single ticker job runs every minute, reads `deployments` rows whose `completed_at IS NOT NULL` and have unsatisfied snapshot times, computes the relevant `(p95_latency, error_rate)` from the configured health source (reusing the existing `internal/health` integrations), and inserts a row.

Snapshots stop after T+24h. Operators can replay missed snapshots manually via a CLI subcommand `deploysentry deploy snapshot-metrics <id>`.

## API

- `GET /deployments?include=metrics` adds `latest_metrics` + `baseline` to each row.
- `GET /deployments/:id/metrics` returns the full snapshot timeline.

## UI

- `DeploymentsPage` and `OrgDeploymentsPage` rows render chips below the status pill: `↘ 12ms` / `⚠ 0.4%`. Color = delta-derived. Hover tooltip shows `vs 14ms baseline (-14%)`.
- Chips render `?` when the row has no snapshot yet but is in window; collapse entirely after T+24h if no data ever arrived (source not configured).

## Out of scope

- Per-region / per-instance breakdowns. Single org-level p95 + err rate.
- Alerting on the metrics. They're informational; rollout abort conditions are the authoritative gate.
- Backfilling historical deploys with snapshots.

## Done when

- Both list pages show metric chips with real data for new deploys.
- Pre-deploy baseline shows for non-rollout deploys when a health source is configured.
- Tooltip + delta colorization match mockup intent.
