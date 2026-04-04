package flags

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/deploysentry/deploysentry/internal/analytics"
	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/webhooks"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// FlagRatingSvc is the subset of ratings.RatingService needed by the flags handler.
// Defined as a local interface to avoid a circular import.
type FlagRatingSvc interface {
	GetRatingSummary(ctx context.Context, flagID uuid.UUID) (*models.RatingSummary, error)
	GetErrorSummary(ctx context.Context, flagID uuid.UUID, period time.Duration) (*models.ErrorSummary, error)
	IsRatingsEnabled(ctx context.Context, orgID uuid.UUID) (bool, error)
}

// flagWithRatings wraps a FeatureFlag with optional rating and error data.
type flagWithRatings struct {
	*models.FeatureFlag
	RatingSummary *models.RatingSummary `json:"rating_summary,omitempty"`
	ErrorRate     *models.ErrorSummary  `json:"error_rate,omitempty"`
}

// Handler provides HTTP endpoints for managing feature flags.
type Handler struct {
	service      FlagService
	rbac         *auth.RBACChecker
	sse          *SSEBroker
	webhookSvc   *webhooks.Service
	analyticsSvc *analytics.Service
	ratingSvc    FlagRatingSvc
}

// NewHandler creates a new feature flag HTTP handler.
func NewHandler(service FlagService, rbac *auth.RBACChecker, webhookSvc *webhooks.Service, analyticsSvc *analytics.Service) *Handler {
	return &Handler{
		service:      service,
		rbac:         rbac,
		sse:          NewSSEBroker(),
		webhookSvc:   webhookSvc,
		analyticsSvc: analyticsSvc,
	}
}

// SetRatingService wires up an optional rating service for augmented flag responses.
func (h *Handler) SetRatingService(svc FlagRatingSvc) { h.ratingSvc = svc }

// RegisterRoutes mounts all feature flag API routes on the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	flags := rg.Group("/flags")
	{
		flags.POST("", auth.RequirePermission(h.rbac, auth.PermFlagCreate), h.createFlag)
		flags.GET("", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.listFlags)
		flags.GET("/stream", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.streamFlags)
		flags.POST("/evaluate", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.evaluate)
		flags.POST("/batch-evaluate", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.batchEvaluate)
		flags.POST("/bulk-toggle", auth.RequirePermission(h.rbac, auth.PermFlagToggle), h.bulkToggle)
		flags.GET("/:id", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.getFlag)
		flags.PUT("/:id", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.updateFlag)
		flags.POST("/:id/archive", auth.RequirePermission(h.rbac, auth.PermFlagArchive), h.archiveFlag)
		flags.POST("/:id/toggle", auth.RequirePermission(h.rbac, auth.PermFlagToggle), h.toggleFlag)

		rules := flags.Group("/:id/rules")
		{
			rules.GET("", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.listRules)
			rules.POST("", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.addRule)
			rules.PUT("/:ruleId", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.updateRule)
			rules.DELETE("/:ruleId", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.deleteRule)
		}

		envs := flags.Group("/:id/environments")
		{
			envs.GET("", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.listFlagEnvStates)
			envs.PUT("/:envId", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.setFlagEnvState)
		}
	}
}

// createFlagRequest is the JSON body for creating a new feature flag.
type createFlagRequest struct {
	ProjectID     uuid.UUID  `json:"project_id" binding:"required"`
	EnvironmentID uuid.UUID  `json:"environment_id" binding:"required"`
	ApplicationID *uuid.UUID `json:"application_id"`
	Key           string     `json:"key" binding:"required"`
	Name          string     `json:"name" binding:"required"`
	Description   string     `json:"description"`
	FlagType      string     `json:"flag_type" binding:"required"`
	Category      string     `json:"category"`
	Purpose       string     `json:"purpose"`
	Owners        []string   `json:"owners"`
	IsPermanent   bool       `json:"is_permanent"`
	ExpiresAt     *time.Time `json:"expires_at"`
	DefaultValue  string     `json:"default_value"`
	Tags          []string   `json:"tags"`
}

