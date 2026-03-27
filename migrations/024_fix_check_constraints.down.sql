-- Revert to original CHECK constraints

ALTER TABLE deployment_phases DROP CONSTRAINT IF EXISTS deployment_phases_status_check;
ALTER TABLE deployment_phases ADD CONSTRAINT deployment_phases_status_check
    CHECK (status IN ('pending', 'active', 'passed', 'failed', 'skipped'));

ALTER TABLE deployments DROP CONSTRAINT IF EXISTS deployments_status_check;
ALTER TABLE deployments ADD CONSTRAINT deployments_status_check
    CHECK (status IN ('pending', 'in_progress', 'paused', 'promoting',
                      'completed', 'rolling_back', 'failed'));

ALTER TABLE feature_flags DROP CONSTRAINT IF EXISTS feature_flags_flag_type_check;
ALTER TABLE feature_flags ADD CONSTRAINT feature_flags_flag_type_check
    CHECK (flag_type IN ('boolean', 'string', 'number', 'json'));
