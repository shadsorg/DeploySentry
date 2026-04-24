-- migrations/058_drop_bogus_deployments_project_id_fkey.down.sql
--
-- Restore the (broken) constraint for symmetry. Note that with any real
-- deployment rows present this ALTER will fail — that is intentional,
-- since the original constraint was itself broken. See up migration.
ALTER TABLE deployments
    ADD CONSTRAINT deployments_project_id_fkey
        FOREIGN KEY (application_id) REFERENCES projects(id);
