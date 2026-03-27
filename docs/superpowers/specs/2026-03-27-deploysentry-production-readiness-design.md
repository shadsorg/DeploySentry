# DeploySentry Production Readiness Design

**Date:** 2026-03-27
**Status:** Draft
**Scope:** All work required to take DeploySentry from architecturally complete to production-ready

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Target | Full production readiness | Not demo/dogfood or SDK-first |
| Data access | Raw SQL with pgx | Matches existing platform layer, no new dependencies |
| Auth header standard | `Authorization: ApiKey <key>` | Avoids ambiguity with JWT Bearer tokens; 4 of 7 SDKs already use it |
| SDK test strategy | Unit + contract tests | Catches cross-SDK drift (like the auth header bug) without requiring a live server |
| Dashboard auth | Email/password + JWT | Requires new login endpoint and password hash column; OAuth layered later |
| Sequencing | Core-first expansion | Working auth + flags system after ~35% of work; validates architecture early |

---

## 1. Route Wiring & Middleware Registration

### Location
`cmd/api/main.go` — after router initialization (currently line 91)

### Changes

**Middleware stack** applied to all `/api` routes:
1. CORS middleware (`internal/platform/middleware/`)
2. Rate limiting middleware (`internal/platform/middleware/`, uses Redis)
3. Auth middleware (`internal/auth/middleware.go`) — supports both JWT and ApiKey

**Repository instantiation** — concrete pgx-backed implementations:
```go
userRepo := postgres.NewUserRepository(db.Pool())
apiKeyRepo := postgres.NewAPIKeyRepository(db.Pool())
auditRepo := postgres.NewAuditLogRepository(db.Pool())
flagRepo := postgres.NewFlagRepository(db.Pool())
deployRepo := postgres.NewDeployRepository(db.Pool())
releaseRepo := postgres.NewReleaseRepository(db.Pool())
```

**Cache adapter** — wraps Redis client to satisfy the `flags.Cache` interface:
```go
flagCache := postgres.NewFlagCache(rdb)  // implements flags.Cache (GetFlag, SetFlag, GetRules, SetRules, Invalidate)
```

**Service instantiation** — wire repos, cache, and messaging (must match actual constructors):
```go
flagService := flags.NewFlagService(flagRepo, flagCache, nc)  // flags.NewFlagService(FlagRepository, Cache, EventPublisher)
deployService := deploy.NewDeployService(deployRepo, nc)       // deploy.NewDeployService(DeployRepository, MessagePublisher)
releaseService := releases.NewReleaseServiceWithPublisher(releaseRepo, nc)  // releases.NewReleaseServiceWithPublisher(ReleaseRepository, EventPublisher)
apiKeyService := auth.NewAPIKeyService(apiKeyRepo)             // auth.NewAPIKeyService(APIKeyRepository)
rbacChecker := auth.NewRBACChecker(userRepo)                   // needed by flags handler
```

**Handler registration** on `/api/v1` group (must match actual constructors):
```go
api := router.Group("/api/v1")
flags.NewHandler(flagService, rbacChecker).RegisterRoutes(api)   // requires (FlagService, *RBACChecker)
deploy.NewHandler(deployService).RegisterRoutes(api)              // requires (DeployService)
releases.NewHandler(releaseService).RegisterRoutes(api)           // requires (ReleaseService)
auth.NewUserHandler(userRepo).RegisterRoutes(api)
auth.NewAPIKeyHandler(apiKeyService).RegisterRoutes(api)          // requires (*APIKeyService), not repo
auth.NewAuditHandler(auditRepo).RegisterRoutes(api)
auth.NewLoginHandler(userRepo, cfg.Auth).RegisterRoutes(api)      // NEW — see Section 7a
```

### New Packages
- `internal/platform/database/postgres/` — all concrete repository implementations, one file per domain
- `internal/platform/cache/flagcache/` — adapter implementing `flags.Cache` interface over the Redis client (methods: `GetFlag`, `SetFlag`, `GetRules`, `SetRules`, `Invalidate`)

---

## 2. PostgreSQL Repository Layer

