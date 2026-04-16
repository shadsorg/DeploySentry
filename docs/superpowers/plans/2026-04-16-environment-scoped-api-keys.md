# Environment-Scoped API Keys Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make API keys optionally restricted to specific environments so a production key cannot be used against staging.

**Architecture:** Migrate from single `environment_id UUID` to `environment_ids UUID[]` on the api_keys table, drop the mutually-exclusive CHECK constraint, make `org_id` always required, fix the repository to load the new column, enforce environment restrictions in auth middleware, and update the dashboard/CLI to support creating environment-scoped keys.

**Tech Stack:** Go (Gin, pgxpool), PostgreSQL, React + TypeScript, Cobra CLI

**Spec:** `docs/superpowers/specs/2026-04-16-environment-scoped-api-keys-design.md`

---

## File Map

### New Files

| File | Responsibility |
|------|---------------|
| `migrations/042_environment_scoped_api_keys.up.sql` | Drop CHECK constraint, make org_id NOT NULL, add environment_ids UUID[], migrate data, drop old columns |
| `migrations/042_environment_scoped_api_keys.down.sql` | Reverse migration |

### Modified Files

| File | Change |
|------|--------|
| `internal/models/api_key.go` | Replace `EnvironmentID *uuid.UUID` with `EnvironmentIDs []uuid.UUID`, drop `ApplicationID`, update Validate() |
| `internal/platform/database/postgres/apikeys.go` | Update apiKeySelectCols, scanAPIKey, CreateAPIKey, ListAPIKeys to handle environment_ids array |
| `internal/auth/apikeys.go` | Update GenerateKey signature (envID → envIDs), update APIKeyRepository interface |
| `internal/auth/apikey_handler.go` | Add environment_ids to create request, pass through to service |
| `internal/auth/middleware.go` | Add environment enforcement after API key validation, update APIKeyInfo struct |
| `cmd/cli/apikeys.go` | Add --env flag (repeatable) to create command |
| `web/src/pages/APIKeysPage.tsx` | Replace "coming soon" placeholder with environment multi-select checkboxes |
| `web/src/types.ts` | Update ApiKey interface (environment_targets → environment_ids) |

---

## Task 1: Database Migration

**Files:**
- Create: `migrations/042_environment_scoped_api_keys.up.sql`
- Create: `migrations/042_environment_scoped_api_keys.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- 042_environment_scoped_api_keys.up.sql

-- 1. Drop the exclusive-scope CHECK constraint
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS chk_api_keys_single_scope;

-- 2. Backfill org_id from project where missing
UPDATE api_keys SET org_id = (
    SELECT p.org_id FROM projects p WHERE p.id = api_keys.project_id
) WHERE org_id IS NULL AND project_id IS NOT NULL;

-- 3. Make org_id NOT NULL
ALTER TABLE api_keys ALTER COLUMN org_id SET NOT NULL;

-- 4. Add environment_ids array column
ALTER TABLE api_keys ADD COLUMN environment_ids UUID[] NOT NULL DEFAULT '{}';

-- 5. Migrate existing singular environment_id data
UPDATE api_keys SET environment_ids = ARRAY[environment_id]
WHERE environment_id IS NOT NULL;

-- 6. Drop old columns
ALTER TABLE api_keys DROP COLUMN IF EXISTS environment_id;
ALTER TABLE api_keys DROP COLUMN IF EXISTS application_id;

-- 7. Index for array containment queries
CREATE INDEX idx_api_keys_environment_ids ON api_keys USING GIN (environment_ids);
```

- [ ] **Step 2: Write the down migration**

