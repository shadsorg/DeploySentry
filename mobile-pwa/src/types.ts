export interface AuthUser {
  id: string;
  email: string;
  name: string;
  avatar_url?: string;
}

export interface Organization {
  id: string;
  name: string;
  slug: string;
  created_at: string;
  updated_at: string;
}

export interface MonitoringLink {
  label: string;
  url: string;
  icon?: string;
}

export type HealthState = 'healthy' | 'degraded' | 'unhealthy' | 'unknown';
export type HealthStaleness = 'fresh' | 'stale' | 'missing';

export interface OrgStatusHealthBlock {
  state: HealthState;
  score?: number | null;
  reason?: string;
  source: string;
  last_reported_at?: string | null;
  staleness: HealthStaleness;
}

export interface OrgStatusDeploymentMini {
  id: string;
  version: string;
  commit_sha?: string;
  status: string;
  mode: string;
  source?: string | null;
  completed_at?: string | null;
}

export interface OrgStatusEnvCell {
  environment: { id: string; slug?: string; name?: string };
  current_deployment?: OrgStatusDeploymentMini | null;
  health: OrgStatusHealthBlock;
  never_deployed: boolean;
}

export interface OrgStatusApplicationNode {
  application: {
    id: string;
    slug: string;
    name: string;
    monitoring_links?: MonitoringLink[] | null;
  };
  environments: OrgStatusEnvCell[];
}

export interface OrgStatusProjectNode {
  project: { id: string; slug: string; name: string };
  aggregate_health: HealthState;
  applications: OrgStatusApplicationNode[];
}

export interface OrgStatusResponse {
  org: { id: string; slug: string; name: string };
  generated_at: string;
  projects: OrgStatusProjectNode[];
}

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

export interface Deployment {
  id: string;
  application_id: string;
  environment_id: string;
  version: string;
  commit_sha?: string;
  artifact?: string;
  strategy: DeployStrategy;
  status: DeployStatus;
  mode?: 'orchestrate' | 'record';
  source?: string | null;
  traffic_percent: number;
  health_score?: number;
  created_by: string;
  created_at: string;
  updated_at: string;
  started_at?: string;
  completed_at: string | null;
}

export interface OrgDeploymentRow extends Deployment {
  application: { id: string; slug: string; name: string };
  environment: { id: string; slug?: string; name?: string };
  project: { id: string; slug: string; name: string };
}

export interface OrgDeploymentsResponse {
  deployments: OrgDeploymentRow[];
  next_cursor?: string;
}

export interface OrgDeploymentsFilters {
  project_id?: string;
  application_id?: string;
  environment_id?: string;
  status?: string;
  mode?: string;
  from?: string;
  to?: string;
  cursor?: string;
  limit?: number;
}

export interface Project {
  id: string;
  name: string;
  slug: string;
  org_id: string;
  description?: string;
}

export interface Application {
  id: string;
  slug: string;
  name: string;
  project_id: string;
}

export interface OrgEnvironment {
  id: string;
  slug: string;
  name: string;
  is_production?: boolean;
  sort_order?: number;
}

export type FlagCategory = 'release' | 'feature' | 'experiment' | 'ops' | 'permission';
export type FlagType = 'boolean' | 'string' | 'number' | 'json';

export interface Flag {
  id: string;
  project_id: string;
  application_id?: string | null;
  environment_id?: string;
  key: string;
  name: string;
  description?: string;
  flag_type: FlagType;
  category: FlagCategory;
  purpose?: string;
  owners?: string[];
  tags?: string[];
  is_permanent: boolean;
  expires_at?: string | null;
  default_value: string;
  enabled: boolean;
  archived: boolean;
  created_by?: string;
  created_by_name?: string;
  created_at: string;
  updated_at: string;
}

export type RuleType = 'percentage' | 'user_target' | 'attribute' | 'segment' | 'schedule' | 'compound';

export interface TargetingRule {
  id: string;
  flag_id: string;
  rule_type?: RuleType;
  attribute?: string;
  operator?: string;
  target_values?: string[];
  value: string;
  priority: number;
  percentage?: number | null;
  user_ids?: string[] | null;
  segment_id?: string | null;
  start_time?: string | null;
  end_time?: string | null;
  created_at: string;
  updated_at: string;
}

export interface FlagEnvironmentState {
  id?: string;
  flag_id: string;
  environment_id: string;
  enabled: boolean;
  value?: unknown;
  updated_by?: string;
  updated_at?: string;
}

export interface RuleEnvironmentState {
  id?: string;
  rule_id: string;
  environment_id: string;
  enabled: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface AuditLogEntry {
  id: string;
  resource_type: string;
  resource_id: string;
  action: string;
  actor_id?: string;
  actor_name?: string;
  old_value?: string | null;
  new_value?: string | null;
  metadata?: Record<string, unknown>;
  created_at: string;
}
