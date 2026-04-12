package deploy

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/deploysentry/deploysentry/internal/analytics"
	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/webhooks"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for managing deployments.
type Handler struct {
	service      DeployService
	webhookSvc   *webhooks.Service
	analyticsSvc *analytics.Service
}

// NewHandler creates a new deployment HTTP handler.
func NewHandler(service DeployService, webhookSvc *webhooks.Service, analyticsSvc *analytics.Service) *Handler {
	return &Handler{
		service:      service,
		webhookSvc:   webhookSvc,
		analyticsSvc: analyticsSvc,
	}
}

// RegisterRoutes mounts all deployment API routes on the given router group.
// Each route is protected by RBAC middleware that verifies the caller holds
// the required permission. When rbac is nil the routes are registered without
// permission checks (useful for testing).
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, rbac *auth.RBACChecker) {
	deployments := rg.Group("/deployments")
	{
		deployments.POST("", mw(rbac, auth.PermDeployCreate), h.createDeployment)
		deployments.GET("", mw(rbac, auth.PermDeployRead), h.listDeployments)
		deployments.GET("/:id", mw(rbac, auth.PermDeployRead), h.getDeployment)
		deployments.POST("/:id/promote", mw(rbac, auth.PermDeployPromote), h.promoteDeployment)
		deployments.POST("/:id/rollback", mw(rbac, auth.PermDeployRollback), h.rollbackDeployment)
		deployments.POST("/:id/pause", mw(rbac, auth.PermDeployManage), h.pauseDeployment)
		deployments.POST("/:id/resume", mw(rbac, auth.PermDeployManage), h.resumeDeployment)
	}

	// Application-scoped routes.
	applications := rg.Group("/applications")
	{
		applications.GET("/:app_id/deployments/active", mw(rbac, auth.PermDeployRead), h.getActiveDeployments)
	}
}

// mw returns a RequirePermission middleware when rbac is non-nil, or a no-op
// handler when rbac is nil (for backwards-compatible test setups).
func mw(rbac *auth.RBACChecker, perm auth.Permission) gin.HandlerFunc {
	if rbac == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return auth.RequirePermission(rbac, perm)
}

