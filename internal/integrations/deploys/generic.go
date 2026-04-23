package deploys

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
)

// GenericAdapter accepts the canonical DeployEvent payload directly. It is
// the fallback for CI pipelines (GitHub Actions, CircleCI, Jenkins…) or any
// provider without a first-party adapter.
type GenericAdapter struct{}

func (GenericAdapter) Provider() string { return models.DeployProviderGeneric }

// VerifySignature checks either HMAC (sha256=<hex> in X-DeploySentry-Signature)
// or bearer token (Authorization: Bearer <token>), driven by integration.AuthMode.
// The integration type isn't available here so we try both — whichever secret
// matches wins. Callers supply the shared secret derived from the integration.
func (GenericAdapter) VerifySignature(r *http.Request, body []byte, secret string) error {
	if secret == "" {
		return ErrInvalidSignature
	}
	if sig := r.Header.Get("X-DeploySentry-Signature"); sig != "" {
		return verifyHMACSignature(sig, body, secret)
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		if subtleEqual(token, secret) {
			return nil
		}
		return ErrInvalidSignature
	}
	return ErrInvalidSignature
}

func (GenericAdapter) ParsePayload(body []byte, _ *models.DeployIntegration) (models.DeployEvent, error) {
	var ev models.DeployEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return models.DeployEvent{}, fmt.Errorf("parse generic payload: %w", err)
	}
	if ev.EventType == "" {
		return models.DeployEvent{}, errors.New("event_type is required")
	}
	if ev.Version == "" {
		return models.DeployEvent{}, errors.New("version is required")
	}
	if ev.Environment == "" {
		return models.DeployEvent{}, errors.New("environment is required")
	}
	if ev.OccurredAt.IsZero() {
		ev.OccurredAt = time.Now().UTC()
	}
	return ev, nil
}

// verifyHMACSignature expects the header value in the form "sha256=<hex>".
// Leading "sha256=" is optional for tolerance.
func verifyHMACSignature(header string, body []byte, secret string) error {
	given := strings.TrimPrefix(header, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !subtleEqual(given, expected) {
		return ErrInvalidSignature
	}
	return nil
}

// subtleEqual is a constant-time string comparison.
func subtleEqual(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}
