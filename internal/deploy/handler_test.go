package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Mock
// ---------------------------------------------------------------------------

type mockDeployService struct {
	createFn            func(ctx context.Context, d *models.Deployment) error
	getFn               func(ctx context.Context, id uuid.UUID) (*models.Deployment, error)
	listFn              func(ctx context.Context, applicationID uuid.UUID, opts ListOptions) ([]*models.Deployment, error)
	promoteFn           func(ctx context.Context, id uuid.UUID) error
	rollbackFn          func(ctx context.Context, id uuid.UUID) error
	pauseFn             func(ctx context.Context, id uuid.UUID) error
	resumeFn            func(ctx context.Context, id uuid.UUID) error
	getActiveFn         func(ctx context.Context, applicationID uuid.UUID) ([]*models.Deployment, error)
	listPhasesFn        func(ctx context.Context, deploymentID uuid.UUID) ([]*models.DeploymentPhase, error)
	listRollbacksFn     func(ctx context.Context, deploymentID uuid.UUID) ([]*models.RollbackRecord, error)
	setTrafficPercentFn func(ctx context.Context, deploymentID uuid.UUID, pct int) error
}

func (m *mockDeployService) CreateDeployment(ctx context.Context, d *models.Deployment) error {
	if m.createFn != nil {
		return m.createFn(ctx, d)
	}
	return nil
}

func (m *mockDeployService) GetDeployment(ctx context.Context, id uuid.UUID) (*models.Deployment, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return &models.Deployment{ID: id}, nil
}

func (m *mockDeployService) ListDeployments(ctx context.Context, applicationID uuid.UUID, opts ListOptions) ([]*models.Deployment, error) {
	if m.listFn != nil {
		return m.listFn(ctx, applicationID, opts)
	}
	return []*models.Deployment{}, nil
}

func (m *mockDeployService) PromoteDeployment(ctx context.Context, id uuid.UUID) error {
	if m.promoteFn != nil {
		return m.promoteFn(ctx, id)
	}
	return nil
}

func (m *mockDeployService) RollbackDeployment(ctx context.Context, id uuid.UUID) error {
	if m.rollbackFn != nil {
		return m.rollbackFn(ctx, id)
	}
	return nil
}

func (m *mockDeployService) PauseDeployment(ctx context.Context, id uuid.UUID) error {
	if m.pauseFn != nil {
		return m.pauseFn(ctx, id)
	}
	return nil
}

func (m *mockDeployService) ResumeDeployment(ctx context.Context, id uuid.UUID) error {
	if m.resumeFn != nil {
		return m.resumeFn(ctx, id)
	}
	return nil
}

func (m *mockDeployService) GetActiveDeployments(ctx context.Context, applicationID uuid.UUID) ([]*models.Deployment, error) {
	if m.getActiveFn != nil {
		return m.getActiveFn(ctx, applicationID)
	}
	return []*models.Deployment{}, nil
}

func (m *mockDeployService) ListPhases(ctx context.Context, deploymentID uuid.UUID) ([]*models.DeploymentPhase, error) {
	if m.listPhasesFn != nil {
		return m.listPhasesFn(ctx, deploymentID)
	}
	return []*models.DeploymentPhase{}, nil
}

func (m *mockDeployService) ListRollbackRecords(ctx context.Context, deploymentID uuid.UUID) ([]*models.RollbackRecord, error) {
	if m.listRollbacksFn != nil {
		return m.listRollbacksFn(ctx, deploymentID)
	}
	return []*models.RollbackRecord{}, nil
}

func (m *mockDeployService) CancelDeployment(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockDeployService) SetTrafficPercent(ctx context.Context, deploymentID uuid.UUID, pct int) error {
	if m.setTrafficPercentFn != nil {
		return m.setTrafficPercentFn(ctx, deploymentID, pct)
	}
	return nil
}

func (m *mockDeployService) ListArtifacts(_ context.Context, _ uuid.UUID, _ int) ([]ArtifactSuggestion, error) {
	return nil, nil
}

