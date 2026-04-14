# API Security Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 7 security findings from the API audit: SSRF protection, user enumeration, webhook secret encryption, SSE limits, API key IP allowlist, JWT blacklist check, and JWT lifetime reduction.

**Architecture:** Each fix is independent and touches a small surface area. Ordered by severity: critical first, then medium, then low. Each task is self-contained and can be merged independently.

**Tech Stack:** Go 1.22, Gin, pgx/v5, AES-256-GCM, net.IP/net.IPNet

**Spec:** `docs/superpowers/specs/2026-04-13-api-security-hardening-design.md`

---

## Task 1: SSRF Protection for Webhook URLs (Critical)

Prevent webhook registration to private/internal network addresses.

**Files:**
- Create: `internal/webhooks/urlvalidator.go`
- Create: `internal/webhooks/urlvalidator_test.go`
- Modify: `internal/webhooks/handler.go`

- [ ] **Step 1: Write tests for URL validator**

Create `internal/webhooks/urlvalidator_test.go`:

```go
package webhooks

import (
	"testing"
)

func TestValidateWebhookURL_ValidPublicURLs(t *testing.T) {
	valid := []string{
		"https://example.com/webhook",
		"https://hooks.slack.com/services/T00/B00/xxx",
		"http://my-service.example.com:8080/hooks",
	}
	for _, u := range valid {
		if err := ValidateWebhookURL(u); err != nil {
			t.Errorf("expected valid URL %q, got error: %v", u, err)
		}
	}
}

func TestValidateWebhookURL_RejectsPrivateIPs(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1/hook",
		"http://localhost/hook",
		"http://10.0.0.1/hook",
		"http://172.16.0.1/hook",
		"http://192.168.1.1/hook",
		"http://169.254.169.254/latest/meta-data",
		"http://[::1]/hook",
		"http://0.0.0.0/hook",
	}
	for _, u := range blocked {
		if err := ValidateWebhookURL(u); err == nil {
			t.Errorf("expected blocked URL %q to be rejected", u)
		}
	}
}

func TestValidateWebhookURL_RejectsInvalidSchemes(t *testing.T) {
	blocked := []string{
		"ftp://example.com/file",
		"file:///etc/passwd",
		"gopher://evil.com",
		"javascript:alert(1)",
		"",
		"not-a-url",
	}
	for _, u := range blocked {
		if err := ValidateWebhookURL(u); err == nil {
			t.Errorf("expected blocked URL %q to be rejected", u)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/webhooks/ -run TestValidateWebhookURL -v`
Expected: FAIL — `ValidateWebhookURL` undefined.

- [ ] **Step 3: Implement URL validator**

Create `internal/webhooks/urlvalidator.go`:

```go
package webhooks

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// privateRanges contains CIDR blocks that webhook URLs must not resolve to.
var privateRanges = []string{
	"127.0.0.0/8",    // loopback
	"10.0.0.0/8",     // RFC 1918
	"172.16.0.0/12",  // RFC 1918
	"192.168.0.0/16", // RFC 1918
	"169.254.0.0/16", // link-local / cloud metadata
	"0.0.0.0/8",      // current network
	"::1/128",         // IPv6 loopback
	"fe80::/10",       // IPv6 link-local
	"fc00::/7",        // IPv6 unique local
}

var parsedPrivateRanges []*net.IPNet

func init() {
	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("invalid CIDR in privateRanges: %s", cidr))
		}
		parsedPrivateRanges = append(parsedPrivateRanges, network)
	}
}

// ValidateWebhookURL checks that a webhook URL uses an allowed scheme and
// does not resolve to a private or internal network address.
func ValidateWebhookURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("invalid URL")
	}

	// Only allow http and https.
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("webhook URL must use http or https scheme")
	}

	// Extract hostname (strip port).
	host := parsed.Hostname()

	// Reject "localhost" by name.
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("webhook URL must not point to localhost")
	}

	// Resolve hostname to IPs and check each against private ranges.
	ips, err := net.LookupHost(host)
	if err != nil {
		// If we can't resolve, check if host is a raw IP.
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("webhook URL hostname could not be resolved: %s", host)
		}
		ips = []string{ip.String()}
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		for _, network := range parsedPrivateRanges {
			if network.Contains(ip) {
				return fmt.Errorf("webhook URL must not point to private or internal networks")
			}
		}
	}

	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/webhooks/ -run TestValidateWebhookURL -v`
