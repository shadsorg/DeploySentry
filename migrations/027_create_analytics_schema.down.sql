-- Remove analytics schema

-- Drop indexes first
DROP INDEX IF EXISTS idx_daily_flag_stats_flag_date;
DROP INDEX IF EXISTS idx_daily_flag_stats_project_date;
DROP INDEX IF EXISTS idx_api_metrics_user_time;
DROP INDEX IF EXISTS idx_api_metrics_status_time;
DROP INDEX IF EXISTS idx_api_metrics_path_time;
DROP INDEX IF EXISTS idx_deployment_events_type_time;
DROP INDEX IF EXISTS idx_deployment_events_deployment_time;
DROP INDEX IF EXISTS idx_flag_eval_events_user;
DROP INDEX IF EXISTS idx_flag_eval_events_flag_time;
DROP INDEX IF EXISTS idx_flag_eval_events_project_time;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS daily_flag_stats;
DROP TABLE IF EXISTS api_request_metrics;
DROP TABLE IF EXISTS deployment_events;
DROP TABLE IF EXISTS flag_evaluation_events;