package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// resetWebhooksCreateFlags clears cobra flag state on webhooksCreateCmd
// between tests.
func resetWebhooksCreateFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"name", "url", "events", "retry-attempts", "timeout"} {
			if f := webhooksCreateCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

func resetWebhooksListFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"project", "active", "inactive", "events", "limit"} {
			if f := webhooksListCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

func resetWebhooksDeleteFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		if f := webhooksDeleteCmd.Flags().Lookup("confirm"); f != nil {
			f.Changed = false
			_ = f.Value.Set(f.DefValue)
		}
	})
}

// TestWebhooksCreate_Success verifies the CLI POSTs to /api/v1/webhooks (no
// org prefix; the server resolves the org from the auth context) with the
// expected CreateWebhookRequest body shape.
func TestWebhooksCreate_Success(t *testing.T) {
	resetWebhooksCreateFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("POST", "/api/v1/webhooks", func(req recordedRequest) (int, any) {
		require.Equal(t, "Deploy Notifications", req.Body["name"])
		require.Equal(t, "https://api.example.com/webhook", req.Body["url"])
		events, _ := req.Body["events"].([]any)
		require.ElementsMatch(t, []any{"deployment.completed"}, events)
		return 201, map[string]any{"webhook": map[string]any{
			"id":   "w-1",
			"name": "Deploy Notifications",
			"url":  "https://api.example.com/webhook",
		}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	stdout, _, err := runCmd(t, rootCmd, "webhooks", "create",
		"--name", "Deploy Notifications",
		"--url", "https://api.example.com/webhook",
		"--events", "deployment.completed",
	)
	require.NoError(t, err)
	require.Contains(t, stdout, "w-1")
}

// TestWebhooksList_Success verifies GET /api/v1/webhooks (no org prefix).
func TestWebhooksList_Success(t *testing.T) {
	resetWebhooksListFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/webhooks", func(req recordedRequest) (int, any) {
		return 200, map[string]any{"webhooks": []map[string]any{
			{
				"id":        "11111111-1111-1111-1111-111111111111",
				"name":      "Deploys",
				"url":       "https://example.com",
				"is_active": true,
				"events":    []string{"deployment.completed"},
			},
		}}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	stdout, _, err := runCmd(t, rootCmd, "webhooks", "list")
	require.NoError(t, err)
	require.Contains(t, stdout, "Deploys")
}

// TestWebhooksDelete_Success verifies DELETE /api/v1/webhooks/<id> (no org
// prefix). The --confirm flag skips the interactive prompt.
func TestWebhooksDelete_Success(t *testing.T) {
	resetWebhooksDeleteFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("DELETE", "/api/v1/webhooks/w-1", func(req recordedRequest) (int, any) {
		return 200, map[string]any{"status": "deleted"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	_, _, err := runCmd(t, rootCmd, "webhooks", "delete", "w-1", "--confirm")
	require.NoError(t, err)
}
