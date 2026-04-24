#!/usr/bin/env bash
#
# smoke-github-workflow.sh — end-to-end verification of the GitHub Actions
# workflow_run integration plus the artifact/version autocomplete endpoints.
#
# Mirrors the steps in
# docs/superpowers/plans/2026-04-23-build-status-smoke-test.md:
#
#   Part A — /applications/:id/artifacts and /versions (autocomplete)
#   Part B — POST /applications/:app_id/integrations/github/workflow
#   Part C — DB sanity check (requires psql + DS_DATABASE_DSN)
#   Part D — negative cases (app-scope mismatch, missing head_sha, ping)
#   Part E — idempotency: replay B.1 and assert action=="updated"
#
# The script is intentionally self-contained; it only needs curl + jq and
# (for Part C) psql. It does NOT hit the web UI — UI wiring for the build
# pill is currently reverted and should be re-exercised manually once
# restored.
#
# Usage:
#
#   DS_API_KEY=ds_xxx \
#   APP_ID=<uuid> \
#   ENV_ID=<uuid> \
#   OTHER_APP_ID=<uuid>            \   # required only for Part D.1
#   DS_DATABASE_DSN=postgres://…   \   # optional; enables Part C
#   DS_BASE_URL=http://localhost:8080  \   # optional; default shown
#   bash scripts/smoke-github-workflow.sh
#
# Exit code 0 = all runnable parts passed. Non-zero = one or more failed.

set -u

# ---------------------------------------------------------------------------
# Config + preflight
# ---------------------------------------------------------------------------

: "${DS_BASE_URL:=http://localhost:8080}"
: "${DS_API_KEY:?DS_API_KEY is required (ds_... token with status:write scope)}"
: "${APP_ID:?APP_ID is required (uuid of the app the key is scoped to)}"
: "${ENV_ID:?ENV_ID is required (uuid of the env the key is scoped to)}"

SHA="$(git rev-parse HEAD 2>/dev/null || echo 0000000000000000000000000000000000000000)"
FAIL_SHA="deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

RED=$(printf '\033[31m')
GRN=$(printf '\033[32m')
YEL=$(printf '\033[33m')
DIM=$(printf '\033[2m')
RST=$(printf '\033[0m')

PASS=0
FAIL=0
SKIP=0

say_pass() { echo "  ${GRN}PASS${RST} $1"; PASS=$((PASS + 1)); }
say_fail() { echo "  ${RED}FAIL${RST} $1 — $2"; FAIL=$((FAIL + 1)); }
say_skip() { echo "  ${YEL}SKIP${RST} $1 — $2"; SKIP=$((SKIP + 1)); }
section()  { echo; echo "=== $1 ==="; }

for bin in curl jq; do
  if ! command -v "$bin" >/dev/null 2>&1; then
    echo "${RED}error:${RST} required binary '$bin' not found on PATH" >&2
    exit 2
  fi
done

# Helper: send a workflow_run event. Arguments:
#   $1 = target APP_ID (defaults to $APP_ID)
#   $2 = body JSON
#   $3 = extra curl flags (e.g. headers)
# Echoes "<http_code>\t<body>" on stdout.
fire_workflow() {
  local target_app="$1"; shift
  local body="$1"; shift
  local url="$DS_BASE_URL/api/v1/applications/$target_app/integrations/github/workflow"
  local tmp; tmp=$(mktemp)
  local code
  code=$(curl -sS -o "$tmp" -w '%{http_code}' \
    -H "Authorization: Bearer $DS_API_KEY" \
    -H "Content-Type: application/json" \
    -H "X-GitHub-Event: workflow_run" \
    -X POST "$url" \
    -d "$body" "$@")
  local body_out; body_out=$(cat "$tmp")
  rm -f "$tmp"
  printf '%s\t%s\n' "$code" "$body_out"
}

# Helper: workflow_run payload template. $1=action, $2=conclusion or empty,
# $3=workflow name, $4=head_sha, $5=html_url.
payload() {
  jq -cn \
    --arg action "$1" --arg conclusion "$2" \
    --arg name "$3" --arg sha "$4" --arg url "$5" \
    '{
      action: $action,
      workflow_run: {
        name: $name,
        status: (if $action == "completed" then "completed" else $action end),
        conclusion: ($conclusion // null),
        head_sha: $sha,
        head_branch: "main",
        html_url: $url
      },
      repository: { full_name: "deploysentry/smoke-test" }
    }'
}

echo "Target: $DS_BASE_URL"
echo "App:    $APP_ID"
echo "Env:    $ENV_ID"
echo "SHA:    ${DIM}$SHA${RST}"

# ---------------------------------------------------------------------------
# Part A — Autocomplete endpoints
# ---------------------------------------------------------------------------
section "Part A — Autocomplete endpoints"

