package deploys

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/stretchr/testify/assert"
)

// Each case asserts (a) signature verification passes for a correctly-signed
// body and fails for a tampered one, and (b) the parser yields the expected
// canonical event type + environment + version.

type providerCase struct {
	name        string
	adapter     DeployEventAdapter
	payload     []byte
	signHeader  func(req *http.Request, body []byte, secret string)
	wantType    string
	wantVersion string
	wantEnv     string
}

func runProviderCase(t *testing.T, c providerCase) {
	t.Helper()
	secret := "s3cret"

	// Good signature.
	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	c.signHeader(req, c.payload, secret)
	assert.NoError(t, c.adapter.VerifySignature(req, c.payload, secret), c.name+": good signature")

	// Tampered signature.
	reqBad := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	c.signHeader(reqBad, []byte("different body"), secret)
	assert.ErrorIs(t, c.adapter.VerifySignature(reqBad, c.payload, secret), ErrInvalidSignature, c.name+": tampered signature")

	// Parse.
	ev, err := c.adapter.ParsePayload(c.payload, &models.DeployIntegration{})
	assert.NoError(t, err, c.name+": parse")
	assert.Equal(t, c.wantType, ev.EventType, c.name+": event type")
	assert.Equal(t, c.wantVersion, ev.Version, c.name+": version")
	assert.Equal(t, c.wantEnv, ev.Environment, c.name+": environment")
}

func renderSign(req *http.Request, body []byte, secret string) {
	ts := "1700000000"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "." + string(body)))
	req.Header.Set("Render-Webhook-Signature", "t="+ts+",v1="+hex.EncodeToString(mac.Sum(nil)))
}

func herokuSign(req *http.Request, body []byte, secret string) {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	req.Header.Set("Heroku-Webhook-Hmac-SHA256", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}

func vercelSign(req *http.Request, body []byte, secret string) {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	req.Header.Set("x-vercel-signature", hex.EncodeToString(mac.Sum(nil)))
}

func sha256HMACSign(header string) func(*http.Request, []byte, string) {
	return func(req *http.Request, body []byte, secret string) {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		req.Header.Set(header, "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
}

func TestProviders_RoundTrip(t *testing.T) {
	cases := []providerCase{
		{
			name:    "render-success",
			adapter: RenderAdapter{},
			payload: []byte(`{
				"type":"deploy_ended",
				"timestamp":"2026-04-23T10:00:00Z",
				"data":{
					"deploy":{"id":"dep-1","status":"live","commit":{"id":"sha-render"}},
					"service":{"id":"srv-1","name":"prod"}
				}
			}`),
			signHeader:  renderSign,
			wantType:    models.DeployEventSucceeded,
			wantVersion: "sha-render",
			wantEnv:     "prod",
		},
		{
			name:    "fly-success",
			adapter: FlyAdapter{},
			payload: []byte(`{
				"event":"release.succeeded",
				"app":{"name":"prod-api"},
				"release":{"version":42,"image_ref":"img:42","commit_sha":"sha-fly"}
			}`),
			signHeader:  sha256HMACSign("X-Fly-Signature"),
			wantType:    models.DeployEventSucceeded,
			wantVersion: "v42",
			wantEnv:     "prod-api",
		},
		{
			name:    "heroku-success",
			adapter: HerokuAdapter{},
			payload: []byte(`{
				"resource":"release",
				"action":"create",
				"data":{
					"id":"rel-1","status":"succeeded","version":5,
					"app":{"name":"my-app-prod"},
					"slug":{"commit":"sha-heroku"}
				}
			}`),
			signHeader:  herokuSign,
			wantType:    models.DeployEventSucceeded,
			wantVersion: "sha-heroku",
			wantEnv:     "my-app-prod",
		},
		{
			name:    "vercel-success",
			adapter: VercelAdapter{},
			payload: []byte(`{
				"type":"deployment.succeeded",
				"payload":{
					"deployment":{"id":"dpl_1","url":"proj.vercel.app","meta":{"githubCommitSha":"sha-vercel"}},
					"project":{"id":"prj_1","name":"proj"},
					"target":"production"
				}
			}`),
			signHeader:  vercelSign,
			wantType:    models.DeployEventSucceeded,
			wantVersion: "sha-vercel",
			wantEnv:     "production",
		},
		{
			name:    "netlify-success",
			adapter: NetlifyAdapter{},
			payload: []byte(`{
				"id":"dep-n1","state":"ready","branch":"main","commit_ref":"sha-netlify",
				"context":"production","deploy_url":"https://x.netlify.app"
			}`),
			signHeader:  sha256HMACSign("x-webhook-signature"),
			wantType:    models.DeployEventSucceeded,
			wantVersion: "sha-netlify",
			wantEnv:     "production",
		},
		{
			name:    "github-actions-success",
			adapter: GitHubActionsAdapter{},
			payload: []byte(`{
				"action":"completed",
				"workflow_run":{
					"id":1,"name":"Deploy prod","head_branch":"main","head_sha":"sha-gha",
					"conclusion":"success","html_url":"https://github.com/x/y/actions/runs/1",
					"workflow_id":10
				},
				"repository":{"full_name":"x/y"}
			}`),
			signHeader:  sha256HMACSign("X-Hub-Signature-256"),
			wantType:    models.DeployEventSucceeded,
			wantVersion: "sha-gha",
			wantEnv:     "main",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { runProviderCase(t, c) })
	}
}

func TestProviders_FailedEventMapping(t *testing.T) {
	cases := []struct {
		name    string
		adapter DeployEventAdapter
		payload []byte
	}{
		{"render-failed", RenderAdapter{}, []byte(`{"type":"deploy_ended","data":{"deploy":{"id":"d","status":"update_failed","commit":{"id":"x"}},"service":{"id":"s","name":"prod"}}}`)},
		{"heroku-failed", HerokuAdapter{}, []byte(`{"resource":"release","action":"create","data":{"id":"r","status":"failed","version":1,"app":{"name":"x"},"slug":{"commit":"c"}}}`)},
		{"vercel-failed", VercelAdapter{}, []byte(`{"type":"deployment.error","payload":{"deployment":{"id":"d","meta":{"githubCommitSha":"c"}},"target":"production","project":{"id":"p","name":"p"}}}`)},
		{"netlify-failed", NetlifyAdapter{}, []byte(`{"id":"x","state":"error","branch":"main","commit_ref":"c","context":"production"}`)},
		{"github-failed", GitHubActionsAdapter{}, []byte(`{"action":"completed","workflow_run":{"id":1,"head_sha":"c","conclusion":"failure","head_branch":"main"},"repository":{"full_name":"x/y"}}`)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev, err := c.adapter.ParsePayload(c.payload, &models.DeployIntegration{})
			assert.NoError(t, err)
			assert.Equal(t, models.DeployEventFailed, ev.EventType)
		})
	}
}

func TestProviders_VersionExtractorOverride(t *testing.T) {
	ev, err := RenderAdapter{}.ParsePayload([]byte(`{
		"type":"deploy_ended",
		"data":{"deploy":{"id":"dep-1","status":"live","commit":{"id":"sha-default"}},
		         "service":{"id":"s","name":"prod"},
		         "custom":{"release":"r-override"}}
	}`), &models.DeployIntegration{VersionExtractors: []string{"data.custom.release"}})
	assert.NoError(t, err)
	assert.Equal(t, "r-override", ev.Version)
}
