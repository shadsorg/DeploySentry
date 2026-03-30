package settings

import (
	"encoding/json"
	"net/http"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler exposes HTTP endpoints for hierarchical settings management.
type Handler struct {
	service SettingService
	rbac    *auth.RBACChecker
}

// NewHandler creates a new settings Handler.
func NewHandler(service SettingService, rbac *auth.RBACChecker) *Handler {
	return &Handler{service: service, rbac: rbac}
}

// RegisterRoutes mounts settings endpoints on the provided router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	s := rg.Group("/settings")
	{
		s.GET("", auth.RequirePermission(h.rbac, auth.PermSettingsRead), h.listSettings)
		s.GET("/resolve", auth.RequirePermission(h.rbac, auth.PermSettingsRead), h.resolveSettings)
		s.PUT("", auth.RequirePermission(h.rbac, auth.PermSettingsWrite), h.setSetting)
		s.DELETE("/:id", auth.RequirePermission(h.rbac, auth.PermSettingsWrite), h.deleteSetting)
	}
}

func (h *Handler) listSettings(c *gin.Context) {
	scope := c.Query("scope")
	targetStr := c.Query("target")
	if scope == "" || targetStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope and target query parameters are required"})
		return
	}
	targetID, err := uuid.Parse(targetStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target UUID"})
		return
	}
	settings, err := h.service.List(c.Request.Context(), scope, targetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list settings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

func (h *Handler) resolveSettings(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key query parameter is required"})
		return
	}
	var orgID, projectID, appID, envID *uuid.UUID
	if v := c.Query("org_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org_id"})
			return
		}
		orgID = &id
	}
	if v := c.Query("project_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
			return
		}
		projectID = &id
	}
	if v := c.Query("application_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application_id"})
			return
		}
		appID = &id
	}
	if v := c.Query("environment_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment_id"})
			return
		}
		envID = &id
	}
	setting, err := h.service.Resolve(c.Request.Context(), key, orgID, projectID, appID, envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "setting not found"})
		return
	}
	c.JSON(http.StatusOK, setting)
}

// setSettingRequest is the JSON body for PUT /settings.
type setSettingRequest struct {
	Scope    string          `json:"scope" binding:"required"`
	TargetID string          `json:"target_id" binding:"required"`
	Key      string          `json:"key" binding:"required"`
	Value    json.RawMessage `json:"value" binding:"required"`
}

func (h *Handler) setSetting(c *gin.Context) {
	var req setSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	targetID, err := uuid.Parse(req.TargetID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target_id"})
		return
	}

	setting := &models.Setting{Key: req.Key, Value: req.Value}

	// Extract user ID for updated_by.
	if uid, exists := c.Get("user_id"); exists {
		if userID, ok := uid.(uuid.UUID); ok {
			setting.UpdatedBy = &userID
		}
	}

	switch req.Scope {
	case "org":
		setting.OrgID = &targetID
	case "project":
		setting.ProjectID = &targetID
	case "application":
		setting.ApplicationID = &targetID
	case "environment":
		setting.EnvironmentID = &targetID
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope must be org, project, application, or environment"})
		return
	}

	if err := h.service.Set(c.Request.Context(), setting); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, setting)
}

func (h *Handler) deleteSetting(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid setting id"})
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
