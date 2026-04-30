# CLI Auth + Onboarding Fix

**Status**: Implementation
**Date**: 2026-04-23

## Problem

`deploysentry auth login` opens a browser pointed at `https://api.dr-sentry.com/oauth/authorize?...` which 404s. There is no `/oauth/authorize` handler on the API — the server-side OAuth endpoints were never implemented. The CLI has been shipping with a broken login flow, and the onboarding docs (`Getting_Started.md`, `Bootstrap_My_App.md`, `Deploy_Integration_Guide.md`) instruct users to run that broken command.

## Fix

Replace the OAuth codepath in `cmd/cli/auth.go` with an API-key persistence flow.

1. `deploysentry auth login` now:
   - Accepts the key via `--token <value>`, or `DEPLOYSENTRY_API_KEY` env var, or interactive stdin prompt.
   - Validates the key with `GET /api/v1/orgs` before persisting (any authenticated endpoint; `/orgs` is cheap).
   - Writes to the same `~/.config/deploysentry/credentials.json` the existing tooling reads, but with `token_type: "api_key"`.
   - Prints "create a key at https://app.dr-sentry.com/orgs/<your-org>/api-keys → paste it here" when neither flag nor env is supplied.

2. `ensureValidToken` in `cmd/cli/auth.go` no longer calls the phantom `/oauth/token` refresh endpoint. When `token_type == "api_key"`, the credential never expires; when `token_type == "bearer"` (legacy path, unreachable from current login but respected if a user migrated a file from a pre-existing session), return as-is.

3. `deploy.go` + `internal/mcp/context.go` look at `token_type` to decide which auth scheme to use:
   - `api_key` → `Authorization: ApiKey <value>`.
   - anything else → `Authorization: Bearer <value>` (legacy; keeps door open for future real OAuth).

4. Onboarding docs rewritten so:
   - `Getting_Started.md` reorders to put **CLI install + auth + MCP install** in the first three steps, not a shortcut at step 11.
   - `Bootstrap_My_App.md` + `Deploy_Integration_Guide.md` reflect the API-key-based `auth login` flow, and delete any OAuth mentions.

## Tasks

- `cmd/cli/auth.go`: rewrite `runAuthLogin`, add `--token` flag, drop `refreshToken` calls (keep it as a no-op returning `creds, nil` for backward compat with `ensureValidToken`).
- `cmd/cli/client.go`: keep `setToken` / `setAPIKey` unchanged; add a helper `applyCredentials(creds)` that picks the right auth scheme.
- `cmd/cli/deploy.go`: swap the `client.setToken(creds.AccessToken)` line for `client.applyCredentials(creds)`.
- `internal/mcp/context.go`: same swap.
- Docs: edits to the three onboarding files.

## Verification

```
go build ./...
go test  ./...
```

Manual smoke:
```
deploysentry auth login --token ds_live_your_key      # persists
deploysentry auth status                               # shows authenticated as <user>
deploysentry orgs list                                 # succeeds
deploysentry auth logout                               # clears
```

## Out of scope

Real OAuth / SSO implementation on the server. Revisit when we have real browser-facing flows (dashboard-initiated token exchange, social login). For now, the dashboard's "create API key" UI is the canonical auth ceremony.