Expected: All 3 tests PASS.

- [ ] **Step 5: Wire into webhook handler**

In `internal/webhooks/handler.go`, in the `createWebhook` handler, AFTER the `c.ShouldBindJSON` call succeeds and BEFORE creating the webhook, add:

```go
	if err := ValidateWebhookURL(req.URL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
```

In the `updateWebhook` handler, add the same check when URL is being updated:

```go
	if req.URL != nil && *req.URL != "" {
		if err := ValidateWebhookURL(*req.URL); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
```

Read the handler to find the exact field name — the update request may use `*string` for optional fields.

- [ ] **Step 6: Verify build and tests**

Run: `go build ./... && go test ./internal/webhooks/ -v -timeout 15s`
Expected: All pass.

- [ ] **Step 7: Commit**

```bash
git add internal/webhooks/urlvalidator.go internal/webhooks/urlvalidator_test.go internal/webhooks/handler.go
git commit -m "security: add SSRF protection for webhook URLs"
```

---

## Task 2: User Enumeration Fix (Medium)

Restrict `GET /users/:id` to own profile or same-org users.

**Files:**
- Modify: `internal/auth/user_handler.go`

- [ ] **Step 1: Read the current handler**

Read `internal/auth/user_handler.go` to find `getUser` (around line 108-122). Understand how context keys work — the authenticated user's ID is set as `ContextKeyUserID` (value `"user_id"`) by the auth middleware.

- [ ] **Step 2: Add ownership check**

Replace the `getUser` handler body with a version that checks ownership:

```go
func (h *UserHandler) getUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	// Check if requesting own profile
	authedUserID, exists := c.Get(ContextKeyUserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	requestorID, ok := authedUserID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}

	// Allow if requesting own profile
	if id != requestorID {
		// Not own profile — return 404 to prevent enumeration
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	user, err := h.repo.GetUser(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}
```

Note: This is the simplest fix — only allow querying own profile. If same-org lookup is needed later, it can be added with an org membership check.

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add internal/auth/user_handler.go
git commit -m "security: restrict user lookup to own profile to prevent enumeration"
```

---

## Task 3: Webhook Secret Encryption (Medium)

Encrypt webhook secrets at rest using AES-256-GCM.

**Files:**
- Create: `internal/platform/crypto/aes.go`
- Create: `internal/platform/crypto/aes_test.go`
- Create: `migrations/039_add_webhook_encrypted.up.sql`
- Create: `migrations/039_add_webhook_encrypted.down.sql`
- Modify: `internal/platform/config/config.go`
- Modify: `internal/webhooks/service.go`

- [ ] **Step 1: Write crypto tests**

Create `internal/platform/crypto/aes_test.go`:

```go
package crypto

