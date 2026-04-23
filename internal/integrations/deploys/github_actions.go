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

// GitHubActionsAdapter consumes GitHub `workflow_run` webhook events.
// Signed using the classic GitHub `X-Hub-Signature-256: sha256=<hex>`
// header over the raw body.
type GitHubActionsAdapter struct{}

func (GitHubActionsAdapter) Provider() string { return models.DeployProviderGitHubActions }

func (GitHubActionsAdapter) VerifySignature(r *http.Request, body []byte, secret string) error {
	sig := r.Header.Get("X-Hub-Signature-256")
	if sig == "" {
		return ErrInvalidSignature
	}
	return verifyHMACSignature(sig, body, secret)
}

type githubActionsPayload struct {
	Action      string `json:"action"`
	WorkflowRun struct {
		ID         int64     `json:"id"`
		Name       string    `json:"name"`
		HeadBranch string    `json:"head_branch"`
		HeadSha    string    `json:"head_sha"`
		Conclusion string    `json:"conclusion"`
		HTMLURL    string    `json:"html_url"`
		UpdatedAt  time.Time `json:"updated_at"`
		WorkflowID int64     `json:"workflow_id"`
	} `json:"workflow_run"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func (GitHubActionsAdapter) ParsePayload(body []byte, integration *models.DeployIntegration) (models.DeployEvent, error) {
	var p githubActionsPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return models.DeployEvent{}, fmt.Errorf("parse github actions payload: %w", err)
	}
	if p.Action != "completed" {
		return models.DeployEvent{}, fmt.Errorf("ignoring workflow_run action %q", p.Action)
	}
	// Optional workflow filter via provider_config.workflow_name — lets users
	// isolate their "deploy" workflow from build/test workflows on the same repo.
	if integration != nil && integration.ProviderConfig != nil {
		if want, _ := integration.ProviderConfig["workflow_name"].(string); want != "" && want != p.WorkflowRun.Name {
			return models.DeployEvent{}, fmt.Errorf("workflow %q is not the configured deploy workflow %q", p.WorkflowRun.Name, want)
		}
	}

	eventType := ""
	switch strings.ToLower(p.WorkflowRun.Conclusion) {
	case "success":
		eventType = models.DeployEventSucceeded
	case "failure", "cancelled", "timed_out":
		eventType = models.DeployEventFailed
	default:
		return models.DeployEvent{}, fmt.Errorf("unsupported workflow_run conclusion %q", p.WorkflowRun.Conclusion)
	}

	version := extractVersion(body, integration.VersionExtractors)
	if version == "" {
		version = p.WorkflowRun.HeadSha
	}
	if version == "" {
		return models.DeployEvent{}, errors.New("workflow_run.head_sha is required")
	}

	env := p.WorkflowRun.HeadBranch
	// Allow users to pin a single-env deploy to a fixed mapping key.
	if integration != nil && integration.ProviderConfig != nil {
		if forced, _ := integration.ProviderConfig["environment"].(string); forced != "" {
			env = forced
		}
	}

	occurred := p.WorkflowRun.UpdatedAt
	if occurred.IsZero() {
		occurred = time.Now().UTC()
	}

	return models.DeployEvent{
		EventType:   eventType,
		Environment: env,
		Version:     version,
		CommitSHA:   p.WorkflowRun.HeadSha,
		OccurredAt:  occurred,
		URL:         p.WorkflowRun.HTMLURL,
		Metadata:    body,
	}, nil
}
