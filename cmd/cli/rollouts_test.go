package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// resetRolloutsListFlags clears cobra flag state on rolloutsListCmd between
// tests.
func resetRolloutsListFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		if f := rolloutsListCmd.Flags().Lookup("status"); f != nil {
			f.Changed = false
			_ = f.Value.Set(f.DefValue)
		}
	})
}

// TestRolloutsList_Success verifies GET /api/v1/orgs/<org>/rollouts.
func TestRolloutsList_Success(t *testing.T) {
	resetRolloutsListFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/orgs/acme/rollouts", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/orgs/acme/rollouts", req.Path)
		return 200, map[string]any{"rollouts": []map[string]any{{"id": "ro-1", "status": "active"}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	_, _, err := runCmd(t, rootCmd, "rollouts", "list")
	require.NoError(t, err)
}

// TestRolloutsList_StatusFilter verifies the --status flag is appended as a
// query string.
func TestRolloutsList_StatusFilter(t *testing.T) {
	resetRolloutsListFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/orgs/acme/rollouts", func(req recordedRequest) (int, any) {
		require.Contains(t, req.Path, "status=paused")
		return 200, map[string]any{"rollouts": []map[string]any{}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	_, _, err := runCmd(t, rootCmd, "rollouts", "list", "--status", "paused")
	require.NoError(t, err)
}

// TestRolloutsGet_Success verifies GET /api/v1/orgs/<org>/rollouts/<id>.
func TestRolloutsGet_Success(t *testing.T) {
	srv := newMockServer(t)
	// More-specific path first.
	srv.onPathFunc("GET", "/api/v1/orgs/acme/rollouts/ro-1", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/orgs/acme/rollouts/ro-1", req.Path)
		return 200, map[string]any{"id": "ro-1", "status": "active"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	_, _, err := runCmd(t, rootCmd, "rollouts", "get", "ro-1")
	require.NoError(t, err)
}

// TestRolloutsPause_Success verifies POST /api/v1/orgs/<org>/rollouts/<id>/pause.
// The pause action does not require --reason.
func TestRolloutsPause_Success(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("POST", "/api/v1/orgs/acme/rollouts/ro-1/pause", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/orgs/acme/rollouts/ro-1/pause", req.Path)
		return 200, map[string]any{"id": "ro-1", "status": "paused"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	_, _, err := runCmd(t, rootCmd, "rollouts", "pause", "ro-1")
	require.NoError(t, err)
}
