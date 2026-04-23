# Agentless Deploy Reporting — Phase 5: SDK reporter parity (Python, Java, Ruby)

**Status**: Implementation
**Spec**: [2026-04-23-agentless-deploy-reporting-design.md](../specs/2026-04-23-agentless-deploy-reporting-design.md)
**Date**: 2026-04-23

## Scope

Port the Node + Go status reporter (Phase 4) to the remaining first-party server SDKs:

- `sdk/python` (`deploysentry`)
- `sdk/java` (`io.deploysentry`)
- `sdk/ruby` (`deploysentry`)

Each SDK gains the same opt-in contract: config flag `report_status` / `reportStatus` / `report_status:`, optional interval / version override / health provider / deploy slot / tags. Behavior matches Phase 4 line-for-line: fire once on init, repeat on interval, exponential backoff on failure, swallow errors so flag evaluation is unaffected.

## Tasks

### T1 — Python

- New options on `DeploySentryClient.__init__`: `application_id`, `report_status`, `report_status_interval` (seconds, default 30), `report_status_version`, `report_status_commit_sha`, `report_status_deploy_slot`, `report_status_tags`, `report_status_health_provider`.
- New module `deploysentry/status_reporter.py` exporting `StatusReporter` and `resolve_version`. Uses a `threading.Timer` for repeat cadence (daemon thread so it doesn't block process exit). `reportOnce()` calls `POST {base_url}/api/v1/applications/{application_id}/status` via an `httpx.Client`.
- Mirror the Node env-var chain: `APP_VERSION`, `GIT_SHA`, `GIT_COMMIT`, `SOURCE_COMMIT`, `RAILWAY_GIT_COMMIT_SHA`, `RENDER_GIT_COMMIT`, `VERCEL_GIT_COMMIT_SHA`, `HEROKU_SLUG_COMMIT`. Fallback: `importlib.metadata.version(...)` guarded by try/except; else `"unknown"`.
- Wire `initialize()` to start the reporter when enabled and `application_id` is set. `close()` stops it.
- Tests: `tests/test_status_reporter.py` — URL/method/auth, health-provider integration, version resolution, error-path swallowed.
- README: "Status reporting (optional)" section.

### T2 — Java

- Extend `ClientOptions` (+ Builder) with `applicationId`, `reportStatus`, `reportStatusInterval`, `reportStatusVersion`, `reportStatusCommitSha`, `reportStatusDeploySlot`, `reportStatusTags` (`Map<String,String>`), `healthProvider` (`Supplier<HealthReport>`).
- New top-level types `HealthReport` and `StatusReporter` in `io.deploysentry`.
- Reporter uses a `ScheduledExecutorService` (daemon threads). `reportOnce()` builds a `HttpRequest` to `POST /api/v1/applications/{id}/status` with `Authorization: ApiKey …`, JSON body from `org.json`.
- Version chain: env vars → `System.getProperty("java.version")` is *not* used (meaningless); try `Package.getImplementationVersion()` via the caller's class; else `"unknown"`.
- Wire into `DeploySentryClient.initialize` / `close`.
- Tests with a `com.sun.net.httpserver.HttpServer` stub: URL, body, header, options-validation.
- README: "Status Reporting" section.

### T3 — Ruby

- Add new keyword args to `DeploySentry::Client#initialize`: `application_id:`, `report_status:` (default `false`), `report_status_interval:` (seconds, default 30), `report_status_version:`, `report_status_commit_sha:`, `report_status_deploy_slot:`, `report_status_tags:`, `report_status_health_provider:` (a callable).
- New file `sdk/ruby/lib/deploysentry/status_reporter.rb` with `StatusReporter` using `Thread` + `sleep` in a daemon loop; `#start`, `#stop`, `#report_once`. HTTP via `Net::HTTP` (SDK already uses it).
- Version chain via `ENV` + final `"unknown"` fallback.
- Wire into `Client#initialize!` + `#close`.
- Tests: `sdk/ruby/test/status_reporter_test.rb` — stub Net::HTTP with `webmock`-style request matching (if webmock is available) or hand-rolled `TCPServer` stub. Keep it lightweight; favor the hand-rolled stub for parity with other Ruby tests in the repo.
- README: "Status reporting (optional)" section.

### T4 — Docs

- Update `docs/Deploy_Integration_Guide.md`'s SDK subsection to list Python/Java/Ruby alongside Node/Go.
- Flip the Current_Initiatives row note to "Phases 1–5 implemented".

## Verification

```bash
cd sdk/python && python -m pytest tests/test_status_reporter.py
cd sdk/java   && mvn -q test -Dtest=StatusReporterTest
cd sdk/ruby   && ruby -Ilib -Itest test/status_reporter_test.rb
go build ./... && go test ./...
```

## Out of scope

- React / Flutter.
- Shutdown-hook "farewell" reports (deferred open question).
- Serverless-aware interval defaults.