import (
	"crypto/rand"
	"testing"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("my-webhook-secret-value")
	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if string(ciphertext) == string(plaintext) {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("got %q, want %q", decrypted, plaintext)
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	rand.Read(key1)
	rand.Read(key2)

	ciphertext, _ := Encrypt([]byte("secret"), key1)
	_, err := Decrypt(ciphertext, key2)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestEncrypt_InvalidKeyLength(t *testing.T) {
	_, err := Encrypt([]byte("data"), []byte("short"))
	if err == nil {
		t.Fatal("expected error for invalid key length")
	}
}
```

- [ ] **Step 2: Implement crypto package**

Create `internal/platform/crypto/aes.go`:

```go
// Package crypto provides encryption utilities for sensitive data at rest.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// Key must be exactly 32 bytes. Returns nonce prepended to ciphertext.
func Encrypt(plaintext, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce generation: %w", err)
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts ciphertext produced by Encrypt. Key must be exactly 32 bytes.
func Decrypt(ciphertext, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}
```

- [ ] **Step 3: Run crypto tests**

Run: `go test ./internal/platform/crypto/ -v`
Expected: All 3 tests PASS.

- [ ] **Step 4: Create migration**

Create `migrations/039_add_webhook_encrypted.up.sql`:
```sql
ALTER TABLE webhooks ADD COLUMN encrypted BOOLEAN NOT NULL DEFAULT false;
```

Create `migrations/039_add_webhook_encrypted.down.sql`:
```sql
ALTER TABLE webhooks DROP COLUMN IF EXISTS encrypted;
```

- [ ] **Step 5: Add encryption key to config**

In `internal/platform/config/config.go`, add to the appropriate config struct (read the file to find the right place — likely a `SecurityConfig` or add to `ServerConfig`):

```go
EncryptionKey string `mapstructure:"encryption_key"`
```

Add default and env var mapping:
```go
v.SetDefault("security.encryption_key", "")
```

Add production validation alongside the JWT secret check:
```go
if c.Security.EncryptionKey == "" {
    log.Println("WARNING: DS_SECURITY_ENCRYPTION_KEY not set — webhook secrets will not be encrypted")
}
```

Read the config file first to find exact struct names and placement.

- [ ] **Step 6: Wire encryption into webhook service**

In `internal/webhooks/service.go`, the service needs the encryption key. Add it to the Service struct (or pass via config). Then:

On webhook **create** (in the create method), after generating the secret:
```go
if s.encryptionKey != nil {
    encrypted, err := crypto.Encrypt([]byte(webhook.Secret), s.encryptionKey)
    if err != nil {
        return fmt.Errorf("encrypting webhook secret: %w", err)
    }
    webhook.Secret = hex.EncodeToString(encrypted)
    webhook.Encrypted = true
}
```

On webhook **delivery** (in deliverWebhook), before using the secret for signature:
```go
secret := webhook.Secret
if webhook.Encrypted && s.encryptionKey != nil {
    raw, err := hex.DecodeString(secret)
    if err == nil {
        decrypted, err := crypto.Decrypt(raw, s.encryptionKey)
        if err == nil {
            secret = string(decrypted)
        }
    }
}
signature := s.generateSignature(secret, timestamp, payloadBytes)
```

Add `Encrypted bool` field to the Webhook model in `internal/models/webhook.go`:
```go
Encrypted bool `json:"-" db:"encrypted"`
```

- [ ] **Step 7: Verify build and tests**

Run: `go build ./... && go test ./internal/platform/crypto/ ./internal/webhooks/ -v -timeout 15s`
Expected: All pass.

- [ ] **Step 8: Commit**

```bash
git add internal/platform/crypto/ migrations/039_* internal/platform/config/config.go internal/webhooks/service.go internal/models/webhook.go
git commit -m "security: encrypt webhook secrets at rest with AES-256-GCM"
```

---

## Task 4: SSE Connection Limits (Medium)

Add per-org connection limit to the SSE streaming endpoint.

**Files:**
- Modify: `internal/flags/handler.go`

- [ ] **Step 1: Read the current SSE handler**

Read `internal/flags/handler.go` around the `streamFlags` handler (lines 939-961) and the SSEBroker (lines 894-935). Understand how the org ID is available in context (set by auth middleware as `ContextKeyOrgID`).

- [ ] **Step 2: Add connection counter to the handler struct or SSEBroker**

The simplest approach: add a connection counter directly in the `streamFlags` handler using a package-level sync.Map. In `internal/flags/handler.go`, add near the top:

```go
var (
	sseConnections   sync.Map // map[uuid.UUID]*int32 (org ID → connection count)
	sseMaxPerOrg     int32 = 50
)
```

- [ ] **Step 3: Add limit check and tracking to streamFlags**

At the beginning of the `streamFlags` handler, BEFORE subscribing:

```go
	// Get org ID for connection tracking
	orgIDVal, exists := c.Get(auth.ContextKeyOrgID)
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "org context required"})
		return
	}
	orgID, ok := orgIDVal.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid org context"})
		return
	}

	// Check and increment connection count
	counterPtr, _ := sseConnections.LoadOrStore(orgID, new(int32))
	counter := counterPtr.(*int32)
	current := atomic.AddInt32(counter, 1)
	if current > sseMaxPerOrg {
		atomic.AddInt32(counter, -1)
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many SSE connections for this organization"})
		return
	}
	defer atomic.AddInt32(counter, -1)
