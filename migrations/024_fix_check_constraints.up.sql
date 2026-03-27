-- Fix CHECK constraints to match Go model constants

-- feature_flags.flag_type: Go uses 'integer', migration 012 had 'number'
ALTER TABLE feature_flags DROP CONSTRAINT IF EXISTS feature_flags_flag_type_check;
ALTER TABLE feature_flags ADD CONSTRAINT feature_flags_flag_type_check
    CHECK (flag_type IN ('boolean', 'string', 'integer', 'number', 'json'));

-- deployments.status: Go uses 'running', 'rolled_back', 'cancelled'
-- migration 010 had 'in_progress', 'rolling_back' and no 'cancelled'
ALTER TABLE deployments DROP CONSTRAINT IF EXISTS deployments_status_check;
ALTER TABLE deployments ADD CONSTRAINT deployments_status_check
    CHECK (status IN ('pending', 'running', 'in_progress', 'paused', 'promoting',
                      'completed', 'failed', 'rolled_back', 'rolling_back', 'cancelled'));

-- deployment_phases.status: align with deployment status values
ALTER TABLE deployment_phases DROP CONSTRAINT IF EXISTS deployment_phases_status_check;
ALTER TABLE deployment_phases ADD CONSTRAINT deployment_phases_status_check
    CHECK (status IN ('pending', 'active', 'passed', 'failed', 'skipped',
                      'running', 'in_progress', 'completed', 'cancelled'));
