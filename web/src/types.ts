export type FlagCategory = 'release' | 'feature' | 'experiment' | 'ops' | 'permission';
export type FlagType = 'boolean' | 'string' | 'integer' | 'json';
export type DeployStrategy = 'canary' | 'blue-green' | 'rolling';
export type DeployStatus =
  | 'pending'
  | 'running'
  | 'promoting'
  | 'paused'
  | 'completed'
  | 'failed'
  | 'rolled_back'
  | 'cancelled';
export type ReleaseStatus = 'draft' | 'rolling_out' | 'paused' | 'completed' | 'rolled_back';
export type RuleType = 'percentage' | 'user_target' | 'attribute' | 'segment' | 'schedule';

export interface FlagMetadata {
  category: FlagCategory;
  purpose: string;
  owners: string[];
  isPermanent: boolean;
  expiresAt: string | null;
  tags: string[];
}

export interface Flag {
  id: string;
  project_id: string;
  application_id?: string;
  environment_id: string;
  key: string;
  name: string;
  description: string;
  flag_type: FlagType;
  category: FlagCategory;
  purpose: string;
  owners: string[];
  is_permanent: boolean;
  expires_at: string | null;
  enabled: boolean;
  default_value: string;
  archived: boolean;
  tags: string[];
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface TargetingRule {
  id: string;
  flag_id: string;
  rule_type: RuleType;
  priority: number;
  value: string;
  percentage?: number;
  attribute?: string;
  operator?: string;
  target_values?: string[];
  segment_id?: string;
  start_time?: string;
  end_time?: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface Deployment {
  id: string;
  application_id: string;
  environment_id: string;
  version: string;
  commit_sha?: string;
  artifact?: string;
  strategy: DeployStrategy;
  status: DeployStatus;
  traffic_percent: number;
  health_score: number;
  created_by: string;
  created_at: string;
  updated_at: string;
  started_at?: string;
  completed_at: string | null;
}

export interface Release {
  id: string;
  application_id: string;
  name: string;
  description?: string;
  session_sticky: boolean;
  sticky_header?: string;
  traffic_percent: number;
  status: ReleaseStatus;
  created_by: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ApiKey {
  id: string;
  name: string;
  prefix: string;
  scopes: string[];
  environment_targets: string[];
  created_at: string;
  last_used_at: string | null;
  expires_at: string | null;
}

export interface Project {
  id: string;
  name: string;
  slug: string;
  org_id: string;
}

export interface Environment {
  id: string;
  application_id: string;
  name: string;
  slug: string;
  description?: string;
  is_production: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export interface CreateFlagRequest {
  project_id: string;
  environment_id: string;
  application_id?: string;
  key: string;
  name: string;
  description?: string;
  flag_type: FlagType;
  category: FlagCategory;
  purpose?: string;
  owners?: string[];
  is_permanent?: boolean;
  expires_at?: string;
  default_value: string;
  tags?: string[];
}

export interface UpdateFlagRequest {
  name?: string;
  description?: string;
  category?: FlagCategory;
  purpose?: string;
  owners?: string[];
  is_permanent?: boolean;
  expires_at?: string;
  default_value?: string;
}

export interface Organization {
  id: string;
  name: string;
  slug: string;
  created_at: string;
  updated_at: string;
}

export interface Application {
  id: string;
  project_id: string;
  name: string;
  slug: string;
  description?: string;
  repo_url?: string;
  created_at: string;
  updated_at: string;
}

export interface FlagEnvState {
  flag_id: string;
  environment_id: string;
  environment_name: string;
  enabled: boolean;
  value: string;
  updated_by: string;
  updated_at: string;
}

export interface FlagEnvironmentState {
  id: string;
  flag_id: string;
  environment_id: string;
  enabled: boolean;
  value?: unknown;
  updated_by?: string;
  updated_at: string;
}

export interface Setting {
  id: string;
  org_id?: string;
  project_id?: string;
  application_id?: string;
  environment_id?: string;
  key: string;
  value: unknown;
  updated_by?: string;
  updated_at: string;
}

export interface DeploymentEvent {
  status: DeployStatus;
  timestamp: string;
  note: string;
}

export interface ReleaseFlagChange {
  id: string;
  release_id: string;
  flag_key: string;
  environment_name: string;
  previous_enabled: boolean;
  new_enabled: boolean;
  previous_value: string;
  new_value: string;
  applied_at: string | null;
}

export interface ReleaseFlagChangeAPI {
  id: string;
  release_id: string;
  flag_id: string;
  environment_id: string;
  previous_value?: unknown;
  new_value?: unknown;
  previous_enabled?: boolean;
  new_enabled?: boolean;
  applied_at: string | null;
  created_at: string;
}

export type GroupRole = 'viewer' | 'editor' | 'admin';

export interface Member {
  id: string;
  user_id: string;
  name: string;
  email: string;
  avatar_url?: string;
  role: 'owner' | 'admin' | 'member' | 'viewer';
  joined_at: string;
}

export interface Group {
  id: string;
  name: string;
  role: GroupRole;
  environment_ids: string[];
  application_ids: string[];
  member_ids: string[];
  created_at: string;
}

export interface OrgEnvironment {
  id: string;
  org_id: string;
  name: string;
  slug: string;
  is_production: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export type PhaseStatus = 'pending' | 'active' | 'passed' | 'failed' | 'skipped';

export interface DeploymentPhase {
  id: string;
  deployment_id: string;
  name: string;
  status: PhaseStatus;
  traffic_percent: number;
  duration_seconds: number;
  sort_order: number;
  auto_promote: boolean;
  started_at?: string;
  completed_at?: string;
}
