package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/deploy"
	"github.com/deploysentry/deploysentry/internal/models"
)

// WorkflowRunHandler receives GitHub `workflow_run` events and writes
// record-mode deployment rows that the Org Status board surfaces as
// "build in progress" / "build failed" pills.
//
// Auth: API-key with `status:write` scope, scoped to a single
// (application, environment). Symmetric with the /applications/:id/status
// contract, so the same key style works for both SDK health pushes and
// CI build status.
type WorkflowRunHandler struct {
	repo deploy.DeployRepository
}

// NewWorkflowRunHandler constructs the handler.
func NewWorkflowRunHandler(repo deploy.DeployRepository) *WorkflowRunHandler {
	return &WorkflowRunHandler{repo: repo}
}

// RegisterRoutes mounts POST /applications/:app_id/integrations/github/workflow.
// Route is scoped under applications so the app_id is explicit; the env is
// resolved from the API key's environment scope (exactly one env required).
func (h *WorkflowRunHandler) RegisterRoutes(rg *gin.RouterGroup, rbac *auth.RBACChecker) {
	g := rg.Group("/applications")
	g.POST("/:app_id/integrations/github/workflow", mw(rbac, auth.PermStatusWrite), h.handle)
}

func mw(rbac *auth.RBACChecker, perm auth.Permission) gin.HandlerFunc {
	if rbac == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return auth.RequirePermission(rbac, perm)
}

