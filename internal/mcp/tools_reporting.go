package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// ---------------------------------------------------------------------------
// ds_reporting_setup_deploy_integration
// ---------------------------------------------------------------------------

var setupDeployIntegrationTool = mcp.NewTool("ds_reporting_setup_deploy_integration",
	mcp.WithDescription(
		"Create an agentless deploy-event integration for a provider (railway, render, fly, heroku, vercel, netlify, github-actions, generic). "+
			"Returns the webhook URL + signing secret + provider-specific setup instructions.",
	),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("application_id", mcp.Required(), mcp.Description("DeploySentry application UUID")),
	mcp.WithString("provider", mcp.Required(), mcp.Description("Provider name")),
	mcp.WithString("webhook_secret", mcp.Required(), mcp.Description("Webhook HMAC signing secret (or bearer token when auth_mode=bearer)")),
	mcp.WithString("auth_mode", mcp.Description("'hmac' (default) or 'bearer'")),
	mcp.WithString("env_mapping", mcp.Required(), mcp.Description("Comma-separated provider=ds_env_uuid pairs, e.g. 'production=uuid-a,staging=uuid-b'")),
	mcp.WithString("provider_config", mcp.Description("JSON-encoded provider-specific config (e.g. {\"service_id\":\"svc-123\"})")),
	mcp.WithString("version_extractors", mcp.Description("Comma-separated dot-paths to try in order when extracting version")),
)

func handleSetupDeployIntegration(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := checkReady()
	if err != nil {
		return errResult(err), nil
	}

	appID, err := req.RequireString("application_id")
	if err != nil {
		return errResult(err), nil
	}
	provider, err := req.RequireString("provider")
	if err != nil {
		return errResult(err), nil
	}
	secret, err := req.RequireString("webhook_secret")
	if err != nil {
		return errResult(err), nil
	}
	envMapStr, err := req.RequireString("env_mapping")
	if err != nil {
		return errResult(err), nil
	}

	envMap := map[string]string{}
	for _, pair := range splitTrim(envMapStr) {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return errResult(fmt.Errorf("invalid env_mapping entry %q (expected key=uuid)", pair)), nil
		}
		envMap[kv[0]] = kv[1]
	}

	body := map[string]interface{}{
		"application_id": appID,
		"provider":       provider,
		"webhook_secret": secret,
		"env_mapping":    envMap,
	}
	if v := req.GetString("auth_mode", ""); v != "" {
		body["auth_mode"] = v
	}
	if v := req.GetString("provider_config", ""); v != "" {
		body["provider_config"] = v // server parses JSON; still useful when client knows the shape
	}
	if v := req.GetString("version_extractors", ""); v != "" {
		body["version_extractors"] = splitTrim(v)
	}

	data, err := c.post("/api/v1/integrations/deploys", body)
	if err != nil {
		return errResult(err), nil
	}

	integrationID, _ := data["id"].(string)
	webhookURL := ""
	instructions := providerInstructions(provider)
	if provider == "generic" {
		webhookURL = c.baseURL + "/api/v1/integrations/deploys/webhook"
	} else {
		webhookURL = c.baseURL + "/api/v1/integrations/" + provider + "/webhook"
	}

	result := map[string]interface{}{
		"integration_id":  integrationID,
		"webhook_url":     webhookURL,
		"signing_secret":  "(Use the secret you supplied. It is stored encrypted; DeploySentry cannot read it back.)",
		"provider":        provider,
		"instructions":    instructions,
		"integration":     data,
	}
	return jsonResult(result)
}

// ---------------------------------------------------------------------------
// ds_reporting_check_deploy_integration
// ---------------------------------------------------------------------------

var checkDeployIntegrationTool = mcp.NewTool("ds_reporting_check_deploy_integration",
	mcp.WithDescription(
		"Check whether a deploy-event integration has received any valid webhook events recently. "+
			"Useful right after wiring a provider webhook to confirm it's working.",
	),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithString("integration_id", mcp.Required(), mcp.Description("DeploySentry integration UUID")),
	mcp.WithString("within_minutes", mcp.Description("Window to consider an event 'recent' (default 10)")),
)

func handleCheckDeployIntegration(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := checkReady()
	if err != nil {
		return errResult(err), nil
	}
	id, err := req.RequireString("integration_id")
	if err != nil {
		return errResult(err), nil
	}
	within := 10
	if v := req.GetString("within_minutes", ""); v != "" {
		fmt.Sscanf(v, "%d", &within)
	}
	data, err := c.get("/api/v1/integrations/deploys/" + id + "/events?limit=5")
	if err != nil {
		return errResult(err), nil
	}

	events, _ := data["events"].([]interface{})
	cutoff := time.Now().UTC().Add(-time.Duration(within) * time.Minute)

	var mostRecent time.Time
	hit := false
	for _, e := range events {
		m, _ := e.(map[string]interface{})
		ts, _ := m["received_at"].(string)
		t, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			t, _ = time.Parse(time.RFC3339, ts)
		}
		if t.After(mostRecent) {
			mostRecent = t
		}
		if !t.IsZero() && t.After(cutoff) {
			hit = true
		}
	}

	status := "fail"
	if hit {
		status = "pass"
	}
	return jsonResult(map[string]interface{}{
		"status":            status,
		"window_minutes":    within,
		"most_recent_event": mostRecent,
		"event_count":       len(events),
		"recent_events":     events,
	})
}

