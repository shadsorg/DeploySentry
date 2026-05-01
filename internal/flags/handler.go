package flags

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/shadsorg/deploysentry/internal/analytics"
	"github.com/shadsorg/deploysentry/internal/auth"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/webhooks"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// RetentionWindow is the period between archive and eligibility for hard-delete.
// Configurable retention is a follow-up; the constant is fine for v1.
const RetentionWindow = 30 * 24 * time.Hour

// EntityResolver resolves slugs to entity models for SDK-facing endpoints.
type EntityResolver interface {
	GetAppBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error)
	GetProjectBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error)
	GetUserName(ctx context.Context, id uuid.UUID) (string, error)
}

// EnvironmentSlugResolver resolves an environment slug to its UUID.
type EnvironmentSlugResolver interface {
	ResolveEnvironmentSlug(ctx context.Context, orgID uuid.UUID, slug string) (uuid.UUID, error)
}

// RolloutAttacher is implemented by the rollout package and injected at wiring time.
// A nil attacher means rollouts are not enabled; rule edits apply immediately.
type RolloutAttacher interface {
	AttachFromRuleRequest(ctx context.Context, rule *models.TargetingRule, previousPercentage int, req *RolloutAttachRequest, actor uuid.UUID) error
}

// RolloutAttachRequest is the client-supplied rollout intent on a rule edit.
type RolloutAttachRequest struct {
	StrategyName     string          `json:"strategy_name,omitempty"`
	StrategyID       *uuid.UUID      `json:"strategy_id,omitempty"`
	Overrides        json.RawMessage `json:"overrides,omitempty"`
	ReleaseID        *uuid.UUID      `json:"release_id,omitempty"`
	ApplyImmediately bool            `json:"apply_immediately,omitempty"`
}

// ErrRolloutInProgress is returned by the attacher when the target rule already
// has an active rollout (client should abort/wait/amend).
var ErrRolloutInProgress = errors.New("rollout in progress on this rule")

// AuditWriter persists audit log entries.
type AuditWriter interface {
	WriteAuditLog(ctx context.Context, entry *models.AuditLogEntry) error
}

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
	CreatedByName string               `json:"created_by_name,omitempty"`
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
	entityRepo   EntityResolver
	envResolver  EnvironmentSlugResolver
	auditWriter  AuditWriter
	rollouts     RolloutAttacher
}

// NewHandler creates a new feature flag HTTP handler.
func NewHandler(service FlagService, rbac *auth.RBACChecker, webhookSvc *webhooks.Service, analyticsSvc *analytics.Service, entityRepo EntityResolver, envResolver EnvironmentSlugResolver, auditWriter AuditWriter) *Handler {
	return &Handler{
		service:      service,
		rbac:         rbac,
		sse:          NewSSEBroker(),
		webhookSvc:   webhookSvc,
		analyticsSvc: analyticsSvc,
		entityRepo:   entityRepo,
		envResolver:  envResolver,
		auditWriter:  auditWriter,
	}
}

