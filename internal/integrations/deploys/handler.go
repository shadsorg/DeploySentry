package deploys

import (
	"bytes"
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/shadsorg/deploysentry/internal/auth"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func constantTimeStringEqual(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

// Handler wires the HTTP routes for integrations and inbound webhooks.
type Handler struct {
	svc      *Service
	registry *Registry
}

func NewHandler(svc *Service, reg *Registry) *Handler {
	return &Handler{svc: svc, registry: reg}
}

// RegisterRoutes mounts the integration CRUD + webhook endpoints.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, rbac *auth.RBACChecker) {
	// CRUD
	grp := rg.Group("/integrations/deploys")
	grp.POST("", mw(rbac, auth.PermDeployCreate), h.createIntegration)
	grp.GET("", mw(rbac, auth.PermDeployRead), h.listIntegrations)
	grp.GET("/:id/events", mw(rbac, auth.PermDeployRead), h.listIntegrationEvents)
	grp.DELETE("/:id", mw(rbac, auth.PermDeployCreate), h.deleteIntegration)

	// Generic canonical endpoint — signature/bearer verified inside the adapter.
	grp.POST("/webhook", h.genericWebhook)

	// Provider-specific endpoints. :provider matches registered adapters.
	rg.POST("/integrations/:provider/webhook", h.providerWebhook)
}

func mw(rbac *auth.RBACChecker, perm auth.Permission) gin.HandlerFunc {
	if rbac == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return auth.RequirePermission(rbac, perm)
}

// -----------------------------------------------------------------------------
// CRUD
// -----------------------------------------------------------------------------

type createIntegrationRequest struct {
	ApplicationID     uuid.UUID            `json:"application_id" binding:"required"`
	Provider          string               `json:"provider" binding:"required"`
	AuthMode          string               `json:"auth_mode"`
	WebhookSecret     string               `json:"webhook_secret" binding:"required"`
	ProviderConfig    map[string]any       `json:"provider_config"`
	EnvMapping        map[string]uuid.UUID `json:"env_mapping" binding:"required"`
	VersionExtractors []string             `json:"version_extractors"`
}

func (h *Handler) createIntegration(c *gin.Context) {
	var req createIntegrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	integration, err := h.svc.CreateIntegration(c.Request.Context(), CreateIntegrationInput(req))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, integration)
}

func (h *Handler) listIntegrations(c *gin.Context) {
	appIDStr := c.Query("application_id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "application_id query parameter is required"})
		return
	}
	items, err := h.svc.ListIntegrationsForApp(c.Request.Context(), appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"integrations": items})
}

func (h *Handler) deleteIntegration(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.svc.DeleteIntegration(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) listIntegrationEvents(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	limit := 20
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	events, err := h.svc.ListRecentEvents(c.Request.Context(), id, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": events})
}

// -----------------------------------------------------------------------------
// Webhook ingestion
// -----------------------------------------------------------------------------

func (h *Handler) genericWebhook(c *gin.Context) {
	integrationIDStr := c.GetHeader("X-DeploySentry-Integration-Id")
	integrationID, err := uuid.Parse(integrationIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-DeploySentry-Integration-Id header is required"})
		return
	}
	integration, err := h.svc.GetIntegration(c.Request.Context(), integrationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "integration not found"})
		return
	}
	if integration.Provider != models.DeployProviderGeneric {
		c.JSON(http.StatusBadRequest, gin.H{"error": "generic endpoint requires a generic integration"})
		return
	}
	h.dispatch(c, integration)
}

func (h *Handler) providerWebhook(c *gin.Context) {
	provider := c.Param("provider")
	adapter, ok := h.registry.Lookup(provider)
	if !ok || provider == models.DeployProviderGeneric {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown provider"})
		return
	}

	// Read the body once so we can both verify + parse + persist.
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	// Peek at the payload to find the integration row.
	integration, err := h.lookupProviderIntegration(c.Request.Context(), provider, body)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	secret, err := h.svc.ResolveSecret(integration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not resolve integration secret"})
		return
	}
	if err := verifyInboundAuth(c.Request, body, integration, secret, adapter); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	event, err := adapter.ParsePayload(body, integration)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.performIngest(c, integration, event, body)
}

// verifyInboundAuth short-circuits on auth_mode=bearer (every provider
// gets a uniform Authorization: Bearer check) and falls through to the
// adapter's native signature check otherwise. Lets providers that no
// longer offer HMAC signing in their public API — like Railway's
// notification-rules surface — still deliver authenticated webhooks by
// configuring a custom header in the provider's dashboard.
func verifyInboundAuth(r *http.Request, body []byte, integration *models.DeployIntegration, secret string, adapter DeployEventAdapter) error {
	if integration.AuthMode == models.DeployIntegrationAuthBearer {
		return verifyBearerHeader(r, secret)
	}
	return adapter.VerifySignature(r, body, secret)
}

// verifyBearerHeader accepts Authorization: Bearer <secret> on any
// provider endpoint. Constant-time compare to avoid timing oracles.
func verifyBearerHeader(r *http.Request, secret string) error {
	got := r.Header.Get("Authorization")
	if !strings.HasPrefix(got, "Bearer ") {
		return ErrInvalidSignature
	}
	token := strings.TrimPrefix(got, "Bearer ")
	if !constantTimeStringEqual(token, secret) {
		return ErrInvalidSignature
	}
	return nil
}

