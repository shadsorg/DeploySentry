DROP INDEX IF EXISTS idx_feature_flags_expires_at;
DROP INDEX IF EXISTS idx_feature_flags_category;

ALTER TABLE feature_flags
    DROP COLUMN IF EXISTS expires_at,
    DROP COLUMN IF EXISTS is_permanent,
    DROP COLUMN IF EXISTS owners,
    DROP COLUMN IF EXISTS purpose,
    DROP COLUMN IF EXISTS category;
