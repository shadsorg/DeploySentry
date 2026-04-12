package webhooks

import (
	"bytes"
	"context"
	"log"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/platform/messaging"
)

// Service provides webhook management and delivery functionality.
type Service struct {
	repo      Repository
	publisher messaging.Publisher
	client    *http.Client
}

// Repository defines the interface for webhook data access.
type Repository interface {
	// Webhook management
	CreateWebhook(ctx context.Context, webhook *models.Webhook) error
	GetWebhook(ctx context.Context, id uuid.UUID) (*models.Webhook, error)
	GetWebhooksByOrg(ctx context.Context, orgID uuid.UUID, opts models.WebhookListOptions) ([]*models.Webhook, error)
	UpdateWebhook(ctx context.Context, webhook *models.Webhook) error
	DeleteWebhook(ctx context.Context, id uuid.UUID) error

	// Delivery tracking
	CreateDelivery(ctx context.Context, delivery *models.WebhookDelivery) error
	GetDelivery(ctx context.Context, id uuid.UUID) (*models.WebhookDelivery, error)
	GetDeliveriesByWebhook(ctx context.Context, webhookID uuid.UUID, opts models.WebhookDeliveryListOptions) ([]*models.WebhookDelivery, error)
	UpdateDelivery(ctx context.Context, delivery *models.WebhookDelivery) error
	GetPendingRetries(ctx context.Context, limit int) ([]*models.WebhookDelivery, error)
}

// NewService creates a new webhook service.
func NewService(repo Repository, publisher messaging.Publisher) *Service {
	return &Service{
		repo:      repo,
		publisher: publisher,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateWebhook creates a new webhook endpoint.
func (s *Service) CreateWebhook(ctx context.Context, orgID uuid.UUID, req models.CreateWebhookRequest, userID *uuid.UUID) (*models.Webhook, error) {
	// Generate a secure secret for the webhook
	secret, err := s.generateSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to generate webhook secret: %w", err)
	}

	// Set default values
	retryAttempts := 3
	if req.RetryAttempts != nil {
		retryAttempts = *req.RetryAttempts
	}

	timeoutSeconds := 10
	if req.TimeoutSeconds != nil {
		timeoutSeconds = *req.TimeoutSeconds
	}

	webhook := &models.Webhook{
		ID:             uuid.New(),
		OrgID:          orgID,
		ProjectID:      req.ProjectID,
		Name:           req.Name,
		URL:            req.URL,
		Secret:         secret,
		Events:         req.Events,
		IsActive:       true,
		RetryAttempts:  retryAttempts,
		TimeoutSeconds: timeoutSeconds,
		CreatedBy:      userID,
		UpdatedBy:      userID,
	}

	if err := s.repo.CreateWebhook(ctx, webhook); err != nil {
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}

	// Publish webhook created event
	if err := s.publishEvent(ctx, models.EventAuditLog, orgID, req.ProjectID, map[string]interface{}{
		"action":     "webhook.created",
		"webhook_id": webhook.ID,
		"name":       webhook.Name,
		"url":        webhook.URL,
		"events":     webhook.Events,
	}, userID); err != nil {
		log.Printf("failed to publish webhook created event: %v", err)
	}

	return webhook, nil
}

// GetWebhook retrieves a webhook by ID.
func (s *Service) GetWebhook(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
	return s.repo.GetWebhook(ctx, id)
}

// ListWebhooks retrieves webhooks for an organization.
func (s *Service) ListWebhooks(ctx context.Context, orgID uuid.UUID, opts models.WebhookListOptions) ([]*models.Webhook, error) {
	return s.repo.GetWebhooksByOrg(ctx, orgID, opts)
}

// UpdateWebhook updates an existing webhook.
func (s *Service) UpdateWebhook(ctx context.Context, id uuid.UUID, req models.UpdateWebhookRequest, userID *uuid.UUID) (*models.Webhook, error) {
	webhook, err := s.repo.GetWebhook(ctx, id)
	if err != nil {
		return nil, err
	}

	// Update fields
	updated := false
	if req.Name != nil {
		webhook.Name = *req.Name
		updated = true
	}
	if req.URL != nil {
		webhook.URL = *req.URL
		updated = true
	}
	if req.Events != nil {
		webhook.Events = req.Events
		updated = true
	}
	if req.IsActive != nil {
		webhook.IsActive = *req.IsActive
		updated = true
	}
	if req.RetryAttempts != nil {
		webhook.RetryAttempts = *req.RetryAttempts
		updated = true
	}
	if req.TimeoutSeconds != nil {
		webhook.TimeoutSeconds = *req.TimeoutSeconds
		updated = true
	}

	if !updated {
		return webhook, nil
	}

	webhook.UpdatedBy = userID
	webhook.UpdatedAt = time.Now()

	if err := s.repo.UpdateWebhook(ctx, webhook); err != nil {
		return nil, fmt.Errorf("failed to update webhook: %w", err)
	}

	// Publish webhook updated event
	if err := s.publishEvent(ctx, models.EventAuditLog, webhook.OrgID, webhook.ProjectID, map[string]interface{}{
		"action":     "webhook.updated",
		"webhook_id": webhook.ID,
		"name":       webhook.Name,
		"changes":    req,
	}, userID); err != nil {
		log.Printf("failed to publish webhook updated event: %v", err)
	}

	return webhook, nil
}

// DeleteWebhook deletes a webhook.
func (s *Service) DeleteWebhook(ctx context.Context, id uuid.UUID, userID *uuid.UUID) error {
	webhook, err := s.repo.GetWebhook(ctx, id)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteWebhook(ctx, id); err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}

	// Publish webhook deleted event
	if err := s.publishEvent(ctx, models.EventAuditLog, webhook.OrgID, webhook.ProjectID, map[string]interface{}{
		"action":     "webhook.deleted",
		"webhook_id": webhook.ID,
		"name":       webhook.Name,
	}, userID); err != nil {
		log.Printf("failed to publish webhook deleted event: %v", err)
	}

	return nil
}

