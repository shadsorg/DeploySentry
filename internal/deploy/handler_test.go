package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Mock
// ---------------------------------------------------------------------------

type mockDeployService struct {
	createFn    func(ctx context.Context, d *models.Deployment) error
	getFn       func(ctx context.Context, id uuid.UUID) (*models.Deployment, error)
	listFn      func(ctx context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.Deployment, error)
	promoteFn   func(ctx context.Context, id uuid.UUID) error
	rollbackFn  func(ctx context.Context, id uuid.UUID) error
	pauseFn     func(ctx context.Context, id uuid.UUID) error
	resumeFn    func(ctx context.Context, id uuid.UUID) error
	getActiveFn func(ctx context.Context, projectID uuid.UUID) ([]*models.Deployment, error)
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

func (m *mockDeployService) ListDeployments(ctx context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.Deployment, error) {
	if m.listFn != nil {
		return m.listFn(ctx, projectID, opts)
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

func (m *mockDeployService) GetActiveDeployments(ctx context.Context, projectID uuid.UUID) ([]*models.Deployment, error) {
	if m.getActiveFn != nil {
		return m.getActiveFn(ctx, projectID)
	}
	return []*models.Deployment{}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func setupDeployRouter(svc DeployService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(svc, nil, nil)
	// Pass nil for RBAC to disable permission checks in unit tests.
	handler.RegisterRoutes(router.Group("/api"), nil)
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
		"project_id":     uuid.New().String(),
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
		"project_id":     uuid.New().String(),
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
// GET /deployments?project_id=  (listDeployments)
// ---------------------------------------------------------------------------

func TestListDeployments_Valid(t *testing.T) {
	projectID := uuid.New()
	svc := &mockDeployService{
		listFn: func(_ context.Context, pid uuid.UUID, _ ListOptions) ([]*models.Deployment, error) {
			return []*models.Deployment{
				{ID: uuid.New(), ProjectID: pid, Artifact: "app:v1"},
			}, nil
		},
	}
	router := setupDeployRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/deployments?project_id="+projectID.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]json.RawMessage
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "deployments")
}

func TestListDeployments_MissingProjectID(t *testing.T) {
	router := setupDeployRouter(&mockDeployService{})

	req := httptest.NewRequest(http.MethodGet, "/api/deployments", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListDeployments_InvalidProjectID(t *testing.T) {
	router := setupDeployRouter(&mockDeployService{})

	req := httptest.NewRequest(http.MethodGet, "/api/deployments?project_id=invalid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
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