```sql
-- 042_environment_scoped_api_keys.down.sql

-- 1. Drop the GIN index
DROP INDEX IF EXISTS idx_api_keys_environment_ids;

-- 2. Add back the old columns
ALTER TABLE api_keys ADD COLUMN application_id UUID REFERENCES applications(id) ON DELETE CASCADE;
ALTER TABLE api_keys ADD COLUMN environment_id UUID REFERENCES environments(id) ON DELETE CASCADE;

-- 3. Migrate first element back to singular column
UPDATE api_keys SET environment_id = environment_ids[1]
WHERE array_length(environment_ids, 1) > 0;

-- 4. Drop the array column
ALTER TABLE api_keys DROP COLUMN environment_ids;

-- 5. Make org_id nullable again
ALTER TABLE api_keys ALTER COLUMN org_id DROP NOT NULL;

-- 6. Restore the CHECK constraint
ALTER TABLE api_keys ADD CONSTRAINT chk_api_keys_single_scope
    CHECK (num_nonnulls(org_id, project_id, application_id, environment_id) = 1);

-- 7. Null out org_id where project_id is set (restore old behavior)
UPDATE api_keys SET org_id = NULL WHERE project_id IS NOT NULL;
```

- [ ] **Step 3: Run the migration**

Run: `make migrate-up`
Expected: Migration 042 applied successfully

- [ ] **Step 4: Verify the schema**

Run: `psql "$DATABASE_URL" -c "\d api_keys" | grep -E "environment_ids|application_id|environment_id|org_id"`
Expected: `environment_ids` column present as `uuid[]`, `org_id` is `NOT NULL`, no `environment_id` or `application_id` columns

- [ ] **Step 5: Commit**

```bash
git add migrations/042_environment_scoped_api_keys.up.sql migrations/042_environment_scoped_api_keys.down.sql
git commit -m "feat: migrate api_keys to environment_ids UUID[] array"
```

---

## Task 2: Update the APIKey Model

**Files:**
- Modify: `internal/models/api_key.go`

- [ ] **Step 1: Update the APIKey struct**

Replace the current struct fields. Remove `ApplicationID` and `EnvironmentID`, add `EnvironmentIDs`:

In `internal/models/api_key.go`, change the struct from:

```go
	OrgID         *uuid.UUID    `json:"org_id,omitempty" db:"org_id"`
	ProjectID     *uuid.UUID    `json:"project_id,omitempty" db:"project_id"`
	ApplicationID *uuid.UUID    `json:"application_id,omitempty" db:"application_id"`
	EnvironmentID *uuid.UUID    `json:"environment_id,omitempty" db:"environment_id"`
```

to:

```go
	OrgID          uuid.UUID     `json:"org_id" db:"org_id"`
	ProjectID      *uuid.UUID    `json:"project_id,omitempty" db:"project_id"`
	EnvironmentIDs []uuid.UUID   `json:"environment_ids" db:"environment_ids"`
```

Note: `OrgID` is now non-pointer `uuid.UUID` (always required).

- [ ] **Step 2: Update the Validate() method**

Replace the existing Validate method. The old one enforced exactly-one-of four scope fields. The new one enforces: org_id required, name required, at least one scope:

```go
func (k *APIKey) Validate() error {
	if k.OrgID == uuid.Nil {
		return errors.New("org_id is required")
	}
	if k.Name == "" {
		return errors.New("name is required")
	}
	if len(k.Scopes) == 0 {
		return errors.New("at least one scope is required")
	}
	return nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/models/...`