resp=$(curl -sS -o /tmp/ds-smoke-artifacts.json -w '%{http_code}' \
  -H "Authorization: Bearer $DS_API_KEY" \
  "$DS_BASE_URL/api/v1/applications/$APP_ID/artifacts")
if [ "$resp" = "200" ] && jq -e '.artifacts | type == "array"' /tmp/ds-smoke-artifacts.json >/dev/null; then
  count=$(jq '.artifacts | length' /tmp/ds-smoke-artifacts.json)
  say_pass "A.1 GET /artifacts → 200, ${count} entries"
else
  say_fail "A.1 GET /artifacts" "HTTP $resp, body: $(cat /tmp/ds-smoke-artifacts.json)"
fi

resp=$(curl -sS -o /tmp/ds-smoke-versions.json -w '%{http_code}' \
  -H "Authorization: Bearer $DS_API_KEY" \
  "$DS_BASE_URL/api/v1/applications/$APP_ID/versions?environment_id=$ENV_ID")
if [ "$resp" = "200" ] && jq -e '.versions | type == "array"' /tmp/ds-smoke-versions.json >/dev/null; then
  count=$(jq '.versions | length' /tmp/ds-smoke-versions.json)
  say_pass "A.2 GET /versions?environment_id=… → 200, ${count} entries"
else
  say_fail "A.2 GET /versions" "HTTP $resp, body: $(cat /tmp/ds-smoke-versions.json)"
fi

say_skip "A.3 UI autocomplete (combobox)" "web wiring reverted; run manually after restoration"

# ---------------------------------------------------------------------------
# Part B — workflow_run ingestion
# ---------------------------------------------------------------------------
section "Part B — workflow_run ingestion"

# B.1 in_progress
body=$(payload in_progress "" CI "$SHA" "https://github.com/x/y/actions/runs/1")
result=$(fire_workflow "$APP_ID" "$body")
code="${result%%	*}"
resp_body="${result#*	}"
b1_action=$(echo "$resp_body" | jq -r '.action // empty')
b1_id=$(echo "$resp_body" | jq -r '.deployment_id // empty')
if [ "$code" = "202" ] && [ "$b1_action" = "created" ]; then
  say_pass "B.1 in_progress → 202 action=created id=${b1_id:0:8}…"
else
  say_fail "B.1 in_progress" "HTTP $code, body: $resp_body"
fi

# B.2 completed/success on the SAME sha+workflow → update
body=$(payload completed success CI "$SHA" "https://github.com/x/y/actions/runs/1")
result=$(fire_workflow "$APP_ID" "$body")
code="${result%%	*}"
resp_body="${result#*	}"
b2_action=$(echo "$resp_body" | jq -r '.action // empty')
b2_status=$(echo "$resp_body" | jq -r '.status // empty')
if [ "$code" = "202" ] && [ "$b2_action" = "updated" ] && [ "$b2_status" = "completed" ]; then
  say_pass "B.2 completed/success → 202 action=updated status=completed"
else
  say_fail "B.2 completed/success" "HTTP $code, body: $resp_body (expected action=updated status=completed)"
fi

# B.3 parallel lane: different workflow name on the SAME sha → new row
body=$(payload in_progress "" e2e "$SHA" "https://github.com/x/y/actions/runs/2")
result=$(fire_workflow "$APP_ID" "$body")
code="${result%%	*}"
resp_body="${result#*	}"
b3_action=$(echo "$resp_body" | jq -r '.action // empty')
if [ "$code" = "202" ] && [ "$b3_action" = "created" ]; then
  say_pass "B.3 parallel lane (workflow=e2e) → 202 action=created"
else
  say_fail "B.3 parallel lane" "HTTP $code, body: $resp_body"
fi

# B.4 failure lane on a fresh sha
body=$(payload completed failure CI "$FAIL_SHA" "https://github.com/x/y/actions/runs/3")
result=$(fire_workflow "$APP_ID" "$body")
code="${result%%	*}"
resp_body="${result#*	}"
b4_action=$(echo "$resp_body" | jq -r '.action // empty')
b4_status=$(echo "$resp_body" | jq -r '.status // empty')
if [ "$code" = "202" ] && [ "$b4_action" = "created" ] && [ "$b4_status" = "failed" ]; then
  say_pass "B.4 failure lane → 202 action=created status=failed"
else
  say_fail "B.4 failure lane" "HTTP $code, body: $resp_body"
fi

# ---------------------------------------------------------------------------
# Part C — DB sanity
# ---------------------------------------------------------------------------
section "Part C — DB sanity"