// ---------------------------------------------------------------------------
// ds_reporting_verify
// ---------------------------------------------------------------------------

var reportingVerifyTool = mcp.NewTool("ds_reporting_verify",
	mcp.WithDescription(
		"End-to-end verification of agentless reporting for an application + environment: "+
			"(a) a deployment is recorded, (b) a /status sample has landed recently, "+
			"(c) integration events (if an integration_id is supplied) have arrived.",
	),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithString("application_id", mcp.Required(), mcp.Description("DeploySentry application UUID")),
	mcp.WithString("environment_id", mcp.Required(), mcp.Description("DeploySentry environment UUID")),
	mcp.WithString("integration_id", mcp.Description("Optional — if set, include a recent-events check for this integration")),
)

func handleReportingVerify(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := checkReady()
	if err != nil {
		return errResult(err), nil
	}
	appID, err := req.RequireString("application_id")
	if err != nil {
		return errResult(err), nil
	}
	envID, err := req.RequireString("environment_id")
	if err != nil {
		return errResult(err), nil
	}

	state, err := c.get(fmt.Sprintf("/api/v1/applications/%s/environments/%s/current-state", appID, envID))
	if err != nil {
		return errResult(err), nil
	}

	checks := []map[string]interface{}{}
	// Check 1: current deployment exists
	depCheck := map[string]interface{}{"name": "deployment_recorded", "status": "fail"}
	if cd, ok := state["current_deployment"].(map[string]interface{}); ok && cd != nil {
		depCheck["status"] = "pass"
		depCheck["version"] = cd["version"]
		depCheck["source"] = cd["source"]
	}
	checks = append(checks, depCheck)

	// Check 2: health freshness
	healthCheck := map[string]interface{}{"name": "health_fresh", "status": "fail"}
	if h, ok := state["health"].(map[string]interface{}); ok {
		staleness, _ := h["staleness"].(string)
		healthCheck["staleness"] = staleness
		healthCheck["source"] = h["source"]
		if staleness == "fresh" {
			healthCheck["status"] = "pass"
		}
	}
	checks = append(checks, healthCheck)

	// Check 3: optional integration recent events
	if integrationID := req.GetString("integration_id", ""); integrationID != "" {
		events, err := c.get("/api/v1/integrations/deploys/" + integrationID + "/events?limit=1")
		integCheck := map[string]interface{}{"name": "integration_received_event", "status": "fail"}
		if err == nil {
			if evs, _ := events["events"].([]interface{}); len(evs) > 0 {
				integCheck["status"] = "pass"
				integCheck["latest"] = evs[0]
			}
		}
		checks = append(checks, integCheck)
	}

	allPass := true
	for _, ch := range checks {
		if ch["status"] != "pass" {
			allPass = false
			break
		}
	}
	overall := "fail"
	if allPass {
		overall = "pass"
	}
	return jsonResult(map[string]interface{}{
		"overall": overall,
		"checks":  checks,
		"state":   state,
	})
}

// ---------------------------------------------------------------------------
// Provider instructions copy
// ---------------------------------------------------------------------------

func providerInstructions(provider string) string {
	switch provider {
	case "railway":
		return "In Railway: service settings → Webhooks → paste the webhook URL; set the signing secret to the webhook_secret you supplied. Railway sends `X-Railway-Signature: sha256=<hex>`."
	case "render":
		return "In Render: service settings → Webhooks → paste the URL and the signing secret. Render sends `Render-Webhook-Signature: t=<ts>,v1=<hex>` covering `ts + \".\" + body`."
	case "heroku":
		return "In Heroku: `heroku webhooks:add -u <URL> -i api:release --secret <secret>`. Header: `Heroku-Webhook-Hmac-SHA256` (base64 HMAC of raw body)."
	case "vercel":
		return "In Vercel: project settings → Webhooks → paste the URL and the signing secret. Header: `x-vercel-signature` (hex HMAC of raw body)."
	case "netlify":
		return "In Netlify: site settings → Build & deploy → Deploy notifications → Outgoing webhook with HMAC signing. Header: `x-webhook-signature: sha256=<hex>`."
	case "fly":
		return "In your Fly deploy pipeline: POST the Fly-shaped payload to the URL with `X-Fly-Signature: sha256=<hex>` covering the raw body."
	case "github-actions":
		return "In GitHub: repo/org settings → Webhooks → add the URL, content type application/json, secret set to your webhook_secret, and subscribe to the 'workflow runs' event. Header: `X-Hub-Signature-256: sha256=<hex>`."
	case "generic":
		return "Hit POST <webhook_url> from your CI/script with the canonical DeployEvent payload. Include `X-DeploySentry-Integration-Id: <id>` plus either `Authorization: Bearer <secret>` (if auth_mode=bearer) or `X-DeploySentry-Signature: sha256=<hex>` over the raw body (if auth_mode=hmac)."
	}
	return ""
}