### Location
`internal/platform/database/postgres/`

### Files

| File | Interface | Methods | Key Tables |
|------|-----------|---------|------------|
| `users.go` | UserRepository | 14 | `users`, `org_members`, `project_members` |
| `apikeys.go` | APIKeyRepository | 7 | `api_keys` |
| `audit.go` | AuditLogRepository | 1 | `audit_logs` |
| `flags.go` | FlagRepository | 12 | `feature_flags`, `targeting_rules`, `evaluation_logs` |
| `deploy.go` | DeployRepository | 9 | `deployments`, `deployment_phases`, `deploy_pipelines` |
| `releases.go` | ReleaseRepository | 9 | `releases`, `release_environments` |
| `helpers.go` | (shared) | — | — |

**Total: 52 interface methods** (plus 5 methods for the `flags.Cache` adapter)

### Domain Error Types (`errors.go`)
```go
var ErrNotFound = errors.New("not found")
```
All repositories map `pgx.ErrNoRows` to `ErrNotFound`. Callers check with `errors.Is(err, postgres.ErrNotFound)`.

### Transaction Handling
Multi-table operations use `pgxpool.Pool.Begin()` for atomicity:
- `CreateDeployment` — creates deployment + initial phase in one transaction
- `DeleteRule` — deletes rule + reorders remaining rule priorities
- API key rotation (if added) — creates new key + revokes old atomically

### `whereBuilder` Helper API
```go
type whereBuilder struct { conditions []string; args []any }
func (w *whereBuilder) Add(condition string, arg any)  // e.g., w.Add("project_id = $%d", projectID)
func (w *whereBuilder) Build() (string, []any)          // returns " WHERE x AND y", [args...]
```

### Patterns

- Each struct holds a `*pgxpool.Pool`
- Constructor: `NewXxxRepository(pool *pgxpool.Pool) *XxxRepository`
- All queries use `$1, $2` parameterized placeholders — no string interpolation
- No `deploy.` schema prefix — `search_path` handles it per project convention
- `ListXxx` methods support pagination via `LIMIT $n OFFSET $m`
- `ListOptions` filtering maps to `WHERE` clauses built with a `whereBuilder` helper
- UUIDs as `uuid.UUID` — pgx native support
- Timestamps as `time.Time` — pgx handles `timestamptz`
- `RETURNING` clause on INSERT/UPDATE where caller needs the populated record
- Errors wrapped: `fmt.Errorf("postgres.GetFlag: %w", err)` for traceability
- `pgx.ErrNoRows` mapped to domain-level "not found" errors

### Shared Helpers (`helpers.go`)
- `whereBuilder` — accumulates WHERE conditions and args for dynamic list queries
- `scanFlag()`, `scanDeployment()`, etc. — row scanning helpers to reduce repetition

---

## 3. SDK Auth Header Standardization

### Standard Format
```
Authorization: ApiKey <key>
```

### Changes Required

| SDK | File | Current | Target |
|-----|------|---------|--------|
| Java | `DeploySentryClient.java:267` | `"Bearer "` | `"ApiKey "` |
| Java | `SSEClient.java:94` | `"Bearer "` | `"ApiKey "` |
| React | `client.ts:144` | `Bearer` | `ApiKey` |
| Flutter | `client.dart:42` | `'Bearer $apiKey'` | `'ApiKey $apiKey'` |

Go, Node, Python, Ruby already use `ApiKey` — no changes needed.

---

## 4. React SDK Typed Evaluation Methods

### Current State
`DeploySentryClient` only has `getFlag()` and `getAllFlags()`. Users must manually access `.value` and cast.

### Methods to Add (in `sdk/react/src/client.ts`)

```typescript
boolValue(key: string, defaultValue: boolean): boolean
stringValue(key: string, defaultValue: string): string
numberValue(key: string, defaultValue: number): number  // "number" is idiomatic TS (no int type)
jsonValue(key: string, defaultValue: object): object
detail(key: string): FlagDetail  // { value, enabled, reason, metadata }
```

These pull from the in-memory flag store (already fetched via provider), matching the pattern used in the Node SDK. No additional API calls.

