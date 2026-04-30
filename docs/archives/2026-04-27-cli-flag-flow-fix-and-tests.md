# CLI Flag Flow — Fix and Test Coverage Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox syntax.

**Goal:** Fix the CLI's flag commands (which all currently 404 or 400 due to URL + body-shape drift) and back them with table-driven tests using an httptest mock server. Add subcommands for targeting-rule CRUD and per-environment value setting so operators can do the full flag flow from the CLI.

**Architecture:** Shared test harness (`cli_test_helpers.go`) runs each cobra command against an httptest server and asserts both the request shape (URL, method, body) and CLI output. A new `resolve.go` helper resolves project-slug→UUID, environment-slug→UUID, and flag-key→UUID via the existing entity + flag list endpoints (results are not cached — CLI runs are short-lived).

**Tech Stack:** Go 1.25, `cobra`, `httptest`, `testify` (already a project dep).

**Scope:** Phase A only — flag flow + targeting rules + per-env values. Other CLI commands (apps, projects, deploy, releases, etc.) get the same treatment in a separate Phase B.

---

## Investigation summary (read this first)

The real flag API endpoints (mounted on `/api/v1/`):

| Method | Path | Purpose | Body |
|--------|------|---------|------|
| POST | `/flags` | Create | `{project_id (uuid, req), key, name (req), flag_type (req), category, description, default_value, environment_id (uuid, opt), tags, owners, is_permanent, expires_at}` |
| GET | `/flags?project_id=...&category=...&archived=...` | List | — |
| GET | `/flags/:id` | Get by UUID | — |
| PUT | `/flags/:id` | Update | partial of create |
| POST | `/flags/:id/archive` | Archive | — |
| POST | `/flags/:id/toggle` | Unscoped enable/disable | `{enabled: bool}` |
| POST | `/flags/evaluate` | Evaluate | `{flag_key, project_id?, environment_id?, context}` |
| GET | `/flags/:id/rules` | List rules | — |
| POST | `/flags/:id/rules` | Add rule | `{rule_type, priority, value, percentage?, attribute?, operator?, target_values?, segment_id?, start_time?, end_time?, enabled}` |
| PUT | `/flags/:id/rules/:ruleId` | Update rule | partial |
| DELETE | `/flags/:id/rules/:ruleId` | Delete rule | — |
| PUT | `/flags/:id/rules/:ruleId/environments/:envId` | Set rule env state | `{enabled: bool}` |
| GET | `/flags/:id/environments` | List flag env states | — |
| PUT | `/flags/:id/environments/:envId` | Set flag env state | `{enabled: bool, value?: any}` |

Resolvers needed:

- **project-slug → project UUID:** `GET /api/v1/orgs/:orgSlug/projects/:projectSlug` returns `{id: uuid, ...}`.
- **env-slug → env UUID:** `GET /api/v1/orgs/:orgSlug/environments` returns `[{id, slug, ...}]` — pick by slug match.
- **flag-key → flag UUID:** `GET /api/v1/flags?project_id=<uuid>` returns `{flags: [{id, key, ...}]}` — pick by key match.

---

## File Structure

```
cmd/cli/
├── flags.go                        # MODIFY heavily — fix every flag command
├── flags_test.go                   # CREATE — table-driven tests for every flag command
├── flags_rules.go                  # CREATE — `flags rules` subcommands
├── flags_rules_test.go             # CREATE
├── flags_envstate.go               # CREATE — `flags set-value` and env-scoped toggles
├── flags_envstate_test.go          # CREATE
├── resolve.go                      # MODIFY — add resolveProjectID, resolveEnvID, resolveFlagID
├── resolve_test.go                 # CREATE
├── client.go                       # MODIFY (small) — expose a `WithBaseURL` test seam if not already
└── cli_test_helpers.go             # CREATE — httptest server harness + cobra runner

docs/superpowers/plans/
└── 2026-04-27-cli-flag-flow-fix-and-tests.md   # THIS FILE
```

---

## Task 1: Test harness (cli_test_helpers.go)

**Files:**
- Create: `cmd/cli/cli_test_helpers.go` (build tag-free; only used by `_test.go` files)

The harness gives each test:
- `newMockServer(t, handler)` — `*httptest.Server` whose responses come from a `RouteHandler` map.
- `runCmd(t, cmd, args...)` — executes a cobra command, capturing stdout / stderr / err.
- `setTestConfig(t, baseURL, token, org, project, env)` — points the CLI at the mock server, sets in-memory config.

- [ ] **Step 1:** Read `cmd/cli/client.go` to understand how the CLI builds the API URL today. The `clientFromConfig()` function reads from a viper config; we'll need to inject `api.url` and `auth.token` for tests.

