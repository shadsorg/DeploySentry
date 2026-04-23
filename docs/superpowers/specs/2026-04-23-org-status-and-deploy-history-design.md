# Org-Level Application Status & Deploy History

**Date:** 2026-04-23
**Status:** Design

## Overview

DeploySentry now has the primitives to know, per `(application, environment)`: what version is running, whether it's healthy, and when it deployed (via the just-shipped Agentless Deploy Reporting initiative). What it doesn't yet have is an **operator-level fan-in view** — one page that answers "across my entire org, what's the state of everything, grouped by project, right now?" and a sibling view that answers "what deployed anywhere in my org recently?"

This design adds both, plus a small but load-bearing extension: per-application **monitoring links** so the dashboard can one-click-jump to the team's real observability / paging / logs tools.

Three additions:

1. **`monitoring_links` on `applications`** — a small JSONB array of `{label, url, icon?}` entries, editable from the existing app-settings surface.
2. **`GET /orgs/:orgSlug/status`** — single-call fan-in returning every project → every application → every environment, with current version + health for each cell. Powers the new Status page.
3. **`GET /orgs/:orgSlug/deployments`** — org-scoped chronological deploy-history list with the usual filters. Powers the new Deploy History page.

Plus two web pages and two new org-nav items.

## Goals

- An operator opening DeploySentry for their org gets a **compact, at-a-glance heatmap** of every app × env in two clicks.
- Every app in that view offers a direct link to relevant monitoring / paging / logs for that service — without the operator having to remember URLs.
- A single combined deploy history page answers "what shipped recently?" across the whole org, filterable by project / app / env / status / mode / date.
- Existing per-project and per-app pages are not disturbed — the new pages are additive.

## Non-Goals

- **Real-time streaming** of status changes. Poll-with-ETag is enough for v1; SSE is future work if latency demands it.
- **Proxying or previewing monitoring-link targets.** Links are plain external `<a>` tags; content is the user's responsibility.
- **Cross-org views.** Scope stops at a single org's boundary — multi-org aggregation is a separate product concern.
- **Bulk-edit of monitoring links.** One app at a time; bulk-import is a future polish.
- **Status-page replacement for the existing org landing.** Navigation stays where it is; Status is a new nav item, not a new home.

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Where it lives in nav | Two new org-level nav items: **Status** and **Deploy History** | Operators requested sibling nav items, not a new landing. Keeps the existing home intact. |
| Row grain on Status page | One row per application, with a per-env chip | Matches the request for compactness. Env chips stay readable even with 5–7 envs. |
| Environment coverage | Every env the app is mapped to, always shown | Users explicitly asked to keep unseen envs visible as faded indicators so the coverage story is obvious. |
| Never-seen state | Render the chip in a muted color (`slate-300`) + "not deployed" tooltip | Distinct from "unhealthy" (red) and "unknown / stale" (amber). |
| Project grouping | Collapsible project-level section bar with project name + app count + aggregate health pill | Gives operators fast "which project is on fire?" triage. |
| Monitoring links storage | `JSONB` column on `applications`, max 10 per app | Small data, read-alongside-app, no separate table needed. |
| Icon library | Curated SVG set: GitHub, Datadog, New Relic, Grafana, PagerDuty, Sentry, Slack, Loki, Prometheus, CloudWatch + **Custom** (URL favicon via `<img>` with graceful fallback) | Operators pick from a recognizable list; custom URL covers the long tail. Adds ~10 KB to the bundle. |
| Deploy history source | `deployments` table only (not `app_status_history` samples) | History = "deploys that happened." Samples are a different concept. |
| Deploy history pagination | Cursor-based on `created_at` + `id` tiebreaker | Survives new deploys landing mid-scroll. Offset-based pagination double-shows rows in this scenario. |
| Fan-in query shape | One SQL round-trip per `/status` call, backed by a LATERAL subquery for "latest deployment per (app, env)" | O(apps × envs) rows, acceptable for the practical ceiling (10s of projects × 10s of apps × ≤10 envs). Larger orgs get ETag-based 304s on poll. |
| Refresh cadence | UI polls `/status` every 15s with `If-None-Match` | Matches existing dashboard rhythms. Configurable per user later. |

## Data Model

### Migration 057 — `applications.monitoring_links`

```sql
ALTER TABLE applications
    ADD COLUMN monitoring_links JSONB NOT NULL DEFAULT '[]'::jsonb;
-- No index needed; filtered only inside the owning row.
```

Shape:

```jsonc
[
  { "label": "Datadog APM",       "url": "https://app.datadoghq.com/…", "icon": "datadog"   },
  { "label": "GitHub",            "url": "https://github.com/acme/api", "icon": "github"    },
  { "label": "Runbook",           "url": "https://notion.so/…",         "icon": "custom"    }
]
```

