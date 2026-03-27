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

## Project Structure

- `cmd/api/` — API server entrypoint
- `cmd/cli/` — CLI tool entrypoint
- `internal/` — All backend services (auth, deploy, flags, health, notifications, releases, rollback)
- `internal/platform/` — Shared infra (database, cache, messaging, config, middleware)
- `migrations/` — PostgreSQL migrations (deploy schema)
- `sdk/` — Client SDKs (go, node, python, java, react, flutter, ruby)
- `web/` — React + TypeScript dashboard
- `deploy/` — Docker, Kubernetes, docker-compose configs

## Flag Categories

Flags must have a `category`: release, feature, experiment, ops, or permission.
Release flags require an `expires_at` date or `is_permanent = true`.