---

## 5. SSE Reconnection Standardization

### Standard Across All 7 SDKs

| Parameter | Value |
|-----------|-------|
| Initial delay | 1 second |
| Max delay | 30 seconds |
| Backoff factor | 2x |
| Jitter | +/- 20% (prevents thundering herd) |

### Changes Required

| SDK | Current | Change |
|-----|---------|--------|
| Go | 1s–60s exponential | Cap max at 30s, add jitter |
| Python | 1s–30s exponential | Add jitter |
| Node | Fixed 3s | Replace with exponential backoff |
| Java | Unclear | Implement standard |
| React | EventSource auto-reconnect | Add manual reconnect with standard backoff |
| Flutter | Basic reconnect | Implement standard |
| Ruby | Basic reconnect | Implement standard |

---

## 6. Session Consistency

### Problem
Users should get stable flag values for the duration of a session. Without this, flags can change mid-session causing inconsistent UX.

### Header
```
X-DeploySentry-Session: <session-id>
```

### Flow

1. SDK sends `session_id` with evaluation requests (via header and/or request body)
2. Server checks Redis for `flag_session:{session_id}` (note: `flag_session:` prefix to avoid collision with auth `session:` prefix)
3. **Cache hit:** return cached evaluation results (even if underlying flag changed)
4. **Cache miss:** evaluate fresh, cache results in Redis with sliding TTL
5. SSE updates are still received by SDKs but queued — take effect on next session refresh

### Cache Key Format
Per-flag granularity: `flag_session:{session_id}:{flag_key}` — allows individual flag evaluation without caching all flags at once. Batch evaluations cache each flag separately under the same session prefix.

### Session ID Composition (SDK-side)

```go
// Auto-generated per client instance (default)
client := deploysentry.NewClient(opts...)

// Tied to user
deploysentry.WithSessionID("user:" + userID)

// Tied to app version
deploysentry.WithSessionID("v2.3.1:" + userID)

// Custom composite
deploysentry.WithSessionID(fmt.Sprintf("%s:%s:%s", userID, appVersion, region))
```

### Server Changes
- New optional field on evaluation request: `session_id`
- Redis cache key: `flag_session:{session_id}:{flag_key}` → serialized evaluation result
- New config: `DS_SESSION_TTL` (default 30 minutes, sliding)
- No session header = fresh evaluation every time (backwards compatible)

### SDK Changes (all 7)
- `WithSessionID(id)` / `session_id` option on client init — **opt-in only, no auto-generation**
- If no session ID configured, SDKs do not send `X-DeploySentry-Session` header (preserves backward compatibility)
- Send `X-DeploySentry-Session` header only when session ID is explicitly set
- `refreshSession()` method to clear local + server cache and get fresh evaluations

### New Reason Value
`SESSION_CACHED` — returned when evaluation was served from session cache

---

## 7a. Email/Password Authentication (New)

The existing auth package only supports OAuth (GitHub, Google). There is no email/password login, no password storage, and no `CreateUser` method. This section adds what's needed for dashboard JWT auth.

### New Migration
`022_add_password_auth.up.sql`:
```sql
ALTER TABLE users ADD COLUMN password_hash TEXT;
ALTER TABLE users ADD COLUMN password_set_at TIMESTAMPTZ;
```

### New Repository Method
Add to `UserRepository` interface and postgres implementation:
- `CreateUser(ctx context.Context, user *models.User) error`

**Updated total: 53 interface methods** (52 original + 1 new)

### New Handler
`internal/auth/login_handler.go` — `LoginHandler`:
- `POST /api/v1/auth/register` — creates user with email + argon2id-hashed password, returns JWT
- `POST /api/v1/auth/login` — validates email + password, returns JWT
- Uses existing `auth.GenerateToken()` for JWT creation
- Uses existing argon2id constants from `apikeys.go` for password hashing
- Constructor: `NewLoginHandler(userRepo UserRepository, cfg config.AuthConfig) *LoginHandler`