Server-side validation (new service method):
- ≤10 entries.
- `label` required, ≤60 chars, trimmed.
- `url` required, must parse as `http(s)://…`.
- `icon` optional; if present must be in the allow-list (`github | datadog | newrelic | grafana | pagerduty | sentry | slack | loki | prometheus | cloudwatch | custom`).

## API Surface

### 1. `PATCH /api/v1/applications/:id/monitoring-links`

Replace-only (no merge semantics). RBAC: `PermAPIKeyManage` or `PermProjectManage` (whichever covers app-edit today; reuse rather than invent).

```jsonc
// Request
{ "monitoring_links": [ { "label": "…", "url": "…", "icon": "…" } ] }

// Response: the updated application row.
```

### 2. `GET /api/v1/orgs/:orgSlug/status`

Returns the org-wide fan-in. Supports `If-None-Match`; the handler computes an ETag from `MAX(updated_at)` across the involved tables + `MAX(reported_at)` from `app_status`. 304 on match.

```jsonc
{
  "org":       { "id", "slug", "name" },
  "generated_at": "2026-04-23T…",
  "projects": [
    {
      "project": { "id", "slug", "name" },
      "aggregate_health": "healthy | degraded | unhealthy | unknown",
      "applications": [
        {
          "application": {
            "id", "slug", "name",
            "monitoring_links": [ … ]
          },
          "environments": [
            {
              "environment": { "id", "slug", "name" },
              "current_deployment": {
                "id", "version", "commit_sha",
                "status", "mode", "source",
                "completed_at"
              } | null,
              "health": {
                "state":           "healthy | degraded | unhealthy | unknown",
                "staleness":       "fresh | stale | missing",
                "source":          "app-push | agent | observability | unknown",
                "last_reported_at": "…"
              }
            }
          ]
        }
      ]
    }
  ]
}
```

Visibility honors the existing grants/groups filter (same as the per-app pages).

### 3. `GET /api/v1/orgs/:orgSlug/deployments`

Org-scoped chronological list.

| Param | Type | Default | Notes |
|---|---|---|---|
| `project_id` | uuid | – | Narrow to one project |
| `application_id` | uuid | – | Narrow to one app |
| `environment_id` | uuid | – | Narrow to one env |
| `status` | string | – | Filter on `DeployStatus` |
| `mode` | string | – | `orchestrate` or `record` |
| `from`, `to` | RFC3339 | – | Date-range filter on `created_at` |
| `cursor` | opaque | – | Returned by previous call for "older than" |
| `limit` | int | 50 | Capped at 200 |

Response:

```jsonc
{
  "deployments": [
    {
      "id", "application_id", "environment_id", "project_id",
      "version", "commit_sha", "status", "mode", "source",
      "traffic_percent", "started_at", "completed_at", "created_at",
      "created_by",
      "application": { "slug", "name" },
      "environment": { "slug", "name" },
      "project":     { "slug", "name" }
    }
  ],
  "next_cursor": "base64(created_at + id)" | null
}
```

Inner joins for project / app / env slugs + names so the table renders without extra round-trips. Visibility honors the same filter as above.

## UI Surface (web)

Routes, new components, and nav items — spec only; a subsequent plan file covers pixel-level layout.

### Routes

- `/orgs/:orgSlug/status` — `OrgStatusPage.tsx`.
- `/orgs/:orgSlug/deployments` — `OrgDeploymentsPage.tsx`.
- Existing app-settings page gains a new **Monitoring links** section.

### `OrgStatusPage.tsx`

- **Top bar:** org name + "Last updated X seconds ago" + manual refresh button. Auto-poll every 15s.
- **Global status strip:** counts per state (healthy / degraded / unhealthy / unknown) across all apps × envs.
- **One collapsible section per project** with header bar: project name, app count, aggregate health pill. Default expanded; state persisted per-user in `localStorage`.
- **Compact app rows** inside each section:
  - App name (click → app detail page).
  - Current version (short; hover → commit SHA + `deployed Xh ago`).
  - **Env chips strip** — one chip per mapped env, horizontal. Chip colors: healthy=green, degraded=amber, unhealthy=red, unknown=slate-400, never-seen=slate-300 (faded) with a "never deployed" tooltip. Chip label = env slug abbreviation (e.g. `prod` / `stg`).
  - **Monitoring links strip** — icon buttons for each configured link. `rel="noopener noreferrer" target="_blank"`. Hover shows the label. Custom-icon entries render a favicon from the URL's host with a text-fallback.
  - **History link** — small "History" button → navigates to `OrgDeploymentsPage` pre-filtered to that app (all envs).