Expected: Compile errors in callers (expected — we'll fix them in subsequent tasks)

- [ ] **Step 4: Commit**

```bash
git add internal/models/api_key.go
git commit -m "feat: update APIKey model — org_id required, environment_ids array, drop application_id"
```

---

## Task 3: Update Postgres Repository

**Files:**
- Modify: `internal/platform/database/postgres/apikeys.go`

- [ ] **Step 1: Update apiKeySelectCols to include environment_ids**

Replace the current `apiKeySelectCols` constant:

```go
const apiKeySelectCols = `
	id, org_id,
	COALESCE(project_id, '00000000-0000-0000-0000-000000000000'::uuid),
	name, key_prefix, key_hash,
	scopes::text[],
	environment_ids,
	expires_at, last_used_at, created_by, created_at, revoked_at`
```

- [ ] **Step 2: Update scanAPIKey to read environment_ids**

The scan function needs to read the new `environment_ids` column. After scanning the existing fields, add a scan for the UUID array:

```go
func scanAPIKey(row pgx.Row) (*models.APIKey, error) {
	var k models.APIKey
	var scopeStrings []string
	var projectID uuid.UUID

	if err := row.Scan(
		&k.ID, &k.OrgID, &projectID,
		&k.Name, &k.KeyPrefix, &k.KeyHash,
		&scopeStrings,
		&k.EnvironmentIDs,
		&k.ExpiresAt, &k.LastUsedAt, &k.CreatedBy, &k.CreatedAt, &k.RevokedAt,
	); err != nil {
		return nil, err
	}

	if projectID != uuid.Nil {
		k.ProjectID = &projectID
	}

	k.Scopes = make([]models.APIKeyScope, len(scopeStrings))
	for i, s := range scopeStrings {
		k.Scopes[i] = models.APIKeyScope(s)
	}

	if k.EnvironmentIDs == nil {
		k.EnvironmentIDs = []uuid.UUID{}
	}

	return &k, nil
}
```

- [ ] **Step 3: Update CreateAPIKey to insert environment_ids**

Update the INSERT statement to include the `environment_ids` column:

```go
func (r *APIKeyRepository) CreateAPIKey(ctx context.Context, key *models.APIKey) error {
	scopeStrings := make([]string, len(key.Scopes))
	for i, s := range key.Scopes {
		scopeStrings[i] = string(s)
	}

	_, err := r.pool.Exec(ctx,
		`INSERT INTO api_keys (id, org_id, project_id, name, key_prefix, key_hash, scopes, environment_ids, expires_at, last_used_at, created_by, created_at, revoked_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7::text[], $8, $9, $10, $11, $12, $13)`,
		key.ID, key.OrgID, key.ProjectID,
		key.Name, key.KeyPrefix, key.KeyHash,
		scopeStrings,
		key.EnvironmentIDs,
		key.ExpiresAt, key.LastUsedAt, key.CreatedBy, key.CreatedAt, key.RevokedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateAPIKey: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Update ListAPIKeys to support environment_id filter**

Add an optional `environmentID *uuid.UUID` parameter. When set, filter to keys whose `environment_ids` array contains the given UUID (using `@>` operator) or keys with empty `environment_ids` (unrestricted):

```go
func (r *APIKeyRepository) ListAPIKeys(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, environmentID *uuid.UUID, limit, offset int) ([]*models.APIKey, error) {
	q := `SELECT ` + apiKeySelectCols + ` FROM api_keys WHERE org_id = $1 AND revoked_at IS NULL`
	args := []interface{}{orgID}
	argN := 2

	if projectID != nil {
		q += fmt.Sprintf(` AND project_id = $%d`, argN)
		args = append(args, *projectID)
		argN++
	}

	if environmentID != nil {
		q += fmt.Sprintf(` AND (environment_ids = '{}' OR environment_ids @> ARRAY[$%d]::uuid[])`, argN)
		args = append(args, *environmentID)
		argN++
	}

	q += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, argN, argN+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListAPIKeys: %w", err)
	}
	defer rows.Close()

	var result []*models.APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListAPIKeys scan: %w", err)
		}
		result = append(result, k)
	}
	return result, rows.Err()
}
```

- [ ] **Step 5: Verify it compiles**

Run: `go build ./internal/platform/database/postgres/...`
Expected: Compile errors in callers of ListAPIKeys (signature changed) — fixed in next tasks

- [ ] **Step 6: Commit**

```bash
git add internal/platform/database/postgres/apikeys.go
git commit -m "feat: update API key repository — load environment_ids, support filtering"
```

---

## Task 4: Update API Key Service

**Files:**
- Modify: `internal/auth/apikeys.go`

- [ ] **Step 1: Update the APIKeyRepository interface**

Update the `ListAPIKeys` method in the interface to include the new `environmentID` parameter:

```go
ListAPIKeys(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, environmentID *uuid.UUID, limit, offset int) ([]*models.APIKey, error)
```

- [ ] **Step 2: Update GenerateKey signature**

Change `envID *uuid.UUID` parameter to `envIDs []uuid.UUID`:

```go
func (s *APIKeyService) GenerateKey(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, name string, scopes []models.APIKeyScope, createdBy uuid.UUID, envIDs []uuid.UUID, expiresAt *time.Time) (*GenerateKeyResult, error)
```

Note: `orgID` is now non-pointer `uuid.UUID` (always required, matching the model change).

- [ ] **Step 3: Update GenerateKey implementation**

In the body where the `models.APIKey` is constructed, change:

```go
key := &models.APIKey{
	ID:             uuid.New(),
	OrgID:          orgID,
	ProjectID:      projectID,
	Name:           name,
	KeyPrefix:      prefix,
	KeyHash:        hashStr,
	Scopes:         scopes,
	EnvironmentIDs: envIDs,
	CreatedBy:      createdBy,
	CreatedAt:      time.Now(),
	ExpiresAt:      expiresAt,
}
```

If `envIDs` is nil, default it to empty slice:

```go
if envIDs == nil {
	envIDs = []uuid.UUID{}
}
```

- [ ] **Step 4: Update ValidateKey to populate EnvironmentIDs**

`ValidateKey` returns a `*models.APIKey` from the repository. Since the repository now loads `environment_ids`, this should work automatically. Verify the return value is used correctly.

- [ ] **Step 5: Update ListKeys to pass through environmentID**

Find the `ListKeys` method and update its signature and the call to `r.repo.ListAPIKeys` to include the new `environmentID` parameter:

```go
func (s *APIKeyService) ListKeys(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, environmentID *uuid.UUID, limit, offset int) ([]*models.APIKey, error) {
	return s.repo.ListAPIKeys(ctx, orgID, projectID, environmentID, limit, offset)
}
```

- [ ] **Step 6: Update RotateKey**

The `RotateKey` method copies fields from the old key. Update it to copy `EnvironmentIDs` instead of `EnvironmentID`:

Find where the new key is constructed in RotateKey and ensure it copies `EnvironmentIDs`:

```go
newKey.EnvironmentIDs = oldKey.EnvironmentIDs
```

- [ ] **Step 7: Verify it compiles**

Run: `go build ./internal/auth/...`
Expected: Compile errors in handler (signature changed) — fixed in next task

- [ ] **Step 8: Commit**

```bash
git add internal/auth/apikeys.go
git commit -m "feat: update APIKeyService — envIDs []uuid.UUID, org_id required"
```

---

## Task 5: Update API Key Handler

**Files:**
- Modify: `internal/auth/apikey_handler.go`

- [ ] **Step 1: Update createAPIKeyRequest**

Add `EnvironmentIDs` to the request struct:

```go
type createAPIKeyRequest struct {
	Name           string               `json:"name" binding:"required"`
	ProjectID      *uuid.UUID           `json:"project_id"`
	Scopes         []models.APIKeyScope `json:"scopes" binding:"required"`
	EnvironmentIDs []uuid.UUID          `json:"environment_ids"`
	ExpiresAt      *time.Time           `json:"expires_at"`
}
```

- [ ] **Step 2: Update createAPIKey handler**

Pass `req.EnvironmentIDs` to `GenerateKey` instead of `nil`:

Find the `h.service.GenerateKey(...)` call and update:

```go
result, err := h.service.GenerateKey(
	c.Request.Context(),
	orgID,          // now uuid.UUID, not *uuid.UUID
	req.ProjectID,
	req.Name,
	req.Scopes,
	userID,
	req.EnvironmentIDs,
	req.ExpiresAt,
)
```

Default empty slice if nil:
```go
if req.EnvironmentIDs == nil {
	req.EnvironmentIDs = []uuid.UUID{}
}
```

- [ ] **Step 3: Update listAPIKeys handler**

Parse optional `environment_id` query parameter and pass to ListKeys:

```go
func (h *APIKeyHandler) listAPIKeys(c *gin.Context) {
	// ... existing orgID, projectID extraction ...

	var environmentID *uuid.UUID
	if envStr := c.Query("environment_id"); envStr != "" {
		if eid, err := uuid.Parse(envStr); err == nil {
			environmentID = &eid
		}
	}

	keys, err := h.service.ListKeys(c.Request.Context(), orgID, projectID, environmentID, limit, offset)
	// ... rest unchanged ...
}
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./internal/auth/...`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add internal/auth/apikey_handler.go
git commit -m "feat: accept environment_ids on API key creation, filter on list"
```