// dispatch runs the generic adapter end-to-end for a pre-resolved integration.
func (h *Handler) dispatch(c *gin.Context, integration *models.DeployIntegration) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	secret, err := h.svc.ResolveSecret(integration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not resolve integration secret"})
		return
	}
	adapter, _ := h.registry.Lookup(models.DeployProviderGeneric)
	if adapter == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "generic adapter not registered"})
		return
	}
	if err := verifyInboundAuth(c.Request, body, integration, secret, adapter); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	event, err := adapter.ParsePayload(body, integration)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.performIngest(c, integration, event, body)
}

func (h *Handler) performIngest(c *gin.Context, integration *models.DeployIntegration, event models.DeployEvent, body []byte) {
	result, err := h.svc.Ingest(c.Request.Context(), IngestInput{
		Integration: integration,
		Event:       event,
		RawPayload:  body,
	})
	if err != nil {
		if errors.Is(err, ErrUnmappedEnvironment) {
			c.JSON(http.StatusAccepted, gin.H{
				"status":      "unmapped",
				"environment": event.Environment,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{
		"event":     result.Event,
		"duplicate": result.Duplicate,
		"deployment": result.Deployment,
	})
}

// lookupProviderIntegration finds the configured integration row matching
// the inbound provider payload. Matching strategies are provider-specific.
func (h *Handler) lookupProviderIntegration(ctx context.Context, provider string, body []byte) (*models.DeployIntegration, error) {
	var raw map[string]any
	_ = json.Unmarshal(body, &raw)

	match := func(cfg map[string]any) bool { return true } // permissive default
	switch provider {
	case models.DeployProviderRailway:
		serviceID := stringFromPath(raw, "service.id")
		if serviceID == "" {
			return nil, fmt.Errorf("Railway payload missing service.id")
		}
		match = func(cfg map[string]any) bool {
			if cfg == nil {
				return false
			}
			v, _ := cfg["service_id"].(string)
			return v != "" && v == serviceID
		}
	case models.DeployProviderRender:
		serviceID := stringFromPath(raw, "data.service.id")
		if serviceID == "" {
			return nil, fmt.Errorf("Render payload missing data.service.id")
		}
		match = func(cfg map[string]any) bool {
			if cfg == nil {
				return false
			}
			v, _ := cfg["service_id"].(string)
			return v != "" && v == serviceID
		}
	case models.DeployProviderHeroku:
		appName := stringFromPath(raw, "data.app.name")
		if appName == "" {
			return nil, fmt.Errorf("Heroku payload missing data.app.name")
		}
		match = func(cfg map[string]any) bool {
			if cfg == nil {
				return false
			}
			v, _ := cfg["app_name"].(string)
			return v != "" && v == appName
		}
	case models.DeployProviderVercel:
		projectID := stringFromPath(raw, "payload.project.id")
		if projectID == "" {
			return nil, fmt.Errorf("Vercel payload missing payload.project.id")
		}
		match = func(cfg map[string]any) bool {
			if cfg == nil {
				return false
			}
			v, _ := cfg["project_id"].(string)
			return v != "" && v == projectID
		}
	case models.DeployProviderNetlify:
		siteID := stringFromPath(raw, "site_id")
		if siteID == "" {
			return nil, fmt.Errorf("Netlify payload missing site_id")
		}
		match = func(cfg map[string]any) bool {
			if cfg == nil {
				return false
			}
			v, _ := cfg["site_id"].(string)
			return v != "" && v == siteID
		}
	case models.DeployProviderGitHubActions:
		repoFull := stringFromPath(raw, "repository.full_name")
		if repoFull == "" {
			return nil, fmt.Errorf("GitHub Actions payload missing repository.full_name")
		}
		match = func(cfg map[string]any) bool {
			if cfg == nil {
				return false
			}
			v, _ := cfg["repository"].(string)
			return v != "" && v == repoFull
		}
	case models.DeployProviderFly:
		appName := stringFromPath(raw, "app.name")
		if appName == "" {
			return nil, fmt.Errorf("Fly payload missing app.name")
		}
		match = func(cfg map[string]any) bool {
			if cfg == nil {
				return false
			}
			v, _ := cfg["app_name"].(string)
			return v != "" && v == appName
		}
	}

	integration, err := h.svc.FindIntegration(ctx, provider, match)
	if err != nil {
		return nil, fmt.Errorf("no matching %s integration", provider)
	}
	return integration, nil
}

func stringFromPath(root any, path string) string {
	cur := root
	for _, part := range splitPath(path) {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[part]
	}
	s, _ := cur.(string)
	return s
}

func splitPath(p string) []string {
	// tiny helper so we don't pull strings.Split for one call
	out := []string{}
	start := 0
	for i := 0; i < len(p); i++ {
		if p[i] == '.' {
			out = append(out, p[start:i])
			start = i + 1
		}
	}
	out = append(out, p[start:])
	return out
}
