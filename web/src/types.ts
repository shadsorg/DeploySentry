export type FlagCategory = 'release' | 'feature' | 'experiment' | 'ops' | 'permission';
export type FlagType = 'boolean' | 'string' | 'integer' | 'json';
export type DeployStrategy = 'canary' | 'blue-green' | 'rolling';
export type DeployStatus = 'pending' | 'running' | 'paused' | 'completed' | 'failed' | 'rolled_back';
export type ReleaseStatus = 'draft' | 'staging' | 'canary' | 'production' | 'archived';
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
  project_id: string;
  environment_id: string;
  version: string;
  strategy: DeployStrategy;
  status: DeployStatus;
  traffic_percentage: number;
  health_score: number;
  created_by: string;
  created_at: string;
  updated_at: string;
  completed_at: string | null;
}

export interface Release {
  id: string;
  project_id: string;
  version: string;
  status: ReleaseStatus;
  commit_sha: string;
  description: string;
  created_by: string;
  created_at: string;
  updated_at: string;
  promoted_at: string | null;
}

export interface ApiKey {
  id: string;
  name: string;
  prefix: string;
  scopes: string[];
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
  name: string;
  project_id: string;
}

export interface CreateFlagRequest {
  project_id: string;
  environment_id: string;
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
