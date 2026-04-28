package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// resetRolloutGroupsCreateFlags clears cobra flag state on
// rolloutGroupsCreateCmd between tests.
func resetRolloutGroupsCreateFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"name", "description", "policy"} {
			if f := rolloutGroupsCreateCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

func resetRolloutGroupsAttachFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		if f := rolloutGroupsAttachCmd.Flags().Lookup("rollout"); f != nil {
			f.Changed = false
			_ = f.Value.Set(f.DefValue)
		}
	})
}

// TestRolloutGroupsList_Success verifies GET /api/v1/orgs/<org>/rollout-groups.
// The list command uses getRaw + fmt.Println so we can't assert on stdout
// via runCmd (cobra's SetOut buffer); we verify the URL via the mock server.
func TestRolloutGroupsList_Success(t *testing.T) {
	srv := newMockServer(t)
	called := false
	srv.onPathFunc("GET", "/api/v1/orgs/acme/rollout-groups", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/orgs/acme/rollout-groups", req.Path)
		called = true
		return 200, map[string]any{"groups": []map[string]any{{"id": "g-1", "name": "Bundle A"}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	_, _, err := runCmd(t, rootCmd, "rollout-groups", "list")
	require.NoError(t, err)
	require.True(t, called, "expected GET /api/v1/orgs/acme/rollout-groups to be called")
}

// TestRolloutGroupsCreate_Success verifies POST
// /api/v1/orgs/<org>/rollout-groups with a JSON body containing name and
// optional coordination_policy.
func TestRolloutGroupsCreate_Success(t *testing.T) {
	resetRolloutGroupsCreateFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("POST", "/api/v1/orgs/acme/rollout-groups", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/orgs/acme/rollout-groups", req.Path)
		require.Equal(t, "Holiday Bundle", req.Body["name"])
		require.Equal(t, "cascade_abort", req.Body["coordination_policy"])
		return 201, map[string]any{"id": "g-new", "name": "Holiday Bundle"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	_, _, err := runCmd(t, rootCmd, "rollout-groups", "create",
		"--name", "Holiday Bundle",
		"--policy", "cascade_abort",
	)
	require.NoError(t, err)
}

// TestRolloutGroupsAttach_Success verifies POST
// /api/v1/orgs/<org>/rollout-groups/<id>/attach with the rollout_id in the
// body.
func TestRolloutGroupsAttach_Success(t *testing.T) {
	resetRolloutGroupsAttachFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("POST", "/api/v1/orgs/acme/rollout-groups/g-1/attach", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/orgs/acme/rollout-groups/g-1/attach", req.Path)
		require.Equal(t, "ro-42", req.Body["rollout_id"])
		return 200, map[string]any{"status": "attached"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	_, _, err := runCmd(t, rootCmd, "rollout-groups", "attach", "g-1",
		"--rollout", "ro-42",
	)
	require.NoError(t, err)
}