---

## Task 6: Enforce Environment Restriction in Auth Middleware

**Files:**
- Modify: `internal/auth/middleware.go`

- [ ] **Step 1: Update APIKeyInfo struct**

Replace single `EnvironmentID` with `EnvironmentIDs`:

```go
type APIKeyInfo struct {
	OrgID          uuid.UUID   `json:"org_id"`
	ProjectID      *uuid.UUID  `json:"project_id,omitempty"`
	EnvironmentIDs []uuid.UUID `json:"environment_ids"`
	Scopes         []string    `json:"scopes"`
}
```

- [ ] **Step 2: Update the authenticateAPIKey function**

Where context values are set after API key validation, replace the single `environment_id` set with `environment_ids`:

Remove:
```go
if info.ApplicationID != nil {
	c.Set("application_id", info.ApplicationID.String())
}
if info.EnvironmentID != nil {
	c.Set("environment_id", info.EnvironmentID.String())
}
```

Replace with:
```go
if len(info.EnvironmentIDs) > 0 {
	envStrs := make([]string, len(info.EnvironmentIDs))
	for i, eid := range info.EnvironmentIDs {
		envStrs[i] = eid.String()
	}
	c.Set("api_key_environment_ids", envStrs)
}
```

- [ ] **Step 3: Add environment enforcement after key validation**

