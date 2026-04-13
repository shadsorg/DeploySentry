# CLI Subcommands & Install Script Design

**Date:** 2026-03-30
**Scope:** Add org, app, settings, apikey CLI subcommands + install script

## Summary

Add four new CLI subcommand files following the existing Cobra pattern (matching `projects.go`), plus a shell install script. All subcommands use `clientFromConfig()` for API access and support both JSON and table output.

## Subcommands

### `orgs` (`cmd/cli/orgs.go`)
- `orgs list` — list organizations the user belongs to
- `orgs create --name <name> --slug <slug>` — create a new organization
- `orgs set <slug>` — set active org in config (convenience for `config set org <slug>`)

API paths: `GET /api/v1/orgs`, `POST /api/v1/orgs`

### `apps` (`cmd/cli/apps.go`)
- `apps list` — list applications in the current project (requires `--org` + `--project`)
- `apps create --name <name> --slug <slug>` — create a new application
- `apps get <slug>` — get application details

API paths: `GET/POST /api/v1/orgs/{org}/projects/{project}/apps`, `GET /api/v1/orgs/{org}/projects/{project}/apps/{slug}`

### `settings` (`cmd/cli/settings.go`)
- `settings list --scope <scope> --target <id>` — list settings for a scope/target
- `settings set --scope <scope> --target <id> --key <key> --value <value>` — set a setting
- `settings delete <id>` — delete a setting

API paths: `GET /api/v1/settings?scope=...&target=...`, `PUT /api/v1/settings`, `DELETE /api/v1/settings/{id}`

### `apikeys` (`cmd/cli/apikeys.go`)
- `apikeys list` — list API keys
- `apikeys create --name <name> --scopes <scopes>` — create a new API key (displays token once)
- `apikeys revoke <id>` — revoke an API key

API paths: `GET /api/v1/api-keys`, `POST /api/v1/api-keys`, `DELETE /api/v1/api-keys/{id}`

## Install Script

`scripts/install.sh` — downloads the latest release binary for the user's OS/arch and installs to `/usr/local/bin/deploysentry`. Supports Linux and macOS, amd64 and arm64.

## Pattern

All subcommands follow the exact pattern from `projects.go`:
- Cobra command definitions with Use, Short, Long, RunE
- `init()` function registers flags and adds commands
- `clientFromConfig()` for API access
- `getOutputFormat() == "json"` check for output mode
- `tabwriter` for table output
- `requireOrg()` / `requireProject()` for mandatory context
