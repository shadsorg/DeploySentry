-- 1. Drop the exclusive-scope CHECK constraint
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS chk_api_keys_single_scope;

-- 2. Backfill org_id from project where missing
UPDATE api_keys SET org_id = (
    SELECT p.org_id FROM projects p WHERE p.id = api_keys.project_id
) WHERE org_id IS NULL AND project_id IS NOT NULL;

-- 3. Make org_id NOT NULL
ALTER TABLE api_keys ALTER COLUMN org_id SET NOT NULL;

-- 4. Add environment_ids array column
ALTER TABLE api_keys ADD COLUMN environment_ids UUID[] NOT NULL DEFAULT '{}';

-- 5. Migrate existing singular environment_id data
UPDATE api_keys SET environment_ids = ARRAY[environment_id]
WHERE environment_id IS NOT NULL;

-- 6. Drop old columns
ALTER TABLE api_keys DROP COLUMN IF EXISTS environment_id;
ALTER TABLE api_keys DROP COLUMN IF EXISTS application_id;

-- 7. Index for array containment queries
CREATE INDEX idx_api_keys_environment_ids ON api_keys USING GIN (environment_ids);