After the context values are set and before `c.Next()`, add enforcement:

```go
// Enforce environment restriction
if len(info.EnvironmentIDs) > 0 {
	targetEnv := ""

	// Check query parameter
	if eid := c.Query("environment_id"); eid != "" {
		targetEnv = eid
	}

	// Check JSON body (for POST/PUT) — peek without consuming
	if targetEnv == "" {
		if eid, exists := c.Get("environment_id"); exists {
			if s, ok := eid.(string); ok {
				targetEnv = s
			}
		}
	}

	// If a target environment is identified, verify it's allowed
	if targetEnv != "" {
		allowed := false
		for _, eid := range info.EnvironmentIDs {
			if eid.String() == targetEnv {
				allowed = true
				break
			}
		}
		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "api key is not authorized for this environment",
			})
			return
		}
	}
}
```

- [ ] **Step 4: Update where APIKeyInfo is constructed from the validated key**

Find where the `*models.APIKey` returned by `ValidateKey` is converted to `APIKeyInfo`. Update to map the new fields:

```go
info := &APIKeyInfo{
	OrgID:          key.OrgID,
	ProjectID:      key.ProjectID,
	EnvironmentIDs: key.EnvironmentIDs,
	Scopes:         scopeStrings,
}
```

- [ ] **Step 5: Verify it compiles**

Run: `go build ./internal/auth/...`
Expected: No errors

- [ ] **Step 6: Verify full project compiles**

Run: `go build ./...`
Expected: No errors. If there are callers that reference `ApplicationID` or `EnvironmentID` on APIKeyInfo, fix them.

- [ ] **Step 7: Commit**

```bash
git add internal/auth/middleware.go
git commit -m "feat: enforce environment restriction on API key requests"
```

---

## Task 7: Wire and Fix All Callers

**Files:**
- Modify: `cmd/api/main.go` (if GenerateKey call exists there)
- Modify: any other files that reference `APIKey.ApplicationID`, `APIKey.EnvironmentID`, or old `GenerateKey` signature

- [ ] **Step 1: Search for all broken references**

