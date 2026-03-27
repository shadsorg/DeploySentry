# Backend Production Readiness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the DeploySentry backend fully functional — all 53 repository methods implemented, routes wired, email/password auth working, flag evaluation end-to-end.

**Architecture:** Raw SQL with pgx against the `deploy` PostgreSQL schema. Repositories implement existing interfaces. A cache adapter bridges Redis to the `flags.Cache` interface. A new login handler adds email/password auth. Route wiring in `main.go` connects everything.

**Tech Stack:** Go 1.22, pgx v5, gin-gonic, Redis (go-redis), NATS JetStream, argon2id, JWT (golang-jwt)

**Spec:** `docs/superpowers/specs/2026-03-27-deploysentry-production-readiness-design.md`

---

## Critical: Schema-Model Mismatches

The migration schema and Go model structs have diverged. Before writing repositories, we must reconcile them. Migration 023 addresses all gaps.

**Key mismatches (migration schema → model):**
- `feature_flags`: no `environment_id` column → model has `EnvironmentID uuid.UUID`
- `feature_flags`: `archived_at TIMESTAMPTZ` → model has `Archived bool`
- `feature_flags`: `default_value JSONB` → model has `DefaultValue string`
- `flag_targeting_rules`: `conditions JSONB` + `serve_value JSONB` → model has individual fields (`Percentage`, `Attribute`, `Operator`, `TargetValues`, etc.)
- `users`: no `updated_at`, `email_verified`, `password_hash` → model has all three
- `api_keys`: no `org_id`, no `revoked_at`, `project_id` NOT NULL → model has `OrgID`, `RevokedAt`, `ProjectID *uuid.UUID`
- `org_members`: composite PK, no `id` column → model has `ID uuid.UUID`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `migrations/022_add_password_auth.up.sql` | Create | Add password columns to users |
| `migrations/022_add_password_auth.down.sql` | Create | Reverse migration 022 |
| `migrations/023_schema_reconciliation.up.sql` | Create | Align schema with models |
| `migrations/023_schema_reconciliation.down.sql` | Create | Reverse migration 023 |
| `internal/platform/database/postgres/errors.go` | Create | Domain error types |
| `internal/platform/database/postgres/helpers.go` | Create | whereBuilder, row scanning helpers |
| `internal/platform/cache/flagcache/flagcache.go` | Create | flags.Cache adapter over Redis |
| `internal/platform/database/postgres/users.go` | Create | UserRepository (15 methods) |
| `internal/platform/database/postgres/apikeys.go` | Create | APIKeyRepository (7 methods) |
| `internal/platform/database/postgres/audit.go` | Create | AuditLogRepository (1 method) |
| `internal/platform/database/postgres/flags.go` | Create | FlagRepository (12 methods) |
| `internal/platform/database/postgres/deploy.go` | Create | DeployRepository (9 methods) |
| `internal/platform/database/postgres/releases.go` | Create | ReleaseRepository (9 methods) |
| `internal/auth/jwt.go` | Create | Shared GenerateJWT function |
| `internal/auth/login_handler.go` | Create | Email/password login + register |
| `internal/auth/user_handler.go` | Modify | Add CreateUser to UserRepository interface |
| `internal/platform/config/config.go` | Modify | Add SessionTTL to AuthConfig |
| `cmd/api/main.go` | Modify | Wire routes, middleware, repos, services |
| `internal/platform/database/postgres/users_test.go` | Create | UserRepository tests |
| `internal/platform/database/postgres/flags_test.go` | Create | FlagRepository tests |
| `internal/platform/database/postgres/apikeys_test.go` | Create | APIKeyRepository tests |
| `internal/platform/database/postgres/deploy_test.go` | Create | DeployRepository tests |
| `internal/platform/database/postgres/releases_test.go` | Create | ReleaseRepository tests |
| `internal/platform/cache/flagcache/flagcache_test.go` | Create | Flag cache adapter tests |
| `internal/auth/login_handler_test.go` | Create | Login handler tests |

---

### Task 1: Schema Reconciliation Migrations

**Files:**
- Create: `migrations/022_add_password_auth.up.sql`
- Create: `migrations/022_add_password_auth.down.sql`
- Create: `migrations/023_schema_reconciliation.up.sql`
- Create: `migrations/023_schema_reconciliation.down.sql`

- [ ] **Step 1: Create migration 022 — password auth**

```sql
-- migrations/022_add_password_auth.up.sql
ALTER TABLE users ADD COLUMN password_hash TEXT;
ALTER TABLE users ADD COLUMN password_set_at TIMESTAMPTZ;
```

```sql
-- migrations/022_add_password_auth.down.sql
ALTER TABLE users DROP COLUMN IF EXISTS password_set_at;
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
```

- [ ] **Step 2: Create migration 023 — schema reconciliation**

This aligns the migration schema with the Go model structs.

```sql
-- migrations/023_schema_reconciliation.up.sql

-- users: add missing columns
ALTER TABLE users ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT false;

-- feature_flags: add environment_id for multi-environment flag evaluation
ALTER TABLE feature_flags ADD COLUMN IF NOT EXISTS environment_id UUID REFERENCES environments(id);
CREATE INDEX IF NOT EXISTS idx_feature_flags_env ON feature_flags (project_id, environment_id, key);

-- api_keys: add org_id, make project_id nullable, add revoked_at
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES organizations(id);
ALTER TABLE api_keys ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;

-- org_members: add id, invited_by, created_at, updated_at
ALTER TABLE org_members ADD COLUMN IF NOT EXISTS id UUID DEFAULT gen_random_uuid();
ALTER TABLE org_members ADD COLUMN IF NOT EXISTS invited_by UUID REFERENCES users(id);
ALTER TABLE org_members ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE org_members ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- project_members: add id, created_at, updated_at
ALTER TABLE project_members ADD COLUMN IF NOT EXISTS id UUID DEFAULT gen_random_uuid();
ALTER TABLE project_members ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE project_members ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- deployments: add columns the model expects
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS project_id UUID REFERENCES projects(id);
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS strategy TEXT;
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS version TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS artifact TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS commit_sha TEXT;
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS traffic_percent INT NOT NULL DEFAULT 0;
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
-- Rename environment to environment_id style (keep as TEXT for now, handler resolves)
ALTER TABLE deployments ALTER COLUMN pipeline_id DROP NOT NULL;
ALTER TABLE deployments ALTER COLUMN release_id DROP NOT NULL;

-- deployment_phases: add deployment model fields
ALTER TABLE deployment_phases ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT '';
ALTER TABLE deployment_phases ADD COLUMN IF NOT EXISTS sort_order INT NOT NULL DEFAULT 0;

-- flag_targeting_rules: add individual columns that model uses
-- (conditions JSONB stays for complex queries; individual cols for simple access)
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS value TEXT NOT NULL DEFAULT '';
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS percentage INT;
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS attribute TEXT;
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS operator TEXT;
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS target_values TEXT[];
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS segment_id UUID;
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS start_time TIMESTAMPTZ;
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS end_time TIMESTAMPTZ;
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- releases: add missing model fields
ALTER TABLE releases ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '';
ALTER TABLE releases ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE releases ADD COLUMN IF NOT EXISTS artifact TEXT NOT NULL DEFAULT '';
ALTER TABLE releases ADD COLUMN IF NOT EXISTS lifecycle_status TEXT;
ALTER TABLE releases ADD COLUMN IF NOT EXISTS released_at TIMESTAMPTZ;
ALTER TABLE releases ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
-- Rename artifact_url -> kept, but also add artifact field

-- release_environments: add missing model fields
ALTER TABLE release_environments ADD COLUMN IF NOT EXISTS environment_id UUID;
ALTER TABLE release_environments ADD COLUMN IF NOT EXISTS deployment_id UUID;
ALTER TABLE release_environments ADD COLUMN IF NOT EXISTS lifecycle_status TEXT;
ALTER TABLE release_environments ADD COLUMN IF NOT EXISTS deployed_by UUID;
ALTER TABLE release_environments ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE release_environments ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
```

