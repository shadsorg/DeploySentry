# Re-integrate Security Features Plan

**Phase:** Implementation
**Priority:** Medium — security hardening features that were dropped during merge

## Overview

During the merge of `feature/groups-and-resource-authorization` into `main`, two PRs from remote had code conflicts resolved with `-X ours`, dropping some features. The database migrations are present but the Go code was overwritten.

## Already Re-integrated (this session)

- [x] **AllowedCIDRs on API keys** — IP allowlist field, middleware CIDR check
- [x] **FlagActivityChecker** — Active flag check before project deletion

## Remaining: Session Blacklisting

### What it does
JWT tokens can be immediately revoked by adding them to a blacklist. The auth middleware checks the blacklist on every JWT-authenticated request. This prevents a stolen/compromised token from being used after the user logs out or an admin revokes it.

### Implementation tasks

#### Task 1: Session Manager Interface and Redis Implementation

**Files:**
- Create: `internal/auth/session.go`

Define a `SessionManager` interface:
```go
type SessionManager interface {
    BlacklistToken(ctx context.Context, jti string, expiry time.Duration) error
    IsTokenBlacklisted(ctx context.Context, jti string) (bool, error)
}
```

Redis implementation stores blacklisted JTIs as keys with TTL matching the token's remaining lifetime:
```go
type RedisSessionManager struct {
    client *redis.Client
    prefix string // "session:blacklist:"
}

func (m *RedisSessionManager) BlacklistToken(ctx context.Context, jti string, expiry time.Duration) error {
    return m.client.Set(ctx, m.prefix+jti, "1", expiry).Err()
}

func (m *RedisSessionManager) IsTokenBlacklisted(ctx context.Context, jti string) (bool, error) {
    exists, err := m.client.Exists(ctx, m.prefix+jti).Result()
    return exists > 0, err
}
```

#### Task 2: Wire Session Manager into Auth Middleware

**Files:**
- Modify: `internal/auth/middleware.go`

Add `sessionMgr SessionManager` field to `AuthMiddleware`. Update the constructor to accept it. In `authenticateJWT`, after validating the token, check:

```go
if claims.ID != "" && m.sessionMgr != nil {
    blacklisted, err := m.sessionMgr.IsTokenBlacklisted(c.Request.Context(), claims.ID)
    if err == nil && blacklisted {
        c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
        return false
    }
}
```

#### Task 3: Add Logout/Revoke Endpoints

**Files:**
- Modify: `internal/auth/user_handler.go` (or create `internal/auth/session_handler.go`)

Add `POST /api/v1/auth/logout` — blacklists the current JWT's JTI with remaining TTL.
Add `POST /api/v1/auth/revoke` (admin) — blacklists a specific user's active tokens.

#### Task 4: Wire into API Server

**Files:**
- Modify: `cmd/api/main.go`

Create `RedisSessionManager` from the existing Redis client, pass it to `NewAuthMiddleware`.

### Estimated effort
Small — ~100 lines of Go across 4 files. Redis operations are simple SET/EXISTS with TTL.

### Dependencies
- Redis (already running)
- JWT claims must include `jti` (JWT ID) field — verify this is already set during token generation

## Checklist

- [x] AllowedCIDRs on API keys
- [x] FlagActivityChecker for project deletion
- [ ] Session blacklisting (SessionManager + middleware + endpoints + wiring)