// NewHandlerWithRollouts creates a Handler with an optional rollout attacher.
// All other behaviour is identical to NewHandler.
func NewHandlerWithRollouts(service FlagService, rbac *auth.RBACChecker, webhookSvc *webhooks.Service, analyticsSvc *analytics.Service, entityRepo EntityResolver, envResolver EnvironmentSlugResolver, auditWriter AuditWriter, rollouts RolloutAttacher) *Handler {
	h := NewHandler(service, rbac, webhookSvc, analyticsSvc, entityRepo, envResolver, auditWriter)
	h.rollouts = rollouts
	return h
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
		flags.GET("/export", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.exportFlags)
		flags.GET("/:id", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.getFlag)
		flags.PUT("/:id", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.updateFlag)
		flags.POST("/:id/archive", auth.RequirePermission(h.rbac, auth.PermFlagArchive), h.archiveFlag)
		flags.POST("/:id/queue-deletion", auth.RequirePermission(h.rbac, auth.PermFlagArchive), h.queueFlagDeletion)
		flags.DELETE("/:id/queue-deletion", auth.RequirePermission(h.rbac, auth.PermFlagArchive), h.cancelFlagDeletion)
		flags.DELETE("/:id", auth.RequirePermission(h.rbac, auth.PermFlagArchive), h.hardDeleteFlag)
		flags.POST("/:id/restore", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.restoreFlag)
		flags.POST("/:id/toggle", auth.RequirePermission(h.rbac, auth.PermFlagToggle), h.toggleFlag)

		rules := flags.Group("/:id/rules")
		{
			rules.GET("", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.listRules)
			rules.GET("/environment-states", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.listRuleEnvStates)
			rules.POST("", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.addRule)
			rules.PUT("/:ruleId", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.updateRule)
			rules.PUT("/:ruleId/environments/:envId", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.setRuleEnvState)
			rules.DELETE("/:ruleId", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.deleteRule)
		}

		envs := flags.Group("/:id/environments")
		{
			envs.GET("", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.listFlagEnvStates)
			envs.PUT("/:envId", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.setFlagEnvState)
		}
	}

	// Lifecycle layer — ApiKey-authenticated. The `:id` path param accepts
	// either a flag UUID or a flag key; keys are resolved against the API
	// key's project scope (lookup in handler). Reuses the ":id" param name
	// because Gin's router requires all siblings under `/flags/:*` to share
	// the same parameter name.
	flags.POST("/:id/smoke-test-result", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.recordSmokeTestResult)
	flags.POST("/:id/user-test-result", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.recordUserTestResult)
	flags.POST("/:id/schedule-removal", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.scheduleRemoval)
	flags.DELETE("/:id/schedule-removal", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.cancelScheduledRemoval)
	flags.POST("/:id/mark-exhausted", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.markIterationExhausted)
}