```

Add imports for `sync/atomic` and `sync` if not present.

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: Build succeeds.

- [ ] **Step 5: Commit**

```bash
git add internal/flags/handler.go
git commit -m "security: add per-org SSE connection limit (max 50)"
```

---

## Task 5: API Key IP Allowlist (Medium)

Add optional CIDR-based IP restriction to API keys.

**Files:**
- Create: `migrations/040_add_apikey_allowed_cidrs.up.sql`
- Create: `migrations/040_add_apikey_allowed_cidrs.down.sql`
- Modify: `internal/models/api_key.go`
- Modify: `internal/auth/apikeys.go`
- Modify: `internal/auth/apikey_handler.go`

- [ ] **Step 1: Create migration**

Create `migrations/040_add_apikey_allowed_cidrs.up.sql`:
```sql
ALTER TABLE api_keys ADD COLUMN allowed_cidrs TEXT[];
```

Create `migrations/040_add_apikey_allowed_cidrs.down.sql`:
```sql
ALTER TABLE api_keys DROP COLUMN IF EXISTS allowed_cidrs;
```

- [ ] **Step 2: Add field to model**

In `internal/models/api_key.go`, add to the `APIKey` struct:

```go
AllowedCIDRs []string `json:"allowed_cidrs,omitempty" db:"allowed_cidrs"`
```

- [ ] **Step 3: Add IP check to ValidateKey**

In `internal/auth/apikeys.go`, add a function:

```go
// CheckIPAllowed verifies that clientIP falls within at least one of the
// allowed CIDRs. Returns true if allowedCIDRs is empty (no restriction).
func CheckIPAllowed(clientIP string, allowedCIDRs []string) bool {
	if len(allowedCIDRs) == 0 {
		return true
	}

	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	for _, cidr := range allowedCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
```

Add `"net"` import.

- [ ] **Step 4: Wire IP check into API key middleware**

In `internal/auth/middleware.go`, in the `authenticateAPIKey` method, AFTER the key is validated successfully by `m.keyValidator.ValidateAPIKey()`, add:

```go
	if len(apiKey.AllowedCIDRs) > 0 {
		clientIP := c.ClientIP()
		if !CheckIPAllowed(clientIP, apiKey.AllowedCIDRs) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "request IP not in API key allowlist"})
			return false
		}
	}
