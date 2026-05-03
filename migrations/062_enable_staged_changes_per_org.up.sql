-- 062_enable_staged_changes_per_org: bulk-enable the staged-changes-enabled
-- org-level setting for every existing org so all operators get staging
-- without manually flipping a toggle.
--
-- The unique index idx_settings_org on (org_id, key) WHERE org_id IS NOT NULL
-- makes ON CONFLICT idempotent — re-running the migration is safe.

INSERT INTO settings (org_id, key, value)
SELECT id, 'staged-changes-enabled', 'true'::jsonb
FROM organizations
ON CONFLICT DO NOTHING;
