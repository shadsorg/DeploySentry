# Org Status & Deploy History — Phase 3: OrgDeploymentsPage

**Status**: Implementation
**Spec**: [2026-04-23-org-status-and-deploy-history-design.md](../specs/2026-04-23-org-status-and-deploy-history-design.md)
**Date**: 2026-04-23

## Scope

Replace the Phase-2 placeholder `OrgDeploymentsPage` with the full chronological deploy-history view on top of `GET /orgs/:slug/deployments` (Phase 1 backend).

- Sticky filters sidebar (project, cascading app, environment, status, mode, from/to datetime).
- URL-serialized filters (shareable links).
- Cursor pagination ("Load older" button).
- Row click → existing per-app deployment detail page.
- `Deployment` client type gains `mode` + `source` so the columns render.

## Verification

```
cd web && npx tsc --noEmit   # type check passes
cd web && npm run build       # production bundle builds
cd web && npm test -- --run   # existing 11 vitest cases still pass
```

Manual smoke not run in this environment — recommended browser checks before shipping:
- `/orgs/:slug/deployments` loads with no filters.
- Pick a project → app cascade populates.
- Filter status=failed or mode=record → list narrows.
- URL updates as filters change; reload preserves them.
- "Load older" paginates cleanly when more data exists.
- Row click navigates to the correct deployment detail page.

## Out of scope

Phase 4 polish: client-side ETag caching on `/status`, CSV export from this page, org-level default monitoring-link templates.