### Scope Note
OAuth (GitHub/Google) is out of scope for this spec. The existing OAuth handler scaffold remains; it can be wired up in a future iteration.

---

## 7b. Web Dashboard — API Integration & Auth Flow

### New Components

| Component | Purpose |
|-----------|---------|
| `LoginPage.tsx` | Email/password form, calls `POST /api/v1/auth/login` |
| `RegisterPage.tsx` | Registration form, calls `POST /api/v1/auth/register` |
| `ProtectedRoute.tsx` | Wraps routes, redirects to `/login` if no valid token |
| `AuthProvider.tsx` | React context for auth state, token in `localStorage` (`ds_token`) |

Token refresh strategy: on 401 response, redirect to login. No refresh token in v1.

### Page-by-Page API Integration

| Page | Mock Data Replaced With |
|------|------------------------|
| Dashboard | `GET /api/v1/flags`, `GET /api/v1/deployments`, `GET /api/v1/releases` for counts + recent items |
| FlagListPage | `GET /api/v1/flags?project_id=X` with filter/search params |
| FlagCreatePage | `POST /api/v1/flags` |
| FlagDetailPage | `GET /api/v1/flags/:id`, `GET /api/v1/flags/:id/rules` |
| DeploymentsPage | `GET /api/v1/deployments?project_id=X` |
| ReleasesPage | `GET /api/v1/releases?project_id=X` |
| SettingsPage | `GET /api/v1/apikeys`, `POST /api/v1/apikeys`, `DELETE /api/v1/apikeys/:id` |

### Real-time Updates
- `useSSE()` hook connects to `GET /api/v1/flags/stream` with JWT token as query param
- On `flag_change` events (current server-side SSE event name), invalidate React Query cache
- Connection managed by `AuthProvider` — connects on login, disconnects on logout

### Error/Loading States
- Loading skeleton per page
- Error boundary per page
- Global error toast for network failures
- Optimistic updates for toggle/archive actions
- Empty states for no-data scenarios

---

## 8. SDK Tests — Unit + Contract Framework

### Shared Contract Fixtures
**Location:** `sdk/testdata/`

| Fixture | Purpose |
|---------|---------|
| `auth_request.json` | Validates `Authorization: ApiKey <key>` header |
| `evaluate_request.json` | Request body for `POST /flags/evaluate` with context |
| `evaluate_response.json` | Response shape: `{ value, enabled, reason, metadata }` |
| `batch_evaluate_request.json` | Batch evaluation request format |
| `batch_evaluate_response.json` | Batch response with multiple flags |
| `list_flags_response.json` | Flag list with all fields including metadata |
| `sse_messages.txt` | Raw SSE frames: `flag_change` events with `flag.toggled`, `flag.created`, `flag.updated`, `flag.deleted` payloads |

### Contract Test Behavior
Each SDK's contract tests load these fixtures and verify:
1. Client sends requests matching request fixtures (correct headers, body shape, URL)
2. Client correctly parses response fixtures into typed objects
3. SSE parser correctly handles the raw event frames

### Per-SDK Unit Tests

| Test Area | Coverage |
|-----------|----------|
| Client init | Required params, defaults, bad config rejection |
| Cache | TTL expiry, thread safety, stale vs fresh, cache miss |
| Evaluation | Type coercion, defaults on miss, detail() shape |
| Context | Serialization, optional fields omitted correctly |
| Session | Session ID sent in header, refreshSession() clears cache |
| Offline mode | Falls back to cache, no HTTP calls |
| Streaming | Reconnection on disconnect, backoff timing, event parsing |

### Test Tooling

| SDK | Framework | HTTP Mocking |
|-----|-----------|-------------|
| Go | `go test` | `net/http/httptest` |
| Node | Jest | `msw` or fetch mocks |
| Python | pytest | `httpx.MockTransport` |
| Java | JUnit 5 | `MockWebServer` |
| React | Jest + RTL | fetch mocks |
| Flutter | `flutter_test` | `http.MockClient` |
| Ruby | Minitest | `webmock` |

---

## 9. README Documentation Updates

### Additions to Main README

