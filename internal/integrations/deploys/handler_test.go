package deploys

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/platform/crypto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func setupRouter(t *testing.T) (*gin.Engine, *Service, *fakeRepo, *fakeDeployCreator, []byte) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	svc, repo, deploys, key := newTestService(t)
	reg := NewRegistry()
	reg.Register(GenericAdapter{})
	reg.Register(RailwayAdapter{})
	NewHandler(svc, reg).RegisterRoutes(router.Group("/api"), nil)
	return router, svc, repo, deploys, key
}

func TestHandler_RailwayWebhook_HappyPath(t *testing.T) {
	router, _, repo, deploys, key := setupRouter(t)
	envID := uuid.New()
	enc, _ := crypto.Encrypt([]byte("rail-s3cret"), key)
	integration := &models.DeployIntegration{
		ID:               uuid.New(),
		ApplicationID:    uuid.New(),
		Provider:         models.DeployProviderRailway,
		AuthMode:         models.DeployIntegrationAuthHMAC,
		WebhookSecretEnc: enc,
		ProviderConfig:   map[string]any{"service_id": "svc-123"},
		EnvMapping:       map[string]uuid.UUID{"production": envID},
		Enabled:          true,
	}
	repo.integrations[integration.ID] = integration

	payload := []byte(`{
		"type":"DEPLOY","status":"SUCCESS",
		"meta":{"deploymentId":"dep-abc"},
		"commit":{"sha":"sha-abc"},
		"environment":{"name":"production"},
		"service":{"id":"svc-123"}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/integrations/railway/webhook", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Railway-Signature", sign(payload, "rail-s3cret"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, 1, len(deploys.created))
	assert.Equal(t, "dep-abc", deploys.created[0].Version)
}

// TestHandler_RailwayWebhook_BearerMode verifies the auth-mode=bearer
// escape hatch added when Railway removed its signed-webhook surface in
// favor of unsigned notification rules (Task 11 findings): users set
// auth_mode=bearer on the integration and put Authorization: Bearer
// <secret> into the Railway notification-rule header config.
func TestHandler_RailwayWebhook_BearerMode(t *testing.T) {
	router, _, repo, deploys, key := setupRouter(t)
	envID := uuid.New()
	bearer := "rail-bearer-abc"
	enc, _ := crypto.Encrypt([]byte(bearer), key)
	integration := &models.DeployIntegration{
		ID:               uuid.New(),
		ApplicationID:    uuid.New(),
		Provider:         models.DeployProviderRailway,
		AuthMode:         models.DeployIntegrationAuthBearer,
		WebhookSecretEnc: enc,
		ProviderConfig:   map[string]any{"service_id": "svc-123"},
		EnvMapping:       map[string]uuid.UUID{"production": envID},
		Enabled:          true,
	}
	repo.integrations[integration.ID] = integration

	payload := []byte(`{
		"type":"DEPLOY","status":"SUCCESS",
		"meta":{"deploymentId":"dep-bearer"},
		"commit":{"sha":"sha-bearer"},
		"environment":{"name":"production"},
		"service":{"id":"svc-123"}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/integrations/railway/webhook", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	// Deliberately NO X-Railway-Signature — this is the unsigned-delivery case.
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, 1, len(deploys.created))
	assert.Equal(t, "dep-bearer", deploys.created[0].Version)
}

func TestHandler_RailwayWebhook_BearerMode_WrongToken(t *testing.T) {
	router, _, repo, _, key := setupRouter(t)
	enc, _ := crypto.Encrypt([]byte("right"), key)
	integration := &models.DeployIntegration{
		ID:               uuid.New(),
		ApplicationID:    uuid.New(),
		Provider:         models.DeployProviderRailway,
		AuthMode:         models.DeployIntegrationAuthBearer,
		WebhookSecretEnc: enc,
		ProviderConfig:   map[string]any{"service_id": "svc-x"},
		EnvMapping:       map[string]uuid.UUID{"production": uuid.New()},
		Enabled:          true,
	}
	repo.integrations[integration.ID] = integration

	payload := []byte(`{"type":"DEPLOY","status":"SUCCESS","service":{"id":"svc-x"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/railway/webhook", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer wrong")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_RailwayWebhook_BadSignature(t *testing.T) {
	router, _, repo, _, key := setupRouter(t)
	enc, _ := crypto.Encrypt([]byte("rail-s3cret"), key)
	integration := &models.DeployIntegration{
		ID: uuid.New(), ApplicationID: uuid.New(),
		Provider: models.DeployProviderRailway, AuthMode: models.DeployIntegrationAuthHMAC,
		WebhookSecretEnc: enc,
		ProviderConfig:   map[string]any{"service_id": "svc-123"},
		EnvMapping:       map[string]uuid.UUID{"production": uuid.New()},
		Enabled:          true,
	}
	repo.integrations[integration.ID] = integration

	payload := []byte(`{"type":"DEPLOY","status":"SUCCESS","service":{"id":"svc-123"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/railway/webhook", bytes.NewReader(payload))
	req.Header.Set("X-Railway-Signature", "sha256=deadbeef")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_GenericWebhook_HappyPath(t *testing.T) {
	router, _, repo, deploys, key := setupRouter(t)
	envID := uuid.New()
	enc, _ := crypto.Encrypt([]byte("tok_abc"), key)
	integration := &models.DeployIntegration{
		ID:               uuid.New(),
		ApplicationID:    uuid.New(),
		Provider:         models.DeployProviderGeneric,
		AuthMode:         models.DeployIntegrationAuthBearer,
		WebhookSecretEnc: enc,
		EnvMapping:       map[string]uuid.UUID{"production": envID},
		Enabled:          true,
	}
	repo.integrations[integration.ID] = integration

	body, _ := json.Marshal(models.DeployEvent{
		EventType:   models.DeployEventSucceeded,
		Environment: "production",
		Version:     "1.4.2",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/deploys/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DeploySentry-Integration-Id", integration.ID.String())
	req.Header.Set("Authorization", "Bearer tok_abc")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, 1, len(deploys.created))
	if assert.NotNil(t, deploys.created[0].Source) {
		assert.Equal(t, "generic-webhook", *deploys.created[0].Source)
	}
}

func TestHandler_GenericWebhook_MissingIntegrationId(t *testing.T) {
	router, _, _, _, _ := setupRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/deploys/webhook", bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_CreateListDelete(t *testing.T) {
	router, _, _, _, _ := setupRouter(t)
	appID := uuid.New()
	envID := uuid.New()

	body, _ := json.Marshal(map[string]any{
		"application_id": appID,
		"provider":       "railway",
		"auth_mode":      "hmac",
		"webhook_secret": "s3cret",
		"provider_config": map[string]any{"service_id": "svc-1"},
		"env_mapping": map[string]string{"production": envID.String()},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/deploys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created models.DeployIntegration
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, "railway", created.Provider)

	req2 := httptest.NewRequest(http.MethodGet, "/api/integrations/deploys?application_id="+appID.String(), nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	req3 := httptest.NewRequest(http.MethodDelete, "/api/integrations/deploys/"+created.ID.String(), nil)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusNoContent, w3.Code)
}
