# 03 — Authentication & Authorization

## OAuth 2.0 / OIDC Authentication
- [x] Implement OAuth 2.0 flow in `internal/auth/oauth.go`
  - [x] GitHub OAuth provider
  - [x] Google OAuth provider
  - [ ] Okta SSO provider
- [x] Browser-based OAuth callback handler
- [x] Token generation (JWT) after successful OAuth
- [ ] Token refresh mechanism
- [ ] Session management (Redis-backed)
- [ ] Logout / session invalidation

## Auth Middleware (`internal/auth/middleware.go`)
- [x] JWT validation middleware
- [x] API key authentication middleware
- [x] Request context enrichment (inject user, org, project info)
- [x] Rate limiting integration (per-user, per-API-key)
- [x] CORS configuration

## RBAC System (`internal/auth/rbac.go`)
- [ ] Role definitions:
  - [ ] `org:owner` — full access to all projects and settings
  - [ ] `org:admin` — manage projects, users, and billing
  - [ ] `project:admin` — full access within a project
  - [ ] `project:editor` — create/edit deploys, flags, releases
  - [ ] `project:viewer` — read-only access
  - [ ] `environment:deployer` — deploy to specific environments only
- [x] Permission checking middleware
- [ ] Project-level scope enforcement
- [ ] Environment-level scope enforcement
- [ ] Resource ownership validation

## API Key Management
- [ ] API key generation with secure random bytes
- [ ] Key hashing (bcrypt/argon2) for storage
- [x] Key prefix storage for identification (first 8 chars)
- [x] Scoped permissions per key
- [ ] Optional environment restriction per key
- [x] Key expiration support
- [ ] Key rotation workflow
- [x] Last-used tracking
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
- [x] Immutable append-only audit log for all mutations
- [x] Capture: user, action, resource type/id, old/new values, IP, user agent
- [ ] Audit log query API with filtering

## Security Hardening
- [ ] TLS configuration for API server
- [x] Input validation and sanitization
- [ ] SQL injection prevention (parameterized queries)
- [ ] XSS prevention in any rendered content
- [ ] CSRF protection for web UI
- [ ] Secrets management integration (Vault / AWS Secrets Manager)
