# Platform Redesign: Data Model & Migrations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Implement the Org → Project → Application → Environment hierarchy, redefine Deployment (code shipping) and Release (flag change bundles), add flag_environment_state, hierarchical API keys, and cascading settings.

**Architecture:** Single PostgreSQL migration adds the Application entity between Project and Environment, restructures Deployments to own version/artifact/commit data, replaces Releases with flag-change-bundle semantics, and adds per-environment flag state. Go models and repositories are updated to match. Existing data is preserved via a "default application" migration strategy.

**Tech Stack:** PostgreSQL 16, Go 1.22, pgx v5, golang-migrate, Gin

**Spec:** `docs/superpowers/specs/2026-03-28-platform-redesign-design.md`

---

## File Structure

### New Files
- `migrations/029_platform_redesign.up.sql` — single migration for all schema changes
- `migrations/029_platform_redesign.down.sql` — rollback migration
- `internal/models/application.go` — Application model
- `internal/models/application_test.go` — Application model tests
- `internal/models/release_flag_change.go` — ReleaseFlagChange model
- `internal/models/release_flag_change_test.go` — tests
- `internal/models/flag_environment_state.go` — FlagEnvironmentState model
- `internal/models/flag_environment_state_test.go` — tests
- `internal/models/setting.go` — Setting model
- `internal/models/setting_test.go` — tests
- `internal/platform/database/postgres/applications.go` — Application repository
- `internal/platform/database/postgres/applications_test.go` — tests
- `internal/platform/database/postgres/flag_env_state.go` — FlagEnvironmentState repository
- `internal/platform/database/postgres/flag_env_state_test.go` — tests
- `internal/platform/database/postgres/settings.go` — Settings repository
- `internal/platform/database/postgres/settings_test.go` — tests

### Modified Files
- `internal/models/deployment.go` — Remove pipeline/release refs, add ApplicationID, keep typed DeployStatus/DeployStrategyType
- `internal/models/deployment_test.go` — Update tests
- `internal/models/release.go` — Complete rewrite: flag-change-bundle semantics with typed ReleaseStatus
- `internal/models/release_test.go` — Complete rewrite
- `internal/models/flag.go` — Add ApplicationID field, relax EnvironmentID validation for project-wide flags
- `internal/models/flag_test.go` — Update tests
- `internal/models/api_key.go` — Make OrgID nullable, add ApplicationID/EnvironmentID, keep []APIKeyScope
- `internal/models/api_key_test.go` — Update tests
- `internal/models/project.go` — Environment struct moves application_id ref
- `internal/platform/database/postgres/deploy.go` — Update for application_id, remove phase/pipeline methods
- `internal/platform/database/postgres/flags.go` — Update for application_id
- `internal/platform/database/postgres/releases.go` �� Complete rewrite for new Release model
- `internal/platform/database/postgres/apikeys.go` — Update for hierarchical scope
- `internal/deploy/repository.go` — Update DeployRepository interface: remove phase/pipeline methods, projectID → applicationID
- `internal/deploy/service.go` — Update DeployService: projectID → applicationID in method signatures
- `internal/deploy/handler.go` — Update routes and handler methods for applicationID
- `internal/deploy/strategies/` — Update strategy files for model type changes
- `internal/releases/repository.go` — Complete rewrite of ReleaseRepository interface
- `internal/releases/service.go` — Complete rewrite for new Release model
- `internal/releases/handler.go` — Update routes and handlers for new Release model

---

## Task 1: Write the Migration

**Files:**
- Create: `migrations/029_platform_redesign.up.sql`
- Create: `migrations/029_platform_redesign.down.sql`

- [x] **Step 1: Write the up migration**

