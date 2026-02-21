# 12 — Infrastructure & Operations

## Kubernetes Deployment

### Base Resources (`deploy/kubernetes/base/`)
- [x] API server Deployment manifest
- [x] API server Service (ClusterIP)
- [x] Ingress resource (L7 load balancer)
- [x] Horizontal Pod Autoscaler (HPA) based on CPU/memory and custom metrics
- [x] Pod Disruption Budgets (PDB) for safe rollouts
- [x] Resource requests and limits per pod
- [x] Readiness probes (HTTP health check)
- [x] Liveness probes (HTTP health check)
- [x] ConfigMaps for application config
- [x] Secrets for sensitive config
- [ ] Service accounts with RBAC
- [ ] Network policies for pod-to-pod communication

### Environment Overlays

#### Dev (`deploy/kubernetes/overlays/dev/`)
- [x] Dev-specific resource limits (lower)
- [x] Dev environment variables
- [ ] Auto-deploy on merge to `main`
- [ ] Synthetic/seeded test data

#### Staging (`deploy/kubernetes/overlays/staging/`)
- [x] Staging-specific resource limits
- [x] Staging environment variables
- [ ] Anonymized prod data mirror
- [ ] Auto-deploy after dev health check

#### Prod (`deploy/kubernetes/overlays/prod/`)
- [x] Production resource limits (higher)
- [x] Production environment variables
- [ ] Canary deployment with manual promotion gate
- [x] Multi-replica configuration
- [x] Pod anti-affinity rules

### Service Mesh (Optional)
- [ ] mTLS between services (sidecar proxy)
- [ ] Service-to-service observability
- [ ] Traffic management policies

## Docker Images

### API Server (`deploy/docker/Dockerfile.api`)
- [x] Multi-stage build
- [x] Stage 1: Build Go binary
- [x] Stage 2: Minimal runtime image (distroless/alpine)
- [x] Non-root user
- [ ] Health check instruction

### Web UI (`deploy/docker/Dockerfile.web`)
- [x] Multi-stage build
- [x] Stage 1: Build React static bundle
- [x] Stage 2: Nginx serving static files
- [x] Nginx configuration for SPA routing

## Docker Compose (`deploy/docker-compose.yml`)
- [x] PostgreSQL 16 with initialization scripts
- [x] Redis 7
- [x] NATS JetStream
- [ ] API server (optional, for full-stack local dev)
- [ ] Web UI dev server (optional)
- [x] Volume mounts for data persistence
- [x] Health checks for all services

## Observability Stack

### OpenTelemetry Integration
- [ ] OpenTelemetry SDK instrumentation in Go services
  - [ ] Traces (HTTP handlers, DB queries, external calls)
  - [ ] Metrics (request count, latency, error rate)
  - [x] Logs (structured JSON logging)
- [ ] OTel Collector deployment and configuration
- [ ] Trace propagation (W3C Trace Context)

### Prometheus
- [ ] Prometheus deployment/configuration
- [ ] Scrape configuration for Go services
- [ ] Custom metrics:
  - [ ] Flag evaluations/sec
  - [ ] Deployment duration
  - [ ] Health score distribution
  - [ ] API latency histograms
- [ ] Alert rules for SLO violations

### Grafana Dashboards
- [ ] Deployment overview dashboard
- [ ] Feature flag usage dashboard
- [ ] Release health dashboard
- [ ] System health dashboard (API latency, error rates, resource utilization)
- [ ] Alert notification channels

### Logging
- [x] Structured JSON logging format
- [ ] Loki integration for log aggregation
- [ ] Log correlation with trace IDs
- [x] Log level configuration

### Tracing
- [ ] Jaeger or Tempo for distributed tracing
- [ ] Trace visualization
- [ ] Span tagging for deployment/flag context

## Security Infrastructure
- [ ] CloudFlare CDN / WAF configuration
- [ ] TLS certificate management (cert-manager)
- [ ] Network policies in Kubernetes
- [ ] Secrets management (Vault / AWS Secrets Manager)
- [ ] Data encryption at rest (AES-256)
- [ ] mTLS between internal services
- [ ] Security scanning in CI (container image scanning)

## Data Management
- [ ] Database backup strategy (automated daily backups)
- [ ] Backup storage in S3-compatible object storage
- [ ] Data retention policies:
  - [ ] Audit logs: 2 years (archive to S3 after 90 days)
  - [ ] Flag evaluation logs: 30 days (sample at 10% after 7 days)
  - [ ] Deployment history: indefinite
  - [ ] Webhook delivery logs: 30 days
  - [ ] Metrics: 90 days (downsample after 30 days)
- [ ] Data migration tooling
- [ ] Disaster recovery procedures
