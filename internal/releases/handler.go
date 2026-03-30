package releases

import (
	"net/http"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for managing releases.
type Handler struct {
	service ReleaseService
	rbac    *auth.RBACChecker
}

// NewHandler creates a new release HTTP handler.
func NewHandler(service ReleaseService) *Handler {
	return &Handler{
		service: service,
		rbac:    auth.NewRBACChecker(),
	}
}

// RegisterRoutes mounts all release API routes on the given router group.
// Routes are nested under /applications/:app_id/releases.
// Flat routes under /releases/:id are also registered for operations that
// only require the release ID (get, delete, and state-transition actions).
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	releases := rg.Group("/applications/:app_id/releases")
	{
		releases.POST("", auth.RequirePermission(h.rbac, auth.PermReleaseCreate), h.createRelease)
		releases.GET("", auth.RequirePermission(h.rbac, auth.PermReleaseRead), h.listReleases)
		releases.GET("/:id", auth.RequirePermission(h.rbac, auth.PermReleaseRead), h.getRelease)
		releases.DELETE("/:id", auth.RequirePermission(h.rbac, auth.PermReleaseCreate), h.deleteRelease)
		releases.POST("/:id/start", auth.RequirePermission(h.rbac, auth.PermReleasePromote), h.startRelease)
		releases.POST("/:id/promote", auth.RequirePermission(h.rbac, auth.PermReleasePromote), h.promoteRelease)
		releases.POST("/:id/pause", auth.RequirePermission(h.rbac, auth.PermReleasePromote), h.pauseRelease)
		releases.POST("/:id/rollback", auth.RequirePermission(h.rbac, auth.PermReleasePromote), h.rollbackRelease)
		releases.POST("/:id/complete", auth.RequirePermission(h.rbac, auth.PermReleasePromote), h.completeRelease)
		releases.POST("/:id/flag-changes", auth.RequirePermission(h.rbac, auth.PermReleaseCreate), h.addFlagChange)
		releases.GET("/:id/flag-changes", auth.RequirePermission(h.rbac, auth.PermReleaseRead), h.listFlagChanges)
	}

	// Flat routes for operations by release ID (no app_id needed).
	// listReleases and createRelease are excluded as they require app_id.
	flat := rg.Group("/releases")
	{
		flat.GET("/:id", auth.RequirePermission(h.rbac, auth.PermReleaseRead), h.getRelease)
		flat.DELETE("/:id", auth.RequirePermission(h.rbac, auth.PermReleaseCreate), h.deleteRelease)
		flat.POST("/:id/start", auth.RequirePermission(h.rbac, auth.PermReleasePromote), h.startRelease)
		flat.POST("/:id/promote", auth.RequirePermission(h.rbac, auth.PermReleasePromote), h.promoteRelease)
		flat.POST("/:id/pause", auth.RequirePermission(h.rbac, auth.PermReleasePromote), h.pauseRelease)
		flat.POST("/:id/rollback", auth.RequirePermission(h.rbac, auth.PermReleasePromote), h.rollbackRelease)
		flat.POST("/:id/complete", auth.RequirePermission(h.rbac, auth.PermReleasePromote), h.completeRelease)
		flat.POST("/:id/flag-changes", auth.RequirePermission(h.rbac, auth.PermReleaseCreate), h.addFlagChange)
		flat.GET("/:id/flag-changes", auth.RequirePermission(h.rbac, auth.PermReleaseRead), h.listFlagChanges)
	}
}

// createReleaseRequest is the JSON body for creating a new release.
type createReleaseRequest struct {
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	SessionSticky bool   `json:"session_sticky"`
	StickyHeader  string `json:"sticky_header"`
}

func (h *Handler) createRelease(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("app_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}

	var req createReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var createdBy *uuid.UUID
	if userID, exists := c.Get("user_id"); exists {
		if uid, ok := userID.(uuid.UUID); ok {
			createdBy = &uid
		}
	}

	release := &models.Release{
		ApplicationID: appID,
		Name:          req.Name,
		Description:   req.Description,
		SessionSticky: req.SessionSticky,
		StickyHeader:  req.StickyHeader,
		CreatedBy:     createdBy,
	}

	if err := h.service.Create(c.Request.Context(), release); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, release)
}

func (h *Handler) getRelease(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid release id"})
		return
	}

	release, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "release not found"})
		return
	}

	c.JSON(http.StatusOK, release)
}

func (h *Handler) listReleases(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("app_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}

	releases, err := h.service.ListByApplication(c.Request.Context(), appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list releases"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"releases": releases})
}

func (h *Handler) deleteRelease(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid release id"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *Handler) startRelease(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid release id"})
		return
	}

	if err := h.service.Start(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "started"})
}

// promoteReleaseRequest is the JSON body for promoting a release.
type promoteReleaseRequest struct {
	TrafficPercent int `json:"traffic_percent" binding:"required"`
}

func (h *Handler) promoteRelease(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid release id"})
		return
	}

	var req promoteReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.Promote(c.Request.Context(), id, req.TrafficPercent); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "promoted"})
}

func (h *Handler) pauseRelease(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid release id"})
		return
	}

	if err := h.service.Pause(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "paused"})
}

func (h *Handler) rollbackRelease(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid release id"})
		return
	}

	if err := h.service.Rollback(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "rolled_back"})
}

func (h *Handler) completeRelease(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid release id"})
		return
	}

	if err := h.service.Complete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "completed"})
}

// addFlagChangeRequest is the JSON body for adding a flag change to a release.
type addFlagChangeRequest struct {
	FlagID        uuid.UUID `json:"flag_id" binding:"required"`
	EnvironmentID uuid.UUID `json:"environment_id" binding:"required"`
	NewEnabled    *bool     `json:"new_enabled"`
}

func (h *Handler) addFlagChange(c *gin.Context) {
	releaseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid release id"})
		return
	}

	var req addFlagChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fc := &models.ReleaseFlagChange{
		ReleaseID:     releaseID,
		FlagID:        req.FlagID,
		EnvironmentID: req.EnvironmentID,
		NewEnabled:    req.NewEnabled,
	}

	if err := h.service.AddFlagChange(c.Request.Context(), fc); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, fc)
}

func (h *Handler) listFlagChanges(c *gin.Context) {
	releaseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid release id"})
		return
	}

	changes, err := h.service.ListFlagChanges(c.Request.Context(), releaseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list flag changes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"flag_changes": changes})
}
