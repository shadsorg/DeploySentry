-- 060_add_flag_delete_after: 30-day retention window for hard-delete.
--
-- delete_after  : when set, the sweep job will tombstone the flag at/after this time.
-- deleted_at    : tombstone marker. Once set, the flag is treated as gone by the API
--                 surface, but the row remains so audit_log / flag_evaluation_log
--                 rows referencing it stay joinable. A separate compaction job
--                 (out of scope) hard-DELETEs tombstoned rows older than the audit
--                 retention window, which fires the existing CASCADE FKs.
ALTER TABLE feature_flags
    ADD COLUMN IF NOT EXISTS delete_after TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at   TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_feature_flags_delete_after
    ON feature_flags (delete_after)
    WHERE delete_after IS NOT NULL AND deleted_at IS NULL;
