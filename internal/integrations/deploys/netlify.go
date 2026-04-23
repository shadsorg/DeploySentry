package deploys

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
)

// NetlifyAdapter consumes Netlify deploy notifications configured with
// "HMAC signature" signing, which delivers `x-webhook-signature: sha256=<hex>`
// over the raw body.
type NetlifyAdapter struct{}

func (NetlifyAdapter) Provider() string { return models.DeployProviderNetlify }

func (NetlifyAdapter) VerifySignature(r *http.Request, body []byte, secret string) error {
	sig := r.Header.Get("x-webhook-signature")
	if sig == "" {
		return ErrInvalidSignature
	}
	return verifyHMACSignature(sig, body, secret)
}

type netlifyPayload struct {
	ID         string    `json:"id"`
	State      string    `json:"state"`      // ready | error | building | enqueued
	Branch     string    `json:"branch"`
	CommitRef  string    `json:"commit_ref"`
	DeployURL  string    `json:"deploy_url"`
	Context    string    `json:"context"`
	UpdatedAt  time.Time `json:"updated_at"`
	SiteID     string    `json:"site_id"`
}

func (NetlifyAdapter) ParsePayload(body []byte, integration *models.DeployIntegration) (models.DeployEvent, error) {
	var p netlifyPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return models.DeployEvent{}, fmt.Errorf("parse netlify payload: %w", err)
	}
	eventType := ""
	switch strings.ToLower(p.State) {
	case "ready":
		eventType = models.DeployEventSucceeded
	case "error":
		eventType = models.DeployEventFailed
	case "enqueued", "building":
		eventType = models.DeployEventStarted
	default:
		return models.DeployEvent{}, fmt.Errorf("unsupported Netlify state %q", p.State)
	}

	version := extractVersion(body, integration.VersionExtractors)
	if version == "" {
		switch {
		case p.CommitRef != "":
			version = p.CommitRef
		case p.ID != "":
			version = p.ID
		}
	}
	if version == "" {
		return models.DeployEvent{}, errors.New("could not extract version from Netlify payload")
	}

	env := p.Context
	if env == "" {
		env = p.Branch
	}

	occurred := p.UpdatedAt
	if occurred.IsZero() {
		occurred = time.Now().UTC()
	}

	return models.DeployEvent{
		EventType:   eventType,
		Environment: env,
		Version:     version,
		CommitSHA:   p.CommitRef,
		OccurredAt:  occurred,
		URL:         p.DeployURL,
		Metadata:    body,
	}, nil
}
