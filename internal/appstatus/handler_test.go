package appstatus

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func setupRouter(appID, envID uuid.UUID, keyAppID *uuid.UUID, keyEnvs []string) (*gin.Engine, *mockRepo, *mockDeploy) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	repo := &mockRepo{versionExists: true}
	dep := &mockDeploy{}
	svc := NewService(repo, dep)
	handler := NewHandler(svc)

	// Inject fake auth context so the env-resolution path exercises the
	// api_key branch without standing up real middleware.
	router.Use(func(c *gin.Context) {
		c.Set("auth_method", "api_key")
		if keyAppID != nil {
			c.Set("api_key_app_id", keyAppID.String())
		}
		if keyEnvs != nil {
			c.Set("api_key_environment_ids", keyEnvs)
		}
		c.Next()
	})
	handler.RegisterRoutes(router.Group("/api"), nil)
	return router, repo, dep
}

func TestHandler_ReportStatus_HappyPath(t *testing.T) {
	appID := uuid.New()
	envID := uuid.New()
	router, repo, _ := setupRouter(appID, envID, &appID, []string{envID.String()})

	body := map[string]interface{}{"version": "1.4.2", "health": "healthy"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/applications/"+appID.String()+"/status", bytes.NewBuffer(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.NotNil(t, repo.latest)
	assert.Equal(t, "1.4.2", repo.latest.Version)
}

func TestHandler_ReportStatus_AppIDMismatch(t *testing.T) {
	appID := uuid.New()
	envID := uuid.New()
	otherApp := uuid.New()
	router, _, _ := setupRouter(appID, envID, &otherApp, []string{envID.String()})

	body := map[string]interface{}{"version": "1.0.0", "health": "healthy"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/applications/"+appID.String()+"/status", bytes.NewBuffer(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestHandler_ReportStatus_MultipleEnvsOnKey(t *testing.T) {
	appID := uuid.New()
	envA := uuid.New().String()
	envB := uuid.New().String()
	router, _, _ := setupRouter(appID, uuid.Nil, &appID, []string{envA, envB})

	body := map[string]interface{}{"version": "1.0.0", "health": "healthy"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/applications/"+appID.String()+"/status", bytes.NewBuffer(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_ReportStatus_MissingEnv(t *testing.T) {
	appID := uuid.New()
	router, _, _ := setupRouter(appID, uuid.Nil, &appID, nil)

	body := map[string]interface{}{"version": "1.0.0", "health": "healthy"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/applications/"+appID.String()+"/status", bytes.NewBuffer(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_ReportStatus_InvalidBody(t *testing.T) {
	appID := uuid.New()
	envID := uuid.New()
	router, _, _ := setupRouter(appID, envID, &appID, []string{envID.String()})

	// Missing health field
	body := map[string]interface{}{"version": "1.0.0"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/applications/"+appID.String()+"/status", bytes.NewBuffer(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