```sql
-- migrations/023_schema_reconciliation.down.sql
-- Reversing all additions from 023
ALTER TABLE release_environments DROP COLUMN IF EXISTS updated_at;
ALTER TABLE release_environments DROP COLUMN IF EXISTS created_at;
ALTER TABLE release_environments DROP COLUMN IF EXISTS deployed_by;
ALTER TABLE release_environments DROP COLUMN IF EXISTS lifecycle_status;
ALTER TABLE release_environments DROP COLUMN IF EXISTS deployment_id;
ALTER TABLE release_environments DROP COLUMN IF EXISTS environment_id;

ALTER TABLE releases DROP COLUMN IF EXISTS updated_at;
ALTER TABLE releases DROP COLUMN IF EXISTS released_at;
ALTER TABLE releases DROP COLUMN IF EXISTS lifecycle_status;
ALTER TABLE releases DROP COLUMN IF EXISTS artifact;
ALTER TABLE releases DROP COLUMN IF EXISTS description;
ALTER TABLE releases DROP COLUMN IF EXISTS title;

ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS updated_at;
ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS end_time;
ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS start_time;
ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS segment_id;
ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS target_values;
ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS operator;
ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS attribute;
ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS percentage;
ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS value;

ALTER TABLE deployment_phases DROP COLUMN IF EXISTS sort_order;
ALTER TABLE deployment_phases DROP COLUMN IF EXISTS name;

ALTER TABLE deployments DROP COLUMN IF EXISTS updated_at;
ALTER TABLE deployments DROP COLUMN IF EXISTS traffic_percent;
ALTER TABLE deployments DROP COLUMN IF EXISTS commit_sha;
ALTER TABLE deployments DROP COLUMN IF EXISTS artifact;
ALTER TABLE deployments DROP COLUMN IF EXISTS version;
ALTER TABLE deployments DROP COLUMN IF EXISTS strategy;
ALTER TABLE deployments DROP COLUMN IF EXISTS project_id;

ALTER TABLE project_members DROP COLUMN IF EXISTS updated_at;
ALTER TABLE project_members DROP COLUMN IF EXISTS created_at;
ALTER TABLE project_members DROP COLUMN IF EXISTS id;

ALTER TABLE org_members DROP COLUMN IF EXISTS updated_at;
ALTER TABLE org_members DROP COLUMN IF EXISTS created_at;
ALTER TABLE org_members DROP COLUMN IF EXISTS invited_by;
ALTER TABLE org_members DROP COLUMN IF EXISTS id;

ALTER TABLE api_keys DROP COLUMN IF EXISTS revoked_at;
ALTER TABLE api_keys ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE api_keys DROP COLUMN IF EXISTS org_id;

ALTER TABLE feature_flags DROP COLUMN IF EXISTS environment_id;

ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
ALTER TABLE users DROP COLUMN IF EXISTS updated_at;
```

- [ ] **Step 3: Run migrations**

Run: `make migrate-up`
Expected: All migrations apply successfully (000 through 023)

- [ ] **Step 4: Commit**

```bash
git add migrations/022_add_password_auth.up.sql migrations/022_add_password_auth.down.sql \
      migrations/023_schema_reconciliation.up.sql migrations/023_schema_reconciliation.down.sql
git commit -m "feat: add password auth and schema reconciliation migrations"
```

---

### Task 2: Domain Error Types

**Files:**
- Create: `internal/platform/database/postgres/errors.go`

- [ ] **Step 1: Create errors.go**

```go
package postgres

import "errors"

// ErrNotFound is returned when a queried entity does not exist.
// Callers check with errors.Is(err, postgres.ErrNotFound).
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when an insert or update violates a uniqueness constraint.
var ErrConflict = errors.New("conflict")
```

- [ ] **Step 2: Commit**

```bash
git add internal/platform/database/postgres/errors.go
git commit -m "feat: add domain error types for postgres package"
```

---

### Task 3: whereBuilder Helper

**Files:**
- Create: `internal/platform/database/postgres/helpers.go`

- [ ] **Step 1: Create helpers.go**

```go
package postgres

import (
	"fmt"
	"strings"
)

// whereBuilder accumulates WHERE conditions and positional arguments for
// building dynamic SQL queries safely.
type whereBuilder struct {
	conditions []string
	args       []any
}

// Add appends a condition. The placeholder must use %d for the argument position,
// which will be replaced with the next $N placeholder.
// Example: w.Add("project_id = $%d", projectID)
func (w *whereBuilder) Add(condition string, arg any) {
	pos := len(w.args) + 1
	w.conditions = append(w.conditions, fmt.Sprintf(condition, pos))
	w.args = append(w.args, arg)
}

// Build returns the WHERE clause string and the accumulated arguments.
// Returns empty string and nil args if no conditions were added.
func (w *whereBuilder) Build() (string, []any) {
	if len(w.conditions) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(w.conditions, " AND "), w.args
}

// paginationClause returns a LIMIT/OFFSET clause and appends the args.
func paginationClause(limit, offset int, args []any) (string, []any) {
	if limit <= 0 {
		limit = 20
	}
	startPos := len(args) + 1
	clause := fmt.Sprintf(" LIMIT $%d OFFSET $%d", startPos, startPos+1)
	args = append(args, limit, offset)
	return clause, args
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/platform/database/postgres/helpers.go
git commit -m "feat: add whereBuilder and pagination helpers"
```

---

### Task 4: Flag Cache Adapter

**Files:**
- Create: `internal/platform/cache/flagcache/flagcache.go`
- Create: `internal/platform/cache/flagcache/flagcache_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/platform/cache/flagcache/flagcache_test.go
package flagcache_test

import (
	"context"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/platform/cache/flagcache"
	"github.com/google/uuid"
)

// mockRedis implements the subset of cache.Redis methods that FlagCache needs.
type mockRedis struct {
	store map[string]string
}

func newMockRedis() *mockRedis {
	return &mockRedis{store: make(map[string]string)}
}

func (m *mockRedis) Set(_ context.Context, key string, value interface{}, _ time.Duration) error {
	m.store[key] = value.(string)
	return nil
}

func (m *mockRedis) Get(_ context.Context, key string) (string, error) {
	v, ok := m.store[key]
	if !ok {
		return "", fmt.Errorf("key not found")
	}
	return v, nil
}

func (m *mockRedis) Delete(_ context.Context, keys ...string) error {
	for _, k := range keys {
		delete(m.store, k)
	}
	return nil
}

func TestFlagCache_SetAndGetFlag(t *testing.T) {
	r := newMockRedis()
	fc := flagcache.NewFlagCache(r)
	ctx := context.Background()

	projectID := uuid.New()
	envID := uuid.New()
	flag := &models.FeatureFlag{
		ID:            uuid.New(),
		ProjectID:     projectID,
		EnvironmentID: envID,
		Key:           "test-flag",
		Name:          "Test Flag",
		FlagType:      models.FlagTypeBoolean,
		DefaultValue:  "true",
		Enabled:       true,
	}

	err := fc.SetFlag(ctx, flag, 5*time.Minute)
	if err != nil {
		t.Fatalf("SetFlag: %v", err)
	}

	got, err := fc.GetFlag(ctx, projectID, envID, "test-flag")
	if err != nil {
		t.Fatalf("GetFlag: %v", err)
	}
	if got.Key != "test-flag" {
		t.Errorf("got key %q, want %q", got.Key, "test-flag")
	}
}

func TestFlagCache_GetFlag_NotFound(t *testing.T) {
	r := newMockRedis()
	fc := flagcache.NewFlagCache(r)
	ctx := context.Background()

	got, err := fc.GetFlag(ctx, uuid.New(), uuid.New(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for missing flag")
	}
}

func TestFlagCache_Invalidate(t *testing.T) {
	r := newMockRedis()
	fc := flagcache.NewFlagCache(r)
	ctx := context.Background()

	projectID := uuid.New()
	envID := uuid.New()
	flagID := uuid.New()
	flag := &models.FeatureFlag{
		ID:            flagID,
		ProjectID:     projectID,
		EnvironmentID: envID,
		Key:           "to-invalidate",
		FlagType:      models.FlagTypeBoolean,
		DefaultValue:  "false",
	}

	_ = fc.SetFlag(ctx, flag, 5*time.Minute)
	err := fc.Invalidate(ctx, flagID)
	if err != nil {
		t.Fatalf("Invalidate: %v", err)
	}

	got, _ := fc.GetFlag(ctx, projectID, envID, "to-invalidate")
	if got != nil {
		t.Error("flag should be nil after invalidation")
	}
}
```

- [ ] **Step 2: Implement the cache adapter**

