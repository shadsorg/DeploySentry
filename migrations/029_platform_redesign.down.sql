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
