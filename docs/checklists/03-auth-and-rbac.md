# 03 — Authentication & Authorization

## OAuth 2.0 / OIDC Authentication
- [x] Implement OAuth 2.0 flow in `internal/auth/oauth.go`
  - [x] GitHub OAuth provider
  - [x] Google OAuth provider
  - [ ] Okta SSO provider
- [x] Browser-based OAuth callback handler
- [x] Token generation (JWT) after successful OAuth
- [x] Token refresh mechanism
- [x] Session management (Redis-backed)
- [x] Logout / session invalidation

## Auth Middleware (`internal/auth/middleware.go`)
- [x] JWT validation middleware
- [x] API key authentication middleware
- [x] Request context enrichment (inject user, org, project info)
- [x] Rate limiting integration (per-user, per-API-key)
- [x] CORS configuration

## RBAC System (`internal/auth/rbac.go`)
- [x] Role definitions:
  - [x] `org:owner` — full access to all projects and settings
  - [x] `org:admin` — manage projects, users, and billing
  - [x] `project:admin` — full access within a project
  - [x] `project:editor` — create/edit deploys, flags, releases
  - [x] `project:viewer` — read-only access
  - [x] `environment:deployer` — deploy to specific environments only
- [x] Permission checking middleware
- [x] Project-level scope enforcement
- [x] Environment-level scope enforcement
- [x] Resource ownership validation

## API Key Management
- [x] API key generation with secure random bytes
- [x] Key hashing (bcrypt/argon2) for storage
- [x] Key prefix storage for identification (first 8 chars)
- [x] Scoped permissions per key
- [x] Optional environment restriction per key
- [x] Key expiration support
- [x] Key rotation workflow
- [x] Last-used tracking
- [x] CRUD API endpoints for API keys

## User Management
- [x] User CRUD operations
- [x] Organization membership management
  - [x] Invite user to org
  - [x] Remove user from org
  - [x] Change user role in org
- [x] Project membership management
  - [x] Add user to project
  - [x] Remove user from project
  - [x] Change user role in project
- [x] User profile (name, avatar)

## Audit Logging
- [x] Immutable append-only audit log for all mutations
- [x] Capture: user, action, resource type/id, old/new values, IP, user agent
- [x] Audit log query API with filtering

## Security Hardening
- [ ] TLS configuration for API server
- [x] Input validation and sanitization
- [x] SQL injection prevention (parameterized queries)
- [ ] XSS prevention in any rendered content
- [ ] CSRF protection for web UI
- [ ] Secrets management integration (Vault / AWS Secrets Manager)
