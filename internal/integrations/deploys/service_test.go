package deploys

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/platform/crypto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// -----------------------------------------------------------------------------
// Mocks
// -----------------------------------------------------------------------------

type fakeRepo struct {
	integrations map[uuid.UUID]*models.DeployIntegration
	events       map[string]*models.DeployIntegrationEvent
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		integrations: map[uuid.UUID]*models.DeployIntegration{},
		events:       map[string]*models.DeployIntegrationEvent{},
	}
}

func (f *fakeRepo) Create(_ context.Context, i *models.DeployIntegration) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	i.CreatedAt = time.Now()
	i.UpdatedAt = time.Now()
	f.integrations[i.ID] = i
	return nil
}

func (f *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (*models.DeployIntegration, error) {
	i, ok := f.integrations[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return i, nil
}

func (f *fakeRepo) FindByProviderAndConfig(_ context.Context, provider string, match func(cfg map[string]any) bool) (*models.DeployIntegration, error) {
	for _, i := range f.integrations {
		if i.Provider == provider && i.Enabled && match(i.ProviderConfig) {
			return i, nil
		}
	}
	return nil, errors.New("not found")
}

func (f *fakeRepo) ListForApp(_ context.Context, appID uuid.UUID) ([]*models.DeployIntegration, error) {
	out := []*models.DeployIntegration{}
	for _, i := range f.integrations {
		if i.ApplicationID == appID {
			out = append(out, i)
		}
	}
	return out, nil
}

func (f *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(f.integrations, id)
	return nil
}

func (f *fakeRepo) RecordEvent(_ context.Context, e *models.DeployIntegrationEvent) (*models.DeployIntegrationEvent, bool, error) {
	if existing, ok := f.events[e.DedupKey]; ok {
		return existing, false, nil
	}
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	e.ReceivedAt = time.Now()
	f.events[e.DedupKey] = e
	return e, true, nil
}

func (f *fakeRepo) SetEventDeployment(_ context.Context, eventID, deploymentID uuid.UUID) error {
	for _, e := range f.events {
		if e.ID == eventID {
			dID := deploymentID
			e.DeploymentID = &dID
			return nil
		}
	}
	return errors.New("event not found")
}

func (f *fakeRepo) ListRecentEvents(_ context.Context, integrationID uuid.UUID, _ int) ([]*models.DeployIntegrationEvent, error) {
	out := []*models.DeployIntegrationEvent{}
	for _, e := range f.events {
		if e.IntegrationID == integrationID {
			out = append(out, e)
		}
	}
	return out, nil
}

type fakeDeployCreator struct {
	created []*models.Deployment
}

func (f *fakeDeployCreator) CreateDeployment(_ context.Context, d *models.Deployment) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	f.created = append(f.created, d)
	return nil
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func newTestService(t *testing.T) (*Service, *fakeRepo, *fakeDeployCreator, []byte) {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo()
	deploys := &fakeDeployCreator{}
	return NewService(repo, deploys, key), repo, deploys, key
}

func preconfiguredIntegration(repo *fakeRepo, key []byte, envSlug string, envID uuid.UUID) *models.DeployIntegration {
	enc, _ := crypto.Encrypt([]byte("s3cret"), key)
	i := &models.DeployIntegration{
		ID:               uuid.New(),
		ApplicationID:    uuid.New(),
		Provider:         models.DeployProviderRailway,
		AuthMode:         models.DeployIntegrationAuthHMAC,
		WebhookSecretEnc: enc,
		ProviderConfig:   map[string]any{"service_id": "svc-123"},
		EnvMapping:       map[string]uuid.UUID{envSlug: envID},
		Enabled:          true,
	}
	repo.integrations[i.ID] = i
	return i
}

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

func TestIngest_SucceededCreatesDeployment(t *testing.T) {
	svc, repo, deploys, key := newTestService(t)
	envID := uuid.New()
	i := preconfiguredIntegration(repo, key, "production", envID)

	ev := models.DeployEvent{
		EventType:   models.DeployEventSucceeded,
		Environment: "production",
		Version:     "1.4.2",
		CommitSHA:   "abc123",
	}
	body, _ := json.Marshal(ev)

	result, err := svc.Ingest(context.Background(), IngestInput{
		Integration: i,
		Event:       ev,
		RawPayload:  body,
	})
	assert.NoError(t, err)
	assert.True(t, result.Created)
	assert.False(t, result.Duplicate)
	if assert.Equal(t, 1, len(deploys.created)) {
		d := deploys.created[0]
		assert.Equal(t, models.DeployModeRecord, d.Mode)
		assert.Equal(t, "1.4.2", d.Version)
		assert.Equal(t, envID, d.EnvironmentID)
		if assert.NotNil(t, d.Source) {
			assert.Equal(t, "railway-webhook", *d.Source)
		}
	}
	assert.NotNil(t, result.Event.DeploymentID)
}

func TestIngest_ReplayReturnsExistingDeployment(t *testing.T) {
	svc, repo, deploys, key := newTestService(t)
	envID := uuid.New()
	i := preconfiguredIntegration(repo, key, "production", envID)
	ev := models.DeployEvent{
		EventType: models.DeployEventSucceeded, Environment: "production", Version: "1",
	}

	_, err := svc.Ingest(context.Background(), IngestInput{Integration: i, Event: ev})
	assert.NoError(t, err)
	// Second delivery with the same (app, env, version, event_type).
	result, err := svc.Ingest(context.Background(), IngestInput{Integration: i, Event: ev})
	assert.NoError(t, err)
	assert.True(t, result.Duplicate)
	assert.False(t, result.Created)
	assert.Equal(t, 1, len(deploys.created), "no second deployment")
}

func TestIngest_UnmappedEnvironmentReturnsError(t *testing.T) {
	svc, repo, deploys, key := newTestService(t)
	i := preconfiguredIntegration(repo, key, "production", uuid.New())
	ev := models.DeployEvent{
		EventType: models.DeployEventSucceeded, Environment: "staging", Version: "1",
	}

	_, err := svc.Ingest(context.Background(), IngestInput{Integration: i, Event: ev})
	assert.ErrorIs(t, err, ErrUnmappedEnvironment)
	assert.Equal(t, 0, len(deploys.created))
}

func TestIngest_FailedEventDoesNotCreateDeployment(t *testing.T) {
	svc, repo, deploys, key := newTestService(t)
	envID := uuid.New()
	i := preconfiguredIntegration(repo, key, "production", envID)

	_, err := svc.Ingest(context.Background(), IngestInput{
		Integration: i,
		Event:       models.DeployEvent{EventType: models.DeployEventFailed, Environment: "production", Version: "1"},
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(deploys.created))
}

func TestCreateIntegration_EncryptsSecret(t *testing.T) {
	svc, _, _, key := newTestService(t)
	i, err := svc.CreateIntegration(context.Background(), CreateIntegrationInput{
		ApplicationID: uuid.New(),
		Provider:      models.DeployProviderRailway,
		AuthMode:      models.DeployIntegrationAuthHMAC,
		WebhookSecret: "s3cret",
		ProviderConfig: map[string]any{"service_id": "svc-123"},
		EnvMapping: map[string]uuid.UUID{"production": uuid.New()},
	})
	assert.NoError(t, err)
	assert.NotEqual(t, []byte("s3cret"), i.WebhookSecretEnc, "secret must be stored encrypted")
	dec, err := crypto.Decrypt(i.WebhookSecretEnc, key)
	assert.NoError(t, err)
	assert.Equal(t, "s3cret", string(dec))
}

func TestCreateIntegration_ValidatesProvider(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	_, err := svc.CreateIntegration(context.Background(), CreateIntegrationInput{
		ApplicationID: uuid.New(),
		Provider:      "not-real",
		WebhookSecret: "x",
		EnvMapping:    map[string]uuid.UUID{"production": uuid.New()},
	})
	assert.Error(t, err)
}