// createFlagRequest is the JSON body for creating a new feature flag.
type createFlagRequest struct {
	ProjectID     uuid.UUID  `json:"project_id" binding:"required"`
	EnvironmentID *uuid.UUID `json:"environment_id"`
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
		EnvironmentID: req.EnvironmentID,	// nil-safe: model field is *uuid.UUID
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

	newVal, _ := json.Marshal(map[string]interface{}{
		"key": flag.Key, "name": flag.Name, "flag_type": flag.FlagType,
		"category": flag.Category, "default_value": flag.DefaultValue,
	})
	h.writeAudit(c, "flag.created", "flag", flag.ID, "", string(newVal))

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

	// Overlay per-environment state when environment_id is provided.
	if envIDStr := c.Query("environment_id"); envIDStr != "" {
		envID, envErr := uuid.Parse(envIDStr)
		if envErr != nil && h.envResolver != nil {
			orgIDStr := c.GetString("org_id")
			if orgID, oErr := uuid.Parse(orgIDStr); oErr == nil {
				envID, envErr = h.envResolver.ResolveEnvironmentSlug(c.Request.Context(), orgID, envIDStr)
			}
		}
		if envErr == nil {
			states, sErr := h.service.ListFlagEnvStates(c.Request.Context(), flag.ID)
			if sErr == nil {
				for _, s := range states {
					if s.EnvironmentID == envID {
						flag.Enabled = s.Enabled
						if s.Value != nil {
							flag.DefaultValue = unquoteJSON(*s.Value)
						}
						break
					}
				}
			}
		}
	}

	createdByName := ""
	if flag.CreatedBy != uuid.Nil {
		if name, err := h.entityRepo.GetUserName(c.Request.Context(), flag.CreatedBy); err == nil {
			createdByName = name
		}
	}

	if h.ratingSvc != nil {
		resp := &flagWithRatings{FeatureFlag: flag}
		resp.CreatedByName = createdByName

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

	c.JSON(http.StatusOK, &flagWithRatings{FeatureFlag: flag, CreatedByName: createdByName})
}

func (h *Handler) listFlags(c *gin.Context) {
	projectIDStr := c.Query("project_id")
	if projectIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id query parameter is required"})
		return
	}

	projectID, err := h.resolveProjectID(c, projectIDStr)
	if err != nil {
		return // response already written
	}

	opts := ListOptions{Limit: 20, Offset: 0}

	if appSlug := c.Query("application"); appSlug != "" {
		app, err := h.entityRepo.GetAppBySlug(c.Request.Context(), projectID, appSlug)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve application"})
			return
		}
		if app == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
			return
		}
		opts.ApplicationID = &app.ID
	}

	flags, err := h.service.ListFlags(c.Request.Context(), projectID, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list flags"})
		return
	}

	// When environment_id is provided, overlay per-environment enabled state
	// onto each flag so SDKs see the environment-specific toggle.
	if envIDStr := c.Query("environment_id"); envIDStr != "" {
		envID, envErr := uuid.Parse(envIDStr)
		if envErr != nil && h.envResolver != nil {
			// Treat as a slug and resolve to UUID.
			orgIDStr := c.GetString("org_id")
			if orgID, oErr := uuid.Parse(orgIDStr); oErr == nil {
				envID, envErr = h.envResolver.ResolveEnvironmentSlug(c.Request.Context(), orgID, envIDStr)
			}
		}
		if envErr == nil {
			for _, f := range flags {
				states, sErr := h.service.ListFlagEnvStates(c.Request.Context(), f.ID)
				if sErr != nil {
					continue
				}
				for _, s := range states {
					if s.EnvironmentID == envID {
						f.Enabled = s.Enabled
						if s.Value != nil {
							f.DefaultValue = unquoteJSON(*s.Value)
						}
						break
					}
				}
			}
		}
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

	oldVal, _ := json.Marshal(map[string]interface{}{
		"name": flag.Name, "description": flag.Description, "category": flag.Category,
		"purpose": flag.Purpose, "owners": flag.Owners, "is_permanent": flag.IsPermanent,
		"expires_at": flag.ExpiresAt, "default_value": flag.DefaultValue, "tags": flag.Tags,
	})

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

	newVal, _ := json.Marshal(map[string]interface{}{
		"name": flag.Name, "description": flag.Description, "category": flag.Category,
		"purpose": flag.Purpose, "owners": flag.Owners, "is_permanent": flag.IsPermanent,
		"expires_at": flag.ExpiresAt, "default_value": flag.DefaultValue, "tags": flag.Tags,
	})
	h.writeAudit(c, "flag.updated", "flag", flag.ID, string(oldVal), string(newVal))

	h.broadcastEvent("flag.updated", id, "")

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

	h.writeAudit(c, "flag.archived", "flag", id, "", "")

	h.broadcastEvent("flag.archived", id, "")

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

func (h *Handler) queueFlagDeletion(c *gin.Context) {
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
	if !flag.Archived {
		c.JSON(http.StatusConflict, gin.H{"error": "flag must be archived first"})
		return
	}

	if err := h.service.QueueDeletion(c.Request.Context(), id, RetentionWindow); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	flag, _ = h.service.GetFlag(c.Request.Context(), id)
	deleteAfter := ""
	if flag != nil && flag.DeleteAfter != nil {
		deleteAfter = flag.DeleteAfter.Format(time.RFC3339)
	}
	newVal, _ := json.Marshal(map[string]string{"delete_after": deleteAfter})
	h.writeAudit(c, "flag.queued_for_deletion", "flag", id, "", string(newVal))
	h.broadcastEvent("flag.queued_for_deletion", id, "")

	c.JSON(http.StatusOK, flag)
}

func (h *Handler) cancelFlagDeletion(c *gin.Context) {
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
	priorDeleteAfter := ""
	if flag.DeleteAfter != nil {
		priorDeleteAfter = flag.DeleteAfter.Format(time.RFC3339)
	}
	if err := h.service.ClearDeleteAfter(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	oldVal, _ := json.Marshal(map[string]string{"delete_after": priorDeleteAfter})
	h.writeAudit(c, "flag.queued_for_deletion.cancelled", "flag", id, string(oldVal), "")
	h.broadcastEvent("flag.queued_for_deletion.cancelled", id, "")

	flag, _ = h.service.GetFlag(c.Request.Context(), id)
	c.JSON(http.StatusOK, flag)
}

func (h *Handler) hardDeleteFlag(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
		return
	}
	if c.Query("force") != "true" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hard delete requires ?force=true"})
		return
	}

	flag, err := h.service.GetFlag(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "flag not found"})
		return
	}

	if c.GetHeader("X-Confirm-Slug") != flag.Key {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Confirm-Slug header must match flag key"})
		return
	}
	if !flag.Archived {
		c.JSON(http.StatusConflict, gin.H{"error": "flag must be archived first"})
		return
	}

	// Capture flag state before deletion — the row is about to be gone, so
	// audit_log.old_value is the only place this content survives.
	oldValBytes, _ := json.Marshal(flag)

	if err := h.service.HardDeleteFlag(c.Request.Context(), id, RetentionWindow); err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "retention") {
			eligibleAt := ""
			if flag.ArchivedAt != nil {
				eligibleAt = flag.ArchivedAt.Add(RetentionWindow).Format(time.RFC3339)
			}
			c.JSON(http.StatusConflict, gin.H{
				"error":       "retention not elapsed",
				"code":        "retention_not_elapsed",
				"eligible_at": eligibleAt,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.writeAudit(c, "flag.hard_deleted", "flag", id, string(oldValBytes), "")
	h.broadcastEvent("flag.hard_deleted", id, "")
	c.JSON(http.StatusOK, gin.H{"deleted": true, "id": id})
}

func (h *Handler) restoreFlag(c *gin.Context) {
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
	priorDeleteAfter := ""
	if flag.DeleteAfter != nil {
		priorDeleteAfter = flag.DeleteAfter.Format(time.RFC3339)
	}
	priorArchivedAt := ""
	if flag.ArchivedAt != nil {
		priorArchivedAt = flag.ArchivedAt.Format(time.RFC3339)
	}
	if err := h.service.RestoreFlag(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	oldVal, _ := json.Marshal(map[string]string{
		"archived_at":  priorArchivedAt,
		"delete_after": priorDeleteAfter,
	})
	h.writeAudit(c, "flag.restored", "flag", id, string(oldVal), "")
	h.broadcastEvent("flag.restored", id, "")

	flag, _ = h.service.GetFlag(c.Request.Context(), id)
	c.JSON(http.StatusOK, flag)
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

	h.writeAudit(c, "flag.toggled", "flag", id,
		fmt.Sprintf(`{"enabled":%v}`, !req.Enabled),
		fmt.Sprintf(`{"enabled":%v}`, req.Enabled))

	// Broadcast toggle event to SSE clients.
	h.broadcastEvent("flag.toggled", id, "")

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
			log.Printf("failed to publish flag toggled webhook: %v", err)
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

	for _, flagID := range req.FlagIDs {
		h.broadcastEvent("flag.bulk_toggled", flagID, "")
	}

	c.JSON(http.StatusOK, gin.H{"toggled": len(req.FlagIDs), "enabled": req.Enabled})
}

// resolveProjectID parses value as a UUID; if that fails it treats it as a
// slug and looks up the project via the org_id set by auth middleware.
func (h *Handler) resolveProjectID(c *gin.Context, value string) (uuid.UUID, error) {
	if id, err := uuid.Parse(value); err == nil {
		return id, nil
	}
	orgIDStr := c.GetString("org_id")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot resolve project slug without org context"})
		return uuid.Nil, err
	}
	project, err := h.entityRepo.GetProjectBySlug(c.Request.Context(), orgID, value)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve project"})
		return uuid.Nil, err
	}
	if project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return uuid.Nil, fmt.Errorf("project not found: %s", value)
	}
	return project.ID, nil
}

