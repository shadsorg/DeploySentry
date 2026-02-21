package notifications

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// WebhookEndpoint represents a registered webhook endpoint configuration.
type WebhookEndpoint struct {
	ID         uuid.UUID   `json:"id"`
	URL        string      `json:"url"`
	Secret     string      `json:"secret,omitempty"`
	EventTypes []EventType `json:"event_types,omitempty"`
	ProjectID  string      `json:"project_id"`
	Active     bool        `json:"active"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

// WebhookEndpointStore defines the interface for persisting webhook endpoints.
type WebhookEndpointStore interface {
	CreateEndpoint(ctx context.Context, endpoint *WebhookEndpoint) error
	GetEndpoint(ctx context.Context, id uuid.UUID) (*WebhookEndpoint, error)
	ListEndpoints(ctx context.Context, projectID string) ([]*WebhookEndpoint, error)
	UpdateEndpoint(ctx context.Context, endpoint *WebhookEndpoint) error
	DeleteEndpoint(ctx context.Context, id uuid.UUID) error
}

// InMemoryWebhookEndpointStore is a thread-safe in-memory implementation of
// WebhookEndpointStore, useful for testing and development.
type InMemoryWebhookEndpointStore struct {
	mu        sync.RWMutex
	endpoints map[uuid.UUID]*WebhookEndpoint
}

// NewInMemoryWebhookEndpointStore creates a new in-memory webhook endpoint store.
func NewInMemoryWebhookEndpointStore() *InMemoryWebhookEndpointStore {
	return &InMemoryWebhookEndpointStore{
		endpoints: make(map[uuid.UUID]*WebhookEndpoint),
	}
}

// CreateEndpoint stores a new webhook endpoint.
func (s *InMemoryWebhookEndpointStore) CreateEndpoint(_ context.Context, endpoint *WebhookEndpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endpoints[endpoint.ID] = endpoint
	return nil
}

// GetEndpoint retrieves a webhook endpoint by ID.
func (s *InMemoryWebhookEndpointStore) GetEndpoint(_ context.Context, id uuid.UUID) (*WebhookEndpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ep, ok := s.endpoints[id]
	if !ok {
		return nil, ErrWebhookEndpointNotFound
	}
	return ep, nil
}

// ListEndpoints returns all webhook endpoints for the given project.
func (s *InMemoryWebhookEndpointStore) ListEndpoints(_ context.Context, projectID string) ([]*WebhookEndpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*WebhookEndpoint
	for _, ep := range s.endpoints {
		if ep.ProjectID == projectID {
			result = append(result, ep)
		}
	}
	return result, nil
}

// UpdateEndpoint updates an existing webhook endpoint.
func (s *InMemoryWebhookEndpointStore) UpdateEndpoint(_ context.Context, endpoint *WebhookEndpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.endpoints[endpoint.ID]; !ok {
		return ErrWebhookEndpointNotFound
	}
	s.endpoints[endpoint.ID] = endpoint
	return nil
}

// DeleteEndpoint removes a webhook endpoint by ID.
func (s *InMemoryWebhookEndpointStore) DeleteEndpoint(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.endpoints[id]; !ok {
		return ErrWebhookEndpointNotFound
	}
	delete(s.endpoints, id)
	return nil
}

// webhookEndpointError is a sentinel error for webhook endpoint operations.
type webhookEndpointError string

func (e webhookEndpointError) Error() string { return string(e) }

// ErrWebhookEndpointNotFound is returned when a webhook endpoint cannot be found.
const ErrWebhookEndpointNotFound = webhookEndpointError("webhook endpoint not found")

// WebhookHandler provides HTTP endpoints for managing webhook endpoint configurations.
type WebhookHandler struct {
	store WebhookEndpointStore
}

// NewWebhookHandler creates a new webhook management HTTP handler.
func NewWebhookHandler(store WebhookEndpointStore) *WebhookHandler {
	return &WebhookHandler{store: store}
}

// RegisterRoutes mounts all webhook management API routes on the given router group.
func (h *WebhookHandler) RegisterRoutes(rg *gin.RouterGroup) {
	webhooks := rg.Group("/webhooks")
	{
		webhooks.POST("", h.createWebhook)
		webhooks.GET("", h.listWebhooks)
		webhooks.GET("/:id", h.getWebhook)
		webhooks.PUT("/:id", h.updateWebhook)
		webhooks.DELETE("/:id", h.deleteWebhook)
	}
}

// createWebhookRequest is the JSON body for creating a new webhook endpoint.
type createWebhookRequest struct {
	URL        string      `json:"url" binding:"required"`
	Secret     string      `json:"secret"`
	EventTypes []EventType `json:"event_types"`
	ProjectID  string      `json:"project_id" binding:"required"`
}

func (h *WebhookHandler) createWebhook(c *gin.Context) {
	var req createWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now().UTC()
	endpoint := &WebhookEndpoint{
		ID:         uuid.New(),
		URL:        req.URL,
		Secret:     req.Secret,
		EventTypes: req.EventTypes,
		ProjectID:  req.ProjectID,
		Active:     true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := h.store.CreateEndpoint(c.Request.Context(), endpoint); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create webhook endpoint"})
		return
	}

	c.JSON(http.StatusCreated, endpoint)
}

func (h *WebhookHandler) getWebhook(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook id"})
		return
	}

	endpoint, err := h.store.GetEndpoint(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook endpoint not found"})
		return
	}

	c.JSON(http.StatusOK, endpoint)
}

func (h *WebhookHandler) listWebhooks(c *gin.Context) {
	projectID := c.Query("project_id")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id query parameter is required"})
		return
	}

	endpoints, err := h.store.ListEndpoints(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list webhook endpoints"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"webhooks": endpoints})
}

// updateWebhookRequest is the JSON body for updating a webhook endpoint.
type updateWebhookRequest struct {
	URL        string      `json:"url"`
	Secret     string      `json:"secret"`
	EventTypes []EventType `json:"event_types"`
	Active     *bool       `json:"active"`
}

func (h *WebhookHandler) updateWebhook(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook id"})
		return
	}

	endpoint, err := h.store.GetEndpoint(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook endpoint not found"})
		return
	}

	var req updateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.URL != "" {
		endpoint.URL = req.URL
	}
	if req.Secret != "" {
		endpoint.Secret = req.Secret
	}
	if req.EventTypes != nil {
		endpoint.EventTypes = req.EventTypes
	}
	if req.Active != nil {
		endpoint.Active = *req.Active
	}
	endpoint.UpdatedAt = time.Now().UTC()

	if err := h.store.UpdateEndpoint(c.Request.Context(), endpoint); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update webhook endpoint"})
		return
	}

	c.JSON(http.StatusOK, endpoint)
}

func (h *WebhookHandler) deleteWebhook(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook id"})
		return
	}

	if err := h.store.DeleteEndpoint(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook endpoint not found"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
