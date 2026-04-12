-- The project-scoped environments table from migration 006 (later migrated to
-- application-scoped in migration 029) is being replaced by an org-scoped
-- design. Drop the old table via CASCADE so any dependent FK constraints on
-- other tables are removed before we recreate environments with the new shape.
DROP TABLE IF EXISTS environments CASCADE;

CREATE TABLE environments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    slug          TEXT NOT NULL,
    is_production BOOLEAN NOT NULL DEFAULT false,
    sort_order    INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, slug)
);

CREATE INDEX idx_environments_org_id ON environments(org_id);

CREATE TABLE app_environment_overrides (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id          UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    config          JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (app_id, environment_id)
);

CREATE INDEX idx_app_env_overrides_app_id ON app_environment_overrides(app_id);
