-- Revert index names to original non-prefixed versions

-- Audit log indexes
ALTER INDEX IF EXISTS idx_deploy_audit_log_created_at
    RENAME TO idx_audit_log_created_at;
ALTER INDEX IF EXISTS idx_deploy_audit_log_org_created_at
    RENAME TO idx_audit_log_org_created_at;

-- Feature flags indexes
ALTER INDEX IF EXISTS idx_deploy_feature_flags_category
    RENAME TO idx_feature_flags_category;
ALTER INDEX IF EXISTS idx_deploy_feature_flags_expires_at
    RENAME TO idx_feature_flags_expires_at;
ALTER INDEX IF EXISTS idx_deploy_feature_flags_env
    RENAME TO idx_feature_flags_env;

-- Flag evaluation log indexes
ALTER INDEX IF EXISTS idx_deploy_flag_evaluation_log_evaluated_at
    RENAME TO idx_flag_evaluation_log_evaluated_at;
ALTER INDEX IF EXISTS idx_deploy_flag_eval_log_partition_evaluated_at_flag
    RENAME TO idx_flag_eval_log_partition_evaluated_at_flag;
ALTER INDEX IF EXISTS idx_deploy_flag_eval_log_partition_key_env
    RENAME TO idx_flag_eval_log_partition_key_env;