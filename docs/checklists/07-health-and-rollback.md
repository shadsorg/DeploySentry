# 07 — Health Monitor & Rollback Controller

## Health Monitor (`internal/health/`)

### Core Monitor (`monitor.go`)
- [x] Health check orchestration for active deployments
- [x] Periodic health evaluation loop
- [x] Signal aggregation from multiple sources
- [x] Health status change event emission
- [x] Configurable evaluation windows

### Health Score Computation (`scorer.go`)
- [x] Composite health score (0–100) from weighted signals:
  - [x] Error rate (5xx): 30% default weight
  - [x] Latency p99: 20% default weight
  - [x] Error tracking (new errors): 20% default weight
  - [x] Custom metrics: 15% default weight
  - [x] Synthetic checks: 15% default weight
- [x] Formula: `health_score = sum(signal_score_i * weight_i)`
- [x] Signal score: `100 * max(0, 1 - (current_value / threshold_value))`
- [x] Configurable weights per project/environment
- [x] Configurable thresholds per signal
- [x] Health score history tracking

### Integration Points (`integrations/`)

#### Prometheus Integration (`prometheus.go`)
- [x] PromQL query execution
- [x] Metric scraping for error rate, latency percentiles
- [x] Custom metric query support
- [x] Connection pooling and timeout handling

#### Sentry Integration (`sentry.go`)
- [x] Webhook receiver for new issues
- [x] Error count per release API integration
- [x] New error detection for health scoring

#### Datadog Integration (`datadog.go`)
- [x] Datadog API client for metrics
- [x] Monitor status integration
- [x] Custom metric queries

#### Custom Integration
- [x] HTTP endpoint polling
- [x] Configurable response parsing
- [x] Health signal extraction from arbitrary endpoints

---

## Rollback Controller (`internal/rollback/`)

### Controller (`controller.go`)
- [x] Rollback state machine implementation:
  ```
  HEALTHY → EVALUATING → RECOVERED (if auto-heal)
  HEALTHY → EVALUATING → ROLLING_BACK → ROLLED_BACK (if threshold breached)
  ```
- [x] Rollback trigger evaluation:
  - [x] Error rate exceeds threshold (configurable, e.g., > 5% for 2 min)
  - [x] Latency p99 exceeds threshold
  - [x] Health check failures
  - [x] Manual trigger via API/CLI/UI
  - [ ] External signal (PagerDuty incident auto-created)
- [x] Rollback decision cooldown (prevent flapping)
- [x] Manual override capability

### Rollback Strategies (`strategies.go`)
- [x] Re-deploy previous known-good version
- [x] Traffic shift back to blue environment (for blue/green)
- [x] Feature flag kill switch (disable flag that gates new code)
- [x] Strategy selection based on deployment type
- [x] Rollback verification (ensure rolled-back version is healthy)

### Rollback API
- [x] `POST /api/v1/deployments/:id/rollback` — Manual rollback trigger
- [x] `GET /api/v1/rollbacks` — List rollback history
- [x] Rollback event emission for notifications
