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

// RailwayAdapter maps Railway's deploy webhook payload into the canonical
// DeployEvent shape. Railway signs with HMAC-SHA256 delivered in the
// X-Railway-Signature header (`sha256=<hex>`).
type RailwayAdapter struct{}

func (RailwayAdapter) Provider() string { return models.DeployProviderRailway }

func (RailwayAdapter) VerifySignature(r *http.Request, body []byte, secret string) error {
	sig := r.Header.Get("X-Railway-Signature")
	if sig == "" {
		return ErrInvalidSignature
	}
	return verifyHMACSignature(sig, body, secret)
}

// railwayPayload is a permissive view of Railway's webhook JSON. Railway
// doesn't publish a versioned schema, so we read the fields we care about
// and tolerate everything else.
type railwayPayload struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Meta   struct {
		DeploymentID string `json:"deploymentId"`
	} `json:"meta"`
	Commit struct {
		SHA string `json:"sha"`
		Tag string `json:"tag"`
	} `json:"commit"`
	Image struct {
		Tag string `json:"tag"`
	} `json:"image"`
	Environment struct {
		Name string `json:"name"`
	} `json:"environment"`
	Service struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"service"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
}

func (RailwayAdapter) ParsePayload(body []byte, integration *models.DeployIntegration) (models.DeployEvent, error) {
	var p railwayPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return models.DeployEvent{}, fmt.Errorf("parse railway payload: %w", err)
	}
	if !strings.EqualFold(p.Type, "DEPLOY") {
		return models.DeployEvent{}, fmt.Errorf("unsupported Railway event type %q", p.Type)
	}

	eventType := ""
	switch strings.ToUpper(p.Status) {
	case "SUCCESS", "SUCCEEDED", "COMPLETED":
		eventType = models.DeployEventSucceeded
	case "FAILED":
		eventType = models.DeployEventFailed
	case "CRASHED":
		eventType = models.DeployEventCrashed
	case "DEPLOYING", "BUILDING", "QUEUED":
		eventType = models.DeployEventStarted
	default:
		return models.DeployEvent{}, fmt.Errorf("unknown Railway deploy status %q", p.Status)
	}

	// Version extraction — try the integration-provided chain first, then
	// fall back to Railway-specific heuristics.
	version := extractVersion(body, integration.VersionExtractors)
	if version == "" {
		switch {
		case p.Meta.DeploymentID != "":
			version = p.Meta.DeploymentID
		case p.Commit.SHA != "":
			version = p.Commit.SHA
		case p.Commit.Tag != "":
			version = p.Commit.Tag
		case p.Image.Tag != "":
			version = p.Image.Tag
		}
	}
	if version == "" {
		return models.DeployEvent{}, errors.New("could not extract version from Railway payload")
	}

	occurred := p.CreatedAt
	if occurred.IsZero() {
		occurred = time.Now().UTC()
	}

	return models.DeployEvent{
		EventType:   eventType,
		Environment: p.Environment.Name,
		Version:     version,
		CommitSHA:   p.Commit.SHA,
		Artifact:    p.Image.Tag,
		OccurredAt:  occurred,
		URL:         p.URL,
		Metadata:    body,
	}, nil
}

// extractVersion walks a list of simple dot paths (e.g. "meta.deploymentId")
// and returns the first non-empty string it finds in the JSON body.
// Supports only string leaves; array indexing is out of scope.
func extractVersion(body []byte, paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return ""
	}
	for _, p := range paths {
		if v := walkPath(root, strings.Split(p, ".")); v != "" {
			return v
		}
	}
	return ""
}

func walkPath(cur any, parts []string) string {
	for _, part := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[part]
	}
	s, _ := cur.(string)
	return s
}
