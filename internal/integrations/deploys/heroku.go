package deploys

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
)

// HerokuAdapter consumes Heroku App Webhook `release` events. The
// `Heroku-Webhook-Hmac-SHA256` header carries the base64-encoded HMAC
// of the raw body.
type HerokuAdapter struct{}

func (HerokuAdapter) Provider() string { return models.DeployProviderHeroku }

func (HerokuAdapter) VerifySignature(r *http.Request, body []byte, secret string) error {
	given := r.Header.Get("Heroku-Webhook-Hmac-SHA256")
	if given == "" {
		return ErrInvalidSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !subtleEqual(given, expected) {
		return ErrInvalidSignature
	}
	return nil
}

type herokuPayload struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Data     struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Version int    `json:"version"`
		App     struct {
			Name string `json:"name"`
		} `json:"app"`
		Slug struct {
			Commit string `json:"commit"`
		} `json:"slug"`
		UpdatedAt time.Time `json:"updated_at"`
	} `json:"data"`
}

func (HerokuAdapter) ParsePayload(body []byte, integration *models.DeployIntegration) (models.DeployEvent, error) {
	var p herokuPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return models.DeployEvent{}, fmt.Errorf("parse heroku payload: %w", err)
	}
	if p.Resource != "release" {
		return models.DeployEvent{}, fmt.Errorf("unsupported Heroku resource %q", p.Resource)
	}
	eventType := ""
	switch strings.ToLower(p.Data.Status) {
	case "succeeded":
		eventType = models.DeployEventSucceeded
	case "failed":
		eventType = models.DeployEventFailed
	case "pending":
		eventType = models.DeployEventStarted
	default:
		return models.DeployEvent{}, fmt.Errorf("unsupported Heroku release status %q", p.Data.Status)
	}

	version := extractVersion(body, integration.VersionExtractors)
	if version == "" {
		switch {
		case p.Data.Slug.Commit != "":
			version = p.Data.Slug.Commit
		case p.Data.Version > 0:
			version = fmt.Sprintf("v%d", p.Data.Version)
		case p.Data.ID != "":
			version = p.Data.ID
		}
	}
	if version == "" {
		return models.DeployEvent{}, errors.New("could not extract version from Heroku payload")
	}

	occurred := p.Data.UpdatedAt
	if occurred.IsZero() {
		occurred = time.Now().UTC()
	}

	return models.DeployEvent{
		EventType:   eventType,
		Environment: p.Data.App.Name,
		Version:     version,
		CommitSHA:   p.Data.Slug.Commit,
		OccurredAt:  occurred,
		Metadata:    body,
	}, nil
}
