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
