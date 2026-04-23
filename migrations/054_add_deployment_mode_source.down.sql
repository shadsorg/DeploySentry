DROP INDEX IF EXISTS deployments_app_env_created_idx;

ALTER TABLE deployments
    DROP CONSTRAINT IF EXISTS deployments_mode_check;

ALTER TABLE deployments
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS mode;
