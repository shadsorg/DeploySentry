-- 1. Drop the GIN index
DROP INDEX IF EXISTS idx_api_keys_environment_ids;

-- 2. Add back the old columns
ALTER TABLE api_keys ADD COLUMN application_id UUID REFERENCES applications(id) ON DELETE CASCADE;
ALTER TABLE api_keys ADD COLUMN environment_id UUID REFERENCES environments(id) ON DELETE CASCADE;

-- 3. Migrate first element back to singular column
UPDATE api_keys SET environment_id = environment_ids[1]
WHERE array_length(environment_ids, 1) > 0;

-- 4. Drop the array column
ALTER TABLE api_keys DROP COLUMN environment_ids;

-- 5. Make org_id nullable again
ALTER TABLE api_keys ALTER COLUMN org_id DROP NOT NULL;

-- 6. Restore the CHECK constraint
ALTER TABLE api_keys ADD CONSTRAINT chk_api_keys_single_scope
    CHECK (num_nonnulls(org_id, project_id, application_id, environment_id) = 1);

-- 7. Null out org_id where project_id is set (restore old behavior)
UPDATE api_keys SET org_id = NULL WHERE project_id IS NOT NULL;