// resolveEnvironmentID parses value as a UUID; if that fails it treats it as a
// slug and looks up the org-level environment via the org_id set by auth middleware.
func (h *Handler) resolveEnvironmentID(c *gin.Context, value string) (uuid.UUID, error) {
	if id, err := uuid.Parse(value); err == nil {
		return id, nil
	}
	orgIDStr := c.GetString("org_id")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot resolve environment slug without org context"})
		return uuid.Nil, err
	}
	envID, err := h.envResolver.ResolveEnvironmentSlug(c.Request.Context(), orgID, value)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return uuid.Nil, err
	}
	return envID, nil
}

// evaluateRequest is the JSON body for evaluating a feature flag.
// ProjectID and EnvironmentID accept either a UUID or a slug string.
type evaluateRequest struct {
	ProjectID     string                   `json:"project_id" binding:"required"`
	EnvironmentID string                   `json:"environment_id" binding:"required"`
	FlagKey       string                   `json:"flag_key" binding:"required"`
	Application   string                   `json:"application"`
	Context       models.EvaluationContext `json:"context"`
}

func (h *Handler) evaluate(c *gin.Context) {
	var req evaluateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Resolve project: UUID or slug.
	projectID, err := h.resolveProjectID(c, req.ProjectID)
	if err != nil {
		return // response already written
	}

	// Resolve environment: UUID or slug.
	environmentID, err := h.resolveEnvironmentID(c, req.EnvironmentID)
	if err != nil {
		return // response already written
	}

	if req.Application != "" {
		app, err := h.entityRepo.GetAppBySlug(c.Request.Context(), projectID, req.Application)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve application"})
			return
		}
		if app == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
			return
		}
	}

	startTime := time.Now()
	result, err := h.service.Evaluate(c.Request.Context(), projectID, environmentID, req.FlagKey, req.Context)
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
				ProjectID:     projectID,
				EnvironmentID: environmentID,
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
					log.Printf("failed to record flag evaluation: %v", recordErr)
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
			ProjectID:     projectID,
			EnvironmentID: environmentID,
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
				log.Printf("failed to record flag evaluation: %v", recordErr)
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
	Application   string                   `json:"application"`
	Context       models.EvaluationContext `json:"context"`
}

