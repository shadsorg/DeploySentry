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

// DRIFT NOTE — the audit marked webhooks.go OK, but the CLI hits
// /api/v1/orgs/<org>/webhooks while the server (internal/webhooks/handler.go)
// mounts the routes at /api/v1/webhooks (no org prefix; org is resolved from
// the auth context). The tests below stub BOTH URLs and fail loudly via
// require.Fail when the CLI hits the wrong one, so the orchestrator can
// dispatch a fix without us silently rubber-stamping the drift.

// TestWebhooksCreate_DriftURL stubs the wrong CLI URL and the correct server
// URL. If the CLI hits the correct one, the test passes; if it hits the
// wrong one (current behavior), the test fails loudly with require.Fail.
func TestWebhooksCreate_DriftURL(t *testing.T) {
	resetWebhooksCreateFlags(t)
	srv := newMockServer(t)
	// Correct URL per server contract.
	srv.onPathFunc("POST", "/api/v1/webhooks", func(req recordedRequest) (int, any) {
		require.Equal(t, "Deploy Notifications", req.Body["name"])
		require.Equal(t, "https://api.example.com/webhook", req.Body["url"])
		events, _ := req.Body["events"].([]any)
		require.ElementsMatch(t, []any{"deployment.completed"}, events)
		return 201, map[string]any{"webhook": map[string]any{"id": "w-1", "name": "Deploy Notifications", "url": "https://api.example.com/webhook"}}
	})
	// Wrong URL the CLI currently uses — fail loudly if it lands here.
	srv.onPathFunc("POST", "/api/v1/orgs/acme/webhooks", func(req recordedRequest) (int, any) {
		require.Fail(t, "drift detected: webhooks create posts to /api/v1/orgs/<org>/webhooks; server contract is /api/v1/webhooks")
		return 599, nil
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	_, _, err := runCmd(t, rootCmd, "webhooks", "create",
		"--name", "Deploy Notifications",
		"--url", "https://api.example.com/webhook",
		"--events", "deployment.completed",
	)
	require.NoError(t, err)
}

// TestWebhooksList_DriftURL — same drift pattern as create.
func TestWebhooksList_DriftURL(t *testing.T) {
	resetWebhooksListFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/webhooks", func(req recordedRequest) (int, any) {
		return 200, map[string]any{"webhooks": []map[string]any{
			{"id": "11111111-1111-1111-1111-111111111111", "name": "Deploys", "url": "https://example.com", "is_active": true, "events": []string{"deployment.completed"}},
		}}
	})
	srv.onPathFunc("GET", "/api/v1/orgs/acme/webhooks", func(req recordedRequest) (int, any) {
		require.Fail(t, "drift detected: webhooks list GETs /api/v1/orgs/<org>/webhooks; server contract is /api/v1/webhooks")
		return 599, nil
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	_, _, err := runCmd(t, rootCmd, "webhooks", "list")
	require.NoError(t, err)
}

// TestWebhooksDelete_DriftURL — same drift pattern. The --confirm flag skips
// the interactive prompt.
func TestWebhooksDelete_DriftURL(t *testing.T) {
	resetWebhooksDeleteFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("DELETE", "/api/v1/webhooks/w-1", func(req recordedRequest) (int, any) {
		return 200, map[string]any{"status": "deleted"}
	})
	srv.onPathFunc("DELETE", "/api/v1/orgs/acme/webhooks/w-1", func(req recordedRequest) (int, any) {
		require.Fail(t, "drift detected: webhooks delete DELETEs /api/v1/orgs/<org>/webhooks/<id>; server contract is /api/v1/webhooks/<id>")
		return 599, nil
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "acme", "", "")

	_, _, err := runCmd(t, rootCmd, "webhooks", "delete", "w-1", "--confirm")
	require.NoError(t, err)
}
