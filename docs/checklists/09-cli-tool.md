# 09 — CLI Tool

## CLI Framework Setup
- [ ] Go CLI with Cobra framework (`cmd/cli/main.go`)
- [ ] Single binary distribution (cross-platform)
- [ ] Configuration file support (`.deploysentry.yml`)
- [ ] Global flags: `--org`, `--project`, `--env`, `--output` (json/table/yaml)
- [ ] Colored output and progress indicators
- [ ] Version command with build info

## Auth Commands (`auth`)
- [ ] `deploysentry auth login` — Authenticate via browser OAuth flow
- [ ] `deploysentry auth logout` — Clear local credentials
- [ ] `deploysentry auth status` — Show current auth status
- [ ] Token storage (OS keychain or config file)
- [ ] Token refresh on expiry

## Deploy Commands (`deploy`)
- [ ] `deploysentry deploy create` — Create a new deployment
  - [ ] Flags: `--release`, `--env`, `--strategy` (canary/bluegreen/rolling)
- [ ] `deploysentry deploy status` — Show deployment status
  - [ ] `--watch` flag for live updates
- [ ] `deploysentry deploy promote` — Advance canary to next phase
- [ ] `deploysentry deploy rollback` — Trigger rollback
- [ ] `deploysentry deploy pause` — Pause active deployment
- [ ] `deploysentry deploy resume` — Resume paused deployment
- [ ] `deploysentry deploy list` — List recent deployments
  - [ ] Filtering by environment, status
- [ ] `deploysentry deploy logs` — Stream deployment logs

## Feature Flag Commands (`flags`)
- [ ] `deploysentry flags create` — Create a new feature flag
  - [ ] Flags: `--key`, `--type` (boolean/string/number/json), `--default`
- [ ] `deploysentry flags list` — List flags with filtering
  - [ ] Flags: `--tag`, `--status`, `--search`
- [ ] `deploysentry flags get` — Get flag details
- [ ] `deploysentry flags toggle` — Toggle flag on/off
  - [ ] Flags: `--on`, `--off`
- [ ] `deploysentry flags update` — Update flag configuration
  - [ ] `--add-rule` (JSON rule definition)
- [ ] `deploysentry flags evaluate` — Test flag evaluation locally
  - [ ] `--context` (JSON context)
- [ ] `deploysentry flags archive` — Archive a flag

## Release Commands (`releases`)
- [ ] `deploysentry releases create` — Create a new release
  - [ ] Flags: `--version`, `--commit`
- [ ] `deploysentry releases list` — List releases
- [ ] `deploysentry releases status` — Show release status across environments
- [ ] `deploysentry releases promote` — Promote release to next environment

## Project Commands (`projects`)
- [ ] `deploysentry projects create` — Create a new project
- [ ] `deploysentry projects list` — List projects
- [ ] `deploysentry projects config` — View/update project configuration

## Config Commands (`config`)
- [ ] `deploysentry config init` — Initialize project config (`.deploysentry.yml`)
- [ ] `deploysentry config set` — Set configuration value
- [ ] `deploysentry config get` — Get configuration value

## Cross-Cutting CLI Concerns
- [ ] HTTP client with auth header injection
- [ ] Error handling with user-friendly messages
- [ ] API base URL configuration
- [ ] Request/response debug mode (`--verbose`)
- [ ] Shell completion (bash, zsh, fish)
