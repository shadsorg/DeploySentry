# CLI Phase B — Fix remaining drift + lock in coverage

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox syntax.

**Goal:** Finish what Phase A started for `flags`. Audit discovered the drift is concentrated in 5 files (`analytics.go`, `releases.go`, `apps.go`, `deploy.go`, `environments.go`); the other 12 are already correct but completely untested. Fix the broken five, then add minimal smoke tests for the rest using the harness from PR #56 — every CLI subcommand ends Phase B with at least one test that pins its URL + body shape against the real API.

**Architecture:** Reuse the existing `cmd/cli/testing_helpers_test.go` harness, `stubProjectAndEnv` helper, and slug→UUID resolvers (`resolveProjectID`, `resolveEnvID`, `resolveFlagID`). No new infrastructure.

**Tech Stack:** Same as Phase A.

---

## Audit summary (from the parallel audit subagent)

| File | Severity | Fix size | New tests |
|------|----------|----------|-----------|
| `analytics.go` | Broken | medium | 6 |
| `releases.go` | Broken | large | 4 |
| `apps.go` | Mixed | small | 3 |
| `deploy.go` | Mostly OK (verify release resolver) | small | 4 |
| `environments.go` | Mostly OK (verify path) | small | 1 |
| `apikeys.go` | OK | smoke only | 3 |
| `auth.go` | OK | none (auth flow non-trivial; skip API tests) | 0 |
| `integrations.go` | OK | smoke only | 3 |
| `mcp.go` | OK (no API calls) | none | 0 |
| `orgs.go` | OK | smoke only | 3 |
| `projects.go` | OK | smoke only | 3 |
| `rolloutgroups.go` | OK | smoke only | 3 |
| `rolloutpolicy.go` | OK | smoke only | 2 |
| `rollouts.go` | OK | smoke only | 3 |
| `settings.go` | OK | smoke only | 3 |
| `strategies.go` | OK | smoke only | 2 |
| `webhooks.go` | OK | smoke only | 3 |

**Total new tests:** ~46 (on top of the existing 28).

## Out of scope for this PR

- `auth.go` interactive flows (browser callback, device code) — too complex to mock cleanly; defer.
- Re-validating EVERY CLI subcommand, only the major ones get smoke tests.
- Self-update check (separate follow-up initiative — already queued).

---

## Tasks

### Task 1: Fix `analytics.go` (Broken)

**Files:** `cmd/cli/analytics.go`, `cmd/cli/analytics_test.go` (new)

The CLI prepends `/api/v1/orgs/<org>/projects/<project>` to every analytics path. Real routes are mounted on plain `/api/v1/analytics/*`. RBAC + `org_id` from JWT claims handles scoping; project filtering is via query string `project_id=<uuid>`.

- [ ] **Step 1:** Read the existing `cmd/cli/analytics.go` to enumerate every subcommand and the URL it builds. Note that `client.get/post` takes a path; just rewrite the path constants.

- [ ] **Step 2:** Rewrite each subcommand's `run...` function to:
  - Drop the `/api/v1/orgs/%s/projects/%s` prefix.
  - Resolve `project_id` via `resolveProjectID` and append `?project_id=<uuid>` to the query string for the analytics endpoints that filter by project.
  - Map paths:
    - `analytics summary` → `GET /api/v1/analytics/summary?project_id=<uuid>&environment_id=<uuid>&time_range=<range>`
    - `analytics flags stats` → `GET /api/v1/analytics/flags/stats?project_id=...&environment_id=...&time_range=...`
    - `analytics flags usage <key>` → `GET /api/v1/analytics/flags/<key>/usage?project_id=...&environment_id=...&time_range=...`
    - `analytics deployments stats` → `GET /api/v1/analytics/deployments/stats?from=...&to=...`
    - `analytics health` (POST) → `POST /api/v1/analytics/health` (already correct — leave alone)
    - `analytics export` → `GET /api/v1/analytics/admin/export?project_id=...&environment_id=...`

- [ ] **Step 3:** Verify the API handler signatures by reading `internal/analytics/handler.go` for each route's expected query params and JSON body. Adjust the CLI accordingly. (The audit didn't drill into bodies — do it here.)

- [ ] **Step 4:** Write `cmd/cli/analytics_test.go` with 6 tests:

