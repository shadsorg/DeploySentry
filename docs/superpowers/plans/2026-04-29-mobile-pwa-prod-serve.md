# Mobile PWA Production Serve

**Status**: Implementation (PR #65 open, CI failing)
**Date**: 2026-04-29
**Branch**: `feature/mobile-pwa-prod-serve`
**PR**: [#65 — `feat(api): serve Mobile PWA at /m/* from embedded dist`](https://github.com/shadsorg/DeploySentry/pull/65)

## Context

The Mobile PWA initiative shipped through Phase 6 and was archived 2026-04-29. This is the productionization follow-up: embed the built PWA into the API binary and serve it at `/m/*` so a single Go binary ships with both the dashboard and the PWA. Avoids needing a second deploy target.

## Current state

PR #65 is **open** on `feature/mobile-pwa-prod-serve`. CI status:

- ❌ **Lint** — failing
- ❌ **Lint UI** — failing
- ❌ **E2E SDK Tests** — failing
- ✅ **Lint SQL Migrations** — passing
- ⏭ Test / Build / Build Docker / Deploy to Dev — skipped (gated on lint passing)

## Tasks

- [ ] Diagnose and fix the three failing checks (Lint, Lint UI, E2E SDK Tests).
- [ ] Confirm the `/m/*` route serves the embedded dist with correct MIME types and SPA fallback.
- [ ] Verify auth flow round-trips between dashboard and PWA when both are served from the same origin.
- [ ] Re-run CI green, request review, merge.

## Done when

PR #65 is merged and the Mobile PWA is reachable at `/m/*` on a deployed API instance.