func (h *Handler) createFlag(c *gin.Context) {
	var req createFlagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	createdBy, ok := userID.(uuid.UUID)
	if !ok {
		createdBy = uuid.Nil
	}

	category := models.FlagCategory(req.Category)
	if category == "" {
		category = models.FlagCategoryFeature
	}

	flag := &models.FeatureFlag{
		ProjectID:     req.ProjectID,
		EnvironmentID: req.EnvironmentID,
		ApplicationID: req.ApplicationID,
		Key:           req.Key,
		Name:          req.Name,
		Description:   req.Description,
		FlagType:      models.FlagType(req.FlagType),
		Category:      category,
		Purpose:       req.Purpose,
		Owners:        req.Owners,
		IsPermanent:   req.IsPermanent,
		ExpiresAt:     req.ExpiresAt,
		DefaultValue:  req.DefaultValue,
		Tags:          req.Tags,
		Enabled:       false,
		CreatedBy:     createdBy,
	}
	if flag.Tags == nil {
		flag.Tags = []string{}
	}
	if flag.Owners == nil {
		flag.Owners = []string{}
	}

	if err := h.service.CreateFlag(c.Request.Context(), flag); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	// Trigger webhook event for flag creation
	if h.webhookSvc != nil {
		webhookData := map[string]interface{}{
			"flag_id":      flag.ID,
			"flag_key":     flag.Key,
			"flag_name":    flag.Name,
			"project_id":   flag.ProjectID,
			"environment_id": flag.EnvironmentID,
			"category":     string(flag.Category),
			"flag_type":    string(flag.FlagType),
			"enabled":      flag.Enabled,
		}

		// Get org ID from context (assuming it's set by middleware)
		var orgID uuid.UUID
		if orgIDVal, exists := c.Get("org_id"); exists {
			orgID, _ = orgIDVal.(uuid.UUID)
		}

		if err := h.webhookSvc.PublishEvent(c.Request.Context(), models.EventFlagCreated, orgID, &flag.ProjectID, webhookData, &createdBy); err != nil {
			log.Printf("failed to publish flag created webhook: %v", err)
		}
	}

	c.JSON(http.StatusCreated, flag)
}

func (h *Handler) getFlag(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
		return
	}

	flag, err := h.service.GetFlag(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "flag not found"})
		return
	}

	if h.ratingSvc != nil {
		resp := &flagWithRatings{FeatureFlag: flag}

		// Always attach error summary for the trailing 7-day window.
		if errSummary, err := h.ratingSvc.GetErrorSummary(c.Request.Context(), id, 7*24*time.Hour); err == nil {
			resp.ErrorRate = errSummary
		}

		// Attach rating summary only when ratings are enabled for this org.
		orgIDStr := c.GetString("org_id")
		if orgIDStr != "" {
			if orgID, parseErr := uuid.Parse(orgIDStr); parseErr == nil {
				if enabled, checkErr := h.ratingSvc.IsRatingsEnabled(c.Request.Context(), orgID); checkErr == nil && enabled {
					if ratingSummary, rErr := h.ratingSvc.GetRatingSummary(c.Request.Context(), id); rErr == nil {
						resp.RatingSummary = ratingSummary
					}
				}
			}
		}

		c.JSON(http.StatusOK, resp)
		return
	}

	c.JSON(http.StatusOK, flag)
}

func (h *Handler) listFlags(c *gin.Context) {
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

	flags, err := h.service.ListFlags(c.Request.Context(), projectID, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list flags"})
		return
	}

	if h.ratingSvc != nil {
		enriched := make([]*flagWithRatings, len(flags))
		orgIDStr := c.GetString("org_id")
		orgID, _ := uuid.Parse(orgIDStr)
		ratingsEnabled := false
		if orgID != uuid.Nil {
			ratingsEnabled, _ = h.ratingSvc.IsRatingsEnabled(c.Request.Context(), orgID)
		}
		for i, f := range flags {
			resp := &flagWithRatings{FeatureFlag: f}
			if errSummary, sErr := h.ratingSvc.GetErrorSummary(c.Request.Context(), f.ID, 7*24*time.Hour); sErr == nil {
				resp.ErrorRate = errSummary
			}
			if ratingsEnabled {
				if ratingSummary, rErr := h.ratingSvc.GetRatingSummary(c.Request.Context(), f.ID); rErr == nil {
					resp.RatingSummary = ratingSummary
				}
			}
			enriched[i] = resp
		}
		c.JSON(http.StatusOK, gin.H{"flags": enriched})
		return
	}

	c.JSON(http.StatusOK, gin.H{"flags": flags})
}

