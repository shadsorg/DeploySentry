# DeploySentry Platform Redesign — CLI Installer, Hierarchy, UI, and Terminology

## Overview

Redesign DeploySentry around a four-level hierarchy (Org → Project → Application → Environment), clarify Deployment vs Release semantics, overhaul the web UI with org-centric navigation, add a CLI installer script, and introduce hierarchical API keys and settings.

## Goals

1. Add "Application" as a deployable unit between Project and Environment.
2. Redefine **Deployment** as shipping application code (version/patch bumps) and **Release** as enabling or changing flags with rollout strategy.
3. Provide a `curl | sh` CLI installer for easy setup.
4. Redesign the web UI with org switcher, accordion sidebar, and per-environment flag state.
5. Support hierarchical API keys and settings (Org/Project/Application/Environment) with cascading inheritance.

## Non-Goals

- GitHub webhook integration for deploy status (separate initiative).
- OAuth/SSO login (layered later).
- Mobile app changes.

---

## 1. Terminology

| Term | Definition |
|------|-----------|
| **Deployment** | Shipping application code. Tracks version, commit SHA, artifact, and rollout strategy (canary, blue-green, rolling). Lives at the Application + Environment level. |
| **Release** | A bundle of flag changes applied with a rollout strategy. Tracks which flags change, in which environments, with what traffic percentage. Lives at the Application level. |
| **Feature Flag** | A flag defined at the **Project** level. Can span multiple applications. Categories: feature, experiment, ops, permission. |
| **Release Flag** | A flag defined at the **Application** level. Category: release. Tied to isolating code changes for a specific application. |
| **Application** | A deployable unit (microservice, frontend app, mobile app) within a project. Owns its own environments, deployments, releases, and release flags. |
| **Session Stickiness** | A modifier on Release rollout strategy. Once a user/tenant is bucketed via a client-provided header, they remain in that bucket for the session duration. |

---

## 2. Data Model

### 2.1 New Table: `applications`

```sql
CREATE TABLE applications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL,
    description     TEXT,
    repo_url        TEXT,
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(project_id, slug)
);

CREATE INDEX idx_applications_project ON applications(project_id);
```

### 2.2 Modified: `environments`

Move from project-scoped to application-scoped. The existing unique constraint `(project_id, slug)` must be dropped and recreated as `(application_id, slug)`.

```sql
-- Drop existing constraint and FK
ALTER TABLE environments DROP CONSTRAINT IF EXISTS environments_project_id_slug_key;
ALTER TABLE environments DROP CONSTRAINT IF EXISTS environments_project_id_fkey;

-- Rename column
ALTER TABLE environments RENAME COLUMN project_id TO application_id;

-- Add new FK and unique constraint
ALTER TABLE environments ADD CONSTRAINT fk_environments_application
    FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE CASCADE;
ALTER TABLE environments ADD CONSTRAINT environments_application_id_slug_key
    UNIQUE (application_id, slug);

CREATE INDEX idx_environments_application ON environments(application_id);
```

### 2.3 Modified: `deployments`

Deployments become the code-shipping concept. Absorb version/artifact/commit fields from the old Release model. Scoped to application + environment.

Note: The existing `deployments` table has `pipeline_id` (FK to `deploy_pipelines`), `release_id`, and a TEXT `environment` column from the original migration. The `project_id` was added later in migration 023 as nullable. This migration must handle all of these legacy columns.

```sql
-- Drop legacy FKs and columns
ALTER TABLE deployments DROP CONSTRAINT IF EXISTS deployments_pipeline_id_fkey;
ALTER TABLE deployments DROP CONSTRAINT IF EXISTS deployments_release_id_fkey;
ALTER TABLE deployments DROP COLUMN IF EXISTS pipeline_id;
ALTER TABLE deployments DROP COLUMN IF EXISTS release_id;
ALTER TABLE deployments DROP COLUMN IF EXISTS environment;  -- TEXT column, replaced by environment_id FK

-- Rename project_id → application_id
ALTER TABLE deployments RENAME COLUMN project_id TO application_id;
ALTER TABLE deployments ADD CONSTRAINT fk_deployments_application
    FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE CASCADE;

-- Ensure environment_id exists as a proper FK
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS environment_id UUID
    REFERENCES environments(id) ON DELETE SET NULL;

-- Ensure code-shipping fields exist
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS version TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS commit_sha TEXT;
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS artifact TEXT NOT NULL DEFAULT '';
-- strategy, traffic_percent, status already exist

CREATE INDEX idx_deployments_application ON deployments(application_id);
CREATE INDEX idx_deployments_environment ON deployments(environment_id);
```

