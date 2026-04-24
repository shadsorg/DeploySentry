-- migrations/058_drop_bogus_deployments_project_id_fkey.up.sql
--
-- Migration 029 renamed deployments.project_id → application_id, but the
-- pre-existing FK constraint (named deployments_project_id_fkey) survived
-- the rename. After the rename the constraint reads as
--   FOREIGN KEY (application_id) REFERENCES projects(id)
-- which fails every INSERT because application UUIDs are not project
-- UUIDs. The correct FK — fk_deployments_application referencing
-- applications(id) — was added in the same migration and remains in
-- place, so dropping the stale constraint is the whole fix.
ALTER TABLE deployments
    DROP CONSTRAINT IF EXISTS deployments_project_id_fkey;