// updateFlagRequest is the JSON body for updating a feature flag.
type updateFlagRequest struct {
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Category     string     `json:"category"`
	Purpose      string     `json:"purpose"`
	Owners       []string   `json:"owners"`
	IsPermanent  *bool      `json:"is_permanent"`
	ExpiresAt    *time.Time `json:"expires_at"`
	DefaultValue string     `json:"default_value"`
}

func (h *Handler) updateFlag(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
		return
	}

	flag, err := h.service.GetFlag(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "flag not found"})
		return
	}

	var req updateFlagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != "" {
		flag.Name = req.Name
	}
	if req.Description != "" {
		flag.Description = req.Description
	}
	if req.Category != "" {
		flag.Category = models.FlagCategory(req.Category)
	}
	if req.Purpose != "" {
		flag.Purpose = req.Purpose
	}
	if req.Owners != nil {
		flag.Owners = req.Owners
	}
	if req.IsPermanent != nil {
		flag.IsPermanent = *req.IsPermanent
	}
	if req.ExpiresAt != nil {
		flag.ExpiresAt = req.ExpiresAt
	}
	if req.DefaultValue != "" {
		flag.DefaultValue = req.DefaultValue
	}

	if err := h.service.UpdateFlag(c.Request.Context(), flag); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	// Trigger webhook event for flag update
	if h.webhookSvc != nil {
		userID, _ := c.Get("user_id")
		updatedBy, ok := userID.(uuid.UUID)
		if !ok {
			updatedBy = uuid.Nil
		}

		webhookData := map[string]interface{}{
			"flag_id":        flag.ID,
			"flag_key":       flag.Key,
			"flag_name":      flag.Name,
			"project_id":     flag.ProjectID,
			"environment_id": flag.EnvironmentID,
			"category":       string(flag.Category),
			"enabled":        flag.Enabled,
		}

		var orgID uuid.UUID
		if orgIDVal, exists := c.Get("org_id"); exists {
			orgID, _ = orgIDVal.(uuid.UUID)
		}

		if err := h.webhookSvc.PublishEvent(c.Request.Context(), models.EventFlagUpdated, orgID, &flag.ProjectID, webhookData, &updatedBy); err != nil {
			log.Printf("failed to publish flag updated webhook: %v", err)
		}
	}

	c.JSON(http.StatusOK, flag)
}

