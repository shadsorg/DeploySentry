# Environment-Scoped API Keys

**Date**: 2026-04-16
**Status**: Design

## Overview

API keys should be scopeable to a specific environment so that a production key cannot be used against staging, and vice versa. The database schema and model already have an `environment_id` column, but it is not loaded from queries, not exposed in the create/list API, not filterable, and not enforced during request authorization. This spec closes those gaps.

## Problem

Today, an API key created for a project can evaluate flags and create deployments against any environment in that project. There is no way to restrict a key to a single environment. This means:

- A `ds_test_` key for staging could accidentally be used in a production deployment workflow
- SDK clients initialized with the wrong key silently evaluate against the wrong environment
- There is no least-privilege path for environment-level access

## Goals

1. API keys can optionally be scoped to one or more environments at creation time
2. When an environment-scoped key is used, requests targeting a different environment are rejected
3. The dashboard, CLI, and API all support creating/viewing environment-scoped keys
4. Existing keys (no environment scope) continue to work unchanged — full project access
5. The `environment_id` context value set by API key auth is enforced by downstream handlers

## Current State

### What exists

- **DB column**: `api_keys.environment_id` (UUID, nullable, FK → environments, added in migration 029)
- **DB constraint**: `CHECK (num_nonnulls(org_id, project_id, application_id, environment_id) = 1)` — only one scope level set
- **Model field**: `APIKey.EnvironmentID *uuid.UUID`
- **Service**: `GenerateKey()` accepts `envID *uuid.UUID` parameter
- **Auth middleware**: Sets `environment_id` on Gin context if present on the key
- **Frontend**: `APIKeysPage` has an "Environment Restrictions coming soon" placeholder

### What's broken or missing

| Layer | Gap |
|-------|-----|
| **Postgres repository** | `apiKeySelectCols` does not include `application_id` or `environment_id`. `scanAPIKey()` never reads them. Keys are inserted with `environment_id` but it's never loaded back. |
| **List query** | `ListAPIKeys()` filters by `org_id` and optionally `project_id`, but not `environment_id`. |
| **Create handler** | `POST /api-keys` request body only accepts `project_id`. No field for `environment_id`. |
| **Enforcement** | No middleware or handler checks that the request's target environment matches the key's `environment_id`. A key scoped to staging can currently hit production endpoints. |
| **Frontend** | Create form has a placeholder instead of an environment selector. List table expects `environment_targets: string[]` but backend returns singular `environment_id`. |
| **CLI** | `apikeys create` has no `--env` flag. |
| **DB constraint conflict** | The current CHECK constraint requires exactly one of (org_id, project_id, application_id, environment_id). This means a key scoped to an environment CANNOT also be scoped to a project — they're mutually exclusive. For environment scoping to work alongside project scoping, the constraint needs to change. |

## Design

### Data Model Change

The current CHECK constraint (`num_nonnulls = 1`) prevents a key from being scoped to both a project AND an environment. This is wrong — the natural hierarchy is: a key belongs to a project and is optionally restricted to specific environments within that project.

**Change the model** from single-scope to hierarchical:

- `org_id` — the org the key belongs to (always set)
- `project_id` — the project the key targets (required for non-admin keys)
- `environment_ids` — optional list of environments the key is restricted to

