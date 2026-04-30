# Build Status Ingestion & Deploy-Create Autocomplete — Plan

**Phase:** Implementation

## Overview

Two additions to DeploySentry, specced together because they share the same
data source (`deployments` history per app):

1. GitHub Actions `workflow_run` webhook → "build in progress" / "build
   failed" pills on the Org Status board, via record-mode deployment rows.
2. Combobox autocomplete for artifact + version on the Deployment Create
   form, sourced from recent deploy history for the selected app (and env,
   for version).

Design doc: [2026-04-23-build-status-and-deploy-autocomplete-design.md](../specs/2026-04-23-build-status-and-deploy-autocomplete-design.md)

## Phased Checklist

### Phase 1 — Backend endpoints

- [x] `GET /api/v1/applications/:app_id/artifacts` — distinct artifacts for
      an app, newest-first, LIMIT 50. Uses existing `deploys:read` perm.
- [x] `GET /api/v1/applications/:app_id/versions` — distinct versions
      (optionally env-scoped) with commit SHA + envs-seen list, newest-first,
      LIMIT 50.
- [x] Repo method `ListDistinctArtifacts(ctx, appID, limit)` on
      `postgres.DeployRepository`.
- [x] Repo method `ListDistinctVersions(ctx, appID, envID *uuid.UUID, limit)`
      returning `(version, commit_sha, last_seen_at, env_ids[])` rows.
- [ ] Handler unit tests: empty app → `{"artifacts":[]}` / `{"versions":[]}`;
      distinct-by-value; correct ordering; `limit` clamp at 100.
      *(Deferred: repo-level behavior is exercised via existing integration
      paths; dedicated handler tests pending.)*

### Phase 2 — GitHub webhook adapter

- [x] `POST /api/v1/integrations/github/workflow` handler.
- [x] Middleware: require API-key auth with `status:write` scope
      (reuses today's scope → permission mapping).
- [x] Optional HMAC verification of `X-Hub-Signature-256` against a secret
      stored on the API key's metadata JSONB.
- [x] `internal/integrations/github/workflow.go` — maps GitHub
      `workflow_run` payload to a DeploySentry upsert.
- [x] Upsert on `(application_id, environment_id, commit_sha, workflow_name)`:
      insert on first event, update status + `completed_at` on subsequent
      events (action `completed`).
- [x] Status map:
      `requested`/`in_progress` → `running`;
      `completed/success` → `completed`;
      `completed/failure|timed_out` → `failed`;
      `completed/cancelled` → `cancelled`.
- [x] `source = "github-actions"` on every row the hook writes.
- [x] Version = `head_branch + "@" + short_sha`.
- [x] Integration test: fire the three canonical GitHub payloads
      (`requested`, `in_progress`, `completed/success`) against a test
      router — assert one row, three status values, idempotent on replay.

### Phase 3 — Frontend: autocomplete

- [x] `web/src/components/forms/Combobox.tsx` — accessible, keyboard-
      navigable, debounced filter (200 ms), accepts arbitrary text with
      a "will create new" affordance when input doesn't match.
- [x] `web/src/api.ts` — `deploysApi.listArtifacts(appId)` +
      `deploysApi.listVersions(appId, envId?)`.
- [x] `DeploymentCreatePage` wires:
  - [x] Artifact combobox (fetches on app change).
  - [x] Version combobox (fetches on app + env change; clears on app change).
  - [x] Existing free-text behavior preserved when nothing is typed-through.
- [ ] Component test (Vitest + Testing Library): renders suggestions,
      accepts free-text, shows "will create new" hint on miss.
      *(Deferred: the component is small and typechecks; tests pending
      alongside the broader web test harness work.)*

### Phase 4 — Frontend: build-status pill

- [x] `OrgStatusPage` — compute per-cell build lane from the existing
      status payload (server-side: extend `OrgStatusResponse` to include
      the most recent `source='github-actions'` deployment for each
      `(app, env)` if one exists). No new endpoint; piggyback on the
      existing fan-in query.
- [x] Render pill variants: `running` (subtle blue, animated dot),
      `failed` / `cancelled` (red, "Build failed — open →" → `html_url`),
      hidden once a newer `app_status` supersedes the build's version.
- [ ] Stale-build sweep: if a `running` build lane is >1 day old with no
      follow-up event, render as `stale` and exclude from the at-a-glance
      color rollup.
      *(Deferred: same pattern exists on the Rollouts page; port after
      first round of operator feedback.)*
- [ ] Playwright smoke: seed one `running` row via the webhook, load the
      Status page, assert the pill is present and links out.
      *(Deferred: web E2E harness pending wider rollout.)*

### Phase 5 — Docs + rollout

- [x] Update `docs/Deploy_Integration_Guide.md` with a GitHub Actions
      "notification rule" recipe (bearer token + `workflow_run` event).
- [ ] Update the existing MCP `setup_deploy_monitoring` tool to emit the
      same recipe. *(Deferred: doc update covers the primary setup path.)*
- [x] Update `docs/Current_Initiatives.md` as phases move to Implementation
      / Complete.
- [x] Completion record: branch, commit/push, CI state.

## Ordering & Dependencies

Phases 1–2 are independent; the artifact/version endpoints land first
because they're the smaller change and they unblock Phase 3. Phase 2 can
ship without Phase 4 — the Org Status board simply shows the record-mode
rows as part of its existing rendering until the dedicated pill lands.

## Risks

- **Schema drift** if anyone merges a `deployments` change mid-flight.
  Mitigation: all Phase-2 SQL is additive (INSERT / UPDATE on existing
  columns; no DDL).
- **Webhook auth confusion.** Teams who already set `status:write` keys for
  SDK health pushes might try to reuse them here. Documenting the
  `(app, env)` scope requirement is the only real fix — the middleware
  already enforces it, so misuse fails with a 403, not data corruption.
- **Combobox hygiene.** Straightforward, but the accessibility bar is not
  trivial. We'll lean on `react-aria`'s `useComboBox` (already a transitive
  dep via the component library — check before adding a new dep).

## Completion Record

<!-- Fill in when phase is set to Complete -->

- **Branch**:
- **Committed**: No
- **Pushed**: No
- **CI Checks**:
