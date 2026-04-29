package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// resetIntegrationsCreateFlags clears cobra flag state between tests.
func resetIntegrationsCreateFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"app", "provider", "webhook-secret", "auth-mode", "provider-config", "env-mapping", "version-extractor"} {
			if f := integrationsDeployCreateCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

func resetIntegrationsListFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		if f := integrationsDeployListCmd.Flags().Lookup("app"); f != nil {
			f.Changed = false
			_ = f.Value.Set(f.DefValue)
		}
	})
}

// TestIntegrationsDeployCreate_Success verifies the CLI POSTs to
// /api/v1/integrations/deploys with the right body shape, including resolved
// app UUID and env-mapping with environment UUIDs.
func TestIntegrationsDeployCreate_Success(t *testing.T) {
	resetIntegrationsCreateFlags(t)
	srv := newMockServer(t)
	// Resolve app slug → UUID: project lookup, then app lookup.
	srv.onPathFunc("GET", "/api/v1/orgs/acme/projects/payments/apps/api-server", func(recordedRequest) (int, any) {
		return 200, map[string]any{"id": "app-uuid", "slug": "api-server", "name": "API Server"}
	})
	srv.onPathFunc("GET", "/api/v1/orgs/acme/projects/payments", func(recordedRequest) (int, any) {
		return 200, map[string]any{"id": "proj-uuid", "slug": "payments"}
	})
	srv.onPathFunc("GET", "/api/v1/orgs/acme/environments", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"environments": []map[string]any{
				{"id": "env-prod-uuid", "slug": "production"},
				{"id": "env-stg-uuid", "slug": "staging"},
			},
		}
	})
	srv.onPathFunc("POST", "/api/v1/integrations/deploys", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/integrations/deploys", req.Path)
		require.Equal(t, "app-uuid", req.Body["application_id"])
		require.Equal(t, "railway", req.Body["provider"])
		require.Equal(t, "hmac", req.Body["auth_mode"])
		require.Equal(t, "topsecret", req.Body["webhook_secret"])
		envMap, _ := req.Body["env_mapping"].(map[string]any)
		require.Equal(t, "env-prod-uuid", envMap["production"])
		return 201, map[string]any{"id": "intg-1", "provider": "railway"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "integrations", "deploy", "create",
		"--app", "api-server",
		"--provider", "railway",
		"--webhook-secret", "topsecret",
		"--provider-config", `{"service_id":"svc-1"}`,
		"--env-mapping", "production=production",
	)
	require.NoError(t, err)
	require.Contains(t, stdout, "intg-1")
}

// TestIntegrationsDeployList_Success verifies GET /api/v1/integrations/deploys
// with the application_id query parameter.
func TestIntegrationsDeployList_Success(t *testing.T) {
	resetIntegrationsListFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/orgs/acme/projects/payments/apps/api-server", func(recordedRequest) (int, any) {
		return 200, map[string]any{"id": "app-uuid", "slug": "api-server"}
	})
	srv.onPathFunc("GET", "/api/v1/orgs/acme/projects/payments", func(recordedRequest) (int, any) {
		return 200, map[string]any{"id": "proj-uuid", "slug": "payments"}
	})
	srv.onPathFunc("GET", "/api/v1/integrations/deploys", func(req recordedRequest) (int, any) {
		require.Contains(t, req.Path, "application_id=app-uuid")
		return 200, map[string]any{
			"integrations": []map[string]any{
				{"id": "intg-1", "provider": "railway", "auth_mode": "hmac", "enabled": true, "created_at": "2025-01-01"},
			},
		}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "integrations", "deploy", "list", "--app", "api-server")
	require.NoError(t, err)
	require.Contains(t, stdout, "intg-1")
	require.Contains(t, stdout, "railway")
}

// TestIntegrationsDeployDelete_Success verifies DELETE /api/v1/integrations/deploys/<id>.
func TestIntegrationsDeployDelete_Success(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("DELETE", "/api/v1/integrations/deploys/intg-1", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/integrations/deploys/intg-1", req.Path)
		return 204, nil
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "integrations", "deploy", "delete", "intg-1")
	require.NoError(t, err)
	require.Contains(t, stdout, "intg-1 deleted")
}
