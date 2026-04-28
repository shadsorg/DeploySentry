package releases

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shadsorg/deploysentry/internal/auth"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Mock
// ---------------------------------------------------------------------------

type mockReleaseServiceHandler struct {
	createFn          func(ctx context.Context, release *models.Release) error
	getByIDFn         func(ctx context.Context, id uuid.UUID) (*models.Release, error)
	listByAppFn       func(ctx context.Context, appID uuid.UUID) ([]models.Release, error)
	startFn           func(ctx context.Context, id uuid.UUID) error
	promoteFn         func(ctx context.Context, id uuid.UUID, trafficPct int) error
	pauseFn           func(ctx context.Context, id uuid.UUID) error
	rollbackFn        func(ctx context.Context, id uuid.UUID) error
	completeFn        func(ctx context.Context, id uuid.UUID) error
	deleteFn          func(ctx context.Context, id uuid.UUID) error
	addFlagChangeFn   func(ctx context.Context, fc *models.ReleaseFlagChange) error
	listFlagChangesFn func(ctx context.Context, releaseID uuid.UUID) ([]models.ReleaseFlagChange, error)
}

func (m *mockReleaseServiceHandler) Create(ctx context.Context, release *models.Release) error {
	if m.createFn != nil {
		return m.createFn(ctx, release)
	}
	release.ID = uuid.New()
	return nil
}

func (m *mockReleaseServiceHandler) GetByID(ctx context.Context, id uuid.UUID) (*models.Release, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return &models.Release{ID: id, Name: "test"}, nil
}

func (m *mockReleaseServiceHandler) ListByApplication(ctx context.Context, appID uuid.UUID) ([]models.Release, error) {
	if m.listByAppFn != nil {
		return m.listByAppFn(ctx, appID)
	}
	return []models.Release{}, nil
}

func (m *mockReleaseServiceHandler) Start(ctx context.Context, id uuid.UUID) error {
	if m.startFn != nil {
		return m.startFn(ctx, id)
	}
	return nil
}

func (m *mockReleaseServiceHandler) Promote(ctx context.Context, id uuid.UUID, trafficPct int) error {
	if m.promoteFn != nil {
		return m.promoteFn(ctx, id, trafficPct)
	}
	return nil
}

func (m *mockReleaseServiceHandler) Pause(ctx context.Context, id uuid.UUID) error {
	if m.pauseFn != nil {
		return m.pauseFn(ctx, id)
	}
	return nil
}

func (m *mockReleaseServiceHandler) Rollback(ctx context.Context, id uuid.UUID) error {
	if m.rollbackFn != nil {
		return m.rollbackFn(ctx, id)
	}
	return nil
}

func (m *mockReleaseServiceHandler) Complete(ctx context.Context, id uuid.UUID) error {
	if m.completeFn != nil {
		return m.completeFn(ctx, id)
	}
	return nil
}

func (m *mockReleaseServiceHandler) Delete(ctx context.Context, id uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockReleaseServiceHandler) AddFlagChange(ctx context.Context, fc *models.ReleaseFlagChange) error {
	if m.addFlagChangeFn != nil {
		return m.addFlagChangeFn(ctx, fc)
	}
	fc.ID = uuid.New()
	return nil
}

func (m *mockReleaseServiceHandler) ListFlagChanges(ctx context.Context, releaseID uuid.UUID) ([]models.ReleaseFlagChange, error) {
	if m.listFlagChangesFn != nil {
		return m.listFlagChangesFn(ctx, releaseID)
	}
	return []models.ReleaseFlagChange{}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func injectRole(role auth.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("role", role)
		c.Next()
	}
}

func setupReleaseRouter(svc ReleaseService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(injectRole(auth.RoleAdmin))
	handler := NewHandler(svc)
	handler.RegisterRoutes(router.Group("/api"))
	return router
}

func toJSON(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	assert.NoError(t, err)
	return bytes.NewBuffer(b)
}

