# Deploy Onboarding & API Key Scoping

**Date:** 2026-04-17
**Status:** Design

## Overview

Three interconnected improvements that fix the onboarding friction for deploy monitoring and control:

1. **API Key Application Scoping** — Add `application_id` to the API key model with enforced middleware restriction. The UI gets cascading project → application dropdowns on the create form.
2. **Agent Config Simplification** — The agent derives org/project/app/environment from the scoped API key on registration, reducing required config from 5 env vars to 2.
3. **Onboarding Documentation** — A new `docs/Deploy_Monitoring_Setup.md` covering developer quickstart, ops production setup, platform-specific guides, and LLM-assisted bootstrap.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Application scoping | Enforced at middleware | If you scope a key to an app, it should be enforced. Also lets the agent derive app_id from the key. |
| Agent config | Key-derived (DS_APP_ID optional override) | Scoped key carries all context. Agent only needs DS_API_KEY + DS_UPSTREAMS. |
| Doc audience | Single doc, two paths (dev + ops) | Avoids duplication. Developers read sections 1-4, ops reads 5-8. |
| UI dropdowns | Cascading project → app | Apps only shown when a project is selected. "All Projects" / "All Applications" for broad scope. |

## Database Changes

### Migration 045: Add application_id to api_keys

```sql
-- up
ALTER TABLE api_keys ADD COLUMN application_id UUID REFERENCES applications(id) ON DELETE SET NULL;
CREATE INDEX idx_api_keys_application_id ON api_keys(application_id);

-- down
DROP INDEX IF EXISTS idx_api_keys_application_id;
ALTER TABLE api_keys DROP COLUMN IF EXISTS application_id;
```

The column is nullable. `NULL` means "all applications" (no restriction), matching the existing `project_id` pattern.

## API Key Model Changes

### Go Model (`internal/models/api_key.go`)

Add field to `APIKey` struct:
```go
ApplicationID  *uuid.UUID `json:"application_id,omitempty" db:"application_id"`
```

### TypeScript Type (`web/src/types.ts`)

Add to `ApiKey` interface:
```typescript
application_id?: string;
```

### Postgres Repository (`internal/platform/database/postgres/apikeys.go`)

Add `application_id` to:
- `apiKeySelectCols` column list
- `scanAPIKey` scan fields
- `CreateAPIKey` INSERT columns and values

### API Handler (`internal/auth/apikey_handler.go`)

Add to `createAPIKeyRequest`:
```go
ApplicationID *uuid.UUID `json:"application_id"`
```

Pass through to `GenerateKey()`. Add `applicationID *uuid.UUID` parameter to `GenerateKey` and `RotateKey`.

### API Key Info (`internal/auth/middleware.go`)

Add to `APIKeyInfo`:
```go
ApplicationID *uuid.UUID `json:"application_id,omitempty"`
```

Populate from the validated key in `authenticateAPIKey`. The `apiKeyValidatorAdapter` in `cmd/api/main.go` maps the model field to `APIKeyInfo.ApplicationID`.

### Middleware Enforcement

In `authenticateAPIKey`, after the existing environment restriction check, add:
```go
if info.ApplicationID != nil {
    targetApp := c.Query("app_id")
    if targetApp == "" {
        if aid, exists := c.Get("app_id"); exists {
            if s, ok := aid.(string); ok {
                targetApp = s
            }
        }
    }
    if targetApp != "" {
        targetID, err := uuid.Parse(targetApp)
        if err == nil && targetID != *info.ApplicationID {
            c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "API key is not authorized for this application"})
            return false
        }
    }
}
```

## API Key Create UI Changes

### Cascading Dropdowns on APIKeysPage.tsx

Add two new dropdowns between "Scopes" and "Environment Restrictions":

**Project Scope dropdown:**
- Options: "All Projects" (value: null) + list from `GET /api/v1/orgs/:org/projects`
- Default: "All Projects"
- When changed: reset application selection, fetch apps for the selected project

**Application Scope dropdown:**
- Disabled when project is "All Projects" — shows "Select a project first"
- When project is selected: options are "All Applications" (value: null) + list from `GET /api/v1/orgs/:org/projects/:project/apps`
- Default: "All Applications"

**Data fetching:**
- Projects: fetch on dialog open using existing `useProjects` or inline fetch
- Applications: fetch when project changes using existing `useApps` or inline fetch
- Both use the org slug from the current route context

**Create request body update:**
```typescript
apiKeysApi.create({
  name,
  scopes,
  environment_ids,
  project_id,       // new: UUID or undefined
  application_id,   // new: UUID or undefined
})
```

Update `apiKeysApi.create` type signature to include `project_id?: string` and `application_id?: string`.

### API Key List Display

Show scope badges on each key in the list:
- If `project_id` is set: show project name badge
- If `application_id` is set: show app name badge
- If neither: show "Org-wide" badge

