package releases

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

type mockReleaseService struct {
	createFn  func(ctx context.Context, release *models.Release) error
	getFn     func(ctx context.Context, id uuid.UUID) (*models.Release, error)
	listFn    func(ctx context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.Release, error)
	promoteFn func(ctx context.Context, releaseID, environmentID, deployedBy uuid.UUID) error
}

func (m *mockReleaseService) Create(ctx context.Context, release *models.Release) error {
	if m.createFn != nil {
		return m.createFn(ctx, release)
	}
	return nil
}

func (m *mockReleaseService) Get(ctx context.Context, id uuid.UUID) (*models.Release, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return &models.Release{ID: id}, nil
}

func (m *mockReleaseService) List(ctx context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.Release, error) {
	if m.listFn != nil {
		return m.listFn(ctx, projectID, opts)
	}
	return []*models.Release{}, nil
}

func (m *mockReleaseService) Promote(ctx context.Context, releaseID, environmentID, deployedBy uuid.UUID) error {
	if m.promoteFn != nil {
		return m.promoteFn(ctx, releaseID, environmentID, deployedBy)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func setupReleaseRouter(svc ReleaseService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
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

// ---------------------------------------------------------------------------
// POST /releases  (createRelease)
// ---------------------------------------------------------------------------

func TestCreateRelease_Valid(t *testing.T) {
	svc := &mockReleaseService{
		createFn: func(_ context.Context, r *models.Release) error {
			r.ID = uuid.New()
			return nil
		},
	}
	router := setupReleaseRouter(svc)

	body := map[string]interface{}{
		"project_id":  uuid.New().String(),
		"version":     "v1.0.0",
		"title":       "Initial Release",
		"description": "First public release",
		"commit_sha":  "abc123def",
		"artifact":    "myapp:v1.0.0",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/releases", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp models.Release
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "v1.0.0", resp.Version)
	assert.Equal(t, "Initial Release", resp.Title)
}

func TestCreateRelease_InvalidJSON(t *testing.T) {
	router := setupReleaseRouter(&mockReleaseService{})

	req := httptest.NewRequest(http.MethodPost, "/api/releases", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// GET /releases/:id  (getRelease)
// ---------------------------------------------------------------------------

func TestGetRelease_Valid(t *testing.T) {
	releaseID := uuid.New()
	svc := &mockReleaseService{
		getFn: func(_ context.Context, id uuid.UUID) (*models.Release, error) {
			return &models.Release{
				ID:      id,
				Version: "v2.0.0",
				Title:   "Second Release",
			}, nil
		},
	}
	router := setupReleaseRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/releases/"+releaseID.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.Release
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, releaseID, resp.ID)
	assert.Equal(t, "v2.0.0", resp.Version)
}

func TestGetRelease_InvalidID(t *testing.T) {
	router := setupReleaseRouter(&mockReleaseService{})

	req := httptest.NewRequest(http.MethodGet, "/api/releases/not-a-uuid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetRelease_NotFound(t *testing.T) {
	svc := &mockReleaseService{
		getFn: func(_ context.Context, _ uuid.UUID) (*models.Release, error) {
			return nil, errors.New("not found")
		},
	}
	router := setupReleaseRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/releases/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---------------------------------------------------------------------------
// GET /releases?project_id=  (listReleases)
// ---------------------------------------------------------------------------

func TestListReleases_Valid(t *testing.T) {
	projectID := uuid.New()
	svc := &mockReleaseService{
		listFn: func(_ context.Context, pid uuid.UUID, _ ListOptions) ([]*models.Release, error) {
			return []*models.Release{
				{ID: uuid.New(), ProjectID: pid, Version: "v1.0.0"},
				{ID: uuid.New(), ProjectID: pid, Version: "v1.1.0"},
			}, nil
		},
	}
	router := setupReleaseRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/releases?project_id="+projectID.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]json.RawMessage
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "releases")
}

func TestListReleases_MissingProjectID(t *testing.T) {
	router := setupReleaseRouter(&mockReleaseService{})

	req := httptest.NewRequest(http.MethodGet, "/api/releases", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// POST /releases/:id/promote  (promoteRelease)
// ---------------------------------------------------------------------------

func TestPromoteRelease_Valid(t *testing.T) {
	svc := &mockReleaseService{
		promoteFn: func(_ context.Context, _, _, _ uuid.UUID) error {
			return nil
		},
	}
	router := setupReleaseRouter(svc)

	body := map[string]interface{}{
		"environment_id": uuid.New().String(),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/releases/"+uuid.New().String()+"/promote", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "promoted", resp["status"])
}

func TestPromoteRelease_InvalidID(t *testing.T) {
	router := setupReleaseRouter(&mockReleaseService{})

	body := map[string]interface{}{
		"environment_id": uuid.New().String(),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/releases/bad-uuid/promote", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
