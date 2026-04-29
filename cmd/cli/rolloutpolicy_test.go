package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// resetRolloutPolicySetFlags clears cobra flag state on rolloutPolicySetCmd
// between tests.
func resetRolloutPolicySetFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"policy", "enabled", "env", "target"} {
			if f := rolloutPolicySetCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

// TestRolloutPolicyGet_OrgScope verifies GET
// /api/v1/orgs/<org>/rollout-policy when only --org is set.
func TestRolloutPolicyGet_OrgScope(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/orgs/acme/rollout-policy", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/orgs/acme/rollout-policy", req.Path)
		return 200, map[string]any{"items": []map[string]any{{"policy": "off", "enabled": false}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	_, _, err := runCmd(t, rootCmd, "rollout-policy", "get")
	require.NoError(t, err)
}

// TestRolloutPolicySet_ProjectScope verifies PUT
// /api/v1/orgs/<org>/projects/<proj>/rollout-policy with the upsert body.
func TestRolloutPolicySet_ProjectScope(t *testing.T) {
	resetRolloutPolicySetFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("PUT", "/api/v1/orgs/acme/projects/payments/rollout-policy", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/orgs/acme/projects/payments/rollout-policy", req.Path)
		require.Equal(t, "mandate", req.Body["policy"])
		require.Equal(t, true, req.Body["enabled"])
		require.Equal(t, "production", req.Body["environment"])
		require.Equal(t, "deploy", req.Body["target_type"])
		return 200, map[string]any{"status": "ok"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	_, _, err := runCmd(t, rootCmd, "rollout-policy", "set",
		"--policy", "mandate",
		"--enabled", "true",
		"--env", "production",
		"--target", "deploy",
	)
	require.NoError(t, err)
}
