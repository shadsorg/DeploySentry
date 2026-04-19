-- 052_add_flag_lifecycle: adds the lifecycle layer used by the CrowdSoft
-- feature-agent to drive validated rollouts end-to-end (smoke-test status,
-- user-test status, iteration tracking, scheduled removal). All columns are
-- optional — a flag with no lifecycle data behaves exactly as before.

ALTER TABLE feature_flags
    ADD COLUMN IF NOT EXISTS smoke_test_status         TEXT,
    ADD COLUMN IF NOT EXISTS user_test_status          TEXT,
    ADD COLUMN IF NOT EXISTS scheduled_removal_at      TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS scheduled_removal_fired_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS iteration_count           INTEGER      NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS iteration_exhausted       BOOLEAN      NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS last_smoke_test_notes     TEXT,
    ADD COLUMN IF NOT EXISTS last_user_test_notes      TEXT;

-- Constrain the enumerated status fields. Named separately so the constraint
-- can be dropped cleanly in the down migration.
ALTER TABLE feature_flags
    ADD CONSTRAINT feature_flags_smoke_test_status_check
        CHECK (smoke_test_status IS NULL OR smoke_test_status IN ('pending', 'pass', 'fail'));

ALTER TABLE feature_flags
    ADD CONSTRAINT feature_flags_user_test_status_check
        CHECK (user_test_status IS NULL OR user_test_status IN ('pending', 'pass', 'fail'));

-- Partial index powering the scheduler: only flags that are queued for removal
-- and haven't had their 'due' webhook fired yet.
CREATE INDEX IF NOT EXISTS idx_feature_flags_pending_removal
    ON feature_flags (scheduled_removal_at)
    WHERE scheduled_removal_at IS NOT NULL AND scheduled_removal_fired_at IS NULL;