if [ -z "${DS_DATABASE_DSN:-}" ] || ! command -v psql >/dev/null 2>&1; then
  say_skip "C.1 DB row inspection" "DS_DATABASE_DSN unset or psql not on PATH"
else
  query="SET search_path TO deploy;
         SELECT COUNT(*) FILTER (WHERE source LIKE 'github-actions%') AS gh_count,
                COUNT(*) FILTER (WHERE source = 'github-actions:CI' AND commit_sha = '$SHA') AS ci_sha,
                COUNT(*) FILTER (WHERE source = 'github-actions:e2e' AND commit_sha = '$SHA') AS e2e_sha,
                COUNT(*) FILTER (WHERE status = 'failed' AND commit_sha = '$FAIL_SHA') AS fail_rows
         FROM deployments;"
  row=$(psql "$DS_DATABASE_DSN" -tA -F '|' -c "$query" 2>/dev/null || true)
  if [ -z "$row" ]; then
    say_fail "C.1 DB counts" "psql query returned no output (check DSN + schema)"
  else
    IFS='|' read -r gh_count ci_sha e2e_sha fail_rows <<<"$row"
    if [ "${ci_sha:-0}" -ge 1 ] && [ "${e2e_sha:-0}" -ge 1 ] && [ "${fail_rows:-0}" -ge 1 ]; then
      say_pass "C.1 DB shows CI, e2e, and failure rows (gh total=$gh_count)"
    else
      say_fail "C.1 DB counts" "ci=$ci_sha e2e=$e2e_sha fail=$fail_rows (each should be ≥1)"
    fi
  fi
fi

# ---------------------------------------------------------------------------
# Part D — Negative cases
# ---------------------------------------------------------------------------
section "Part D — Negative cases"

# D.1 app-scope mismatch (needs OTHER_APP_ID — different uuid, same key)
if [ -z "${OTHER_APP_ID:-}" ]; then
  say_skip "D.1 app-scope mismatch" "OTHER_APP_ID not set"
else
  body=$(payload in_progress "" CI "$SHA" "https://example.test/")
  result=$(fire_workflow "$OTHER_APP_ID" "$body")
  code="${result%%	*}"
  resp_body="${result#*	}"
  if [ "$code" = "403" ]; then
    say_pass "D.1 app-scope mismatch → 403"
  else
    say_fail "D.1 app-scope mismatch" "HTTP $code, body: $resp_body (expected 403)"
  fi
fi

# D.2 missing head_sha
bad_body='{"action":"in_progress","workflow_run":{"name":"CI"}}'
result=$(fire_workflow "$APP_ID" "$bad_body")
code="${result%%	*}"
resp_body="${result#*	}"
if [ "$code" = "400" ]; then
  say_pass "D.2 missing head_sha → 400"
else
  say_fail "D.2 missing head_sha" "HTTP $code, body: $resp_body (expected 400)"
fi

# D.3 ping
tmp=$(mktemp)
code=$(curl -sS -o "$tmp" -w '%{http_code}' \
  -H "Authorization: Bearer $DS_API_KEY" \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: ping" \
  -X POST \
  "$DS_BASE_URL/api/v1/applications/$APP_ID/integrations/github/workflow" \
  -d '{}')
body_out=$(cat "$tmp"); rm -f "$tmp"
status=$(echo "$body_out" | jq -r '.status // empty' 2>/dev/null)
if [ "$code" = "200" ] && [ "$status" = "pong" ]; then
  say_pass "D.3 ping → 200 pong"
else
  say_fail "D.3 ping" "HTTP $code, body: $body_out"
fi

# ---------------------------------------------------------------------------
# Part E — Idempotency
# ---------------------------------------------------------------------------
section "Part E — Idempotency"

# Replay B.1 verbatim. The server should return action=updated since
# (app, env, sha, workflow) already exists from B.1/B.2.
body=$(payload in_progress "" CI "$SHA" "https://github.com/x/y/actions/runs/1")
result=$(fire_workflow "$APP_ID" "$body")
code="${result%%	*}"
resp_body="${result#*	}"
e_action=$(echo "$resp_body" | jq -r '.action // empty')
e_id=$(echo "$resp_body" | jq -r '.deployment_id // empty')
if [ "$code" = "202" ] && [ "$e_action" = "updated" ] && [ "$e_id" = "$b1_id" ]; then
  say_pass "E.1 replay → 202 action=updated same id"
else
  say_fail "E.1 replay" "HTTP $code action=$e_action id=${e_id:0:8} expected=(updated, ${b1_id:0:8})"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
section "Summary"
printf '  ${GRN}%d passed${RST}  ${RED}%d failed${RST}  ${YEL}%d skipped${RST}\n' \
  "$PASS" "$FAIL" "$SKIP"
# Re-emit cleanly without ANSI-in-format when piping.
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
exit 0
