# 02 — Data Model & Database Migrations

## Migration Tooling
- [ ] Set up golang-migrate for migration management
- [ ] Create migration runner in `internal/platform/database/`
- [ ] Add migration commands to Makefile

## Core Entity Migrations

### Organizations (001)
- [ ] `001_create_organizations.up.sql`
  - [ ] `organizations` table: id (UUID PK), name, slug (UNIQUE), plan, settings (JSONB), created_at, updated_at
- [ ] `001_create_organizations.down.sql`

### Users (002)
- [ ] `002_create_users.up.sql`
  - [ ] `users` table: id (UUID PK), email (UNIQUE), name, avatar_url, auth_provider, auth_provider_id, created_at, last_login_at
- [ ] `002_create_users.down.sql`

### Organization Membership (003)
- [ ] `003_create_org_members.up.sql`
  - [ ] `org_members` table: org_id (FK), user_id (FK), role (CHECK: owner/admin/member), joined_at, PRIMARY KEY (org_id, user_id)
- [ ] `003_create_org_members.down.sql`

### Projects (004)
- [ ] `004_create_projects.up.sql`
  - [ ] `projects` table: id (UUID PK), org_id (FK), name, slug, description, repo_url, settings (JSONB), created_at, updated_at, UNIQUE (org_id, slug)
- [ ] `004_create_projects.down.sql`

### Project Membership (005)
- [ ] `005_create_project_members.up.sql`
  - [ ] `project_members` table: project_id (FK), user_id (FK), role (CHECK: admin/editor/viewer/deployer), PRIMARY KEY (project_id, user_id)
- [ ] `005_create_project_members.down.sql`

### Environments (006)
- [ ] `006_create_environments.up.sql`
  - [ ] `environments` table: id (UUID PK), project_id (FK), name, slug, is_production, requires_approval, settings (JSONB), sort_order, created_at, UNIQUE (project_id, slug)
- [ ] `006_create_environments.down.sql`

### API Keys (007)
- [ ] `007_create_api_keys.up.sql`
  - [ ] `api_keys` table: id (UUID PK), project_id (FK), name, key_hash (UNIQUE), key_prefix, scopes (TEXT[]), environment, created_by (FK), expires_at, last_used_at, created_at
- [ ] `007_create_api_keys.down.sql`

### Audit Log (008)
- [ ] `008_create_audit_log.up.sql`
  - [ ] `audit_log` table: id (UUID PK), org_id, project_id, user_id, action, resource_type, resource_id, old_value (JSONB), new_value (JSONB), ip_address (INET), user_agent, created_at
  - [ ] Index on (created_at)
  - [ ] Index on (org_id, created_at)
- [ ] `008_create_audit_log.down.sql`

## Deploy Service Migrations

### Deploy Pipelines (009)
- [ ] `009_create_deploy_pipelines.up.sql`
  - [ ] `deploy_pipelines` table: id (UUID PK), project_id (FK), name, strategy (CHECK: canary/blue_green/rolling), config (JSONB), created_at, updated_at
- [ ] `009_create_deploy_pipelines.down.sql`

### Deployments (010)
- [ ] `010_create_deployments.up.sql`
  - [ ] `deployments` table: id (UUID PK), pipeline_id (FK), release_id (FK), environment, status (CHECK: pending/in_progress/paused/promoting/completed/rolling_back/failed), started_at, completed_at, initiated_by (FK), metadata (JSONB), created_at
- [ ] `010_create_deployments.down.sql`

### Deployment Phases (011)
- [ ] `011_create_deployment_phases.up.sql`
  - [ ] `deployment_phases` table: id (UUID PK), deployment_id (FK), phase_number, traffic_pct (CHECK: 0-100), duration_secs, status (CHECK: pending/active/passed/failed/skipped), health_snapshot (JSONB), started_at, completed_at
- [ ] `011_create_deployment_phases.down.sql`

## Feature Flag Migrations

### Feature Flags (012)
- [ ] `012_create_feature_flags.up.sql`
  - [ ] `feature_flags` table: id (UUID PK), project_id (FK), key, name, description, flag_type (CHECK: boolean/string/number/json), default_value (JSONB), enabled, tags (TEXT[]), created_by (FK), created_at, updated_at, archived_at, UNIQUE (project_id, key)
- [ ] `012_create_feature_flags.down.sql`

### Flag Targeting Rules (013)
- [ ] `013_create_flag_targeting_rules.up.sql`
  - [ ] `flag_targeting_rules` table: id (UUID PK), flag_id (FK CASCADE), environment, priority, rule_type (CHECK: percentage/user_target/attribute/segment/schedule), conditions (JSONB), serve_value (JSONB), enabled, created_at
- [ ] `013_create_flag_targeting_rules.down.sql`

### Flag Evaluation Log (014)
- [ ] `014_create_flag_evaluation_log.up.sql`
  - [ ] `flag_evaluation_log` table: id (UUID PK), flag_id, flag_key, environment, context_hash, result_value (JSONB), rule_matched, evaluated_at
  - [ ] Index on (evaluated_at) — partitioned by month
- [ ] `014_create_flag_evaluation_log.down.sql`

## Release Tracker Migrations

### Releases (015)
- [ ] `015_create_releases.up.sql`
  - [ ] `releases` table: id (UUID PK), project_id (FK), version, commit_sha, branch, changelog, artifact_url, status (CHECK: building/built/deploying/deployed/healthy/degraded/rolled_back), created_by (FK), created_at, UNIQUE (project_id, version)
- [ ] `015_create_releases.down.sql`

### Release Environments (016)
- [ ] `016_create_release_environments.up.sql`
  - [ ] `release_environments` table: id (UUID PK), release_id (FK), environment, status, deployed_at, health_score (NUMERIC 5,2), UNIQUE (release_id, environment)
- [ ] `016_create_release_environments.down.sql`

## Webhook Migrations

### Webhook Endpoints (017)
- [ ] `017_create_webhook_endpoints.up.sql`
  - [ ] `webhook_endpoints` table: id (UUID PK), project_id (FK), url, secret, events (TEXT[]), enabled, created_at
- [ ] `017_create_webhook_endpoints.down.sql`

### Webhook Deliveries (018)
- [ ] `018_create_webhook_deliveries.up.sql`
  - [ ] `webhook_deliveries` table: id (UUID PK), endpoint_id (FK), event_type, payload (JSONB), response_status, response_body, delivered_at, attempts, next_retry_at, status (CHECK: pending/delivered/failed), created_at
- [ ] `018_create_webhook_deliveries.down.sql`
