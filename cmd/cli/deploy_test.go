package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// resetDeployCreateFlags clears cobra flag state on deployCreateCmd so a
// required flag set in one test doesn't bleed into the next.
func resetDeployCreateFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"release", "env", "strategy", "description", "apply-immediately"} {
			if f := deployCreateCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

func resetDeployListFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"env", "status", "limit"} {
			if f := deployListCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

// TestDeployCreate_Success exercises the rolling-strategy happy path against
// the URL the CLI actually posts to today.
func TestDeployCreate_Success(t *testing.T) {
	resetDeployCreateFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("POST", "/api/v1/orgs/acme/projects/payments/deployments", func(req recordedRequest) (int, any) {
		require.Equal(t, "v1.2.3", req.Body["release"])
		require.Equal(t, "production", req.Body["environment"])
		require.Equal(t, "rolling", req.Body["strategy"])
		return 201, map[string]any{"id": "deploy-1", "status": "pending"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	stdout, _, err := runCmd(t, rootCmd, "deploy", "create", "--release", "v1.2.3", "--env", "production", "--strategy", "rolling")
	require.NoError(t, err)
	require.Contains(t, stdout, "deploy-1")
}

// TestDeployStatus_Success covers `deploy status <id>`.
func TestDeployStatus_Success(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/orgs/acme/projects/payments/deployments/deploy-1", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"id":          "deploy-1",
			"status":      "running",
			"release":     "v1.2.3",
			"environment": "production",
			"strategy":    "rolling",
			"progress":    float64(50),
			"created_at":  "2026-04-24T00:00:00Z",
		}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	stdout, _, err := runCmd(t, rootCmd, "deploy", "status", "deploy-1")
	require.NoError(t, err)
	require.Contains(t, stdout, "deploy-1")
	require.Contains(t, stdout, "running")
}

// TestDeployList_Success covers `deploy list` with no filters.
func TestDeployList_Success(t *testing.T) {
	resetDeployListFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/orgs/acme/projects/payments/deployments", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"deployments": []map[string]any{
				{
					"id":          "deploy-1",
					"release":     "v1.2.3",
					"environment": "production",
					"strategy":    "rolling",
					"status":      "completed",
					"progress":    float64(100),
					"created_at":  "2026-04-24T00:00:00Z",
				},
			},
		}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "deploy", "list")
	require.NoError(t, err)
	require.Contains(t, stdout, "deploy-1")
}

// TestDeployPromote_Success covers `deploy promote <id>`.
func TestDeployPromote_Success(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("POST", "/api/v1/orgs/acme/projects/payments/deployments/deploy-1/promote", func(recordedRequest) (int, any) {
		return 200, map[string]any{"id": "deploy-1", "status": "promoting", "progress": float64(75)}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "deploy", "promote", "deploy-1")
	require.NoError(t, err)
	require.Contains(t, stdout, "promoted")
}
