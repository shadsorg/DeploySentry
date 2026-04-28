package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/shadsorg/deploysentry/internal/analytics"
	"github.com/shadsorg/deploysentry/internal/auth"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/rollout"
	"github.com/shadsorg/deploysentry/internal/webhooks"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PhaseAdvancer is implemented by any engine that can advance a deployment
// through its next phase (e.g. the canary rollout engine).
type PhaseAdvancer interface {
	Advance(ctx context.Context, deploymentID uuid.UUID) error
}

// RolloutAttacher is implemented by the rollout package and injected at wiring time.
// Kept as an interface inside deploy to avoid a reverse dependency.
type RolloutAttacher interface {
	AttachFromDeployRequest(ctx context.Context, deployment *models.Deployment, req *RolloutAttachRequest, actor uuid.UUID) error
}

// RolloutAttachRequest is the client-supplied rollout intent on a deploy request.
type RolloutAttachRequest struct {
	StrategyName     string          `json:"strategy_name,omitempty"`
	StrategyID       *uuid.UUID      `json:"strategy_id,omitempty"`
	Overrides        json.RawMessage `json:"overrides,omitempty"`
	ReleaseID        *uuid.UUID      `json:"release_id,omitempty"`
	ApplyImmediately bool            `json:"apply_immediately,omitempty"`
}

// Handler provides HTTP endpoints for managing deployments.
type Handler struct {
	service      DeployService
	webhookSvc   *webhooks.Service
	analyticsSvc *analytics.Service
	engine       PhaseAdvancer
	rollouts     RolloutAttacher
}

// NewHandler creates a new deployment HTTP handler.
// engine may be nil; when nil the /advance endpoint returns 503.
func NewHandler(service DeployService, webhookSvc *webhooks.Service, analyticsSvc *analytics.Service, engine PhaseAdvancer) *Handler {
	return &Handler{
		service:      service,
		webhookSvc:   webhookSvc,
		analyticsSvc: analyticsSvc,
		engine:       engine,
	}
}

// NewHandlerWithRollouts creates a Handler with an optional RolloutAttacher.
// The attacher is called after a deployment is created to bind any rollout
// strategy supplied in the POST body. Task 14 wiring should call this instead
// of NewHandler when a rollout service is available.
func NewHandlerWithRollouts(service DeployService, webhookSvc *webhooks.Service, analyticsSvc *analytics.Service, engine PhaseAdvancer, rollouts RolloutAttacher) *Handler {
	h := NewHandler(service, webhookSvc, analyticsSvc, engine)
	h.rollouts = rollouts
	return h
}

// actorFromDeployContext extracts the authenticated user UUID from the Gin
// context. For JWT auth this is user_id from the token; for API-key auth
// it falls back to the ancestor user who minted the key (api_key_created_by).
// Delegates to auth.ActorUserID so the logic stays in one place.
func actorFromDeployContext(c *gin.Context) uuid.UUID {
	return auth.ActorUserID(c)
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
		// NOTE: /:id/rollback is registered by the rollback package handler.
		deployments.POST("/:id/pause", mw(rbac, auth.PermDeployManage), h.pauseDeployment)
		deployments.POST("/:id/resume", mw(rbac, auth.PermDeployManage), h.resumeDeployment)
		deployments.POST("/:id/cancel", mw(rbac, auth.PermDeployManage), h.cancelDeployment)
		deployments.GET("/:id/desired-state", mw(rbac, auth.PermDeployRead), h.getDesiredState)
		deployments.POST("/:id/advance", mw(rbac, auth.PermDeployPromote), h.advanceDeployment)
		deployments.GET("/:id/rollback-history", mw(rbac, auth.PermDeployRead), h.getRollbackHistory)
		deployments.GET("/:id/phases", mw(rbac, auth.PermDeployRead), h.listPhases)
	}

	// Application-scoped routes.
	applications := rg.Group("/applications")
	{
		applications.GET("/:app_id/deployments/active", mw(rbac, auth.PermDeployRead), h.getActiveDeployments)
		applications.GET("/:app_id/desired-state", mw(rbac, auth.PermDeployRead), h.getAppDesiredState)
		applications.GET("/:app_id/artifacts", mw(rbac, auth.PermDeployRead), h.listArtifacts)
		applications.GET("/:app_id/versions", mw(rbac, auth.PermDeployRead), h.listVersions)
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
	ApplicationID uuid.UUID             `json:"application_id" binding:"required"`
	EnvironmentID uuid.UUID             `json:"environment_id" binding:"required"`
	Strategy      string                `json:"strategy"`
	Artifact      string                `json:"artifact" binding:"required"`
	Version       string                `json:"version" binding:"required"`
	CommitSHA     string                `json:"commit_sha"`
	FlagTestKey   *string               `json:"flag_test_key"`
	// Mode is "orchestrate" (default) or "record". In record mode the platform
	// (Railway, Render, …) already deployed the artifact; DeploySentry only
	// stores the fact for history/status and does not drive a rollout.
	Mode    string                `json:"mode,omitempty"`
	Source  string                `json:"source,omitempty"`
	Rollout *RolloutAttachRequest `json:"rollout,omitempty"`
}

