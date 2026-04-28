package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// resetAppsCreateFlags clears cobra flag state on appsCreateCmd between tests
// so required flags from a prior test don't bleed into the next invocation.
func resetAppsCreateFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"name", "slug", "description", "repo"} {
			if f := appsCreateCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

func TestAppsCreate_Success(t *testing.T) {
	resetAppsCreateFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("POST", "/api/v1/orgs/acme/projects/payments/apps", func(req recordedRequest) (int, any) {
		require.Equal(t, "API", req.Body["name"])
		require.Equal(t, "api", req.Body["slug"])
		return 201, map[string]any{"id": "app-1", "name": "API", "slug": "api"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "apps", "create", "--name", "API", "--slug", "api")
	require.NoError(t, err)
	require.NotEmpty(t, stdout)
}

func TestAppsList_Success(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/orgs/acme/projects/payments/apps", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"applications": []map[string]any{
				{"id": "a1", "slug": "api", "name": "API"},
			},
		}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "apps", "list")
	require.NoError(t, err)
	require.Contains(t, stdout, "api")
}

func TestAppsGet_Success(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/orgs/acme/projects/payments/apps/api", func(recordedRequest) (int, any) {
		return 200, map[string]any{"id": "a1", "slug": "api", "name": "API"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "apps", "get", "api")
	require.NoError(t, err)
	require.Contains(t, stdout, "api")
}
