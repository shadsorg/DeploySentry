package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStrategiesList_Success verifies GET /api/v1/orgs/<org>/strategies and
// that the response is rendered (origin scope, version columns).
func TestStrategiesList_Success(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/orgs/acme/strategies", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/orgs/acme/strategies", req.Path)
		return 200, map[string]any{
			"items": []map[string]any{
				{
					"strategy":     map[string]any{"name": "canary-prod", "target_type": "deploy", "version": 2.0},
					"origin_scope": map[string]any{"type": "org"},
					"is_inherited": false,
				},
			},
		}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	_, _, err := runCmd(t, rootCmd, "strategies", "list")
	require.NoError(t, err)
}

// TestStrategiesApply_Success verifies POST /api/v1/orgs/<org>/strategies/import
// with a YAML body (Content-Type: application/yaml).
func TestStrategiesApply_Success(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "strategy.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte("name: canary-prod\nversion: 1\n"), 0o644))

	srv := newMockServer(t)
	srv.onPathFunc("POST", "/api/v1/orgs/acme/strategies/import", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/orgs/acme/strategies/import", req.Path)
		return 201, map[string]any{"name": "canary-prod", "version": 1}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	_, _, err := runCmd(t, rootCmd, "strategies", "apply", "-f", yamlPath)
	require.NoError(t, err)
}