Deployment statuses: `pending`, `running`, `promoting`, `paused`, `completed`, `failed`, `rolled_back`, `cancelled`.

State machine transitions:
- `pending` → `running`
- `running` → `promoting`, `paused`, `completed`, `failed`, `rolled_back`
- `promoting` → `running`, `completed`, `failed`
- `paused` → `running`, `rolled_back`, `cancelled`
- `failed` → `rolled_back`

### 2.4 Drop legacy tables

The `deploy_pipelines` table and old `releases`/`release_environments` tables are no longer needed.

```sql
DROP TABLE IF EXISTS release_environments;
DROP TABLE IF EXISTS releases;
DROP TABLE IF EXISTS deploy_pipelines;
```

Note: Any `audit_log` entries referencing old release or pipeline IDs will have dangling `resource_id` values. This is acceptable — the audit log stores IDs as TEXT references, not FKs. Old entries remain for historical record but the referenced resources no longer exist.

### 2.5 Redefined: `releases`

Releases are now flag-change bundles with rollout strategy.

Release strategy is always `percentage` — traffic is rolled out as a percentage of users/requests. The distinction between "target specific segments first" vs "broad rollout" is handled by targeting rules on the individual flags, not by the release strategy. This avoids ambiguity with deployment-level canary (which is an infrastructure concept).

```sql
CREATE TABLE releases (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id  UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT,
    session_sticky  BOOLEAN NOT NULL DEFAULT false,
    sticky_header   TEXT,           -- e.g., 'X-Session-ID', required when session_sticky = true
    traffic_percent INT NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft', 'rolling_out', 'paused', 'completed', 'rolled_back')),
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_releases_application ON releases(application_id);
CREATE INDEX idx_releases_status ON releases(status);
```

### 2.6 New Table: `release_flag_changes`

Tracks each flag change within a release.

```sql
CREATE TABLE release_flag_changes (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    release_id        UUID NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    flag_id           UUID NOT NULL REFERENCES feature_flags(id) ON DELETE CASCADE,
    environment_id    UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    previous_value    JSONB,
    new_value         JSONB,
    previous_enabled  BOOLEAN,
    new_enabled       BOOLEAN,
    applied_at        TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_release_flag_changes_release ON release_flag_changes(release_id);
CREATE INDEX idx_release_flag_changes_flag ON release_flag_changes(flag_id);
```

### 2.7 Modified: `feature_flags`

Add optional `application_id`. Release flags require it; other categories leave it null (project-wide).

The existing unique constraint `(project_id, key)` must be replaced with partial indexes to support both project-level and application-level flags.

```sql
ALTER TABLE feature_flags ADD COLUMN IF NOT EXISTS application_id UUID
    REFERENCES applications(id) ON DELETE CASCADE;

-- Drop existing unique constraint
ALTER TABLE feature_flags DROP CONSTRAINT IF EXISTS feature_flags_project_id_key_key;

-- Project-level flags: unique by (project_id, key) where application_id IS NULL
CREATE UNIQUE INDEX idx_flags_project_key
    ON feature_flags(project_id, key) WHERE application_id IS NULL;

-- Application-level flags: unique by (application_id, key)
CREATE UNIQUE INDEX idx_flags_application_key
    ON feature_flags(application_id, key) WHERE application_id IS NOT NULL;

-- Deprecate feature_flags.enabled and feature_flags.environment_id
-- These are replaced by flag_environment_state table.
-- The columns are kept for backward compatibility during migration but
-- should not be used by new code. The feature_flags.enabled column becomes
-- a "global default" — the initial value used when seeding flag_environment_state.
```

Application-layer rules:
- `category = 'release'` → `application_id` REQUIRED
- `category IN ('feature', 'experiment', 'ops', 'permission')` → `application_id` NULLABLE (project-wide)

### 2.8 New Table: `flag_environment_state`

Per-environment enabled state and value override for each flag. Replaces the single `enabled` column on `feature_flags`.

```sql
CREATE TABLE flag_environment_state (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    flag_id         UUID NOT NULL REFERENCES feature_flags(id) ON DELETE CASCADE,
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    enabled         BOOLEAN NOT NULL DEFAULT false,
    value           JSONB,
    updated_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(flag_id, environment_id)
);

CREATE INDEX idx_flag_env_state_flag ON flag_environment_state(flag_id);
CREATE INDEX idx_flag_env_state_env ON flag_environment_state(environment_id);
```

