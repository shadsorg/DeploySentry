DROP INDEX IF EXISTS idx_feature_flags_pending_removal;

ALTER TABLE feature_flags
    DROP CONSTRAINT IF EXISTS feature_flags_smoke_test_status_check,
    DROP CONSTRAINT IF EXISTS feature_flags_user_test_status_check;

ALTER TABLE feature_flags
    DROP COLUMN IF EXISTS smoke_test_status,
    DROP COLUMN IF EXISTS user_test_status,
    DROP COLUMN IF EXISTS scheduled_removal_at,
    DROP COLUMN IF EXISTS scheduled_removal_fired_at,
    DROP COLUMN IF EXISTS iteration_count,
    DROP COLUMN IF EXISTS iteration_exhausted,
    DROP COLUMN IF EXISTS last_smoke_test_notes,
    DROP COLUMN IF EXISTS last_user_test_notes;
