# End-to-End SDK Flag Delivery Test Suite — Design Spec

**Phase**: Design
**Date**: 2026-04-12

## Overview

A new end-to-end test suite that proves the full DeploySentry chain — web dashboard → REST API → Postgres → NATS → Sentinel SSE → client SDK — works as designed in a single assertion. The existing Playwright suite already covers UI ↔ backend (smoke) and UI in isolation (mocked); this spec closes the remaining gap: **does a flag change made in the dashboard actually reach live SDK clients on the three runtimes DeploySentry users care about most (Node, React, Flutter)?**

## Goals

- A single green check per PR means "UI change propagates to Node, React, and Flutter SDKs within a bounded latency, with correct targeting evaluation."
- Hermetic: runs identically in CI and on a developer laptop with only Docker + Node installed.
- Attributable failures: when the suite goes red, the artifact bundle identifies which leg of the chain broke (UI write, persistence, Sentinel fan-out, or a specific SDK).
- Bounded CI cost: per-PR run under 5 minutes; Flutter leg isolated to a nightly job.

## Non-Goals

- Reconnection / network-partition resilience (deferred to a separate resilience suite).
- Coverage for Go, Python, Java, Ruby SDKs — contract tests in `sdk/*/` cover those.
- Load / throughput testing.
- CLI coverage (existing unit tests cover it).

## Architecture

```
┌─────────────────── docker-compose.e2e.yml (hermetic, per run) ──────────────────┐
│  postgres   redis   nats-jetstream   deploysentry-api:8080   sentinel:8090      │
└──────────────────────────────────────────────────────────────────────────────────┘
                 ▲                           ▲                ▲
                 │ REST                      │ SSE            │ SSE
                 │                           │                │
┌────────────────┴────────────┐     ┌────────┴───────┐  ┌─────┴────────┐
│ Playwright spec (driver)    │     │ Node probe     │  │ React probe  │
│  - logs in to dashboard     │     │ (subprocess,   │  │ (2nd browser │
│  - creates org/project/app  │     │  sdk/node)     │  │  context,    │
│  - creates flag + rules     │     │                │  │  sdk/react   │
│  - toggles flag value       │     └────────────────┘  │  harness)    │
│  - asserts probe observations│                         └──────────────┘
└──────────────────────────────┘             ▲
                                      ┌──────┴────────┐
                                      │ Flutter probe │
                                      │ (nightly only)│
                                      │ sdk/flutter   │
                                      └───────────────┘
```

**Lifecycle:** compose up → wait for `/healthz` → Playwright spec performs UI setup and captures API key → spec launches probes → probes report baseline observations via IPC (file tail for Node, `window.__ds_observations` for React, stdout JSON lines for Flutter) → spec mutates flag via UI → spec awaits expected observation on every probe with a latency budget → assertions → compose down.

**Key design principle:** one test, one assertion chain. The dashboard click is the input; the probe observations are the outputs; the test fails if any link in between drops the change.

## Components

### 1. `docker-compose.e2e.yml` (repo root)

New hermetic compose file, separate from the existing `docker-compose.yml` used by `make dev-up`. Defines postgres, redis, nats-jetstream, deploysentry-api, and sentinel with:
- ephemeral named volumes (wiped each run)
- deterministic non-conflicting ports so developers can run dev-up and e2e-sdk simultaneously
- healthchecks on every service; compose up uses `--wait`
- API container built from the current checkout (`build: .`), not a published image, so the test exercises branch code

### 2. `web/e2e/sdk-probes/` (new directory)

**`node-probe/index.js`** — imports `@deploysentry/node` from `sdk/node` via a local path reference. Reads `DS_API_KEY`, `DS_API_URL`, `DS_CONTEXT_JSON`, `DS_FLAG_KEYS`, `OBSERVATIONS_FILE` from env. Subscribes to each flag, writes every observation as a JSON line `{flagKey, value, ts}` to `OBSERVATIONS_FILE`. Handles SIGTERM by flushing and exiting cleanly.

**`react-harness/`** — minimal Vite-built static page that mounts `<DeploySentryProvider>` from `sdk/react`, calls `useFlag()` for flag keys passed via URL query params, and writes every observed value to `window.__ds_observations` (append-only array). Served statically; loaded in a second Playwright browser context.

**`flutter-probe/`** — a Dart `integration_test` binary using `sdk/flutter`, reading the same env vars as the Node probe and writing observations to stdout as JSON lines. Only built and run when `RUN_FLUTTER_PROBE=1`.

### 3. `web/e2e/helpers/sdk-driver.ts` (new)

Shared probe lifecycle helpers consumed by specs:
- `startNodeProbe(ctx: ProbeContext): Promise<Probe>` — spawns the Node subprocess, tails the observations file, exposes `observations()` and `stop()`.
- `startReactProbe(browser, ctx): Promise<Probe>` — opens a second browser context on the harness URL, polls `window.__ds_observations` via `page.evaluate`.
- `startFlutterProbe(ctx): Promise<Probe>` — spawns `flutter test` subprocess, parses stdout. No-op (returns a stub probe) when `RUN_FLUTTER_PROBE` is unset.
- `waitForValue(probe, flagKey, expected, {timeoutMs}): Promise<number>` — resolves with observed latency in ms or throws with the full observation log.

All probes expose an identical `Probe` interface so scenarios don't branch on SDK.

### 4. `web/e2e/sdk/flag-delivery.spec.ts` (new)

Single spec file containing the three scenarios (see Data Flow below). Uses a `beforeAll` that performs UI login and org/project/app setup once, and a `beforeEach` that creates the per-scenario flag so assertions are independent.

