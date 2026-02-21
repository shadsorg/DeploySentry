# 13 — Testing Strategy & Quality

## Unit Tests (~60% of total tests)
- [x] Go `testing` + `testify` for backend services
- [ ] Business logic tests:
  - [x] Flag evaluation engine (all rule types, edge cases)
  - [x] Health score computation (weighted signals, thresholds)
  - [x] Deployment state machine transitions
  - [x] Rollback trigger evaluation
  - [x] Canary phase progression logic
  - [x] RBAC permission checks
  - [x] API key validation
- [x] Model validation tests
- [ ] Jest for React UI component tests

## Integration Tests (~30% of total tests)
- [ ] Go `testing` + `testcontainers` for service interactions
- [ ] Database query tests (PostgreSQL via testcontainers)
  - [ ] Migration up/down roundtrip
  - [ ] CRUD operations for all entities
  - [ ] Complex queries (filtering, pagination, joins)
- [ ] Redis cache tests
  - [ ] Cache set/get/invalidation
  - [ ] TTL behavior
  - [ ] Pub/sub for flag updates
- [ ] NATS messaging tests
  - [ ] Event publishing and subscription
  - [ ] At-least-once delivery verification
- [x] API handler tests (Go `httptest` + `testify`)
  - [ ] Request validation
  - [x] Auth middleware
  - [x] RBAC enforcement
  - [ ] Response format

## End-to-End Tests (~10% of total tests)
- [ ] Playwright for critical user flows:
  - [ ] Login via OAuth
  - [ ] Create project and invite member
  - [ ] Create and deploy a release
  - [ ] Create, configure, and toggle a feature flag
  - [ ] View deployment dashboard
  - [ ] Trigger manual rollback
- [ ] E2E environment setup (Docker Compose)

## SDK Tests
- [ ] Language-specific testing frameworks per SDK
- [ ] Evaluation logic correctness
- [ ] Caching behavior
- [ ] Streaming update handling
- [ ] Offline mode / graceful degradation
- [ ] Contract tests (Pact) for SDK ↔ API compatibility

## Load Tests
- [ ] k6 load testing scripts
- [ ] Targets:
  - [ ] Flag evaluation throughput: 10K evaluations/sec
  - [ ] API latency under load: < 10ms p99
  - [ ] Concurrent deployments: 100
  - [ ] Webhook delivery: 1K/min
- [ ] Load test CI integration (nightly runs)
- [ ] Performance regression detection

## CI Pipeline (`.github/workflows/ci.yml`)
- [x] Lint stage:
  - [x] golangci-lint for Go code
  - [ ] eslint + prettier for React UI
  - [ ] sqlfluff for SQL migrations
- [x] Test stage:
  - [x] Unit tests (parallel by package)
  - [x] Integration tests (with testcontainers)
  - [x] Code coverage reporting
- [x] Build stage:
  - [x] Go binaries (linux/amd64, darwin/amd64, darwin/arm64)
  - [x] Docker images
  - [ ] UI static bundle

## Release Pipeline (`.github/workflows/release.yml`)
- [x] Semantic version tagging
- [x] Changelog generation
- [x] Docker image build and push to registry
- [x] GitHub Release creation
- [x] Binary artifact uploads (CLI)

## Non-Functional Requirements Verification

### Performance Targets
- [ ] Flag evaluation (SDK, cached): < 1ms p99
- [ ] Flag evaluation (API): < 10ms p99
- [ ] Deployment creation API: < 200ms p99
- [ ] Dashboard page load: < 2s (LCP)
- [ ] Deployment status update: < 500ms (SSE/WebSocket)
- [ ] System availability: 99.9% uptime

### Scalability Targets (GA)
- [ ] Flag evaluations: 10K/sec
- [ ] Concurrent deployments: 100
- [ ] Feature flags per project: 1,000
- [ ] Projects per org: 50
- [ ] Users per org: 100
- [ ] Webhook deliveries: 1K/min

### Security Audit
- [ ] OWASP Top 10 vulnerability scan
- [ ] Dependency vulnerability scanning
- [ ] Container image scanning
- [ ] Penetration testing (pre-GA)
- [ ] Security documentation
