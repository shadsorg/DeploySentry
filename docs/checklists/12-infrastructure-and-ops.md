# 12 — Infrastructure & Operations

## Kubernetes Deployment

### Base Resources (`deploy/kubernetes/base/`)
- [ ] API server Deployment manifest
- [ ] API server Service (ClusterIP)
- [ ] Ingress resource (L7 load balancer)
- [ ] Horizontal Pod Autoscaler (HPA) based on CPU/memory and custom metrics
- [ ] Pod Disruption Budgets (PDB) for safe rollouts
- [ ] Resource requests and limits per pod
- [ ] Readiness probes (HTTP health check)
- [ ] Liveness probes (HTTP health check)
- [ ] ConfigMaps for application config
- [ ] Secrets for sensitive config
- [ ] Service accounts with RBAC
- [ ] Network policies for pod-to-pod communication

### Environment Overlays

#### Dev (`deploy/kubernetes/overlays/dev/`)
- [ ] Dev-specific resource limits (lower)
- [ ] Dev environment variables
- [ ] Auto-deploy on merge to `main`
- [ ] Synthetic/seeded test data

#### Staging (`deploy/kubernetes/overlays/staging/`)
- [ ] Staging-specific resource limits
- [ ] Staging environment variables
- [ ] Anonymized prod data mirror
- [ ] Auto-deploy after dev health check

#### Prod (`deploy/kubernetes/overlays/prod/`)
- [ ] Production resource limits (higher)
- [ ] Production environment variables
- [ ] Canary deployment with manual promotion gate
- [ ] Multi-replica configuration
- [ ] Pod anti-affinity rules

### Service Mesh (Optional)
- [ ] mTLS between services (sidecar proxy)
- [ ] Service-to-service observability
- [ ] Traffic management policies

## Docker Images

### API Server (`deploy/docker/Dockerfile.api`)
- [ ] Multi-stage build
- [ ] Stage 1: Build Go binary
- [ ] Stage 2: Minimal runtime image (distroless/alpine)
- [ ] Non-root user
- [ ] Health check instruction

### Web UI (`deploy/docker/Dockerfile.web`)
- [ ] Multi-stage build
- [ ] Stage 1: Build React static bundle
- [ ] Stage 2: Nginx serving static files
- [ ] Nginx configuration for SPA routing

## Docker Compose (`deploy/docker-compose.yml`)
- [ ] PostgreSQL 16 with initialization scripts
- [ ] Redis 7
- [ ] NATS JetStream
- [ ] API server (optional, for full-stack local dev)
- [ ] Web UI dev server (optional)
- [ ] Volume mounts for data persistence
- [ ] Health checks for all services

## Observability Stack

### OpenTelemetry Integration
- [ ] OpenTelemetry SDK instrumentation in Go services
  - [ ] Traces (HTTP handlers, DB queries, external calls)
  - [ ] Metrics (request count, latency, error rate)
  - [ ] Logs (structured JSON logging)
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
- [ ] Structured JSON logging format
- [ ] Loki integration for log aggregation
- [ ] Log correlation with trace IDs
- [ ] Log level configuration

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
