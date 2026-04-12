# CLI

The `deploysentry` CLI manages organizations, projects, applications, deployments, releases, and flags from the terminal.

## Install

```bash
go install github.com/shadsorg/DeploySentry/cmd/cli@latest
```

## Authenticate

```bash
deploysentry auth login
```

## Common commands

| Command | What it does |
|---|---|
| `deploysentry orgs list` | List organizations you belong to |
| `deploysentry projects list` | List projects in the current org |
| `deploysentry apps list` | List applications in the current project |
| `deploysentry flags list` | List flags |
| `deploysentry flags create <key>` | Create a flag |
| `deploysentry deploy create` | Trigger a deployment |
| `deploysentry releases list` | List releases |
| `deploysentry apikeys create` | Create an API key |

## Examples

```bash
# List flags in the current project
deploysentry flags list

# Create a release flag
deploysentry flags create new-checkout --category release --expires 2026-09-01

# Toggle a flag for the production environment
deploysentry flags set new-checkout --env production --on
```