```go
// internal/platform/cache/flagcache/flagcache.go
package flagcache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// RedisClient is the subset of cache.Redis methods that FlagCache needs.
// Using an interface here avoids importing the full cache package and makes testing easy.
type RedisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, keys ...string) error
}

// FlagCache implements the flags.Cache interface using Redis as the backing store.
type FlagCache struct {
	redis RedisClient
}

// NewFlagCache creates a new FlagCache wrapping the given Redis client.
func NewFlagCache(redis RedisClient) *FlagCache {
	return &FlagCache{redis: redis}
}

func flagKey(projectID, environmentID uuid.UUID, key string) string {
	return fmt.Sprintf("flag:%s:%s:%s", projectID, environmentID, key)
}

func flagIDKey(flagID uuid.UUID) string {
	return fmt.Sprintf("flag:id:%s", flagID)
}

func rulesKey(flagID uuid.UUID) string {
	return fmt.Sprintf("rules:%s", flagID)
}

// GetFlag returns a cached flag, or nil if not found.
func (c *FlagCache) GetFlag(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*models.FeatureFlag, error) {
	data, err := c.redis.Get(ctx, flagKey(projectID, environmentID, key))
	if err != nil {
		return nil, nil // cache miss
	}
	var flag models.FeatureFlag
	if err := json.Unmarshal([]byte(data), &flag); err != nil {
		return nil, fmt.Errorf("flagcache.GetFlag unmarshal: %w", err)
	}
	return &flag, nil
}

// SetFlag stores a flag in the cache with a TTL. It stores under both the
// composite key (for lookup by key) and the ID key (for invalidation).
func (c *FlagCache) SetFlag(ctx context.Context, flag *models.FeatureFlag, ttl time.Duration) error {
	data, err := json.Marshal(flag)
	if err != nil {
		return fmt.Errorf("flagcache.SetFlag marshal: %w", err)
	}
	s := string(data)
	if err := c.redis.Set(ctx, flagKey(flag.ProjectID, flag.EnvironmentID, flag.Key), s, ttl); err != nil {
		return fmt.Errorf("flagcache.SetFlag: %w", err)
	}
	// Also store a reverse mapping from flag ID to cache key for invalidation.
	meta := fmt.Sprintf("%s:%s:%s", flag.ProjectID, flag.EnvironmentID, flag.Key)
	if err := c.redis.Set(ctx, flagIDKey(flag.ID), meta, ttl); err != nil {
		return fmt.Errorf("flagcache.SetFlag id mapping: %w", err)
	}
	return nil
}

// GetRules returns cached targeting rules for a flag, or nil if not found.
func (c *FlagCache) GetRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error) {
	data, err := c.redis.Get(ctx, rulesKey(flagID))
	if err != nil {
		return nil, nil // cache miss
	}
	var rules []*models.TargetingRule
	if err := json.Unmarshal([]byte(data), &rules); err != nil {
		return nil, fmt.Errorf("flagcache.GetRules unmarshal: %w", err)
	}
	return rules, nil
}

// SetRules stores targeting rules in the cache with a TTL.
func (c *FlagCache) SetRules(ctx context.Context, flagID uuid.UUID, rules []*models.TargetingRule, ttl time.Duration) error {
	data, err := json.Marshal(rules)
	if err != nil {
		return fmt.Errorf("flagcache.SetRules marshal: %w", err)
	}
	if err := c.redis.Set(ctx, rulesKey(flagID), string(data), ttl); err != nil {
		return fmt.Errorf("flagcache.SetRules: %w", err)
	}
	return nil
}

// Invalidate removes a flag and its rules from the cache.
func (c *FlagCache) Invalidate(ctx context.Context, flagID uuid.UUID) error {
	// Look up the composite key via the ID mapping.
	meta, err := c.redis.Get(ctx, flagIDKey(flagID))
	if err == nil && meta != "" {
		_ = c.redis.Delete(ctx, "flag:"+meta)
	}
	_ = c.redis.Delete(ctx, flagIDKey(flagID), rulesKey(flagID))
	return nil
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/platform/cache/flagcache/... -v`
Expected: All tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/platform/cache/flagcache/
git commit -m "feat: add flag cache adapter implementing flags.Cache over Redis"
```

---

### Task 5: UserRepository + CreateUser Interface Change

**Files:**
- Modify: `internal/auth/user_handler.go` (add CreateUser to interface)
- Create: `internal/platform/database/postgres/users.go`
- Create: `internal/platform/database/postgres/users_test.go`

- [ ] **Step 1: Add CreateUser to UserRepository interface**

In `internal/auth/user_handler.go`, add to the `UserRepository` interface:

```go
// CreateUser persists a new user record.
CreateUser(ctx context.Context, user *models.User) error
```

- [ ] **Step 2: Create the UserRepository implementation**

```go
// internal/platform/database/postgres/users.go
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepository implements auth.UserRepository using PostgreSQL.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a new PostgreSQL-backed user repository.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func scanUser(row pgx.Row) (*models.User, error) {
	var u models.User
	err := row.Scan(
		&u.ID, &u.Email, &u.Name, &u.AvatarURL,
		&u.AuthProvider, &u.ProviderID, &u.PasswordHash,
		&u.EmailVerified, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

const userColumns = `id, email, name, COALESCE(avatar_url, ''), auth_provider,
	COALESCE(auth_provider_id, ''), COALESCE(password_hash, ''),
	COALESCE(email_verified, false), last_login_at, created_at,
	COALESCE(updated_at, created_at)`

func (r *UserRepository) CreateUser(ctx context.Context, user *models.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (id, email, name, avatar_url, auth_provider, auth_provider_id, password_hash, email_verified)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		user.ID, user.Email, user.Name, user.AvatarURL,
		user.AuthProvider, user.ProviderID, user.PasswordHash, user.EmailVerified,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateUser: %w", err)
	}
	return nil
}

func (r *UserRepository) GetUser(ctx context.Context, id uuid.UUID) (*models.User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
	u, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetUser: %w", err)
	}
	return u, nil
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, email)
	u, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetUserByEmail: %w", err)
	}
	return u, nil
}

func (r *UserRepository) UpdateUser(ctx context.Context, user *models.User) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET name = $2, avatar_url = $3, email_verified = $4,
		       last_login_at = $5, updated_at = now()
		WHERE id = $1`,
		user.ID, user.Name, user.AvatarURL, user.EmailVerified, user.LastLoginAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateUser: %w", err)
	}
	return nil
}

func (r *UserRepository) DeleteUser(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres.DeleteUser: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Org Members ---

func scanOrgMember(row pgx.Row) (*models.OrgMember, error) {
	var m models.OrgMember
	err := row.Scan(
		&m.ID, &m.OrgID, &m.UserID, &m.Role, &m.InvitedBy,
		&m.JoinedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

const orgMemberColumns = `COALESCE(id, gen_random_uuid()), org_id, user_id, role,
	invited_by, joined_at, COALESCE(created_at, joined_at), COALESCE(updated_at, joined_at)`

func (r *UserRepository) ListOrgMembers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.OrgMember, error) {
	pag, args := paginationClause(limit, offset, []any{orgID})
	rows, err := r.pool.Query(ctx, `SELECT `+orgMemberColumns+` FROM org_members WHERE org_id = $1 ORDER BY joined_at`+pag, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListOrgMembers: %w", err)
	}
	defer rows.Close()

	var members []*models.OrgMember
	for rows.Next() {
		m, err := scanOrgMember(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListOrgMembers scan: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *UserRepository) GetOrgMember(ctx context.Context, orgID, userID uuid.UUID) (*models.OrgMember, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+orgMemberColumns+` FROM org_members WHERE org_id = $1 AND user_id = $2`, orgID, userID)
	m, err := scanOrgMember(row)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetOrgMember: %w", err)
	}
	return m, nil
}

func (r *UserRepository) CreateOrgMember(ctx context.Context, member *models.OrgMember) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO org_members (org_id, user_id, role, invited_by)
		VALUES ($1, $2, $3, $4)`,
		member.OrgID, member.UserID, member.Role, member.InvitedBy,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateOrgMember: %w", err)
	}
	return nil
}

