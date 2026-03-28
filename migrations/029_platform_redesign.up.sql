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