func (h *Handler) archiveFlag(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
		return
	}

	// Get flag details before archiving for webhook
	flag, err := h.service.GetFlag(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "flag not found"})
		return
	}

	if err := h.service.ArchiveFlag(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	// Trigger webhook event for flag archive
	if h.webhookSvc != nil {
		userID, _ := c.Get("user_id")
		archivedBy, ok := userID.(uuid.UUID)
		if !ok {
			archivedBy = uuid.Nil
		}

		webhookData := map[string]interface{}{
			"flag_id":        flag.ID,
			"flag_key":       flag.Key,
			"flag_name":      flag.Name,
			"project_id":     flag.ProjectID,
			"environment_id": flag.EnvironmentID,
			"category":       string(flag.Category),
		}

		var orgID uuid.UUID
		if orgIDVal, exists := c.Get("org_id"); exists {
			orgID, _ = orgIDVal.(uuid.UUID)
		}

		if err := h.webhookSvc.PublishEvent(c.Request.Context(), models.EventFlagArchived, orgID, &flag.ProjectID, webhookData, &archivedBy); err != nil {
			log.Printf("failed to publish flag archived webhook: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "archived"})
}

// toggleFlagRequest is the JSON body for toggling a feature flag.
type toggleFlagRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *Handler) toggleFlag(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
		return
	}

	var req toggleFlagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get flag details for webhook
	flag, err := h.service.GetFlag(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "flag not found"})
		return
	}

	if err := h.service.ToggleFlag(c.Request.Context(), id, req.Enabled); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	// Broadcast toggle event to SSE clients.
	h.sse.Broadcast(fmt.Sprintf(`{"event":"flag.toggled","flag_id":"%s","enabled":%t}`, id, req.Enabled))

	// Trigger webhook event for flag toggle
	if h.webhookSvc != nil {
		userID, _ := c.Get("user_id")
		toggledBy, ok := userID.(uuid.UUID)
		if !ok {
			toggledBy = uuid.Nil
		}

		webhookData := map[string]interface{}{
			"flag_id":        flag.ID,
			"flag_key":       flag.Key,
			"flag_name":      flag.Name,
			"project_id":     flag.ProjectID,
			"environment_id": flag.EnvironmentID,
			"enabled":        req.Enabled,
			"previous_state": flag.Enabled,
		}

		var orgID uuid.UUID
		if orgIDVal, exists := c.Get("org_id"); exists {
			orgID, _ = orgIDVal.(uuid.UUID)
		}

		if err := h.webhookSvc.PublishEvent(c.Request.Context(), models.EventFlagToggled, orgID, &flag.ProjectID, webhookData, &toggledBy); err != nil {
			// Log error but don't fail the request
		}
	}

	c.JSON(http.StatusOK, gin.H{"enabled": req.Enabled})
}

// bulkToggleRequest is the JSON body for toggling multiple flags.
type bulkToggleRequest struct {
	FlagIDs []uuid.UUID `json:"flag_ids" binding:"required"`
	Enabled bool        `json:"enabled"`
}

func (h *Handler) bulkToggle(c *gin.Context) {
	var req bulkToggleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.FlagIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "flag_ids must not be empty"})
		return
	}

	if err := h.service.BulkToggle(c.Request.Context(), req.FlagIDs, req.Enabled); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"toggled": len(req.FlagIDs), "enabled": req.Enabled})
}

// evaluateRequest is the JSON body for evaluating a feature flag.
type evaluateRequest struct {
	ProjectID     uuid.UUID                `json:"project_id" binding:"required"`
	EnvironmentID uuid.UUID                `json:"environment_id" binding:"required"`
	FlagKey       string                   `json:"flag_key" binding:"required"`
	Context       models.EvaluationContext `json:"context"`
}