```sql
-- migrations/029_platform_redesign.up.sql
-- Platform Redesign: Org → Project → Application → Environment hierarchy

-- 1. Create applications table
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

-- 2. Create a default application for each existing project
INSERT INTO applications (id, project_id, name, slug, description, repo_url, created_at, updated_at)
SELECT
    gen_random_uuid(),
    id,
    name,
    slug,
    description,
    repo_url,
    created_at,
    COALESCE(updated_at, now())
FROM projects
WHERE EXISTS (SELECT 1 FROM projects LIMIT 1);

-- 3. Migrate environments: project_id → application_id
ALTER TABLE environments DROP CONSTRAINT IF EXISTS environments_project_id_slug_key;
ALTER TABLE environments DROP CONSTRAINT IF EXISTS environments_project_id_fkey;

-- Update environment rows to point at the default application for their project
UPDATE environments e
SET project_id = a.id
FROM applications a
WHERE a.project_id = e.project_id;

ALTER TABLE environments RENAME COLUMN project_id TO application_id;
ALTER TABLE environments ADD CONSTRAINT fk_environments_application
    FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE CASCADE;
ALTER TABLE environments ADD CONSTRAINT environments_application_id_slug_key
    UNIQUE (application_id, slug);
CREATE INDEX IF NOT EXISTS idx_environments_application ON environments(application_id);

-- 4. Migrate deployments
-- Drop legacy FKs
ALTER TABLE deployments DROP CONSTRAINT IF EXISTS deployments_pipeline_id_fkey;
ALTER TABLE deployments DROP CONSTRAINT IF EXISTS fk_deployments_release_id;
ALTER TABLE deployments DROP COLUMN IF EXISTS pipeline_id;
ALTER TABLE deployments DROP COLUMN IF EXISTS release_id;
ALTER TABLE deployments DROP COLUMN IF EXISTS environment;
ALTER TABLE deployments DROP COLUMN IF EXISTS metadata;

-- Update deployment rows to point at default application
UPDATE deployments d
SET project_id = a.id
FROM applications a
WHERE a.project_id = d.project_id
AND d.project_id IS NOT NULL;

ALTER TABLE deployments RENAME COLUMN project_id TO application_id;
ALTER TABLE deployments ADD CONSTRAINT fk_deployments_application
    FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE CASCADE;
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS environment_id UUID
    REFERENCES environments(id) ON DELETE SET NULL;
-- Rename initiated_by to created_by for consistency with Go model
ALTER TABLE deployments RENAME COLUMN initiated_by TO created_by;

ALTER TABLE deployments ADD COLUMN IF NOT EXISTS version TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS commit_sha TEXT;
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS artifact TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_deployments_application ON deployments(application_id);
CREATE INDEX IF NOT EXISTS idx_deployments_environment ON deployments(environment_id);

-- 5. Drop legacy tables
DROP TABLE IF EXISTS release_environments;
DROP TABLE IF EXISTS releases;
DROP TABLE IF EXISTS deployment_phases;
DROP TABLE IF EXISTS deploy_pipelines;

-- 6. Create new releases table (flag-change bundles)
CREATE TABLE releases (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id  UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT,
    session_sticky  BOOLEAN NOT NULL DEFAULT false,
    sticky_header   TEXT,
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

-- 7. Create release_flag_changes table
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

-- 8. Modify feature_flags: add application_id, fix unique constraints
ALTER TABLE feature_flags ADD COLUMN IF NOT EXISTS application_id UUID
    REFERENCES applications(id) ON DELETE CASCADE;

-- Drop old unique constraint
ALTER TABLE feature_flags DROP CONSTRAINT IF EXISTS feature_flags_project_id_key_key;
DROP INDEX IF EXISTS feature_flags_project_id_key_key;
DROP INDEX IF EXISTS deploy_idx_feature_flags_project_key;

-- Partial indexes for project-level and app-level flag uniqueness
CREATE UNIQUE INDEX idx_flags_project_key
    ON feature_flags(project_id, key) WHERE application_id IS NULL;
CREATE UNIQUE INDEX idx_flags_application_key
    ON feature_flags(application_id, key) WHERE application_id IS NOT NULL;

-- 9. Create flag_environment_state table
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

-- Seed flag_environment_state from existing flags + environments
INSERT INTO flag_environment_state (flag_id, environment_id, enabled, value, updated_at)
SELECT
    f.id,
    e.id,
    f.enabled,
    to_jsonb(f.default_value),
    now()
FROM feature_flags f
CROSS JOIN environments e
INNER JOIN applications a ON a.project_id = f.project_id
WHERE e.application_id = a.id
ON CONFLICT (flag_id, environment_id) DO NOTHING;

-- 10. Modify api_keys: make org_id nullable, add scope columns
ALTER TABLE api_keys ALTER COLUMN org_id DROP NOT NULL;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS application_id UUID
    REFERENCES applications(id) ON DELETE CASCADE;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS environment_id UUID
    REFERENCES environments(id) ON DELETE CASCADE;

-- Migrate existing keys: project-scoped keys should have org_id nulled
UPDATE api_keys SET org_id = NULL WHERE project_id IS NOT NULL;

-- Add scope check constraint
ALTER TABLE api_keys ADD CONSTRAINT chk_api_keys_single_scope
    CHECK (num_nonnulls(org_id, project_id, application_id, environment_id) = 1);

-- 11. Create settings table
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
    CONSTRAINT chk_settings_single_scope
        CHECK (num_nonnulls(org_id, project_id, application_id, environment_id) = 1)
);
CREATE UNIQUE INDEX idx_settings_org ON settings(org_id, key) WHERE org_id IS NOT NULL;
CREATE UNIQUE INDEX idx_settings_project ON settings(project_id, key) WHERE project_id IS NOT NULL;
CREATE UNIQUE INDEX idx_settings_app ON settings(application_id, key) WHERE application_id IS NOT NULL;
CREATE UNIQUE INDEX idx_settings_env ON settings(environment_id, key) WHERE environment_id IS NOT NULL;
```

- [x] **Step 2: Write the down migration**

```sql
-- migrations/029_platform_redesign.down.sql
-- Rollback: this is destructive and only useful during development

DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS flag_environment_state;
DROP TABLE IF EXISTS release_flag_changes;
DROP TABLE IF EXISTS releases;

-- Restore api_keys
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS chk_api_keys_single_scope;
ALTER TABLE api_keys DROP COLUMN IF EXISTS environment_id;
ALTER TABLE api_keys DROP COLUMN IF EXISTS application_id;

-- Restore feature_flags
ALTER TABLE feature_flags DROP COLUMN IF EXISTS application_id;
DROP INDEX IF EXISTS idx_flags_project_key;
DROP INDEX IF EXISTS idx_flags_application_key;

-- Restore deployments
ALTER TABLE deployments RENAME COLUMN application_id TO project_id;
ALTER TABLE deployments DROP CONSTRAINT IF EXISTS fk_deployments_application;
ALTER TABLE deployments DROP COLUMN IF EXISTS environment_id;

-- Restore environments
ALTER TABLE environments DROP CONSTRAINT IF EXISTS fk_environments_application;
ALTER TABLE environments DROP CONSTRAINT IF EXISTS environments_application_id_slug_key;
ALTER TABLE environments RENAME COLUMN application_id TO project_id;

-- Drop applications
DROP TABLE IF EXISTS applications;
```

- [x] **Step 3: Run the migration against the Docker database**

Run:
```bash
cd deploy/selfhost && docker compose --profile setup run --rm migrate
```
Expected: All migrations pass including 029.

- [x] **Step 4: Verify the schema**

Run:
```bash
cd deploy/selfhost && docker compose exec postgres psql -U deploysentry -d deploysentry -c "SET search_path TO deploy; \dt"
```
Expected: See `applications`, `releases`, `release_flag_changes`, `flag_environment_state`, `settings` in the table list. `deploy_pipelines` should be gone.

- [x] **Step 5: Commit**

```bash
git add migrations/029_platform_redesign.up.sql migrations/029_platform_redesign.down.sql
git commit -m "feat: add migration 029 for platform redesign hierarchy"
```

---

## Task 2: Application Model

**Files:**
- Create: `internal/models/application.go`
- Create: `internal/models/application_test.go`

- [x] **Step 1: Write the failing test**

```go
// internal/models/application_test.go
package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestApplication_Validate(t *testing.T) {
	valid := Application{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Name:      "api-server",
		Slug:      "api-server",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}

	// Missing name
	noName := valid
	noName.Name = ""
	if err := noName.Validate(); err == nil {
		t.Fatal("expected error for missing name")
	}

	// Missing slug
	noSlug := valid
	noSlug.Slug = ""
	if err := noSlug.Validate(); err == nil {
		t.Fatal("expected error for missing slug")
	}

	// Missing project ID
	noProject := valid
	noProject.ProjectID = uuid.Nil
	if err := noProject.Validate(); err == nil {
		t.Fatal("expected error for missing project_id")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/models/ -run TestApplication_Validate -v`
Expected: FAIL — `Application` type not defined.

- [x] **Step 3: Write the Application model**

