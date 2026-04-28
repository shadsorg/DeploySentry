package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlagsRulesAdd_Percentage(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("POST", "/api/v1/flags/f-1/rules", func(req recordedRequest) (int, any) {
		require.Equal(t, "percentage", req.Body["rule_type"])
		require.Equal(t, float64(25), req.Body["percentage"])
		require.Equal(t, "true", req.Body["value"])
		require.Equal(t, true, req.Body["enabled"])
		return 201, map[string]any{"id": "rule-1"}
	})
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "flags", "rules", "add", "dark-mode", "--rule-type", "percentage", "--percentage", "25", "--value", "true")
	require.NoError(t, err)
	require.Contains(t, stdout, "rule-1")
}

func TestFlagsRulesAdd_Attribute(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("POST", "/api/v1/flags/f-1/rules", func(req recordedRequest) (int, any) {
		require.Equal(t, "attribute", req.Body["rule_type"])
		require.Equal(t, "plan", req.Body["attribute"])
		require.Equal(t, "eq", req.Body["operator"])
		return 201, map[string]any{"id": "rule-2"}
	})
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	_, _, err := runCmd(t, rootCmd, "flags", "rules", "add", "dark-mode", "--rule-type", "attribute", "--attribute", "plan", "--operator", "eq", "--value", "pro")
	require.NoError(t, err)
}

func TestFlagsRulesList(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/flags/f-1/rules", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"rules": []map[string]any{
				{"id": "r-1", "rule_type": "percentage", "priority": 10, "value": "true", "enabled": true},
			},
		}
	})
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "flags", "rules", "list", "dark-mode")
	require.NoError(t, err)
	require.Contains(t, stdout, "r-1")
	require.Contains(t, stdout, "percentage")
}

func TestFlagsRulesDelete(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("DELETE", "/api/v1/flags/f-1/rules/r-1", func(recordedRequest) (int, any) {
		return 204, map[string]any{}
	})
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "flags", "rules", "delete", "dark-mode", "r-1")
	require.NoError(t, err)
	require.Contains(t, stdout, "deleted")
}

func TestFlagsRulesSetEnvState(t *testing.T) {
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("PUT", "/api/v1/flags/f-1/rules/r-1/environments/env-prod-uuid", func(req recordedRequest) (int, any) {
		require.Equal(t, true, req.Body["enabled"])
		return 200, map[string]any{}
	})
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{"flags": []map[string]any{{"id": "f-1", "key": "dark-mode"}}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	stdout, _, err := runCmd(t, rootCmd, "flags", "rules", "set-env-state", "dark-mode", "r-1", "--on")
	require.NoError(t, err)
	require.Contains(t, stdout, "ON")
}
