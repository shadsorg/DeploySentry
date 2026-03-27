ALTER TABLE release_environments DROP COLUMN IF EXISTS updated_at;
ALTER TABLE release_environments DROP COLUMN IF EXISTS created_at;
ALTER TABLE release_environments DROP COLUMN IF EXISTS deployed_by;
ALTER TABLE release_environments DROP COLUMN IF EXISTS lifecycle_status;
ALTER TABLE release_environments DROP COLUMN IF EXISTS deployment_id;
ALTER TABLE release_environments DROP COLUMN IF EXISTS environment_id;

ALTER TABLE releases DROP COLUMN IF EXISTS updated_at;
ALTER TABLE releases DROP COLUMN IF EXISTS released_at;
ALTER TABLE releases DROP COLUMN IF EXISTS lifecycle_status;
ALTER TABLE releases DROP COLUMN IF EXISTS artifact;
ALTER TABLE releases DROP COLUMN IF EXISTS description;
ALTER TABLE releases DROP COLUMN IF EXISTS title;

ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS updated_at;
ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS end_time;
ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS start_time;
ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS segment_id;
ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS target_values;
ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS operator;
ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS attribute;
ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS percentage;
ALTER TABLE flag_targeting_rules DROP COLUMN IF EXISTS value;

ALTER TABLE deployment_phases DROP COLUMN IF EXISTS sort_order;
ALTER TABLE deployment_phases DROP COLUMN IF EXISTS name;

ALTER TABLE deployments DROP COLUMN IF EXISTS updated_at;
ALTER TABLE deployments DROP COLUMN IF EXISTS traffic_percent;
ALTER TABLE deployments DROP COLUMN IF EXISTS commit_sha;
ALTER TABLE deployments DROP COLUMN IF EXISTS artifact;
ALTER TABLE deployments DROP COLUMN IF EXISTS version;
ALTER TABLE deployments DROP COLUMN IF EXISTS strategy;
ALTER TABLE deployments DROP COLUMN IF EXISTS project_id;

ALTER TABLE project_members DROP COLUMN IF EXISTS updated_at;
ALTER TABLE project_members DROP COLUMN IF EXISTS created_at;
ALTER TABLE project_members DROP COLUMN IF EXISTS id;

ALTER TABLE org_members DROP COLUMN IF EXISTS updated_at;
ALTER TABLE org_members DROP COLUMN IF EXISTS created_at;
ALTER TABLE org_members DROP COLUMN IF EXISTS invited_by;
ALTER TABLE org_members DROP COLUMN IF EXISTS id;

ALTER TABLE api_keys DROP COLUMN IF EXISTS revoked_at;
ALTER TABLE api_keys ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE api_keys DROP COLUMN IF EXISTS org_id;

ALTER TABLE feature_flags DROP COLUMN IF EXISTS environment_id;

ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
ALTER TABLE users DROP COLUMN IF EXISTS updated_at;
