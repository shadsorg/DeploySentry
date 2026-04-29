package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// resetFlagsSetValueFlags resets cobra flag state to avoid bleed across tests.
func resetFlagsSetValueFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"value", "enabled", "disabled"} {
			if f := flagsSetValueCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

func TestFlagsSetValue_ValueOnly(t *testing.T) {
	resetFlagsSetValueFlags(t)
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("PUT", "/api/v1/flags/f-1/environments/env-prod-uuid", func(req recordedRequest) (int, any) {
		require.Equal(t, "100", req.Body["value"])
		_, hasEnabled := req.Body["enabled"]
		require.False(t, hasEnabled)
		return 200, map[string]any{}
	})
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "rate-limit"}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	_, _, err := runCmd(t, rootCmd, "flags", "set-value", "rate-limit", "--value", "100")
	require.NoError(t, err)
}

func TestFlagsSetValue_ValueAndEnabled(t *testing.T) {
	resetFlagsSetValueFlags(t)
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("PUT", "/api/v1/flags/f-1/environments/env-prod-uuid", func(req recordedRequest) (int, any) {
		require.Equal(t, "100", req.Body["value"])
		require.Equal(t, true, req.Body["enabled"])
		return 200, map[string]any{}
	})
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "rate-limit"}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	_, _, err := runCmd(t, rootCmd, "flags", "set-value", "rate-limit", "--value", "100", "--enabled")
	require.NoError(t, err)
}

func TestFlagsSetValue_RequiresEnv(t *testing.T) {
	resetFlagsSetValueFlags(t)
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "rate-limit"}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	_, _, err := runCmd(t, rootCmd, "flags", "set-value", "rate-limit", "--value", "100")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--env")
}

func TestFlagsSetValue_BothEnabledAndDisabled(t *testing.T) {
	resetFlagsSetValueFlags(t)
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "rate-limit"}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	_, _, err := runCmd(t, rootCmd, "flags", "set-value", "rate-limit", "--value", "100", "--enabled", "--disabled")
	require.Error(t, err)
	require.Contains(t, err.Error(), "both --enabled and --disabled")
}