### 2.9 Modified: `api_keys` — Hierarchical Scoping

API keys can be scoped to exactly one level. Keys inherit down the hierarchy.

The existing Go model has `OrgID` as a required (non-pointer) field. This changes — all scope columns become nullable, and exactly one must be set.

```sql
-- org_id already exists from schema reconciliation but may need to become nullable
ALTER TABLE api_keys ALTER COLUMN org_id DROP NOT NULL;
-- project_id already exists and is nullable
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS application_id UUID
    REFERENCES applications(id) ON DELETE CASCADE;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS environment_id UUID
    REFERENCES environments(id) ON DELETE CASCADE;

-- Enforce exactly one scope is set
ALTER TABLE api_keys ADD CONSTRAINT chk_api_keys_single_scope
    CHECK (num_nonnulls(org_id, project_id, application_id, environment_id) = 1);
```

Inheritance rules:
- **Org key** → access to all projects, applications, environments under the org.
- **Project key** → access to all applications and environments under the project.
- **Application key** → access to all environments under the application.
- **Environment key** → access to only that environment (typical for SDK use in production).

### 2.10 New Table: `settings`

Cascading configuration at any hierarchy level.

```sql
CREATE TABLE settings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID REFERENCES organizations(id) ON DELETE CASCADE,
    project_id      UUID REFERENCES projects(id) ON DELETE CASCADE,
    application_id  UUID REFERENCES applications(id) ON DELETE CASCADE,
    environment_id  UUID REFERENCES environments(id) ON DELETE CASCADE,
    key             TEXT NOT NULL,
    value           JSONB NOT NULL,
    updated_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Enforce exactly one scope is set
    CONSTRAINT chk_settings_single_scope
        CHECK (num_nonnulls(org_id, project_id, application_id, environment_id) = 1)
);

-- Ensure one setting per key per scope
CREATE UNIQUE INDEX idx_settings_org ON settings(org_id, key) WHERE org_id IS NOT NULL;
CREATE UNIQUE INDEX idx_settings_project ON settings(project_id, key) WHERE project_id IS NOT NULL;
CREATE UNIQUE INDEX idx_settings_app ON settings(application_id, key) WHERE application_id IS NOT NULL;
CREATE UNIQUE INDEX idx_settings_env ON settings(environment_id, key) WHERE environment_id IS NOT NULL;
```

Resolution order (lowest wins): Environment → Application → Project → Org → system default.

Example settings keys:
- `deployment.default_strategy` — default deploy strategy
- `release.sticky_header` — default session-sticky header name
- `notifications.slack_webhook_url` — Slack integration
- `approval.required` — whether deploys/releases require approval
- `api.rate_limit` — rate limit for SDK calls

---

## 3. Session Stickiness

### How It Works

When a Release has `session_sticky = true`:

1. Client includes the header specified by `sticky_header` (e.g., `X-Session-ID: abc123`).
2. The flag evaluator hashes `(sticky_header_value + flag_id)` to produce a deterministic bucket (0-99).
3. If `bucket < traffic_percent`, the user gets the new flag state. Otherwise, the old state.
4. Same session ID always produces the same bucket — no flip-flopping.

### Client Contract

SDKs and API consumers MUST include the sticky header on every request when evaluating flags that are part of a session-sticky release. If the header is missing, the evaluator falls back to random bucketing (non-sticky behavior) and logs a warning.

Documentation must clearly state:
- Which header to send and what value to use (typically a session ID, user ID, or tenant ID).
- That omitting the header causes non-deterministic behavior.
- That the bucket assignment is for the duration of the release rollout — once the release completes, the flag state is permanent for all users.

---

## 4. Web UI Design

### 4.1 Layout Structure

```
┌──────────────────┬──────────────────────────────────────┐
│     Sidebar      │           Main Content               │
│                  │                                      │
│  [Org Switcher]  │  Breadcrumb: Org > Project > App     │
│                  │                                      │
│  ▶ Project A     │  ┌──────────────────────────────┐    │
│  ▼ Project B     │  │     Page Content             │    │
│    ▶ web-app     │  │                              │    │
│    ▼ api-server  │  │                              │    │
│      Deployments │  │                              │    │
│      Releases    │  │                              │    │
│      Flags       │  │                              │    │
│      Settings    │  │                              │    │
│    ▶ mobile      │  └──────────────────────────────┘    │
│                  │                                      │
│  ── Project ──   │                                      │
│    Feature Flags │                                      │
│    Settings      │                                      │
│                  │                                      │
│  ── Org ───────  │                                      │
│    Members       │                                      │
│    API Keys      │                                      │
│    Settings      │                                      │
└──────────────────┴──────────────────────────────────────┘
```