func (h *Handler) batchEvaluate(c *gin.Context) {
	var req batchEvaluateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Application != "" {
		app, err := h.entityRepo.GetAppBySlug(c.Request.Context(), req.ProjectID, req.Application)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve application"})
			return
		}
		if app == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
			return
		}
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
					log.Printf("failed to record flag evaluation: %v", recordErr)
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
	Rollout      *RolloutAttachRequest `json:"rollout,omitempty"`
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

	newVal, _ := json.Marshal(map[string]interface{}{
		"rule_id": rule.ID, "attribute": rule.Attribute, "operator": rule.Operator,
		"target_values": rule.TargetValues, "value": rule.Value, "priority": rule.Priority,
	})
	h.writeAudit(c, "flag.rule.created", "flag", flagID, "", string(newVal))

	h.broadcastEvent("rule.created", rule.FlagID, "")

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

	// If a rollout attacher is configured and the request includes rollout intent
	// without apply_immediately, delegate to the rollout engine instead of
	// applying the rule change directly. The rollout engine will call UpdateRule
	// when it is ready to apply the new percentage.
	if h.rollouts != nil && req.Rollout != nil && !req.Rollout.ApplyImmediately {
		// Read the current rule from DB so we can capture the real previousPercentage.
		prevPct := 0
		if existing, err := h.service.GetRule(c.Request.Context(), ruleID); err == nil && existing != nil && existing.Percentage != nil {
			prevPct = *existing.Percentage
		}
		actor := actorFromFlagContext(c)
		if err := h.rollouts.AttachFromRuleRequest(c.Request.Context(), rule, prevPct, req.Rollout, actor); err != nil {
			if errors.Is(err, ErrRolloutInProgress) {
				c.JSON(http.StatusConflict, gin.H{
					"error":   "rollout_in_progress",
					"options": []string{"abort", "wait", "amend"},
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, rule)
		return
	}

	if err := h.service.UpdateRule(c.Request.Context(), rule); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	h.broadcastEvent("rule.updated", rule.FlagID, "")

	c.JSON(http.StatusOK, rule)
}

func (h *Handler) deleteRule(c *gin.Context) {
	ruleID, err := uuid.Parse(c.Param("ruleId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule id"})
		return
	}

	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
		return
	}

	// Capture the rule state before deletion so the audit entry can be reverted.
	oldRuleVal := fmt.Sprintf(`{"rule_id":"%s"}`, ruleID)
	if existingRule, rErr := h.service.GetRule(c.Request.Context(), ruleID); rErr == nil {
		if b, mErr := json.Marshal(existingRule); mErr == nil {
			oldRuleVal = string(b)
		}
	}

	if err := h.service.DeleteRule(c.Request.Context(), ruleID); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	h.writeAudit(c, "flag.rule.deleted", "flag", flagID, oldRuleVal, "")

	h.broadcastEvent("rule.deleted", flagID, "")

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
// RuleEnvironmentState handlers
// ---------------------------------------------------------------------------

type setRuleEnvStateRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *Handler) setRuleEnvState(c *gin.Context) {
	ruleID, err := uuid.Parse(c.Param("ruleId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule id"})
		return
	}
	envID, err := uuid.Parse(c.Param("envId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment id"})
		return
	}
	var req setRuleEnvStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	flagID, _ := uuid.Parse(c.Param("id"))

	// Capture prior state so the audit entry can be reverted.
	oldRuleEnvVal := ""
	if priorStates, lErr := h.service.ListRuleEnvironmentStates(c.Request.Context(), flagID); lErr == nil {
		for _, s := range priorStates {
			if s.RuleID == ruleID && s.EnvironmentID == envID {
				if b, mErr := json.Marshal(map[string]interface{}{
					"rule_id": ruleID, "environment_id": envID, "enabled": s.Enabled,
				}); mErr == nil {
					oldRuleEnvVal = string(b)
				}
				break
			}
		}
	}

	state, err := h.service.SetRuleEnvironmentState(c.Request.Context(), ruleID, envID, req.Enabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	newVal, _ := json.Marshal(map[string]interface{}{
		"rule_id": ruleID, "environment_id": envID, "enabled": req.Enabled,
	})
	h.writeAudit(c, "flag.rule.env_state.updated", "flag", flagID, oldRuleEnvVal, string(newVal))

	h.broadcastEvent("rule.updated", flagID, "")

	c.JSON(http.StatusOK, state)
}