- [ ] **Step 2:** Create `cmd/cli/cli_test_helpers.go` with build-tag-free test helpers (so they compile in test binaries only because they're imported only from `_test.go`):

```go
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// recordedRequest captures one inbound HTTP call so tests can assert on shape.
type recordedRequest struct {
	Method string
	Path   string         // includes query string
	Body   map[string]any // parsed JSON body (nil for GET / empty body)
}

// mockServer is a tiny httptest wrapper that records every request and
// dispatches based on (METHOD, path-prefix) to a stub function returning
// (status, body).
type mockServer struct {
	t        *testing.T
	srv      *httptest.Server
	requests []recordedRequest
	routes   []routeStub
}

type routeStub struct {
	Method  string
	Match   func(path string) bool
	Respond func(req recordedRequest) (status int, body any)
}

func newMockServer(t *testing.T) *mockServer {
	t.Helper()
	m := &mockServer{t: t}
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.srv.Close)
	return m
}

func (m *mockServer) URL() string { return m.srv.URL }

func (m *mockServer) on(method string, match func(string) bool, respond func(recordedRequest) (int, any)) {
	m.routes = append(m.routes, routeStub{Method: method, Match: match, Respond: respond})
}

// onPath stubs an exact-method, exact-path-prefix route. `pathPrefix` is matched
// against the URL.Path *before* the query string. It matches if the incoming path
// starts with `pathPrefix`. Use this for most tests; for exact match include
// trailing characters or use `on()` directly.
func (m *mockServer) onPath(method, pathPrefix string, status int, body any) {
	m.on(method, func(p string) bool { return strings.HasPrefix(p, pathPrefix) }, func(recordedRequest) (int, any) {
		return status, body
	})
}

// onPathFunc is like onPath but lets the test inspect the request and return
// a dynamic response (e.g., echo back posted fields).
func (m *mockServer) onPathFunc(method, pathPrefix string, fn func(recordedRequest) (int, any)) {
	m.on(method, func(p string) bool { return strings.HasPrefix(p, pathPrefix) }, fn)
}

func (m *mockServer) handle(w http.ResponseWriter, r *http.Request) {
	rec := recordedRequest{Method: r.Method, Path: r.URL.RequestURI()}
	if r.Body != nil {
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &rec.Body)
		}
	}
	m.requests = append(m.requests, rec)

	for _, route := range m.routes {
		if route.Method != r.Method {
			continue
		}
		if !route.Match(r.URL.Path) {
			continue
		}
		status, body := route.Respond(rec)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
		return
	}
	// Unmatched: return 599 so the test fails loudly with a descriptive body.
	m.t.Errorf("unexpected request %s %s", r.Method, r.URL.RequestURI())
	w.WriteHeader(599)
	_, _ = w.Write([]byte(`{"error":"unstubbed route"}`))
}

// setTestConfig points the CLI's viper config at the mock server. Call this
// from each test before invoking a command.
func setTestConfig(t *testing.T, baseURL, token, org, project, env string) {
	t.Helper()
	viper.Reset()
	viper.Set("api.url", baseURL)
	viper.Set("auth.token", token)
	viper.Set("org", org)
	viper.Set("project", project)
	if env != "" {
		viper.Set("env", env)
	}
	t.Cleanup(viper.Reset)
}

// runCmd executes a cobra command tree with the given argv (rooted at flagsCmd
// or rootCmd in the calling test). Returns combined stdout, stderr, and the
// command's RunE error.
func runCmd(t *testing.T, root *cobra.Command, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}
```

- [ ] **Step 3:** Verify it compiles:

```bash
cd /Users/sgamel/git/DeploySentry/.worktrees/cli-flags-fix && go build ./cmd/cli/...
```

Expected: clean.

- [ ] **Step 4:** Commit:

```bash
git add cmd/cli/cli_test_helpers.go
git commit -m "test(cli): add httptest harness + cobra runner"
```

---

## Task 2: Resolvers (slug→UUID for project, env, flag)

**Files:**
- Modify: `cmd/cli/resolve.go` — add `resolveProjectID`, `resolveEnvID`, `resolveFlagID`
- Create: `cmd/cli/resolve_test.go`

### Step 1: Read existing `cmd/cli/resolve.go`

Note what's already there. The existing helpers (e.g., `requireOrg`, `requireProject`) read viper. We'll add three new functions.

### Step 2: Append resolvers to `cmd/cli/resolve.go`

```go
// resolveProjectID returns the project's UUID by GETting
// /api/v1/orgs/:org/projects/:project. Errors out with a useful
// message if the project doesn't exist or the API is unreachable.
func resolveProjectID(client *apiClient, org, projectSlug string) (string, error) {
	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s", org, projectSlug)
	resp, err := client.get(path)
	if err != nil {
		return "", fmt.Errorf("resolve project %q: %w", projectSlug, err)
	}
	id, ok := resp["id"].(string)
	if !ok || id == "" {
		return "", fmt.Errorf("resolve project %q: response missing id", projectSlug)
	}
	return id, nil
}

// resolveEnvID returns the environment's UUID for the given org by listing
// org-level environments and matching on slug.
func resolveEnvID(client *apiClient, org, envSlug string) (string, error) {
	path := fmt.Sprintf("/api/v1/orgs/%s/environments", org)
	resp, err := client.get(path)
	if err != nil {
		return "", fmt.Errorf("resolve environment %q: %w", envSlug, err)
	}
	envs, _ := resp["environments"].([]any)
	for _, e := range envs {
		obj, ok := e.(map[string]any)
		if !ok {
			continue
		}
		slug, _ := obj["slug"].(string)
		if slug == envSlug {
			id, _ := obj["id"].(string)
			if id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("environment %q not found in org %q", envSlug, org)
}

// resolveFlagID returns a flag's UUID by listing flags in the given project
// and matching on key. Returns ErrFlagNotFound if no flag has that key.
var ErrFlagNotFound = errors.New("flag not found")

func resolveFlagID(client *apiClient, projectID, flagKey string) (string, error) {
	path := fmt.Sprintf("/api/v1/flags?project_id=%s", projectID)
	resp, err := client.get(path)
	if err != nil {
		return "", fmt.Errorf("resolve flag %q: %w", flagKey, err)
	}
	flags, _ := resp["flags"].([]any)
	for _, f := range flags {
		obj, ok := f.(map[string]any)
		if !ok {
			continue
		}
		key, _ := obj["key"].(string)
		if key == flagKey {
			id, _ := obj["id"].(string)
			if id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("%w: %q in project %s", ErrFlagNotFound, flagKey, projectID)
}
```

If `errors` isn't already imported, add it. Same for `fmt`.

### Step 3: Write `cmd/cli/resolve_test.go`

```go
package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveProjectID(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/orgs/acme/projects/payments", func(recordedRequest) (int, any) {
		return 200, map[string]any{"id": "00000000-0000-0000-0000-000000000001", "slug": "payments"}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")
	client, err := clientFromConfig()
	require.NoError(t, err)

	id, err := resolveProjectID(client, "acme", "payments")
	require.NoError(t, err)
	require.Equal(t, "00000000-0000-0000-0000-000000000001", id)
}

func TestResolveEnvID(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/orgs/acme/environments", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"environments": []map[string]any{
				{"id": "env-staging-uuid", "slug": "staging"},
				{"id": "env-prod-uuid", "slug": "production"},
			},
		}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")
	client, err := clientFromConfig()
	require.NoError(t, err)

	id, err := resolveEnvID(client, "acme", "production")
	require.NoError(t, err)
	require.Equal(t, "env-prod-uuid", id)

	_, err = resolveEnvID(client, "acme", "nope")
	require.Error(t, err)
}

func TestResolveFlagID(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"flags": []map[string]any{
				{"id": "flag-1-uuid", "key": "dark-mode"},
				{"id": "flag-2-uuid", "key": "new-checkout"},
			},
		}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")
	client, err := clientFromConfig()
	require.NoError(t, err)

	id, err := resolveFlagID(client, "00000000-0000-0000-0000-000000000001", "new-checkout")
	require.NoError(t, err)
	require.Equal(t, "flag-2-uuid", id)

	_, err = resolveFlagID(client, "00000000-0000-0000-0000-000000000001", "missing")
	require.ErrorIs(t, err, ErrFlagNotFound)
}
```

### Step 4: Verify

```bash
cd .worktrees/cli-flags-fix && go test ./cmd/cli/ -run TestResolve -v
```

Expected: 3 tests pass.

### Step 5: Commit

```bash
git add cmd/cli/resolve.go cmd/cli/resolve_test.go
git commit -m "feat(cli): add slug→UUID resolvers for project, env, flag"
```

---

## Task 3: Fix `flags create` — URL, body, env-scoping

**Files:**
- Modify: `cmd/cli/flags.go` (`runFlagsCreate`)
- Create: `cmd/cli/flags_test.go` (start with create tests)

### Step 1: Rewrite `runFlagsCreate`

Replace the existing function with one that:

1. Resolves project-slug → project UUID.
2. Resolves env-slug → env UUID (only if `--env` was passed).
3. POSTs to `/api/v1/flags` (NOT the org/project path) with body:
   ```json
   {
     "project_id": "<uuid>",
     "key": "<key>",
     "name": "<name or key>",
     "flag_type": "<boolean|string|number|json>",
     "category": "feature",
     "description": "<...>",
     "default_value": "<...>",
     "environment_id": "<uuid or null>",
     "tags": [...]
   }
   ```
4. Adds new `--name` flag (defaults to `--key` value if not provided).
5. Adds new `--category` flag (defaults to `"feature"`).

Final `runFlagsCreate`:

```go
func runFlagsCreate(cmd *cobra.Command, args []string) error {
	_ = args
	org, err := requireOrg()
	if err != nil {
		return err
	}
	projectSlug, err := requireProject()
	if err != nil {
		return err
	}

	key, _ := cmd.Flags().GetString("key")
	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		name = key
	}
	flagType, _ := cmd.Flags().GetString("type")
	defaultVal, _ := cmd.Flags().GetString("default")
	description, _ := cmd.Flags().GetString("description")
	category, _ := cmd.Flags().GetString("category")
	if category == "" {
		category = "feature"
	}
	tags, _ := cmd.Flags().GetStringSlice("tag")

	validTypes := map[string]bool{"boolean": true, "string": true, "integer": true, "json": true}
	if !validTypes[flagType] {
		return fmt.Errorf("invalid flag type %q; must be one of: boolean, string, integer, json", flagType)
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	projectID, err := resolveProjectID(client, org, projectSlug)
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"project_id": projectID,
		"key":        key,
		"name":       name,
		"flag_type":  flagType,
		"category":   category,
	}
	if defaultVal != "" {
		body["default_value"] = defaultVal
	}
	if description != "" {
		body["description"] = description
	}
	if len(tags) > 0 {
		body["tags"] = tags
	}
	if envSlug := getEnv(); envSlug != "" {
		envID, err := resolveEnvID(client, org, envSlug)
		if err != nil {
			return err
		}
		body["environment_id"] = envID
	}

	resp, err := client.post("/api/v1/flags", body)
	if err != nil {
		return fmt.Errorf("failed to create flag: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Feature flag created successfully.\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Key:     %s\n", key)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Type:    %s\n", flagType)
	if defaultVal != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Default: %s\n", defaultVal)
	}
	return nil
}
```

### Step 2: Add the new flags in `init()` of `flags.go`:

After the existing `flagsCreateCmd.Flags()...` block, add:

```go
flagsCreateCmd.Flags().String("name", "", "human-readable name (defaults to --key)")
flagsCreateCmd.Flags().String("category", "feature", "flag category: release, feature, experiment, ops, permission")
```

### Step 3: Write `cmd/cli/flags_test.go` (create tests first)

```go
package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// stubProjectAndEnv stubs the resolvers used by every flag command.
func stubProjectAndEnv(t *testing.T, srv *mockServer, projectID, envID string) {
	t.Helper()
	srv.onPathFunc("GET", "/api/v1/orgs/acme/projects/payments", func(recordedRequest) (int, any) {
		return 200, map[string]any{"id": projectID, "slug": "payments", "name": "Payments"}
	})
	srv.onPathFunc("GET", "/api/v1/orgs/acme/environments", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"environments": []map[string]any{
				{"id": envID, "slug": "production", "name": "Production"},
				{"id": "env-staging-uuid", "slug": "staging", "name": "Staging"},
			},
		}
	})
}

func TestFlagsCreate_Success_NoEnv(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("POST", "/api/v1/flags", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/flags", req.Path)
		require.Equal(t, "proj-uuid", req.Body["project_id"])
		require.Equal(t, "dark-mode", req.Body["key"])
		require.Equal(t, "dark-mode", req.Body["name"])      // defaults to key
		require.Equal(t, "boolean", req.Body["flag_type"])
		require.Equal(t, "feature", req.Body["category"])
		_, hasEnv := req.Body["environment_id"]
		require.False(t, hasEnv, "no env_id when --env not passed")
		return 201, map[string]any{"id": "f-1", "key": "dark-mode"}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	stdout, _, err := runCmd(t, flagsCreateCmd, "--key", "dark-mode", "--type", "boolean")
	require.NoError(t, err)
	require.Contains(t, stdout, "created successfully")
}

func TestFlagsCreate_Success_WithEnv(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("POST", "/api/v1/flags", func(req recordedRequest) (int, any) {
		require.Equal(t, "env-prod-uuid", req.Body["environment_id"])
		return 201, map[string]any{"id": "f-1", "key": "dark-mode"}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "production")

	_, _, err := runCmd(t, flagsCreateCmd, "--key", "dark-mode", "--type", "boolean", "--default", "false")
	require.NoError(t, err)
}

func TestFlagsCreate_Success_FullPayload(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("POST", "/api/v1/flags", func(req recordedRequest) (int, any) {
		require.Equal(t, "Dark Mode UI", req.Body["name"])
		require.Equal(t, "release", req.Body["category"])
		require.Equal(t, "Toggles dark mode", req.Body["description"])
		require.Equal(t, "false", req.Body["default_value"])
		tags, _ := req.Body["tags"].([]any)
		require.ElementsMatch(t, []any{"ui", "rollout"}, tags)
		return 201, map[string]any{"id": "f-1", "key": "dark-mode"}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	_, _, err := runCmd(t, flagsCreateCmd,
		"--key", "dark-mode",
		"--name", "Dark Mode UI",
		"--type", "boolean",
		"--category", "release",
		"--description", "Toggles dark mode",
		"--default", "false",
		"--tag", "ui",
		"--tag", "rollout",
	)
	require.NoError(t, err)
}

func TestFlagsCreate_InvalidType(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	_, _, err := runCmd(t, flagsCreateCmd, "--key", "x", "--type", "weird")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid flag type")
}

func TestFlagsCreate_APIError(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("POST", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 422, map[string]any{"error": "key must be unique"}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	_, _, err := runCmd(t, flagsCreateCmd, "--key", "dup", "--type", "boolean")
	require.Error(t, err)
	require.Contains(t, err.Error(), "key must be unique")
}
```

### Step 4: Run

```bash
cd .worktrees/cli-flags-fix && go test ./cmd/cli/ -run TestFlagsCreate -v
```

Expected: 5 tests pass.

### Step 5: Commit

```bash
git add cmd/cli/flags.go cmd/cli/flags_test.go
git commit -m "fix(cli): flags create — correct URL + body + project/env resolvers"
```

---

## Task 4: Fix `flags get` — resolve key→UUID

### Step 1: Rewrite `runFlagsGet`

Replace with:

```go
func runFlagsGet(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	projectSlug, err := requireProject()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}
	projectID, err := resolveProjectID(client, org, projectSlug)
	if err != nil {
		return err
	}
	flagID, err := resolveFlagID(client, projectID, args[0])
	if err != nil {
		return err
	}

	resp, err := client.get(fmt.Sprintf("/api/v1/flags/%s", flagID))
	if err != nil {
		return fmt.Errorf("failed to get flag %q: %w", args[0], err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Feature Flag: %s\n", args[0])
	if t, ok := resp["flag_type"].(string); ok {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Type:        %s\n", t)
	}
	if d, ok := resp["default_value"]; ok {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Default:     %v\n", d)
	}
	if desc, ok := resp["description"].(string); ok && desc != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Description: %s\n", desc)
	}
	return nil
}
```

(Drop the targeting-rules pretty-print here — `flags rules list` covers that in Task 8.)

### Step 2: Append tests to `flags_test.go`

```go
func TestFlagsGet_Success(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags?project_id=", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	srv.onPathFunc("GET", "/api/v1/flags/f-1", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"id": "f-1", "key": "dark-mode", "flag_type": "boolean",
			"default_value": "false", "description": "Dark UI",
		}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	stdout, _, err := runCmd(t, flagsGetCmd, "dark-mode")
	require.NoError(t, err)
	require.Contains(t, stdout, "Type:        boolean")
	require.Contains(t, stdout, "Default:     false")
	require.Contains(t, stdout, "Description: Dark UI")
}

func TestFlagsGet_FlagNotFound(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags?project_id=", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{}}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	_, _, err := runCmd(t, flagsGetCmd, "missing")
	require.Error(t, err)
}
```

### Step 3: Run

```bash
go test ./cmd/cli/ -run TestFlagsGet -v
```

Expected: 2 pass.

### Step 4: Commit

```bash
git add cmd/cli/flags.go cmd/cli/flags_test.go
git commit -m "fix(cli): flags get — resolve key to UUID"
```

---

## Task 5: Fix `flags update`

### Step 1: Rewrite `runFlagsUpdate`

The handler `updateFlag` accepts a partial `updateFlagRequest` (no `add_rule` field — rules go through `flags rules` subcommands per Task 8). Replace:

```go
func runFlagsUpdate(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	projectSlug, err := requireProject()
	if err != nil {
		return err
	}
	client, err := clientFromConfig()
	if err != nil {
		return err
	}
	projectID, err := resolveProjectID(client, org, projectSlug)
	if err != nil {
		return err
	}
	flagID, err := resolveFlagID(client, projectID, args[0])
	if err != nil {
		return err
	}

	body := map[string]interface{}{}
	if cmd.Flags().Changed("default") {
		v, _ := cmd.Flags().GetString("default")
		body["default_value"] = v
	}
	if cmd.Flags().Changed("description") {
		v, _ := cmd.Flags().GetString("description")
		body["description"] = v
	}
	if cmd.Flags().Changed("name") {
		v, _ := cmd.Flags().GetString("name")
		body["name"] = v
	}
	if cmd.Flags().Changed("category") {
		v, _ := cmd.Flags().GetString("category")
		body["category"] = v
	}
	if cmd.Flags().Changed("tag") {
		v, _ := cmd.Flags().GetStringSlice("tag")
		body["tags"] = v
	}

	if len(body) == 0 {
		return fmt.Errorf("no updates specified; use --default, --description, --name, --category, or --tag")
	}

	resp, err := client.put(fmt.Sprintf("/api/v1/flags/%s", flagID), body)
	if err != nil {
		return fmt.Errorf("failed to update flag %q: %w", args[0], err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Flag %q updated successfully.\n", args[0])
	return nil
}
```

### Step 2: Add `--name` and `--category` to update flags in `init()`:

```go
flagsUpdateCmd.Flags().String("name", "", "updated name")
flagsUpdateCmd.Flags().String("category", "", "updated category")
```

Remove `--add-rule` from `flagsUpdateCmd.Flags()` (rules now have their own subcommand).

### Step 3: Verify `client.put` exists in `client.go`

If it does not, add it (mirrors `client.post`):

```go
func (c *apiClient) put(path string, body any) (map[string]any, error) {
	return c.do("PUT", path, body)
}
```

(Adapt to whatever the existing `client.go` factoring is.)

### Step 4: Append tests

```go
func TestFlagsUpdate_DefaultValue(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags?project_id=", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	srv.onPathFunc("PUT", "/api/v1/flags/f-1", func(req recordedRequest) (int, any) {
		require.Equal(t, "true", req.Body["default_value"])
		return 200, map[string]any{"id": "f-1"}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	_, _, err := runCmd(t, flagsUpdateCmd, "dark-mode", "--default", "true")
	require.NoError(t, err)
}

func TestFlagsUpdate_NoChanges(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags?project_id=", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	_, _, err := runCmd(t, flagsUpdateCmd, "dark-mode")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no updates specified")
}
```

### Step 5: Run + commit

```bash
go test ./cmd/cli/ -run TestFlagsUpdate -v
git add cmd/cli/flags.go cmd/cli/flags_test.go cmd/cli/client.go
git commit -m "fix(cli): flags update — resolve key + drop bogus --add-rule"
```

---

## Task 6: Fix `flags toggle` — env-scoped via env-state PUT, unscoped via /toggle

### Step 1: Rewrite `runFlagsToggle`

The contract: if `--env <slug>` is set, toggle is per-environment via `PUT /flags/:id/environments/:envId`. Otherwise unscoped via `POST /flags/:id/toggle`.

```go
func runFlagsToggle(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	projectSlug, err := requireProject()
	if err != nil {
		return err
	}

	on, _ := cmd.Flags().GetBool("on")
	off, _ := cmd.Flags().GetBool("off")
	if !on && !off {
		return fmt.Errorf("you must specify either --on or --off")
	}
	if on && off {
		return fmt.Errorf("cannot specify both --on and --off")
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}
	projectID, err := resolveProjectID(client, org, projectSlug)
	if err != nil {
		return err
	}
	flagID, err := resolveFlagID(client, projectID, args[0])
	if err != nil {
		return err
	}

	enabled := on
	envSlug := getEnv()

	var resp map[string]any
	if envSlug != "" {
		envID, err := resolveEnvID(client, org, envSlug)
		if err != nil {
			return err
		}
		resp, err = client.put(
			fmt.Sprintf("/api/v1/flags/%s/environments/%s", flagID, envID),
			map[string]any{"enabled": enabled},
		)
		if err != nil {
			return fmt.Errorf("failed to toggle flag %q in env %q: %w", args[0], envSlug, err)
		}
	} else {
		resp, err = client.post(
			fmt.Sprintf("/api/v1/flags/%s/toggle", flagID),
			map[string]any{"enabled": enabled},
		)
		if err != nil {
			return fmt.Errorf("failed to toggle flag %q: %w", args[0], err)
		}
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	state := "OFF"
	if enabled {
		state = "ON"
	}
	if envSlug != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Flag %q toggled %s in %s.\n", args[0], state, envSlug)
	} else {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Flag %q toggled %s.\n", args[0], state)
	}
	return nil
}
```

### Step 2: Append tests

```go
func TestFlagsToggle_Unscoped_On(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags?project_id=", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	srv.onPathFunc("POST", "/api/v1/flags/f-1/toggle", func(req recordedRequest) (int, any) {
		require.Equal(t, true, req.Body["enabled"])
		return 200, map[string]any{"enabled": true}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	stdout, _, err := runCmd(t, flagsToggleCmd, "dark-mode", "--on")
	require.NoError(t, err)
	require.Contains(t, stdout, `toggled ON`)
	require.NotContains(t, stdout, "in ")
}

func TestFlagsToggle_EnvScoped_Off(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags?project_id=", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	srv.onPathFunc("PUT", "/api/v1/flags/f-1/environments/env-prod-uuid", func(req recordedRequest) (int, any) {
		require.Equal(t, false, req.Body["enabled"])
		return 200, map[string]any{"enabled": false}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "production")

	stdout, _, err := runCmd(t, flagsToggleCmd, "dark-mode", "--off")
	require.NoError(t, err)
	require.Contains(t, stdout, "toggled OFF in production")
}

func TestFlagsToggle_NoFlag(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	_, _, err := runCmd(t, flagsToggleCmd, "dark-mode")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--on or --off")
}
```

### Step 3: Run + commit

```bash
go test ./cmd/cli/ -run TestFlagsToggle -v
git add cmd/cli/flags.go cmd/cli/flags_test.go
git commit -m "fix(cli): flags toggle — env-scoped via env-state PUT, unscoped via POST /toggle"
```

---

## Task 7: Fix `flags archive` and `flags evaluate`

### Step 1: Rewrite `runFlagsArchive`

```go
func runFlagsArchive(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	projectSlug, err := requireProject()
	if err != nil {
		return err
	}
	client, err := clientFromConfig()
	if err != nil {
		return err
	}
	projectID, err := resolveProjectID(client, org, projectSlug)
	if err != nil {
		return err
	}
	flagID, err := resolveFlagID(client, projectID, args[0])
	if err != nil {
		return err
	}

	resp, err := client.post(fmt.Sprintf("/api/v1/flags/%s/archive", flagID), nil)
	if err != nil {
		return fmt.Errorf("failed to archive flag %q: %w", args[0], err)
	}
	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Flag %q archived successfully.\n", args[0])
	return nil
}
```

### Step 2: Rewrite `runFlagsEvaluate`

The real endpoint is `POST /api/v1/flags/evaluate` with body `{flag_key, project_id?, environment_id?, context}`. No flagID resolution needed — the API does that itself.

```go
func runFlagsEvaluate(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	projectSlug, err := requireProject()
	if err != nil {
		return err
	}
	client, err := clientFromConfig()
	if err != nil {
		return err
	}
	projectID, err := resolveProjectID(client, org, projectSlug)
	if err != nil {
		return err
	}

	contextJSON, _ := cmd.Flags().GetString("context")
	var evalContext map[string]interface{}
	if err := json.Unmarshal([]byte(contextJSON), &evalContext); err != nil {
		return fmt.Errorf("invalid context JSON: %w", err)
	}

	body := map[string]interface{}{
		"flag_key":   args[0],
		"project_id": projectID,
		"context":    evalContext,
	}
	if envSlug := getEnv(); envSlug != "" {
		envID, err := resolveEnvID(client, org, envSlug)
		if err != nil {
			return err
		}
		body["environment_id"] = envID
	}

	resp, err := client.post("/api/v1/flags/evaluate", body)
	if err != nil {
		return fmt.Errorf("failed to evaluate flag %q: %w", args[0], err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Flag:   %s\n", args[0])
	if v, ok := resp["value"]; ok {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Value:  %v\n", v)
	}
	if r, ok := resp["reason"].(string); ok {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Reason: %s\n", r)
	}
	return nil
}
```

### Step 3: Append tests

```go
func TestFlagsArchive_Success(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags?project_id=", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "old"}}}
	})
	srv.onPathFunc("POST", "/api/v1/flags/f-1/archive", func(recordedRequest) (int, any) {
		return 200, map[string]any{"status": "archived"}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	stdout, _, err := runCmd(t, flagsArchiveCmd, "old")
	require.NoError(t, err)
	require.Contains(t, stdout, "archived successfully")
}

func TestFlagsEvaluate_Success(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("POST", "/api/v1/flags/evaluate", func(req recordedRequest) (int, any) {
		require.Equal(t, "dark-mode", req.Body["flag_key"])
		require.Equal(t, "proj-uuid", req.Body["project_id"])
		ctx, _ := req.Body["context"].(map[string]any)
		require.Equal(t, "u1", ctx["user_id"])
		return 200, map[string]any{"value": true, "reason": "DEFAULT_VALUE"}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	stdout, _, err := runCmd(t, flagsEvaluateCmd, "dark-mode", "--context", `{"user_id":"u1"}`)
	require.NoError(t, err)
	require.Contains(t, stdout, "Value:  true")
	require.Contains(t, stdout, "DEFAULT_VALUE")
}
```

### Step 4: Run + commit

```bash
go test ./cmd/cli/ -run "TestFlagsArchive|TestFlagsEvaluate" -v
git add cmd/cli/flags.go cmd/cli/flags_test.go
git commit -m "fix(cli): flags archive + evaluate — correct URLs and bodies"
```

---

## Task 8: Add `flags rules` subcommands (CRUD + env-state)

**Files:**
- Create: `cmd/cli/flags_rules.go`
- Create: `cmd/cli/flags_rules_test.go`

### Step 1: Implement `flags_rules.go`

```go
package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var flagsRulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Manage targeting rules on a flag",
}

var flagsRulesListCmd = &cobra.Command{
	Use:   "list <flag-key>",
	Short: "List targeting rules for a flag",
	Args:  cobra.ExactArgs(1),
	RunE:  runFlagsRulesList,
}

var flagsRulesAddCmd = &cobra.Command{
	Use:   "add <flag-key>",
	Short: "Add a targeting rule",
	Long: `Add a targeting rule to a flag. Use --rule-type and the appropriate type-specific flags.

Examples:
  # Percentage rollout to 25%
  deploysentry flags rules add dark-mode --rule-type percentage --percentage 25 --value true --priority 10

  # Attribute match
  deploysentry flags rules add dark-mode --rule-type attribute --attribute plan --operator eq --value pro

  # User target
  deploysentry flags rules add dark-mode --rule-type user_target --target-values u1,u2,u3 --value true`,
	Args: cobra.ExactArgs(1),
	RunE: runFlagsRulesAdd,
}

var flagsRulesUpdateCmd = &cobra.Command{
	Use:   "update <flag-key> <rule-id>",
	Short: "Update a targeting rule",
	Args:  cobra.ExactArgs(2),
	RunE:  runFlagsRulesUpdate,
}

var flagsRulesDeleteCmd = &cobra.Command{
	Use:   "delete <flag-key> <rule-id>",
	Short: "Delete a targeting rule",
	Args:  cobra.ExactArgs(2),
	RunE:  runFlagsRulesDelete,
}

var flagsRulesEnableEnvCmd = &cobra.Command{
	Use:   "set-env-state <flag-key> <rule-id>",
	Short: "Enable or disable a rule in a specific environment",
	Args:  cobra.ExactArgs(2),
	RunE:  runFlagsRulesSetEnvState,
}

func init() {
	flagsRulesAddCmd.Flags().String("rule-type", "", "rule type: percentage, user_target, attribute, segment, schedule (required)")
	flagsRulesAddCmd.Flags().Int("priority", 100, "rule priority (lower = higher precedence)")
	flagsRulesAddCmd.Flags().String("value", "", "value returned when rule matches")
	flagsRulesAddCmd.Flags().Int("percentage", 0, "percentage 0-100 (for percentage rules)")
	flagsRulesAddCmd.Flags().String("attribute", "", "attribute name (for attribute rules)")
	flagsRulesAddCmd.Flags().String("operator", "", "operator: eq, neq, in, contains, etc.")
	flagsRulesAddCmd.Flags().StringSlice("target-values", nil, "target values (for user_target / attribute in)")
	flagsRulesAddCmd.Flags().String("segment-id", "", "segment UUID (for segment rules)")
	flagsRulesAddCmd.Flags().Bool("disabled", false, "create the rule disabled")
	_ = flagsRulesAddCmd.MarkFlagRequired("rule-type")

	flagsRulesUpdateCmd.Flags().Int("priority", 0, "new priority")
	flagsRulesUpdateCmd.Flags().String("value", "", "new value")
	flagsRulesUpdateCmd.Flags().Int("percentage", 0, "new percentage")
	flagsRulesUpdateCmd.Flags().Bool("enabled", true, "rule enabled")

	flagsRulesEnableEnvCmd.Flags().Bool("on", false, "enable the rule in this env")
	flagsRulesEnableEnvCmd.Flags().Bool("off", false, "disable the rule in this env")

	flagsRulesCmd.AddCommand(flagsRulesListCmd)
	flagsRulesCmd.AddCommand(flagsRulesAddCmd)
	flagsRulesCmd.AddCommand(flagsRulesUpdateCmd)
	flagsRulesCmd.AddCommand(flagsRulesDeleteCmd)
	flagsRulesCmd.AddCommand(flagsRulesEnableEnvCmd)
	flagsCmd.AddCommand(flagsRulesCmd)
}

// resolveFlagFromArgs is a shared helper that resolves org/project + flag key
// to (apiClient, flagID, projectID). It's defined here (not in resolve.go) to
// keep the rules file self-contained.
func resolveFlagFromArgs(flagKey string) (*apiClient, string, string, error) {
	org, err := requireOrg()
	if err != nil {
		return nil, "", "", err
	}
	projectSlug, err := requireProject()
	if err != nil {
		return nil, "", "", err
	}
	client, err := clientFromConfig()
	if err != nil {
		return nil, "", "", err
	}
	projectID, err := resolveProjectID(client, org, projectSlug)
	if err != nil {
		return nil, "", "", err
	}
	flagID, err := resolveFlagID(client, projectID, flagKey)
	if err != nil {
		return nil, "", "", err
	}
	return client, flagID, projectID, nil
}

func runFlagsRulesList(cmd *cobra.Command, args []string) error {
	client, flagID, _, err := resolveFlagFromArgs(args[0])
	if err != nil {
		return err
	}
	resp, err := client.get(fmt.Sprintf("/api/v1/flags/%s/rules", flagID))
	if err != nil {
		return fmt.Errorf("failed to list rules: %w", err)
	}
	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	rules, _ := resp["rules"].([]any)
	if len(rules) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No targeting rules.")
		return nil
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tTYPE\tPRIORITY\tVALUE\tENABLED")
	for _, r := range rules {
		rule, _ := r.(map[string]any)
		id, _ := rule["id"].(string)
		t, _ := rule["rule_type"].(string)
		p, _ := rule["priority"].(float64)
		v := rule["value"]
		en, _ := rule["enabled"].(bool)
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%v\t%t\n", id, t, int(p), v, en)
	}
	return w.Flush()
}

func runFlagsRulesAdd(cmd *cobra.Command, args []string) error {
	client, flagID, _, err := resolveFlagFromArgs(args[0])
	if err != nil {
		return err
	}
	ruleType, _ := cmd.Flags().GetString("rule-type")
	priority, _ := cmd.Flags().GetInt("priority")
	value, _ := cmd.Flags().GetString("value")
	pct, _ := cmd.Flags().GetInt("percentage")
	attr, _ := cmd.Flags().GetString("attribute")
	op, _ := cmd.Flags().GetString("operator")
	tv, _ := cmd.Flags().GetStringSlice("target-values")
	seg, _ := cmd.Flags().GetString("segment-id")
	disabled, _ := cmd.Flags().GetBool("disabled")

	body := map[string]any{
		"rule_type": ruleType,
		"priority":  priority,
		"value":     value,
		"enabled":   !disabled,
	}
	if cmd.Flags().Changed("percentage") {
		body["percentage"] = pct
	}
	if attr != "" {
		body["attribute"] = attr
	}
	if op != "" {
		body["operator"] = op
	}
	if len(tv) > 0 {
		body["target_values"] = tv
	}
	if seg != "" {
		body["segment_id"] = seg
	}

	resp, err := client.post(fmt.Sprintf("/api/v1/flags/%s/rules", flagID), body)
	if err != nil {
		return fmt.Errorf("failed to add rule: %w", err)
	}
	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	id, _ := resp["id"].(string)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Rule added: %s\n", id)
	return nil
}

func runFlagsRulesUpdate(cmd *cobra.Command, args []string) error {
	client, flagID, _, err := resolveFlagFromArgs(args[0])
	if err != nil {
		return err
	}
	ruleID := args[1]

	body := map[string]any{}
	if cmd.Flags().Changed("priority") {
		v, _ := cmd.Flags().GetInt("priority")
		body["priority"] = v
	}
	if cmd.Flags().Changed("value") {
		v, _ := cmd.Flags().GetString("value")
		body["value"] = v
	}
	if cmd.Flags().Changed("percentage") {
		v, _ := cmd.Flags().GetInt("percentage")
		body["percentage"] = v
	}
	if cmd.Flags().Changed("enabled") {
		v, _ := cmd.Flags().GetBool("enabled")
		body["enabled"] = v
	}
	if len(body) == 0 {
		return fmt.Errorf("no updates specified")
	}

	resp, err := client.put(fmt.Sprintf("/api/v1/flags/%s/rules/%s", flagID, ruleID), body)
	if err != nil {
		return fmt.Errorf("failed to update rule: %w", err)
	}
	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Rule %s updated.\n", ruleID)
	return nil
}

func runFlagsRulesDelete(cmd *cobra.Command, args []string) error {
	client, flagID, _, err := resolveFlagFromArgs(args[0])
	if err != nil {
		return err
	}
	ruleID := args[1]
	if _, err := client.delete(fmt.Sprintf("/api/v1/flags/%s/rules/%s", flagID, ruleID)); err != nil {
		return fmt.Errorf("failed to delete rule: %w", err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Rule %s deleted.\n", ruleID)
	return nil
}

func runFlagsRulesSetEnvState(cmd *cobra.Command, args []string) error {
	client, flagID, _, err := resolveFlagFromArgs(args[0])
	if err != nil {
		return err
	}
	ruleID := args[1]

	on, _ := cmd.Flags().GetBool("on")
	off, _ := cmd.Flags().GetBool("off")
	if !on && !off {
		return fmt.Errorf("you must specify --on or --off")
	}
	if on && off {
		return fmt.Errorf("cannot specify both --on and --off")
	}

	envSlug := getEnv()
	if envSlug == "" {
		return fmt.Errorf("--env (or DS env config) is required for set-env-state")
	}
	org, _ := requireOrg()
	envID, err := resolveEnvID(client, org, envSlug)
	if err != nil {
		return err
	}

	body := map[string]any{"enabled": on}
	resp, err := client.put(fmt.Sprintf("/api/v1/flags/%s/rules/%s/environments/%s", flagID, ruleID, envID), body)
	if err != nil {
		return fmt.Errorf("failed to set rule env state: %w", err)
	}
	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	state := "OFF"
	if on {
		state = "ON"
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Rule %s in %s: %s\n", ruleID, envSlug, state)
	return nil
}

// (Helper used in formatting; keep here so Go doesn't complain about unused imports.)
var _ = strconv.Itoa
```

Add a `delete` method to `client.go` if it doesn't exist (mirrors `post`/`put`).

### Step 2: Tests in `flags_rules_test.go`

```go
package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlagsRulesAdd_Percentage(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags?project_id=", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	srv.onPathFunc("POST", "/api/v1/flags/f-1/rules", func(req recordedRequest) (int, any) {
		require.Equal(t, "percentage", req.Body["rule_type"])
		require.Equal(t, float64(25), req.Body["percentage"])
		require.Equal(t, "true", req.Body["value"])
		require.Equal(t, true, req.Body["enabled"])
		return 201, map[string]any{"id": "rule-1"}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	stdout, _, err := runCmd(t, flagsRulesAddCmd, "dark-mode", "--rule-type", "percentage", "--percentage", "25", "--value", "true")
	require.NoError(t, err)
	require.Contains(t, stdout, "rule-1")
}

func TestFlagsRulesAdd_Attribute(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags?project_id=", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	srv.onPathFunc("POST", "/api/v1/flags/f-1/rules", func(req recordedRequest) (int, any) {
		require.Equal(t, "attribute", req.Body["rule_type"])
		require.Equal(t, "plan", req.Body["attribute"])
		require.Equal(t, "eq", req.Body["operator"])
		return 201, map[string]any{"id": "rule-2"}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	_, _, err := runCmd(t, flagsRulesAddCmd, "dark-mode", "--rule-type", "attribute", "--attribute", "plan", "--operator", "eq", "--value", "pro")
	require.NoError(t, err)
}

func TestFlagsRulesList(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags?project_id=", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	srv.onPathFunc("GET", "/api/v1/flags/f-1/rules", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"rules": []map[string]any{
				{"id": "r-1", "rule_type": "percentage", "priority": 10, "value": "true", "enabled": true},
			},
		}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	stdout, _, err := runCmd(t, flagsRulesListCmd, "dark-mode")
	require.NoError(t, err)
	require.Contains(t, stdout, "r-1")
	require.Contains(t, stdout, "percentage")
}

func TestFlagsRulesDelete(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags?project_id=", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	srv.onPathFunc("DELETE", "/api/v1/flags/f-1/rules/r-1", func(recordedRequest) (int, any) {
		return 204, map[string]any{}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	stdout, _, err := runCmd(t, flagsRulesDeleteCmd, "dark-mode", "r-1")
	require.NoError(t, err)
	require.Contains(t, stdout, "deleted")
}

func TestFlagsRulesSetEnvState(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags?project_id=", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	srv.onPathFunc("PUT", "/api/v1/flags/f-1/rules/r-1/environments/env-prod-uuid", func(req recordedRequest) (int, any) {
		require.Equal(t, true, req.Body["enabled"])
		return 200, map[string]any{}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "production")

	stdout, _, err := runCmd(t, flagsRulesEnableEnvCmd, "dark-mode", "r-1", "--on")
	require.NoError(t, err)
	require.Contains(t, stdout, "ON")
}
```

### Step 3: Run + commit

```bash
go test ./cmd/cli/ -run TestFlagsRules -v
git add cmd/cli/flags_rules.go cmd/cli/flags_rules_test.go cmd/cli/client.go
git commit -m "feat(cli): add flags rules subcommands (list/add/update/delete/set-env-state)"
```

---

## Task 9: `flags set-value` (per-environment default value)

**Files:**
- Create: `cmd/cli/flags_envstate.go`
- Create: `cmd/cli/flags_envstate_test.go`

The endpoint `PUT /api/v1/flags/:id/environments/:envId` accepts both `enabled` and `value`. The existing `flags toggle` covers `enabled`. The new `flags set-value` covers `value` (and optionally `enabled` together).

### Step 1: Implement

```go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var flagsSetValueCmd = &cobra.Command{
	Use:   "set-value <flag-key>",
	Short: "Set the per-environment default value for a flag",
	Long: `Set or change the per-environment default value of a flag.

Requires --env (or DS env config). Optionally toggle enabled in the same call
with --enabled / --disabled.

Examples:
  # Change the prod default value of a string flag
  deploysentry flags set-value checkout-variant --env production --value "variant-b"

  # Set a numeric default and enable it in staging in one call
  deploysentry flags set-value rate-limit --env staging --value "100" --enabled`,
	Args: cobra.ExactArgs(1),
	RunE: runFlagsSetValue,
}

func init() {
	flagsSetValueCmd.Flags().String("value", "", "new default value for this environment")
	flagsSetValueCmd.Flags().Bool("enabled", false, "set enabled=true in this env")
	flagsSetValueCmd.Flags().Bool("disabled", false, "set enabled=false in this env")
	flagsCmd.AddCommand(flagsSetValueCmd)
}

func runFlagsSetValue(cmd *cobra.Command, args []string) error {
	client, flagID, _, err := resolveFlagFromArgs(args[0])
	if err != nil {
		return err
	}

	envSlug := getEnv()
	if envSlug == "" {
		return fmt.Errorf("--env (or DS env config) is required for set-value")
	}
	org, _ := requireOrg()
	envID, err := resolveEnvID(client, org, envSlug)
	if err != nil {
		return err
	}

	body := map[string]any{}
	if cmd.Flags().Changed("value") {
		v, _ := cmd.Flags().GetString("value")
		body["value"] = v
	}
	en, _ := cmd.Flags().GetBool("enabled")
	dis, _ := cmd.Flags().GetBool("disabled")
	if en && dis {
		return fmt.Errorf("cannot specify both --enabled and --disabled")
	}
	if en {
		body["enabled"] = true
	}
	if dis {
		body["enabled"] = false
	}
	if len(body) == 0 {
		return fmt.Errorf("nothing to update; use --value, --enabled, or --disabled")
	}

	resp, err := client.put(fmt.Sprintf("/api/v1/flags/%s/environments/%s", flagID, envID), body)
	if err != nil {
		return fmt.Errorf("failed to set env state: %w", err)
	}
	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Flag %q updated in env %q.\n", args[0], envSlug)
	return nil
}
```

### Step 2: Tests

```go
package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlagsSetValue_ValueOnly(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags?project_id=", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "rate-limit"}}}
	})
	srv.onPathFunc("PUT", "/api/v1/flags/f-1/environments/env-prod-uuid", func(req recordedRequest) (int, any) {
		require.Equal(t, "100", req.Body["value"])
		_, hasEnabled := req.Body["enabled"]
		require.False(t, hasEnabled)
		return 200, map[string]any{}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "production")

	_, _, err := runCmd(t, flagsSetValueCmd, "rate-limit", "--value", "100")
	require.NoError(t, err)
}

func TestFlagsSetValue_ValueAndEnabled(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags?project_id=", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "rate-limit"}}}
	})
	srv.onPathFunc("PUT", "/api/v1/flags/f-1/environments/env-prod-uuid", func(req recordedRequest) (int, any) {
		require.Equal(t, "100", req.Body["value"])
		require.Equal(t, true, req.Body["enabled"])
		return 200, map[string]any{}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "production")

	_, _, err := runCmd(t, flagsSetValueCmd, "rate-limit", "--value", "100", "--enabled")
	require.NoError(t, err)
}

func TestFlagsSetValue_RequiresEnv(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	_, _, err := runCmd(t, flagsSetValueCmd, "rate-limit", "--value", "100")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--env")
}

func TestFlagsSetValue_BothEnabledAndDisabled(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "production")

	_, _, err := runCmd(t, flagsSetValueCmd, "rate-limit", "--value", "100", "--enabled", "--disabled")
	require.Error(t, err)
	require.Contains(t, err.Error(), "both --enabled and --disabled")
}
```

### Step 3: Run + commit

```bash
go test ./cmd/cli/ -run TestFlagsSetValue -v
git add cmd/cli/flags_envstate.go cmd/cli/flags_envstate_test.go
git commit -m "feat(cli): flags set-value for per-env default value"
```

---

## Task 10: `flags list` URL fix (was already broken — same drift)

The existing `runFlagsList` POSTs to `/api/v1/orgs/<org>/projects/<project>/flags` (wrong). Fix to `GET /api/v1/flags?project_id=<uuid>`.

### Step 1: Replace `runFlagsList`

```go
func runFlagsList(cmd *cobra.Command, args []string) error {
	_ = args
	org, err := requireOrg()
	if err != nil {
		return err
	}
	projectSlug, err := requireProject()
	if err != nil {
		return err
	}
	client, err := clientFromConfig()
	if err != nil {
		return err
	}
	projectID, err := resolveProjectID(client, org, projectSlug)
	if err != nil {
		return err
	}

	params := []string{"project_id=" + projectID}
	if cat, _ := cmd.Flags().GetString("category"); cat != "" {
		params = append(params, "category="+cat)
	}
	if status, _ := cmd.Flags().GetString("status"); status == "archived" {
		params = append(params, "archived=true")
	}

	resp, err := client.get("/api/v1/flags?" + strings.Join(params, "&"))
	if err != nil {
		return fmt.Errorf("failed to list flags: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	flags, _ := resp["flags"].([]interface{})
	if len(flags) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No feature flags found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "KEY\tTYPE\tCATEGORY\tDEFAULT\tTAGS\tUPDATED")
	for _, f := range flags {
		flag, _ := f.(map[string]interface{})
		key, _ := flag["key"].(string)
		flagType, _ := flag["flag_type"].(string)
		category, _ := flag["category"].(string)
		defaultVal, _ := flag["default_value"].(string)
		updatedAt, _ := flag["updated_at"].(string)

		tagList := ""
		if t, ok := flag["tags"].([]interface{}); ok {
			tagStrs := make([]string, 0, len(t))
			for _, tag := range t {
				if s, ok := tag.(string); ok {
					tagStrs = append(tagStrs, s)
				}
			}
			tagList = strings.Join(tagStrs, ", ")
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			key, flagType, category, defaultVal, tagList, updatedAt)
	}
	return w.Flush()
}
```

In `init()`, add `--category` filter, drop the `--tag`, `--search`, `--limit` filters that the API doesn't support (or leave them but document as no-ops; cleaner to drop).

### Step 2: Append tests

```go
func TestFlagsList_Success(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags?project_id=", func(req recordedRequest) (int, any) {
		require.Contains(t, req.Path, "project_id=proj-uuid")
		return 200, map[string]any{
			"flags": []map[string]any{
				{"key": "dark-mode", "flag_type": "boolean", "category": "feature", "default_value": "false"},
			},
		}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	stdout, _, err := runCmd(t, flagsListCmd)
	require.NoError(t, err)
	require.Contains(t, stdout, "dark-mode")
	require.Contains(t, stdout, "boolean")
}

func TestFlagsList_FilterByCategory(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags?project_id=", func(req recordedRequest) (int, any) {
		require.Contains(t, req.Path, "category=release")
		return 200, map[string]any{"flags": []map[string]any{}}
	})
	setTestConfig(t, srv.URL(), "tok", "acme", "payments", "")

	_, _, err := runCmd(t, flagsListCmd, "--category", "release")
	require.NoError(t, err)
}
```

### Step 3: Run + commit

```bash
go test ./cmd/cli/ -run TestFlagsList -v
git add cmd/cli/flags.go cmd/cli/flags_test.go
git commit -m "fix(cli): flags list — correct URL + category filter"
```

---

## Task 11: Final verification + push

### Step 1: Full suite

```bash
cd .worktrees/cli-flags-fix
go vet ./cmd/cli/...
go test ./cmd/cli/... -v -count=1
go build ./...
```

Expected: vet clean, all tests pass (~22 new tests), build clean.

### Step 2: Update `docs/Current_Initiatives.md`

Add a new row near the bottom:

```
| CLI Flag Flow Fix + Tests | Implementation | [Plan](./superpowers/plans/2026-04-27-cli-flag-flow-fix-and-tests.md) | All `flags` subcommands rewritten to match the real API: correct URLs, correct body shapes, slug→UUID resolvers for project/env/flag, env-scoped toggles + per-env value setting, full targeting-rule CRUD via `flags rules`. New httptest harness in `cmd/cli/cli_test_helpers.go` covers ~22 cases across create/get/update/toggle/archive/evaluate/list/rules/set-value. Phase B (other CLI verbs) deferred. |
```

Update `> Last updated:` to `2026-04-27`.

### Step 3: Push

```bash
git add docs/Current_Initiatives.md
git commit -m "docs: track CLI flag flow fix initiative"
git push -u origin fix/cli-flags-and-tests
```

### Step 4: Report

```bash
git log --oneline main..fix/cli-flags-and-tests
```

---

## Success criteria

- ✅ `go vet ./cmd/cli/...` — clean
- ✅ `go test ./cmd/cli/... -count=1` — ~22 tests pass
- ✅ `go build ./...` — clean
- ✅ Each flag command's request shape matches the real API (verified by mock server assertions)
- ✅ Targeting rules can be created (TestFlagsRulesAdd_Percentage / Attribute), listed, updated, deleted, and per-env-toggled
- ✅ Per-environment default values can be set (TestFlagsSetValue_*)

## Out of scope

- Phase B: other CLI verbs (apps, projects, deploy, releases, rollouts, strategies, settings, apikeys, orgs, members, webhooks, integrations, mcp, analytics, environments, auth, config). Same drift likely exists for many of those — separate initiative.
- An end-to-end test against a running API + real DB. The httptest mocks cover the request shape; an integration smoke is a follow-up.
- Schedule / segment / compound rule types. Only percentage / attribute / user_target are covered by tests in this PR; the others will land alongside their UI work in mobile-pwa Phase 5.
