# Production Deployment Guide

This guide covers deploying DeploySentry in production environments with proper security, monitoring, and reliability configurations.

## Quick Production Checklist

- [ ] Configure strong authentication secrets
- [ ] Enable database SSL/TLS
- [ ] Set up Redis authentication
- [ ] Configure CORS for your domains
- [ ] Enable production configuration validation
- [ ] Set up Prometheus monitoring
- [ ] Configure structured logging
- [ ] Apply database security migrations
- [ ] Test health checks and graceful shutdown

## Configuration

### Required Production Environment Variables

```bash
# Environment
export DS_ENVIRONMENT=production

# Authentication (CRITICAL - change these)
export DS_AUTH_JWT_SECRET="$(openssl rand -base64 64)"

# Database (enable SSL)
export DS_DATABASE_SSL_MODE=require
export DS_DATABASE_PASSWORD="strong-database-password"

# Redis (enable auth)
export DS_REDIS_PASSWORD="strong-redis-password"

# Server
export DS_SERVER_HOST=0.0.0.0
export DS_SERVER_READ_TIMEOUT=30s
export DS_SERVER_WRITE_TIMEOUT=30s
```

### Production Configuration Validation

DeploySentry automatically validates production configurations when `DS_ENVIRONMENT=production`:

- Ensures JWT secret is changed from defaults
- Requires database SSL/TLS
- Validates strong passwords
- Checks reasonable timeout values

## Security

### Database Namespace Isolation

All tables live in the `deploy` schema for shared database environments:

```sql
-- Create database users
CREATE USER deploysentry_owner WITH LOGIN PASSWORD 'strong-owner-password';
CREATE USER deploysentry_app WITH LOGIN PASSWORD 'strong-app-password';

-- Grant schema permissions
GRANT CREATE ON DATABASE production_db TO deploysentry_owner;
GRANT USAGE ON SCHEMA deploy TO deploysentry_app;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA deploy TO deploysentry_app;
```

Apply security migrations:
```bash
# Migrations 025 and 026 add schema security and prevent naming conflicts
make migrate-up
```

### Security Headers

Production deployment includes security headers middleware:

- **Content Security Policy**: Prevents XSS attacks
- **X-Frame-Options**: Prevents clickjacking
- **Strict-Transport-Security**: Enforces HTTPS
- **X-Content-Type-Options**: Prevents MIME confusion
- **Referrer-Policy**: Controls referrer information

Headers are configured via `middleware.SecurityHeaders()` with sensible defaults.

### CORS Configuration

Configure allowed origins for production:

```go
corsConfig := middleware.ProductionCORSConfig([]string{
    "https://deploysentry.yourdomain.com",
    "https://admin.yourdomain.com",
})
```

## Monitoring & Observability

### Prometheus Metrics

Metrics endpoint available at `/metrics`:

| Metric | Type | Description |
|--------|------|-------------|
| `http_request_duration_seconds` | Histogram | Request latency by method/path/status |
| `http_requests_total` | Counter | Total requests by method/path/status |
| `http_active_connections` | Gauge | Current active connections |
| `flag_evaluations_total` | Counter | Flag evaluations by project/key |
| `deployment_events_total` | Counter | Deployment state changes |
| `database_connections` | Gauge | Database connection pool status |
| `redis_operations_total` | Counter | Redis operations by command |

### Health Checks

| Endpoint | Purpose | Response Time |
|----------|---------|---------------|
| `/health` | Full health check with dependencies | ~3s |
| `/ready` | Lightweight readiness probe | ~1ms |

Health checks validate:
- Database connectivity and schema
- Redis connectivity and auth
- NATS messaging system

### Request Tracing

Every request gets a unique `X-Request-ID` header for distributed tracing:

```bash
curl -H "X-Request-ID: custom-trace-id" https://api.deploysentry.com/health
# Response includes: X-Request-ID: custom-trace-id
```

### Structured Logging

Configure JSON logging for production:

```bash
export DS_LOG_LEVEL=info
export DS_LOG_FORMAT=json
```

