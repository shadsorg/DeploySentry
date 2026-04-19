package rollout

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func newTestRolloutHandler() (*RolloutHandler, *fakeRolloutRepo) {
	gin.SetMode(gin.TestMode)
	repo := newFakeRolloutRepo()
	svc := NewRolloutService(repo, &fakePhaseRepo{}, &fakeEventRepo{}, &fakePublisher{})
	return NewRolloutHandler(svc), repo
}

func TestRolloutHandler_Rollback_RequiresReason(t *testing.T) {
	h, repo := newTestRolloutHandler()
	// Seed an active rollout.
	ro := &models.Rollout{
		ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive,
		TargetRef: models.RolloutTargetRef{DeploymentID: func() *string { s := uuid.NewString(); return &s }()},
	}
	repo.rows[ro.ID] = ro
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	req := httptest.NewRequest("POST", "/api/v1/orgs/a/rollouts/"+ro.ID.String()+"/rollback", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400 got %d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("POST", "/api/v1/orgs/a/rollouts/"+ro.ID.String()+"/rollback", bytes.NewBufferString(`{"reason":"broken"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d body=%s", w.Code, w.Body.String())
	}
	if ro.Status != models.RolloutRolledBack {
		t.Fatalf("not rolled back: %s", ro.Status)
	}
}

func TestRolloutHandler_ForcePromote_RequiresReason(t *testing.T) {
	h, repo := newTestRolloutHandler()
	ro := &models.Rollout{
		ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive,
		TargetRef: models.RolloutTargetRef{DeploymentID: func() *string { s := uuid.NewString(); return &s }()},
	}
	repo.rows[ro.ID] = ro
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	req := httptest.NewRequest("POST", "/api/v1/orgs/a/rollouts/"+ro.ID.String()+"/force-promote", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestRolloutHandler_Pause_ActiveOnly(t *testing.T) {
	h, repo := newTestRolloutHandler()
	ro := &models.Rollout{ID: uuid.New(), Status: models.RolloutPending}
	repo.rows[ro.ID] = ro
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	req := httptest.NewRequest("POST", "/api/v1/orgs/a/rollouts/"+ro.ID.String()+"/pause", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Pending cannot be paused → 409 Conflict.
	if w.Code != 409 {
		t.Fatalf("expected 409, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRolloutHandler_Get_ReturnsShape(t *testing.T) {
	h, repo := newTestRolloutHandler()
	ro := &models.Rollout{ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive}
	repo.rows[ro.ID] = ro
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	req := httptest.NewRequest("GET", "/api/v1/orgs/a/rollouts/"+ro.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d", w.Code)
	}
	var out models.Rollout
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.ID != ro.ID || out.Status != models.RolloutActive {
		t.Fatalf("unexpected: %+v", out)
	}
	_ = context.Background
}

var _ = http.StatusOK // silence import if unused in test trim