**Evaluation API schemas:**
```
POST /api/v1/flags/evaluate
Request:  { flag_key, project_id, environment_id, context: { user_id, org_id, attributes }, session_id? }
Response: { value, enabled, reason, flag_key, flag_type, metadata: { category, purpose, owners, tags, expires_at } }

POST /api/v1/flags/batch-evaluate
Request:  { flag_keys: [...], project_id, environment_id, context, session_id? }
Response: { evaluations: [ { flag_key, value, enabled, reason, metadata } ] }
```

**SSE protocol:**
- Endpoint: `GET /api/v1/flags/stream?project_id=X&token=Y`
- Event name: `flag_change` (the SSE event field; current server implementation uses `c.SSEvent("flag_change", msg)`)
- Data payload includes an `event` field for the specific change type: `flag.toggled`, `flag.created`, `flag.updated`, `flag.deleted`
- Message format: `event: flag_change\ndata: { "event": "flag.toggled", "flag_id": "...", "flag_key": "...", "enabled": true }\n\n`
- Heartbeat: server sends empty comment (`: heartbeat`) every 15 seconds for connection keepalive
- Reconnection: exponential backoff 1s–30s, 2x factor, +/-20% jitter

**All reason values:**
`TARGETING_MATCH`, `PERCENTAGE_ROLLOUT`, `DEFAULT_VALUE`, `FLAG_DISABLED`, `NOT_FOUND`, `ERROR`, `SESSION_CACHED`

**Rule management endpoints:**
- `POST /api/v1/flags/:id/rules` — create targeting rule (with request schema)
- `PUT /api/v1/flags/:id/rules/:ruleId` — update rule
- `DELETE /api/v1/flags/:id/rules/:ruleId` — delete rule

**Session consistency:**
- `X-DeploySentry-Session` header behavior
- Session ID composition examples
- TTL and refresh semantics

**Percentage rollout algorithm:**
- SHA256 hash of `{flag_key}:{user_id}`
- Converted to 0–100 bucket
- Deterministic: same user always gets same result for same flag

### SDK README Updates (minor)
- Add session consistency examples to each
- Fix auth header documentation in Java, React, Flutter READMEs

---

## Execution Sequence

**Phase 1 — Core (working auth + flags end-to-end):**
1. Flags.Cache adapter over Redis (Section 1)
2. Auth repositories: users (+ CreateUser), API keys, audit (Section 2 — 23 methods)
3. Email/password login handler + migration 022 (Section 7a)
4. Flags repository (Section 2 — 12 methods)
5. Route wiring in `main.go` — all middleware + handlers (Section 1)
6. **Checkpoint: flag evaluation works via API with auth**

**Phase 2 — Remaining backend:**
7. Deploy repository (Section 2 — 9 methods)
8. Release repository (Section 2 — 9 methods)
9. Session consistency server-side — Redis caching (Section 6)

**Phase 3 — Dashboard:**
10. Login/register flow + auth provider (Section 7b)
11. Connect all pages to API (Section 7b)
12. SSE real-time updates (Section 7b)

**Phase 4 — SDKs (parallelizable, except 17→18 dependency):**
13. Fix auth headers in Java, React, Flutter (Section 3)
14. Add React typed evaluation methods (Section 4)
15. Standardize SSE reconnection (Section 5)
16. Add session consistency to all SDKs (Section 6)
17. Shared contract fixtures — must complete before step 18 (Section 8)
18. Unit + contract tests for all 7 SDKs (Section 8)

**Phase 5 — Documentation:**
19. README updates (Section 9)

## Out of Scope

The following exist in migrations/code but are not addressed in this spec:
- **Organizations, Projects, Environments CRUD** — migrations 001, 004, 006 create these tables but no handlers or repos exist. For initial production use, seed data can be inserted via migration or CLI tool. Full CRUD is a separate spec.
- **Webhook endpoints and deliveries** — migrations 017, 018. Notification service publishes events but webhook management UI/API is deferred.
- **OAuth (GitHub/Google)** — existing handler scaffold remains; wired up in a future iteration.
- **Prometheus metrics export** — mentioned in README as planned.
