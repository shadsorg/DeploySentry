package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lib/pq"

	"github.com/shadsorg/deploysentry/internal/models"
)

// WebhookRepository provides PostgreSQL-based webhook data access.
type WebhookRepository struct {
	db *pgxpool.Pool
}

// NewWebhookRepository creates a new webhook repository.
func NewWebhookRepository(db *pgxpool.Pool) *WebhookRepository {
	return &WebhookRepository{db: db}
}

// CreateWebhook creates a new webhook.
func (r *WebhookRepository) CreateWebhook(ctx context.Context, webhook *models.Webhook) error {
	webhook.CreatedAt = time.Now()
	webhook.UpdatedAt = webhook.CreatedAt

	query := `
		INSERT INTO webhooks (
			id, org_id, project_id, name, url, secret, encrypted, events, is_active,
			retry_attempts, timeout_seconds, created_at, updated_at,
			created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)`

	_, err := r.db.Exec(ctx, query,
		webhook.ID,
		webhook.OrgID,
		webhook.ProjectID,
		webhook.Name,
		webhook.URL,
		webhook.Secret,
		webhook.Encrypted,
		pq.Array(webhook.Events),
		webhook.IsActive,
		webhook.RetryAttempts,
		webhook.TimeoutSeconds,
		webhook.CreatedAt,
		webhook.UpdatedAt,
		webhook.CreatedBy,
		webhook.UpdatedBy,
	)

	return err
}

// GetWebhook retrieves a webhook by ID.
func (r *WebhookRepository) GetWebhook(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
	var webhook models.Webhook

	query := `
		SELECT
			id, org_id, project_id, name, url, secret, encrypted, events, is_active,
			retry_attempts, timeout_seconds, created_at, updated_at,
			created_by, updated_by
		FROM webhooks
		WHERE id = $1`

	row := r.db.QueryRow(ctx, query, id)
	err := row.Scan(
		&webhook.ID,
		&webhook.OrgID,
		&webhook.ProjectID,
		&webhook.Name,
		&webhook.URL,
		&webhook.Secret,
		&webhook.Encrypted,
		pq.Array(&webhook.Events),
		&webhook.IsActive,
		&webhook.RetryAttempts,
		&webhook.TimeoutSeconds,
		&webhook.CreatedAt,
		&webhook.UpdatedAt,
		&webhook.CreatedBy,
		&webhook.UpdatedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrWebhookNotFound
		}
		return nil, err
	}

	return &webhook, nil
}

