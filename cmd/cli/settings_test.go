package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// resetSettingsListFlags clears cobra flag state on settingsListCmd between tests.
func resetSettingsListFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"scope", "target"} {
			if f := settingsListCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

func resetSettingsSetFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"scope", "target", "key", "value"} {
			if f := settingsSetCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

// TestSettingsList_Success verifies GET /api/v1/settings?scope=&target=.
func TestSettingsList_Success(t *testing.T) {
	resetSettingsListFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/settings", func(req recordedRequest) (int, any) {
		require.Contains(t, req.Path, "scope=project")
		require.Contains(t, req.Path, "target=p-1")
		return 200, map[string]any{
			"settings": []map[string]any{
				{"key": "rollout.strategy", "value": "canary", "updated_by": "u1", "updated_at": "2025-01-01"},
			},
		}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	stdout, _, err := runCmd(t, rootCmd, "settings", "list", "--scope", "project", "--target", "p-1")
	require.NoError(t, err)
	require.Contains(t, stdout, "rollout.strategy")
	require.Contains(t, stdout, "canary")
}

// TestSettingsSet_Success verifies PUT /api/v1/settings with the upsert body.
//
// NOTE: The CLI sends the value as a plain string, but the server's
// setSettingRequest binds Value as json.RawMessage. That's a body-shape
// mismatch the audit missed; this test asserts only that the CLI uses the
// right URL + method + scope/target/key fields. The mismatch is captured
// in the Phase B report for follow-up.
func TestSettingsSet_Success(t *testing.T) {
	resetSettingsSetFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("PUT", "/api/v1/settings", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/settings", req.Path)
		require.Equal(t, "project", req.Body["scope"])
		require.Equal(t, "p-1", req.Body["target_id"])
		require.Equal(t, "rollout.strategy", req.Body["key"])
		require.Equal(t, "canary", req.Body["value"])
		return 200, map[string]any{"id": "s-1"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	stdout, _, err := runCmd(t, rootCmd, "settings", "set",
		"--scope", "project",
		"--target", "p-1",
		"--key", "rollout.strategy",
		"--value", "canary",
	)
	require.NoError(t, err)
	require.Contains(t, stdout, "Setting updated")
}

// TestSettingsDelete_Success verifies DELETE /api/v1/settings/<id>.
func TestSettingsDelete_Success(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("DELETE", "/api/v1/settings/s-1", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/settings/s-1", req.Path)
		return 200, map[string]any{"status": "deleted"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	stdout, _, err := runCmd(t, rootCmd, "settings", "delete", "s-1")
	require.NoError(t, err)
	require.Contains(t, stdout, "s-1 deleted")
}
