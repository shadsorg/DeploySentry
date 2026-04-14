# CLAUDE.md — DeploySentry project context

## Database Schema

All DeploySentry tables live in the PostgreSQL `deploy` schema (namespace), NOT in `public`.

- The `deploy` schema is created by migration `000_create_deploy_schema.up.sql`
- The connection DSN includes `search_path=deploy` so all queries target the correct schema
- Config default: `DS_DATABASE_SCHEMA=deploy`
- Makefile `MIGRATE_DSN` includes `&search_path=deploy`
- When writing new migrations, do NOT prefix table names with `deploy.` — the search_path handles it
- When running raw SQL outside the app (e.g. psql), set the schema first: `SET search_path TO deploy;`

## Build & Run

```bash
make dev-up        # Start PostgreSQL, Redis, NATS via Docker Compose
make migrate-up    # Run all migrations (into the deploy schema)
make run-api       # Start API server on :8080
make run-web       # Start React dashboard on :3000
make test          # Run all Go tests
```

## Production Security

### NATS
- NATS must be firewalled to internal services only — not exposed to the internet
- Enable authentication: set `DS_NATS_USER` and `DS_NATS_PASSWORD` for NATS connection credentials
- Enable TLS: set `DS_NATS_TLS_CERT` and `DS_NATS_TLS_KEY` for encrypted connections
- The phase engine validates deployment IDs from NATS messages against the database before processing

## Project Structure

- `cmd/api/` — API server entrypoint
- `cmd/cli/` — CLI tool (subcommands: auth, config, deploy, flags, projects, releases, webhooks, analytics, orgs, apps, settings, apikeys)
- `internal/auth/` — Authentication, RBAC, middleware (includes ResolveOrgRole for org-scoped routes)
- `internal/deploy/` — Deployment service and strategies
- `internal/entities/` — Organization, project, and application CRUD
- `internal/flags/` — Feature flag service, targeting rules, evaluation
- `internal/health/` — Health monitoring and integrations
- `internal/members/` — Org and project member management (repository, service, handler)
- `internal/notifications/` — Slack, email, PagerDuty notification channels
- `internal/ratings/` — Flag ratings
- `internal/releases/` — Release tracking
- `internal/rollback/` — Automated rollback controller
- `internal/settings/` — Hierarchical settings (org > project > app > environment)
- `internal/platform/` — Shared infra (database, cache, messaging, config, middleware)
- `migrations/` — PostgreSQL migrations (deploy schema, currently 032)
- `sdk/` — Client SDKs (go, node, python, java, react, flutter, ruby)
- `web/` — React + TypeScript dashboard (19 pages)
- `mobile/` — Flutter mobile app
- `deploy/` — Docker, Kubernetes, docker-compose configs
- `scripts/` — Install script and utilities

## Flag Categories

Flags must have a `category`: release, feature, experiment, ops, or permission.
Release flags require an `expires_at` date or `is_permanent = true`.

## Member Management

- Org members: `/api/v1/orgs/:orgSlug/members` (GET, POST, PUT, DELETE)
- Project members: `/api/v1/orgs/:orgSlug/projects/:projectSlug/members`
- Org roles: owner, admin, member, viewer
- Project roles: admin, developer, viewer
- `ResolveOrgRole` middleware resolves org role from `org_members` table and sets it in Gin context for `RequirePermission`

## Web Dashboard Pages

The dashboard is at `web/` (Vite + React + TypeScript, port 3001):
- Login, Register, Create Org, Create Project, Create App
- ProjectListPage, ApplicationsListPage, FlagListPage, FlagDetailPage, FlagCreatePage
- DeploymentsPage, DeploymentDetailPage, ReleasesPage, ReleaseDetailPage
- MembersPage, APIKeysPage, SettingsPage (org/project/app levels)
- AnalyticsPage, SDKsPage

## Session Directives (MANDATORY)

Before starting any work, read and follow `docs/README.md`. It defines three mandatory processes:

1. **Prompt History** — Log every prompt to `docs/PROMPT_HISTORY.md` before beginning work. Update with results before ending the session.
2. **Current Initiatives** — Update `docs/CURRENT_INITIATIVES.md` before ending any session to reflect the current state of all specs and plans.
3. **Spec-First Workflow** — For non-trivial tasks, always clarify → spec (`docs/superpowers/specs/`) → plan (`docs/superpowers/plans/`) → execute, even without the Superpowers plugin.

Completed specs and plans are archived to `docs/archives/`.