func (h *Handler) listRuleEnvStates(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
		return
	}
	states, err := h.service.ListRuleEnvironmentStates(c.Request.Context(), flagID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list rule environment states"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rule_environment_states": states})
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

	// Capture prior state so the audit entry can be reverted.
	oldFlagEnvVal := ""
	if priorStates, lErr := h.service.ListFlagEnvStates(c.Request.Context(), flagID); lErr == nil {
		for _, s := range priorStates {
			if s.EnvironmentID == envID {
				if b, mErr := json.Marshal(map[string]interface{}{
					"environment_id": envID, "enabled": s.Enabled, "value": s.Value,
				}); mErr == nil {
					oldFlagEnvVal = string(b)
				}
				break
			}
		}
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

	newVal, _ := json.Marshal(map[string]interface{}{
		"environment_id": envID, "enabled": req.Enabled, "value": req.Value,
	})
	h.writeAudit(c, "flag.env_state.updated", "flag", flagID, oldFlagEnvVal, string(newVal))

	h.broadcastEvent("flag.updated", flagID, "")

	c.JSON(http.StatusOK, state)
}

// ---------------------------------------------------------------------------
// Export handler
// ---------------------------------------------------------------------------

func (h *Handler) exportFlags(c *gin.Context) {
	projectIDStr := c.Query("project_id")
	if projectIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id is required"})
		return
	}
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
		return
	}

	appSlug := c.Query("application")
	format := c.DefaultQuery("format", "yaml")

	export, err := h.service.ExportFlags(c.Request.Context(), projectID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export flags"})
		return
	}
	export.Application = appSlug

	if format == "json" {
		c.JSON(http.StatusOK, export)
		return
	}

	yamlBytes, err := yaml.Marshal(export)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize yaml"})
		return
	}
	c.Data(http.StatusOK, "application/x-yaml", yamlBytes)
}