```

Note: Read the middleware to find the exact variable name for the API key result and the method signature.

- [ ] **Step 5: Add allowed_cidrs to create request**

In `internal/auth/apikey_handler.go`, add to `createAPIKeyRequest`:

```go
AllowedCIDRs []string `json:"allowed_cidrs"`
```

And pass it through to the API key creation.

- [ ] **Step 6: Update the postgres repository scan**

Read the API key postgres repository to find where API keys are scanned from DB rows. Add `allowed_cidrs` to the SELECT and Scan calls. The `pq.Array` scanner handles `TEXT[]` columns:

```go
pq.Array(&key.AllowedCIDRs)
```

- [ ] **Step 7: Verify build and tests**

Run: `go build ./... && go test ./internal/auth/ -v -timeout 15s`
Expected: All pass.

- [ ] **Step 8: Commit**

```bash
git add migrations/040_* internal/models/api_key.go internal/auth/apikeys.go internal/auth/apikey_handler.go internal/auth/middleware.go internal/platform/database/postgres/
git commit -m "security: add optional IP allowlist (CIDR) for API keys"
```

---

## Task 6: JWT Blacklist Check (Low)

Wire the existing session blacklist into JWT validation for immediate token revocation.

**Files:**
- Modify: `internal/auth/middleware.go`

- [ ] **Step 1: Read the current middleware and session manager**

Read `internal/auth/middleware.go` to understand:
- The `AuthMiddleware` struct — what fields does it have? Does it already have access to the session manager?
- The `authenticateJWT` method — the exact flow after token parsing
- The `TokenClaims` struct — does it have a JTI (JWT ID) field?

Also read `internal/auth/session.go` to find `IsTokenBlacklisted` method signature.

- [ ] **Step 2: Add session manager to AuthMiddleware if not present**

If the `AuthMiddleware` struct doesn't already have a `sessionMgr` field, add one and update the constructor.

- [ ] **Step 3: Add blacklist check after JWT parsing**

In `authenticateJWT`, after the token is parsed and validated (after `if err != nil || !token.Valid`), add:

```go
	// Check token blacklist for immediate revocation
	if claims.ID != "" && m.sessionMgr != nil {
		blacklisted, err := m.sessionMgr.IsTokenBlacklisted(c.Request.Context(), claims.ID)
		if err == nil && blacklisted {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
			return false
		}
	}
```

Note: `claims.ID` is the standard JWT `jti` claim. Check if `TokenClaims` embeds `jwt.RegisteredClaims` which includes the `ID` field, or if a custom field is used.

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: Build succeeds.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/middleware.go
git commit -m "security: check JWT blacklist for immediate token revocation"
```

---

## Task 7: JWT Lifetime Reduction + NATS Documentation (Low)

Reduce default JWT expiry and add NATS security documentation.

**Files:**
- Modify: `internal/platform/config/config.go`
- Modify: `CLAUDE.md` or `deploy/README.md`

- [ ] **Step 1: Change JWT default**

In `internal/platform/config/config.go`, find the line:

```go
v.SetDefault("auth.jwt_expiration", 24*time.Hour)
```

Change to:

```go
v.SetDefault("auth.jwt_expiration", 30*time.Minute)
```

- [ ] **Step 2: Add NATS security documentation**

Add to `CLAUDE.md` under the Build & Run section or create a new section:

```markdown
## Production Security

### NATS
- NATS must be firewalled to internal services only — not exposed to the internet
- Enable authentication: set `DS_NATS_USER` and `DS_NATS_PASSWORD` for NATS connection credentials
- Enable TLS: set `DS_NATS_TLS_CERT` and `DS_NATS_TLS_KEY` for encrypted connections
- The phase engine validates deployment IDs from NATS messages against the database before processing
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add internal/platform/config/config.go CLAUDE.md
git commit -m "security: reduce JWT default to 30min, add NATS security docs"
```

---

## Task 8: Final Verification

Run all tests and verify the full build.

**Files:** None (verification only)

- [ ] **Step 1: Full Go build**

Run: `go build ./...`
Expected: Build succeeds.

- [ ] **Step 2: Run all auth tests**

Run: `go test ./internal/auth/ -v -timeout 15s`
Expected: All pass.

- [ ] **Step 3: Run webhook tests**

Run: `go test ./internal/webhooks/ -v -timeout 15s`
Expected: All pass.

- [ ] **Step 4: Run crypto tests**

Run: `go test ./internal/platform/crypto/ -v -timeout 15s`
Expected: All pass.

- [ ] **Step 5: Run flags tests**

Run: `go test ./internal/flags/ -v -timeout 15s`
Expected: All pass.

- [ ] **Step 6: Update Current_Initiatives.md**

Add the security hardening initiative to the active list.

- [ ] **Step 7: Mark spec as complete**

Change `**Status:** Approved` to `**Status:** Complete` in `docs/superpowers/specs/2026-04-13-api-security-hardening-design.md`.

- [ ] **Step 8: Commit**

```bash
git add docs/
git commit -m "docs: mark API security hardening spec as complete"
```
