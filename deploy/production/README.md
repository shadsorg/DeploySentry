# Production Deployment Guide

This guide covers deploying DeploySentry to production with proper security, monitoring, and reliability.

## Infrastructure Requirements

### Core Services
- **PostgreSQL 15+**: Primary database with `deploy` schema
- **Redis 7+**: Caching and rate limiting
- **NATS 2.9+**: Message queue for real-time events

### Monitoring Stack
- **Prometheus**: Metrics collection (scrapes `/metrics`)
- **Grafana**: Dashboards and alerting
- **Log aggregation**: ELK stack or similar for centralized logs

### Load Balancing
- **Reverse Proxy**: nginx, Cloudflare, or AWS ALB
- **SSL/TLS**: Required for production (HSTS enabled)

## Security Configuration

### Environment Variables

#### Required Security Settings
```bash
# Production Environment
DS_ENVIRONMENT=production

# JWT Configuration
DS_JWT_SECRET=<256-bit-random-key>  # openssl rand -base64 32
DS_JWT_EXPIRY=24h

# Database (use connection pooling)
DS_DATABASE_URL=postgres://user:pass@host:5432/deploysentry?search_path=deploy&sslmode=require
DS_DATABASE_MAX_CONNECTIONS=25
DS_DATABASE_MAX_IDLE=10

# Redis (use TLS in production)
DS_REDIS_URL=redis://user:pass@host:6379/0
DS_REDIS_TLS=true

# NATS (use TLS and auth)
DS_NATS_URL=nats://user:pass@host:4222
DS_NATS_TLS=true

# Server Configuration
DS_SERVER_HOST=0.0.0.0
DS_SERVER_PORT=8080
DS_SERVER_READ_TIMEOUT=30s
DS_SERVER_WRITE_TIMEOUT=30s
DS_SERVER_SHUTDOWN_TIMEOUT=10s

# Logging
DS_LOG_LEVEL=info  # info, warn, error for production
```

#### Optional Security Enhancements
```bash
# Rate Limiting (adjust based on traffic)
DS_RATE_LIMIT_REQUESTS=100
DS_RATE_LIMIT_WINDOW=1m

# Request Size Limits
DS_MAX_REQUEST_SIZE=32MB

# CORS (replace with actual frontend domains)
DS_CORS_ORIGINS="https://app.deploysentry.com,https://dashboard.deploysentry.com"

# Webhook Security
DS_WEBHOOK_SECRET=<256-bit-random-key>
DS_WEBHOOK_TIMEOUT=30s
```

### Network Security
1. **Firewall**: Only allow necessary ports (80, 443, health check)
2. **VPC/Private Networks**: Keep databases in private subnets
3. **Security Groups**: Restrict database access to application servers only
4. **WAF**: Consider Web Application Firewall for additional protection

## Health Checks

### Kubernetes Liveness Probe
```yaml
livenessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 30
  timeoutSeconds: 5
  failureThreshold: 3
```

### Kubernetes Readiness Probe
```yaml
readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
  timeoutSeconds: 3
  failureThreshold: 2
```

### Load Balancer Health Check
- **Endpoint**: `GET /ready`
- **Expected**: 200 OK
- **Interval**: 30s
- **Timeout**: 5s

## Monitoring & Alerting

### Key Metrics to Monitor
```
# Request metrics
http_requests_total
http_request_duration_seconds
http_active_connections

# Application metrics
flag_evaluations_total
deployment_events_total
database_connections
redis_operations_total

# System metrics
go_memstats_alloc_bytes
go_goroutines
process_cpu_seconds_total
```

### Critical Alerts
1. **High Error Rate**: >5% 5xx responses in 5m
2. **High Latency**: P99 > 2s for 5m
3. **Database Issues**: Connection errors or high latency
4. **Memory Usage**: >85% of available memory
5. **Service Down**: Health check failures

### Example Grafana Dashboard
```json
{
  "dashboard": {
    "title": "DeploySentry Production",
    "panels": [
      {
        "title": "Request Rate",
        "targets": [
          "rate(http_requests_total[5m])"
        ]
      },
      {
        "title": "Error Rate",
        "targets": [
          "rate(http_requests_total{status=~\"5..\"}[5m]) / rate(http_requests_total[5m])"
        ]
      }
    ]
  }
}
```

## Backup Strategy

### Database Backups
```bash
# Daily automated backups
pg_dump -h $DB_HOST -U $DB_USER -d deploysentry \
  --schema=deploy --clean --if-exists \
  > backup_$(date +%Y%m%d).sql

# Retention: 7 daily, 4 weekly, 12 monthly
```

### Configuration Backups
- Environment variables and secrets
- Infrastructure as Code (Terraform, CloudFormation)
- Kubernetes manifests

## Deployment Checklist

### Pre-deployment
- [ ] Environment variables configured
- [ ] Database migrations tested
- [ ] Health checks configured
- [ ] Monitoring alerts configured
- [ ] Backup strategy implemented
- [ ] SSL certificates installed
- [ ] Security scan completed

### Deployment Process
- [ ] Database migration (if needed)
- [ ] Rolling deployment (zero downtime)
- [ ] Health check verification
- [ ] Smoke tests passed
- [ ] Metrics collecting properly
- [ ] Alerts functioning

### Post-deployment
- [ ] Monitor error rates and latency
- [ ] Verify all services connected
- [ ] Test critical user flows
- [ ] Update runbooks if needed

## Performance Tuning

### Database Optimization
```sql
-- Connection settings
max_connections = 100
shared_buffers = 256MB
effective_cache_size = 1GB
work_mem = 4MB

-- Query optimization
log_min_duration_statement = 1000  -- Log slow queries
```

### Application Scaling
- **Horizontal**: Multiple API server instances behind load balancer
- **Vertical**: Adjust CPU/memory based on metrics
- **Auto-scaling**: Based on CPU, memory, or request rate

### Caching Strategy
- **Redis**: Feature flag cache, rate limiting, session data
- **CDN**: Static assets and API responses where appropriate
- **HTTP Caches**: Use cache headers for appropriate endpoints

## Security Hardening

### Container Security
```dockerfile
# Use non-root user
RUN addgroup -g 1001 deploysentry && \
    adduser -D -s /bin/sh -u 1001 -G deploysentry deploysentry
USER deploysentry

# Read-only root filesystem
docker run --read-only --tmpfs /tmp deploysentry
```

### Network Policies
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deploysentry-api
spec:
  podSelector:
    matchLabels:
      app: deploysentry-api
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: load-balancer
    ports:
    - protocol: TCP
      port: 8080
```

## Troubleshooting

### Common Issues
1. **Database Connection Errors**: Check connection string, network, and credentials
2. **High Memory Usage**: Check for memory leaks, adjust limits
3. **Slow Responses**: Check database query performance, Redis connectivity
4. **Rate Limiting**: Verify Redis connection and rate limit configuration

### Debug Commands
```bash
# Check health status
curl http://localhost:8080/health

# View metrics
curl http://localhost:8080/metrics

# Database connection test
psql $DS_DATABASE_URL -c "SELECT 1"

# Redis connection test
redis-cli -u $DS_REDIS_URL ping
```