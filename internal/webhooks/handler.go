package webhooks

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/platform/response"
)

// Handler provides HTTP handlers for webhook operations.
type Handler struct {
	service *Service
}

// NewHandler creates a new webhook handler.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// RegisterRoutes registers webhook routes with the router.
func (h *Handler) RegisterRoutes(router gin.IRouter) {
	webhooks := router.Group("/webhooks")
	{
		webhooks.POST("", h.CreateWebhook)
		webhooks.GET("", h.ListWebhooks)
		webhooks.GET("/:id", h.GetWebhook)
		webhooks.PUT("/:id", h.UpdateWebhook)
		webhooks.DELETE("/:id", h.DeleteWebhook)
		webhooks.POST("/:id/test", h.TestWebhook)
		webhooks.GET("/:id/deliveries", h.GetWebhookDeliveries)
	}
}

// CreateWebhook creates a new webhook endpoint.
func (h *Handler) CreateWebhook(c *gin.Context) {
	var req models.CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body", err)
		return
	}

	if err := ValidateWebhookURL(req.URL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get organization from context
	orgID, ok := auth.OrgIDFromContext(c)
	if !ok {
		response.Unauthorized(c, "Organization not found")
		return
	}

	// Get user ID from context
	var userID *uuid.UUID
	if userIDValue, exists := c.Get(auth.ContextKeyUserID); exists {
		if uid, ok := userIDValue.(uuid.UUID); ok {
			userID = &uid
		}
	}

	webhook, err := h.service.CreateWebhook(c.Request.Context(), orgID, req, userID)
	if err != nil {
		response.Error(c, "Failed to create webhook", err)
		return
	}

	response.JSON(c, http.StatusCreated, gin.H{
		"webhook": webhook,
	})
}

// ListWebhooks lists webhooks for an organization.
func (h *Handler) ListWebhooks(c *gin.Context) {
	// Get organization from context
	orgID, ok := auth.OrgIDFromContext(c)
	if !ok {
		response.Unauthorized(c, "Organization not found")
		return
	}

	// Parse query parameters
	opts := models.WebhookListOptions{}

	if projectIDStr := c.Query("project_id"); projectIDStr != "" {
		if projectID, err := uuid.Parse(projectIDStr); err == nil {
			opts.ProjectID = &projectID
		}
	}

	if isActiveStr := c.Query("is_active"); isActiveStr != "" {
		if isActive, err := strconv.ParseBool(isActiveStr); err == nil {
			opts.IsActive = &isActive
		}
	}

	if events := c.QueryArray("events"); len(events) > 0 {
		opts.Events = events
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			opts.Limit = limit
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			opts.Offset = offset
		}
	}

	webhooks, err := h.service.ListWebhooks(c.Request.Context(), orgID, opts)
	if err != nil {
		response.Error(c, "Failed to list webhooks", err)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{
		"webhooks": webhooks,
		"total":    len(webhooks),
	})
}

// GetWebhook retrieves a specific webhook.
func (h *Handler) GetWebhook(c *gin.Context) {
	webhookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid webhook ID", err)
		return
	}

	webhook, err := h.service.GetWebhook(c.Request.Context(), webhookID)
	if err != nil {
		if errors.Is(err, models.ErrWebhookNotFound) {
			response.NotFound(c, "Webhook not found")
			return
		}
		response.Error(c, "Failed to get webhook", err)
		return
	}

	// Check if user has access to this webhook
	orgID, ok := auth.OrgIDFromContext(c)
	if !ok || webhook.OrgID != orgID {
		response.NotFound(c, "Webhook not found")
		return
	}

	response.JSON(c, http.StatusOK, gin.H{
		"webhook": webhook,
	})
}

// UpdateWebhook updates a webhook endpoint.
func (h *Handler) UpdateWebhook(c *gin.Context) {
	webhookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid webhook ID", err)
		return
	}

	var req models.UpdateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body", err)
		return
	}

	if req.URL != nil && *req.URL != "" {
		if err := ValidateWebhookURL(*req.URL); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// Get user ID from context
	var userID *uuid.UUID
	if userIDValue, exists := c.Get(auth.ContextKeyUserID); exists {
		if uid, ok := userIDValue.(uuid.UUID); ok {
			userID = &uid
		}
	}

	webhook, err := h.service.UpdateWebhook(c.Request.Context(), webhookID, req, userID)
	if err != nil {
		if errors.Is(err, models.ErrWebhookNotFound) {
			response.NotFound(c, "Webhook not found")
			return
		}
		response.Error(c, "Failed to update webhook", err)
		return
	}

	// Check if user has access to this webhook
	orgID, ok := auth.OrgIDFromContext(c)
	if !ok || webhook.OrgID != orgID {
		response.NotFound(c, "Webhook not found")
		return
	}

	response.JSON(c, http.StatusOK, gin.H{
		"webhook": webhook,
	})
}