### 5. `playwright.config.ts` (modified)

Add a third project `sdk` alongside existing `smoke` and `ui`:
- `testDir: e2e/sdk`
- serial execution (`fullyParallel: false`, `workers: 1`) — single compose stack can't support parallel flag mutations without interference
- `timeout: 90_000` per test
- `retries: 2` (see calibration note below)
- `trace: 'on-first-retry'`
- `globalSetup` that asserts compose stack is healthy before any spec runs (fails fast if the operator forgot `docker compose up`)

### 6. `Makefile` (modified)

New targets:
- `e2e-sdk` — `docker compose -f docker-compose.e2e.yml up -d --wait && make migrate-up (against e2e DSN) && npx playwright test --project=sdk && docker compose -f docker-compose.e2e.yml down -v`
- `e2e-sdk-debug` — same but `--headed --debug` and no teardown
- `e2e-sdk-nightly` — sets `RUN_FLUTTER_PROBE=1`, otherwise identical to `e2e-sdk`

### 7. CI workflows (new)

**`.github/workflows/e2e-sdk.yml`** — per-PR gate
- Path filter: `sdk/node/**`, `sdk/react/**`, `internal/flags/**`, `cmd/api/**`, `web/e2e/sdk/**`, `docker-compose.e2e.yml`
- Steps: checkout → setup Go, Node → compose up --wait → migrate → install web deps → `npx playwright install chromium` → `make e2e-sdk` → upload `playwright-report/` and `test-results/` on failure
- Budget: ~4 min target

**`.github/workflows/e2e-sdk-nightly.yml`** — nightly cron on `main`
- Same as per-PR plus Flutter toolchain setup and `RUN_FLUTTER_PROBE=1`
- Budget: ~10 min
- On failure, update a tracking GitHub issue tagged `e2e-flake`

## Data Flow & Scenarios

All three scenarios share a compose stack and the same org/project/app. Each creates its own flag so assertions remain independent.

### Scenario A — Baseline propagation (all three probes)

1. UI: create flag `e2e-baseline` with default value `false`, no rules.
2. Start Node + React + Flutter (if enabled) probes with context `{userId: "u1"}`.
3. Assert each probe reports `false` as its first observation within 3s of start (proves SSE connect + initial sync).
4. UI: toggle the flag to `true`, capturing the click timestamp via `performance.now()`.
5. Assert each probe observes `true` within 2s of the click timestamp.
6. Record observed latencies in test output for trend tracking.

### Scenario B — Targeting correctness (Node only, two contexts)

1. UI: create flag `e2e-targeting` with default `false` and a rule `user.plan == "pro" → true`.
2. Start two Node probes: context `{userId: "u1", plan: "free"}` and `{userId: "u2", plan: "pro"}`. Targeting is SDK-runtime-agnostic, so running it only on Node keeps the scenario fast without losing coverage.
3. Assert the `free` probe observes `false`, the `pro` probe observes `true`.
4. UI: edit the rule to `user.plan == "enterprise" → true`.
5. Assert both probes now observe `false` within 2s.

### Scenario C — Variant delivery (Node + React)

1. UI: create multivariate flag `e2e-variant` with variants `control|treatment` and a 100/0 split.
2. Start Node + React probes.
3. Assert each observes `control`.
4. UI: change the split to 0/100.
5. Assert each observes `treatment` within 2s.

### Failure visibility

On any assertion failure, the helper dumps:
- full observation log for every probe
- Playwright trace (`on-first-retry`)
- API response from `/debug/flags/:key/subscribers` (if available)
- compose container logs

This ensures a red build identifies the failing leg (UI write didn't persist, Sentinel didn't fan out, or a specific SDK didn't receive).

## Testing the Tests (Meta)

Before the suite becomes a required gate:

1. **Known-good soak:** run 20× locally and 20× in CI against unchanged `main`. Zero flakes required. If flakes appear, root-cause them — do not raise timeouts.
2. **Known-bad fault injection:** manually verify each of the following breakages causes a correctly attributed failure (throwaway diffs, not committed):
   - Stop the Sentinel container mid-test → scenario A fails with "probe never observed baseline."
   - Patch the API to no-op flag update → scenario A fails on the post-toggle assertion.
   - Patch the Node SDK to ignore SSE `flag.updated` events → scenario A fails *only* for the Node probe, not React/Flutter.
   - Patch targeting rule evaluation to ignore `plan` → scenario B fails.
3. **Latency calibration:** before hard-coding the 2s ceilings, run scenario A 50× to measure actual p95 SSE propagation in the compose stack. Set the ceiling at 2× p95 (floor 1s). Record the measurement in the implementation plan's completion record.

## Rollout

1. **Phase 1 — infra only.** Land compose file, playwright project, helpers, empty spec that boots the stack and exits green. Non-required check.
2. **Phase 2 — scenario A for Node + React.** Still non-required. Soak 3 days.
3. **Phase 3 — scenarios B and C.** Soak 3 days.
4. **Phase 4 — mark per-PR job required** in branch protection.
5. **Phase 5 — add Flutter probe to nightly** (never part of the per-PR required gate).

## Definition of Done

- Per-PR `e2e-sdk` job required on `main`.
- Nightly `e2e-sdk-nightly` job green for 7 consecutive days.
- Fault-injection checks documented in the implementation plan's completion record.
- `docs/Current_Initiatives.md` updated.

## Open Questions

None at design time. Implementation-time decisions (exact port numbers for the e2e compose stack, the precise Vite harness build target) will be resolved in the implementation plan.
