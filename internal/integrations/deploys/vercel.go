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

	"github.com/shadsorg/deploysentry/internal/models"
)

// VercelAdapter consumes Vercel deployment webhook events. Signed with
// `x-vercel-signature: <hex>` (no `sha256=` prefix) over the raw body.
type VercelAdapter struct{}

func (VercelAdapter) Provider() string { return models.DeployProviderVercel }

func (VercelAdapter) VerifySignature(r *http.Request, body []byte, secret string) error {
	given := r.Header.Get("x-vercel-signature")
	if given == "" {
		return ErrInvalidSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !subtleEqual(given, expected) {
		return ErrInvalidSignature
	}
	return nil
}

type vercelPayload struct {
	Type    string `json:"type"`
	Payload struct {
		Deployment struct {
			ID   string `json:"id"`
			URL  string `json:"url"`
			Meta struct {
				GithubCommitSha string `json:"githubCommitSha"`
				GitlabCommitSha string `json:"gitlabCommitSha"`
			} `json:"meta"`
		} `json:"deployment"`
		Target  string `json:"target"`
		Project struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
	} `json:"payload"`
	CreatedAt int64 `json:"createdAt"`
}

func (VercelAdapter) ParsePayload(body []byte, integration *models.DeployIntegration) (models.DeployEvent, error) {
	var p vercelPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return models.DeployEvent{}, fmt.Errorf("parse vercel payload: %w", err)
	}
	eventType := ""
	switch p.Type {
	case "deployment.succeeded", "deployment.ready":
		eventType = models.DeployEventSucceeded
	case "deployment.error", "deployment.failed", "deployment.canceled":
		eventType = models.DeployEventFailed
	case "deployment.created":
		eventType = models.DeployEventStarted
	default:
		return models.DeployEvent{}, fmt.Errorf("unsupported Vercel event %q", p.Type)
	}

	version := extractVersion(body, integration.VersionExtractors)
	if version == "" {
		switch {
		case p.Payload.Deployment.Meta.GithubCommitSha != "":
			version = p.Payload.Deployment.Meta.GithubCommitSha
		case p.Payload.Deployment.Meta.GitlabCommitSha != "":
			version = p.Payload.Deployment.Meta.GitlabCommitSha
		case p.Payload.Deployment.ID != "":
			version = p.Payload.Deployment.ID
		}
	}
	if version == "" {
		return models.DeployEvent{}, errors.New("could not extract version from Vercel payload")
	}

	env := p.Payload.Target
	if env == "" {
		env = "production"
	}

	commit := p.Payload.Deployment.Meta.GithubCommitSha
	if commit == "" {
		commit = p.Payload.Deployment.Meta.GitlabCommitSha
	}

	occurred := time.Now().UTC()
	if p.CreatedAt > 0 {
		occurred = time.Unix(p.CreatedAt/1000, 0).UTC()
	}

	url := p.Payload.Deployment.URL
	if url != "" && !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}

	return models.DeployEvent{
		EventType:   eventType,
		Environment: env,
		Version:     version,
		CommitSHA:   commit,
		OccurredAt:  occurred,
		URL:         url,
		Metadata:    body,
	}, nil
}
