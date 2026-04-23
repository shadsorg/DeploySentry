package currentstate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deploysentry/deploysentry/internal/entities"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func setupRouter(svc *Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(svc).RegisterRoutes(router.Group("/api"), nil)
	return router
}

func TestHandler_CurrentState_HappyPath(t *testing.T) {
	env := &entities.OrgEnvironment{ID: uuid.New(), Slug: "prod", Name: "Production"}
	svc := NewService(&fakeDeploys{}, &fakeStatus{}, &fakeEnvs{env: env})
	router := setupRouter(svc)

	url := "/api/applications/" + uuid.New().String() + "/environments/" + env.ID.String() + "/current-state"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.CurrentStateResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, env.ID, resp.Environment.ID)
	assert.Equal(t, "prod", resp.Environment.Slug)
}

func TestHandler_CurrentState_BadAppID(t *testing.T) {
	svc := NewService(&fakeDeploys{}, &fakeStatus{}, &fakeEnvs{})
	router := setupRouter(svc)

	url := "/api/applications/not-a-uuid/environments/" + uuid.New().String() + "/current-state"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_CurrentState_EnvNotFound(t *testing.T) {
	svc := NewService(&fakeDeploys{}, &fakeStatus{}, &fakeEnvs{env: nil})
	router := setupRouter(svc)

	url := "/api/applications/" + uuid.New().String() + "/environments/" + uuid.New().String() + "/current-state"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
