package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// resetDeployCreateFlags clears cobra flag state on deployCreateCmd so a
// required flag set in one test doesn't bleed into the next.
func resetDeployCreateFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"app", "version", "artifact", "env", "strategy", "commit-sha", "mode", "source", "apply-immediately"} {
			if f := deployCreateCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

func resetDeployListFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"app", "env", "limit"} {
			if f := deployListCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

// stubAppLookup stubs the GET that resolveAppID issues. It must be called
// BEFORE any stub that registers a more-general /api/v1/orgs/acme/projects
// prefix, since the mock server matches by registration order.
func stubAppLookup(t *testing.T, srv *mockServer, projectSlug, appSlug, appID string) {
	t.Helper()
	srv.onPathFunc("GET", "/api/v1/orgs/acme/projects/"+projectSlug+"/apps/"+appSlug, func(recordedRequest) (int, any) {
		return 200, map[string]any{"id": appID, "slug": appSlug}
	})
}

// stubEnvList stubs the GET that resolveEnvID issues to list org environments.
func stubEnvList(t *testing.T, srv *mockServer, envSlug, envID string) {
	t.Helper()
	srv.onPathFunc("GET", "/api/v1/orgs/acme/environments", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"environments": []map[string]any{
				{"id": envID, "slug": envSlug, "name": envSlug},
			},
		}
	})
}

// TestDeployCreate_Success exercises the rolling-strategy happy path. It must
// resolve --app and --env to UUIDs and POST to the flat /api/v1/deployments
// route with the real createDeploymentRequest body shape.
func TestDeployCreate_Success(t *testing.T) {
	resetDeployCreateFlags(t)
	srv := newMockServer(t)
	// Register specific routes first so prefix matching prefers them.
	stubAppLookup(t, srv, "payments", "api", "app-uuid")
	stubEnvList(t, srv, "production", "env-prod-uuid")

	srv.onPathFunc("POST", "/api/v1/deployments", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/deployments", req.Path)
		require.Equal(t, "app-uuid", req.Body["application_id"])
		require.Equal(t, "env-prod-uuid", req.Body["environment_id"])
		require.Equal(t, "v1.2.3", req.Body["version"])
		require.Equal(t, "ghcr.io/acme/api:v1.2.3", req.Body["artifact"])
		require.Equal(t, "rolling", req.Body["strategy"])
		_, hasRollout := req.Body["rollout"]
		require.False(t, hasRollout, "no rollout sub-map for plain rolling strategy")
		return 201, map[string]any{"id": "deploy-1", "status": "pending"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	stdout, _, err := runCmd(t, rootCmd, "deploy", "create",
		"--app", "api",
		"--version", "v1.2.3",
		"--artifact", "ghcr.io/acme/api:v1.2.3",
		"--strategy", "rolling",
	)
	require.NoError(t, err)
	require.Contains(t, stdout, "deploy-1")
	require.Contains(t, stdout, "v1.2.3")
}

// TestDeployStatus_Success covers `deploy status <uuid>` against the flat
// /api/v1/deployments/:id route. The handler returns the Deployment model
// directly (version / application_id / environment_id / traffic_percent),
// not the made-up shape the old CLI assumed.
func TestDeployStatus_Success(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/deployments/deploy-1", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"id":              "deploy-1",
			"status":          "running",
			"version":         "v1.2.3",
			"application_id":  "app-uuid",
			"environment_id":  "env-prod-uuid",
			"strategy":        "rolling",
			"traffic_percent": float64(50),
			"created_at":      "2026-04-24T00:00:00Z",
		}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	stdout, _, err := runCmd(t, rootCmd, "deploy", "status", "deploy-1")
	require.NoError(t, err)
	require.Contains(t, stdout, "deploy-1")
	require.Contains(t, stdout, "running")
	require.Contains(t, stdout, "v1.2.3")
	require.Contains(t, stdout, "50%")
}

// TestDeployList_Success covers `deploy list --app api`. The CLI must resolve
// the app slug to a UUID and pass it as app_id in the query string.
func TestDeployList_Success(t *testing.T) {
	resetDeployListFlags(t)
	srv := newMockServer(t)
	stubAppLookup(t, srv, "payments", "api", "app-uuid")

	srv.onPathFunc("GET", "/api/v1/deployments", func(req recordedRequest) (int, any) {
		require.Contains(t, req.Path, "app_id=app-uuid")
		return 200, map[string]any{
			"deployments": []map[string]any{
				{
					"id":              "deploy-1",
					"version":         "v1.2.3",
					"strategy":        "rolling",
					"status":          "completed",
					"traffic_percent": float64(100),
					"created_at":      "2026-04-24T00:00:00Z",
				},
			},
		}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "deploy", "list", "--app", "api")
	require.NoError(t, err)
	require.Contains(t, stdout, "deploy-1")
	require.Contains(t, stdout, "v1.2.3")
}

// TestDeployPromote_Success covers `deploy promote <uuid>` against the flat
// /api/v1/deployments/:id/promote route.
func TestDeployPromote_Success(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("POST", "/api/v1/deployments/deploy-1/promote", func(recordedRequest) (int, any) {
		return 200, map[string]any{"status": "promoting"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "deploy", "promote", "deploy-1")
	require.NoError(t, err)
	require.Contains(t, stdout, "promoted")
	require.Contains(t, stdout, "promoting")
}
