package deploys

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
)

// FlyAdapter consumes Fly.io-shaped deploy events. Fly's native webhook
// surface is minimal; callers typically wire a custom webhook from a CI
// job post-deploy. We expect `X-Fly-Signature: sha256=<hex>` with the
// HMAC over the raw body — matching our generic HMAC convention.
type FlyAdapter struct{}

func (FlyAdapter) Provider() string { return models.DeployProviderFly }

func (FlyAdapter) VerifySignature(r *http.Request, body []byte, secret string) error {
	sig := r.Header.Get("X-Fly-Signature")
	if sig == "" {
		return ErrInvalidSignature
	}
	return verifyHMACSignature(sig, body, secret)
}

type flyPayload struct {
	Event string `json:"event"` // release.succeeded | release.failed | release.started | …
	App   struct {
		Name string `json:"name"`
	} `json:"app"`
	Release struct {
		Version     int    `json:"version"`
		ImageRef    string `json:"image_ref"`
		Status      string `json:"status"`
		Environment string `json:"environment"`
		CommitSHA   string `json:"commit_sha"`
	} `json:"release"`
	CreatedAt time.Time `json:"created_at"`
}

func (FlyAdapter) ParsePayload(body []byte, integration *models.DeployIntegration) (models.DeployEvent, error) {
	var p flyPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return models.DeployEvent{}, fmt.Errorf("parse fly payload: %w", err)
	}
	eventType := ""
	switch p.Event {
	case "release.succeeded", "deploy.succeeded":
		eventType = models.DeployEventSucceeded
	case "release.failed", "deploy.failed":
		eventType = models.DeployEventFailed
	case "release.started", "deploy.started":
		eventType = models.DeployEventStarted
	default:
		if p.Release.Status == "succeeded" {
			eventType = models.DeployEventSucceeded
		} else if p.Release.Status == "failed" {
			eventType = models.DeployEventFailed
		} else {
			return models.DeployEvent{}, fmt.Errorf("unsupported Fly event %q", p.Event)
		}
	}

	version := extractVersion(body, integration.VersionExtractors)
	if version == "" {
		switch {
		case p.Release.Version > 0:
			version = fmt.Sprintf("v%d", p.Release.Version)
		case p.Release.ImageRef != "":
			version = p.Release.ImageRef
		case p.Release.CommitSHA != "":
			version = p.Release.CommitSHA
		}
	}
	if version == "" {
		return models.DeployEvent{}, errors.New("could not extract version from Fly payload")
	}

	env := p.Release.Environment
	if env == "" {
		env = p.App.Name
	}

	occurred := p.CreatedAt
	if occurred.IsZero() {
		occurred = time.Now().UTC()
	}

	return models.DeployEvent{
		EventType:   eventType,
		Environment: env,
		Version:     version,
		CommitSHA:   p.Release.CommitSHA,
		Artifact:    p.Release.ImageRef,
		OccurredAt:  occurred,
		Metadata:    body,
	}, nil
}