// GetWebhooksByOrg retrieves webhooks for an organization.
func (r *WebhookRepository) GetWebhooksByOrg(ctx context.Context, orgID uuid.UUID, opts models.WebhookListOptions) ([]*models.Webhook, error) {
	var wb whereBuilder
	wb.Add("org_id = $%d", orgID)

	if opts.ProjectID != nil {
		wb.Add("project_id = $%d", *opts.ProjectID)
	}

	if opts.IsActive != nil {
		wb.Add("is_active = $%d", *opts.IsActive)
	}

	if len(opts.Events) > 0 {
		wb.Add("events && $%d", pq.Array(opts.Events))
	}

	whereClause, args := wb.Build()

	query := `
		SELECT
			id, org_id, project_id, name, url, secret, encrypted, events, is_active,
			retry_attempts, timeout_seconds, created_at, updated_at,
			created_by, updated_by
		FROM webhooks` + whereClause + `
		ORDER BY created_at DESC`

	if opts.Limit > 0 {
		if opts.Offset > 0 {
			query, args = paginationClause(opts.Limit, opts.Offset, args)
			query = `
		SELECT
			id, org_id, project_id, name, url, secret, encrypted, events, is_active,
			retry_attempts, timeout_seconds, created_at, updated_at,
			created_by, updated_by
		FROM webhooks` + whereClause + `
		ORDER BY created_at DESC` + query
		} else {
			startPos := len(args) + 1
			query += fmt.Sprintf(" LIMIT $%d", startPos)
			args = append(args, opts.Limit)
		}
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []*models.Webhook
	for rows.Next() {
		var webhook models.Webhook
		err := rows.Scan(
			&webhook.ID,
			&webhook.OrgID,
			&webhook.ProjectID,
			&webhook.Name,
			&webhook.URL,
			&webhook.Secret,
			&webhook.Encrypted,
			pq.Array(&webhook.Events),
			&webhook.IsActive,
			&webhook.RetryAttempts,
			&webhook.TimeoutSeconds,
			&webhook.CreatedAt,
			&webhook.UpdatedAt,
			&webhook.CreatedBy,
			&webhook.UpdatedBy,
		)
		if err != nil {
			return nil, err
		}
		webhooks = append(webhooks, &webhook)
	}

	return webhooks, rows.Err()
}

// UpdateWebhook updates a webhook.
func (r *WebhookRepository) UpdateWebhook(ctx context.Context, webhook *models.Webhook) error {
	webhook.UpdatedAt = time.Now()

	query := `
		UPDATE webhooks SET
			name = $2, url = $3, events = $4, is_active = $5,
			retry_attempts = $6, timeout_seconds = $7, updated_at = $8, updated_by = $9
		WHERE id = $1`

	result, err := r.db.Exec(ctx, query,
		webhook.ID,
		webhook.Name,
		webhook.URL,
		pq.Array(webhook.Events),
		webhook.IsActive,
		webhook.RetryAttempts,
		webhook.TimeoutSeconds,
		webhook.UpdatedAt,
		webhook.UpdatedBy,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}

	return nil
}

// DeleteWebhook deletes a webhook.
func (r *WebhookRepository) DeleteWebhook(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM webhooks WHERE id = $1`

	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}

	return nil
}

// CreateDelivery creates a new webhook delivery record.
func (r *WebhookRepository) CreateDelivery(ctx context.Context, delivery *models.WebhookDelivery) error {
	delivery.CreatedAt = time.Now()

	// Convert payload to JSON
	payloadJSON, err := json.Marshal(delivery.Payload)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO webhook_deliveries (
			id, webhook_id, event_type, payload, status, http_status,
			response_body, error_message, attempt_count, created_at,
			sent_at, next_retry_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)`

	_, err = r.db.Exec(ctx, query,
		delivery.ID,
		delivery.WebhookID,
		delivery.EventType,
		payloadJSON,
		string(delivery.Status),
		delivery.HTTPStatus,
		delivery.ResponseBody,
		delivery.ErrorMessage,
		delivery.AttemptCount,
		delivery.CreatedAt,
		delivery.SentAt,
		delivery.NextRetryAt,
	)

	return err
}

// GetDelivery retrieves a webhook delivery by ID.
func (r *WebhookRepository) GetDelivery(ctx context.Context, id uuid.UUID) (*models.WebhookDelivery, error) {
	var delivery models.WebhookDelivery
	var payloadJSON []byte

	query := `
		SELECT
			id, webhook_id, event_type, payload, status, http_status,
			response_body, error_message, attempt_count, created_at,
			sent_at, next_retry_at
		FROM webhook_deliveries
		WHERE id = $1`

	row := r.db.QueryRow(ctx, query, id)
	err := row.Scan(
		&delivery.ID,
		&delivery.WebhookID,
		&delivery.EventType,
		&payloadJSON,
		&delivery.Status,
		&delivery.HTTPStatus,
		&delivery.ResponseBody,
		&delivery.ErrorMessage,
		&delivery.AttemptCount,
		&delivery.CreatedAt,
		&delivery.SentAt,
		&delivery.NextRetryAt,
	)

	if err != nil {
		return nil, err
	}

	// Parse JSON payload
	if err := json.Unmarshal(payloadJSON, &delivery.Payload); err != nil {
		return nil, err
	}

	return &delivery, nil
}

// GetDeliveriesByWebhook retrieves webhook deliveries for a specific webhook.
func (r *WebhookRepository) GetDeliveriesByWebhook(ctx context.Context, webhookID uuid.UUID, opts models.WebhookDeliveryListOptions) ([]*models.WebhookDelivery, error) {
	var wb whereBuilder
	wb.Add("webhook_id = $%d", webhookID)

	if opts.Status != nil {
		wb.Add("status = $%d", string(*opts.Status))
	}

	if opts.EventType != nil {
		wb.Add("event_type = $%d", *opts.EventType)
	}

	if opts.Since != nil {
		wb.Add("created_at >= $%d", *opts.Since)
	}

	whereClause, args := wb.Build()

	query := `
		SELECT
			id, webhook_id, event_type, payload, status, http_status,
			response_body, error_message, attempt_count, created_at,
			sent_at, next_retry_at
		FROM webhook_deliveries` + whereClause + `
		ORDER BY created_at DESC`

	if opts.Limit > 0 {
		if opts.Offset > 0 {
			paginationQuery, paginationArgs := paginationClause(opts.Limit, opts.Offset, args)
			query = `
		SELECT
			id, webhook_id, event_type, payload, status, http_status,
			response_body, error_message, attempt_count, created_at,
			sent_at, next_retry_at
		FROM webhook_deliveries` + whereClause + `
		ORDER BY created_at DESC` + paginationQuery
			args = paginationArgs
		} else {
			startPos := len(args) + 1
			query += fmt.Sprintf(" LIMIT $%d", startPos)
			args = append(args, opts.Limit)
		}
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deliveries []*models.WebhookDelivery
	for rows.Next() {
		var delivery models.WebhookDelivery
		var payloadJSON []byte

		err := rows.Scan(
			&delivery.ID,
			&delivery.WebhookID,
			&delivery.EventType,
			&payloadJSON,
			&delivery.Status,
			&delivery.HTTPStatus,
			&delivery.ResponseBody,
			&delivery.ErrorMessage,
			&delivery.AttemptCount,
			&delivery.CreatedAt,
			&delivery.SentAt,
			&delivery.NextRetryAt,
		)
		if err != nil {
			return nil, err
		}

		// Parse JSON payload
		if err := json.Unmarshal(payloadJSON, &delivery.Payload); err != nil {
			return nil, err
		}

		deliveries = append(deliveries, &delivery)
	}

	return deliveries, rows.Err()
}

// UpdateDelivery updates a webhook delivery record.
func (r *WebhookRepository) UpdateDelivery(ctx context.Context, delivery *models.WebhookDelivery) error {
	// Convert payload to JSON
	payloadJSON, err := json.Marshal(delivery.Payload)
	if err != nil {
		return err
	}

	query := `
		UPDATE webhook_deliveries SET
			status = $2, http_status = $3, response_body = $4, error_message = $5,
			attempt_count = $6, sent_at = $7, next_retry_at = $8, payload = $9
		WHERE id = $1`

	result, err := r.db.Exec(ctx, query,
		delivery.ID,
		string(delivery.Status),
		delivery.HTTPStatus,
		delivery.ResponseBody,
		delivery.ErrorMessage,
		delivery.AttemptCount,
		delivery.SentAt,
		delivery.NextRetryAt,
		payloadJSON,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}

	return nil
}

// GetPendingRetries retrieves webhook deliveries that need to be retried.
func (r *WebhookRepository) GetPendingRetries(ctx context.Context, limit int) ([]*models.WebhookDelivery, error) {
	query := `
		SELECT
			d.id, d.webhook_id, d.event_type, d.payload, d.status, d.http_status,
			d.response_body, d.error_message, d.attempt_count, d.created_at,
			d.sent_at, d.next_retry_at
		FROM webhook_deliveries d
		INNER JOIN webhooks w ON d.webhook_id = w.id
		WHERE d.status = $1
			AND d.next_retry_at IS NOT NULL
			AND d.next_retry_at <= NOW()
			AND w.is_active = true
		ORDER BY d.next_retry_at ASC
		LIMIT $2`

	rows, err := r.db.Query(ctx, query, string(models.DeliveryStatusFailed), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deliveries []*models.WebhookDelivery
	for rows.Next() {
		var delivery models.WebhookDelivery
		var payloadJSON []byte

		err := rows.Scan(
			&delivery.ID,
			&delivery.WebhookID,
			&delivery.EventType,
			&payloadJSON,
			&delivery.Status,
			&delivery.HTTPStatus,
			&delivery.ResponseBody,
			&delivery.ErrorMessage,
			&delivery.AttemptCount,
			&delivery.CreatedAt,
			&delivery.SentAt,
			&delivery.NextRetryAt,
		)
		if err != nil {
			return nil, err
		}

		// Parse JSON payload
		if err := json.Unmarshal(payloadJSON, &delivery.Payload); err != nil {
			return nil, err
		}

		deliveries = append(deliveries, &delivery)
	}

	return deliveries, rows.Err()
}