// DeleteWebhook deletes a webhook endpoint.
func (h *Handler) DeleteWebhook(c *gin.Context) {
	webhookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid webhook ID", err)
		return
	}

	// Get user ID from context
	var userID *uuid.UUID
	if userIDValue, exists := c.Get(auth.ContextKeyUserID); exists {
		if uid, ok := userIDValue.(uuid.UUID); ok {
			userID = &uid
		}
	}

	// First check if webhook exists and user has access
	webhook, err := h.service.GetWebhook(c.Request.Context(), webhookID)
	if err != nil {
		if errors.Is(err, models.ErrWebhookNotFound) {
			response.NotFound(c, "Webhook not found")
			return
		}
		response.Error(c, "Failed to get webhook", err)
		return
	}

	orgID, ok := auth.OrgIDFromContext(c)
	if !ok || webhook.OrgID != orgID {
		response.NotFound(c, "Webhook not found")
		return
	}

	if err := h.service.DeleteWebhook(c.Request.Context(), webhookID, userID); err != nil {
		response.Error(c, "Failed to delete webhook", err)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{
		"message": "Webhook deleted successfully",
	})
}

// TestWebhook sends a test payload to a webhook endpoint.
func (h *Handler) TestWebhook(c *gin.Context) {
	webhookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid webhook ID", err)
		return
	}

	var req models.WebhookTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body", err)
		return
	}

	// Validate event type
	if !models.IsValidEvent(req.EventType) {
		response.BadRequest(c, "Invalid event type", nil)
		return
	}

	// First check if webhook exists and user has access
	webhook, err := h.service.GetWebhook(c.Request.Context(), webhookID)
	if err != nil {
		if errors.Is(err, models.ErrWebhookNotFound) {
			response.NotFound(c, "Webhook not found")
			return
		}
		response.Error(c, "Failed to get webhook", err)
		return
	}

	orgID, ok := auth.OrgIDFromContext(c)
	if !ok || webhook.OrgID != orgID {
		response.NotFound(c, "Webhook not found")
		return
	}

	delivery, err := h.service.TestWebhook(c.Request.Context(), webhookID, req)
	if err != nil {
		// Even if delivery fails, we return the delivery record
		if delivery != nil {
			response.JSON(c, http.StatusOK, gin.H{
				"delivery": delivery,
				"success":  false,
				"error":    err.Error(),
			})
			return
		}
		response.Error(c, "Failed to test webhook", err)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{
		"delivery": delivery,
		"success":  true,
	})
}

// GetWebhookDeliveries retrieves delivery history for a webhook.
func (h *Handler) GetWebhookDeliveries(c *gin.Context) {
	webhookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid webhook ID", err)
		return
	}

	// First check if webhook exists and user has access
	webhook, err := h.service.GetWebhook(c.Request.Context(), webhookID)
	if err != nil {
		if errors.Is(err, models.ErrWebhookNotFound) {
			response.NotFound(c, "Webhook not found")
			return
		}
		response.Error(c, "Failed to get webhook", err)
		return
	}

	orgID, ok := auth.OrgIDFromContext(c)
	if !ok || webhook.OrgID != orgID {
		response.NotFound(c, "Webhook not found")
		return
	}

	// Parse query parameters
	opts := models.WebhookDeliveryListOptions{}

	if statusStr := c.Query("status"); statusStr != "" {
		status := models.DeliveryStatus(statusStr)
		opts.Status = &status
	}

	if eventType := c.Query("event_type"); eventType != "" {
		opts.EventType = &eventType
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			opts.Limit = limit
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			opts.Offset = offset
		}
	}

	deliveries, err := h.service.GetDeliveries(c.Request.Context(), webhookID, opts)
	if err != nil {
		response.Error(c, "Failed to get webhook deliveries", err)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{
		"deliveries": deliveries,
		"total":      len(deliveries),
	})
}

// GetWebhookEvents returns the list of available webhook events.
func (h *Handler) GetWebhookEvents(c *gin.Context) {
	events := make([]string, len(models.AllWebhookEvents()))
	for i, event := range models.AllWebhookEvents() {
		events[i] = string(event)
	}

	response.JSON(c, http.StatusOK, gin.H{
		"events": events,
	})
}