```go
package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnalyticsSummary_Success(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/analytics/summary", func(req recordedRequest) (int, any) {
		require.Contains(t, req.Path, "project_id=proj-uuid")
		require.Contains(t, req.Path, "environment_id=env-prod-uuid")
		require.Contains(t, req.Path, "time_range=24h")
		return 200, map[string]any{"total_evaluations": 1234}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	stdout, _, err := runCmd(t, rootCmd, "analytics", "summary", "--time-range", "24h")
	require.NoError(t, err)
	require.NotEmpty(t, stdout)
}

func TestAnalyticsFlagsStats_Success(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/analytics/flags/stats", func(req recordedRequest) (int, any) {
		require.Contains(t, req.Path, "project_id=proj-uuid")
		return 200, map[string]any{"flags": []any{}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	_, _, err := runCmd(t, rootCmd, "analytics", "flags", "stats", "--time-range", "7d")
	require.NoError(t, err)
}

func TestAnalyticsFlagsUsage_Success(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/analytics/flags/dark-mode/usage", func(req recordedRequest) (int, any) {
		require.True(t, strings.HasPrefix(req.Path, "/api/v1/analytics/flags/dark-mode/usage"))
		return 200, map[string]any{"evaluations": 42}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	_, _, err := runCmd(t, rootCmd, "analytics", "flags", "usage", "dark-mode", "--time-range", "24h")
	require.NoError(t, err)
}

func TestAnalyticsDeploymentsStats_Success(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/analytics/deployments/stats", func(recordedRequest) (int, any) {
		return 200, map[string]any{"total": 10}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	_, _, err := runCmd(t, rootCmd, "analytics", "deployments", "stats")
	require.NoError(t, err)
}

func TestAnalyticsHealth_Success(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("POST", "/api/v1/analytics/health", func(recordedRequest) (int, any) {
		return 200, map[string]any{"healthy": true}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	_, _, err := runCmd(t, rootCmd, "analytics", "health")
	require.NoError(t, err)
}

func TestAnalyticsExport_Success(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/analytics/admin/export", func(recordedRequest) (int, any) {
		return 200, map[string]any{"exported": true}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	_, _, err := runCmd(t, rootCmd, "analytics", "export")
	require.NoError(t, err)
}
```

If a test fails because of CLI flag bleed across tests, add the same `resetXxxFlags(t)` pattern used in flags_test.go.

- [ ] **Step 5:** `go test ./cmd/cli/ -run TestAnalytics -v -count=1` → 6 pass.

- [ ] **Step 6:** `go vet ./cmd/cli/... && go build ./...` → clean.

- [ ] **Step 7:** Commit:

```bash
git add cmd/cli/analytics.go cmd/cli/analytics_test.go
git commit -m "fix(cli): analytics — strip org/project URL prefix, switch to query params"
```

---

### Task 2: Fix `releases.go` (Broken)

**Files:** `cmd/cli/releases.go`, `cmd/cli/releases_test.go` (new)

The real release routes are mounted under `/applications/:app_id/releases`. The CLI uses an `/orgs/<org>/projects/<project>/releases` prefix that doesn't exist.

- [ ] **Step 1:** Read `internal/releases/handler.go` to enumerate the actual routes and bodies. Likely:
  - `POST /api/v1/applications/:app_id/releases` (create)
  - `GET /api/v1/applications/:app_id/releases` (list)
  - `GET /api/v1/releases/:id` (get)
  - `POST /api/v1/releases/:id/promote` (promote)
  - Verify exact paths during implementation.

- [ ] **Step 2:** Add an app-slug → UUID resolver helper to `cmd/cli/resolve.go` (or reuse if `resolveProjectID` style already covers it):

```go
// resolveAppID resolves an app slug to its UUID by GETting
// /api/v1/orgs/:org/projects/:project/apps/:app.
func resolveAppID(client *apiClient, org, projectSlug, appSlug string) (string, error) {
	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/apps/%s", org, projectSlug, appSlug)
	resp, err := client.get(path)
	if err != nil {
		return "", fmt.Errorf("resolve app %q: %w", appSlug, err)
	}
	id, ok := resp["id"].(string)
	if !ok || id == "" {
		return "", fmt.Errorf("resolve app %q: response missing id", appSlug)
	}
	return id, nil
}
```

- [ ] **Step 3:** Rewrite every release subcommand to:
  - Resolve `--app <slug>` → `appID` via `resolveAppID`.
  - Use the correct app-scoped routes for create/list (and the bare `/releases/:id` routes for get/promote).
  - Match body shapes to the real `createReleaseRequest` (likely needs `version`, `description?`, `app_id` is implicit in URL).

