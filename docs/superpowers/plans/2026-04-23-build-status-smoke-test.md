# Build Status & Deploy-Create Autocomplete — Smoke Test Plan

**Date:** 2026-04-23 (for execution 2026-04-24)

## Prereqs (one-time)

- API key scoped to *one* app + *one* env, scope `status:write`. Copy the
  `ds_…` token. Note `APP_ID` and `ENV_ID`.
- `SHA=$(git rev-parse HEAD)` available in the test shell.
- `DS_API_KEY` exported for all curls.

## Part A — Autocomplete (no GitHub needed)

1. **Seed a row** so there's something to suggest:
   ```bash
   deploysentry deploys record \
     --app <slug> --env dev --version v0.0.0-smoke \
     --artifact ghcr.io/<you>/smoke:v0.0.0-smoke --status completed
   ```
2. **Hit the endpoints directly:**
   ```bash
   curl -H "Authorization: Bearer $DS_API_KEY" \
     http://localhost:8080/api/v1/applications/$APP_ID/artifacts | jq

   curl -H "Authorization: Bearer $DS_API_KEY" \
     "http://localhost:8080/api/v1/applications/$APP_ID/versions?environment_id=$ENV_ID" | jq
   ```
   Expect: each list contains the seeded value first, `last_seen_at` within
   a few seconds of now.
3. **UI:** open the app's Deployments tab → **+ New Deployment**. Artifact
   dropdown shows seeded value on focus. Pick env → version dropdown
   populates. Type a novel string → "Will create new: …" hint appears.
   Close without submitting.

## Part B — GitHub workflow_run (synthetic, no real repo)

Fire the canonical payloads in order. Watch Org Status update between each.

1. **`in_progress`** — should create a `running` row:
   ```bash
   curl -X POST -H "Authorization: Bearer $DS_API_KEY" \
     -H "Content-Type: application/json" \
     -H "X-GitHub-Event: workflow_run" \
     "http://localhost:8080/api/v1/applications/$APP_ID/integrations/github/workflow" \
     -d "{\"action\":\"in_progress\",\"workflow_run\":{\"name\":\"CI\",\"status\":\"in_progress\",\"head_sha\":\"$SHA\",\"head_branch\":\"main\",\"html_url\":\"https://github.com/you/repo/actions/runs/1\"},\"repository\":{\"full_name\":\"you/repo\"}}"
   ```
   Expect `202 {"action":"created", …}`. Org Status page within 15 s shows
   `⏱ CI` pill next to the env chip.

2. **`completed`/`success`** — same row flips to completed:
   ```bash
   curl -X POST -H "Authorization: Bearer $DS_API_KEY" \
     -H "Content-Type: application/json" \
     -H "X-GitHub-Event: workflow_run" \
     "http://localhost:8080/api/v1/applications/$APP_ID/integrations/github/workflow" \
     -d "{\"action\":\"completed\",\"workflow_run\":{\"name\":\"CI\",\"status\":\"completed\",\"conclusion\":\"success\",\"head_sha\":\"$SHA\",\"head_branch\":\"main\",\"html_url\":\"https://github.com/you/repo/actions/runs/1\"},\"repository\":{\"full_name\":\"you/repo\"}}"
   ```
   Expect `202 {"action":"updated", …}`. Pill flips to `✓ CI`. Click
   opens html_url in a new tab.

3. **Parallel lane** — same commit, different workflow name:
   ```bash
   # same shape, change "name":"CI" → "name":"e2e"
   ```
   Expect a second `202 {"action":"created", …}` — two pills side by side
   on the same cell.

4. **Failure lane** — new commit SHA, `conclusion: "failure"`:
   ```bash
   # use a fake SHA, set "action":"completed","conclusion":"failure"
   ```
   Expect a `✗ CI` pill (error-colored) linking to html_url.

## Part C — DB sanity (30 s)

```bash
psql "$DS_DATABASE_DSN" -c "
SET search_path TO deploy;
SELECT substring(id::text, 1, 8) AS id,
       status, source, version, commit_sha, mode
FROM deployments
WHERE source LIKE 'github-actions%'
ORDER BY created_at DESC LIMIT 10;"
```

Confirm `mode=record`, `source=github-actions:CI`/`:e2e`, status
transitions visible by row.

## Part D — Negative cases

- Key scoped to a *different* app → POST → **403** "api key is not scoped
  to this application".
- Omit `head_sha` → **400** "workflow_run payload missing head_sha".
- `X-GitHub-Event: ping` + empty body → **200** `{"status":"pong"}`, no
  DB row created.

## Idempotency spot-check

- Fire B.1 twice. Second call should return `{"action":"updated"}`, not
  create a new row.

## Skipped for now (tracked in plan)

- Build pill disappears on stale (>1d running without update). Not
  implemented.
- Playwright smoke + Vitest for Combobox. Deferred.

---

If all of A1–A3, B1–B4, C, D, and the idempotency check pass, the
integration is ready to point at a real GitHub repo.
