package releases

import (
	"context"
	"errors"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockReleaseRepo is an in-memory mock implementation of ReleaseRepository.
type mockReleaseRepo struct {
	releases    map[uuid.UUID]*models.Release
	flagChanges []*models.ReleaseFlagChange
	createErr   error
	updateErr   error
	deleteErr   error
	fcErr       error
}

func newMockRepo() *mockReleaseRepo {
	return &mockReleaseRepo{
		releases: make(map[uuid.UUID]*models.Release),
	}
}

func (m *mockReleaseRepo) Create(_ context.Context, release *models.Release) error {
	if m.createErr != nil {
		return m.createErr
	}
	stored := *release
	m.releases[release.ID] = &stored
	return nil
}

func (m *mockReleaseRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Release, error) {
	r, ok := m.releases[id]
	if !ok {
		return nil, errors.New("release not found")
	}
	copy := *r
	return &copy, nil
}

func (m *mockReleaseRepo) ListByApplication(_ context.Context, appID uuid.UUID) ([]models.Release, error) {
	var result []models.Release
	for _, r := range m.releases {
		if r.ApplicationID == appID {
			result = append(result, *r)
		}
	}
	return result, nil
}

func (m *mockReleaseRepo) Update(_ context.Context, release *models.Release) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	stored := *release
	m.releases[release.ID] = &stored
	return nil
}

func (m *mockReleaseRepo) Delete(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.releases, id)
	return nil
}

func (m *mockReleaseRepo) AddFlagChange(_ context.Context, fc *models.ReleaseFlagChange) error {
	if m.fcErr != nil {
		return m.fcErr
	}
	copy := *fc
	m.flagChanges = append(m.flagChanges, &copy)
	return nil
}

func (m *mockReleaseRepo) ListFlagChanges(_ context.Context, releaseID uuid.UUID) ([]models.ReleaseFlagChange, error) {
	var result []models.ReleaseFlagChange
	for _, fc := range m.flagChanges {
		if fc.ReleaseID == releaseID {
			result = append(result, *fc)
		}
	}
	return result, nil
}

// validRelease returns a fully populated Release with all required fields set.
func validRelease() *models.Release {
	return &models.Release{
		ApplicationID: uuid.New(),
		Name:          "Enable checkout v2",
	}
}

// ---------------------------------------------------------------------------
// Create tests
// ---------------------------------------------------------------------------

func TestCreate_AssignsIDAndDraftStatus(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	err := svc.Create(ctx, release)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, release.ID, "Create should assign a new UUID")
	assert.Equal(t, models.ReleaseDraft, release.Status, "Create should set status to draft")
}

func TestCreate_SetsTimestamps(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	err := svc.Create(ctx, release)
	require.NoError(t, err)

	assert.False(t, release.CreatedAt.IsZero(), "CreatedAt should be set")
	assert.False(t, release.UpdatedAt.IsZero(), "UpdatedAt should be set")
	assert.Equal(t, release.CreatedAt, release.UpdatedAt, "CreatedAt and UpdatedAt should match on creation")
}

func TestCreate_ValidationError(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	tests := []struct {
		name    string
		modify  func(r *models.Release)
		wantMsg string
	}{
		{
			name:    "missing application_id",
			modify:  func(r *models.Release) { r.ApplicationID = uuid.Nil },
			wantMsg: "application_id is required",
		},
		{
			name:    "missing name",
			modify:  func(r *models.Release) { r.Name = "" },
			wantMsg: "name is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			release := validRelease()
			tc.modify(release)
			err := svc.Create(ctx, release)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "validation failed")
			assert.Contains(t, err.Error(), tc.wantMsg)
		})
	}
}

func TestCreate_RepoError(t *testing.T) {
	repoErr := errors.New("database connection lost")
	repo := newMockRepo()
	repo.createErr = repoErr
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	err := svc.Create(ctx, release)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating release")
	assert.ErrorIs(t, err, repoErr)
}

func TestCreate_PersistsToRepo(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	err := svc.Create(ctx, release)
	require.NoError(t, err)

	stored, ok := repo.releases[release.ID]
	require.True(t, ok, "release should be stored in the repository")
	assert.Equal(t, release.Name, stored.Name)
	assert.Equal(t, models.ReleaseDraft, stored.Status)
}