### 4.2 Org Switcher

Dropdown at top of sidebar:
- Shows current org name and user's role (owner/member).
- Lists all orgs the user belongs to.
- "Create Organization" option at the bottom.
- Switching org reloads the sidebar with that org's projects.

### 4.3 Sidebar Navigation

**Project accordion** — each project expands to show its applications.

**Application accordion** — each application expands to show:
- Deployments
- Releases
- Feature Flags (release flags for this app)
- Settings

**Project-level section** (below applications):
- Feature Flags (project-wide: feature, experiment, ops, permission categories)
- Settings

**Org-level section** (at bottom):
- Members (with owner/member role management)
- API Keys
- Settings

### 4.4 Flag Detail Page

Three-tab layout:

**Details tab:**
- Key, name, type, category, description
- Owner, created by, created/updated timestamps
- Expiration info (for release flags)
- Application scope (if release flag) or "Project-wide"

**Targeting Rules tab:**
- List of rules with priority ordering
- Each rule shows: type, conditions, value, percentage, enabled state
- Add/edit/delete/reorder rules

**Environments tab:**
- Table with one row per environment
- Columns: Environment name, Enabled (toggle), Value, Last updated, Updated by
- Inline toggle to enable/disable per environment
- Edit value per environment

### 4.5 Deployment Detail Page

Shows: version, commit SHA, artifact link, strategy, traffic %, status timeline, rollback button.

### 4.6 Release Detail Page

Shows: name, description, traffic %, session-sticky config, status.
Lists all flag changes in the release with before/after values per environment.
Actions: Start, Promote (increase traffic %), Pause, Rollback, Complete.

### 4.7 Routing Structure

```
/login
/register
/orgs/new                                    — create org
/orgs/:orgSlug/projects                      — project list
/orgs/:orgSlug/projects/:projectSlug/flags   — project-level flags list
/orgs/:orgSlug/projects/:projectSlug/flags/:id — project-level flag detail
/orgs/:orgSlug/projects/:projectSlug/settings
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/deployments
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/deployments/:id
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/releases
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/releases/:id
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/flags
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/flags/:id
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/settings
/orgs/:orgSlug/members
/orgs/:orgSlug/api-keys
/orgs/:orgSlug/settings
```

---

## 5. CLI Installer

### 5.1 Install Script

Hosted at a public URL (e.g., `https://dr-sentry.com/get-cli`). Source lives at `scripts/install.sh`.

```bash
curl -fsSL https://dr-sentry.com/get-cli | sh
```

Behavior:
1. Detect OS (`linux` / `darwin`) and architecture (`amd64` / `arm64`).
2. Download the appropriate pre-built binary from the releases URL.
3. Verify checksum (SHA-256) against a published checksum file to ensure binary integrity.
4. Install to `~/.deploysentry/bin/deploysentry`.
5. Make executable (`chmod +x`).
6. Check if `~/.deploysentry/bin` is in PATH. If not, print instructions for adding it (bash/zsh/fish).
7. Print next steps:
   ```
   DeploySentry CLI installed successfully!

   Next steps:
     1. Add to PATH (if not already):
        export PATH="$HOME/.deploysentry/bin:$PATH"
     2. Authenticate:
        deploysentry auth login
   ```

### 5.2 Binary Build

The Go CLI at `cmd/cli/` is compiled for target platforms via a Makefile target or CI job:
- `darwin/amd64`, `darwin/arm64`
- `linux/amd64`, `linux/arm64`

Output binaries are named: `deploysentry-<os>-<arch>`

A SHA-256 checksum file (`checksums.txt`) is generated alongside the binaries.

### 5.3 Version Management

The install script downloads the latest release by default. An optional `VERSION` env var allows pinning:
```bash
VERSION=1.2.0 curl -fsSL https://dr-sentry.com/get-cli | sh
```

---

## 6. CLI Commands

### 6.1 Authentication

```
deploysentry auth login        # Opens browser for OAuth or prompts for email/password
deploysentry auth logout       # Clears stored credentials
deploysentry auth status       # Shows current user, active org, project, app context
```

Credentials stored in `~/.deploysentry/credentials.yml` (JWT token).

