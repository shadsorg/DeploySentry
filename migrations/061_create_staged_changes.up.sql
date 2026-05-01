-- 061_create_staged_changes: per-user staging layer for dashboard mutations.
--
-- Every UI mutation on a "staged-eligible" resource writes here instead of
-- the production table. The dashboard reads "production + my staged rows"
-- so the user sees their pending edits applied. A Deploy action commits
-- selected rows in one transaction; Discard removes them.
--
-- Spec: docs/superpowers/specs/2026-04-30-staged-changes-and-deploy-workflow-design.md
-- Plan: docs/superpowers/plans/2026-05-01-staged-changes-and-deploy-workflow.md

CREATE TABLE IF NOT EXISTS staged_changes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    resource_type   TEXT NOT NULL,
    resource_id     UUID,
    provisional_id  UUID,
    action          TEXT NOT NULL,
    field_path      TEXT,
    old_value       JSONB,
    new_value       JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Latest-edit-wins per (user, resource, field). NULL semantics:
--   resource_id NULL on a staged CREATE → fall back to provisional_id so each
--     staged create is its own row (each has a fresh provisional UUID).
--   field_path NULL on a whole-row action (toggle / delete) → coalesce to ''
--     so an upsert collapses repeat toggles into one row.
CREATE UNIQUE INDEX IF NOT EXISTS uq_staged_changes_resource_field
    ON staged_changes (
        user_id,
        org_id,
        resource_type,
        COALESCE(resource_id, provisional_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(field_path, '')
    );

CREATE INDEX IF NOT EXISTS idx_staged_changes_user_org
    ON staged_changes (user_id, org_id);

CREATE INDEX IF NOT EXISTS idx_staged_changes_resource
    ON staged_changes (resource_type, resource_id)
    WHERE resource_id IS NOT NULL;

-- Sweeper-friendly index for the 30-day cleanup.
CREATE INDEX IF NOT EXISTS idx_staged_changes_created_at
    ON staged_changes (created_at);
