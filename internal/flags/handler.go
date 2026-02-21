package flags

import (
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for managing feature flags.
type Handler struct {
	service FlagService
	rbac    *auth.RBACChecker
	sse     *SSEBroker
}

// NewHandler creates a new feature flag HTTP handler.
func NewHandler(service FlagService, rbac *auth.RBACChecker) *Handler {
	return &Handler{
		service: service,
		rbac:    rbac,
		sse:     NewSSEBroker(),
	}
}

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
			rules.POST("", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.addRule)
			rules.PUT("/:ruleId", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.updateRule)
			rules.DELETE("/:ruleId", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.deleteRule)
		}
	}
}

// createFlagRequest is the JSON body for creating a new feature flag.
type createFlagRequest struct {
	ProjectID     uuid.UUID `json:"project_id" binding:"required"`
	EnvironmentID uuid.UUID `json:"environment_id" binding:"required"`
	Key           string    `json:"key" binding:"required"`
	Name          string    `json:"name" binding:"required"`
	Description   string    `json:"description"`
	FlagType      string    `json:"flag_type" binding:"required"`
	DefaultValue  string    `json:"default_value"`
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

	flag := &models.FeatureFlag{
		ProjectID:     req.ProjectID,
		EnvironmentID: req.EnvironmentID,
		Key:           req.Key,
		Name:          req.Name,
		Description:   req.Description,
		FlagType:      models.FlagType(req.FlagType),
		DefaultValue:  req.DefaultValue,
		Enabled:       false,
		CreatedBy:     createdBy,
	}

	if err := h.service.CreateFlag(c.Request.Context(), flag); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
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

	c.JSON(http.StatusOK, gin.H{"flags": flags})
}

// updateFlagRequest is the JSON body for updating a feature flag.
type updateFlagRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	DefaultValue string `json:"default_value"`
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
	if req.DefaultValue != "" {
		flag.DefaultValue = req.DefaultValue
	}

	if err := h.service.UpdateFlag(c.Request.Context(), flag); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, flag)
}

func (h *Handler) archiveFlag(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
		return
	}

	if err := h.service.ArchiveFlag(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
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

	if err := h.service.ToggleFlag(c.Request.Context(), id, req.Enabled); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	// Broadcast toggle event to SSE clients.
	h.sse.Broadcast(fmt.Sprintf(`{"event":"flag.toggled","flag_id":"%s","enabled":%t}`, id, req.Enabled))

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

	result, err := h.service.Evaluate(c.Request.Context(), req.ProjectID, req.EnvironmentID, req.FlagKey, req.Context)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
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

	results, err := h.service.BatchEvaluate(c.Request.Context(), req.ProjectID, req.EnvironmentID, req.FlagKeys, req.Context)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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