func (r *UserRepository) UpdateOrgMember(ctx context.Context, member *models.OrgMember) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE org_members SET role = $3, updated_at = now()
		WHERE org_id = $1 AND user_id = $2`,
		member.OrgID, member.UserID, member.Role,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateOrgMember: %w", err)
	}
	return nil
}

func (r *UserRepository) DeleteOrgMember(ctx context.Context, orgID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM org_members WHERE org_id = $1 AND user_id = $2`, orgID, userID)
	if err != nil {
		return fmt.Errorf("postgres.DeleteOrgMember: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Project Members ---

func scanProjectMember(row pgx.Row) (*models.ProjectMember, error) {
	var m models.ProjectMember
	err := row.Scan(&m.ID, &m.ProjectID, &m.UserID, &m.Role, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

const projectMemberColumns = `COALESCE(id, gen_random_uuid()), project_id, user_id, role,
	COALESCE(created_at, now()), COALESCE(updated_at, now())`

func (r *UserRepository) ListProjectMembers(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]*models.ProjectMember, error) {
	pag, args := paginationClause(limit, offset, []any{projectID})
	rows, err := r.pool.Query(ctx, `SELECT `+projectMemberColumns+` FROM project_members WHERE project_id = $1`+pag, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListProjectMembers: %w", err)
	}
	defer rows.Close()

	var members []*models.ProjectMember
	for rows.Next() {
		m, err := scanProjectMember(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListProjectMembers scan: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *UserRepository) GetProjectMember(ctx context.Context, projectID, userID uuid.UUID) (*models.ProjectMember, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+projectMemberColumns+` FROM project_members WHERE project_id = $1 AND user_id = $2`, projectID, userID)
	m, err := scanProjectMember(row)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetProjectMember: %w", err)
	}
	return m, nil
}

func (r *UserRepository) CreateProjectMember(ctx context.Context, member *models.ProjectMember) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO project_members (project_id, user_id, role)
		VALUES ($1, $2, $3)`,
		member.ProjectID, member.UserID, member.Role,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateProjectMember: %w", err)
	}
	return nil
}

func (r *UserRepository) UpdateProjectMember(ctx context.Context, member *models.ProjectMember) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE project_members SET role = $3, updated_at = now()
		WHERE project_id = $1 AND user_id = $2`,
		member.ProjectID, member.UserID, member.Role,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateProjectMember: %w", err)
	}
	return nil
}

func (r *UserRepository) DeleteProjectMember(ctx context.Context, projectID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`, projectID, userID)
	if err != nil {
		return fmt.Errorf("postgres.DeleteProjectMember: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./internal/platform/database/postgres/...`
Expected: Compiles without errors

- [ ] **Step 4: Commit**

```bash
git add internal/auth/user_handler.go internal/platform/database/postgres/users.go
git commit -m "feat: implement UserRepository with all 15 methods"
```

---

### Task 6: APIKeyRepository

**Files:**
- Create: `internal/platform/database/postgres/apikeys.go`

- [ ] **Step 1: Implement APIKeyRepository**

```go
// internal/platform/database/postgres/apikeys.go
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// APIKeyRepository implements auth.APIKeyRepository using PostgreSQL.
type APIKeyRepository struct {
	pool *pgxpool.Pool
}

// NewAPIKeyRepository creates a new PostgreSQL-backed API key repository.
func NewAPIKeyRepository(pool *pgxpool.Pool) *APIKeyRepository {
	return &APIKeyRepository{pool: pool}
}

func scanAPIKey(row pgx.Row) (*models.APIKey, error) {
	var k models.APIKey
	err := row.Scan(
		&k.ID, &k.OrgID, &k.ProjectID, &k.Name, &k.KeyPrefix,
		&k.KeyHash, &k.Scopes, &k.ExpiresAt, &k.LastUsedAt,
		&k.CreatedBy, &k.CreatedAt, &k.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

const apiKeyColumns = `id, org_id, project_id, name, key_prefix, key_hash,
	scopes, expires_at, last_used_at, created_by, created_at, revoked_at`

func (r *APIKeyRepository) CreateAPIKey(ctx context.Context, key *models.APIKey) error {
	if key.ID == uuid.Nil {
		key.ID = uuid.New()
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO api_keys (id, org_id, project_id, name, key_prefix, key_hash, scopes, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		key.ID, key.OrgID, key.ProjectID, key.Name, key.KeyPrefix,
		key.KeyHash, key.Scopes, key.ExpiresAt, key.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateAPIKey: %w", err)
	}
	return nil
}

func (r *APIKeyRepository) GetAPIKey(ctx context.Context, id uuid.UUID) (*models.APIKey, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+apiKeyColumns+` FROM api_keys WHERE id = $1`, id)
	k, err := scanAPIKey(row)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetAPIKey: %w", err)
	}
	return k, nil
}

func (r *APIKeyRepository) GetAPIKeyByPrefix(ctx context.Context, prefix string) (*models.APIKey, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+apiKeyColumns+` FROM api_keys WHERE key_prefix = $1 AND revoked_at IS NULL`, prefix)
	k, err := scanAPIKey(row)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetAPIKeyByPrefix: %w", err)
	}
	return k, nil
}

func (r *APIKeyRepository) ListAPIKeys(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, limit, offset int) ([]*models.APIKey, error) {
	var w whereBuilder
	w.Add("org_id = $%d", orgID)
	if projectID != nil {
		w.Add("project_id = $%d", *projectID)
	}
	where, args := w.Build()
	pag, args := paginationClause(limit, offset, args)

	rows, err := r.pool.Query(ctx, `SELECT `+apiKeyColumns+` FROM api_keys`+where+` ORDER BY created_at DESC`+pag, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListAPIKeys: %w", err)
	}
	defer rows.Close()

	var keys []*models.APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListAPIKeys scan: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (r *APIKeyRepository) UpdateAPIKey(ctx context.Context, key *models.APIKey) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE api_keys SET name = $2, scopes = $3, expires_at = $4
		WHERE id = $1`,
		key.ID, key.Name, key.Scopes, key.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateAPIKey: %w", err)
	}
	return nil
}

func (r *APIKeyRepository) DeleteAPIKey(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM api_keys WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres.DeleteAPIKey: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *APIKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE api_keys SET last_used_at = $2 WHERE id = $1`, id, usedAt)
	if err != nil {
		return fmt.Errorf("postgres.UpdateLastUsed: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./internal/platform/database/postgres/...`
Expected: Compiles without errors

- [ ] **Step 3: Commit**

```bash
git add internal/platform/database/postgres/apikeys.go
git commit -m "feat: implement APIKeyRepository with all 7 methods"
```

---

### Task 7: AuditLogRepository

**Files:**
- Create: `internal/platform/database/postgres/audit.go`

- [ ] **Step 1: Implement AuditLogRepository**

```go
// internal/platform/database/postgres/audit.go
package postgres

import (
	"context"
	"fmt"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditLogRepository implements auth.AuditLogRepository using PostgreSQL.
type AuditLogRepository struct {
	pool *pgxpool.Pool
}

// NewAuditLogRepository creates a new PostgreSQL-backed audit log repository.
func NewAuditLogRepository(pool *pgxpool.Pool) *AuditLogRepository {
	return &AuditLogRepository{pool: pool}
}

func (r *AuditLogRepository) QueryAuditLogs(ctx context.Context, filter auth.AuditLogFilter) ([]*models.AuditLogEntry, int, error) {
	var w whereBuilder
	w.Add("org_id = $%d", filter.OrgID)
	if filter.ProjectID != nil {
		w.Add("project_id = $%d", *filter.ProjectID)
	}
	if filter.UserID != nil {
		w.Add("user_id = $%d", *filter.UserID)
	}
	if filter.Action != "" {
		w.Add("action = $%d", filter.Action)
	}
	if filter.ResourceType != "" {
		w.Add("resource_type = $%d", filter.ResourceType)
	}
	if filter.StartDate != nil {
		w.Add("created_at >= $%d", *filter.StartDate)
	}
	if filter.EndDate != nil {
		w.Add("created_at <= $%d", *filter.EndDate)
	}

	where, args := w.Build()

	// Count total matching rows.
	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log`+where, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres.QueryAuditLogs count: %w", err)
	}

	pag, args := paginationClause(filter.Limit, filter.Offset, args)
	rows, err := r.pool.Query(ctx, `
		SELECT id, org_id, project_id, user_id, action, resource_type, resource_id,
		       old_value, new_value, COALESCE(ip_address::TEXT, ''), COALESCE(user_agent, ''), created_at
		FROM audit_log`+where+` ORDER BY created_at DESC`+pag, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres.QueryAuditLogs: %w", err)
	}
	defer rows.Close()

	var entries []*models.AuditLogEntry
	for rows.Next() {
		var e models.AuditLogEntry
		if err := rows.Scan(
			&e.ID, &e.OrgID, &e.ProjectID, &e.ActorID, &e.Action,
			&e.EntityType, &e.EntityID, &e.OldValue, &e.NewValue,
			&e.IPAddress, &e.UserAgent, &e.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("postgres.QueryAuditLogs scan: %w", err)
		}
		entries = append(entries, &e)
	}
	return entries, total, rows.Err()
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/platform/database/postgres/audit.go
git commit -m "feat: implement AuditLogRepository"
```

---

### Task 8: FlagRepository

**Files:**
- Create: `internal/platform/database/postgres/flags.go`

- [ ] **Step 1: Implement FlagRepository (12 methods)**

```go
// internal/platform/database/postgres/flags.go
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/flags"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FlagRepository implements flags.FlagRepository using PostgreSQL.
type FlagRepository struct {
	pool *pgxpool.Pool
}