## Agent Configuration Simplification

### Registration Response Enhancement

`POST /api/v1/agents/register` currently returns the `Agent` model. Enhance the response to include scope information derived from the API key:

```json
{
  "id": "agent-uuid",
  "app_id": "708bb092-...",
  "environment_id": "6b5ac5c9-...",
  "org_id": "org-uuid",
  "project_id": "project-uuid",
  "status": "connected",
  "registered_at": "..."
}
```

The handler reads the API key scope from the Gin context (set by auth middleware) and populates `app_id` and `environment_id` from the key's `ApplicationID` and `EnvironmentIDs[0]` (first scoped environment, or the agent's `DS_ENVIRONMENT` fallback).

### Agent Config Changes (`internal/agent/config.go`)

Make `DS_APP_ID` and `DS_ENVIRONMENT` optional:

```go
type Config struct {
    APIURL            string
    APIKey            string
    AppID             *uuid.UUID        // optional: derived from key if not set
    Environment       string            // optional: derived from key if not set
    Upstreams         map[string]string  // required: local infrastructure
    EnvoyXDSPort      int
    EnvoyListenPort   int
    HeartbeatInterval time.Duration
}
```

`LoadConfig` no longer errors on missing `DS_APP_ID`. The agent resolves it from the registration response.

### Agent Startup Flow (`cmd/agent/main.go`)

Updated flow:
1. Load config (only `DS_API_KEY` and `DS_UPSTREAMS` required)
2. Register with API — receive scope (app_id, environment_id, org_id, project_id)
3. If `DS_APP_ID` was set, use it (override). Otherwise use key-derived `app_id`.
4. If `DS_ENVIRONMENT` was set, use it. Otherwise use key-derived `environment_id`.
5. Start xDS, SSE, heartbeat with resolved IDs.

### Minimum Agent Config

After these changes, the minimum config is:
```bash
DS_API_KEY=ds_xxxxxxxx
DS_UPSTREAMS=blue:app-blue:8081,green:app-green:8082
```

Everything else is either derived from the key or has sensible defaults.

## Onboarding Documentation

### New: `docs/Deploy_Monitoring_Setup.md`

**Developer Path (sections 1-4):**

1. **Prerequisites** — DeploySentry account, org, project, and app created. Links to dashboard.
2. **Create a Scoped API Key** — Step-by-step through the dashboard: API Keys → Create → select project → select app → select environment → select scopes. CLI equivalent shown.
3. **Configure the Agent** — Two env vars (`DS_API_KEY`, `DS_UPSTREAMS`), then platform-specific start command (Docker: `make dev-deploy`, Render: env vars in dashboard, Railway: env vars + start command).
4. **Verify & First Deployment** — Check agent on dashboard, create canary deployment, watch traffic.

**Operations Path (sections 5-8):**

5. **Key Management Strategy** — Table of recommended scope per role: CI/CD pipeline (project-scoped, deploys:write), agent (app-scoped, deploys:read+write+flags:read), developer (project-scoped, flags:read+write), readonly (org-wide, flags:read+deploys:read).
6. **Multi-Environment Setup** — One key per environment per app. Agent derives environment from key scope.
7. **Platform Deployment Guides** — Step-by-step for Docker Compose, Render, Railway, Fly.io.
8. **Monitoring & Troubleshooting** — Dashboard panels, agent health, common issues.

**LLM-Assisted Setup (section 9):**

9. **LLM-Assisted Setup**
   - **MCP Server path:** Connect `deploysentry mcp serve`, provide a bootstrap prompt. The LLM uses MCP tools to discover org/project/app, create a scoped key, and walk through agent config.
   - **Prompt Templates (no MCP):** Copy-paste prompts for any LLM with API endpoint references:
     - Bootstrap prompt (setup from zero)
     - Deploy prompt (create/monitor/promote)
     - Debug prompt (agent status, traffic state, diagnostics)

### Cross-references

- `docs/Traffic_Management_Guide.md` — add link to Deploy Monitoring Setup for initial onboarding
- `docs/Deploy_Integration_Guide.md` — add link for agent-based monitoring setup
- `README.md` — add bullet in Deployments section

## Scope Boundaries

**In scope:**
- Migration 045 (application_id on api_keys)
- API key model, handler, middleware changes for application scoping
- APIKeysPage.tsx cascading dropdowns (project → app)
- Agent config simplification (optional DS_APP_ID, key-derived scope)
- Agent registration response with scope info
- TypeScript type and API function updates
- New docs/Deploy_Monitoring_Setup.md with all 9 sections
- Cross-reference updates

**Out of scope:**
- Session blacklisting (separate plan exists)
- API key rotation UI (existing CLI rotation works)
- Agent auto-discovery of upstreams (DS_UPSTREAMS remains manual)
- MCP tool changes (existing tools sufficient)