var testAppID = uuid.New()

func releasePath(suffix string) string {
	return "/api/applications/" + testAppID.String() + "/releases" + suffix
}

// ---------------------------------------------------------------------------
// POST /applications/:app_id/releases
// ---------------------------------------------------------------------------

func TestCreateRelease_Valid(t *testing.T) {
	svc := &mockReleaseServiceHandler{}
	router := setupReleaseRouter(svc)

	body := map[string]interface{}{
		"name":        "Enable checkout v2",
		"description": "Rolling out new checkout",
	}
	req := httptest.NewRequest(http.MethodPost, releasePath(""), toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateRelease_InvalidJSON(t *testing.T) {
	router := setupReleaseRouter(&mockReleaseServiceHandler{})

	req := httptest.NewRequest(http.MethodPost, releasePath(""), bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// GET /applications/:app_id/releases/:id
// ---------------------------------------------------------------------------

func TestGetRelease_Valid(t *testing.T) {
	releaseID := uuid.New()
	svc := &mockReleaseServiceHandler{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*models.Release, error) {
			return &models.Release{ID: id, Name: "Test Release"}, nil
		},
	}
	router := setupReleaseRouter(svc)

	req := httptest.NewRequest(http.MethodGet, releasePath("/"+releaseID.String()), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetRelease_InvalidID(t *testing.T) {
	router := setupReleaseRouter(&mockReleaseServiceHandler{})

	req := httptest.NewRequest(http.MethodGet, releasePath("/not-a-uuid"), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetRelease_NotFound(t *testing.T) {
	svc := &mockReleaseServiceHandler{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*models.Release, error) {
			return nil, errors.New("not found")
		},
	}
	router := setupReleaseRouter(svc)

	req := httptest.NewRequest(http.MethodGet, releasePath("/"+uuid.New().String()), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---------------------------------------------------------------------------
// GET /applications/:app_id/releases
// ---------------------------------------------------------------------------

func TestListReleases_Valid(t *testing.T) {
	svc := &mockReleaseServiceHandler{
		listByAppFn: func(_ context.Context, appID uuid.UUID) ([]models.Release, error) {
			return []models.Release{
				{ID: uuid.New(), ApplicationID: appID, Name: "Release 1"},
				{ID: uuid.New(), ApplicationID: appID, Name: "Release 2"},
			}, nil
		},
	}
	router := setupReleaseRouter(svc)

	req := httptest.NewRequest(http.MethodGet, releasePath(""), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// POST /applications/:app_id/releases/:id/start
// ---------------------------------------------------------------------------

func TestStartRelease_Valid(t *testing.T) {
	svc := &mockReleaseServiceHandler{}
	router := setupReleaseRouter(svc)

	req := httptest.NewRequest(http.MethodPost, releasePath("/"+uuid.New().String()+"/start"), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// POST /applications/:app_id/releases/:id/promote
// ---------------------------------------------------------------------------

func TestPromoteRelease_Valid(t *testing.T) {
	svc := &mockReleaseServiceHandler{}
	router := setupReleaseRouter(svc)

	body := map[string]interface{}{"traffic_percent": 50}
	req := httptest.NewRequest(http.MethodPost, releasePath("/"+uuid.New().String()+"/promote"), toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// RBAC
// ---------------------------------------------------------------------------

func TestRBAC_ViewerCannotCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(injectRole(auth.RoleViewer))
	handler := NewHandler(&mockReleaseServiceHandler{})
	handler.RegisterRoutes(router.Group("/api"))

	body := map[string]interface{}{
		"name": "test release",
	}
	req := httptest.NewRequest(http.MethodPost, releasePath(""), toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRBAC_ViewerCanRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(injectRole(auth.RoleViewer))
	handler := NewHandler(&mockReleaseServiceHandler{})
	handler.RegisterRoutes(router.Group("/api"))

	req := httptest.NewRequest(http.MethodGet, releasePath("/"+uuid.New().String()), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