// ---------------------------------------------------------------------------
// Get tests
// ---------------------------------------------------------------------------

func TestGetByID_ExistingRelease(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	require.NoError(t, svc.Create(ctx, release))

	fetched, err := svc.GetByID(ctx, release.ID)
	require.NoError(t, err)
	assert.Equal(t, release.ID, fetched.ID)
	assert.Equal(t, release.Name, fetched.Name)
	assert.Equal(t, models.ReleaseDraft, fetched.Status)
}

func TestGetByID_NotFound(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	_, err := svc.GetByID(ctx, uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting release")
}

// ---------------------------------------------------------------------------
// Lifecycle tests
// ---------------------------------------------------------------------------

func TestStart_DraftToRollingOut(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	require.NoError(t, svc.Create(ctx, release))

	err := svc.Start(ctx, release.ID)
	require.NoError(t, err)

	updated, err := svc.GetByID(ctx, release.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ReleaseRollingOut, updated.Status)
	assert.NotNil(t, updated.StartedAt)
}

func TestPromote_UpdatesTrafficPercent(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	require.NoError(t, svc.Create(ctx, release))
	require.NoError(t, svc.Start(ctx, release.ID))

	err := svc.Promote(ctx, release.ID, 50)
	require.NoError(t, err)

	updated, err := svc.GetByID(ctx, release.ID)
	require.NoError(t, err)
	assert.Equal(t, 50, updated.TrafficPercent)
}

func TestPause_RollingOutToPaused(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	require.NoError(t, svc.Create(ctx, release))
	require.NoError(t, svc.Start(ctx, release.ID))

	err := svc.Pause(ctx, release.ID)
	require.NoError(t, err)

	updated, err := svc.GetByID(ctx, release.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ReleasePaused, updated.Status)
}

func TestRollback_SetsRolledBack(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	require.NoError(t, svc.Create(ctx, release))
	require.NoError(t, svc.Start(ctx, release.ID))

	err := svc.Rollback(ctx, release.ID)
	require.NoError(t, err)

	updated, err := svc.GetByID(ctx, release.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ReleaseRolledBack, updated.Status)
	assert.NotNil(t, updated.CompletedAt)
}

func TestComplete_SetsCompleted(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	require.NoError(t, svc.Create(ctx, release))
	require.NoError(t, svc.Start(ctx, release.ID))

	err := svc.Complete(ctx, release.ID)
	require.NoError(t, err)

	updated, err := svc.GetByID(ctx, release.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ReleaseCompleted, updated.Status)
	assert.NotNil(t, updated.CompletedAt)
}

func TestDelete_DraftOnly(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	require.NoError(t, svc.Create(ctx, release))

	// Can delete draft
	err := svc.Delete(ctx, release.ID)
	require.NoError(t, err)

	_, err = svc.GetByID(ctx, release.ID)
	require.Error(t, err)
}

func TestDelete_NonDraftFails(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	require.NoError(t, svc.Create(ctx, release))
	require.NoError(t, svc.Start(ctx, release.ID))

	err := svc.Delete(ctx, release.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only draft releases can be deleted")
}

// ---------------------------------------------------------------------------
// Flag change tests
// ---------------------------------------------------------------------------

func TestAddFlagChange_Valid(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	fc := &models.ReleaseFlagChange{
		ReleaseID:     uuid.New(),
		FlagID:        uuid.New(),
		EnvironmentID: uuid.New(),
	}
	err := svc.AddFlagChange(ctx, fc)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, fc.ID)
}

func TestListFlagChanges(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	releaseID := uuid.New()
	fc := &models.ReleaseFlagChange{
		ReleaseID:     releaseID,
		FlagID:        uuid.New(),
		EnvironmentID: uuid.New(),
	}
	require.NoError(t, svc.AddFlagChange(ctx, fc))

	changes, err := svc.ListFlagChanges(ctx, releaseID)
	require.NoError(t, err)
	assert.Len(t, changes, 1)
}