// NewFlagRepository creates a new PostgreSQL-backed flag repository.
func NewFlagRepository(pool *pgxpool.Pool) *FlagRepository {
	return &FlagRepository{pool: pool}
}

const flagColumns = `id, project_id, environment_id, key, name, description,
	flag_type, category, COALESCE(purpose, ''), owners, is_permanent, expires_at,
	enabled, default_value, (archived_at IS NOT NULL), tags, created_by, created_at, updated_at`

func scanFlag(row pgx.Row) (*models.FeatureFlag, error) {
	var f models.FeatureFlag
	var defaultVal []byte // JSONB comes as bytes
	err := row.Scan(
		&f.ID, &f.ProjectID, &f.EnvironmentID, &f.Key, &f.Name, &f.Description,
		&f.FlagType, &f.Category, &f.Purpose, &f.Owners, &f.IsPermanent, &f.ExpiresAt,
		&f.Enabled, &defaultVal, &f.Archived, &f.Tags, &f.CreatedBy, &f.CreatedAt, &f.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// JSONB default_value → string. Strip quotes if it's a JSON string.
	f.DefaultValue = string(defaultVal)
	return &f, nil
}

func scanFlags(rows pgx.Rows) ([]*models.FeatureFlag, error) {
	var result []*models.FeatureFlag
	for rows.Next() {
		f, err := scanFlag(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, rows.Err()
}

func (r *FlagRepository) CreateFlag(ctx context.Context, flag *models.FeatureFlag) error {
	if flag.ID == uuid.Nil {
		flag.ID = uuid.New()
	}
	now := time.Now()
	flag.CreatedAt = now
	flag.UpdatedAt = now

	_, err := r.pool.Exec(ctx, `
		INSERT INTO feature_flags (id, project_id, environment_id, key, name, description,
		    flag_type, category, purpose, owners, is_permanent, expires_at,
		    enabled, default_value, tags, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		flag.ID, flag.ProjectID, flag.EnvironmentID, flag.Key, flag.Name, flag.Description,
		flag.FlagType, flag.Category, flag.Purpose, flag.Owners, flag.IsPermanent, flag.ExpiresAt,
		flag.Enabled, flag.DefaultValue, flag.Tags, flag.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateFlag: %w", err)
	}
	return nil
}

func (r *FlagRepository) GetFlag(ctx context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+flagColumns+` FROM feature_flags WHERE id = $1`, id)
	f, err := scanFlag(row)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetFlag: %w", err)
	}
	return f, nil
}

func (r *FlagRepository) GetFlagByKey(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*models.FeatureFlag, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+flagColumns+` FROM feature_flags WHERE project_id = $1 AND environment_id = $2 AND key = $3`,
		projectID, environmentID, key)
	f, err := scanFlag(row)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetFlagByKey: %w", err)
	}
	return f, nil
}

func (r *FlagRepository) ListFlags(ctx context.Context, projectID uuid.UUID, opts flags.ListOptions) ([]*models.FeatureFlag, error) {
	var w whereBuilder
	w.Add("project_id = $%d", projectID)
	if opts.EnvironmentID != nil {
		w.Add("environment_id = $%d", *opts.EnvironmentID)
	}
	if opts.Archived != nil {
		if *opts.Archived {
			w.Add("archived_at IS NOT NULL AND 1 = $%d", 1)
		} else {
			w.Add("archived_at IS NULL AND 1 = $%d", 1)
		}
	}
	if opts.Tag != "" {
		w.Add("$%d = ANY(tags)", opts.Tag)
	}

	where, args := w.Build()
	pag, args := paginationClause(opts.Limit, opts.Offset, args)

	rows, err := r.pool.Query(ctx, `SELECT `+flagColumns+` FROM feature_flags`+where+` ORDER BY created_at DESC`+pag, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListFlags: %w", err)
	}
	defer rows.Close()
	return scanFlags(rows)
}

func (r *FlagRepository) UpdateFlag(ctx context.Context, flag *models.FeatureFlag) error {
	flag.UpdatedAt = time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE feature_flags SET name = $2, description = $3, category = $4, purpose = $5,
		    owners = $6, is_permanent = $7, expires_at = $8, enabled = $9,
		    default_value = $10, tags = $11, updated_at = $12
		WHERE id = $1`,
		flag.ID, flag.Name, flag.Description, flag.Category, flag.Purpose,
		flag.Owners, flag.IsPermanent, flag.ExpiresAt, flag.Enabled,
		flag.DefaultValue, flag.Tags, flag.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateFlag: %w", err)
	}
	return nil
}

func (r *FlagRepository) DeleteFlag(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE feature_flags SET archived_at = now(), updated_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres.DeleteFlag: %w", err)
	}
	return nil
}

// --- Targeting Rules ---

const ruleColumns = `id, flag_id, rule_type, priority, COALESCE(value, ''),
	percentage, COALESCE(attribute, ''), COALESCE(operator, ''),
	COALESCE(target_values, '{}'), segment_id, start_time, end_time,
	enabled, created_at, COALESCE(updated_at, created_at)`

func scanRule(row pgx.Row) (*models.TargetingRule, error) {
	var r models.TargetingRule
	err := row.Scan(
		&r.ID, &r.FlagID, &r.RuleType, &r.Priority, &r.Value,
		&r.Percentage, &r.Attribute, &r.Operator,
		&r.TargetValues, &r.SegmentID, &r.StartTime, &r.EndTime,
		&r.Enabled, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (r *FlagRepository) CreateRule(ctx context.Context, rule *models.TargetingRule) error {
	if rule.ID == uuid.Nil {
		rule.ID = uuid.New()
	}
	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	_, err := r.pool.Exec(ctx, `
		INSERT INTO flag_targeting_rules (id, flag_id, environment, priority, rule_type,
		    conditions, serve_value, enabled, value, percentage, attribute, operator,
		    target_values, segment_id, start_time, end_time)
		VALUES ($1, $2, '', $3, $4, '{}', '{}', $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		rule.ID, rule.FlagID, rule.Priority, rule.RuleType,
		rule.Enabled, rule.Value, rule.Percentage, rule.Attribute, rule.Operator,
		rule.TargetValues, rule.SegmentID, rule.StartTime, rule.EndTime,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateRule: %w", err)
	}
	return nil
}

func (r *FlagRepository) GetRule(ctx context.Context, id uuid.UUID) (*models.TargetingRule, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+ruleColumns+` FROM flag_targeting_rules WHERE id = $1`, id)
	rule, err := scanRule(row)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetRule: %w", err)
	}
	return rule, nil
}

func (r *FlagRepository) ListRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+ruleColumns+` FROM flag_targeting_rules WHERE flag_id = $1 ORDER BY priority`, flagID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListRules: %w", err)
	}
	defer rows.Close()

	var rules []*models.TargetingRule
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListRules scan: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (r *FlagRepository) UpdateRule(ctx context.Context, rule *models.TargetingRule) error {
	rule.UpdatedAt = time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE flag_targeting_rules SET rule_type = $2, priority = $3, value = $4,
		    percentage = $5, attribute = $6, operator = $7, target_values = $8,
		    segment_id = $9, start_time = $10, end_time = $11, enabled = $12, updated_at = $13
		WHERE id = $1`,
		rule.ID, rule.RuleType, rule.Priority, rule.Value,
		rule.Percentage, rule.Attribute, rule.Operator, rule.TargetValues,
		rule.SegmentID, rule.StartTime, rule.EndTime, rule.Enabled, rule.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateRule: %w", err)
	}
	return nil
}

func (r *FlagRepository) DeleteRule(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM flag_targeting_rules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres.DeleteRule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *FlagRepository) WriteEvaluationLog(ctx context.Context, logs []flags.EvaluationLog) error {
	if len(logs) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, l := range logs {
		batch.Queue(`
			INSERT INTO flag_evaluation_log (id, flag_key, environment, context_hash, result_value, rule_matched, evaluated_at)
			VALUES ($1, $2, '', $3, $4, $5, $6)`,
			l.ID, l.FlagKey, l.EvalCtx.UserID, l.Value, l.RuleID, l.Timestamp,
		)
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range logs {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("postgres.WriteEvaluationLog: %w", err)
		}
	}
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./internal/platform/database/postgres/...`
Expected: Compiles without errors

- [ ] **Step 3: Commit**

```bash
git add internal/platform/database/postgres/flags.go
git commit -m "feat: implement FlagRepository with all 12 methods"
```

---

### Task 9: DeployRepository

**Files:**
- Create: `internal/platform/database/postgres/deploy.go`

- [ ] **Step 1: Implement DeployRepository (9 methods)**

```go
// internal/platform/database/postgres/deploy.go
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/deploy"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DeployRepository implements deploy.DeployRepository using PostgreSQL.
type DeployRepository struct {
	pool *pgxpool.Pool
}

// NewDeployRepository creates a new PostgreSQL-backed deploy repository.
func NewDeployRepository(pool *pgxpool.Pool) *DeployRepository {
	return &DeployRepository{pool: pool}
}

const deployColumns = `id, project_id, environment, pipeline_id, release_id,
	COALESCE(strategy, ''), status, COALESCE(artifact, ''), COALESCE(version, ''),
	COALESCE(commit_sha, ''), traffic_percent, initiated_by,
	started_at, completed_at, created_at, COALESCE(updated_at, created_at)`

func scanDeployment(row pgx.Row) (*models.Deployment, error) {
	var d models.Deployment
	var envStr string
	err := row.Scan(
		&d.ID, &d.ProjectID, &envStr, &d.PipelineID, &d.ReleaseID,
		&d.Strategy, &d.Status, &d.Artifact, &d.Version,
		&d.CommitSHA, &d.TrafficPercent, &d.CreatedBy,
		&d.StartedAt, &d.CompletedAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// EnvironmentID is UUID in the model but stored as TEXT in the DB.
	// Parse it if it looks like a UUID, otherwise leave as Nil.
	if parsed, parseErr := uuid.Parse(envStr); parseErr == nil {
		d.EnvironmentID = parsed
	}
	return &d, nil
}

func (r *DeployRepository) CreateDeployment(ctx context.Context, d *models.Deployment) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	now := time.Now()
	d.CreatedAt = now
	d.UpdatedAt = now

	_, err := r.pool.Exec(ctx, `
		INSERT INTO deployments (id, project_id, environment, pipeline_id, release_id,
		    strategy, status, artifact, version, commit_sha, traffic_percent, initiated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		d.ID, d.ProjectID, d.EnvironmentID.String(), d.PipelineID, d.ReleaseID,
		d.Strategy, d.Status, d.Artifact, d.Version, d.CommitSHA,
		d.TrafficPercent, d.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateDeployment: %w", err)
	}
	return nil
}

func (r *DeployRepository) GetDeployment(ctx context.Context, id uuid.UUID) (*models.Deployment, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+deployColumns+` FROM deployments WHERE id = $1`, id)
	d, err := scanDeployment(row)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetDeployment: %w", err)
	}
	return d, nil
}

func (r *DeployRepository) ListDeployments(ctx context.Context, projectID uuid.UUID, opts deploy.ListOptions) ([]*models.Deployment, error) {
	var w whereBuilder
	w.Add("project_id = $%d", projectID)
	if opts.EnvironmentID != nil {
		w.Add("environment = $%d", opts.EnvironmentID.String())
	}
	if opts.Status != nil {
		w.Add("status = $%d", string(*opts.Status))
	}

	where, args := w.Build()
	pag, args := paginationClause(opts.Limit, opts.Offset, args)

	rows, err := r.pool.Query(ctx, `SELECT `+deployColumns+` FROM deployments`+where+` ORDER BY created_at DESC`+pag, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListDeployments: %w", err)
	}
	defer rows.Close()

	var result []*models.Deployment
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListDeployments scan: %w", err)
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (r *DeployRepository) UpdateDeployment(ctx context.Context, d *models.Deployment) error {
	d.UpdatedAt = time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE deployments SET status = $2, traffic_percent = $3,
		    started_at = $4, completed_at = $5, updated_at = $6
		WHERE id = $1`,
		d.ID, d.Status, d.TrafficPercent, d.StartedAt, d.CompletedAt, d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateDeployment: %w", err)
	}
	return nil
}

func (r *DeployRepository) GetLatestDeployment(ctx context.Context, projectID, environmentID uuid.UUID) (*models.Deployment, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+deployColumns+` FROM deployments WHERE project_id = $1 AND environment = $2 ORDER BY created_at DESC LIMIT 1`,
		projectID, environmentID.String())
	d, err := scanDeployment(row)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetLatestDeployment: %w", err)
	}
	return d, nil
}

func (r *DeployRepository) GetPipeline(ctx context.Context, id uuid.UUID) (*models.DeployPipeline, error) {
	var p models.DeployPipeline
	err := r.pool.QueryRow(ctx,
		`SELECT id, project_id, name, COALESCE(strategy, ''), created_at, updated_at FROM deploy_pipelines WHERE id = $1`, id,
	).Scan(&p.ID, &p.ProjectID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("postgres.GetPipeline: %w", err)
	}
	return &p, nil
}

// --- Deployment Phases ---

const phaseColumns = `id, deployment_id, COALESCE(name, ''), status,
	traffic_pct, duration_secs, COALESCE(sort_order, phase_number),
	started_at, completed_at`

func scanPhase(row pgx.Row) (*models.DeploymentPhase, error) {
	var p models.DeploymentPhase
	err := row.Scan(
		&p.ID, &p.DeploymentID, &p.Name, &p.Status,
		&p.TrafficPercent, &p.Duration, &p.SortOrder,
		&p.StartedAt, &p.CompletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *DeployRepository) ListDeploymentPhases(ctx context.Context, deploymentID uuid.UUID) ([]*models.DeploymentPhase, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+phaseColumns+` FROM deployment_phases WHERE deployment_id = $1 ORDER BY sort_order`, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListDeploymentPhases: %w", err)
	}
	defer rows.Close()

	var phases []*models.DeploymentPhase
	for rows.Next() {
		p, err := scanPhase(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListDeploymentPhases scan: %w", err)
		}
		phases = append(phases, p)
	}
	return phases, rows.Err()
}

func (r *DeployRepository) CreateDeploymentPhase(ctx context.Context, phase *models.DeploymentPhase) error {
	if phase.ID == uuid.Nil {
		phase.ID = uuid.New()
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO deployment_phases (id, deployment_id, phase_number, name, traffic_pct, duration_secs, status, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		phase.ID, phase.DeploymentID, phase.SortOrder, phase.Name,
		phase.TrafficPercent, phase.Duration, phase.Status, phase.SortOrder,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateDeploymentPhase: %w", err)
	}
	return nil
}

func (r *DeployRepository) UpdateDeploymentPhase(ctx context.Context, phase *models.DeploymentPhase) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE deployment_phases SET status = $2, started_at = $3, completed_at = $4
		WHERE id = $1`,
		phase.ID, phase.Status, phase.StartedAt, phase.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateDeploymentPhase: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./internal/platform/database/postgres/...`

- [ ] **Step 3: Commit**

```bash
git add internal/platform/database/postgres/deploy.go
git commit -m "feat: implement DeployRepository with all 9 methods"
```

---

### Task 10: ReleaseRepository

**Files:**
- Create: `internal/platform/database/postgres/releases.go`

- [ ] **Step 1: Implement ReleaseRepository (9 methods)**

```go
// internal/platform/database/postgres/releases.go
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/releases"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReleaseRepository implements releases.ReleaseRepository using PostgreSQL.
type ReleaseRepository struct {
	pool *pgxpool.Pool
}

// NewReleaseRepository creates a new PostgreSQL-backed release repository.
func NewReleaseRepository(pool *pgxpool.Pool) *ReleaseRepository {
	return &ReleaseRepository{pool: pool}
}

const releaseColumns = `id, project_id, version, COALESCE(title, ''),
	COALESCE(description, ''), COALESCE(commit_sha, ''),
	COALESCE(artifact, COALESCE(artifact_url, '')),
	COALESCE(status, ''), COALESCE(lifecycle_status, ''),
	created_by, released_at, created_at, COALESCE(updated_at, created_at)`

func scanRelease(row pgx.Row) (*models.Release, error) {
	var r models.Release
	err := row.Scan(
		&r.ID, &r.ProjectID, &r.Version, &r.Title,
		&r.Description, &r.CommitSHA, &r.Artifact,
		&r.Status, &r.LifecycleStatus,
		&r.CreatedBy, &r.ReleasedAt, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (r *ReleaseRepository) CreateRelease(ctx context.Context, release *models.Release) error {
	if release.ID == uuid.Nil {
		release.ID = uuid.New()
	}
	now := time.Now()
	release.CreatedAt = now
	release.UpdatedAt = now

	_, err := r.pool.Exec(ctx, `
		INSERT INTO releases (id, project_id, version, title, description, commit_sha, artifact, status, lifecycle_status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		release.ID, release.ProjectID, release.Version, release.Title,
		release.Description, release.CommitSHA, release.Artifact,
		release.Status, release.LifecycleStatus, release.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateRelease: %w", err)
	}
	return nil
}

func (r *ReleaseRepository) GetRelease(ctx context.Context, id uuid.UUID) (*models.Release, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+releaseColumns+` FROM releases WHERE id = $1`, id)
	rel, err := scanRelease(row)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetRelease: %w", err)
	}
	return rel, nil
}

func (r *ReleaseRepository) ListReleases(ctx context.Context, projectID uuid.UUID, opts releases.ListOptions) ([]*models.Release, error) {
	var w whereBuilder
	w.Add("project_id = $%d", projectID)
	if opts.Status != nil {
		w.Add("status = $%d", string(*opts.Status))
	}

	where, args := w.Build()
	pag, args := paginationClause(opts.Limit, opts.Offset, args)

	rows, err := r.pool.Query(ctx, `SELECT `+releaseColumns+` FROM releases`+where+` ORDER BY created_at DESC`+pag, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListReleases: %w", err)
	}
	defer rows.Close()

	var result []*models.Release
	for rows.Next() {
		rel, err := scanRelease(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListReleases scan: %w", err)
		}
		result = append(result, rel)
	}
	return result, rows.Err()
}

func (r *ReleaseRepository) UpdateRelease(ctx context.Context, release *models.Release) error {
	release.UpdatedAt = time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE releases SET status = $2, lifecycle_status = $3, released_at = $4, updated_at = $5
		WHERE id = $1`,
		release.ID, release.Status, release.LifecycleStatus, release.ReleasedAt, release.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateRelease: %w", err)
	}
	return nil
}

func (r *ReleaseRepository) GetLatestRelease(ctx context.Context, projectID, environmentID uuid.UUID) (*models.Release, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+releaseColumns+` FROM releases r
		JOIN release_environments re ON re.release_id = r.id
		WHERE r.project_id = $1 AND re.environment_id = $2
		ORDER BY r.created_at DESC LIMIT 1`,
		projectID, environmentID)
	rel, err := scanRelease(row)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetLatestRelease: %w", err)
	}
	return rel, nil
}

// --- Release Environments ---

const releaseEnvColumns = `id, release_id, environment_id, deployment_id,
	COALESCE(status, ''), COALESCE(lifecycle_status, ''),
	COALESCE(health_score, 0), deployed_at, deployed_by,
	COALESCE(created_at, now()), COALESCE(updated_at, now())`

func scanReleaseEnv(row pgx.Row) (*models.ReleaseEnvironment, error) {
	var re models.ReleaseEnvironment
	err := row.Scan(
		&re.ID, &re.ReleaseID, &re.EnvironmentID, &re.DeploymentID,
		&re.Status, &re.LifecycleStatus,
		&re.HealthScore, &re.DeployedAt, &re.DeployedBy,
		&re.CreatedAt, &re.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &re, nil
}

func (r *ReleaseRepository) CreateReleaseEnvironment(ctx context.Context, re *models.ReleaseEnvironment) error {
	if re.ID == uuid.Nil {
		re.ID = uuid.New()
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO release_environments (id, release_id, environment_id, deployment_id, status, lifecycle_status, deployed_at, deployed_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		re.ID, re.ReleaseID, re.EnvironmentID, re.DeploymentID,
		re.Status, re.LifecycleStatus, re.DeployedAt, re.DeployedBy,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateReleaseEnvironment: %w", err)
	}
	return nil
}

func (r *ReleaseRepository) ListReleaseEnvironments(ctx context.Context, releaseID uuid.UUID) ([]*models.ReleaseEnvironment, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+releaseEnvColumns+` FROM release_environments WHERE release_id = $1`, releaseID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListReleaseEnvironments: %w", err)
	}
	defer rows.Close()

	var result []*models.ReleaseEnvironment
	for rows.Next() {
		re, err := scanReleaseEnv(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListReleaseEnvironments scan: %w", err)
		}
		result = append(result, re)
	}
	return result, rows.Err()
}

func (r *ReleaseRepository) UpdateReleaseEnvironment(ctx context.Context, re *models.ReleaseEnvironment) error {
	re.UpdatedAt = time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE release_environments SET status = $2, lifecycle_status = $3,
		    health_score = $4, deployed_at = $5, deployed_by = $6, updated_at = $7
		WHERE id = $1`,
		re.ID, re.Status, re.LifecycleStatus,
		re.HealthScore, re.DeployedAt, re.DeployedBy, re.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateReleaseEnvironment: %w", err)
	}
	return nil
}

