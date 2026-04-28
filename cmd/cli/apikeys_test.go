package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// resetAPIKeysCreateFlags clears cobra flag state on apikeysCreateCmd between
// tests so required flags from a prior test don't bleed into the next invocation.
func resetAPIKeysCreateFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"name", "scopes", "env", "project", "app"} {
			if f := apikeysCreateCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

// TestAPIKeysList_Success verifies that `apikeys list` GETs /api/v1/api-keys.
// The api-keys routes live on the org-scoped api group at the API level but
// the CLI sends them without an /orgs/<slug>/ prefix because the server
// resolves the org from the authenticated principal.
func TestAPIKeysList_Success(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/api-keys", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/api-keys", req.Path)
		return 200, map[string]any{
			"api_keys": []map[string]any{
				{"id": "k-1", "name": "CI", "prefix": "ds_abcd", "scopes": []string{"flags:read"}},
			},
		}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	stdout, _, err := runCmd(t, rootCmd, "apikeys", "list")
	require.NoError(t, err)
	require.Contains(t, stdout, "k-1")
	require.Contains(t, stdout, "CI")
}

// TestAPIKeysCreate_Success verifies POST /api/v1/api-keys carries the name
// and scopes as a string slice, with no org prefix on the URL.
func TestAPIKeysCreate_Success(t *testing.T) {
	resetAPIKeysCreateFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("POST", "/api/v1/api-keys", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/api-keys", req.Path)
		require.Equal(t, "CI Pipeline", req.Body["name"])
		scopes, _ := req.Body["scopes"].([]any)
		require.ElementsMatch(t, []any{"flags:read", "deploys:write"}, scopes)
		// No --project / --app / --env passed: those keys must be absent.
		_, hasProject := req.Body["project_id"]
		require.False(t, hasProject)
		_, hasApp := req.Body["application_id"]
		require.False(t, hasApp)
		_, hasEnv := req.Body["environment_ids"]
		require.False(t, hasEnv)
		return 201, map[string]any{
			"id":    "k-new",
			"token": "ds_secrettoken",
		}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	stdout, _, err := runCmd(t, rootCmd, "apikeys", "create",
		"--name", "CI Pipeline",
		"--scopes", "flags:read,deploys:write",
	)
	require.NoError(t, err)
	require.Contains(t, stdout, "k-new")
	require.Contains(t, stdout, "ds_secrettoken")
}

// TestAPIKeysRevoke_Success verifies DELETE /api/v1/api-keys/<id>.
func TestAPIKeysRevoke_Success(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("DELETE", "/api/v1/api-keys/k-123", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/api-keys/k-123", req.Path)
		return 200, map[string]any{"status": "revoked"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	stdout, _, err := runCmd(t, rootCmd, "apikeys", "revoke", "k-123")
	require.NoError(t, err)
	require.Contains(t, stdout, "k-123 revoked")
}