### 6.2 Context Management

```
deploysentry org create <name>
deploysentry org list
deploysentry org switch <slug>          # Sets active org in config

deploysentry project create <name> --org <slug>
deploysentry project list

deploysentry app create <name> --project <slug>
deploysentry app list --project <slug>
deploysentry app delete <slug>
deploysentry app add-env <env-name> --app <slug> [--production]
deploysentry app list-env --app <slug>
```

Active context stored in `~/.deploysentry/config.yml`:
```yaml
active_org: acme-corp
active_project: platform
active_app: api-server
api_url: https://deploysentry.example.com
```

Commands use active context as defaults. Explicit flags override.

### 6.3 Feature Flags

```
# Project-level flag (feature, experiment, ops, permission)
deploysentry flag create --key <key> --type boolean --category feature --project <slug>

# Application-level flag (release)
deploysentry flag create --key <key> --type boolean --category release --app <slug>

deploysentry flag list [--app <slug>] [--project <slug>] [--category <cat>]
deploysentry flag get <key>               # Details + all environment states
deploysentry flag toggle <key> --env <env> --on|--off
deploysentry flag evaluate <key> --context '{"user_id":"123"}'
deploysentry flag archive <key>
```

### 6.4 Deployments

```
deploysentry deploy create --app <slug> --env <env> \
    --version 1.2.3 --commit-sha abc123 --artifact <url> \
    --strategy canary --traffic-percent 10
deploysentry deploy list --app <slug>
deploysentry deploy status <id> [--watch]
deploysentry deploy promote <id>          # Increase traffic %
deploysentry deploy rollback <id>
deploysentry deploy pause <id>
deploysentry deploy resume <id>
```

### 6.5 Releases

```
deploysentry release create --app <slug> --name "Enable checkout v2" \
    --traffic-percent 25 --session-sticky --sticky-header X-Session-ID

deploysentry release add-flag <release-id> \
    --flag <key> --env <env> --enable --value "true"

deploysentry release start <id>
deploysentry release promote <id>         # Bump traffic %
deploysentry release pause <id>
deploysentry release rollback <id>
deploysentry release complete <id>
deploysentry release delete <id>          # Only draft releases
deploysentry release list --app <slug>
deploysentry release get <id>
```

### 6.6 API Keys

```
deploysentry apikey create --scope org|project|app|env \
    --target <slug> --permissions flags:read,deploys:write \
    --name "Production SDK Key"
deploysentry apikey list [--scope <scope>]
deploysentry apikey revoke <id>
```

### 6.7 Settings

```
deploysentry settings set <key> <value> --scope org|project|app|env --target <slug>
deploysentry settings get <key> [--scope org|project|app|env --target <slug>]
deploysentry settings list [--scope org|project|app|env --target <slug>]
```

When `--scope` is omitted on `get`, returns the resolved value (cascaded).

---

## 7. API Changes

### 7.1 New Endpoints

```
# Applications
POST   /api/v1/projects/:projectId/applications
GET    /api/v1/projects/:projectId/applications
GET    /api/v1/applications/:id
PUT    /api/v1/applications/:id
DELETE /api/v1/applications/:id
POST   /api/v1/applications/:id/environments    # Add env to app
GET    /api/v1/applications/:id/environments     # List envs for app

# Releases (redefined)
POST   /api/v1/applications/:appId/releases
GET    /api/v1/applications/:appId/releases
GET    /api/v1/releases/:id
PUT    /api/v1/releases/:id
DELETE /api/v1/releases/:id                      # Only draft releases
POST   /api/v1/releases/:id/flags               # Add flag change to release
POST   /api/v1/releases/:id/start
POST   /api/v1/releases/:id/promote
POST   /api/v1/releases/:id/pause
POST   /api/v1/releases/:id/rollback
POST   /api/v1/releases/:id/complete

# Flag environment state
GET    /api/v1/flags/:id/environments            # All env states for a flag
PUT    /api/v1/flags/:id/environments/:envId     # Set state for one env

# Flag evaluation (updated for new hierarchy)
POST   /api/v1/flags/evaluate                    # Accepts application_id in body
# SDK evaluation context now includes application_id alongside project_id and environment

# Settings
GET    /api/v1/settings?scope=org&target=:id     # List settings at scope
GET    /api/v1/settings/resolve?key=:key&env=:envId  # Resolved value (cascaded)
PUT    /api/v1/settings                          # Set a setting at a scope

# Org management
GET    /api/v1/orgs                              # List user's orgs
POST   /api/v1/orgs                              # Create org
GET    /api/v1/orgs/:id/members
POST   /api/v1/orgs/:id/members
```

