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

export type LifecycleTestStatus = 'pending' | 'pass' | 'fail';

export interface Flag {
  id: string;
  project_id: string;
  application_id?: string;
  environment_id?: string;
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
  archived_at?: string | null;
  delete_after?: string | null;
  deleted_at?: string | null;
  tags: string[];
  created_by: string;
  created_by_name?: string;
  created_at: string;
  updated_at: string;
  // Lifecycle layer (CrowdSoft feature-agent). All optional.
  smoke_test_status?: LifecycleTestStatus | null;
  user_test_status?: LifecycleTestStatus | null;
  scheduled_removal_at?: string | null;
  iteration_count?: number;
  iteration_exhausted?: boolean;
  last_smoke_test_notes?: string | null;
  last_user_test_notes?: string | null;
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

export interface RuleEnvironmentState {
  id: string;
  rule_id: string;
  environment_id: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export type RuleOperator =
  | 'equals'
  | 'not_equals'
  | 'in'
  | 'not_in'
  | 'contains'
  | 'starts_with'
  | 'ends_with'
  | 'greater_than'
  | 'less_than';

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
  // Optional: record-mode deploys (e.g. github-actions build lanes) bypass
  // the phase engine and never compute a health score. Treat missing as
  // "unknown" at render time.
  health_score?: number;
  created_by: string;
  created_at: string;
  updated_at: string;
  started_at?: string;
  completed_at: string | null;
  flag_test_key?: string;
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
  environment_ids: string[];
  project_id?: string;
  application_id?: string;
  created_at: string;
  last_used_at: string | null;
  expires_at: string | null;
}

export interface Project {
  id: string;
  name: string;
  slug: string;
  org_id: string;
  description?: string;
  repo_url?: string;
  deleted_at?: string;
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
  environment_id?: string;
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

export interface MonitoringLink {
  label: string;
  url: string;
  icon?: string;
}

export interface Application {
  id: string;
  project_id: string;
  name: string;
  slug: string;
  description?: string;
  repo_url?: string;
  monitoring_links?: MonitoringLink[];
  created_at: string;
  updated_at: string;
  deleted_at?: string;
}

// ---------------------------------------------------------------------------
// Org-level status + deploy history (see internal/models/org_status.go)
// ---------------------------------------------------------------------------

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

export interface OrgDeploymentRow extends Deployment {
  application: { id: string; slug: string; name: string };
  environment: { id: string; slug?: string; name?: string };
  project: { id: string; slug: string; name: string };
}

export interface OrgDeploymentsResponse {
  deployments: OrgDeploymentRow[];
  next_cursor?: string;
}

export interface FlagActivitySummary {
  key: string;
  name: string;
  last_evaluated: string;
}

export interface DeleteResult {
  deleted?: 'permanent' | 'soft';
  eligible_for_hard_delete?: string;
  active_flags?: FlagActivitySummary[];
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

export type PhaseStatus =
  | 'pending'
  | 'active'
  | 'passed'
  | 'failed'
  | 'skipped'
  | 'awaiting_approval'
  | 'rolled_back';

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

export interface AuditLogEntry {
  id: string;
  org_id: string;
  project_id: string;
  actor_id: string;
  actor_name: string;
  action: string;
  entity_type: string;
  entity_id: string;
  old_value: string;
  new_value: string;
  ip_address?: string;
  user_agent?: string;
  created_at: string;
  revertible: boolean;
}

export type AgentStatus = 'connected' | 'stale' | 'disconnected';

export interface Agent {
  id: string;
  app_id: string;
  environment_id: string;
  status: AgentStatus;
  version: string;
  upstream_config: Record<string, string>;
  last_seen_at: string;
  registered_at: string;
}

export interface UpstreamMetrics {
  rps: number;
  error_rate: number;
  p99_ms: number;
  p50_ms: number;
}

export interface ActiveRules {
  weights: Record<string, number>;
  header_overrides?: { header: string; value: string; upstream: string }[];
  sticky_sessions?: { enabled: boolean; strategy?: string; ttl?: string };
}

export interface HeartbeatPayload {
  agent_id: string;
  deployment_id?: string;
  config_version: number;
  actual_traffic: Record<string, number>;
  upstreams: Record<string, UpstreamMetrics>;
  active_rules: ActiveRules;
  envoy_healthy: boolean;
}

export interface AgentHeartbeat {
  id: string;
  agent_id: string;
  deployment_id?: string;
  payload: HeartbeatPayload;
  created_at: string;
}

// ---- Strategies + policies + defaults (Plan 1) ----

export type TargetType = 'deploy' | 'config' | 'any';
export type ScopeType = 'org' | 'project' | 'app';
export type PolicyKind = 'off' | 'prompt' | 'mandate';

export interface Step {
  percent: number;
  min_duration: number; // nanoseconds
  max_duration: number;
  bake_time_healthy: number;
  health_threshold?: number;
  approval?: { required_role: string; timeout: number };
  notify?: { on_entry?: string[]; on_exit?: string[] };
  abort_conditions?: Array<{
    metric: string;
    operator: string;
    threshold: number;
    window: number;
  }>;
  signal_override?: { kind: string };
}

export interface Strategy {
  id: string;
  scope_type: ScopeType;
  scope_id: string;
  name: string;
  description: string;
  target_type: TargetType;
  steps: Step[];
  default_health_threshold: number;
  default_rollback_on_failure: boolean;
  version: number;
  is_system: boolean;
  created_by?: string;
  updated_by?: string;
  created_at: string;
  updated_at: string;
}

export interface EffectiveStrategy {
  strategy: Strategy;
  origin_scope: { type: ScopeType; id: string };
  is_inherited: boolean;
}

export interface RolloutPolicy {
  id: string;
  scope_type: ScopeType;
  scope_id: string;
  environment?: string;
  target_type?: TargetType;
  enabled: boolean;
  policy: PolicyKind;
  created_at: string;
  updated_at: string;
}

export interface StrategyDefault {
  id: string;
  scope_type: ScopeType;
  scope_id: string;
  environment?: string;
  target_type?: TargetType;
  strategy_id: string;
  created_at: string;
  updated_at: string;
}

// ---- Rollouts (Plan 2+3) ----

export type RolloutStatus =
  | 'pending'
  | 'active'
  | 'paused'
  | 'awaiting_approval'
  | 'succeeded'
  | 'rolled_back'
  | 'aborted'
  | 'superseded';

export interface RolloutTargetRef {
  deployment_id?: string;
  flag_key?: string;
  env?: string;
  rule_id?: string;
  previous_percentage?: number;
}

export interface Rollout {
  id: string;
  release_id?: string;
  target_type: 'deploy' | 'config';
  target_ref: RolloutTargetRef;
  strategy_snapshot: Strategy;
  signal_source: { kind: string };
  status: RolloutStatus;
  current_phase_index: number;
  current_phase_started_at?: string;
  last_healthy_since?: string;
  rollback_reason?: string;
  created_by?: string;
  created_at: string;
  completed_at?: string;
}

export interface RolloutEvent {
  id: string;
  rollout_id: string;
  event_type: string;
  actor_type: 'user' | 'system';
  actor_id?: string;
  reason?: string;
  payload: Record<string, unknown>;
  occurred_at: string;
}

/**
 * Enriched rollout list row. Mirrors models.RolloutWithTarget server-side.
 * Every target field is best-effort — deleted rows leave them empty and
 * the UI renders a "(unknown)" placeholder.
 */
export interface RolloutTargetDisplay {
  kind: 'deploy' | 'config';
  summary: string;
  application_slug?: string;
  application_name?: string;
  project_slug?: string;
  environment_slug?: string;
  version?: string;
  flag_key?: string;
}

export interface RolloutWithTarget extends Rollout {
  target: RolloutTargetDisplay;
  age_seconds: number;
}

// ---- Rollout groups (Plan 4) ----

export type CoordinationPolicy = 'independent' | 'pause_on_sibling_abort' | 'cascade_abort';

export interface RolloutGroup {
  id: string;
  scope_type: ScopeType;
  scope_id: string;
  name: string;
  description: string;
  coordination_policy: CoordinationPolicy;
  created_by?: string;
  created_at: string;
  updated_at: string;
}
