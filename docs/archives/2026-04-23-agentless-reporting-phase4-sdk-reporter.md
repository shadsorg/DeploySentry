# Agentless Deploy Reporting — Phase 4: SDK reporter (Node + Go)

**Status**: Implementation
**Spec**: [2026-04-23-agentless-deploy-reporting-design.md](../specs/2026-04-23-agentless-deploy-reporting-design.md)
**Date**: 2026-04-23

## Scope

Embed the `/status` caller in the two first-party server SDKs shipped today:

- `sdk/node` (`@dr-sentry/sdk`)
- `sdk/go`

Both SDKs gain an opt-in status reporter that, when enabled, fires a `POST /api/v1/applications/:app_id/status` on init and on an interval, with automatic version discovery from env vars / package metadata and a default "process alive = healthy" floor. Phases 5 (Python/Java/Ruby) ports the same contract.

## Tasks

### T1 — Node SDK: reporter module

- Extend `ClientOptions` (`sdk/node/src/types.ts`):
  - `applicationId?: string` — UUID. Required when `reportStatus=true`.
  - `reportStatus?: boolean` (default `false`).
  - `reportStatusIntervalMs?: number` (default 30_000; `0` = startup-only).
  - `reportStatusVersion?: string` (override auto-detection).
  - `reportStatusCommitSha?: string`.
  - `reportStatusDeploySlot?: string` (`stable` / `canary` / …).
  - `reportStatusTags?: Record<string, string>`.
  - `reportStatusHealthProvider?: () => Promise<HealthReport> | HealthReport`.
- Add a `HealthReport` type in the same file: `{ state: 'healthy' | 'degraded' | 'unhealthy' | 'unknown'; score?: number; reason?: string }`.
- New file `sdk/node/src/status-reporter.ts`:
  - Exports `StatusReporter` class with `start()`, `stop()`, and `reportOnce()`.
  - `start()` fires one report, then (if interval > 0) schedules repeats.
  - `reportOnce()` resolves the version (override → env vars → `package.json`), builds payload, POSTs to `{baseURL}/api/v1/applications/{applicationId}/status` with the SDK's existing `ApiKey` auth header.
  - On failure: log at `console.warn`, apply exponential backoff (1s, 2s, 4s… cap 5m) before the next retry. Flag evaluation must not be affected.
  - Version env-var chain (first non-empty): `APP_VERSION`, `GIT_SHA`, `GIT_COMMIT`, `SOURCE_COMMIT`, `RAILWAY_GIT_COMMIT_SHA`, `RENDER_GIT_COMMIT`, `VERCEL_GIT_COMMIT_SHA`, `HEROKU_SLUG_COMMIT`. Fallback to reading the *consuming* app's `package.json.version` via `process.env.npm_package_version` when available, else the literal `"unknown"` (with a one-time warning).
- Wire into `DeploySentryClient`:
  - `initialize()` constructs a `StatusReporter` when `reportStatus=true` and `applicationId` is set; calls `reporter.start()`. If `reportStatus=true` but no `applicationId`, log a single warning and skip — flag evaluation continues unaffected.
  - `close()` calls `reporter.stop()`.
- Export `StatusReporter` and `HealthReport` from `sdk/node/src/index.ts`.

### T2 — Node SDK: tests

`sdk/node/src/__tests__/status-reporter.test.ts`:

- Constructor rejects invalid intervals (`<0`).
- `reportOnce()` POSTs to the right URL with the right headers and payload (use `jest.mock` around `global.fetch` or pass in a custom `fetch` for injection).
- Health provider is awaited and its output reflected in the payload.
- Default health when no provider = `healthy`.
- Version auto-detection prefers explicit config > env var > fallback.
- Failed POST is swallowed (does not throw).

`sdk/node/src/__tests__/client.test.ts` (extend):

- Client with `reportStatus=true` + `applicationId` constructs reporter on init.
- Client with `reportStatus=true` and no `applicationId` logs a warning and does not crash.

### T3 — Go SDK: reporter

- New options in `sdk/go/client.go`:
  - `WithApplicationID(id string)` (UUID).
  - `WithReportStatus(enabled bool)`.
  - `WithReportStatusInterval(d time.Duration)` (default 30s; zero = startup-only).
  - `WithReportStatusVersion(v string)`, `WithReportStatusCommitSHA(sha string)`, `WithReportStatusDeploySlot(slot string)`, `WithReportStatusTags(m map[string]string)`.
  - `WithHealthProvider(func() (HealthReport, error))`.
- New type `HealthReport` with fields `State`, `Score *float64`, `Reason string`.
- New file `sdk/go/status_reporter.go`:
  - `statusReporter` struct with `start(ctx)`, `stop()`, `reportOnce(ctx)`.
  - Version chain uses env vars (same list as Node) and falls back to `debug.ReadBuildInfo().Main.Version`.
  - Failures logged via the SDK's logger; exponential backoff inside the ticker loop.
- Wire into `Client.Initialize`: when enabled and `applicationID` non-empty, spawn `reporter.start(ctx)` in a goroutine. `Close()` cancels a derived context to stop.

### T4 — Go SDK: tests

`sdk/go/status_reporter_test.go`:

- `TestReportOnce_PostsToCorrectURL` — use `httptest.NewServer` that captures the request; assert path, method, headers, body.
- `TestResolveVersion_ExplicitWins`, `TestResolveVersion_EnvVarFallback`, `TestResolveVersion_BuildInfoFallback`.
- `TestReportOnce_HealthProviderIsCalled`.
- `TestReportOnce_SwallowsServerError`.

### T5 — Docs

- `sdk/node/README.md`: add a "Status Reporting" section with an init snippet.
- `sdk/go/README.md`: same.
- Append a "Using the SDK reporter" note to `docs/Deploy_Integration_Guide.md`'s PaaS section, pointing at both SDK READMEs and noting that the SDK collapses Path B to a config flag.
- `docs/Current_Initiatives.md` row updated through Phase 4.

## Verification

```bash
cd sdk/node && npm test
cd sdk/go  && go test ./...
go build ./...    # top-level server build stays green
go test  ./...    # top-level server tests stay green
```

## Out of scope for this phase

- SDK reporter for Python / Java / Ruby (Phase 5).
- React / Flutter — client-side SDKs; different reporting story (deferred per spec).
- Short-lived runtime auto-detection (serverless defaults) — tracked as open question in the spec.
- Shutdown signal reporting (`health=unknown, reason="shutting_down"`) — nice-to-have, deferred.