func (r *ReleaseRepository) GetReleaseTimeline(ctx context.Context, projectID uuid.UUID) ([]*models.ReleaseTimeline, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.id, r.version, COALESCE(r.title, ''),
		       re.environment_id, COALESCE(re.lifecycle_status, ''),
		       re.deployed_at, COALESCE(re.health_score, 0)
		FROM releases r
		JOIN release_environments re ON re.release_id = r.id
		WHERE r.project_id = $1
		ORDER BY r.created_at DESC, re.created_at`, projectID)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetReleaseTimeline: %w", err)
	}
	defer rows.Close()

	var result []*models.ReleaseTimeline
	for rows.Next() {
		var t models.ReleaseTimeline
		if err := rows.Scan(&t.ReleaseID, &t.Version, &t.Title,
			&t.EnvironmentID, &t.Status, &t.DeployedAt, &t.HealthScore); err != nil {
			return nil, fmt.Errorf("postgres.GetReleaseTimeline scan: %w", err)
		}
		result = append(result, &t)
	}
	return result, rows.Err()
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./internal/platform/database/postgres/...`

- [ ] **Step 3: Commit**

```bash
git add internal/platform/database/postgres/releases.go
git commit -m "feat: implement ReleaseRepository with all 9 methods"
```

---

### Task 11: Extract Shared GenerateJWT Function

**Files:**
- Create: `internal/auth/jwt.go`
- Modify: `internal/auth/oauth.go` (update generateJWT to call shared function)

- [ ] **Step 1: Create shared JWT function**

```go
// internal/auth/jwt.go
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// GenerateJWT creates a signed JWT token for the given user. Both OAuthHandler
// and LoginHandler use this shared function.
func GenerateJWT(secret []byte, expiry time.Duration, userID uuid.UUID, email string) (string, error) {
	claims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "deploysentry",
		},
		UserID: userID,
		Email:  email,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}
	return signed, nil
}
```

- [ ] **Step 2: Update OAuthHandler.generateJWT to use shared function**

In `internal/auth/oauth.go`, replace the body of `generateJWT` method:

```go
func (h *OAuthHandler) generateJWT(userID uuid.UUID, email string) (string, error) {
	return GenerateJWT(h.jwtSecret, h.jwtExpiry, userID, email)
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./internal/auth/...`

- [ ] **Step 4: Commit**

```bash
git add internal/auth/jwt.go internal/auth/oauth.go
git commit -m "refactor: extract shared GenerateJWT function from OAuthHandler"
```

---

### Task 12: Email/Password Login Handler

**Files:**
- Create: `internal/auth/login_handler.go`
- Create: `internal/auth/login_handler_test.go`

- [ ] **Step 1: Create the login handler**

```go
// internal/auth/login_handler.go
package auth

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/platform/config"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

// LoginHandler provides email/password registration and login endpoints.
type LoginHandler struct {
	repo   UserRepository
	config config.AuthConfig
}

// NewLoginHandler creates a new login handler.
func NewLoginHandler(repo UserRepository, cfg config.AuthConfig) *LoginHandler {
	return &LoginHandler{repo: repo, config: cfg}
}

// RegisterRoutes mounts login/register routes.
func (h *LoginHandler) RegisterRoutes(rg *gin.RouterGroup) {
	auth := rg.Group("/auth")
	{
		auth.POST("/register", h.register)
		auth.POST("/login", h.login)
	}
}

type registerRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name" binding:"required"`
}

