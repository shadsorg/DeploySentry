package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DeployIntegrationRepository persists deploy-integration configs + events.
type DeployIntegrationRepository struct {
	pool *pgxpool.Pool
}

func NewDeployIntegrationRepository(pool *pgxpool.Pool) *DeployIntegrationRepository {
	return &DeployIntegrationRepository{pool: pool}
}

const deployIntegrationCols = `
	id, application_id, provider, auth_mode, webhook_secret_enc,
	provider_config, env_mapping, version_extractors,
	enabled, created_at, updated_at`

func (r *DeployIntegrationRepository) Create(ctx context.Context, i *models.DeployIntegration) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	providerCfg, _ := json.Marshal(i.ProviderConfig)
	envMap, _ := json.Marshal(i.EnvMapping)
	extractors, _ := json.Marshal(i.VersionExtractors)

	const q = `
		INSERT INTO deploy_integrations
			(id, application_id, provider, auth_mode, webhook_secret_enc,
			 provider_config, env_mapping, version_extractors, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.pool.Exec(ctx, q,
		i.ID, i.ApplicationID, i.Provider, i.AuthMode, i.WebhookSecretEnc,
		providerCfg, envMap, extractors, i.Enabled,
	)
	if err != nil {
		return fmt.Errorf("postgres.DeployIntegration.Create: %w", err)
	}
	return nil
}

func (r *DeployIntegrationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.DeployIntegration, error) {
	q := `SELECT ` + deployIntegrationCols + ` FROM deploy_integrations WHERE id = $1`
	row := r.pool.QueryRow(ctx, q, id)
	return scanDeployIntegration(row)
}

func (r *DeployIntegrationRepository) FindByProviderAndConfig(
	ctx context.Context, provider string, match func(cfg map[string]any) bool,
) (*models.DeployIntegration, error) {
	// Callers typically know app_id; exposed here for the Railway adapter
	// that has only the provider payload to key on.
	q := `SELECT ` + deployIntegrationCols + ` FROM deploy_integrations WHERE provider = $1 AND enabled = TRUE`
	rows, err := r.pool.Query(ctx, q, provider)
	if err != nil {
		return nil, fmt.Errorf("postgres.DeployIntegration.FindByProvider: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		i, err := scanDeployIntegration(rows)
		if err != nil {
			return nil, err
		}
		if match(i.ProviderConfig) {
			return i, nil
		}
	}
	return nil, ErrNotFound
}

func (r *DeployIntegrationRepository) ListForApp(ctx context.Context, appID uuid.UUID) ([]*models.DeployIntegration, error) {
	q := `SELECT ` + deployIntegrationCols + ` FROM deploy_integrations WHERE application_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, appID)
	if err != nil {
		return nil, fmt.Errorf("postgres.DeployIntegration.ListForApp: %w", err)
	}
	defer rows.Close()
	var out []*models.DeployIntegration
	for rows.Next() {
		i, err := scanDeployIntegration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, nil
}

func (r *DeployIntegrationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM deploy_integrations WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres.DeployIntegration.Delete: %w", err)
	}
	return nil
}

// RecordEvent inserts an event row. On dedup_key conflict, returns the
// previously-stored (existing, true) without modifying it.
func (r *DeployIntegrationRepository) RecordEvent(ctx context.Context, e *models.DeployIntegrationEvent) (*models.DeployIntegrationEvent, bool, error) {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	const q = `
		INSERT INTO deploy_integration_events
			(id, integration_id, event_type, dedup_key, deployment_id, payload_json, received_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (dedup_key) DO NOTHING
		RETURNING id`

	var insertedID uuid.UUID
	err := r.pool.QueryRow(ctx, q,
		e.ID, e.IntegrationID, e.EventType, e.DedupKey, e.DeploymentID, e.PayloadJSON,
	).Scan(&insertedID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, fmt.Errorf("postgres.RecordEvent insert: %w", err)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		// Conflict: fetch the existing row.
		existing, gerr := r.GetEventByDedup(ctx, e.DedupKey)
		if gerr != nil {
			return nil, false, gerr
		}
		return existing, false, nil
	}
	return e, true, nil
}

func (r *DeployIntegrationRepository) GetEventByDedup(ctx context.Context, key string) (*models.DeployIntegrationEvent, error) {
	const q = `
		SELECT id, integration_id, event_type, dedup_key, deployment_id, payload_json, received_at
		FROM deploy_integration_events WHERE dedup_key = $1`
	row := r.pool.QueryRow(ctx, q, key)
	var e models.DeployIntegrationEvent
	err := row.Scan(&e.ID, &e.IntegrationID, &e.EventType, &e.DedupKey, &e.DeploymentID, &e.PayloadJSON, &e.ReceivedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetEventByDedup: %w", err)
	}
	return &e, nil
}

func (r *DeployIntegrationRepository) SetEventDeployment(ctx context.Context, eventID, deploymentID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE deploy_integration_events SET deployment_id = $1 WHERE id = $2`, deploymentID, eventID)
	return err
}

func (r *DeployIntegrationRepository) ListRecentEvents(ctx context.Context, integrationID uuid.UUID, limit int) ([]*models.DeployIntegrationEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	const q = `
		SELECT id, integration_id, event_type, dedup_key, deployment_id, payload_json, received_at
		FROM deploy_integration_events WHERE integration_id = $1
		ORDER BY received_at DESC LIMIT $2`
	rows, err := r.pool.Query(ctx, q, integrationID, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListRecentEvents: %w", err)
	}
	defer rows.Close()
	var out []*models.DeployIntegrationEvent
	for rows.Next() {
		var e models.DeployIntegrationEvent
		if err := rows.Scan(&e.ID, &e.IntegrationID, &e.EventType, &e.DedupKey, &e.DeploymentID, &e.PayloadJSON, &e.ReceivedAt); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, nil
}

func scanDeployIntegration(row pgx.Row) (*models.DeployIntegration, error) {
	var i models.DeployIntegration
	var providerCfg, envMap, extractors []byte
	err := row.Scan(
		&i.ID, &i.ApplicationID, &i.Provider, &i.AuthMode, &i.WebhookSecretEnc,
		&providerCfg, &envMap, &extractors, &i.Enabled, &i.CreatedAt, &i.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanDeployIntegration: %w", err)
	}
	if len(providerCfg) > 0 {
		_ = json.Unmarshal(providerCfg, &i.ProviderConfig)
	}
	if len(envMap) > 0 {
		raw := map[string]string{}
		if err := json.Unmarshal(envMap, &raw); err != nil {
			return nil, fmt.Errorf("env_mapping: %w", err)
		}
		i.EnvMapping = make(map[string]uuid.UUID, len(raw))
		for k, v := range raw {
			id, err := uuid.Parse(v)
			if err != nil {
				return nil, fmt.Errorf("env_mapping[%s]: %w", k, err)
			}
			i.EnvMapping[k] = id
		}
	}
	if len(extractors) > 0 {
		_ = json.Unmarshal(extractors, &i.VersionExtractors)
	}
	return &i, nil
}
