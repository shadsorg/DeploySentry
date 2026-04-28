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
		for _, name := range []string{"name", "description", "repo", "environments"} {
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
// /api/v1/orgs/<org>/projects with a `name` field.
//
// NOTE: The CLI also sends `repository_url` and `environments`, while the
// server expects `repo_url` and a required `slug`. That's drift the audit
// missed; this test asserts only the URL + name to lock down the
// happy-path contract without papering over the body-shape drift. The
// drift is captured in the Phase B report for follow-up.
func TestProjectsCreate_Success(t *testing.T) {
	resetProjectsCreateFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("POST", "/api/v1/orgs/acme/projects", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/orgs/acme/projects", req.Path)
		require.Equal(t, "my-api", req.Body["name"])
		return 201, map[string]any{"id": "p-1", "slug": "my-api", "name": "my-api"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	stdout, _, err := runCmd(t, rootCmd, "projects", "create", "--name", "my-api")
	require.NoError(t, err)
	require.Contains(t, stdout, "p-1")
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
