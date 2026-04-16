package auth

import (
	"net/http"
	"strconv"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// APIKeyHandler provides HTTP endpoints for managing API keys.
type APIKeyHandler struct {
	service *APIKeyService
}

// NewAPIKeyHandler creates a new APIKeyHandler.
func NewAPIKeyHandler(service *APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{service: service}
}

// RegisterRoutes mounts all API key management routes on the given router group.
func (h *APIKeyHandler) RegisterRoutes(rg *gin.RouterGroup) {
	apiKeys := rg.Group("/api-keys")
	{
		apiKeys.POST("", h.createAPIKey)
		apiKeys.GET("", h.listAPIKeys)
		apiKeys.GET("/:id", h.getAPIKey)
		apiKeys.DELETE("/:id", h.revokeAPIKey)
		apiKeys.POST("/:id/rotate", h.rotateAPIKey)
	}
}

// createAPIKeyRequest is the JSON body for creating a new API key.
type createAPIKeyRequest struct {
	Name           string               `json:"name" binding:"required"`
	ProjectID      *uuid.UUID           `json:"project_id"`
	EnvironmentIDs []uuid.UUID          `json:"environment_ids"`
	Scopes         []models.APIKeyScope `json:"scopes" binding:"required"`
	ExpiresAt      *time.Time           `json:"expires_at"`
}

func (h *APIKeyHandler) createAPIKey(c *gin.Context) {
	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	createdBy, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user identity"})
		return
	}

	orgIDValue, ok := c.Get("org_id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization context required"})
		return
	}

	orgIDStr, ok := orgIDValue.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization identity"})
		return
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}

	if req.EnvironmentIDs == nil {
		req.EnvironmentIDs = []uuid.UUID{}
	}

	result, err := h.service.GenerateKey(
		c.Request.Context(),
		orgID,
		req.ProjectID,
		req.Name,
		req.Scopes,
		createdBy,
		req.EnvironmentIDs,
		req.ExpiresAt,
	)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"api_key":       result.APIKey,
		"plaintext_key": result.PlaintextKey,
	})
}

func (h *APIKeyHandler) listAPIKeys(c *gin.Context) {
	orgIDValue, ok := c.Get("org_id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization context required"})
		return
	}

	orgIDStr, ok := orgIDValue.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization identity"})
		return
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}

	var projectID *uuid.UUID
	if pidStr := c.Query("project_id"); pidStr != "" {
		pid, err := uuid.Parse(pidStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
			return
		}
		projectID = &pid
	}

	var environmentID *uuid.UUID
	if eidStr := c.Query("environment_id"); eidStr != "" {
		eid, err := uuid.Parse(eidStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment_id"})
			return
		}
		environmentID = &eid
	}

	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	keys, err := h.service.ListKeys(c.Request.Context(), orgID, projectID, environmentID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list api keys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"api_keys": keys})
}

func (h *APIKeyHandler) getAPIKey(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid api key id"})
		return
	}

	key, err := h.service.GetKey(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "api key not found"})
		return
	}

	c.JSON(http.StatusOK, key)
}

func (h *APIKeyHandler) revokeAPIKey(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid api key id"})
		return
	}

	if err := h.service.RevokeKey(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

func (h *APIKeyHandler) rotateAPIKey(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid api key id"})
		return
	}

	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	createdBy, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user identity"})
		return
	}

	result, err := h.service.RotateKey(c.Request.Context(), id, createdBy)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"api_key":       result.APIKey,
		"plaintext_key": result.PlaintextKey,
	})
}