**Migration**: 
1. Drop the `num_nonnulls = 1` CHECK constraint
2. Make `org_id` NOT NULL (always required)
3. Keep `project_id` nullable (admin keys don't need it)
4. Replace single `environment_id UUID` with `environment_ids UUID[]` (array) to support multi-environment keys (e.g., a key that works in both staging and production)
5. Drop `application_id` column (unused — application scoping happens through the project, not the key)

After migration:
```sql
api_keys (
    ...
    org_id           UUID NOT NULL REFERENCES organizations(id),
    project_id       UUID REFERENCES projects(id),
    environment_ids  UUID[] NOT NULL DEFAULT '{}',
    ...
)
```

An empty `environment_ids` array means "all environments in the project" (current behavior preserved).

### Enforcement

**In auth middleware**, after API key validation:

```
if key.EnvironmentIDs is not empty:
    extract target environment from request (query param, body, or URL)
    if target environment is not in key.EnvironmentIDs:
        return 403 "api key not authorized for this environment"
```

Target environment sources (checked in order):
1. `environment_id` query parameter (flag evaluation, deployment creation)
2. `environment_id` in JSON request body
3. The environment already resolved on the Gin context by earlier middleware

If no target environment can be determined from the request and the key has environment restrictions, the request proceeds (some endpoints like list endpoints don't target a specific environment).

### API Changes

**Create key** (`POST /api-keys`):

Add `environment_ids` to the request body:

```json
{
  "name": "production-deploy-key",
  "project_id": "uuid",
  "scopes": ["deploys:read", "deploys:write", "flags:read"],
  "environment_ids": ["uuid-of-production"],
  "expires_at": "2027-01-01T00:00:00Z"
}
```

`environment_ids` is optional. Omit or pass `[]` for unrestricted.

**List keys** (`GET /api-keys`):

Add optional query parameter `?environment_id=<uuid>` to filter keys that include that environment in their `environment_ids` array.

**Key response** includes `environment_ids` and resolved environment names:

```json
{
  "id": "uuid",
  "name": "production-deploy-key",
  "key_prefix": "ds_live_",
  "scopes": ["deploys:read", "deploys:write", "flags:read"],
  "project_id": "uuid",
  "environment_ids": ["uuid"],
  "environments": [
    {"id": "uuid", "name": "production", "slug": "production"}
  ],
  "created_at": "...",
  "last_used_at": "..."
}
```

### Frontend Changes

**APIKeysPage create form**:
- Replace "Environment Restrictions coming soon" placeholder with a multi-select checkbox list of environments in the org
- Empty selection = "All environments" (unrestricted)

**APIKeysPage list table**:
- Environment column shows badges for each environment name, or "All" if unrestricted

### CLI Changes

**`apikeys create`** — Add `--env` flag (repeatable):

```bash
deploysentry apikeys create \
  --name "prod-deploy" \
  --scopes "deploys:read,deploys:write,flags:read" \
  --env production \
  --env staging
```

Omit `--env` for unrestricted.

**`apikeys list`** — Show environment names in output table.

## Migration Plan

### Migration 042

```sql
-- 1. Drop the exclusive-scope CHECK constraint
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS chk_api_key_scope;

-- 2. Make org_id NOT NULL (backfill from project's org if needed)
UPDATE api_keys SET org_id = (
    SELECT p.org_id FROM projects p WHERE p.id = api_keys.project_id
) WHERE org_id IS NULL AND project_id IS NOT NULL;

ALTER TABLE api_keys ALTER COLUMN org_id SET NOT NULL;

-- 3. Add environment_ids array column
ALTER TABLE api_keys ADD COLUMN environment_ids UUID[] NOT NULL DEFAULT '{}';

-- 4. Migrate existing environment_id data
UPDATE api_keys SET environment_ids = ARRAY[environment_id]
WHERE environment_id IS NOT NULL;

-- 5. Drop old singular columns
ALTER TABLE api_keys DROP COLUMN environment_id;
ALTER TABLE api_keys DROP COLUMN application_id;

-- 6. Add index for environment array queries
CREATE INDEX idx_api_keys_environment_ids ON api_keys USING GIN (environment_ids);
```

## Scope

### In scope
- Migration to `environment_ids UUID[]`
- Repository: load and filter by environment_ids
- Handler: accept environment_ids on create, return in responses
- Auth middleware: enforce environment restriction on requests
- Frontend: environment multi-select on create, badges on list
- CLI: `--env` flag on create

### Out of scope
- Application-level scoping (drop `application_id` — unused)
- IP allowlist (separate feature)
- Key rotation preserving environment scope (already works once the field loads)
- Rate limiting per environment-scoped key