// SSEEvent is the structured payload broadcast to connected SSE clients when a
// flag or rule mutation occurs.
type SSEEvent struct {
	Event     string    `json:"event"`
	FlagID    string    `json:"flag_id"`
	FlagKey   string    `json:"flag_key,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// unquoteJSON strips JSON string encoding from a raw value. If the value is
// a JSON string like `"true"`, it returns `true`. For non-string JSON (numbers,
// booleans, objects) it returns the raw bytes as a string.
func unquoteJSON(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

// broadcastEvent serialises an SSEEvent and sends it to all connected clients.
func (h *Handler) broadcastEvent(event string, flagID uuid.UUID, flagKey string) {
	data, _ := json.Marshal(SSEEvent{
		Event:     event,
		FlagID:    flagID.String(),
		FlagKey:   flagKey,
		Timestamp: time.Now(),
	})
	h.sse.Broadcast(string(data))
}

// writeAudit records an audit log entry. Failures are logged but do not fail the request.
func (h *Handler) writeAudit(c *gin.Context, action, entityType string, entityID uuid.UUID, oldValue, newValue string) {
	if h.auditWriter == nil {
		return
	}
	var actorID uuid.UUID
	if uid, exists := c.Get("user_id"); exists {
		actorID, _ = uid.(uuid.UUID)
	}
	var orgID uuid.UUID
	if oid := c.GetString("org_id"); oid != "" {
		orgID, _ = uuid.Parse(oid)
	}

	entry := &models.AuditLogEntry{
		OrgID:      orgID,
		ActorID:    actorID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		OldValue:   oldValue,
		NewValue:   newValue,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		CreatedAt:  time.Now(),
	}
	if err := h.auditWriter.WriteAuditLog(c.Request.Context(), entry); err != nil {
		log.Printf("failed to write audit log: %v", err)
	}
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
	log.Printf("SSE broadcast to %d clients: %s", len(b.clients), msg)
	for ch := range b.clients {
		select {
		case ch <- msg:
		default:
			log.Printf("SSE: dropped message for slow client")
		}
	}
}

// streamFlags handles the GET /api/v1/flags/stream SSE endpoint. It keeps the
// connection open and streams flag change events to connected SDK clients.
func (h *Handler) streamFlags(c *gin.Context) {
	// Read application filter (optional). SDK always sends this.
	// Full per-client filtering is deferred — the client-side cache
	// already filters to the correct set via the initial listFlags call.
	_ = c.Query("application")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Disable the server's WriteTimeout for this connection. SSE streams
	// are long-lived — the default 15s WriteTimeout kills them.
	rc := http.NewResponseController(c.Writer)
	_ = rc.SetWriteDeadline(time.Time{})
	// Flush headers and an immediate heartbeat so proxies (Cloudflare,
	// Nginx) see a valid response right away and start streaming instead
	// of buffering. Without this, the client may wait up to 20s for the
	// first byte, which triggers 502 on aggressive proxies.
	c.Writer.WriteHeaderNow()
	_, _ = fmt.Fprintf(c.Writer, ": heartbeat\n\n")
	c.Writer.Flush()

	clientCh := h.sse.Subscribe()
	defer h.sse.Unsubscribe(clientCh)

	ctx := c.Request.Context()
	// Cloudflare terminates idle HTTP/2 streams after ~15s on some plans.
	// Send heartbeats every 10s to stay well within the window.
	heartbeat := time.NewTicker(10 * time.Second)
	defer heartbeat.Stop()

	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-clientCh:
			if !ok {
				return false
			}
			c.SSEvent("flag_change", msg)
			c.Writer.Flush()
			return true
		case <-heartbeat.C:
			// SSE comment to keep the connection alive through proxies.
			_, _ = fmt.Fprintf(w, ": ping\n\n")
			c.Writer.Flush()
			return true
		case <-ctx.Done():
			return false
		}
	})
}

// actorFromFlagContext extracts the authenticated user's UUID from the Gin
// context. Returns uuid.Nil when no user_id has been set (e.g. API-key flows).
func actorFromFlagContext(c *gin.Context) uuid.UUID {
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(uuid.UUID); ok {
			return id
		}
	}
	return uuid.Nil
}
