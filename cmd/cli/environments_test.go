package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvironmentsList_Success(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/orgs/acme/environments", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"environments": []map[string]any{
				{"id": "env-prod-uuid", "slug": "production", "name": "Production"},
				{"id": "env-staging-uuid", "slug": "staging", "name": "Staging"},
			},
		}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")

	stdout, _, err := runCmd(t, rootCmd, "environments", "list")
	require.NoError(t, err)
	require.Contains(t, stdout, "production")
}