func (h *Handler) evaluate(c *gin.Context) {
	var req evaluateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startTime := time.Now()
	result, err := h.service.Evaluate(c.Request.Context(), req.ProjectID, req.EnvironmentID, req.FlagKey, req.Context)
	latencyMs := float64(time.Since(startTime).Nanoseconds()) / 1e6

	if err != nil {
		// Record failed evaluation
		if h.analyticsSvc != nil {
			errMsg := err.Error()
			ip := net.ParseIP(c.ClientIP())
			contextAttrs := make(map[string]interface{}, len(req.Context.Attributes))
			for k, v := range req.Context.Attributes {
				contextAttrs[k] = v
			}

			event := &models.FlagEvaluationEvent{
				ProjectID:     req.ProjectID,
				EnvironmentID: req.EnvironmentID,
				FlagKey:       req.FlagKey,
				UserID:        req.Context.UserID,
				ResultValue:   "",
				LatencyMs:     int(latencyMs),
				CacheHit:      false,
				ErrorMessage:  errMsg,
				IPAddress:     &ip,
				UserAgent:     c.GetHeader("User-Agent"),
				ContextAttrs:  contextAttrs,
				EvaluatedAt:   time.Now(),
				SDKVersion:    c.GetHeader("X-DeploySentry-SDK-Version"),
			}

			go func() {
				if recordErr := h.analyticsSvc.RecordFlagEvaluation(c.Request.Context(), event); recordErr != nil {
					// Log error but don't fail the request
				}
			}()
		}

		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Record successful evaluation
	if h.analyticsSvc != nil {
		ip := net.ParseIP(c.ClientIP())
		contextAttrs := make(map[string]interface{}, len(req.Context.Attributes))
		for k, v := range req.Context.Attributes {
			contextAttrs[k] = v
		}

		var ruleID *uuid.UUID
		if result.RuleID != "" {
			if parsed, parseErr := uuid.Parse(result.RuleID); parseErr == nil {
				ruleID = &parsed
			}
		}

		event := &models.FlagEvaluationEvent{
			ProjectID:     req.ProjectID,
			EnvironmentID: req.EnvironmentID,
			FlagKey:       req.FlagKey,
			UserID:        req.Context.UserID,
			ResultValue:   result.Value,
			RuleID:        ruleID,
			LatencyMs:     int(latencyMs),
			CacheHit:      false,
			IPAddress:     &ip,
			UserAgent:     c.GetHeader("User-Agent"),
			ContextAttrs:  contextAttrs,
			EvaluatedAt:   time.Now(),
			SDKVersion:    c.GetHeader("X-DeploySentry-SDK-Version"),
		}

		go func() {
			if recordErr := h.analyticsSvc.RecordFlagEvaluation(c.Request.Context(), event); recordErr != nil {
				// Log error but don't fail the request
			}
		}()
	}

	c.JSON(http.StatusOK, result)
}

// batchEvaluateRequest is the JSON body for evaluating multiple feature flags.
type batchEvaluateRequest struct {
	ProjectID     uuid.UUID                `json:"project_id" binding:"required"`
	EnvironmentID uuid.UUID                `json:"environment_id" binding:"required"`
	FlagKeys      []string                 `json:"flag_keys" binding:"required"`
	Context       models.EvaluationContext `json:"context"`
}

func (h *Handler) batchEvaluate(c *gin.Context) {
	var req batchEvaluateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.FlagKeys) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "flag_keys must not be empty"})
		return
	}

	startTime := time.Now()
	results, err := h.service.BatchEvaluate(c.Request.Context(), req.ProjectID, req.EnvironmentID, req.FlagKeys, req.Context)
	latencyMs := float64(time.Since(startTime).Nanoseconds()) / 1e6

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record analytics for each flag evaluation in batch
	if h.analyticsSvc != nil && len(results) > 0 {
		ip := net.ParseIP(c.ClientIP())
		contextAttrs := make(map[string]interface{}, len(req.Context.Attributes))
		for k, v := range req.Context.Attributes {
			contextAttrs[k] = v
		}
		avgLatency := int(latencyMs / float64(len(results)))

		baseEvent := models.FlagEvaluationEvent{
			ProjectID:     req.ProjectID,
			EnvironmentID: req.EnvironmentID,
			UserID:        req.Context.UserID,
			LatencyMs:     avgLatency,
			IPAddress:     &ip,
			UserAgent:     c.GetHeader("User-Agent"),
			ContextAttrs:  contextAttrs,
			EvaluatedAt:   time.Now(),
			SDKVersion:    c.GetHeader("X-DeploySentry-SDK-Version"),
		}

		go func() {
			for _, result := range results {
				event := baseEvent
				event.FlagKey = result.FlagKey
				event.ResultValue = result.Value
				if result.RuleID != "" {
					if parsed, parseErr := uuid.Parse(result.RuleID); parseErr == nil {
						event.RuleID = &parsed
					}
				}

				if recordErr := h.analyticsSvc.RecordFlagEvaluation(c.Request.Context(), &event); recordErr != nil {
					// Log error but continue with other events
				}
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

// addRuleRequest is the JSON body for adding a targeting rule.
type addRuleRequest struct {
	RuleType     string     `json:"rule_type" binding:"required"`
	Priority     int        `json:"priority"`
	Value        string     `json:"value"`
	Percentage   *int       `json:"percentage"`
	Attribute    string     `json:"attribute"`
	Operator     string     `json:"operator"`
	TargetValues []string   `json:"target_values"`
	SegmentID    *uuid.UUID `json:"segment_id"`
}

func (h *Handler) addRule(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
		return
	}

	var req addRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule := &models.TargetingRule{
		FlagID:       flagID,
		RuleType:     models.RuleType(req.RuleType),
		Priority:     req.Priority,
		Value:        req.Value,
		Percentage:   req.Percentage,
		Attribute:    req.Attribute,
		Operator:     req.Operator,
		TargetValues: req.TargetValues,
		SegmentID:    req.SegmentID,
		Enabled:      true,
	}

	if err := h.service.AddRule(c.Request.Context(), rule); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rule)
}

func (h *Handler) updateRule(c *gin.Context) {
	ruleID, err := uuid.Parse(c.Param("ruleId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule id"})
		return
	}

	var req addRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
		return
	}

	rule := &models.TargetingRule{
		ID:           ruleID,
		FlagID:       flagID,
		RuleType:     models.RuleType(req.RuleType),
		Priority:     req.Priority,
		Value:        req.Value,
		Percentage:   req.Percentage,
		Attribute:    req.Attribute,
		Operator:     req.Operator,
		TargetValues: req.TargetValues,
		SegmentID:    req.SegmentID,
		Enabled:      true,
	}

	if err := h.service.UpdateRule(c.Request.Context(), rule); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rule)
}

