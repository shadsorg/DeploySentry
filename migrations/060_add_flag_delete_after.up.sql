-- 060_add_flag_delete_after: 30-day retention window for hard-delete.
--
-- delete_after  : when set (operator opt-in), the retention sweep DELETEs the
--                 flag at/after this time. ON DELETE CASCADE FKs on
--                 flag_targeting_rules, flag_ratings, flag_evaluation_metrics,
--                 and release_flag_changes clean up dependent rows. The
--                 audit_log.resource_id column has no FK so prior audit
--                 history survives the row deletion.
ALTER TABLE feature_flags
    ADD COLUMN IF NOT EXISTS delete_after TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_feature_flags_delete_after
    ON feature_flags (delete_after)
    WHERE delete_after IS NOT NULL;