All logs include request IDs for correlation.

## Reliability

### Connection Pooling

Database connection pool configured for production load:

```bash
export DS_DATABASE_MAX_OPEN_CONNS=25
export DS_DATABASE_MAX_IDLE_CONNS=10
export DS_DATABASE_CONN_MAX_LIFETIME=5m
```

### Rate Limiting

Redis-backed sliding window rate limiting:

- **Default**: 100 requests per minute per IP/API key
- **Headers**: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Window`
- **Response**: HTTP 429 with `retry_after` when exceeded

### Graceful Shutdown

Server handles SIGINT/SIGTERM with configurable shutdown timeout:

```bash
export DS_SERVER_SHUTDOWN_TIMEOUT=30s
```

Graceful shutdown sequence:
1. Stop accepting new connections
2. Finish processing active requests
3. Close database/Redis/NATS connections
4. Exit cleanly or force-close after timeout

### Circuit Breaking

External health checks (Prometheus, Datadog) include failure handling:

- **Timeouts**: Configurable per integration
- **Fallbacks**: Assume healthy on integration failures
- **Retries**: Exponential backoff for NATS reconnection

## Deployment

### Container Health Check

Add to your Dockerfile:

```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/ready || exit 1
```

### Kubernetes Readiness/Liveness

```yaml
livenessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 30

readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

### Load Balancer Configuration

Configure your load balancer health checks:

- **Health Check URL**: `/ready`
- **Interval**: 10s
- **Timeout**: 5s
- **Healthy Threshold**: 2
- **Unhealthy Threshold**: 3

## Performance Tuning

### Database

```bash
# Connection pool sizing
export DS_DATABASE_MAX_OPEN_CONNS=25    # Max concurrent connections
export DS_DATABASE_MAX_IDLE_CONNS=10    # Idle connections to maintain
export DS_DATABASE_CONN_MAX_LIFETIME=5m # Connection reuse timeout
```

### Redis

```bash
# Connection settings
export DS_REDIS_DB=0                    # Use dedicated Redis database
export DS_REDIS_PASSWORD="strong-pass"  # Enable authentication
```

### Rate Limiting

Adjust based on your traffic patterns:

```go
rateLimitConfig := middleware.RateLimitConfig{
    RequestsPerWindow: 1000,           // Higher for production
    Window:            1 * time.Minute,
    KeyPrefix:         "ratelimit:",
}
```

## Troubleshooting

### Common Issues

**Configuration Validation Errors**:
```bash
# Check current config
deploysentry config validate

# Generate secure JWT secret
openssl rand -base64 64
```

**Database Connection Issues**:
```bash
# Test database connectivity
psql "postgres://user:pass@host:5432/db?search_path=deploy"

# Check migration status
make migrate-up
```

**Rate Limiting Problems**:
```bash
# Check Redis connectivity
redis-cli -h host -p 6379 -a password ping

# Monitor rate limit keys
redis-cli -h host -p 6379 -a password KEYS "ratelimit:*"
```

### Monitoring Alerts

Set up alerts for:

- **High Error Rate**: `rate(http_requests_total{status=~"5.."}[5m]) > 0.01`
- **High Latency**: `histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m])) > 1.0`
- **Database Connections**: `database_connections{state="open"} > 20`
- **Rate Limit Abuse**: `increase(http_requests_total{status="429"}[5m]) > 100`

## Security Considerations

1. **Secrets Management**: Use proper secrets management (HashiCorp Vault, AWS Secrets Manager, etc.)
2. **Network Security**: Deploy behind WAF and configure firewall rules
3. **Database Security**: Use separate read/write users, enable audit logging
4. **API Key Rotation**: Implement regular API key rotation policies
5. **Container Security**: Scan images for vulnerabilities, run as non-root user

## Compliance

DeploySentry production deployment supports:

- **SOC 2**: Structured logging, audit trails, access controls
- **GDPR**: Data isolation via namespaces, configurable retention
- **HIPAA**: End-to-end encryption, secure defaults
- **FedRAMP**: Strong authentication, comprehensive monitoring