### 7.2 Modified Endpoints

```
# Deployments — scoped to application instead of project
POST   /api/v1/applications/:appId/deployments   (was /projects/:projectId/deployments)
GET    /api/v1/applications/:appId/deployments
GET    /api/v1/deployments/:id
POST   /api/v1/deployments/:id/promote
POST   /api/v1/deployments/:id/rollback
POST   /api/v1/deployments/:id/pause
POST   /api/v1/deployments/:id/resume

# Flags — support application_id filter
GET    /api/v1/flags?project_id=X                 # Project-level flags
GET    /api/v1/flags?application_id=X             # App-level (release) flags
POST   /api/v1/flags                              # Accepts optional application_id
```

### 7.3 API Key Validation

When validating an API key, the auth middleware resolves the key's scope and checks if the requested resource falls within that scope:

1. Look up the key's scope level and target ID.
2. For the requested resource, walk up the hierarchy to find if the key's scope contains it.
3. Check that the key's permissions include the required action.

Example: An application-scoped key for `api-server` can access any environment under `api-server`, but not environments under `web-frontend`.

### 7.4 Webhook Events

New webhook event types for releases:
- `release.created`, `release.started`, `release.promoted`
- `release.paused`, `release.completed`, `release.rolled_back`

Existing deployment events remain: `deploy.started`, `deploy.completed`, `deploy.failed`, `deploy.rolled_back`.

---

## 8. Migration Strategy

Since this redesign changes the hierarchy (adding Application between Project and Environment), the migration must:

1. Create `applications` table.
2. For each existing project, create a default application (same name + slug as the project) to preserve existing data.
3. Drop the `environments` unique constraint `(project_id, slug)`, rename `project_id` → `application_id`, add new FK and unique constraint `(application_id, slug)`. Point existing environments at the default application for their project.
4. Migrate `deployments`: drop legacy `pipeline_id`, `release_id`, TEXT `environment` columns. Rename `project_id` → `application_id`. Point at default application. Add `environment_id` FK.
5. Drop `deploy_pipelines` table (no longer needed).
6. Migrate `feature_flags` — add `application_id` column (nullable). Drop old `(project_id, key)` unique constraint, add partial indexes for project-level and app-level uniqueness. Deprecate `feature_flags.environment_id` (replaced by `flag_environment_state`).
7. Drop old `releases` and `release_environments` tables, create new `releases` and `release_flag_changes`.
8. Create `flag_environment_state` table. Seed from existing `feature_flags.enabled` — for each flag, create a state row for every environment in the flag's project (or application), using the flag's current `enabled` value as the initial state.
9. Create `settings` table.
10. Update `api_keys`: make `org_id` nullable, add `application_id` and `environment_id` columns, add `CHECK(num_nonnulls(...) = 1)` constraint. Migrate existing keys: keys with only `org_id` set stay as org-scoped, keys with `project_id` set become project-scoped (set `org_id` to null).
11. Update Go models: change `APIKey.OrgID` from `uuid.UUID` to `*uuid.UUID` (pointer/nullable). Update `Validate()` methods across all affected models.

This is a breaking migration — old release/pipeline data is dropped. Acceptable for current state (no production data).

---

## 9. Follow-On Initiatives

- **GitHub Webhook Integration** — receive deploy status events from GitHub Actions, link results to deployments on the dashboard. Stub the receiver endpoint in this work.
- **OAuth/SSO** — add GitHub/Google OAuth login alongside email/password.
- **Mobile App** — Flutter app updates to match new hierarchy.
- **SDK Updates** — update all SDKs to support session-sticky header, application-scoped evaluation. The flag evaluation endpoint now accepts `application_id` in the request body; SDKs must be updated to pass this. During transition, omitting `application_id` evaluates only project-level flags (backward compatible).

---

## 10. Success Criteria

1. A user can `curl | sh` to install the CLI, authenticate, create an org, project, application, and environments.
2. Deployments track code shipping with version/commit/artifact and rollout strategy.
3. Releases track flag change bundles with percentage rollout and session stickiness.
4. The web UI shows org switcher, project/app accordion sidebar, and flag detail with per-environment state.
5. API keys and settings cascade correctly through the hierarchy.
6. Session-sticky releases produce deterministic bucketing for the same session header value.
