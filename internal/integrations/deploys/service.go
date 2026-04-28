package deploys

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/platform/crypto"
	"github.com/google/uuid"
)

// Repository is the persistence surface used by the Service.
type Repository interface {
	Create(ctx context.Context, i *models.DeployIntegration) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.DeployIntegration, error)
	FindByProviderAndConfig(ctx context.Context, provider string, match func(cfg map[string]any) bool) (*models.DeployIntegration, error)
	ListForApp(ctx context.Context, appID uuid.UUID) ([]*models.DeployIntegration, error)
	Delete(ctx context.Context, id uuid.UUID) error
	RecordEvent(ctx context.Context, e *models.DeployIntegrationEvent) (*models.DeployIntegrationEvent, bool, error)
	SetEventDeployment(ctx context.Context, eventID, deploymentID uuid.UUID) error
	ListRecentEvents(ctx context.Context, integrationID uuid.UUID, limit int) ([]*models.DeployIntegrationEvent, error)
}

// DeployCreator is the narrow dependency used to insert mode=record rows.
type DeployCreator interface {
	CreateDeployment(ctx context.Context, d *models.Deployment) error
}

// Service is the shared ingestion path. Every adapter funnels its canonical
// DeployEvent here.
type Service struct {
	repo          Repository
	deploys       DeployCreator
	encryptionKey []byte
}

func NewService(repo Repository, deploys DeployCreator, encryptionKey []byte) *Service {
	return &Service{repo: repo, deploys: deploys, encryptionKey: encryptionKey}
}

// ErrUnmappedEnvironment is returned when the event's environment isn't in
// the integration's env_mapping. Callers should treat this as non-fatal
// (HTTP 202 + logged) — fail closed, not silent.
var ErrUnmappedEnvironment = errors.New("unmapped environment")

// ErrIntegrationNotFound is returned from lookup helpers when the integration row is missing.
var ErrIntegrationNotFound = errors.New("integration not found")

// IngestInput bundles the data the handler collects before handing off.
type IngestInput struct {
	Integration *models.DeployIntegration
	Event       models.DeployEvent
	RawPayload  []byte
	ActorUserID uuid.UUID
}

// IngestResult reports the outcome of a single webhook delivery.
type IngestResult struct {
	Deployment *models.Deployment
	Event      *models.DeployIntegrationEvent
	Created    bool // true when this delivery produced a new deployment row
	Duplicate  bool // true when the event was already stored
}

func (s *Service) Ingest(ctx context.Context, in IngestInput) (*IngestResult, error) {
	if in.Integration == nil {
		return nil, ErrIntegrationNotFound
	}

	envID, ok := in.Integration.EnvMapping[in.Event.Environment]
	if !ok || envID == uuid.Nil {
		return nil, ErrUnmappedEnvironment
	}

	dedup := models.DeployEventDedupKey(in.Integration.ApplicationID, envID, in.Event.Version, in.Event.EventType)
	payload := in.RawPayload
	if len(payload) == 0 {
		payload, _ = json.Marshal(in.Event)
	}

	record := &models.DeployIntegrationEvent{
		IntegrationID: in.Integration.ID,
		EventType:     in.Event.EventType,
		DedupKey:      dedup,
		PayloadJSON:   payload,
	}
	saved, inserted, err := s.repo.RecordEvent(ctx, record)
	if err != nil {
		return nil, fmt.Errorf("record event: %w", err)
	}
	result := &IngestResult{Event: saved, Duplicate: !inserted}

	// Idempotent replay — if the prior delivery already produced a deployment,
	// just return it.
	if !inserted && saved.DeploymentID != nil {
		return result, nil
	}

	// Only `deploy.succeeded` events create deployment rows in v1.
	if in.Event.EventType != models.DeployEventSucceeded {
		return result, nil
	}

	artifact := in.Event.Artifact
	if artifact == "" {
		artifact = in.Event.Version
	}
	source := in.Integration.Provider + "-webhook"
	if in.Integration.Provider == models.DeployProviderGeneric {
		source = "generic-webhook"
	}
	src := source
	d := &models.Deployment{
		ApplicationID: in.Integration.ApplicationID,
		EnvironmentID: envID,
		Strategy:      models.DeployStrategyRolling, // informational in record mode
		Artifact:      artifact,
		Version:       in.Event.Version,
		CommitSHA:     in.Event.CommitSHA,
		CreatedBy:     in.ActorUserID,
		Mode:          models.DeployModeRecord,
		Source:        &src,
	}
	if err := s.deploys.CreateDeployment(ctx, d); err != nil {
		return nil, fmt.Errorf("auto-create deployment: %w", err)
	}
	if err := s.repo.SetEventDeployment(ctx, saved.ID, d.ID); err != nil {
		return nil, fmt.Errorf("backfill deployment id: %w", err)
	}
	saved.DeploymentID = &d.ID
	result.Deployment = d
	result.Created = true
	return result, nil
}

