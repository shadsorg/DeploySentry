# Build Status Ingestion & Deploy-Create Autocomplete

**Date:** 2026-04-23
**Status:** Design
**Phase:** Design

## Overview

Two small, intertwined additions to DeploySentry:

1. **GitHub Actions build/test status on the Org Status board.** A new webhook
   endpoint accepts GitHub's native `workflow_run` events and upserts a
   record-mode `deployments` row that moves through `pending → running → completed`
   (or `failed`/`cancelled`). The Org Status grid renders a "build in progress"
   pill driven off those rows so an operator has one place to look — instead of
   tabbing between GitHub Actions and DeploySentry during a release.

2. **Artifact + version autocomplete on the Create Deployment form.** The two
   biggest friction points when manually launching a deploy from the dashboard
   today are (a) typing an exact artifact string and (b) remembering the right
   version. Both are already present in the deployments history; surfacing them
   via a combobox collapses that friction while keeping free-text entry for
   genuinely new values.

The two are specced together because they share a data source: "what versions
has this app ever shipped?" is the same question the `workflow_run` upsert path
needs when deciding whether to update an existing row or create a new one, and
the same question the autocomplete endpoints answer.

## Goals

- **One board, not two.** A build kicked off by GitHub Actions appears on
  DeploySentry's Org Status grid within seconds, with an "in progress" state
  that flips to "completed"/"failed" when the workflow concludes.
- **Less typing.** Operators launching a deploy from DeploySentry pick
  artifact and version from a recency-sorted list, but can still type a
  brand-new value for the first deploy of anything new.
- **No new scope creep.** This initiative delivers only the two additions
  above. It does **not** expand into a general-purpose CI dashboard or a
  full package/artifact registry.

## Non-Goals

- **Multi-CI support.** Only GitHub Actions in v1. Jenkins / CircleCI /
  GitLab can be added later if a customer asks; each needs its own adapter
  and a distinct webhook endpoint.
- **Per-job granularity.** One row per `workflow_run`, not one per job
  inside it. Operators who need job-level drill-down follow the `html_url`
  link back to GitHub Actions.
- **Triggering workflows from DeploySentry.** Status flows in, not out.
- **Autocompleting `strategy`, `environment`, or `artifact` at org scope.**
  Autocomplete is scoped to the selected application (and, for version, the
  selected environment). Cross-env / cross-app lookups are out of scope.
- **Persisting failed workflow logs.** DeploySentry stores the link, not the
  artifact. Failure triage happens in GitHub.

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Signal carrier | A **record-mode `deployments` row** — not a new table | The Org Status grid already reads `deployments.status` + joins `app_status.health_state`; reusing it means the "build in progress" pill is a rendering change, not a data-model change. |
| Identity key for upsert | `(application_id, environment_id, commit_sha, workflow_name)` | A single commit can run many workflows (build / test / e2e); each gets its own lane. Re-runs of the same workflow update in place. |
| Status mapping | `requested` / `in_progress` → `running`; `completed+success` → `completed`; `completed+failure` → `failed`; `completed+cancelled` → `cancelled`; `completed+timed_out` → `failed` | Matches existing `deployments.status` enum — no schema churn. Phase engine is bypassed (record mode). |
| Environment resolution | The webhook endpoint is scoped to an **app + environment by API key**, matching the existing `/status` endpoint's pattern. One key per `(app, env)` the team wants to track. | Avoids guessing environment from branch name. Symmetric with how `/applications/:id/status` already does it. |
| Authentication | API-key, scope `status:write`; optional additional HMAC verification of GitHub's `X-Hub-Signature-256` header if a secret is configured on the key | Bearer auth already works with the notification-rule pattern; HMAC is defense-in-depth. Reuses the scope-to-permission mapping fix from earlier today. |
| Artifact source | `SELECT DISTINCT artifact FROM deployments WHERE application_id = $1 ORDER BY MAX(created_at) DESC LIMIT 50` | Newest-first, capped. Application-scoped only; artifacts are rarely env-specific. |
| Version source | `SELECT DISTINCT ON (version) version, commit_sha, MAX(created_at) FROM deployments WHERE application_id = $1 AND environment_id = $2 ORDER BY version, created_at DESC LIMIT 50` | Env-scoped so the dropdown surfaces promotion candidates naturally. |
| UI widget | **Combobox** (filterable input that also accepts arbitrary text), not pure `<select>` | Handles the new-artifact / new-version case. A pure dropdown would lock out first deploys. |
| Where the combobox lives | `DeploymentCreatePage` only (for now) | Flag-rule autocomplete and strategy-name autocomplete are separate follow-ups — out of scope here. |

## API Shape

### New: `POST /api/v1/integrations/github/workflow`

Accepts GitHub's `workflow_run` webhook payload verbatim.

- **Auth:** `Authorization: Bearer <ds_...>` with `status:write` scope,
  scoped to a single `(application_id, environment_id)`. If the key has
  `deploysentry-github-secret` set in its metadata, the handler also
  verifies `X-Hub-Signature-256`.
