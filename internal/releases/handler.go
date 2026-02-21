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
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	releases := rg.Group("/releases")
	{
		releases.POST("", auth.RequirePermission(h.rbac, auth.PermReleaseCreate), h.createRelease)
		releases.GET("", auth.RequirePermission(h.rbac, auth.PermReleaseRead), h.listReleases)
		releases.GET("/:id", auth.RequirePermission(h.rbac, auth.PermReleaseRead), h.getRelease)
		releases.GET("/:id/status", auth.RequirePermission(h.rbac, auth.PermReleaseRead), h.getReleaseStatus)
		releases.POST("/:id/promote", auth.RequirePermission(h.rbac, auth.PermReleasePromote), h.promoteRelease)
	}
}

// createReleaseRequest is the JSON body for creating a new release.
type createReleaseRequest struct {
	ProjectID   uuid.UUID `json:"project_id" binding:"required"`
	Version     string    `json:"version" binding:"required"`
	Title       string    `json:"title" binding:"required"`
	Description string    `json:"description"`
	CommitSHA   string    `json:"commit_sha"`
	Artifact    string    `json:"artifact" binding:"required"`
}

func (h *Handler) createRelease(c *gin.Context) {
	var req createReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	createdBy, ok := userID.(uuid.UUID)
	if !ok {
		createdBy = uuid.Nil
	}

	release := &models.Release{
		ProjectID:   req.ProjectID,
		Version:     req.Version,
		Title:       req.Title,
		Description: req.Description,
		CommitSHA:   req.CommitSHA,
		Artifact:    req.Artifact,
		CreatedBy:   createdBy,
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

	release, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "release not found"})
		return
	}

	c.JSON(http.StatusOK, release)
}

func (h *Handler) listReleases(c *gin.Context) {
	projectIDStr := c.Query("project_id")
	if projectIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id query parameter is required"})
		return
	}

	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
		return
	}

	opts := ListOptions{Limit: 20, Offset: 0}

	releases, err := h.service.List(c.Request.Context(), projectID, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list releases"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"releases": releases})
}

// promoteReleaseRequest is the JSON body for promoting a release.
type promoteReleaseRequest struct {
	EnvironmentID uuid.UUID `json:"environment_id" binding:"required"`
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

	userID, _ := c.Get("user_id")
	deployedBy, ok := userID.(uuid.UUID)
	if !ok {
		deployedBy = uuid.Nil
	}

	if err := h.service.Promote(c.Request.Context(), id, req.EnvironmentID, deployedBy); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "promoted"})
}

// getReleaseStatus returns the release status across all environments.
func (h *Handler) getReleaseStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid release id"})
		return
	}

	status, err := h.service.GetReleaseStatus(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "release not found"})
		return
	}

	c.JSON(http.StatusOK, status)
}
