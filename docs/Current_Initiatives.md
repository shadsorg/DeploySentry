# Current Initiatives

> Last updated: 2026-04-10

## Active (not yet complete)

| Initiative | Phase | Plan/Spec File | Notes |
|---|---|---|---|
| Documentation Updates | Implementation | [Plan](./superpowers/plans/2026-03-27-documentation-updates.md) | Evaluate endpoint listed in README but missing: SSE protocol, rule management, session consistency, percentage rollout docs. SDK READMEs missing auth header docs (5 of 7) and session consistency examples (all 7) |
## Completed (archive candidates)

These are implemented in the codebase but their plan checklists were never updated. They should be archived to `docs/archives/`.

| Initiative | Plan/Spec File | Status |
|---|---|---|
| SettingsPage Backend Wiring | [Plan](./superpowers/plans/2026-04-10-settings-page-wiring.md) | All 5 tabs wired: environments CRUD, webhooks CRUD, notification preferences, project/app settings. No mock data remains |
| SDK Production Readiness | [Plan](./superpowers/plans/2026-03-27-sdk-production-readiness.md) | All 15 tasks complete: contract fixtures, auth headers, typed eval, SSE reconnection, session consistency, unit/contract tests — all 7 SDKs |
| Feature Flag Engine Improvements | [Plan](./Feature_Flag_Engine_Improvements.md) | All 5 items complete: segments, compound rules, batch concurrency, singleflight, SSE broadcasts |
| Platform Redesign — Data Model | [Plan](./superpowers/plans/2026-03-28-platform-redesign-data-model.md) | Migrations, models, repos all in codebase |
| Platform Redesign — Remaining | [Plan](./superpowers/plans/2026-03-28-platform-redesign-remaining.md) | App/Settings repos, CLI subcommands implemented |
| Web UI Phase 1 — Navigation | [Plan](./superpowers/plans/2026-03-28-web-ui-phase1-navigation.md) | Sidebar, routing, context all implemented |
| Web UI Phase 2 — Page Redesigns | [Plan](./superpowers/plans/2026-03-28-web-ui-phase2-page-redesigns.md) | Flag/deployment/release detail pages done |
| Web UI Phase 3 — Entity Management | [Plan](./superpowers/plans/2026-03-28-web-ui-phase3-entity-management.md) | Members, API keys, settings pages exist |
| Flag Ratings & Error Tracking | [Plan](./superpowers/plans/2026-03-29-flag-ratings-and-error-tracking.md) | Full stack: migration, repo, service, handler, routes |
| Entity Management API Wiring | [Plan](./superpowers/plans/2026-03-29-entity-management-api-wiring.md) | Org/Project/App CRUD wired to router |
| Page API Wiring | [Plan](./superpowers/plans/2026-03-30-page-api-wiring.md) | Flag, deployment, release pages use real API (except SettingsPage) |
| Applications List Page | [Plan](./superpowers/plans/2026-03-30-applications-list-page.md) | Page exists with real API hook |
| Members CRUD | [Plan](./superpowers/plans/2026-03-30-members-crud.md) | Full org + project member management |
| CLI Subcommands | [Spec](./superpowers/specs/2026-03-30-cli-subcommands-and-install-design.md) | All subcommands implemented (~5,800 lines) |
| GELF Structured Logging | [Plan](./superpowers/plans/2026-03-30-gelf-structured-logging.md) | Client, transports, middleware all wired |
| Web Dashboard Integration | [Plan](./superpowers/plans/2026-03-27-web-dashboard-integration.md) | Login/register flows use real API |
| Backend Production Readiness | [Plan](./superpowers/plans/2026-03-27-backend-production-readiness.md) | Repos have error handling, pagination, proper interfaces |
| Production Readiness Design | [Spec](./superpowers/specs/2026-03-27-deploysentry-production-readiness-design.md) | Design spec, execution covered by other plans |
| Platform Redesign Design | [Spec](./superpowers/specs/2026-03-28-platform-redesign-design.md) | Design spec, execution covered by other plans |