```go
// internal/models/application.go
package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Application represents a deployable unit within a project.
// Each application owns its own environments, deployments, releases, and release flags.
type Application struct {
	ID          uuid.UUID  `json:"id"`
	ProjectID   uuid.UUID  `json:"project_id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description string     `json:"description,omitempty"`
	RepoURL     string     `json:"repo_url,omitempty"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (a *Application) Validate() error {
	if a.ProjectID == uuid.Nil {
		return errors.New("project_id is required")
	}
	if a.Name == "" {
		return errors.New("name is required")
	}
	if a.Slug == "" {
		return errors.New("slug is required")
	}
	return nil
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/models/ -run TestApplication_Validate -v`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add internal/models/application.go internal/models/application_test.go
git commit -m "feat: add Application model"
```

---

## Task 3: Update Deployment Model

**Files:**
- Modify: `internal/models/deployment.go`
- Modify: `internal/models/deployment_test.go`

- [x] **Step 1: Write a failing test for the updated model**

Add to `internal/models/deployment_test.go`:

```go
func TestDeployment_Validate_ApplicationID(t *testing.T) {
	d := Deployment{
		ID:            uuid.New(),
		ApplicationID: uuid.New(),
		EnvironmentID: uuid.New(),
		Strategy:      StrategyCanary,
		Status:        DeployPending,
		Version:       "1.2.3",
		CreatedAt:     time.Now().UTC(),
	}

	if err := d.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}

	// Missing application_id
	noApp := d
	noApp.ApplicationID = uuid.Nil
	if err := noApp.Validate(); err == nil {
		t.Fatal("expected error for missing application_id")
	}

	// Missing environment_id
	noEnv := d
	noEnv.EnvironmentID = uuid.Nil
	if err := noEnv.Validate(); err == nil {
		t.Fatal("expected error for missing environment_id")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/models/ -run TestDeployment_Validate_ApplicationID -v`
Expected: FAIL — `ApplicationID` field not found on Deployment.

- [x] **Step 3: Update the Deployment model**

In `internal/models/deployment.go`:
- Rename `ProjectID` to `ApplicationID` (update the struct field name, json tag, and all references in the file)
- Remove `PipelineID` and `ReleaseID` fields
- Ensure `EnvironmentID` is `uuid.UUID` (not pointer)
- Ensure `Version`, `CommitSHA`, `Artifact` fields exist
- Update `Validate()`: require `ApplicationID` and `EnvironmentID` instead of `PipelineID`

Key changes to the struct (keep existing `DeployStatus` and `DeployStrategyType` typed aliases):
```go
type Deployment struct {
	ID            uuid.UUID          `json:"id"`
	ApplicationID uuid.UUID          `json:"application_id"`   // was ProjectID
	EnvironmentID uuid.UUID          `json:"environment_id"`
	Strategy      DeployStrategyType `json:"strategy"`         // keep typed
	Status        DeployStatus       `json:"status"`           // keep typed
	Version       string             `json:"version"`
	CommitSHA     string             `json:"commit_sha,omitempty"`
	Artifact      string             `json:"artifact,omitempty"`
	TrafficPct    int                `json:"traffic_percent"`
	CreatedBy     uuid.UUID          `json:"created_by"`       // keep non-pointer
	StartedAt     *time.Time         `json:"started_at,omitempty"`
	CompletedAt   *time.Time         `json:"completed_at,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}
```

Remove these fields: `PipelineID`, `ReleaseID`, `Metadata`.

Update `Validate()`:
```go
func (d *Deployment) Validate() error {
	if d.ApplicationID == uuid.Nil {
		return errors.New("application_id is required")
	}
	if d.EnvironmentID == uuid.Nil {
		return errors.New("environment_id is required")
	}
	if !isValidStrategy(string(d.Strategy)) {
		return fmt.Errorf("invalid strategy: %s", d.Strategy)
	}
	if !isValidDeployStatus(string(d.Status)) {
		return fmt.Errorf("invalid status: %s", d.Status)
	}
	return nil
}
```

Keep the existing state machine (`validTransitions`, `TransitionTo`) and all 8 statuses.
Keep existing `DeployStatus`, `DeployStrategyType` type aliases and constants.

- [x] **Step 4: Run all model tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/models/ -v`
Expected: All tests pass including the new one.

- [x] **Step 5: Commit**

```bash
git add internal/models/deployment.go internal/models/deployment_test.go
git commit -m "feat: update Deployment model for application-scoped hierarchy"
```

---

## Task 4: Rewrite Release Model

**Files:**
- Modify: `internal/models/release.go`
- Modify: `internal/models/release_test.go`
- Create: `internal/models/release_flag_change.go`
- Create: `internal/models/release_flag_change_test.go`

- [x] **Step 1: Write failing tests for the new Release model**

```go
// internal/models/release_test.go — complete rewrite
package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRelease_Validate(t *testing.T) {
	valid := Release{
		ID:             uuid.New(),
		ApplicationID:  uuid.New(),
		Name:           "Enable checkout v2",
		TrafficPercent: 25,
		Status:         ReleaseDraft,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}

	// Missing application_id
	noApp := valid
	noApp.ApplicationID = uuid.Nil
	if err := noApp.Validate(); err == nil {
		t.Fatal("expected error for missing application_id")
	}

	// Missing name
	noName := valid
	noName.Name = ""
	if err := noName.Validate(); err == nil {
		t.Fatal("expected error for missing name")
	}

	// Session sticky without header
	sticky := valid
	sticky.SessionSticky = true
	sticky.StickyHeader = ""
	if err := sticky.Validate(); err == nil {
		t.Fatal("expected error for session_sticky without sticky_header")
	}

	// Session sticky with header — valid
	sticky.StickyHeader = "X-Session-ID"
	if err := sticky.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestRelease_TransitionTo(t *testing.T) {
	r := Release{Status: ReleaseDraft}

	// Draft → RollingOut
	if err := r.TransitionTo(ReleaseRollingOut); err != nil {
		t.Fatalf("expected draft→rolling_out, got %v", err)
	}

	// RollingOut → Paused
	if err := r.TransitionTo(ReleasePaused); err != nil {
		t.Fatalf("expected rolling_out→paused, got %v", err)
	}

	// Paused → Completed (invalid)
	if err := r.TransitionTo(ReleaseCompleted); err == nil {
		t.Fatal("expected error for paused→completed")
	}

	// Paused → RollingOut
	if err := r.TransitionTo(ReleaseRollingOut); err != nil {
		t.Fatalf("expected paused→rolling_out, got %v", err)
	}

	// RollingOut → Completed
	if err := r.TransitionTo(ReleaseCompleted); err != nil {
		t.Fatalf("expected rolling_out→completed, got %v", err)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/models/ -run TestRelease -v`
Expected: FAIL — new Release fields/constants not defined.

- [x] **Step 3: Rewrite the Release model**

```go
// internal/models/release.go — complete rewrite
package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Release statuses
const (
	ReleaseDraft      = "draft"
	ReleaseRollingOut = "rolling_out"
	ReleasePaused     = "paused"
	ReleaseCompleted  = "completed"
	ReleaseRolledBack = "rolled_back"
)

var releaseTransitions = map[string][]string{
	ReleaseDraft:      {ReleaseRollingOut},
	ReleaseRollingOut: {ReleasePaused, ReleaseCompleted, ReleaseRolledBack},
	ReleasePaused:     {ReleaseRollingOut, ReleaseRolledBack},
	// Terminal states: Completed, RolledBack — no transitions out
}

// Release represents a bundle of flag changes applied with a rollout strategy.
type Release struct {
	ID             uuid.UUID  `json:"id"`
	ApplicationID  uuid.UUID  `json:"application_id"`
	Name           string     `json:"name"`
	Description    string     `json:"description,omitempty"`
	SessionSticky  bool       `json:"session_sticky"`
	StickyHeader   string     `json:"sticky_header,omitempty"`
	TrafficPercent int        `json:"traffic_percent"`
	Status         string     `json:"status"`
	CreatedBy      *uuid.UUID `json:"created_by,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (r *Release) Validate() error {
	if r.ApplicationID == uuid.Nil {
		return errors.New("application_id is required")
	}
	if r.Name == "" {
		return errors.New("name is required")
	}
	if r.SessionSticky && r.StickyHeader == "" {
		return errors.New("sticky_header is required when session_sticky is true")
	}
	if r.TrafficPercent < 0 || r.TrafficPercent > 100 {
		return errors.New("traffic_percent must be between 0 and 100")
	}
	return nil
}

func (r *Release) TransitionTo(newStatus string) error {
	allowed, ok := releaseTransitions[r.Status]
	if !ok {
		return fmt.Errorf("no transitions from terminal status %q", r.Status)
	}
	for _, s := range allowed {
		if s == newStatus {
			r.Status = newStatus
			return nil
		}
	}
	return fmt.Errorf("invalid transition: %s → %s", r.Status, newStatus)
}
```

- [x] **Step 4: Write the ReleaseFlagChange model**

```go
// internal/models/release_flag_change.go
package models

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ReleaseFlagChange tracks a single flag change within a release.
type ReleaseFlagChange struct {
	ID              uuid.UUID        `json:"id"`
	ReleaseID       uuid.UUID        `json:"release_id"`
	FlagID          uuid.UUID        `json:"flag_id"`
	EnvironmentID   uuid.UUID        `json:"environment_id"`
	PreviousValue   *json.RawMessage `json:"previous_value,omitempty"`
	NewValue        *json.RawMessage `json:"new_value,omitempty"`
	PreviousEnabled *bool            `json:"previous_enabled,omitempty"`
	NewEnabled      *bool            `json:"new_enabled,omitempty"`
	AppliedAt       *time.Time       `json:"applied_at,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
}

func (c *ReleaseFlagChange) Validate() error {
	if c.ReleaseID == uuid.Nil {
		return errors.New("release_id is required")
	}
	if c.FlagID == uuid.Nil {
		return errors.New("flag_id is required")
	}
	if c.EnvironmentID == uuid.Nil {
		return errors.New("environment_id is required")
	}
	return nil
}
```

- [x] **Step 5: Write ReleaseFlagChange test**

```go
// internal/models/release_flag_change_test.go
package models

import (
	"testing"

	"github.com/google/uuid"
)

func TestReleaseFlagChange_Validate(t *testing.T) {
	valid := ReleaseFlagChange{
		ReleaseID:     uuid.New(),
		FlagID:        uuid.New(),
		EnvironmentID: uuid.New(),
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}

	noRelease := valid
	noRelease.ReleaseID = uuid.Nil
	if err := noRelease.Validate(); err == nil {
		t.Fatal("expected error for missing release_id")
	}
}
```

- [x] **Step 6: Run all tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/models/ -v`
Expected: All pass.

- [x] **Step 7: Commit**

```bash
git add internal/models/release.go internal/models/release_test.go \
    internal/models/release_flag_change.go internal/models/release_flag_change_test.go
git commit -m "feat: rewrite Release model as flag-change bundle, add ReleaseFlagChange"
```

---

## Task 5: FlagEnvironmentState Model

**Files:**
- Create: `internal/models/flag_environment_state.go`
- Create: `internal/models/flag_environment_state_test.go`

- [x] **Step 1: Write failing test**

```go
// internal/models/flag_environment_state_test.go
package models

import (
	"testing"

	"github.com/google/uuid"
)

func TestFlagEnvironmentState_Validate(t *testing.T) {
	valid := FlagEnvironmentState{
		FlagID:        uuid.New(),
		EnvironmentID: uuid.New(),
		Enabled:       true,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}

	noFlag := valid
	noFlag.FlagID = uuid.Nil
	if err := noFlag.Validate(); err == nil {
		t.Fatal("expected error for missing flag_id")
	}

	noEnv := valid
	noEnv.EnvironmentID = uuid.Nil
	if err := noEnv.Validate(); err == nil {
		t.Fatal("expected error for missing environment_id")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/models/ -run TestFlagEnvironmentState -v`
Expected: FAIL

- [x] **Step 3: Write the model**

```go
// internal/models/flag_environment_state.go
package models

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// FlagEnvironmentState tracks the enabled state and value of a flag in a specific environment.
type FlagEnvironmentState struct {
	ID            uuid.UUID        `json:"id"`
	FlagID        uuid.UUID        `json:"flag_id"`
	EnvironmentID uuid.UUID        `json:"environment_id"`
	Enabled       bool             `json:"enabled"`
	Value         *json.RawMessage `json:"value,omitempty"`
	UpdatedBy     *uuid.UUID       `json:"updated_by,omitempty"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

func (s *FlagEnvironmentState) Validate() error {
	if s.FlagID == uuid.Nil {
		return errors.New("flag_id is required")
	}
	if s.EnvironmentID == uuid.Nil {
		return errors.New("environment_id is required")
	}
	return nil
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/models/ -run TestFlagEnvironmentState -v`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add internal/models/flag_environment_state.go internal/models/flag_environment_state_test.go
git commit -m "feat: add FlagEnvironmentState model"
```

---

## Task 6: Setting Model

**Files:**
- Create: `internal/models/setting.go`
- Create: `internal/models/setting_test.go`

- [x] **Step 1: Write failing test**

```go
// internal/models/setting_test.go
package models

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestSetting_Validate(t *testing.T) {
	orgID := uuid.New()
	val := json.RawMessage(`"canary"`)
	valid := Setting{
		OrgID: &orgID,
		Key:   "deployment.default_strategy",
		Value: val,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}

	// No scope set
	noScope := Setting{Key: "foo", Value: val}
	if err := noScope.Validate(); err == nil {
		t.Fatal("expected error for no scope")
	}

	// Multiple scopes set
	projID := uuid.New()
	multiScope := Setting{OrgID: &orgID, ProjectID: &projID, Key: "foo", Value: val}
	if err := multiScope.Validate(); err == nil {
		t.Fatal("expected error for multiple scopes")
	}

	// Missing key
	noKey := Setting{OrgID: &orgID, Value: val}
	if err := noKey.Validate(); err == nil {
		t.Fatal("expected error for missing key")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/models/ -run TestSetting_Validate -v`
Expected: FAIL

- [x] **Step 3: Write the model**

```go
// internal/models/setting.go
package models

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Setting stores a configuration value at a specific scope level.
// Exactly one of OrgID, ProjectID, ApplicationID, EnvironmentID must be set.
type Setting struct {
	ID            uuid.UUID       `json:"id"`
	OrgID         *uuid.UUID      `json:"org_id,omitempty"`
	ProjectID     *uuid.UUID      `json:"project_id,omitempty"`
	ApplicationID *uuid.UUID      `json:"application_id,omitempty"`
	EnvironmentID *uuid.UUID      `json:"environment_id,omitempty"`
	Key           string          `json:"key"`
	Value         json.RawMessage `json:"value"`
	UpdatedBy     *uuid.UUID      `json:"updated_by,omitempty"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

func (s *Setting) Validate() error {
	if s.Key == "" {
		return errors.New("key is required")
	}
	if len(s.Value) == 0 {
		return errors.New("value is required")
	}

	scopeCount := 0
	if s.OrgID != nil {
		scopeCount++
	}
	if s.ProjectID != nil {
		scopeCount++
	}
	if s.ApplicationID != nil {
		scopeCount++
	}
	if s.EnvironmentID != nil {
		scopeCount++
	}
	if scopeCount == 0 {
		return errors.New("exactly one scope (org_id, project_id, application_id, environment_id) must be set")
	}
	if scopeCount > 1 {
		return errors.New("only one scope (org_id, project_id, application_id, environment_id) may be set")
	}
	return nil
}

// ScopeLevel returns which hierarchy level this setting is scoped to.
func (s *Setting) ScopeLevel() string {
	switch {
	case s.EnvironmentID != nil:
		return "environment"
	case s.ApplicationID != nil:
		return "application"
	case s.ProjectID != nil:
		return "project"
	case s.OrgID != nil:
		return "org"
	default:
		return ""
	}
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/models/ -run TestSetting_Validate -v`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add internal/models/setting.go internal/models/setting_test.go
git commit -m "feat: add Setting model with scope validation"
```

---

## Task 7: Update FeatureFlag Model

**Files:**
- Modify: `internal/models/flag.go`
- Modify: `internal/models/flag_test.go`

- [x] **Step 1: Write failing test for ApplicationID on flags**

Add to `internal/models/flag_test.go`:

```go
func TestFeatureFlag_Validate_ApplicationScope(t *testing.T) {
	appID := uuid.New()

	// Release flag requires application_id
	releaseFlag := FeatureFlag{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Key:       "checkout-v2",
		Name:      "Checkout V2",
		FlagType:  "boolean",
		Category:  CategoryRelease,
	}
	if err := releaseFlag.Validate(); err == nil {
		t.Fatal("expected error for release flag without application_id")
	}

	releaseFlag.ApplicationID = &appID
	if err := releaseFlag.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}

	// Feature flag does not require application_id
	featureFlag := FeatureFlag{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Key:       "dark-mode",
		Name:      "Dark Mode",
		FlagType:  "boolean",
		Category:  CategoryFeature,
	}
	if err := featureFlag.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/models/ -run TestFeatureFlag_Validate_ApplicationScope -v`
Expected: FAIL — `ApplicationID` field not found.

- [x] **Step 3: Update the FeatureFlag model**

In `internal/models/flag.go`:
- Add `ApplicationID *uuid.UUID` field with `json:"application_id,omitempty"`
- Update `Validate()`: if `Category == CategoryRelease` and `ApplicationID` is nil, return error

```go
// Add to FeatureFlag struct:
ApplicationID *uuid.UUID `json:"application_id,omitempty"`

// Update Validate():
// 1. Remove the existing check: if f.EnvironmentID == uuid.Nil { return error }
//    EnvironmentID is now managed via flag_environment_state table, not on the flag itself.
// 2. Add application_id validation:
if f.Category == CategoryRelease && f.ApplicationID == nil {
    return errors.New("application_id is required for release flags")
}
```

- [x] **Step 4: Run all model tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/models/ -v`
Expected: All pass.

- [x] **Step 5: Commit**

```bash
git add internal/models/flag.go internal/models/flag_test.go
git commit -m "feat: add ApplicationID to FeatureFlag, enforce for release category"
```

---

## Task 8: Update APIKey Model

**Files:**
- Modify: `internal/models/api_key.go`
- Modify: `internal/models/api_key_test.go`

- [x] **Step 1: Write failing test for hierarchical scoping**

Add to `internal/models/api_key_test.go`:

```go
func TestAPIKey_Validate_HierarchicalScope(t *testing.T) {
	orgID := uuid.New()

	// Org-scoped key — valid
	orgKey := APIKey{
		ID:     uuid.New(),
		OrgID:  &orgID,
		Name:   "Org Admin Key",
		Scopes: []APIKeyScope{ScopeAdmin},
	}
	if err := orgKey.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}

	// No scope set — invalid
	noScope := APIKey{
		ID:     uuid.New(),
		Name:   "Bad Key",
		Scopes: []APIKeyScope{ScopeFlagsRead},
	}
	if err := noScope.Validate(); err == nil {
		t.Fatal("expected error for no scope")
	}

	// Multiple scopes set — invalid
	projID := uuid.New()
	multiScope := APIKey{
		ID:        uuid.New(),
		OrgID:     &orgID,
		ProjectID: &projID,
		Name:      "Multi Key",
		Scopes:    []APIKeyScope{ScopeFlagsRead},
	}
	if err := multiScope.Validate(); err == nil {
		t.Fatal("expected error for multiple scopes")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/models/ -run TestAPIKey_Validate_HierarchicalScope -v`
Expected: FAIL — `OrgID` is not a pointer type yet.

- [x] **Step 3: Update the APIKey model**

In `internal/models/api_key.go`:
- Change `OrgID uuid.UUID` to `OrgID *uuid.UUID` with `json:"org_id,omitempty"`
- Add `ApplicationID *uuid.UUID` with `json:"application_id,omitempty"`
- Add `EnvironmentID *uuid.UUID` with `json:"environment_id,omitempty"`
- Update `Validate()`: enforce exactly one scope is set

```go
// Update Validate() scope check:
scopeCount := 0
if k.OrgID != nil {
    scopeCount++
}
if k.ProjectID != nil {
    scopeCount++
}
if k.ApplicationID != nil {
    scopeCount++
}
if k.EnvironmentID != nil {
    scopeCount++
}
if scopeCount == 0 {
    return errors.New("exactly one scope (org_id, project_id, application_id, environment_id) must be set")
}
if scopeCount > 1 {
    return errors.New("only one scope may be set")
}
```

- [x] **Step 4: Fix any other tests broken by OrgID pointer change**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/models/ -v`

Fix any existing tests that set `OrgID` as a value — change to pointer:
```go
orgID := uuid.New()
key.OrgID = &orgID
```

- [x] **Step 5: Run all model tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/models/ -v`
Expected: All pass.

- [x] **Step 6: Commit**

```bash
git add internal/models/api_key.go internal/models/api_key_test.go
git commit -m "feat: update APIKey for hierarchical scope (org/project/app/env)"
```

---

## Task 9: Update Environment Model

**Files:**
- Modify: `internal/models/project.go` (Environment struct is defined here)

- [x] **Step 1: Update Environment struct**

In `internal/models/project.go`, the `Environment` struct has `ProjectID`. Rename to `ApplicationID`:

```go
type Environment struct {
	ID              uuid.UUID `json:"id"`
	ApplicationID   uuid.UUID `json:"application_id"`  // was ProjectID
	Name            string    `json:"name"`
	Slug            string    `json:"slug"`
	IsProduction    bool      `json:"is_production"`
	RequiresApproval bool    `json:"requires_approval"`
	SortOrder       int       `json:"sort_order"`
	CreatedAt       time.Time `json:"created_at"`
}
```

Update `Validate()` to check `ApplicationID` instead of `ProjectID`.

- [x] **Step 2: Run all model tests to check for breakage**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/models/ -v`
Expected: All pass (fix any that reference `ProjectID` on Environment).

- [x] **Step 3: Commit**

```bash
git add internal/models/project.go
git commit -m "feat: update Environment to reference ApplicationID"
```

---

## Task 10: Update Repositories

**Files:**
- Modify: `internal/platform/database/postgres/deploy.go`
- Modify: `internal/platform/database/postgres/flags.go`
- Modify: `internal/platform/database/postgres/releases.go`
- Modify: `internal/platform/database/postgres/apikeys.go`
- Create: `internal/platform/database/postgres/applications.go`
- Create: `internal/platform/database/postgres/flag_env_state.go`
- Create: `internal/platform/database/postgres/settings.go`

This is the largest task. Each repository follows the existing pattern: struct wrapping `*pgxpool.Pool`, constructor, scan helpers, CRUD methods.

- [x] **Step 1: Create ApplicationRepository**

```go
// internal/platform/database/postgres/applications.go
package postgres

import (
	"context"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ApplicationRepository struct {
	pool *pgxpool.Pool
}

func NewApplicationRepository(pool *pgxpool.Pool) *ApplicationRepository {
	return &ApplicationRepository{pool: pool}
}

const appSelectCols = `id, project_id, name, slug, description, repo_url, created_by, created_at, updated_at`

func scanApplication(row pgx.Row) (*models.Application, error) {
	var a models.Application
	err := row.Scan(&a.ID, &a.ProjectID, &a.Name, &a.Slug, &a.Description, &a.RepoURL, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *ApplicationRepository) Create(ctx context.Context, a *models.Application) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO applications (id, project_id, name, slug, description, repo_url, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		a.ID, a.ProjectID, a.Name, a.Slug, a.Description, a.RepoURL, a.CreatedBy)
	return err
}

func (r *ApplicationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Application, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+appSelectCols+` FROM applications WHERE id = $1`, id)
	return scanApplication(row)
}

func (r *ApplicationRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.Application, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+appSelectCols+` FROM applications WHERE project_id = $1 ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []models.Application
	for rows.Next() {
		a, err := scanApplication(rows)
		if err != nil {
			return nil, err
		}
		apps = append(apps, *a)
	}
	return apps, rows.Err()
}

func (r *ApplicationRepository) Update(ctx context.Context, a *models.Application) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE applications SET name=$1, slug=$2, description=$3, repo_url=$4, updated_at=now() WHERE id=$5`,
		a.Name, a.Slug, a.Description, a.RepoURL, a.ID)
	return err
}

func (r *ApplicationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM applications WHERE id = $1`, id)
	return err
}
```

- [x] **Step 2: Create FlagEnvironmentStateRepository**

```go
// internal/platform/database/postgres/flag_env_state.go
package postgres

import (
	"context"
	"errors"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FlagEnvStateRepository struct {
	pool *pgxpool.Pool
}

func NewFlagEnvStateRepository(pool *pgxpool.Pool) *FlagEnvStateRepository {
	return &FlagEnvStateRepository{pool: pool}
}

func scanFlagEnvState(row pgx.Row) (*models.FlagEnvironmentState, error) {
	var s models.FlagEnvironmentState
	err := row.Scan(&s.ID, &s.FlagID, &s.EnvironmentID, &s.Enabled, &s.Value, &s.UpdatedBy, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *FlagEnvStateRepository) GetByFlagAndEnv(ctx context.Context, flagID, envID uuid.UUID) (*models.FlagEnvironmentState, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, flag_id, environment_id, enabled, value, updated_by, updated_at
		 FROM flag_environment_state WHERE flag_id = $1 AND environment_id = $2`,
		flagID, envID)
	s, err := scanFlagEnvState(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

func (r *FlagEnvStateRepository) ListByFlag(ctx context.Context, flagID uuid.UUID) ([]models.FlagEnvironmentState, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, flag_id, environment_id, enabled, value, updated_by, updated_at
		 FROM flag_environment_state WHERE flag_id = $1`, flagID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []models.FlagEnvironmentState
	for rows.Next() {
		s, err := scanFlagEnvState(rows)
		if err != nil {
			return nil, err
		}
		states = append(states, *s)
	}
	return states, rows.Err()
}

func (r *FlagEnvStateRepository) Upsert(ctx context.Context, s *models.FlagEnvironmentState) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO flag_environment_state (flag_id, environment_id, enabled, value, updated_by)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (flag_id, environment_id)
		 DO UPDATE SET enabled = $3, value = $4, updated_by = $5, updated_at = now()`,
		s.FlagID, s.EnvironmentID, s.Enabled, s.Value, s.UpdatedBy)
	return err
}
```

- [x] **Step 3: Create SettingsRepository**

```go
// internal/platform/database/postgres/settings.go
package postgres

import (
	"context"
	"errors"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SettingsRepository struct {
	pool *pgxpool.Pool
}

func NewSettingsRepository(pool *pgxpool.Pool) *SettingsRepository {
	return &SettingsRepository{pool: pool}
}

func scanSetting(row pgx.Row) (*models.Setting, error) {
	var s models.Setting
	err := row.Scan(&s.ID, &s.OrgID, &s.ProjectID, &s.ApplicationID, &s.EnvironmentID, &s.Key, &s.Value, &s.UpdatedBy, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SettingsRepository) Upsert(ctx context.Context, s *models.Setting) error {
	// ON CONFLICT requires a unique index, not a CHECK constraint.
	// Route directly to scope-specific upsert.
	return r.upsertByScope(ctx, s)
}

func (r *SettingsRepository) upsertByScope(ctx context.Context, s *models.Setting) error {
	var query string
	var args []interface{}

	switch {
	case s.OrgID != nil:
		query = `INSERT INTO settings (org_id, key, value, updated_by) VALUES ($1, $2, $3, $4)
			ON CONFLICT (org_id, key) WHERE org_id IS NOT NULL DO UPDATE SET value = $3, updated_by = $4, updated_at = now()`
		args = []interface{}{s.OrgID, s.Key, s.Value, s.UpdatedBy}
	case s.ProjectID != nil:
		query = `INSERT INTO settings (project_id, key, value, updated_by) VALUES ($1, $2, $3, $4)
			ON CONFLICT (project_id, key) WHERE project_id IS NOT NULL DO UPDATE SET value = $3, updated_by = $4, updated_at = now()`
		args = []interface{}{s.ProjectID, s.Key, s.Value, s.UpdatedBy}
	case s.ApplicationID != nil:
		query = `INSERT INTO settings (application_id, key, value, updated_by) VALUES ($1, $2, $3, $4)
			ON CONFLICT (application_id, key) WHERE application_id IS NOT NULL DO UPDATE SET value = $3, updated_by = $4, updated_at = now()`
		args = []interface{}{s.ApplicationID, s.Key, s.Value, s.UpdatedBy}
	case s.EnvironmentID != nil:
		query = `INSERT INTO settings (environment_id, key, value, updated_by) VALUES ($1, $2, $3, $4)
			ON CONFLICT (environment_id, key) WHERE environment_id IS NOT NULL DO UPDATE SET value = $3, updated_by = $4, updated_at = now()`
		args = []interface{}{s.EnvironmentID, s.Key, s.Value, s.UpdatedBy}
	default:
		return errors.New("no scope set")
	}

	_, err := r.pool.Exec(ctx, query, args...)
	return err
}

// Resolve returns the effective value for a key, walking up the hierarchy:
// environment → application → project → org.
func (r *SettingsRepository) Resolve(ctx context.Context, key string, envID, appID, projectID, orgID uuid.UUID) (*models.Setting, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, org_id, project_id, application_id, environment_id, key, value, updated_by, updated_at
		 FROM settings
		 WHERE key = $1 AND (
		   environment_id = $2 OR
		   application_id = $3 OR
		   project_id = $4 OR
		   org_id = $5
		 )
		 ORDER BY
		   CASE WHEN environment_id IS NOT NULL THEN 1
		        WHEN application_id IS NOT NULL THEN 2
		        WHEN project_id IS NOT NULL THEN 3
		        WHEN org_id IS NOT NULL THEN 4
		   END
		 LIMIT 1`,
		key, envID, appID, projectID, orgID)

	s, err := scanSetting(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

func (r *SettingsRepository) ListByScope(ctx context.Context, scopeCol string, scopeID uuid.UUID) ([]models.Setting, error) {
	// scopeCol must be one of: org_id, project_id, application_id, environment_id
	// We use a safe whitelist instead of string interpolation.
	var query string
	switch scopeCol {
	case "org_id":
		query = `SELECT id, org_id, project_id, application_id, environment_id, key, value, updated_by, updated_at FROM settings WHERE org_id = $1`
	case "project_id":
		query = `SELECT id, org_id, project_id, application_id, environment_id, key, value, updated_by, updated_at FROM settings WHERE project_id = $1`
	case "application_id":
		query = `SELECT id, org_id, project_id, application_id, environment_id, key, value, updated_by, updated_at FROM settings WHERE application_id = $1`
	case "environment_id":
		query = `SELECT id, org_id, project_id, application_id, environment_id, key, value, updated_by, updated_at FROM settings WHERE environment_id = $1`
	default:
		return nil, errors.New("invalid scope column")
	}

	rows, err := r.pool.Query(ctx, query, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []models.Setting
	for rows.Next() {
		s, err := scanSetting(rows)
		if err != nil {
			return nil, err
		}
		settings = append(settings, *s)
	}
	return settings, rows.Err()
}
```

- [x] **Step 4: Update DeployRepository**

In `internal/platform/database/postgres/deploy.go`:
- Replace `project_id` with `application_id` in all queries
- Remove references to `pipeline_id`, `release_id`
- Add `environment_id`, `version`, `commit_sha`, `artifact` to scan helpers and queries
- Remove `DeployPipeline` and `DeploymentPhase` methods (tables dropped)

- [x] **Step 5: Update FlagRepository**

In `internal/platform/database/postgres/flags.go`:
- Add `application_id` to scan helper and INSERT/SELECT queries
- Support filtering by `application_id` in list queries

- [x] **Step 6: Rewrite ReleaseRepository**

In `internal/platform/database/postgres/releases.go`:
- Replace all Release queries with the new schema (application_id, name, session_sticky, etc.)
- Remove `ReleaseEnvironment` methods
- Add `ReleaseFlagChange` CRUD methods

- [x] **Step 7: Update APIKeyRepository**

In `internal/platform/database/postgres/apikeys.go`:
- Change `org_id` scans to nullable
- Add `application_id`, `environment_id` to scan helpers and queries

- [x] **Step 8: Run all tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./... 2>&1 | head -50`
Expected: Model tests pass. Repository tests may fail if they require a live DB — that's expected. Compilation should succeed.

- [x] **Step 9: Commit**

```bash
git add internal/platform/database/postgres/
git commit -m "feat: update all repositories for platform redesign hierarchy"
```

---

## Task 10a: Update Domain Interfaces, Services, and Handlers

**Files:**
- Modify: `internal/deploy/repository.go`
- Modify: `internal/deploy/service.go`
- Modify: `internal/deploy/handler.go`
- Modify: `internal/deploy/strategies/` (all strategy files)
- Modify: `internal/releases/repository.go`
- Modify: `internal/releases/service.go`
- Modify: `internal/releases/handler.go`
- Modify: `internal/flags/service.go`
- Modify: `internal/flags/handler.go`

- [x] **Step 1: Update deploy.DeployRepository interface**

In `internal/deploy/repository.go`:
- Remove `ListDeploymentPhases`, `CreateDeploymentPhase`, `UpdateDeploymentPhase`, `GetPipeline` methods
- Change all `projectID uuid.UUID` parameters to `applicationID uuid.UUID`
- Update method signatures to match the new Deployment model

- [x] **Step 2: Update deploy.DeployService**

In `internal/deploy/service.go`:
- Change `ListDeployments(projectID)` to `ListDeployments(applicationID)`
- Change `GetActiveDeployments(projectID)` to `GetActiveDeployments(applicationID)`
- Remove any references to `DeployPipeline` and `DeploymentPhase` types
- Ensure all status references use the typed `DeployStatus` constants

- [x] **Step 3: Update deploy handler**

In `internal/deploy/handler.go`:
- Update routes from `/projects/:project_id/` to `/applications/:app_id/`
- Update handler methods to extract `applicationID` from URL params
- Remove phase/pipeline related handlers

- [x] **Step 4: Update deploy strategies**

In `internal/deploy/strategies/` files:
- Ensure they use `DeployStatus` and `DeployStrategyType` typed constants
- Remove any references to `DeploymentPhase` if present

- [x] **Step 5: Rewrite releases.ReleaseRepository interface**

In `internal/releases/repository.go`:
- Replace all methods with new Release model methods:
  - `Create(ctx, *Release) error`
  - `GetByID(ctx, id) (*Release, error)`
  - `ListByApplication(ctx, appID) ([]Release, error)`
  - `Update(ctx, *Release) error`
  - `Delete(ctx, id) error`
  - `AddFlagChange(ctx, *ReleaseFlagChange) error`
  - `ListFlagChanges(ctx, releaseID) ([]ReleaseFlagChange, error)`
- Remove `ReleaseEnvironment`, `ReleaseTimeline`, `ReleaseStatus` type references

- [x] **Step 6: Rewrite releases.ReleaseService**

In `internal/releases/service.go`:
- Rewrite to match the new Release model (flag-change bundles)
- Add methods: `Start`, `Promote`, `Pause`, `Rollback`, `Complete`
- Remove `PromoteToEnvironment` and old lifecycle methods

- [x] **Step 7: Rewrite releases handler**

In `internal/releases/handler.go`:
- Update routes from `/projects/` to `/applications/:app_id/releases`
- Add handlers for: start, promote, pause, rollback, complete, add-flag
- Remove old promotion/environment handlers

- [x] **Step 8: Update flags service and handler**

In `internal/flags/service.go` and `internal/flags/handler.go`:
- Support `application_id` parameter in flag creation and listing
- Add handlers/methods for flag environment state (GET/PUT per environment)

- [x] **Step 9: Run full build to verify compilation**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: Compiles without errors.

- [x] **Step 10: Commit**

```bash
git add internal/deploy/ internal/releases/ internal/flags/
git commit -m "feat: update domain interfaces, services, and handlers for platform redesign"
```

---

## Task 11: Update API Route Registration

**Files:**
- Modify: `cmd/api/main.go`

This task ensures the API server still compiles after the model changes. Full new endpoints (applications, settings, etc.) will be added in the API plan (Spec B).

- [x] **Step 1: Update main.go references**

- Remove `DeployPipeline` references if any exist
- Update service constructors if they reference removed models
- Ensure `releaseService` constructor matches the new Release model

- [x] **Step 2: Verify the API server compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./cmd/api/`
Expected: Compiles without errors.

- [x] **Step 3: Commit**

```bash
git add cmd/api/main.go
git commit -m "fix: update API server for platform redesign model changes"
```

---

## Task 12: Verify Full Migration on Docker

**Files:** None (verification only)

- [x] **Step 1: Tear down and rebuild the Docker database**

Run:
```bash
cd /Users/sgamel/git/DeploySentry/deploy/selfhost
docker compose down -v
docker compose up -d postgres
docker compose --profile setup run --rm migrate
```
Expected: All 30 migrations (0-29) pass.

- [x] **Step 2: Verify table structure**

Run:
```bash
docker compose exec postgres psql -U deploysentry -d deploysentry -c "SET search_path TO deploy; \dt"
```
Expected: `applications`, `releases`, `release_flag_changes`, `flag_environment_state`, `settings` present. `deploy_pipelines`, `release_environments` absent.

- [x] **Step 3: Verify constraints**

Run:
```bash
docker compose exec postgres psql -U deploysentry -d deploysentry -c "
SET search_path TO deploy;
SELECT conname, contype FROM pg_constraint WHERE conrelid = 'api_keys'::regclass AND conname LIKE 'chk_%';
SELECT conname, contype FROM pg_constraint WHERE conrelid = 'settings'::regclass AND conname LIKE 'chk_%';
"
```
Expected: `chk_api_keys_single_scope` and `chk_settings_single_scope` present.

- [x] **Step 4: Verify the Go API compiles and starts**

Run:
```bash
cd /Users/sgamel/git/DeploySentry && go build ./cmd/api/ && echo "BUILD OK"
```
Expected: `BUILD OK`

- [x] **Step 5: Final commit with all fixes**

If any fixes were needed during verification:
```bash
git add -A
git commit -m "fix: resolve migration and compilation issues from platform redesign"
```

---

## Task 13: Update docs

**Files:**
- Modify: `docs/Current_Initiatives.md`

- [x] **Step 1: Update Current_Initiatives.md**

Add the platform redesign initiative and mark the data model phase as complete.

- [x] **Step 2: Commit**

```bash
git add docs/Current_Initiatives.md
git commit -m "docs: update Current Initiatives with platform redesign progress"
```