// workflowRunEvent is the subset of GitHub's `workflow_run` webhook payload
// that the adapter cares about. Everything else is ignored.
type workflowRunEvent struct {
	Action      string `json:"action"`
	WorkflowRun struct {
		Name          string    `json:"name"`
		Status        string    `json:"status"`
		Conclusion    string    `json:"conclusion"`
		HeadSHA       string    `json:"head_sha"`
		HeadBranch    string    `json:"head_branch"`
		HTMLURL       string    `json:"html_url"`
		RunStartedAt  time.Time `json:"run_started_at"`
		UpdatedAt     time.Time `json:"updated_at"`
		CreatedAt     time.Time `json:"created_at"`
	} `json:"workflow_run"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func (h *WorkflowRunHandler) handle(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("app_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}
	envID, rerr := resolveEnvForKey(c, appID)
	if rerr != nil {
		c.JSON(rerr.status, gin.H{"error": rerr.msg})
		return
	}
	// created_by is the ancestor user on the API key. Same fallback chain
	// the deploy + appstatus handlers now use.
	createdBy := auth.ActorUserID(c)
	if method, _ := c.Get("auth_method"); method == "api_key" && createdBy == uuid.Nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "cannot determine created_by: the calling api key has no associated user on record; " +
				"mint a key from the dashboard (which stamps the creating user) and retry",
		})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	// Optional HMAC verification. The secret lives on the api_keys row's
	// metadata under "github_secret"; when absent the hook trusts the
	// Authorization header alone.
	if secret := apiKeyGitHubSecret(c); secret != "" {
		sig := c.GetHeader("X-Hub-Signature-256")
		if !verifySignature(body, sig, secret) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid webhook signature"})
			return
		}
	}

	// Ping events are sent when GitHub first activates the webhook;
	// respond 200 so GitHub marks the delivery successful without
	// creating a deploy row.
	if evt := c.GetHeader("X-GitHub-Event"); evt == "ping" {
		c.JSON(http.StatusOK, gin.H{"status": "pong"})
		return
	}

	var event workflowRunEvent
	if err := json.Unmarshal(body, &event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow_run payload"})
		return
	}
	if event.WorkflowRun.HeadSHA == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow_run payload missing head_sha"})
		return
	}

	status, skip := statusFromEvent(event)
	if skip {
		c.JSON(http.StatusAccepted, gin.H{"action": "noop", "reason": "unhandled action/conclusion"})
		return
	}

	id, created, uerr := h.repo.UpsertBuildDeployment(c.Request.Context(), deploy.BuildDeploymentUpsert{
		ApplicationID: appID,
		EnvironmentID: envID,
		CommitSHA:     event.WorkflowRun.HeadSHA,
		WorkflowName:  event.WorkflowRun.Name,
		Version:       composeVersion(event),
		Artifact:      event.Repository.FullName,
		Status:        status,
		HTMLURL:       event.WorkflowRun.HTMLURL,
		CreatedBy:     createdBy,
		StartedAt:     startedAtPtr(event),
		CompletedAt:   completedAtPtr(event, status),
	})
	if uerr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": uerr.Error()})
		return
	}
	actionTag := "updated"
	if created {
		actionTag = "created"
	}
	c.JSON(http.StatusAccepted, gin.H{
		"deployment_id": id,
		"action":        actionTag,
		"status":        status,
	})
}

// statusFromEvent maps a GitHub workflow_run action+conclusion into the
// DeploySentry deployments.status enum. Returns skip=true for payloads
// the hook intentionally ignores (e.g. `requested` without run_started_at).
func statusFromEvent(e workflowRunEvent) (models.DeployStatus, bool) {
	switch e.Action {
	case "requested", "in_progress":
		return models.DeployStatusRunning, false
	case "completed":
		switch e.WorkflowRun.Conclusion {
		case "success":
			return models.DeployStatusCompleted, false
		case "failure", "timed_out", "action_required", "startup_failure":
			return models.DeployStatusFailed, false
		case "cancelled", "skipped":
			return models.DeployStatusCancelled, false
		case "neutral", "stale", "":
			return models.DeployStatusCompleted, false
		}
	}
	return "", true
}

func composeVersion(e workflowRunEvent) string {
	sha := e.WorkflowRun.HeadSHA
	short := sha
	if len(short) > 7 {
		short = short[:7]
	}
	if b := e.WorkflowRun.HeadBranch; b != "" {
		return fmt.Sprintf("%s@%s", b, short)
	}
	return short
}

func startedAtPtr(e workflowRunEvent) *time.Time {
	if !e.WorkflowRun.RunStartedAt.IsZero() {
		t := e.WorkflowRun.RunStartedAt
		return &t
	}
	if !e.WorkflowRun.CreatedAt.IsZero() {
		t := e.WorkflowRun.CreatedAt
		return &t
	}
	return nil
}

func completedAtPtr(e workflowRunEvent, status models.DeployStatus) *time.Time {
	if status == models.DeployStatusRunning {
		return nil
	}
	if !e.WorkflowRun.UpdatedAt.IsZero() {
		t := e.WorkflowRun.UpdatedAt
		return &t
	}
	now := time.Now().UTC()
	return &now
}

// -----------------------------------------------------------------------------
// Env resolution mirrors internal/appstatus: the key must be scoped to
// exactly one environment and (if app-scoped) to the requested app.

type resolveError struct {
	status int
	msg    string
}

func resolveEnvForKey(c *gin.Context, appID uuid.UUID) (uuid.UUID, *resolveError) {
	if method, _ := c.Get("auth_method"); method != "api_key" {
		// Session / JWT: accept env via query string for smoke-testing.
		if envStr := c.Query("environment_id"); envStr != "" {
			envID, err := uuid.Parse(envStr)
			if err != nil {
				return uuid.Nil, &resolveError{http.StatusBadRequest, "invalid environment_id"}
			}
			return envID, nil
		}
		return uuid.Nil, &resolveError{http.StatusBadRequest, "environment_id is required"}
	}
	if v, ok := c.Get("api_key_app_id"); ok {
		if s, ok := v.(string); ok && s != "" {
			keyAppID, err := uuid.Parse(s)
			if err != nil || keyAppID != appID {
				return uuid.Nil, &resolveError{http.StatusForbidden, "api key is not scoped to this application"}
			}
		}
	}
	envsVal, ok := c.Get("api_key_environment_ids")
	if !ok {
		return uuid.Nil, &resolveError{http.StatusBadRequest, "status:write api key must be scoped to a single environment"}
	}
	envs, ok := envsVal.([]string)
	if !ok || len(envs) != 1 {
		return uuid.Nil, &resolveError{http.StatusBadRequest, "status:write api key must be scoped to exactly one environment"}
	}
	envID, err := uuid.Parse(envs[0])
	if err != nil {
		return uuid.Nil, &resolveError{http.StatusInternalServerError, "invalid environment id on api key"}
	}
	return envID, nil
}

// apiKeyGitHubSecret looks up the optional HMAC secret stored on the API
// key. v1 does not yet provide a UI/CLI surface for setting this; returning
// "" means "no signature verification", which is safe because the handler
// is already behind bearer-token auth.
func apiKeyGitHubSecret(c *gin.Context) string {
	if v, ok := c.Get("api_key_github_secret"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

