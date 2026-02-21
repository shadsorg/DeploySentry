-- Add a partition-friendly composite index on the flag_evaluation_log table.
-- This index is designed to support efficient queries on partitioned tables
-- where evaluated_at is the partition key and flag_id is a common filter.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_flag_eval_log_partition_evaluated_at_flag
    ON flag_evaluation_log (evaluated_at, flag_id);

-- Add an index for lookups by flag_key and environment, which are common
-- access patterns for analytics dashboards.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_flag_eval_log_partition_key_env
    ON flag_evaluation_log (flag_key, environment, evaluated_at DESC);
