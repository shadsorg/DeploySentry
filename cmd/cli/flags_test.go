package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// stubProjectAndEnv stubs the resolvers used by every flag command.
func stubProjectAndEnv(t *testing.T, srv *mockServer, projectID, envID string) {
	t.Helper()
	srv.onPathFunc("GET", "/api/v1/orgs/acme/projects/payments", func(recordedRequest) (int, any) {
		return 200, map[string]any{"id": projectID, "slug": "payments", "name": "Payments"}
	})
	srv.onPathFunc("GET", "/api/v1/orgs/acme/environments", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"environments": []map[string]any{
				{"id": envID, "slug": "production", "name": "Production"},
				{"id": "env-staging-uuid", "slug": "staging", "name": "Staging"},
			},
		}
	})
}

func TestFlagsCreate_Success_NoEnv(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("POST", "/api/v1/flags", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/flags", req.Path)
		require.Equal(t, "proj-uuid", req.Body["project_id"])
		require.Equal(t, "dark-mode", req.Body["key"])
		require.Equal(t, "dark-mode", req.Body["name"])
		require.Equal(t, "boolean", req.Body["flag_type"])
		require.Equal(t, "feature", req.Body["category"])
		_, hasEnv := req.Body["environment_id"]
		require.False(t, hasEnv, "no environment_id when --env not passed")
		return 201, map[string]any{"id": "f-1", "key": "dark-mode"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "flags", "create", "--key", "dark-mode", "--type", "boolean")
	require.NoError(t, err)
	require.Contains(t, stdout, "created successfully")
}

func TestFlagsCreate_Success_WithEnv(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("POST", "/api/v1/flags", func(req recordedRequest) (int, any) {
		require.Equal(t, "env-prod-uuid", req.Body["environment_id"])
		return 201, map[string]any{"id": "f-1", "key": "dark-mode"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	_, _, err := runCmd(t, rootCmd, "flags", "create", "--key", "dark-mode", "--type", "boolean", "--default", "false")
	require.NoError(t, err)
}

func TestFlagsCreate_Success_FullPayload(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("POST", "/api/v1/flags", func(req recordedRequest) (int, any) {
		require.Equal(t, "Dark Mode UI", req.Body["name"])
		require.Equal(t, "release", req.Body["category"])
		require.Equal(t, "Toggles dark mode", req.Body["description"])
		require.Equal(t, "false", req.Body["default_value"])
		tags, _ := req.Body["tags"].([]any)
		require.ElementsMatch(t, []any{"ui", "rollout"}, tags)
		return 201, map[string]any{"id": "f-1", "key": "dark-mode"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	_, _, err := runCmd(t, rootCmd, "flags", "create",
		"--key", "dark-mode",
		"--name", "Dark Mode UI",
		"--type", "boolean",
		"--category", "release",
		"--description", "Toggles dark mode",
		"--default", "false",
		"--tag", "ui",
		"--tag", "rollout",
	)
	require.NoError(t, err)
}

func TestFlagsCreate_InvalidType(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	_, _, err := runCmd(t, rootCmd, "flags", "create", "--key", "x", "--type", "weird")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid flag type")
}

func TestFlagsCreate_APIError(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("POST", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 422, map[string]any{"error": "key must be unique"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	_, _, err := runCmd(t, rootCmd, "flags", "create", "--key", "dup", "--type", "boolean")
	require.Error(t, err)
	require.Contains(t, err.Error(), "key must be unique")
}
