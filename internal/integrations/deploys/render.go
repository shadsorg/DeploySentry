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

// RenderAdapter maps Render's webhook payload into the canonical
// DeployEvent shape. Render signs deliveries using
// `Render-Webhook-Signature: t=<ts>,v1=<hex>` where the HMAC is over
// `ts + "." + body`.
type RenderAdapter struct{}

func (RenderAdapter) Provider() string { return models.DeployProviderRender }

func (RenderAdapter) VerifySignature(r *http.Request, body []byte, secret string) error {
	header := r.Header.Get("Render-Webhook-Signature")
	if header == "" {
		return ErrInvalidSignature
	}
	var ts, v1 string
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			ts = kv[1]
		case "v1":
			v1 = kv[1]
		}
	}
	if ts == "" || v1 == "" {
		return ErrInvalidSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "." + string(body)))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !subtleEqual(v1, expected) {
		return ErrInvalidSignature
	}
	return nil
}

type renderPayload struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Data      struct {
		Deploy struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Commit struct {
				ID string `json:"id"`
			} `json:"commit"`
		} `json:"deploy"`
		Service struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"service"`
	} `json:"data"`
}

func (RenderAdapter) ParsePayload(body []byte, integration *models.DeployIntegration) (models.DeployEvent, error) {
	var p renderPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return models.DeployEvent{}, fmt.Errorf("parse render payload: %w", err)
	}
	eventType := ""
	switch strings.ToLower(p.Data.Deploy.Status) {
	case "live", "succeeded":
		eventType = models.DeployEventSucceeded
	case "update_failed", "build_failed", "failed", "canceled":
		eventType = models.DeployEventFailed
	case "created", "build_in_progress", "update_in_progress":
		eventType = models.DeployEventStarted
	default:
		return models.DeployEvent{}, fmt.Errorf("unsupported Render deploy status %q", p.Data.Deploy.Status)
	}

	version := extractVersion(body, integration.VersionExtractors)
	if version == "" {
		switch {
		case p.Data.Deploy.Commit.ID != "":
			version = p.Data.Deploy.Commit.ID
		case p.Data.Deploy.ID != "":
			version = p.Data.Deploy.ID
		}
	}
	if version == "" {
		return models.DeployEvent{}, errors.New("could not extract version from Render payload")
	}

	// Render doesn't send an explicit environment; users map service name → env.
	environment := p.Data.Service.Name

	occurred, _ := time.Parse(time.RFC3339, p.Timestamp)
	if occurred.IsZero() {
		occurred = time.Now().UTC()
	}

	return models.DeployEvent{
		EventType:   eventType,
		Environment: environment,
		Version:     version,
		CommitSHA:   p.Data.Deploy.Commit.ID,
		OccurredAt:  occurred,
		Metadata:    body,
	}, nil
}
