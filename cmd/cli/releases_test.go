package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// resetReleasesFlags clears persistent cobra flag state across release tests.
// The cobra Command tree is process-global, so flag values bleed across
// invocations within a single `go test` run; reset what each subcommand owns.
func resetReleasesFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		resets := []struct {
			cmd   *cobra.Command
			names []string
		}{
			{releasesCreateCmd, []string{"app", "name", "description", "session-sticky", "sticky-header"}},
			{releasesListCmd, []string{"app"}},
			{releasesGetCmd, []string{"app"}},
			{releasesPromoteCmd, []string{"app", "traffic-percent"}},
		}
		for _, r := range resets {
			for _, name := range r.names {
				if f := r.cmd.Flags().Lookup(name); f != nil {
					f.Changed = false
					_ = f.Value.Set(f.DefValue)
				}
			}
		}
	})
}

// stubProjectAppEnv stubs the project, app, and env resolvers used by release tests.
// Releases don't actually use env, but the env stub is registered for parity
// with the broader test harness and is harmless when unused.
func stubProjectAppEnv(t *testing.T, srv *mockServer, projectID, appID, envID string) {
	t.Helper()
	// More-specific app route registered before the project-only route so it wins
	// the path-prefix match.
	srv.onPathFunc("GET", "/api/v1/orgs/acme/projects/payments/apps/api", func(recordedRequest) (int, any) {
		return 200, map[string]any{"id": appID, "slug": "api", "name": "API"}
	})
	stubProjectAndEnv(t, srv, projectID, envID)
}

func TestReleasesCreate_Success(t *testing.T) {
	resetReleasesFlags(t)
	srv := newMockServer(t)
	stubProjectAppEnv(t, srv, "proj-uuid", "app-uuid", "env-prod-uuid")
	srv.onPathFunc("POST", "/api/v1/applications/app-uuid/releases", func(req recordedRequest) (int, any) {
		require.Equal(t, "v1.2.3", req.Body["name"])
		return 201, map[string]any{"id": "rel-1", "name": "v1.2.3", "status": "draft"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "releases", "create", "--app", "api", "--name", "v1.2.3")
	require.NoError(t, err)
	require.Contains(t, stdout, "v1.2.3")
}

func TestReleasesList_Success(t *testing.T) {
	resetReleasesFlags(t)
	srv := newMockServer(t)
	stubProjectAppEnv(t, srv, "proj-uuid", "app-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/applications/app-uuid/releases", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"releases": []map[string]any{
				{"id": "rel-1", "name": "v1.2.3", "status": "completed", "traffic_percent": float64(100)},
			},
		}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "releases", "list", "--app", "api")
	require.NoError(t, err)
	require.Contains(t, stdout, "v1.2.3")
}

func TestReleasesGet_Success(t *testing.T) {
	resetReleasesFlags(t)
	srv := newMockServer(t)
	stubProjectAppEnv(t, srv, "proj-uuid", "app-uuid", "env-prod-uuid")
	// Get resolves name -> UUID via the list endpoint, then GETs /releases/:id.
	srv.onPathFunc("GET", "/api/v1/applications/app-uuid/releases", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"releases": []map[string]any{
				{"id": "rel-1", "name": "v1.2.3"},
			},
		}
	})
	srv.onPathFunc("GET", "/api/v1/releases/rel-1", func(recordedRequest) (int, any) {
		return 200, map[string]any{"id": "rel-1", "name": "v1.2.3", "status": "completed", "traffic_percent": float64(100)}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "releases", "get", "v1.2.3", "--app", "api")
	require.NoError(t, err)
	require.Contains(t, stdout, "v1.2.3")
}

func TestReleasesPromote_Success(t *testing.T) {
	resetReleasesFlags(t)
	srv := newMockServer(t)
	stubProjectAppEnv(t, srv, "proj-uuid", "app-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/applications/app-uuid/releases", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"releases": []map[string]any{
				{"id": "rel-1", "name": "v1.2.3"},
			},
		}
	})
	srv.onPathFunc("POST", "/api/v1/releases/rel-1/promote", func(req recordedRequest) (int, any) {
		require.Equal(t, float64(50), req.Body["traffic_percent"])
		return 200, map[string]any{"status": "promoted"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	_, _, err := runCmd(t, rootCmd, "releases", "promote", "v1.2.3", "--app", "api", "--traffic-percent", "50")
	require.NoError(t, err)
}
