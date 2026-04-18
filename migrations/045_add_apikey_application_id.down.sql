DROP INDEX IF EXISTS idx_api_keys_application_id;
ALTER TABLE api_keys DROP COLUMN IF EXISTS application_id;