- **Body:** opaque — we only read `action`, `workflow_run.name`,
  `workflow_run.status`, `workflow_run.conclusion`, `workflow_run.head_sha`,
  `workflow_run.head_branch`, `workflow_run.html_url`,
  `workflow_run.run_started_at`, `workflow_run.updated_at`,
  `workflow_run.created_at`. Everything else is ignored.
- **Response:** `202 Accepted` with `{"deployment_id":"…","action":"created"|"updated"|"noop"}`.
- **Idempotency:** upsert on `(application_id, environment_id, commit_sha, workflow_name)` —
  re-deliveries by GitHub are safe.

Version string is inferred as: `head_branch` + "@" + short SHA (first 7 chars
of `head_sha`), unless a `workflow_run.workflow_id`-keyed override is set on
the API key (deferred — document the hook but don't implement).

### New: `GET /api/v1/applications/:app_id/artifacts`

- **Auth:** existing `deploys:read` / session.
- **Query:** `?limit=50` (server caps at 100).
- **Response:** `{"artifacts":[{"value":"ghcr.io/crowdsoft/jobmgr:latest","last_seen_at":"…"}]}`.
- **Notes:** distinct by `value`, newest-first by max `created_at`.

### New: `GET /api/v1/applications/:app_id/versions`

- **Auth:** existing `deploys:read` / session.
- **Query:** `?environment_id=…` (optional — defaults to any env),
  `?limit=50`.
- **Response:**
  ```json
  {"versions":[{"version":"v0.1.4","commit_sha":"abc1234","last_seen_at":"…","environments":["prod","stage"]}]}
  ```
- **Notes:** DISTINCT ON `(version)`. When `environment_id` is omitted, the
  `environments` field lists every env the version has run in — useful for
  promotion-style re-deploys.

## Data-Model Impact

No schema changes required.

- The existing `deployments` table already has `mode`, `source`, `status`,
  `commit_sha`, `version`, `artifact` — all the columns the hook needs.
  `source` becomes `"github-actions"` for these rows.
- The existing `app_status` table is independent; the hook does not write
  to it. Health continues to come from the SDK's `/applications/:id/status`
  report.
- No index changes. The autocomplete queries are small (per-app, LIMIT 50)
  and are well-served by the existing `idx_deployments_application`.

## Frontend Impact

### Org Status Board (`OrgStatusPage`)

- **New pill** on each app-env cell: "Build: `workflow_name` running" when
  the most-recent deployment row with `source='github-actions'` for that
  `(app, env)` has `status IN ('pending','running')`. Flips to "Build
  failed" (error color) on `failed`/`cancelled`, disappears once a real
  app-push `app_status` supersedes the build row with a version match.
- Existing 15 s poll picks it up — no new client logic.

### Create Deployment (`DeploymentCreatePage`)

- A reusable `<Combobox>` component (in `components/forms/`) replaces the
  `<input>` elements for artifact and version.
- Fetches `/applications/:id/artifacts` on app pick and
  `/applications/:id/versions?environment_id=…` on app + env pick.
- Free-text entry is always allowed; when input doesn't match any
  suggestion, a subtle "will create new" hint shows below the field.
- Debounced filter (200 ms) against the cached list — no round-trip per
  keystroke.

## Risks & Open Questions

- **Workflow name volatility.** Renaming a GitHub Actions workflow creates a
  new "lane" on the board without retiring the old one. Mitigation: leave
  stale lanes to age out via the existing stale-pending sweep on the
  Rollouts page's list filter; port the same idea to app-env cells (1-day
  cutoff) as a follow-up.
- **SHA vs. version mismatch.** If a team tags Git differently than they
  version their artifacts, the `head_branch@sha` version string will diverge
  from what the SDK later reports. Acceptable: the build lane and the app
  lane live side by side on the board; they don't pretend to be the same row.
- **Combobox UX for 50+ entries.** Virtualization is overkill at this size;
  a simple scrollable list is fine. If an app ever ships more than 50
  distinct artifacts or versions, tighten the window to the last 14 days
  — not today.
- **Cross-origin CSP.** GitHub's webhook sends from fixed GitHub IP ranges.
  No action needed unless a customer fronts DeploySentry with a restrictive
  WAF.

## Success Criteria

1. Pushing a commit that triggers a GitHub Actions workflow results in a
   `running` pill appearing on the Org Status grid for the corresponding
   app/env within ~2 s of GitHub firing the `in_progress` webhook.
2. When the workflow concludes, the pill updates to `completed`/`failed`/
   `cancelled` without a page refresh (driven by the existing 15 s poll).
3. Opening the Create Deployment form for an app that has prior deploys
   shows a dropdown of recent artifacts on first render (no keystroke
   needed) and a dropdown of recent versions after picking an environment.
4. Typing a value that isn't in the suggestion list still submits
   successfully and creates the deploy.
5. No regressions to existing `deployments` inserts, phases, or the Rollouts
   flow — this initiative is purely additive.