func (m *mockDeployService) ListVersions(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _ int) ([]VersionSuggestion, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func setupDeployRouter(svc DeployService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(svc, nil, nil, nil)
	// Pass nil for RBAC to disable permission checks in unit tests.
	handler.RegisterRoutes(router.Group("/api"), nil)
	// Production registers /:id/rollback via the rollback package; in unit tests
	// we wire the handler method directly so TestRollbackDeployment_* can exercise it.
	router.POST("/api/deployments/:id/rollback", handler.rollbackDeployment)
	return router
}

func toJSON(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	assert.NoError(t, err)
	return bytes.NewBuffer(b)
}

// ---------------------------------------------------------------------------
// POST /deployments  (createDeployment)
// ---------------------------------------------------------------------------

func TestCreateDeployment_Valid(t *testing.T) {
	svc := &mockDeployService{
		createFn: func(_ context.Context, d *models.Deployment) error {
			d.ID = uuid.New()
			return nil
		},
	}
	router := setupDeployRouter(svc)

	body := map[string]interface{}{
		"application_id": uuid.New().String(),
		"environment_id": uuid.New().String(),
		"strategy":       "canary",
		"artifact":       "myapp:v1.2.3",
		"version":        "v1.2.3",
		"commit_sha":     "abc123",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/deployments", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp models.Deployment
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "myapp:v1.2.3", resp.Artifact)
}

func TestCreateDeployment_InvalidJSON(t *testing.T) {
	router := setupDeployRouter(&mockDeployService{})

	req := httptest.NewRequest(http.MethodPost, "/api/deployments", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateDeployment_ServiceError(t *testing.T) {
	svc := &mockDeployService{
		createFn: func(_ context.Context, _ *models.Deployment) error {
			return errors.New("validation failed")
		},
	}
	router := setupDeployRouter(svc)

	body := map[string]interface{}{
		"application_id": uuid.New().String(),
		"environment_id": uuid.New().String(),
		"strategy":       "canary",
		"artifact":       "myapp:v1.0.0",
		"version":        "v1.0.0",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/deployments", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ---------------------------------------------------------------------------
// GET /deployments/:id  (getDeployment)
// ---------------------------------------------------------------------------

func TestGetDeployment_Valid(t *testing.T) {
	depID := uuid.New()
	svc := &mockDeployService{
		getFn: func(_ context.Context, id uuid.UUID) (*models.Deployment, error) {
			return &models.Deployment{ID: id, Artifact: "myapp:v1", Version: "v1"}, nil
		},
	}
	router := setupDeployRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/deployments/"+depID.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.Deployment
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, depID, resp.ID)
}

func TestGetDeployment_InvalidID(t *testing.T) {
	router := setupDeployRouter(&mockDeployService{})

	req := httptest.NewRequest(http.MethodGet, "/api/deployments/not-a-uuid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetDeployment_NotFound(t *testing.T) {
	svc := &mockDeployService{
		getFn: func(_ context.Context, _ uuid.UUID) (*models.Deployment, error) {
			return nil, errors.New("not found")
		},
	}
	router := setupDeployRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/deployments/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---------------------------------------------------------------------------
// GET /deployments?app_id=  (listDeployments)
// ---------------------------------------------------------------------------

func TestListDeployments_Valid(t *testing.T) {
	applicationID := uuid.New()
	svc := &mockDeployService{
		listFn: func(_ context.Context, appID uuid.UUID, _ ListOptions) ([]*models.Deployment, error) {
			return []*models.Deployment{
				{ID: uuid.New(), ApplicationID: appID, Artifact: "app:v1"},
			}, nil
		},
	}
	router := setupDeployRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/deployments?app_id="+applicationID.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]json.RawMessage
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "deployments")
}

func TestListDeployments_MissingAppID(t *testing.T) {
	router := setupDeployRouter(&mockDeployService{})

	req := httptest.NewRequest(http.MethodGet, "/api/deployments", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListDeployments_InvalidAppID(t *testing.T) {
	router := setupDeployRouter(&mockDeployService{})

	req := httptest.NewRequest(http.MethodGet, "/api/deployments?app_id=invalid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListDeployments_EnvFilter(t *testing.T) {
	applicationID := uuid.New()
	environmentID := uuid.New()
	var capturedOpts ListOptions
	svc := &mockDeployService{
		listFn: func(_ context.Context, _ uuid.UUID, opts ListOptions) ([]*models.Deployment, error) {
			capturedOpts = opts
			return nil, nil
		},
	}
	router := setupDeployRouter(svc)

	url := "/api/deployments?app_id=" + applicationID.String() + "&environment_id=" + environmentID.String() + "&limit=5&offset=10"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	if assert.NotNil(t, capturedOpts.EnvironmentID) {
		assert.Equal(t, environmentID, *capturedOpts.EnvironmentID)
	}
	assert.Equal(t, 5, capturedOpts.Limit)
	assert.Equal(t, 10, capturedOpts.Offset)
}

func TestListDeployments_InvalidEnvID(t *testing.T) {
	router := setupDeployRouter(&mockDeployService{})

	url := "/api/deployments?app_id=" + uuid.New().String() + "&environment_id=not-a-uuid"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// POST /deployments with mode=record  (agentless reporting — Phase 1)
// ---------------------------------------------------------------------------

func TestCreateDeployment_ModeRecord_DefaultsCompleted(t *testing.T) {
	var captured *models.Deployment
	svc := &mockDeployService{
		createFn: func(_ context.Context, d *models.Deployment) error {
			// Simulate the service's record-mode shortcut so the handler's
			// response reflects the final stored state.
			d.ID = uuid.New()
			d.Status = models.DeployStatusCompleted
			d.TrafficPercent = 100
			captured = d
			return nil
		},
	}
	router := setupDeployRouter(svc)

	body := map[string]interface{}{
		"application_id": uuid.New().String(),
		"environment_id": uuid.New().String(),
		"artifact":       "img:1.4.2",
		"version":        "1.4.2",
		"mode":           "record",
		"source":         "railway-webhook",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/deployments", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	if assert.NotNil(t, captured) {
		assert.Equal(t, models.DeployModeRecord, captured.Mode)
		if assert.NotNil(t, captured.Source) {
			assert.Equal(t, "railway-webhook", *captured.Source)
		}
	}

	var resp models.Deployment
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, models.DeployStatusCompleted, resp.Status)
	assert.Equal(t, 100, resp.TrafficPercent)
}

func TestCreateDeployment_ModeRecord_StrategyOptional(t *testing.T) {
	svc := &mockDeployService{
		createFn: func(_ context.Context, d *models.Deployment) error {
			d.ID = uuid.New()
			return nil
		},
	}
	router := setupDeployRouter(svc)

	body := map[string]interface{}{
		"application_id": uuid.New().String(),
		"environment_id": uuid.New().String(),
		"artifact":       "img:1",
		"version":        "1",
		"mode":           "record",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/deployments", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateDeployment_ModeRecord_SkipsRollout(t *testing.T) {
	attachCalled := false
	svc := &mockDeployService{
		createFn: func(_ context.Context, d *models.Deployment) error {
			d.ID = uuid.New()
			return nil
		},
	}
	rollouts := &mockRolloutAttacher{
		attachFn: func(_ context.Context, _ *models.Deployment, _ *RolloutAttachRequest, _ uuid.UUID) error {
			attachCalled = true
			return nil
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandlerWithRollouts(svc, nil, nil, nil, rollouts)
	handler.RegisterRoutes(router.Group("/api"), nil)

	body := map[string]interface{}{
		"application_id": uuid.New().String(),
		"environment_id": uuid.New().String(),
		"artifact":       "img:1",
		"version":        "1",
		"mode":           "record",
		"rollout":        map[string]interface{}{"strategy_name": "canary"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/deployments", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.False(t, attachCalled, "rollout attach must not run for mode=record")
}

func TestCreateDeployment_InvalidMode(t *testing.T) {
	router := setupDeployRouter(&mockDeployService{})

	body := map[string]interface{}{
		"application_id": uuid.New().String(),
		"environment_id": uuid.New().String(),
		"artifact":       "img:1",
		"version":        "1",
		"mode":           "totally-wrong",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/deployments", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateDeployment_OrchestrateRequiresStrategy(t *testing.T) {
	router := setupDeployRouter(&mockDeployService{})

	body := map[string]interface{}{
		"application_id": uuid.New().String(),
		"environment_id": uuid.New().String(),
		"artifact":       "img:1",
		"version":        "1",
		// no mode, no strategy
	}
	req := httptest.NewRequest(http.MethodPost, "/api/deployments", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// mockRolloutAttacher satisfies RolloutAttacher for the record-skips-rollout test.
type mockRolloutAttacher struct {
	attachFn func(ctx context.Context, d *models.Deployment, req *RolloutAttachRequest, actor uuid.UUID) error
}

func (m *mockRolloutAttacher) AttachFromDeployRequest(ctx context.Context, d *models.Deployment, req *RolloutAttachRequest, actor uuid.UUID) error {
	if m.attachFn != nil {
		return m.attachFn(ctx, d, req, actor)
	}
	return nil
}

// ---------------------------------------------------------------------------
// POST /deployments/:id/promote  (promoteDeployment)
// ---------------------------------------------------------------------------

func TestPromoteDeployment_Valid(t *testing.T) {
	svc := &mockDeployService{
		promoteFn: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
	}
	router := setupDeployRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/deployments/"+uuid.New().String()+"/promote", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "promoting", resp["status"])
}

func TestPromoteDeployment_InvalidID(t *testing.T) {
	router := setupDeployRouter(&mockDeployService{})

	req := httptest.NewRequest(http.MethodPost, "/api/deployments/bad-id/promote", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPromoteDeployment_ServiceError(t *testing.T) {
	svc := &mockDeployService{
		promoteFn: func(_ context.Context, _ uuid.UUID) error {
			return errors.New("invalid transition")
		},
	}
	router := setupDeployRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/deployments/"+uuid.New().String()+"/promote", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ---------------------------------------------------------------------------
// POST /deployments/:id/rollback  (rollbackDeployment)
// ---------------------------------------------------------------------------

func TestRollbackDeployment_Valid(t *testing.T) {
	svc := &mockDeployService{
		rollbackFn: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
	}
	router := setupDeployRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/deployments/"+uuid.New().String()+"/rollback", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "rolled_back", resp["status"])
}

func TestRollbackDeployment_ServiceError(t *testing.T) {
	svc := &mockDeployService{
		rollbackFn: func(_ context.Context, _ uuid.UUID) error {
			return errors.New("cannot rollback completed deployment")
		},
	}
	router := setupDeployRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/deployments/"+uuid.New().String()+"/rollback", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ---------------------------------------------------------------------------
// POST /deployments/:id/pause  (pauseDeployment)
// ---------------------------------------------------------------------------

func TestPauseDeployment_Valid(t *testing.T) {
	svc := &mockDeployService{
		pauseFn: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
	}
	router := setupDeployRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/deployments/"+uuid.New().String()+"/pause", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "paused", resp["status"])
}

func TestPauseDeployment_ServiceError(t *testing.T) {
	svc := &mockDeployService{
		pauseFn: func(_ context.Context, _ uuid.UUID) error {
			return errors.New("only running deployments can be paused")
		},
	}
	router := setupDeployRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/deployments/"+uuid.New().String()+"/pause", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ---------------------------------------------------------------------------
// POST /deployments/:id/resume  (resumeDeployment)
// ---------------------------------------------------------------------------

func TestResumeDeployment_Valid(t *testing.T) {
	svc := &mockDeployService{
		resumeFn: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
	}
	router := setupDeployRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/deployments/"+uuid.New().String()+"/resume", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "running", resp["status"])
}

func TestResumeDeployment_ServiceError(t *testing.T) {
	svc := &mockDeployService{
		resumeFn: func(_ context.Context, _ uuid.UUID) error {
			return errors.New("only paused deployments can be resumed")
		},
	}
	router := setupDeployRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/deployments/"+uuid.New().String()+"/resume", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ---------------------------------------------------------------------------
// POST /deployments with rollout field  (RolloutAttacher plumbing)
// ---------------------------------------------------------------------------

type fakeRolloutAttacher struct {
	called    bool
	lastDepID uuid.UUID
	lastReq   *RolloutAttachRequest
	returnErr error
}

func (f *fakeRolloutAttacher) AttachFromDeployRequest(_ context.Context, d *models.Deployment, req *RolloutAttachRequest, _ uuid.UUID) error {
	f.called = true
	f.lastDepID = d.ID
	f.lastReq = req
	return f.returnErr
}

func setupDeployRouterWithRollouts(svc DeployService, ra RolloutAttacher) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandlerWithRollouts(svc, nil, nil, nil, ra)
	handler.RegisterRoutes(router.Group("/api"), nil)
	return router
}

func TestCreateDeployment_WithRolloutField_AttacherCalled(t *testing.T) {
	depID := uuid.New()
	svc := &mockDeployService{
		createFn: func(_ context.Context, d *models.Deployment) error {
			d.ID = depID
			return nil
		},
	}
	fa := &fakeRolloutAttacher{}
	router := setupDeployRouterWithRollouts(svc, fa)

	body := map[string]interface{}{
		"application_id": uuid.New().String(),
		"environment_id": uuid.New().String(),
		"strategy":       "canary",
		"artifact":       "myapp:v2.0.0",
		"version":        "v2.0.0",
		"rollout": map[string]interface{}{
			"strategy_name": "x",
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/deployments", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.True(t, fa.called, "expected AttachFromDeployRequest to be called")
	assert.Equal(t, depID, fa.lastDepID)
	assert.Equal(t, "x", fa.lastReq.StrategyName)
}