- [ ] **Step 4:** Write `cmd/cli/releases_test.go` with 4 tests covering create/list/get/promote. Each test stubs the app resolver, the release route, and asserts URL + body.

- [ ] **Step 5:** Run, vet, build, commit:

```bash
go test ./cmd/cli/ -run TestReleases -v -count=1
go vet ./cmd/cli/... && go build ./...
git add cmd/cli/releases.go cmd/cli/releases_test.go cmd/cli/resolve.go
git commit -m "fix(cli): releases — switch to app-scoped routes + add app slug resolver"
```

---

### Task 3: Fix `apps.go` (Mixed)

**Files:** `cmd/cli/apps.go`, `cmd/cli/apps_test.go` (new)

URL paths look mostly right. Verify against `internal/entities/handler.go` and reconcile any field-name drift.

- [ ] **Step 1:** Diff each apps.go URL against `internal/entities/handler.go`'s app-scoped routes. Adjust CLI as needed.

- [ ] **Step 2:** Write 3 tests: create, list (project-scoped), list `--all` (org-wide).

- [ ] **Step 3:** Run + commit:

```bash
git add cmd/cli/apps.go cmd/cli/apps_test.go
git commit -m "fix(cli): apps — reconcile URLs with entities handler + add tests"
```

---

### Task 4: Verify `deploy.go` + add tests

**Files:** `cmd/cli/deploy.go` (small fixes if needed), `cmd/cli/deploy_test.go` (new, 4 tests)

- [ ] **Step 1:** Read `internal/deploy/handler.go` and confirm every CLI path matches.

- [ ] **Step 2:** If the CLI sends release version (string) but handler expects UUID, add a release-version → UUID resolver or fix the body shape.

- [ ] **Step 3:** Write 4 tests: create, status, promote, list.

- [ ] **Step 4:** Commit:

```bash
git add cmd/cli/deploy.go cmd/cli/deploy_test.go
git commit -m "test(cli): cover deploy commands + verify URLs"
```

---

### Task 5: `environments.go` smoke

**Files:** `cmd/cli/environments.go` (verify only), `cmd/cli/environments_test.go` (new, 1 test)

- [ ] One smoke test for `environments list` against the real path. Commit.

---

### Task 6: Smoke tests for the 12 OK files

**Files:** Create one test file per OK CLI file with 1-3 happy-path tests each that pin the URL + body shape. ~30 tests total.

- [ ] `cmd/cli/apikeys_test.go` — list, create, revoke (3 tests)
- [ ] `cmd/cli/integrations_test.go` — deploy create, list, delete (3 tests)
- [ ] `cmd/cli/orgs_test.go` — create, list, set-active (3 tests)
- [ ] `cmd/cli/projects_test.go` — create, list, get (3 tests)
- [ ] `cmd/cli/rolloutgroups_test.go` — list, create, attach (3 tests)
- [ ] `cmd/cli/rolloutpolicy_test.go` — get, set (2 tests)
- [ ] `cmd/cli/rollouts_test.go` — list, get, pause (3 tests)
- [ ] `cmd/cli/settings_test.go` — list, set, delete (3 tests)
- [ ] `cmd/cli/strategies_test.go` — list, apply (2 tests)
- [ ] `cmd/cli/webhooks_test.go` — create, list, delete (3 tests)

Skip `auth_test.go` and `mcp_test.go` (no straightforward API surface to mock).

For each: write happy-path test using `runCmd(t, rootCmd, "<cmd>", ...)`, assert URL + body, commit one file at a time:

```bash
git add cmd/cli/<file>_test.go
git commit -m "test(cli): add smoke coverage for <file> commands"
```

---

### Task 7: Final verify + push

- [ ] `go vet ./cmd/cli/... && go test ./cmd/cli/ -v -count=1 && go build ./...` — expect ~74 tests pass total (28 from Phase A + ~46 new).
- [ ] Update `docs/Current_Initiatives.md`:
  - Bump `Last updated:` to `2026-04-27`.
  - Update the existing CLI Flag Flow Fix row's notes to mark Phase A complete (PR #56) and Phase B in flight on `feature/cli-phase-b` with a brief summary.
- [ ] Push: `git push -u origin fix/cli-phase-b`.

---

## Success criteria

- ~74 tests passing in `cmd/cli/`.
- Every CLI subcommand (except `auth login` interactive flows and `mcp serve`) exercised by at least one test.
- `analytics` and `releases` URLs match the real API.
- `apps` and `deploy` verified end-to-end.
