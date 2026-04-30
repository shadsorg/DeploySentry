# Org Status & Deploy History — Phase 2: OrgStatusPage + monitoring-links editor + nav

**Status**: Implementation
**Spec**: [2026-04-23-org-status-and-deploy-history-design.md](../specs/2026-04-23-org-status-and-deploy-history-design.md)
**Date**: 2026-04-23

## Scope

Frontend layer on top of Phase 1's backend endpoints.

1. `OrgStatusPage.tsx` — project-grouped compact heatmap consuming `GET /orgs/:slug/status`. 15s auto-poll.
2. `MonitoringLinksEditor.tsx` — reusable editor for app-level monitoring-links, wired into the app-settings "General" tab.
3. Sidebar: add **Status** and **Deploy History** org-level nav items.
4. Route registration in `App.tsx`. Deploy History route ships as a Phase-2 placeholder page so the deep-link from per-app History buttons on the Status page doesn't 404; the filterable view lands in Phase 3.
5. Types: `HealthState`, `HealthStaleness`, `MonitoringLink`, and the full `OrgStatus*` tree on the client.
6. CSS: new heatmap section in `globals.css` — health pills, env chips (with faded `never-deployed` + stale/missing outlines), monitoring-link icon buttons, project section bars.

## Verification

```
cd web && npx tsc --noEmit   # type check passes
cd web && npm run build       # production bundle builds
cd web && npm test -- --run   # existing 11 vitest cases still pass
go build ./...                # server build unaffected
```

**Manual browser testing** was not run in this session (no GUI environment available). Recommended smoke tests before shipping to users:

- Log into an org with at least one project + app that has pushed a `/status` sample → confirm the Status page renders with colored env chips + last-deployed time.
- Confirm collapsible project bar persists across reload (localStorage).
- Confirm a never-deployed env renders as a faded dashed chip with the "never deployed" tooltip.
- Confirm monitoring-link icons render and open in a new tab.
- Confirm "History →" link navigates to the (placeholder) `/orgs/:slug/deployments?application_id=…` page.
- App Settings → "Monitoring links" section: add a row, pick Datadog, paste a URL, Save → reload → row still there. Attempt a non-http URL → server rejects with inline error.

## Out of scope

- Phase 3: filterable chronological `OrgDeploymentsPage` table.
- Phase 4 polish: ETag + `If-None-Match` on the client, CSV export, org-default monitoring-link templates.