// createDeploymentRequest is the JSON body for creating a new deployment.
type createDeploymentRequest struct {
	ApplicationID uuid.UUID `json:"application_id" binding:"required"`
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
		ApplicationID: req.ApplicationID,
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

	// Record deployment analytics event
	if h.analyticsSvc != nil {
		event := &models.DeploymentEvent{
			DeploymentID: d.ID,
			EventType:    models.DeploymentEventCreated,
			PhaseName:    "creation",
			TriggeredBy:  &createdBy,
			OccurredAt:   time.Now(),
		}

		go func() {
			if err := h.analyticsSvc.RecordDeploymentEvent(context.Background(), event); err != nil {
				log.Printf("failed to record deployment event: %v", err)
			}
		}()
	}

	// Trigger webhook event for deployment creation
	if h.webhookSvc != nil {
		webhookData := map[string]interface{}{
			"deployment_id":  d.ID,
			"application_id": d.ApplicationID,
			"environment_id": d.EnvironmentID,
			"version":        d.Version,
			"strategy":       string(d.Strategy),
			"artifact":       d.Artifact,
			"commit_sha":     d.CommitSHA,
			"status":         string(d.Status),
		}

		var orgID uuid.UUID
		if orgIDVal, exists := c.Get("org_id"); exists {
			orgID, _ = orgIDVal.(uuid.UUID)
		}

		if err := h.webhookSvc.PublishEvent(c.Request.Context(), models.EventDeploymentCreated, orgID, &d.ApplicationID, webhookData, &createdBy); err != nil {
			log.Printf("failed to publish deployment created webhook: %v", err)
		}
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
	appIDStr := c.Query("app_id")
	if appIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "app_id query parameter is required"})
		return
	}

	applicationID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid app_id"})
		return
	}

	opts := ListOptions{
		Limit:  20,
		Offset: 0,
	}

	deployments, err := h.service.ListDeployments(c.Request.Context(), applicationID, opts)
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

	// Get deployment details for webhook
	deployment, err := h.service.GetDeployment(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
		return
	}

	if err := h.service.RollbackDeployment(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	// Record deployment analytics event
	if h.analyticsSvc != nil {
		userID, _ := c.Get("user_id")
		rolledBackBy, ok := userID.(uuid.UUID)
		if !ok {
			rolledBackBy = uuid.Nil
		}

		event := &models.DeploymentEvent{
			DeploymentID: deployment.ID,
			EventType:    models.DeploymentEventRolledBack,
			PhaseName:    "rollback",
			TriggeredBy:  &rolledBackBy,
			OccurredAt:   time.Now(),
		}

		go func() {
			if err := h.analyticsSvc.RecordDeploymentEvent(context.Background(), event); err != nil {
				log.Printf("failed to record deployment rollback event: %v", err)
			}
		}()
	}

	// Trigger webhook event for deployment rollback
	if h.webhookSvc != nil {
		userID, _ := c.Get("user_id")
		rolledBackBy, ok := userID.(uuid.UUID)
		if !ok {
			rolledBackBy = uuid.Nil
		}

		webhookData := map[string]interface{}{
			"deployment_id":  deployment.ID,
			"application_id": deployment.ApplicationID,
			"environment_id": deployment.EnvironmentID,
			"version":        deployment.Version,
			"strategy":       string(deployment.Strategy),
		}

		var orgID uuid.UUID
		if orgIDVal, exists := c.Get("org_id"); exists {
			orgID, _ = orgIDVal.(uuid.UUID)
		}

		if err := h.webhookSvc.PublishEvent(c.Request.Context(), models.EventDeploymentRolledback, orgID, &deployment.ApplicationID, webhookData, &rolledBackBy); err != nil {
			log.Printf("failed to publish deployment rolled back webhook: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "rolled_back"})
}

func (h *Handler) pauseDeployment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}

	// Get deployment details for webhook
	deployment, err := h.service.GetDeployment(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
		return
	}

	if err := h.service.PauseDeployment(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	// Record deployment analytics event
	if h.analyticsSvc != nil {
		userID, _ := c.Get("user_id")
		pausedBy, ok := userID.(uuid.UUID)
		if !ok {
			pausedBy = uuid.Nil
		}

		event := &models.DeploymentEvent{
			DeploymentID: deployment.ID,
			EventType:    models.DeploymentEventPaused,
			PhaseName:    "pause",
			TriggeredBy:  &pausedBy,
			OccurredAt:   time.Now(),
		}

		go func() {
			if err := h.analyticsSvc.RecordDeploymentEvent(context.Background(), event); err != nil {
				log.Printf("failed to record deployment paused event: %v", err)
			}
		}()
	}

	// Trigger webhook event for deployment pause
	if h.webhookSvc != nil {
		userID, _ := c.Get("user_id")
		pausedBy, ok := userID.(uuid.UUID)
		if !ok {
			pausedBy = uuid.Nil
		}

		webhookData := map[string]interface{}{
			"deployment_id":  deployment.ID,
			"application_id": deployment.ApplicationID,
			"environment_id": deployment.EnvironmentID,
			"version":        deployment.Version,
			"strategy":       string(deployment.Strategy),
		}

		var orgID uuid.UUID
		if orgIDVal, exists := c.Get("org_id"); exists {
			orgID, _ = orgIDVal.(uuid.UUID)
		}

		if err := h.webhookSvc.PublishEvent(c.Request.Context(), models.EventDeploymentPaused, orgID, &deployment.ApplicationID, webhookData, &pausedBy); err != nil {
			log.Printf("failed to publish deployment paused webhook: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "paused"})
}

func (h *Handler) resumeDeployment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}

	// Get deployment details for webhook
	deployment, err := h.service.GetDeployment(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
		return
	}

	if err := h.service.ResumeDeployment(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	// Record deployment analytics event
	if h.analyticsSvc != nil {
		userID, _ := c.Get("user_id")
		resumedBy, ok := userID.(uuid.UUID)
		if !ok {
			resumedBy = uuid.Nil
		}

		event := &models.DeploymentEvent{
			DeploymentID: deployment.ID,
			EventType:    models.DeploymentEventResumed,
			PhaseName:    "resume",
			TriggeredBy:  &resumedBy,
			OccurredAt:   time.Now(),
		}

		go func() {
			if err := h.analyticsSvc.RecordDeploymentEvent(context.Background(), event); err != nil {
				log.Printf("failed to record deployment resumed event: %v", err)
			}
		}()
	}

	// Trigger webhook event for deployment resume
	if h.webhookSvc != nil {
		userID, _ := c.Get("user_id")
		resumedBy, ok := userID.(uuid.UUID)
		if !ok {
			resumedBy = uuid.Nil
		}

		webhookData := map[string]interface{}{
			"deployment_id":  deployment.ID,
			"application_id": deployment.ApplicationID,
			"environment_id": deployment.EnvironmentID,
			"version":        deployment.Version,
			"strategy":       string(deployment.Strategy),
		}

		var orgID uuid.UUID
		if orgIDVal, exists := c.Get("org_id"); exists {
			orgID, _ = orgIDVal.(uuid.UUID)
		}

		if err := h.webhookSvc.PublishEvent(c.Request.Context(), models.EventDeploymentResumed, orgID, &deployment.ApplicationID, webhookData, &resumedBy); err != nil {
			log.Printf("failed to publish deployment resumed webhook: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "running"})
}

// getActiveDeployments returns all non-terminal deployments for the application
// identified by the :app_id URL parameter.
func (h *Handler) getActiveDeployments(c *gin.Context) {
	applicationID, err := uuid.Parse(c.Param("app_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}

	deployments, err := h.service.GetActiveDeployments(c.Request.Context(), applicationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get active deployments"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deployments": deployments})
}
