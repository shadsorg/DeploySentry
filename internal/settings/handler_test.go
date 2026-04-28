package settings

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

type mockSettingService struct {
	listFn    func(ctx context.Context, scope string, targetID uuid.UUID) ([]*models.Setting, error)
	resolveFn func(ctx context.Context, key string, orgID, projectID, appID, envID *uuid.UUID) (*models.Setting, error)
	setFn     func(ctx context.Context, setting *models.Setting) error
	deleteFn  func(ctx context.Context, id uuid.UUID) error
}

func (m *mockSettingService) List(ctx context.Context, scope string, targetID uuid.UUID) ([]*models.Setting, error) {
	if m.listFn != nil {
		return m.listFn(ctx, scope, targetID)
	}
	return []*models.Setting{}, nil
}

func (m *mockSettingService) Resolve(ctx context.Context, key string, orgID, projectID, appID, envID *uuid.UUID) (*models.Setting, error) {
	if m.resolveFn != nil {
		return m.resolveFn(ctx, key, orgID, projectID, appID, envID)
	}
	return &models.Setting{}, nil
}

func (m *mockSettingService) Set(ctx context.Context, setting *models.Setting) error {
	if m.setFn != nil {
		return m.setFn(ctx, setting)
	}
	return nil
}

func (m *mockSettingService) Delete(ctx context.Context, id uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func setupSettingRouter(svc SettingService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", auth.RoleOwner)
		c.Next()
	})
	rbac := auth.NewRBACChecker()
	handler := NewHandler(svc, rbac)
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
// GET /settings  (listSettings)
// ---------------------------------------------------------------------------

func TestListSettings_Valid(t *testing.T) {
	orgID := uuid.New()
	svc := &mockSettingService{
		listFn: func(_ context.Context, scope string, targetID uuid.UUID) ([]*models.Setting, error) {
			assert.Equal(t, "org", scope)
			assert.Equal(t, orgID, targetID)
			return []*models.Setting{
				{ID: uuid.New(), OrgID: &orgID, Key: "theme", Value: json.RawMessage(`"dark"`)},
			}, nil
		},
	}
	router := setupSettingRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/settings?scope=org&target="+orgID.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]json.RawMessage
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "settings")
}

func TestListSettings_MissingParams(t *testing.T) {
	router := setupSettingRouter(&mockSettingService{})

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// GET /settings/resolve  (resolveSettings)
// ---------------------------------------------------------------------------

func TestResolveSettings_Valid(t *testing.T) {
	orgID := uuid.New()
	svc := &mockSettingService{
		resolveFn: func(_ context.Context, key string, oID, _, _, _ *uuid.UUID) (*models.Setting, error) {
			assert.Equal(t, "theme", key)
			return &models.Setting{ID: uuid.New(), OrgID: oID, Key: key, Value: json.RawMessage(`"dark"`)}, nil
		},
	}
	router := setupSettingRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/resolve?key=theme&org_id="+orgID.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.Setting
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "theme", resp.Key)
}

func TestResolveSettings_MissingKey(t *testing.T) {
	router := setupSettingRouter(&mockSettingService{})

	req := httptest.NewRequest(http.MethodGet, "/api/settings/resolve", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// PUT /settings  (setSetting)
// ---------------------------------------------------------------------------

func TestSetSetting_Valid(t *testing.T) {
	svc := &mockSettingService{
		setFn: func(_ context.Context, s *models.Setting) error {
			assert.Equal(t, "theme", s.Key)
			return nil
		},
	}
	router := setupSettingRouter(svc)

	body := map[string]interface{}{
		"scope":     "org",
		"target_id": uuid.New().String(),
		"key":       "theme",
		"value":     "dark",
	}
	req := httptest.NewRequest(http.MethodPut, "/api/settings", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSetSetting_InvalidScope(t *testing.T) {
	router := setupSettingRouter(&mockSettingService{})

	body := map[string]interface{}{
		"scope":     "galaxy",
		"target_id": uuid.New().String(),
		"key":       "theme",
		"value":     "dark",
	}
	req := httptest.NewRequest(http.MethodPut, "/api/settings", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// DELETE /settings/:id  (deleteSetting)
// ---------------------------------------------------------------------------

func TestDeleteSetting_Valid(t *testing.T) {
	svc := &mockSettingService{
		deleteFn: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
	}
	router := setupSettingRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeleteSetting_InvalidID(t *testing.T) {
	router := setupSettingRouter(&mockSettingService{})

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/not-a-uuid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteSetting_ServiceError(t *testing.T) {
	svc := &mockSettingService{
		deleteFn: func(_ context.Context, _ uuid.UUID) error {
			return errors.New("not found")
		},
	}
	router := setupSettingRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
