package deploy

import (
	"net/http"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for managing deployments.
type Handler struct {
	service DeployService
}

// NewHandler creates a new deployment HTTP handler.
func NewHandler(service DeployService) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes mounts all deployment API routes on the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	deployments := rg.Group("/deployments")
	{
		deployments.POST("", h.createDeployment)
		deployments.GET("", h.listDeployments)
		deployments.GET("/:id", h.getDeployment)
		deployments.POST("/:id/promote", h.promoteDeployment)
		deployments.POST("/:id/rollback", h.rollbackDeployment)
		deployments.POST("/:id/pause", h.pauseDeployment)
		deployments.POST("/:id/resume", h.resumeDeployment)
	}
}

// createDeploymentRequest is the JSON body for creating a new deployment.
type createDeploymentRequest struct {
	ProjectID     uuid.UUID `json:"project_id" binding:"required"`
	EnvironmentID uuid.UUID `json:"environment_id" binding:"required"`
	Strategy      string    `json:"strategy" binding:"required"`
	Artifact      string    `json:"artifact" binding:"required"`
	Version       string    `json:"version" binding:"required"`
	CommitSHA     string    `json:"commit_sha"`
}

func (h *Handler) createDeployment(c *gin.Context) {
	var req createDeploymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// In a real application, created_by comes from the authenticated user context.
	userID, _ := c.Get("user_id")
	createdBy, ok := userID.(uuid.UUID)
	if !ok {
		createdBy = uuid.Nil
	}

	d := &models.Deployment{
		ProjectID:     req.ProjectID,
		EnvironmentID: req.EnvironmentID,
		Strategy:      models.DeployStrategyType(req.Strategy),
		Artifact:      req.Artifact,
		Version:       req.Version,
		CommitSHA:     req.CommitSHA,
		CreatedBy:     createdBy,
	}

	if err := h.service.CreateDeployment(c.Request.Context(), d); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, d)
}

func (h *Handler) getDeployment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}

	d, err := h.service.GetDeployment(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
		return
	}

	c.JSON(http.StatusOK, d)
}

func (h *Handler) listDeployments(c *gin.Context) {
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

	opts := ListOptions{
		Limit:  20,
		Offset: 0,
	}

	deployments, err := h.service.ListDeployments(c.Request.Context(), projectID, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list deployments"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deployments": deployments})
}

func (h *Handler) promoteDeployment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}

	if err := h.service.PromoteDeployment(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "promoting"})
}

func (h *Handler) rollbackDeployment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}

	if err := h.service.RollbackDeployment(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "rolled_back"})
}

func (h *Handler) pauseDeployment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}

	if err := h.service.PauseDeployment(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "paused"})
}

func (h *Handler) resumeDeployment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}

	if err := h.service.ResumeDeployment(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "running"})
}