Run: `grep -rn "ApplicationID\|EnvironmentID\|\.EnvironmentID\b" --include="*.go" internal/ cmd/ | grep -v "_test.go" | grep -v "environment_ids"`

Fix every reference:
- `key.ApplicationID` → remove (field deleted)
- `key.EnvironmentID` → `key.EnvironmentIDs`
- `info.EnvironmentID` → `info.EnvironmentIDs`
- `info.ApplicationID` → remove

- [ ] **Step 2: Search for old GenerateKey calls**

Run: `grep -rn "GenerateKey" --include="*.go" internal/ cmd/`

Every call to `GenerateKey` must now pass `[]uuid.UUID` instead of `*uuid.UUID` for the environment parameter, and `uuid.UUID` instead of `*uuid.UUID` for orgID.

- [ ] **Step 3: Search for old ListKeys/ListAPIKeys calls**

Run: `grep -rn "ListKeys\|ListAPIKeys" --include="*.go" internal/ cmd/`

Every call must now include the `environmentID *uuid.UUID` parameter.

- [ ] **Step 4: Verify full build**

Run: `go build ./...`
Expected: Clean compile

- [ ] **Step 5: Run tests**

Run: `make test`
Expected: All tests pass. Fix any test failures caused by the model/signature changes.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "fix: update all callers for environment_ids API key changes"
```

---

## Task 8: Update CLI

**Files:**
- Modify: `cmd/cli/apikeys.go`

- [ ] **Step 1: Add --env flag to create command**

Find the `apikeysCreateCmd` flag registration section (around line 81-84) and add:

```go
apikeysCreateCmd.Flags().StringSlice("env", nil, "environment slug(s) to restrict key to (repeatable, omit for unrestricted)")
```

- [ ] **Step 2: Update runAPIKeysCreate to include environment_ids**

In the `runAPIKeysCreate` function, read the `--env` flag and include it in the API request body:

```go
envSlugs, _ := cmd.Flags().GetStringSlice("env")

body := map[string]interface{}{
	"name":   name,
	"scopes": scopes,
}

if len(envSlugs) > 0 {
	// Resolve slugs to UUIDs via the environments API
	// For now, pass slugs and let the server resolve, or require UUIDs
	body["environment_ids"] = envSlugs
}
```

Note: If the CLI currently sends environment slugs but the API expects UUIDs, the CLI should either resolve slugs to UUIDs first (by calling the environments list API) or the create handler should accept slugs. Check what's simpler — if the CLI already has an org context with environment list access, resolve locally.

- [ ] **Step 3: Update list output to show environments**

In the list table output, add an "Environments" column:

```go
// In the table header
fmt.Fprintf(w, "ID\tNAME\tPREFIX\tSCOPES\tENVIRONMENTS\tCREATED\tLAST USED\tEXPIRES\n")

// In the row
envDisplay := "All"
if len(key.EnvironmentIDs) > 0 {
	envDisplay = fmt.Sprintf("%d env(s)", len(key.EnvironmentIDs))
}
```

- [ ] **Step 4: Verify CLI builds**

Run: `go build ./cmd/cli/...`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add cmd/cli/apikeys.go
git commit -m "feat: add --env flag to CLI apikeys create command"
```

---

## Task 9: Update Frontend Types

**Files:**
- Modify: `web/src/types.ts`

- [ ] **Step 1: Update the ApiKey interface**

Find the `ApiKey` interface in `web/src/types.ts` and replace `environment_targets: string[]` with `environment_ids: string[]`:

```typescript
export interface ApiKey {
  id: string;
  name: string;
  prefix: string;
  scopes: string[];
  environment_ids: string[];
  project_id?: string;
  created_at: string;
  last_used_at: string | null;
  expires_at: string | null;
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/types.ts
git commit -m "feat: update ApiKey type — environment_ids replaces environment_targets"
```

---

## Task 10: Update APIKeysPage Frontend

**Files:**
- Modify: `web/src/pages/APIKeysPage.tsx`

- [ ] **Step 1: Add environment hooks and state**

At the top of the component, add imports and state for environments:

