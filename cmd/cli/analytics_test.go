package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// resetAnalyticsFlags clears persistent cobra flag state across analytics tests.
// Each subcommand has its own --time-range / --limit / --breakdown / etc., so
// reset every flag we touch on each subcommand.
func resetAnalyticsFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		resets := []struct {
			cmd   *cobra.Command
			names []string
		}{
			{analyticsaSummaryCmd, []string{"time-range"}},
			{analyticsFlagsStatsCmd, []string{"time-range", "limit"}},
			{analyticsFlagsUsageCmd, []string{"time-range", "breakdown"}},
			{analyticsDeploymentsStatsCmd, []string{"time-range", "breakdown"}},
			{analyticsHealthCmd, []string{"watch", "detailed", "interval"}},
			{analyticsExportCmd, []string{"start-date", "end-date", "time-range", "format", "type"}},
		}
		for _, r := range resets {
			for _, name := range r.names {
				if f := r.cmd.Flags().Lookup(name); f != nil {
					f.Changed = false
					_ = f.Value.Set(f.DefValue)
				}
			}
		}
	})
}

func TestAnalyticsSummary_Success(t *testing.T) {
	resetAnalyticsFlags(t)
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/analytics/summary", func(req recordedRequest) (int, any) {
		require.Contains(t, req.Path, "project_id=proj-uuid")
		require.Contains(t, req.Path, "environment_id=env-prod-uuid")
		require.Contains(t, req.Path, "time_range=24h")
		return 200, map[string]any{"summary": map[string]any{}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	stdout, _, err := runCmd(t, rootCmd, "analytics", "summary", "--time-range", "24h")
	require.NoError(t, err)
	require.NotEmpty(t, stdout)
}

func TestAnalyticsFlagsStats_Success(t *testing.T) {
	resetAnalyticsFlags(t)
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/analytics/flags/stats", func(req recordedRequest) (int, any) {
		require.Contains(t, req.Path, "project_id=proj-uuid")
		require.Contains(t, req.Path, "environment_id=env-prod-uuid")
		require.Contains(t, req.Path, "time_range=7d")
		return 200, map[string]any{"flags": []any{}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	_, _, err := runCmd(t, rootCmd, "analytics", "flags", "stats", "--time-range", "7d")
	require.NoError(t, err)
}

func TestAnalyticsFlagsUsage_Success(t *testing.T) {
	resetAnalyticsFlags(t)
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/analytics/flags/dark-mode/usage", func(req recordedRequest) (int, any) {
		require.True(t, strings.HasPrefix(req.Path, "/api/v1/analytics/flags/dark-mode/usage"))
		require.Contains(t, req.Path, "project_id=proj-uuid")
		require.Contains(t, req.Path, "environment_id=env-prod-uuid")
		return 200, map[string]any{"usage": map[string]any{}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	_, _, err := runCmd(t, rootCmd, "analytics", "flags", "usage", "dark-mode", "--time-range", "24h")
	require.NoError(t, err)
}

func TestAnalyticsDeploymentsStats_Success(t *testing.T) {
	resetAnalyticsFlags(t)
	srv := newMockServer(t)
	// deployments stats only resolves project (no env), so we still need the
	// project resolver stub. stubProjectAndEnv registers both — env stub is
	// unused here but harmless.
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/analytics/deployments/stats", func(req recordedRequest) (int, any) {
		require.Contains(t, req.Path, "project_id=proj-uuid")
		require.Contains(t, req.Path, "time_range=7d")
		return 200, map[string]any{"summary": map[string]any{"total_deployments": float64(10)}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	_, _, err := runCmd(t, rootCmd, "analytics", "deployments", "stats")
	require.NoError(t, err)
}

func TestAnalyticsHealth_Success(t *testing.T) {
	resetAnalyticsFlags(t)
	srv := newMockServer(t)
	// health does not need project/env resolution.
	srv.onPathFunc("GET", "/api/v1/analytics/health", func(recordedRequest) (int, any) {
		return 200, map[string]any{"health": map[string]any{}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	_, _, err := runCmd(t, rootCmd, "analytics", "health")
	require.NoError(t, err)
}

func TestAnalyticsExport_Success(t *testing.T) {
	resetAnalyticsFlags(t)
	srv := newMockServer(t)
	stubProjectAndEnv(t, srv, "proj-uuid", "env-prod-uuid")
	srv.onPathFunc("GET", "/api/v1/analytics/admin/export", func(req recordedRequest) (int, any) {
		require.Contains(t, req.Path, "project_id=proj-uuid")
		require.Contains(t, req.Path, "start_date=2026-01-01")
		require.Contains(t, req.Path, "end_date=2026-01-31")
		return 200, map[string]any{"data": []any{}, "count": 0}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "production")

	_, _, err := runCmd(t, rootCmd, "analytics", "export",
		"--start-date", "2026-01-01",
		"--end-date", "2026-01-31",
	)
	require.NoError(t, err)
}
