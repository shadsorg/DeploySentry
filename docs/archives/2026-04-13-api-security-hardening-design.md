# API Security Hardening — Design Spec

**Date:** 2026-04-13
**Status:** Approved

## Overview

Address 7 security findings from a comprehensive API audit. Fixes range from critical (SSRF) to low (JWT lifetime). All changes are surgical — each fix is independent and touches a small surface area.

---

## Section 1: SSRF Protection (Critical)

**Problem:** Webhook URLs are not validated against private/internal networks. Users can register webhooks to `127.0.0.1`, `169.254.169.254` (AWS metadata), or internal services.

**Fix:** New `internal/webhooks/urlvalidator.go` with a `ValidateWebhookURL` function that:
- Parses the URL and requires `http` or `https` scheme
- Resolves the hostname to IP addresses (catches DNS rebinding to internal IPs)
- Rejects: loopback (`127.0.0.0/8`, `::1`), private (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`), link-local (`169.254.0.0/16`, `fe80::/10`), and cloud metadata (`169.254.169.254`)
- Called in webhook `Create` and `Update` handlers before persisting
- Returns a clear error message: "webhook URL must not point to private or internal networks"

---

## Section 2: User Enumeration Fix (Medium)

**Problem:** `GET /users/:id` returns any user's profile (email, name) without ownership check.

**Fix:** In the `getUser` handler, check that the requested user ID matches the authenticated user's ID from the JWT claims. If not, check whether the requested user belongs to the same organization as the authenticated user. If neither condition is met, return 404.

---

## Section 3: Webhook Secret Encryption (Medium)

**Problem:** Webhook secrets stored plaintext in the `secret` column of the webhooks table.

**Fix:**
- New `internal/platform/crypto/aes.go` package with `Encrypt(plaintext []byte, key []byte) ([]byte, error)` and `Decrypt(ciphertext []byte, key []byte) ([]byte, error)` using AES-256-GCM with random nonce
- New env var `DS_ENCRYPTION_KEY` (32-byte hex-encoded key) with production validation (must be set, must not be default)
- Encrypt webhook secret on create/update in the webhook service
- Decrypt webhook secret on delivery in the webhook service
- New migration adding an `encrypted` boolean column to webhooks (default false), allowing gradual migration of existing secrets
- On delivery: if `encrypted` is true, decrypt before use; if false, use plaintext (backward compatible)

---

## Section 4: SSE Connection Limits (Medium)

**Problem:** No limit on concurrent SSE connections per org. Could exhaust server resources.

**Fix:**
- Add a connection counter map (`map[uuid.UUID]int32`) to the SSE/streaming handler, protected by a mutex
- On connection open: increment counter for the org; if counter exceeds limit (default 50, configurable via `DS_SSE_MAX_CONNECTIONS_PER_ORG`), return HTTP 429 Too Many Requests
- On connection close (via defer): decrement counter
- The counter is in-memory — acceptable because SSE connections are per-process anyway

---

## Section 5: API Key IP Allowlist (Medium)

**Problem:** API keys work from any IP address.

**Fix:**
- New migration adding `allowed_cidrs TEXT[]` column to `api_keys` table (nullable, default null)
- In the API key authentication middleware, after validating the key hash: if `allowed_cidrs` is non-empty, parse the request IP from `X-Forwarded-For` or `c.ClientIP()` and check it against each CIDR using `net.IPNet.Contains`. If no match, return 403 Forbidden.
- Null/empty `allowed_cidrs` means no IP restriction (backward compatible)
- API key creation/update endpoints accept an optional `allowed_cidrs` field

---

## Section 6: NATS Access Control (Documentation)

**Problem:** Phase engine trusts NATS messages. If NATS is externally reachable, attackers could trigger deployments.

**Fix:** No code change. The engine already validates by fetching the deployment from the database after receiving a NATS message — a fabricated deployment ID results in a "not found" error and no action.

Add documentation to `deploy/` README or CLAUDE.md:
- NATS must be firewalled to internal services only
- Enable NATS authentication (user/password or token) in production
- Enable TLS for NATS connections in production

---

## Section 7: JWT Lifetime Reduction (Low)

**Problem:** 24h JWT lifetime with no immediate revocation mechanism wired to JWT validation.

**Fix:**
- Change default `DS_AUTH_JWT_EXPIRATION` from `24h` to `30m`
- In the JWT auth middleware (`RequireAuth`), after parsing the token, check the token's JTI against the session blacklist (`SessionManager.IsTokenBlacklisted`). If blacklisted, return 401.
- This enables immediate token revocation via the existing blacklist infrastructure

---

## Files Affected

| File | Change |
|------|--------|
| `internal/webhooks/urlvalidator.go` | New: SSRF URL validation |
| `internal/webhooks/urlvalidator_test.go` | New: tests for URL validation |
| `internal/webhooks/handler.go` | Call ValidateWebhookURL on create/update |
| `internal/auth/user_handler.go` | Restrict getUser to self or same-org |
| `internal/platform/crypto/aes.go` | New: AES-256-GCM encrypt/decrypt |
| `internal/platform/crypto/aes_test.go` | New: crypto tests |
| `internal/webhooks/service.go` | Encrypt/decrypt webhook secrets |
| `internal/platform/config/config.go` | Add DS_ENCRYPTION_KEY, DS_SSE_MAX_CONNECTIONS_PER_ORG |
| `internal/flags/handler.go` | Add SSE connection counter with org limit |
| `internal/auth/apikeys.go` | Check allowed_cidrs on validation |
| `internal/auth/apikey_handler.go` | Accept allowed_cidrs on create/update |
| `internal/auth/middleware.go` | Add blacklist check to JWT validation |
| `internal/platform/config/config.go` | Change JWT default from 24h to 30m |
| `migrations/039_add_webhook_encrypted.up.sql` | Add encrypted boolean to webhooks |
| `migrations/040_add_apikey_allowed_cidrs.up.sql` | Add allowed_cidrs to api_keys |

## What's NOT Changing

- Authentication flow (JWT + API key dual auth)
- RBAC permission model
- Webhook delivery mechanics (retry, backoff, signatures)
- Rate limiting configuration
- CORS or security headers
