-- users: add missing columns
ALTER TABLE users ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT false;

-- feature_flags: add environment_id for multi-environment flag evaluation
ALTER TABLE feature_flags ADD COLUMN IF NOT EXISTS environment_id UUID REFERENCES environments(id);
CREATE INDEX IF NOT EXISTS idx_feature_flags_env ON feature_flags (project_id, environment_id, key);

-- api_keys: add org_id, make project_id nullable, add revoked_at
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES organizations(id);
ALTER TABLE api_keys ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;

-- org_members: add id, invited_by, created_at, updated_at
ALTER TABLE org_members ADD COLUMN IF NOT EXISTS id UUID DEFAULT gen_random_uuid();
ALTER TABLE org_members ADD COLUMN IF NOT EXISTS invited_by UUID REFERENCES users(id);
ALTER TABLE org_members ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE org_members ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- project_members: add id, created_at, updated_at
ALTER TABLE project_members ADD COLUMN IF NOT EXISTS id UUID DEFAULT gen_random_uuid();
ALTER TABLE project_members ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE project_members ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- deployments: add columns the model expects
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS project_id UUID REFERENCES projects(id);
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS strategy TEXT;
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS version TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS artifact TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS commit_sha TEXT;
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS traffic_percent INT NOT NULL DEFAULT 0;
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE deployments ALTER COLUMN pipeline_id DROP NOT NULL;
ALTER TABLE deployments ALTER COLUMN release_id DROP NOT NULL;

-- deployment_phases: add deployment model fields
ALTER TABLE deployment_phases ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT '';
ALTER TABLE deployment_phases ADD COLUMN IF NOT EXISTS sort_order INT NOT NULL DEFAULT 0;

-- flag_targeting_rules: add individual columns that model uses
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS value TEXT NOT NULL DEFAULT '';
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS percentage INT;
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS attribute TEXT;
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS operator TEXT;
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS target_values TEXT[];
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS segment_id UUID;
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS start_time TIMESTAMPTZ;
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS end_time TIMESTAMPTZ;
ALTER TABLE flag_targeting_rules ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- releases: add missing model fields
ALTER TABLE releases ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '';
ALTER TABLE releases ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE releases ADD COLUMN IF NOT EXISTS artifact TEXT NOT NULL DEFAULT '';
ALTER TABLE releases ADD COLUMN IF NOT EXISTS lifecycle_status TEXT;
ALTER TABLE releases ADD COLUMN IF NOT EXISTS released_at TIMESTAMPTZ;
ALTER TABLE releases ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- release_environments: add missing model fields
ALTER TABLE release_environments ADD COLUMN IF NOT EXISTS environment_id UUID;
ALTER TABLE release_environments ADD COLUMN IF NOT EXISTS deployment_id UUID;
ALTER TABLE release_environments ADD COLUMN IF NOT EXISTS lifecycle_status TEXT;
ALTER TABLE release_environments ADD COLUMN IF NOT EXISTS deployed_by UUID;
ALTER TABLE release_environments ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE release_environments ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