```typescript
import { useEnvironments } from '../hooks/useEntities';

// Inside the component:
const { environments } = useEnvironments(orgSlug);
const [selectedEnvIds, setSelectedEnvIds] = useState<string[]>([]);
```

- [ ] **Step 2: Replace the "coming soon" placeholder with environment checkboxes**

Find the "Environment restrictions coming soon" text (around line 125-128) and replace with:

```tsx
<div style={{ marginBottom: 16 }}>
  <label className="form-label">Environment Restrictions</label>
  <p className="text-muted" style={{ fontSize: '0.85rem', marginBottom: 8 }}>
    Leave all unchecked for unrestricted access to all environments.
  </p>
  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12 }}>
    {environments.map((env) => (
      <label key={env.id} style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
        <input
          type="checkbox"
          checked={selectedEnvIds.includes(env.id)}
          onChange={(e) => {
            if (e.target.checked) {
              setSelectedEnvIds((prev) => [...prev, env.id]);
            } else {
              setSelectedEnvIds((prev) => prev.filter((id) => id !== env.id));
            }
          }}
        />
        {env.name}
      </label>
    ))}
  </div>
</div>
```

- [ ] **Step 3: Include environment_ids in the create API call**

Find where the create API call is made and add `environment_ids`:

```typescript
const response = await apiKeysApi.create({
  name: newKeyName,
  scopes: selectedScopes,
  environment_ids: selectedEnvIds.length > 0 ? selectedEnvIds : undefined,
  // ... other fields
});
```

Reset after creation:
```typescript
setSelectedEnvIds([]);
```

- [ ] **Step 4: Update the environment display column in the list table**

Find where the environment column is rendered (around lines 192-201). Update to use `environment_ids` instead of `environment_targets`:

```tsx
<td>
  {key.environment_ids.length === 0 ? (
    <span className="badge">All</span>
  ) : (
    key.environment_ids.map((eid) => {
      const env = environments.find((e) => e.id === eid);
      return (
        <span key={eid} className="badge" style={{ marginRight: 4 }}>
          {env?.name || eid.slice(0, 8)}
        </span>
      );
    })
  )}
</td>
```

- [ ] **Step 5: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/APIKeysPage.tsx
git commit -m "feat: add environment multi-select to API key creation, show in list"
```

---

## Task 11: Rebuild and Verify

- [ ] **Step 1: Full backend build**

Run: `go build ./...`
Expected: Clean compile

- [ ] **Step 2: Full frontend build**

Run: `cd web && npx tsc --noEmit`
Expected: Clean compile

- [ ] **Step 3: Run tests**

Run: `make test`
Expected: All tests pass

- [ ] **Step 4: Run migration**

Run: `make migrate-up`
Expected: Migration 042 applied (or already applied)

- [ ] **Step 5: Rebuild and restart API server**

Run: `go build -o bin/deploysentry-api ./cmd/api/`
Then restart the launchd service.

- [ ] **Step 6: Manual verification**

1. Open the dashboard → API Keys page
2. Click Create — verify environment checkboxes appear
3. Create a key restricted to one environment
4. Verify the list shows the environment badge(s)
5. Use the restricted key to call an endpoint targeting a different environment — should get 403
6. Use the restricted key for its allowed environment — should succeed

- [ ] **Step 7: Commit any fixes**

```bash
git add -A
git commit -m "fix: address issues found during verification"
```

---

## Task 12: Update Documentation

**Files:**
- Modify: `docs/Current_Initiatives.md`
- Modify: `docs/superpowers/specs/2026-04-16-environment-scoped-api-keys-design.md`

- [ ] **Step 1: Update the spec phase**

Change the status in the spec from `Design` to `Complete`.

- [ ] **Step 2: Update current initiatives**

Update the Environment-Scoped API Keys row in Current_Initiatives.md to show Implementation phase with a link to the plan.

- [ ] **Step 3: Commit**

```bash
git add docs/
git commit -m "docs: mark environment-scoped API keys as complete"
```