// -----------------------------------------------------------------------------
// Integration CRUD
// -----------------------------------------------------------------------------

// CreateIntegrationInput is the external API shape.
type CreateIntegrationInput struct {
	ApplicationID     uuid.UUID
	Provider          string
	AuthMode          string
	WebhookSecret     string // plaintext; encrypted here before persisting
	ProviderConfig    map[string]any
	EnvMapping        map[string]uuid.UUID
	VersionExtractors []string
}

func (s *Service) CreateIntegration(ctx context.Context, in CreateIntegrationInput) (*models.DeployIntegration, error) {
	if in.ApplicationID == uuid.Nil {
		return nil, errors.New("application_id is required")
	}
	if !validProvider(in.Provider) {
		return nil, fmt.Errorf("unsupported provider %q", in.Provider)
	}
	authMode := in.AuthMode
	if authMode == "" {
		authMode = models.DeployIntegrationAuthHMAC
	}
	if authMode != models.DeployIntegrationAuthHMAC && authMode != models.DeployIntegrationAuthBearer {
		return nil, fmt.Errorf("auth_mode must be hmac or bearer")
	}
	if in.WebhookSecret == "" {
		return nil, errors.New("webhook_secret is required")
	}
	enc, err := crypto.Encrypt([]byte(in.WebhookSecret), s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt secret: %w", err)
	}
	row := &models.DeployIntegration{
		ApplicationID:     in.ApplicationID,
		Provider:          in.Provider,
		AuthMode:          authMode,
		WebhookSecretEnc:  enc,
		ProviderConfig:    in.ProviderConfig,
		EnvMapping:        in.EnvMapping,
		VersionExtractors: in.VersionExtractors,
		Enabled:           true,
	}
	if err := s.repo.Create(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}

// ResolveSecret decrypts the stored webhook secret for adapter use. Never exposed over HTTP.
func (s *Service) ResolveSecret(integration *models.DeployIntegration) (string, error) {
	dec, err := crypto.Decrypt(integration.WebhookSecretEnc, s.encryptionKey)
	if err != nil {
		return "", err
	}
	return string(dec), nil
}

func (s *Service) GetIntegration(ctx context.Context, id uuid.UUID) (*models.DeployIntegration, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) ListIntegrationsForApp(ctx context.Context, appID uuid.UUID) ([]*models.DeployIntegration, error) {
	return s.repo.ListForApp(ctx, appID)
}

func (s *Service) FindIntegration(ctx context.Context, provider string, match func(cfg map[string]any) bool) (*models.DeployIntegration, error) {
	return s.repo.FindByProviderAndConfig(ctx, provider, match)
}

func (s *Service) DeleteIntegration(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) ListRecentEvents(ctx context.Context, integrationID uuid.UUID, limit int) ([]*models.DeployIntegrationEvent, error) {
	return s.repo.ListRecentEvents(ctx, integrationID, limit)
}

func validProvider(p string) bool {
	switch p {
	case models.DeployProviderGeneric, models.DeployProviderRailway, models.DeployProviderRender,
		models.DeployProviderFly, models.DeployProviderHeroku, models.DeployProviderVercel,
		models.DeployProviderNetlify, models.DeployProviderGitHubActions:
		return true
	}
	return false
}