func (h *Handler) createDeployment(c *gin.Context) {
	var req createDeploymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// created_by comes from the authenticated caller. For JWT this is the
	// session user; for API-key auth the ancestor user is threaded through
	// the context as api_key_created_by (see auth.ActorUserID). When the
	// caller is an API key with no ancestor on record, surface an
	// actionable 422 instead of the downstream "created_by is required".
	createdBy := auth.ActorUserID(c)
	if method, _ := c.Get("auth_method"); method == "api_key" && createdBy == uuid.Nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "cannot determine created_by: the calling api key has no associated user on record; " +
				"mint a key from the dashboard (which stamps the creating user) and retry",
		})
		return
	}

	mode := models.DeployMode(req.Mode)
	if mode == "" {
		mode = models.DeployModeOrchestrate
	}
	if mode != models.DeployModeOrchestrate && mode != models.DeployModeRecord {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mode must be 'orchestrate' or 'record'"})
		return
	}

	// Strategy is required for orchestrated deploys but optional for recorded ones.
	if mode == models.DeployModeOrchestrate && req.Strategy == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "strategy is required"})
		return
	}

	var sourcePtr *string
	if req.Source != "" {
		s := req.Source
		sourcePtr = &s
	}

	d := &models.Deployment{
		ApplicationID: req.ApplicationID,
		EnvironmentID: req.EnvironmentID,
		Strategy:      models.DeployStrategyType(req.Strategy),
		Artifact:      req.Artifact,
		Version:       req.Version,
		CommitSHA:     req.CommitSHA,
		FlagTestKey:   req.FlagTestKey,
		CreatedBy:     createdBy,
		Mode:          mode,
		Source:        sourcePtr,
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
			"mode":           string(d.Mode),
		}
		if d.Source != nil {
			webhookData["source"] = *d.Source
		}

		var orgID uuid.UUID
		if orgIDVal, exists := c.Get("org_id"); exists {
			orgID, _ = orgIDVal.(uuid.UUID)
		}

		event := models.EventDeploymentCreated
		if d.Mode == models.DeployModeRecord {
			event = models.EventDeploymentRecorded
		}
		if err := h.webhookSvc.PublishEvent(c.Request.Context(), event, orgID, &d.ApplicationID, webhookData, &createdBy); err != nil {
			log.Printf("failed to publish %s webhook: %v", event, err)
		}
	}

	// Recorded deploys never attach a rollout — the platform already shipped.
	if d.Mode != models.DeployModeRecord && h.rollouts != nil && req.Rollout != nil && !req.Rollout.ApplyImmediately {
		actor := actorFromDeployContext(c)
		if err := h.rollouts.AttachFromDeployRequest(c.Request.Context(), d, req.Rollout, actor); err != nil {
			if errors.Is(err, rollout.ErrAlreadyActiveOnTarget) {
				c.JSON(http.StatusConflict, gin.H{
					"error":   "rollout_in_progress",
					"options": []string{"abort", "wait", "amend"},
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
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

	if envIDStr := c.Query("environment_id"); envIDStr != "" {
		envID, err := uuid.Parse(envIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment_id"})
			return
		}
		opts.EnvironmentID = &envID
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			opts.Limit = v
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			opts.Offset = v
		}
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

func (h *Handler) cancelDeployment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}

	if err := h.service.CancelDeployment(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
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

// advanceDeployment handles POST /deployments/:id/advance.
// It delegates to the PhaseAdvancer engine to move the deployment to its next
// phase. Returns 503 when no engine is configured.
func (h *Handler) advanceDeployment(c *gin.Context) {
	if h.engine == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "phase advance engine not available"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}

	if err := h.engine.Advance(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "advanced"})
}

// getRollbackHistory handles GET /deployments/:id/rollback-history.
// It returns the rollback records associated with the given deployment.
func (h *Handler) getRollbackHistory(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}

	records, err := h.service.ListRollbackRecords(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list rollback history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rollbacks": records})
}

// listPhases handles GET /deployments/:id/phases.
// It returns all phases (in sort order) for the given deployment.
func (h *Handler) listPhases(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}

	phases, err := h.service.ListPhases(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list phases"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"phases": phases})
}

// listArtifacts handles GET /applications/:app_id/artifacts.
func (h *Handler) listArtifacts(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("app_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}
	limit := parseLimit(c.Query("limit"), 50)
	items, err := h.service.ListArtifacts(c.Request.Context(), appID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"artifacts": items})
}

// listVersions handles GET /applications/:app_id/versions?environment_id=&limit=.
func (h *Handler) listVersions(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("app_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}
	var envID *uuid.UUID
	if s := c.Query("environment_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment_id"})
			return
		}
		envID = &id
	}
	limit := parseLimit(c.Query("limit"), 50)
	items, err := h.service.ListVersions(c.Request.Context(), appID, envID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"versions": items})
}

// parseLimit returns a positive int from the query string, clamped to 1..100,
// defaulting to def.
func parseLimit(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	if n > 100 {
		return 100
	}
	return n
}