func (h *LoginHandler) register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already exists.
	existing, _ := h.repo.GetUserByEmail(c.Request.Context(), req.Email)
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}

	// Hash password with argon2id.
	hash, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	user := &models.User{
		ID:           uuid.New(),
		Email:        req.Email,
		Name:         req.Name,
		AuthProvider: models.AuthProviderEmail,
		PasswordHash: hash,
	}

	if err := h.repo.CreateUser(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	token, err := GenerateJWT(
		[]byte(h.config.JWTSecret),
		h.config.JWTExpiration,
		user.ID,
		user.Email,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": token,
		"user":  user,
	})
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *LoginHandler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.repo.GetUserByEmail(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if user.PasswordHash == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if !verifyPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Update last login.
	now := time.Now()
	user.LastLoginAt = &now
	_ = h.repo.UpdateUser(c.Request.Context(), user)

	token, err := GenerateJWT(
		[]byte(h.config.JWTSecret),
		h.config.JWTExpiration,
		user.ID,
		user.Email,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  user,
	})
}

// hashPassword uses argon2id with the same parameters as API key hashing.
func hashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	// Encode as hex: salt + hash
	return fmt.Sprintf("%x$%x", salt, hash), nil
}

// verifyPassword checks a password against a stored hash.
func verifyPassword(password, stored string) bool {
	var saltHex, hashHex string
	n, _ := fmt.Sscanf(stored, "%64s", &saltHex)
	if n == 0 {
		return false
	}
	parts := splitOnce(stored, '$')
	if len(parts) != 2 {
		return false
	}
	saltHex = parts[0]
	hashHex = parts[1]

	salt := hexDecode(saltHex)
	expectedHash := hexDecode(hashHex)
	if salt == nil || expectedHash == nil {
		return false
	}

	computed := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return subtleCompare(computed, expectedHash)
}

