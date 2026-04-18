package registry

import (
	"encoding/json"
	"net/http"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for agent registration and heartbeats.
type Handler struct {
	service Service
}

// NewHandler creates a new agent registry HTTP handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes mounts agent registry API routes on the given router group.
// The rbac parameter is accepted for interface consistency but not used;
// agents authenticate via API key, not RBAC.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, rbac *auth.RBACChecker) {
	agents := rg.Group("/agents")
	{
		agents.POST("/register", h.registerAgent)
		agents.POST("/:id/heartbeat", h.heartbeat)
		agents.DELETE("/:id", h.deregisterAgent)
		agents.GET("/:id/heartbeats", h.listHeartbeats)
	}
	apps := rg.Group("/applications")
	{
		apps.GET("/:app_id/agents", h.listAgents)
	}
}

type registerRequest struct {
	AppID         uuid.UUID       `json:"app_id"`
	EnvironmentID uuid.UUID       `json:"environment_id"`
	Version       string          `json:"version"`
	Upstreams     json.RawMessage `json:"upstreams"`
}

type heartbeatRequest struct {
	DeploymentID  *uuid.UUID                        `json:"deployment_id"`
	ConfigVersion int                               `json:"config_version"`
	ActualTraffic map[string]float64                `json:"actual_traffic"`
	Upstreams     map[string]models.UpstreamMetrics `json:"upstreams"`
	ActiveRules   models.ActiveRules                `json:"active_rules"`
	EnvoyHealthy  bool                              `json:"envoy_healthy"`
}

func (h *Handler) registerAgent(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.AppID == uuid.Nil {
		if appIDStr := c.GetString("api_key_app_id"); appIDStr != "" {
			if parsed, err := uuid.Parse(appIDStr); err == nil {
				req.AppID = parsed
			}
		}
	}

	if req.EnvironmentID == uuid.Nil {
		if envIDs, ok := c.Get("api_key_environment_ids"); ok {
			if ids, ok := envIDs.([]string); ok && len(ids) > 0 {
				if parsed, err := uuid.Parse(ids[0]); err == nil {
					req.EnvironmentID = parsed
				}
			}
		}
	}

	if req.AppID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "app_id is required: either scope your API key to an application or provide app_id in the request body"})
		return
	}

	agent, err := h.service.Register(c.Request.Context(), req.AppID, req.EnvironmentID, req.Version, req.Upstreams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := map[string]interface{}{
		"id":             agent.ID,
		"app_id":         agent.AppID,
		"environment_id": agent.EnvironmentID,
		"status":         agent.Status,
		"version":        agent.Version,
		"registered_at":  agent.RegisteredAt,
	}
	if orgIDStr := c.GetString("org_id"); orgIDStr != "" {
		response["org_id"] = orgIDStr
	}
	if projectIDStr := c.GetString("project_id"); projectIDStr != "" {
		response["project_id"] = projectIDStr
	}
	c.JSON(http.StatusCreated, response)
}

func (h *Handler) heartbeat(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	var req heartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payload := models.HeartbeatPayload{
		AgentID:       id,
		DeploymentID:  req.DeploymentID,
		ConfigVersion: req.ConfigVersion,
		ActualTraffic: req.ActualTraffic,
		Upstreams:     req.Upstreams,
		ActiveRules:   req.ActiveRules,
		EnvoyHealthy:  req.EnvoyHealthy,
	}

	if err := h.service.Heartbeat(c.Request.Context(), id, req.DeploymentID, payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) deregisterAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	if err := h.service.Deregister(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *Handler) listHeartbeats(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	var deploymentID *uuid.UUID
	if raw := c.Query("deployment_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment_id"})
			return
		}
		deploymentID = &parsed
	}

	hbs, err := h.service.LatestHeartbeats(c.Request.Context(), id, deploymentID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"heartbeats": hbs})
}

func (h *Handler) listAgents(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("app_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid app_id"})
		return
	}

	agents, err := h.service.ListByApp(c.Request.Context(), appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if agents == nil {
		agents = []models.Agent{}
	}

	c.JSON(http.StatusOK, gin.H{"agents": agents})
}
