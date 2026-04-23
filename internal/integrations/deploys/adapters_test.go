package deploys

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/stretchr/testify/assert"
)

func sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// -----------------------------------------------------------------------------
// Generic adapter
// -----------------------------------------------------------------------------

func TestGenericAdapter_VerifyHMAC(t *testing.T) {
	adapter := GenericAdapter{}
	body := []byte(`{"foo":"bar"}`)
	secret := "s3cret"
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(body)))
	req.Header.Set("X-DeploySentry-Signature", sign(body, secret))

	assert.NoError(t, adapter.VerifySignature(req, body, secret))
}

func TestGenericAdapter_VerifyBearer(t *testing.T) {
	adapter := GenericAdapter{}
	secret := "tok_abc"
	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	req.Header.Set("Authorization", "Bearer "+secret)

	assert.NoError(t, adapter.VerifySignature(req, nil, secret))
}

func TestGenericAdapter_VerifyRejectsBadSignature(t *testing.T) {
	adapter := GenericAdapter{}
	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	req.Header.Set("X-DeploySentry-Signature", "sha256=deadbeef")
	assert.ErrorIs(t, adapter.VerifySignature(req, []byte("body"), "s3cret"), ErrInvalidSignature)
}

func TestGenericAdapter_ParseRequiresFields(t *testing.T) {
	adapter := GenericAdapter{}
	_, err := adapter.ParsePayload([]byte(`{}`), nil)
	assert.Error(t, err)

	_, err = adapter.ParsePayload([]byte(`{"event_type":"deploy.succeeded"}`), nil)
	assert.Error(t, err)

	ev, err := adapter.ParsePayload([]byte(`{"event_type":"deploy.succeeded","environment":"production","version":"1"}`), nil)
	assert.NoError(t, err)
	assert.Equal(t, "1", ev.Version)
}

// -----------------------------------------------------------------------------
// Railway adapter
// -----------------------------------------------------------------------------

func TestRailwayAdapter_ParseSuccessPayload(t *testing.T) {
	adapter := RailwayAdapter{}
	payload := []byte(`{
		"type": "DEPLOY",
		"status": "SUCCESS",
		"meta": {"deploymentId": "dep-abc"},
		"commit": {"sha": "sha-abc"},
		"environment": {"name": "production"},
		"service": {"id": "svc-123"}
	}`)
	ev, err := adapter.ParsePayload(payload, &models.DeployIntegration{})
	assert.NoError(t, err)
	assert.Equal(t, models.DeployEventSucceeded, ev.EventType)
	assert.Equal(t, "production", ev.Environment)
	assert.Equal(t, "dep-abc", ev.Version)
	assert.Equal(t, "sha-abc", ev.CommitSHA)
}

func TestRailwayAdapter_VersionExtractorOverride(t *testing.T) {
	adapter := RailwayAdapter{}
	payload := []byte(`{
		"type": "DEPLOY",
		"status": "SUCCESS",
		"meta": {"deploymentId": "dep-abc"},
		"commit": {"sha": "sha-xyz"},
		"environment": {"name": "production"}
	}`)
	ev, err := adapter.ParsePayload(payload, &models.DeployIntegration{
		VersionExtractors: []string{"commit.sha"},
	})
	assert.NoError(t, err)
	assert.Equal(t, "sha-xyz", ev.Version, "extractor should override default")
}

func TestRailwayAdapter_VerifySignature(t *testing.T) {
	adapter := RailwayAdapter{}
	body := []byte(`{"type":"DEPLOY"}`)
	secret := "rail-s3cret"
	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	req.Header.Set("X-Railway-Signature", sign(body, secret))

	assert.NoError(t, adapter.VerifySignature(req, body, secret))

	reqBad := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	reqBad.Header.Set("X-Railway-Signature", "sha256=deadbeef")
	assert.ErrorIs(t, adapter.VerifySignature(reqBad, body, secret), ErrInvalidSignature)
}

func TestRailwayAdapter_RejectsNonDeployType(t *testing.T) {
	adapter := RailwayAdapter{}
	_, err := adapter.ParsePayload([]byte(`{"type":"BUILD","status":"SUCCESS","environment":{"name":"x"}}`), &models.DeployIntegration{})
	assert.Error(t, err)
}

func TestRailwayAdapter_MapsFailedStatus(t *testing.T) {
	adapter := RailwayAdapter{}
	ev, err := adapter.ParsePayload([]byte(`{
		"type":"DEPLOY","status":"FAILED",
		"meta":{"deploymentId":"x"},
		"environment":{"name":"staging"}}`), &models.DeployIntegration{})
	assert.NoError(t, err)
	assert.Equal(t, models.DeployEventFailed, ev.EventType)
}