func splitOnce(s string, sep byte) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

func hexDecode(s string) []byte {
	b := make([]byte, len(s)/2)
	for i := 0; i < len(b); i++ {
		_, err := fmt.Sscanf(s[2*i:2*i+2], "%02x", &b[i])
		if err != nil {
			return nil
		}
	}
	return b
}

func subtleCompare(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := range a {
		result |= a[i] ^ b[i]
	}
	return result == 0
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./internal/auth/...`

- [ ] **Step 3: Commit**

```bash
git add internal/auth/login_handler.go
git commit -m "feat: add email/password login handler with register and login endpoints"
```

---

### Task 13: Add SessionTTL to Config

**Files:**
- Modify: `internal/platform/config/config.go`

- [ ] **Step 1: Add SessionTTL field to AuthConfig**

Add to the `AuthConfig` struct:

```go
SessionTTL time.Duration `mapstructure:"session_ttl"`
```

And in the defaults section, add:

```go
viper.SetDefault("auth.session_ttl", 30*time.Minute)
```

- [ ] **Step 2: Commit**

```bash
git add internal/platform/config/config.go
git commit -m "feat: add SessionTTL config for flag session consistency"
```

---

### Task 14: Wire Routes in main.go

**Files:**
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Add imports and wire everything**

Add these imports to `cmd/api/main.go`:

```go
"github.com/deploysentry/deploysentry/internal/auth"
"github.com/deploysentry/deploysentry/internal/deploy"
"github.com/deploysentry/deploysentry/internal/flags"
"github.com/deploysentry/deploysentry/internal/platform/cache/flagcache"
"github.com/deploysentry/deploysentry/internal/platform/database/postgres"
"github.com/deploysentry/deploysentry/internal/platform/middleware"
"github.com/deploysentry/deploysentry/internal/releases"
```

After the `router.GET("/ready", ...)` block (line 131), add:

```go
// -------------------------------------------------------------------------
// Initialize Repositories
// -------------------------------------------------------------------------
userRepo := postgres.NewUserRepository(db.Pool)
apiKeyRepo := postgres.NewAPIKeyRepository(db.Pool)
auditRepo := postgres.NewAuditLogRepository(db.Pool)
flagRepo := postgres.NewFlagRepository(db.Pool)
deployRepo := postgres.NewDeployRepository(db.Pool)
releaseRepo := postgres.NewReleaseRepository(db.Pool)

// -------------------------------------------------------------------------
// Initialize Services
// -------------------------------------------------------------------------
flagCache := flagcache.NewFlagCache(rdb)
flagService := flags.NewFlagService(flagRepo, flagCache, nc)
deployService := deploy.NewDeployService(deployRepo, nc)
releaseService := releases.NewReleaseServiceWithPublisher(releaseRepo, nc)
apiKeyService := auth.NewAPIKeyService(apiKeyRepo)
rbacChecker := auth.NewRBACChecker()

// -------------------------------------------------------------------------
// Initialize Middleware
// -------------------------------------------------------------------------
authMiddleware := auth.NewAuthMiddleware(cfg.Auth.JWTSecret, apiKeyService)
corsMiddleware := middleware.CORS(middleware.DefaultCORSConfig())
rateLimiter := middleware.NewRateLimiter(rdb.Client, middleware.DefaultRateLimitConfig())

// -------------------------------------------------------------------------
// Register API Routes
// -------------------------------------------------------------------------
api := router.Group("/api/v1")
api.Use(corsMiddleware)
api.Use(rateLimiter.Middleware())
api.Use(authMiddleware.RequireAuth())

flags.NewHandler(flagService, rbacChecker).RegisterRoutes(api)
deploy.NewHandler(deployService).RegisterRoutes(api)
releases.NewHandler(releaseService).RegisterRoutes(api)
auth.NewUserHandler(userRepo).RegisterRoutes(api)
auth.NewAPIKeyHandler(apiKeyService).RegisterRoutes(api)
auth.NewAuditHandler(auditRepo).RegisterRoutes(api)

// Auth routes are public (no RequireAuth middleware).
public := router.Group("/api/v1")
public.Use(corsMiddleware)
auth.NewLoginHandler(userRepo, cfg.Auth).RegisterRoutes(public)

log.Println("API routes registered")
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./cmd/api/...`

- [ ] **Step 3: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat: wire all routes, middleware, repositories, and services in main.go"
```

---

### Task 15: End-to-End Verification

- [ ] **Step 1: Start infrastructure**

Run: `make dev-up`
Expected: PostgreSQL, Redis, NATS containers start

- [ ] **Step 2: Run migrations**

Run: `make migrate-up`
Expected: All migrations (000-023) apply

- [ ] **Step 3: Start the API server**

Run: `make run-api`
Expected: Server starts on :8080, logs "API routes registered"

- [ ] **Step 4: Test health endpoint**

Run: `curl http://localhost:8080/health`
Expected: `{"status":"healthy","checks":{"database":"ok","redis":"ok","nats":"ok"},...}`

- [ ] **Step 5: Test user registration**

Run: `curl -X POST http://localhost:8080/api/v1/auth/register -H 'Content-Type: application/json' -d '{"email":"test@example.com","password":"testpass123","name":"Test User"}'`
Expected: 201 with `{"token":"eyJ...","user":{...}}`

- [ ] **Step 6: Test authenticated flag creation**

Using the token from step 5:

Run: `curl -X POST http://localhost:8080/api/v1/flags -H 'Content-Type: application/json' -H 'Authorization: Bearer <token>' -d '{"project_id":"<uuid>","environment_id":"<uuid>","key":"test-flag","name":"Test Flag","flag_type":"boolean","category":"feature","default_value":"true"}'`

Note: This requires seed data for a project and environment. If no seed data exists, expect a foreign key error — which confirms the route wiring and auth work correctly.

- [ ] **Step 7: Run existing tests**

Run: `make test`
Expected: Existing tests pass (they use mocks, not the new postgres implementations)

- [ ] **Step 8: Commit any fixes**

```bash
git add -A
git commit -m "fix: resolve any issues found during end-to-end verification"
```

---

## Next Plans

This plan covers **Phases 1-2** (backend). The following plans should be created separately:

- **`2026-03-27-web-dashboard-integration.md`** — Phase 3: Login flow, replace mock data, SSE real-time updates
- **`2026-03-27-sdk-production-readiness.md`** — Phase 4: Auth header fixes, React eval methods, SSE backoff, session consistency, contract tests
- **`2026-03-27-documentation-updates.md`** — Phase 5: README API schemas, SSE protocol, session docs
