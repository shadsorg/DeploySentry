package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveProjectID(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/orgs/acme/projects/payments", func(recordedRequest) (int, any) {
		return 200, map[string]any{"id": "00000000-0000-0000-0000-000000000001", "slug": "payments"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")
	client, err := clientFromConfig()
	require.NoError(t, err)

	id, err := resolveProjectID(client, "acme", "payments")
	require.NoError(t, err)
	require.Equal(t, "00000000-0000-0000-0000-000000000001", id)
}

func TestResolveEnvID(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/orgs/acme/environments", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"environments": []map[string]any{
				{"id": "env-staging-uuid", "slug": "staging"},
				{"id": "env-prod-uuid", "slug": "production"},
			},
		}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")
	client, err := clientFromConfig()
	require.NoError(t, err)

	id, err := resolveEnvID(client, "acme", "production")
	require.NoError(t, err)
	require.Equal(t, "env-prod-uuid", id)

	_, err = resolveEnvID(client, "acme", "nope")
	require.Error(t, err)
}

func TestResolveFlagID(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/flags", func(recordedRequest) (int, any) {
		return 200, map[string]any{
			"flags": []map[string]any{
				{"id": "flag-1-uuid", "key": "dark-mode"},
				{"id": "flag-2-uuid", "key": "new-checkout"},
			},
		}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "payments", "")
	client, err := clientFromConfig()
	require.NoError(t, err)

	id, err := resolveFlagID(client, "00000000-0000-0000-0000-000000000001", "new-checkout")
	require.NoError(t, err)
	require.Equal(t, "flag-2-uuid", id)

	_, err = resolveFlagID(client, "00000000-0000-0000-0000-000000000001", "missing")
	require.ErrorIs(t, err, ErrFlagNotFound)
}
