# 03 — Authentication & Authorization

## OAuth 2.0 / OIDC Authentication
- [ ] Implement OAuth 2.0 flow in `internal/auth/oauth.go`
  - [ ] GitHub OAuth provider
  - [ ] Google OAuth provider
  - [ ] Okta SSO provider
- [ ] Browser-based OAuth callback handler
- [ ] Token generation (JWT) after successful OAuth
- [ ] Token refresh mechanism
- [ ] Session management (Redis-backed)
- [ ] Logout / session invalidation

## Auth Middleware (`internal/auth/middleware.go`)
- [ ] JWT validation middleware
- [ ] API key authentication middleware
- [ ] Request context enrichment (inject user, org, project info)
- [ ] Rate limiting integration (per-user, per-API-key)
- [ ] CORS configuration

## RBAC System (`internal/auth/rbac.go`)
- [ ] Role definitions:
  - [ ] `org:owner` — full access to all projects and settings
  - [ ] `org:admin` — manage projects, users, and billing
  - [ ] `project:admin` — full access within a project
  - [ ] `project:editor` — create/edit deploys, flags, releases
  - [ ] `project:viewer` — read-only access
  - [ ] `environment:deployer` — deploy to specific environments only
- [ ] Permission checking middleware
- [ ] Project-level scope enforcement
- [ ] Environment-level scope enforcement
- [ ] Resource ownership validation

## API Key Management
- [ ] API key generation with secure random bytes
- [ ] Key hashing (bcrypt/argon2) for storage
- [ ] Key prefix storage for identification (first 8 chars)
- [ ] Scoped permissions per key
- [ ] Optional environment restriction per key
- [ ] Key expiration support
- [ ] Key rotation workflow
- [ ] Last-used tracking
- [ ] CRUD API endpoints for API keys

## User Management
- [ ] User CRUD operations
- [ ] Organization membership management
  - [ ] Invite user to org
  - [ ] Remove user from org
  - [ ] Change user role in org
- [ ] Project membership management
  - [ ] Add user to project
  - [ ] Remove user from project
  - [ ] Change user role in project
- [ ] User profile (name, avatar)

## Audit Logging
- [ ] Immutable append-only audit log for all mutations
- [ ] Capture: user, action, resource type/id, old/new values, IP, user agent
- [ ] Audit log query API with filtering

## Security Hardening
- [ ] TLS configuration for API server
- [ ] Input validation and sanitization
- [ ] SQL injection prevention (parameterized queries)
- [ ] XSS prevention in any rendered content
- [ ] CSRF protection for web UI
- [ ] Secrets management integration (Vault / AWS Secrets Manager)
