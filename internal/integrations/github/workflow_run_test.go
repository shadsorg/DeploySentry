package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/shadsorg/deploysentry/internal/deploy"
	"github.com/shadsorg/deploysentry/internal/models"
)

// fakeRepo implements just the DeployRepository methods the handler uses.
// Every other method is a no-op; we only care about UpsertBuildDeployment.
type fakeRepo struct {
	deploy.DeployRepository // embed for forward-compat; tests only call Upsert
	calls                   []deploy.BuildDeploymentUpsert
	returnErr               error
}

func (f *fakeRepo) UpsertBuildDeployment(_ context.Context, in deploy.BuildDeploymentUpsert) (uuid.UUID, bool, error) {
	if f.returnErr != nil {
		return uuid.Nil, false, f.returnErr
	}
	f.calls = append(f.calls, in)
	// First call == created; subsequent calls on the same key == updated.
	created := len(f.calls) == 1 ||
		f.calls[0].CommitSHA != in.CommitSHA ||
		f.calls[0].WorkflowName != in.WorkflowName
	return uuid.New(), created, nil
}

// setup wires a router that stamps fake api_key context keys the same way
// the real middleware would.
func setup(t *testing.T, appID, envID uuid.UUID) (*gin.Engine, *fakeRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	creator := uuid.New()
	r.Use(func(c *gin.Context) {
		c.Set("auth_method", "api_key")
		c.Set("api_key_created_by", creator.String())
		c.Set("api_key_app_id", appID.String())
		c.Set("api_key_environment_ids", []string{envID.String()})
		c.Next()
	})
	repo := &fakeRepo{}
	h := NewWorkflowRunHandler(repo)
	h.RegisterRoutes(r.Group("/api/v1"), nil) // nil rbac = permit
	return r, repo
}

func fire(t *testing.T, r *gin.Engine, appID uuid.UUID, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/applications/"+appID.String()+"/integrations/github/workflow",
		bytes.NewReader(raw),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "workflow_run")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestWorkflowRun_InProgress(t *testing.T) {
	appID, envID := uuid.New(), uuid.New()
	r, repo := setup(t, appID, envID)

	body := map[string]any{
		"action": "in_progress",
		"workflow_run": map[string]any{
			"name":        "CI",
			"status":      "in_progress",
			"head_sha":    "abcdef1234567890",
			"head_branch": "main",
			"html_url":    "https://github.com/org/repo/actions/runs/1",
		},
		"repository": map[string]any{"full_name": "org/repo"},
	}
	w := fire(t, r, appID, body)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	if len(repo.calls) != 1 {
		t.Fatalf("expected 1 upsert call, got %d", len(repo.calls))
	}
	got := repo.calls[0]
	if got.Status != models.DeployStatusRunning {
		t.Fatalf("status: want running, got %s", got.Status)
	}
	if got.Version != "main@abcdef1" {
		t.Fatalf("version: want main@abcdef1, got %s", got.Version)
	}
	if got.ApplicationID != appID || got.EnvironmentID != envID {
		t.Fatalf("app/env mismatch: %+v", got)
	}
}

func TestWorkflowRun_CompletedSuccess(t *testing.T) {
	appID, envID := uuid.New(), uuid.New()
	r, repo := setup(t, appID, envID)
	body := map[string]any{
		"action": "completed",
		"workflow_run": map[string]any{
			"name":        "CI",
			"status":      "completed",
			"conclusion":  "success",
			"head_sha":    "abcdef1234567890",
			"head_branch": "main",
		},
		"repository": map[string]any{"full_name": "org/repo"},
	}
	w := fire(t, r, appID, body)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	if repo.calls[0].Status != models.DeployStatusCompleted {
		t.Fatalf("status: want completed, got %s", repo.calls[0].Status)
	}
	if repo.calls[0].CompletedAt == nil {
		t.Fatalf("expected completed_at set for terminal status")
	}
}

func TestWorkflowRun_CompletedFailure(t *testing.T) {
	appID, envID := uuid.New(), uuid.New()
	r, repo := setup(t, appID, envID)
	body := map[string]any{
		"action": "completed",
		"workflow_run": map[string]any{
			"name":        "CI",
			"status":      "completed",
			"conclusion":  "failure",
			"head_sha":    "abcdef1234567890",
			"head_branch": "main",
		},
	}
	w := fire(t, r, appID, body)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}
	if repo.calls[0].Status != models.DeployStatusFailed {
		t.Fatalf("status: want failed, got %s", repo.calls[0].Status)
	}
}

func TestWorkflowRun_MissingHeadSHA(t *testing.T) {
	appID, envID := uuid.New(), uuid.New()
	r, _ := setup(t, appID, envID)
	body := map[string]any{
		"action":       "in_progress",
		"workflow_run": map[string]any{"name": "CI"},
	}
	w := fire(t, r, appID, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing head_sha, got %d", w.Code)
	}
}

func TestWorkflowRun_AppScopeMismatch(t *testing.T) {
	appID, envID := uuid.New(), uuid.New()
	r, _ := setup(t, appID, envID)
	body := map[string]any{
		"action": "in_progress",
		"workflow_run": map[string]any{
			"name":     "CI",
			"head_sha": "abcdef1234567890",
		},
	}
	// Fire at a different app URL — the middleware-stamped api_key_app_id
	// is appID, so this should 403.
	raw, _ := json.Marshal(body)
	other := uuid.New()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/applications/"+other.String()+"/integrations/github/workflow",
		bytes.NewReader(raw),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on app mismatch, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWorkflowRun_PingEvent(t *testing.T) {
	appID, envID := uuid.New(), uuid.New()
	r, repo := setup(t, appID, envID)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/applications/"+appID.String()+"/integrations/github/workflow",
		bytes.NewReader([]byte(`{}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "ping")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for ping, got %d", w.Code)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("ping should not upsert, got %d calls", len(repo.calls))
	}
}

func TestWorkflowRun_RepoError(t *testing.T) {
	appID, envID := uuid.New(), uuid.New()
	r, repo := setup(t, appID, envID)
	repo.returnErr = errors.New("db down")
	body := map[string]any{
		"action": "in_progress",
		"workflow_run": map[string]any{
			"name":     "CI",
			"head_sha": "abcdef1234567890",
		},
	}
	w := fire(t, r, appID, body)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on repo error, got %d", w.Code)
	}
}
