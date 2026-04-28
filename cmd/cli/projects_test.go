package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// resetProjectsCreateFlags clears cobra flag state on projectsCreateCmd between
// tests.
func resetProjectsCreateFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"name", "slug", "description", "repo"} {
			if f := projectsCreateCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

func resetProjectsConfigFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"default-strategy", "repo", "default-env"} {
			if f := projectsConfigCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

// TestProjectsCreate_Success verifies the CLI POSTs to
// /api/v1/orgs/<org>/projects with the body shape the server's
// createProject handler binds: name, slug (auto-derived from name when not
// supplied), and repo_url (NOT repository_url). environments must NOT be
// sent — they are managed at the org level.
func TestProjectsCreate_Success(t *testing.T) {
	resetProjectsCreateFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("POST", "/api/v1/orgs/acme/projects", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/orgs/acme/projects", req.Path)
		require.Equal(t, "my-api", req.Body["name"])
		// Auto-derived slug: lowercase + non-alnum -> dash. "my-api" stays "my-api".
		require.Equal(t, "my-api", req.Body["slug"])
		require.Equal(t, "https://github.com/acme/my-api", req.Body["repo_url"])
		_, hasEnvironments := req.Body["environments"]
		require.False(t, hasEnvironments, "environments must not be sent on project create (org-level concept)")
		_, hasRepositoryURL := req.Body["repository_url"]
		require.False(t, hasRepositoryURL, "repository_url must be renamed to repo_url")
		return 201, map[string]any{"id": "p-1", "slug": "my-api", "name": "my-api"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	stdout, _, err := runCmd(t, rootCmd, "projects", "create",
		"--name", "my-api",
		"--repo", "https://github.com/acme/my-api",
	)
	require.NoError(t, err)
	require.Contains(t, stdout, "p-1")
}

// TestProjectsCreate_ExplicitSlug verifies that an explicit --slug is sent
// verbatim (instead of being derived from --name) and that repo_url uses the
// server-side field name.
func TestProjectsCreate_ExplicitSlug(t *testing.T) {
	resetProjectsCreateFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("POST", "/api/v1/orgs/acme/projects", func(req recordedRequest) (int, any) {
		require.Equal(t, "Payments Stack", req.Body["name"])
		require.Equal(t, "pay-stack", req.Body["slug"])
		require.Equal(t, "https://github.com/acme/pay", req.Body["repo_url"])
		return 201, map[string]any{"id": "p-1", "slug": "pay-stack"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	_, _, err := runCmd(t, rootCmd, "projects", "create",
		"--name", "Payments Stack",
		"--slug", "pay-stack",
		"--repo", "https://github.com/acme/pay",
	)
	require.NoError(t, err)
}

// TestProjectsList_Success verifies GET /api/v1/orgs/<org>/projects.
func TestProjectsList_Success(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/orgs/acme/projects", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/orgs/acme/projects", req.Path)
		return 200, map[string]any{
			"projects": []map[string]any{
				{"id": "p-1", "slug": "my-api", "name": "my-api", "description": "Backend API", "environments": []string{"dev", "prod"}},
			},
		}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	stdout, _, err := runCmd(t, rootCmd, "projects", "list")
	require.NoError(t, err)
	require.Contains(t, stdout, "my-api")
}

// TestProjectsConfig_Get verifies GET /api/v1/orgs/<org>/projects/<proj>/config
// when no update flags are passed.
func TestProjectsConfig_Get(t *testing.T) {
	resetProjectsConfigFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/orgs/acme/projects/payments/config", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/orgs/acme/projects/payments/config", req.Path)
		return 200, map[string]any{
			"default_strategy": "canary",
			"repository_url":   "https://github.com/acme/payments",
		}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "projects", "config")
	require.NoError(t, err)
	require.Contains(t, stdout, "payments")
	require.Contains(t, stdout, "canary")
}
