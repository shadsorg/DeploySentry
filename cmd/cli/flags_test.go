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

func TestFlagsGet_Success(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	// Register the more specific route first so prefix matching prefers it.
	srv.onPathFunc("GET", "/api/v1/flags/f-1", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"id": "f-1", "key": "dark-mode", "flag_type": "boolean",
			"default_value": "false", "description": "Dark UI",
		}
	})
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "flags", "get", "dark-mode")
	require.NoError(t, err)
	require.Contains(t, stdout, "Type:        boolean")
	require.Contains(t, stdout, "Default:     false")
	require.Contains(t, stdout, "Description: Dark UI")
}

func TestFlagsGet_FlagNotFound(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	_, _, err := runCmd(t, rootCmd, "flags", "get", "missing")
	require.Error(t, err)
}

func resetFlagsUpdateFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"default", "description", "name", "category", "tag"} {
			f := flagsUpdateCmd.Flags().Lookup(name)
			if f == nil {
				continue
			}
			f.Changed = false
			_ = f.Value.Set(f.DefValue)
		}
	})
}

func TestFlagsUpdate_DefaultValue(t *testing.T) {
	resetFlagsUpdateFlags(t)
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	// More specific route first.
	srv.onPathFunc("PUT", "/api/v1/flags/f-1", func(req recordedRequest) (int, any) {
		require.Equal(t, "true", req.Body["default_value"])
		return 200, map[string]any{"id": "f-1"}
	})
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	_, _, err := runCmd(t, rootCmd, "flags", "update", "dark-mode", "--default", "true")
	require.NoError(t, err)
}

func TestFlagsUpdate_NoChanges(t *testing.T) {
	resetFlagsUpdateFlags(t)
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	_, _, err := runCmd(t, rootCmd, "flags", "update", "dark-mode")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no updates specified")
}

func resetFlagsToggleFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"on", "off"} {
			f := flagsToggleCmd.Flags().Lookup(name)
			if f == nil {
				continue
			}
			f.Changed = false
			_ = f.Value.Set(f.DefValue)
		}
	})
}

func TestFlagsToggle_Unscoped_On(t *testing.T) {
	resetFlagsToggleFlags(t)
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("POST", "/api/v1/flags/f-1/toggle", func(req recordedRequest) (int, any) {
		require.Equal(t, true, req.Body["enabled"])
		return 200, map[string]any{"enabled": true}
	})
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "flags", "toggle", "dark-mode", "--on")
	require.NoError(t, err)
	require.Contains(t, stdout, "toggled ON")
	require.NotContains(t, stdout, " in ")
}

func TestFlagsToggle_EnvScoped_Off(t *testing.T) {
	resetFlagsToggleFlags(t)
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("PUT", "/api/v1/flags/f-1/environments/env-prod-uuid", func(req recordedRequest) (int, any) {
		require.Equal(t, false, req.Body["enabled"])
		return 200, map[string]any{"enabled": false}
	})
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	stdout, _, err := runCmd(t, rootCmd, "flags", "toggle", "dark-mode", "--off")
	require.NoError(t, err)
	require.Contains(t, stdout, "toggled OFF in production")
}

func TestFlagsToggle_NoFlag(t *testing.T) {
	resetFlagsToggleFlags(t)
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	_, _, err := runCmd(t, rootCmd, "flags", "toggle", "dark-mode")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--on or --off")
}

func TestFlagsArchive_Success(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("POST", "/api/v1/flags/f-1/archive", func(recordedRequest) (int, any) {
		return 200, map[string]any{"status": "archived"}
	})
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "old"}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "flags", "archive", "old")
	require.NoError(t, err)
	require.Contains(t, stdout, "archived successfully")
}

func TestFlagsEvaluate_Success(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("POST", "/api/v1/flags/evaluate", func(req recordedRequest) (int, any) {
		require.Equal(t, "dark-mode", req.Body["flag_key"])
		require.Equal(t, "proj-uuid", req.Body["project_id"])
		ctx, _ := req.Body["context"].(map[string]any)
		require.Equal(t, "u1", ctx["user_id"])
		return 200, map[string]any{"value": true, "reason": "DEFAULT_VALUE"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "flags", "evaluate", "dark-mode", "--context", `{"user_id":"u1"}`)
	require.NoError(t, err)
	require.Contains(t, stdout, "Value:  true")
	require.Contains(t, stdout, "DEFAULT_VALUE")
}