- **Empty state** — "No applications yet. [Create one]." centered.

### `OrgDeploymentsPage.tsx`

- **Filters sidebar** (sticky left): project, application (cascading), environment (cascading), status, mode, date range. Filters serialize into the URL so links are shareable.
- **Table** with columns: **When** (relative + tooltip absolute), **Project → App → Env** (combined hierarchy cell), **Version** (+ commit), **Status** pill, **Mode** badge (orchestrate|record), **Source** (agent | app-push | railway-webhook | …), **Actor**.
- Row click navigates to the deployment detail page.
- **"Load older"** button at the bottom uses the cursor. No infinite-scroll — explicit "load" avoids surprise network use.
- **Export CSV** button (Phase 4 polish).

### Monitoring-links editor

- Lives on `ApplicationSettingsPage.tsx` (or equivalent). Repeating-row form:
  - Icon dropdown (curated set + "Custom").
  - Label input.
  - URL input with live validation.
  - Add / Remove row buttons; drag-handle to reorder.
- "Save" posts the complete array via `PATCH /applications/:id/monitoring-links`. Optimistic UI update with rollback on error.
- Character + count limits shown inline.

### Nav updates

New org-level entries alongside existing ones (Projects, Members, etc.):

- **Status** — uses a dashboard/grid icon.
- **Deploy History** — uses a clock/history icon.

Placement: between the existing "Overview" and "Projects" entries, in that order, so the new views are prominent without displacing established navigation.

## Performance Notes

- `/orgs/:orgSlug/status` query shape (pseudocode):
  ```sql
  SELECT p.*, a.*, e.*,
         latest_deploy.*,
         s.*
  FROM projects p
  JOIN applications a USING (project_id)
  JOIN application_environments ae ON ae.application_id = a.id
  JOIN environments e ON e.id = ae.environment_id
  LEFT JOIN LATERAL (
      SELECT * FROM deployments d
      WHERE d.application_id = a.id
        AND d.environment_id = e.id
      ORDER BY d.created_at DESC LIMIT 1
  ) latest_deploy ON TRUE
  LEFT JOIN app_status s
      ON s.application_id = a.id AND s.environment_id = e.id
  WHERE p.org_id = $1 AND <visibility filter>;
  ```
  Single query, O(apps × envs) rows. Existing indexes (`deployments_app_env_created_idx` from Migration 054, `app_status` primary key) cover it.
- ETag: hash of `(max(deployments.updated_at), max(app_status.reported_at), max(applications.updated_at))` across the visible set. Cheap to compute; invalidates on any material change.
- UI auto-poll = 15s. Operators who leave the tab open don't thrash the DB.

## Rollout Phases

1. **Phase 1 — backend endpoints.** Migration 057, `monitoring_links` CRUD, `/orgs/:slug/status`, `/orgs/:slug/deployments`. Unit + integration tests. Shippable standalone (CLI / curl usable) with no UI.
2. **Phase 2 — OrgStatusPage + monitoring-links editor + nav.** Consumes Phase 1. The UI half of the core ask.
3. **Phase 3 — OrgDeploymentsPage.** Consumes the org deployments endpoint. Separate PR so status + history land independently.
4. **Phase 4 (polish, optional).** ETag-based 304 short-circuit on `/status`, CSV export on history, org-level default monitoring-link templates mergeable per app, sticky filters that persist across page visits.

## Open Questions (defer-and-return)

- **Which existing app-edit endpoint should `monitoring_links` piggyback on?** If there's already a `PATCH /applications/:id` covering name/slug/description, extend it to accept `monitoring_links`; otherwise add the narrow endpoint above. Will confirm during Phase 1 implementation.
- **Should "Status" become the org landing page eventually?** Out of scope for this ship, but worth revisiting after a few weeks of operator usage.
- **Monitoring-link URL allow-list?** Currently any `http(s)://`. Enterprise customers may want to restrict to their own domains. Defer until a real ask surfaces.

## Related Designs

- `docs/archives/2026-04-23-agentless-deploy-reporting-design.md` — completed initiative that delivers the `/current-state` + `/status` primitives this page layers on top of.
- `docs/Deploy_Integration_Guide.md` — operator-facing guide that will gain a "Viewing org-wide status" section once Phase 2 ships.

## Status & Next Actions

- **Spec open for review.** Merge as Design once approved.
- **First plan to carve:** Phase 1 — `docs/superpowers/plans/2026-04-23-org-status-phase1-backend.md` (migration + two read endpoints + monitoring-links CRUD). Small, shippable, unblocks everything downstream.
