# 09 — CLI Tool

## CLI Framework Setup
- [x] Go CLI with Cobra framework (`cmd/cli/main.go`)
- [x] Single binary distribution (cross-platform)
- [x] Configuration file support (`.deploysentry.yml`)
- [x] Global flags: `--org`, `--project`, `--env`, `--output` (json/table/yaml)
- [x] Colored output and progress indicators
- [x] Version command with build info

## Auth Commands (`auth`)
- [x] `deploysentry auth login` — Authenticate via browser OAuth flow
- [x] `deploysentry auth logout` — Clear local credentials
- [x] `deploysentry auth status` — Show current auth status
- [x] Token storage (OS keychain or config file)
- [x] Token refresh on expiry

## Deploy Commands (`deploy`)
- [x] `deploysentry deploy create` — Create a new deployment
  - [x] Flags: `--release`, `--env`, `--strategy` (canary/bluegreen/rolling)
- [x] `deploysentry deploy status` — Show deployment status
  - [x] `--watch` flag for live updates
- [x] `deploysentry deploy promote` — Advance canary to next phase
- [x] `deploysentry deploy rollback` — Trigger rollback
- [x] `deploysentry deploy pause` — Pause active deployment
- [x] `deploysentry deploy resume` — Resume paused deployment
- [x] `deploysentry deploy list` — List recent deployments
  - [x] Filtering by environment, status
- [x] `deploysentry deploy logs` — Stream deployment logs

## Feature Flag Commands (`flags`)
- [x] `deploysentry flags create` — Create a new feature flag
  - [x] Flags: `--key`, `--type` (boolean/string/number/json), `--default`
- [x] `deploysentry flags list` — List flags with filtering
  - [x] Flags: `--tag`, `--status`, `--search`
- [x] `deploysentry flags get` — Get flag details
- [x] `deploysentry flags toggle` — Toggle flag on/off
  - [x] Flags: `--on`, `--off`
- [x] `deploysentry flags update` — Update flag configuration
  - [x] `--add-rule` (JSON rule definition)
- [x] `deploysentry flags evaluate` — Test flag evaluation locally
  - [x] `--context` (JSON context)
- [x] `deploysentry flags archive` — Archive a flag

## Release Commands (`releases`)
- [x] `deploysentry releases create` — Create a new release
  - [x] Flags: `--version`, `--commit`
- [x] `deploysentry releases list` — List releases
- [x] `deploysentry releases status` — Show release status across environments
- [x] `deploysentry releases promote` — Promote release to next environment

## Project Commands (`projects`)
- [x] `deploysentry projects create` — Create a new project
- [x] `deploysentry projects list` — List projects
- [x] `deploysentry projects config` — View/update project configuration

## Config Commands (`config`)
- [x] `deploysentry config init` — Initialize project config (`.deploysentry.yml`)
- [x] `deploysentry config set` — Set configuration value
- [x] `deploysentry config get` — Get configuration value

## Cross-Cutting CLI Concerns
- [x] HTTP client with auth header injection
- [x] Error handling with user-friendly messages
- [x] API base URL configuration
- [x] Request/response debug mode (`--verbose`)
- [x] Shell completion (bash, zsh, fish)
