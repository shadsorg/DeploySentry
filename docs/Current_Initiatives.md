# Current Initiatives

> Last updated: 2026-04-10

## Active (not yet complete)

| Initiative | Phase | Plan/Spec File | Notes |
|---|---|---|---|
| Feature Flag Engine Improvements | Implementation | [Plan](./Feature_Flag_Engine_Improvements.md) | Segments, compound rules, SSE overhaul, singleflight, batch concurrency — all implemented on main |
| SDK Production Readiness | Implementation | [Plan](./superpowers/plans/2026-03-27-sdk-production-readiness.md) | SDKs implemented but no contract tests, auth header fixes unverified |
| Documentation Updates | Implementation | [Plan](./superpowers/plans/2026-03-27-documentation-updates.md) | SDK READMEs exist; API docs, SSE protocol docs, evaluation schemas still needed |
| SettingsPage Backend Wiring | Implementation | [Spec](./superpowers/specs/2026-03-30-page-api-wiring-design.md) | Webhooks & notifications still use mock data |

## Completed (archive candidates)

These are implemented in the codebase but their plan checklists were never updated. They should be archived to `docs/archives/`.

| Initiative | Plan/Spec File | Status |
|---|---|---|
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
