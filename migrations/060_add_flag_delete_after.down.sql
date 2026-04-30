DROP INDEX IF EXISTS idx_feature_flags_delete_after;
ALTER TABLE feature_flags
    DROP COLUMN IF EXISTS delete_after,
    DROP COLUMN IF EXISTS deleted_at;