// TestWebhook sends a test payload to a webhook endpoint.
func (s *Service) TestWebhook(ctx context.Context, id uuid.UUID, req models.WebhookTestRequest) (*models.WebhookDelivery, error) {
	webhook, err := s.repo.GetWebhook(ctx, id)
	if err != nil {
		return nil, err
	}

	// Create test payload
	payload := models.WebhookEventPayload{
		Event:     models.WebhookEvent(req.EventType),
		Timestamp: time.Now(),
		OrgID:     webhook.OrgID,
		ProjectID: webhook.ProjectID,
		Data:      req.Payload,
		Metadata: map[string]interface{}{
			"test":       true,
			"webhook_id": webhook.ID,
		},
	}

	// Create delivery record
	delivery := &models.WebhookDelivery{
		ID:        uuid.New(),
		WebhookID: webhook.ID,
		EventType: req.EventType,
		Payload:   map[string]interface{}(payload.Data),
		Status:    models.DeliveryStatusPending,
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateDelivery(ctx, delivery); err != nil {
		return nil, fmt.Errorf("failed to create delivery record: %w", err)
	}

	// Send the webhook
	if err := s.deliverWebhook(ctx, webhook, delivery, payload); err != nil {
		delivery.Status = models.DeliveryStatusFailed
		errMsg := err.Error()
		delivery.ErrorMessage = &errMsg
		delivery.AttemptCount = 1
	} else {
		delivery.Status = models.DeliveryStatusSent
		now := time.Now()
		delivery.SentAt = &now
		delivery.AttemptCount = 1
	}

	// Update delivery record
	if updateErr := s.repo.UpdateDelivery(ctx, delivery); updateErr != nil {
		return delivery, fmt.Errorf("webhook delivered but failed to update record: %w", updateErr)
	}

	return delivery, err // Return original error if delivery failed
}

// PublishEvent publishes an event that may trigger webhooks.
func (s *Service) PublishEvent(ctx context.Context, event models.WebhookEvent, orgID uuid.UUID, projectID *uuid.UUID, data map[string]interface{}, userID *uuid.UUID) error {
	_ = s.publishEvent(ctx, event, orgID, projectID, data, userID)
	return nil
}

// GetDeliveries retrieves webhook deliveries.
func (s *Service) GetDeliveries(ctx context.Context, webhookID uuid.UUID, opts models.WebhookDeliveryListOptions) ([]*models.WebhookDelivery, error) {
	return s.repo.GetDeliveriesByWebhook(ctx, webhookID, opts)
}

// RetryFailedDeliveries processes pending webhook retries.
func (s *Service) RetryFailedDeliveries(ctx context.Context) error {
	deliveries, err := s.repo.GetPendingRetries(ctx, 100)
	if err != nil {
		return fmt.Errorf("failed to get pending retries: %w", err)
	}

	for _, delivery := range deliveries {
		if err := s.retryDelivery(ctx, delivery); err != nil {
			// Log error but continue processing other deliveries
			continue
		}
	}

	return nil
}

// deliverWebhook delivers a webhook payload to an endpoint.
func (s *Service) deliverWebhook(ctx context.Context, webhook *models.Webhook, delivery *models.WebhookDelivery, payload models.WebhookEventPayload) error {
	// Create HTTP client with webhook-specific timeout
	client := &http.Client{
		Timeout: time.Duration(webhook.TimeoutSeconds) * time.Second,
	}

	// Serialize payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook.URL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := s.generateSignature(webhook.Secret, timestamp, payloadBytes)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "DeploySentry-Webhooks/1.0")
	req.Header.Set("X-DeploySentry-Event", string(payload.Event))
	req.Header.Set("X-DeploySentry-Timestamp", timestamp)
	req.Header.Set("X-DeploySentry-Signature", signature)

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response
	responseBody, _ := io.ReadAll(resp.Body)
	responseBodyStr := string(responseBody)

	// Update delivery record with response details
	delivery.HTTPStatus = &resp.StatusCode
	if len(responseBodyStr) > 0 {
		delivery.ResponseBody = &responseBodyStr
	}

	// Check if delivery was successful (2xx status codes)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, responseBodyStr)
}

// retryDelivery retries a failed webhook delivery.
func (s *Service) retryDelivery(ctx context.Context, delivery *models.WebhookDelivery) error {
	webhook, err := s.repo.GetWebhook(ctx, delivery.WebhookID)
	if err != nil {
		return err
	}

	if !webhook.IsActive {
		// Mark as cancelled if webhook is no longer active
		delivery.Status = models.DeliveryStatusCancelled
		return s.repo.UpdateDelivery(ctx, delivery)
	}

	if !delivery.ShouldRetry(webhook.RetryAttempts) {
		// Max retries exceeded, mark as failed
		delivery.Status = models.DeliveryStatusFailed
		delivery.NextRetryAt = nil
		return s.repo.UpdateDelivery(ctx, delivery)
	}

	// Create payload from delivery data
	payload := models.WebhookEventPayload{
		Event:     models.WebhookEvent(delivery.EventType),
		Timestamp: delivery.CreatedAt,
		OrgID:     webhook.OrgID,
		ProjectID: webhook.ProjectID,
		Data:      delivery.Payload,
	}

	// Attempt delivery
	delivery.AttemptCount++
	if err := s.deliverWebhook(ctx, webhook, delivery, payload); err != nil {
		// Delivery failed, schedule next retry
		delivery.Status = models.DeliveryStatusFailed
		errMsg := err.Error()
		delivery.ErrorMessage = &errMsg
		delivery.SetNextRetryAt()
	} else {
		// Delivery succeeded
		delivery.Status = models.DeliveryStatusSent
		now := time.Now()
		delivery.SentAt = &now
		delivery.NextRetryAt = nil
	}

	return s.repo.UpdateDelivery(ctx, delivery)
}

// publishEvent publishes an event to trigger webhook deliveries.
func (s *Service) publishEvent(ctx context.Context, event models.WebhookEvent, orgID uuid.UUID, projectID *uuid.UUID, data map[string]interface{}, userID *uuid.UUID) error {
	payload := models.WebhookEventPayload{
		Event:     event,
		Timestamp: time.Now(),
		OrgID:     orgID,
		ProjectID: projectID,
		Data:      data,
		UserID:    userID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling webhook payload: %w", err)
	}

	return s.publisher.Publish(ctx, "webhooks.event", payloadBytes)
}

// generateSecret generates a cryptographically secure secret for webhook signing.
func (s *Service) generateSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// generateSignature generates an HMAC-SHA256 signature for webhook verification.
func (s *Service) generateSignature(secret, timestamp string, payload []byte) string {
	message := timestamp + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature verifies a webhook signature.
func VerifySignature(secret, timestamp, signature string, payload []byte) bool {
	expectedSignature := generateSignatureForVerification(secret, timestamp, payload)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// generateSignatureForVerification is a helper for signature verification.
func generateSignatureForVerification(secret, timestamp string, payload []byte) string {
	message := timestamp + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}