func (h *Handler) deleteRule(c *gin.Context) {
	ruleID, err := uuid.Parse(c.Param("ruleId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule id"})
		return
	}

	if err := h.service.DeleteRule(c.Request.Context(), ruleID); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *Handler) listRules(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
		return
	}

	rules, err := h.service.ListRules(c.Request.Context(), flagID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list rules"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

// ---------------------------------------------------------------------------
// FlagEnvironmentState handlers
// ---------------------------------------------------------------------------

func (h *Handler) listFlagEnvStates(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
		return
	}
	states, err := h.service.ListFlagEnvStates(c.Request.Context(), flagID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list environment states"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"environment_states": states})
}

type setFlagEnvStateRequest struct {
	Enabled bool             `json:"enabled"`
	Value   *json.RawMessage `json:"value,omitempty"`
}

func (h *Handler) setFlagEnvState(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
		return
	}
	envID, err := uuid.Parse(c.Param("envId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment id"})
		return
	}
	var req setFlagEnvStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	state := &models.FlagEnvironmentState{
		FlagID:        flagID,
		EnvironmentID: envID,
		Enabled:       req.Enabled,
		Value:         req.Value,
	}

	if uid, exists := c.Get("user_id"); exists {
		if userID, ok := uid.(uuid.UUID); ok {
			state.UpdatedBy = &userID
		}
	}

	if err := h.service.SetFlagEnvState(c.Request.Context(), state); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, state)
}

// --- SSE (Server-Sent Events) support ---

// SSEBroker manages Server-Sent Events clients for real-time flag change
// streaming. Connected SDK clients receive flag change events as they occur.
type SSEBroker struct {
	mu      sync.RWMutex
	clients map[chan string]struct{}
}

// NewSSEBroker creates a new SSE broker for managing streaming connections.
func NewSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients: make(map[chan string]struct{}),
	}
}

// Subscribe registers a new client channel. The caller must call Unsubscribe
// when the client disconnects.
func (b *SSEBroker) Subscribe() chan string {
	ch := make(chan string, 64)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a client channel and closes it.
func (b *SSEBroker) Unsubscribe(ch chan string) {
	b.mu.Lock()
	delete(b.clients, ch)
	close(ch)
	b.mu.Unlock()
}

// Broadcast sends a message to all connected SSE clients. Slow clients with
// full buffers are skipped to prevent blocking the broadcaster.
func (b *SSEBroker) Broadcast(msg string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- msg:
		default:
			// Skip slow clients to avoid blocking.
		}
	}
}

// streamFlags handles the GET /api/v1/flags/stream SSE endpoint. It keeps the
// connection open and streams flag change events to connected SDK clients.
func (h *Handler) streamFlags(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	clientCh := h.sse.Subscribe()
	defer h.sse.Unsubscribe(clientCh)

	ctx := c.Request.Context()
	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-clientCh:
			if !ok {
				return false
			}
			c.SSEvent("flag_change", msg)
			return true
		case <-ctx.Done():
			return false
		}
	})
}
