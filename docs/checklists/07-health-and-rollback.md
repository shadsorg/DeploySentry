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
  - [ ] Error rate (5xx): 30% default weight
  - [ ] Latency p99: 20% default weight
  - [ ] Error tracking (new errors): 20% default weight
  - [ ] Custom metrics: 15% default weight
  - [ ] Synthetic checks: 15% default weight
- [x] Formula: `health_score = sum(signal_score_i * weight_i)`
- [ ] Signal score: `100 * max(0, 1 - (current_value / threshold_value))`
- [x] Configurable weights per project/environment
- [x] Configurable thresholds per signal
- [ ] Health score history tracking

### Integration Points (`integrations/`)

#### Prometheus Integration (`prometheus.go`)
- [x] PromQL query execution
- [x] Metric scraping for error rate, latency percentiles
- [x] Custom metric query support
- [x] Connection pooling and timeout handling

#### Sentry Integration (`sentry.go`)
- [ ] Webhook receiver for new issues
- [x] Error count per release API integration
- [x] New error detection for health scoring

#### Datadog Integration (`datadog.go`)
- [x] Datadog API client for metrics
- [ ] Monitor status integration
- [x] Custom metric queries

#### Custom Integration
- [ ] HTTP endpoint polling
- [ ] Configurable response parsing
- [ ] Health signal extraction from arbitrary endpoints

---

## Rollback Controller (`internal/rollback/`)

### Controller (`controller.go`)
- [x] Rollback state machine implementation:
  ```
  HEALTHY → EVALUATING → RECOVERED (if auto-heal)
  HEALTHY → EVALUATING → ROLLING_BACK → ROLLED_BACK (if threshold breached)
  ```
- [ ] Rollback trigger evaluation:
  - [ ] Error rate exceeds threshold (configurable, e.g., > 5% for 2 min)
  - [ ] Latency p99 exceeds threshold
  - [x] Health check failures
  - [x] Manual trigger via API/CLI/UI
  - [ ] External signal (PagerDuty incident auto-created)
- [ ] Rollback decision cooldown (prevent flapping)
- [x] Manual override capability

### Rollback Strategies (`strategies.go`)
- [x] Re-deploy previous known-good version
- [ ] Traffic shift back to blue environment (for blue/green)
- [ ] Feature flag kill switch (disable flag that gates new code)
- [ ] Strategy selection based on deployment type
- [ ] Rollback verification (ensure rolled-back version is healthy)

### Rollback API
- [ ] `POST /api/v1/deployments/:id/rollback` — Manual rollback trigger
- [ ] `GET /api/v1/rollbacks` — List rollback history
- [x] Rollback event emission for notifications
