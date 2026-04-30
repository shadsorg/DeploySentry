# Org Status & Deploy History — Phase 4: Polish

**Status**: Design
**Date**: 2026-04-29
**Spec**: [`../specs/2026-04-23-org-status-and-deploy-history-design.md`](../specs/2026-04-23-org-status-and-deploy-history-design.md)

## Context

Phases 1–3 of the Org Status & Deploy History initiative landed on main (PRs through 2026-04-23). The parent spec listed three polish items as deferred. They never landed and have been carried forward in `Current_Initiatives.md` for over a week. Splitting them out into their own plan so the parent initiative can close.

## Scope

### Task 1: ETag client caching for `/orgs/:slug/status`

The status endpoint is polled every 15s by `OrgStatusPage`. Each poll currently re-serializes the full fan-in response. Add `ETag` + `If-None-Match` so unchanged responses return `304 Not Modified` and the client reuses its cached body.

- [ ] Compute strong ETag from `(updated_at, latest_build.html_url, latest_deployment.id)` per app, hashed.
- [ ] Add middleware in `internal/orgstatus` (or wherever the handler lives) that returns `304` when `If-None-Match` matches.
- [ ] Update the React `useOrgStatus` hook to pass the previous response's `ETag` and reuse cached state on `304`.

### Task 2: CSV export from `OrgDeploymentsPage`

The page already supports cursor-paginated filtering. Add a "Export CSV" button that streams the current filter set to a CSV.

- [ ] Add `GET /orgs/:slug/deployments?format=csv` (same filter params).
- [ ] Stream rows with `text/csv` content-type; columns: `id, project, app, environment, status, mode, source, started_at, finished_at, version`.
- [ ] Frontend: button in the filters sidebar that hits the URL with `format=csv`.

### Task 3: Org-default monitoring-link templates

Today every app's monitoring links are entered manually via `MonitoringLinksEditor`. For orgs with a fleet of similar apps, this is repetitive.

- [ ] Add `monitoring_link_templates` table on the org row (or a new table keyed by org_id).
- [ ] When creating an app, auto-populate its monitoring links from the org's templates with `{{app_slug}}` / `{{env}}` substitution.
- [ ] UI: "Org defaults" section in org settings that manages the template list.

## Out of scope

Anything not in those three items. The Phase 4 work was always meant as polish, not a re-architecture.

## Done when

- `OrgStatusPage` poll loop sees a measurable rate of `304` responses (verify in network tab).
- `OrgDeploymentsPage` CSV export downloads a file matching the visible filter set.
- New apps in an org with templates set get their `monitoring_links` pre-filled on creation.
