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
	releases     map[uuid.UUID]*models.Release
	releaseEnvs  []*models.ReleaseEnvironment
	createErr    error
	updateErr    error
	createEnvErr error
}

func newMockRepo() *mockReleaseRepo {
	return &mockReleaseRepo{
		releases: make(map[uuid.UUID]*models.Release),
	}
}

func (m *mockReleaseRepo) CreateRelease(_ context.Context, release *models.Release) error {
	if m.createErr != nil {
		return m.createErr
	}
	stored := *release
	m.releases[release.ID] = &stored
	return nil
}

func (m *mockReleaseRepo) GetRelease(_ context.Context, id uuid.UUID) (*models.Release, error) {
	r, ok := m.releases[id]
	if !ok {
		return nil, errors.New("release not found")
	}
	copy := *r
	return &copy, nil
}

func (m *mockReleaseRepo) ListReleases(_ context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.Release, error) {
	var result []*models.Release
	for _, r := range m.releases {
		if r.ProjectID == projectID {
			if opts.Status != nil && r.Status != *opts.Status {
				continue
			}
			copy := *r
			result = append(result, &copy)
		}
	}
	// Apply simple limit/offset.
	if opts.Offset > 0 && opts.Offset < len(result) {
		result = result[opts.Offset:]
	} else if opts.Offset >= len(result) {
		result = nil
	}
	if opts.Limit > 0 && opts.Limit < len(result) {
		result = result[:opts.Limit]
	}
	return result, nil
}

func (m *mockReleaseRepo) UpdateRelease(_ context.Context, release *models.Release) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	stored := *release
	m.releases[release.ID] = &stored
	return nil
}

func (m *mockReleaseRepo) CreateReleaseEnvironment(_ context.Context, re *models.ReleaseEnvironment) error {
	if m.createEnvErr != nil {
		return m.createEnvErr
	}
	copy := *re
	m.releaseEnvs = append(m.releaseEnvs, &copy)
	return nil
}

func (m *mockReleaseRepo) ListReleaseEnvironments(_ context.Context, releaseID uuid.UUID) ([]*models.ReleaseEnvironment, error) {
	var result []*models.ReleaseEnvironment
	for _, re := range m.releaseEnvs {
		if re.ReleaseID == releaseID {
			copy := *re
			result = append(result, &copy)
		}
	}
	return result, nil
}

func (m *mockReleaseRepo) UpdateReleaseEnvironment(_ context.Context, re *models.ReleaseEnvironment) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	for i, existing := range m.releaseEnvs {
		if existing.ID == re.ID {
			copy := *re
			m.releaseEnvs[i] = &copy
			return nil
		}
	}
	return errors.New("release environment not found")
}

// validRelease returns a fully populated Release with all required fields set.
func validRelease() *models.Release {
	return &models.Release{
		ProjectID: uuid.New(),
		Version:   "1.0.0",
		Title:     "Initial Release",
		Artifact:  "app:1.0.0",
		CreatedBy: uuid.New(),
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
	assert.Equal(t, models.ReleaseStatusDraft, release.Status, "Create should set status to draft")
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

func TestCreate_PreservesExistingID(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	presetID := uuid.New()
	release := validRelease()
	release.ID = presetID

	err := svc.Create(ctx, release)
	require.NoError(t, err)
	assert.Equal(t, presetID, release.ID, "Create should preserve a pre-set non-nil ID")
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
			name:    "missing project_id",
			modify:  func(r *models.Release) { r.ProjectID = uuid.Nil },
			wantMsg: "project_id is required",
		},
		{
			name:    "missing version",
			modify:  func(r *models.Release) { r.Version = "" },
			wantMsg: "version is required",
		},
		{
			name:    "missing title",
			modify:  func(r *models.Release) { r.Title = "" },
			wantMsg: "title is required",
		},
		{
			name:    "missing artifact",
			modify:  func(r *models.Release) { r.Artifact = "" },
			wantMsg: "artifact is required",
		},
		{
			name:    "missing created_by",
			modify:  func(r *models.Release) { r.CreatedBy = uuid.Nil },
			wantMsg: "created_by is required",
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
	assert.Equal(t, release.Version, stored.Version)
	assert.Equal(t, models.ReleaseStatusDraft, stored.Status)
}

// ---------------------------------------------------------------------------
// Get tests
// ---------------------------------------------------------------------------

func TestGet_ExistingRelease(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	require.NoError(t, svc.Create(ctx, release))

	fetched, err := svc.Get(ctx, release.ID)
	require.NoError(t, err)
	assert.Equal(t, release.ID, fetched.ID)
	assert.Equal(t, release.Version, fetched.Version)
	assert.Equal(t, release.Title, fetched.Title)
	assert.Equal(t, models.ReleaseStatusDraft, fetched.Status)
}

func TestGet_NotFound(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	_, err := svc.Get(ctx, uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting release")
}

// ---------------------------------------------------------------------------
// List tests
// ---------------------------------------------------------------------------

func TestList_DefaultLimit(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	projectID := uuid.New()
	// Create 25 releases for the same project.
	for i := 0; i < 25; i++ {
		r := validRelease()
		r.ProjectID = projectID
		require.NoError(t, svc.Create(ctx, r))
	}

	// Limit 0 should default to 20.
	results, err := svc.List(ctx, projectID, ListOptions{Limit: 0})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), 20, "default limit should cap results at 20")
}

func TestList_CapsAt100(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	projectID := uuid.New()
	// Create 105 releases.
	for i := 0; i < 105; i++ {
		r := validRelease()
		r.ProjectID = projectID
		require.NoError(t, svc.Create(ctx, r))
	}

	// Requesting limit 200 should be capped to 100.
	results, err := svc.List(ctx, projectID, ListOptions{Limit: 200})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), 100, "limit should be capped at 100")
}

func TestList_NegativeLimit(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	projectID := uuid.New()
	for i := 0; i < 25; i++ {
		r := validRelease()
		r.ProjectID = projectID
		require.NoError(t, svc.Create(ctx, r))
	}

	// Negative limit should default to 20.
	results, err := svc.List(ctx, projectID, ListOptions{Limit: -5})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), 20, "negative limit should default to 20")
}

func TestList_ReturnsResultsFromRepo(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	projectID := uuid.New()
	for i := 0; i < 3; i++ {
		r := validRelease()
		r.ProjectID = projectID
		require.NoError(t, svc.Create(ctx, r))
	}

	// Also create a release for a different project to verify filtering.
	other := validRelease()
	require.NoError(t, svc.Create(ctx, other))

	results, err := svc.List(ctx, projectID, ListOptions{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, results, 3, "should return exactly the releases for the given project")
	for _, r := range results {
		assert.Equal(t, projectID, r.ProjectID)
	}
}

func TestList_EmptyResults(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	results, err := svc.List(ctx, uuid.New(), ListOptions{Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, results, "listing for non-existent project should return empty slice")
}

// ---------------------------------------------------------------------------
// Promote tests
// ---------------------------------------------------------------------------

func TestPromote_DraftToActive(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	require.NoError(t, svc.Create(ctx, release))
	assert.Equal(t, models.ReleaseStatusDraft, release.Status)

	envID := uuid.New()
	deployedBy := uuid.New()
	err := svc.Promote(ctx, release.ID, envID, deployedBy)
	require.NoError(t, err)

	// Verify the release was transitioned to active.
	updated, err := svc.Get(ctx, release.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ReleaseStatusActive, updated.Status)
	assert.NotNil(t, updated.ReleasedAt, "ReleasedAt should be set after promotion")
	assert.False(t, updated.UpdatedAt.IsZero(), "UpdatedAt should be refreshed")
}

func TestPromote_CreatesReleaseEnvironment(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	require.NoError(t, svc.Create(ctx, release))

	envID := uuid.New()
	deployedBy := uuid.New()
	err := svc.Promote(ctx, release.ID, envID, deployedBy)
	require.NoError(t, err)

	// Verify the release-environment record was created.
	require.Len(t, repo.releaseEnvs, 1)
	re := repo.releaseEnvs[0]
	assert.Equal(t, release.ID, re.ReleaseID)
	assert.Equal(t, envID, re.EnvironmentID)
	assert.Equal(t, models.ReleaseStatusActive, re.Status)
	assert.NotNil(t, re.DeployedAt)
	require.NotNil(t, re.DeployedBy)
	assert.Equal(t, deployedBy, *re.DeployedBy)
	assert.NotEqual(t, uuid.Nil, re.ID, "release environment should have an assigned ID")
}

func TestPromote_AlreadyActiveSkipsTransition(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	require.NoError(t, svc.Create(ctx, release))

	// First promotion: draft -> active.
	envID1 := uuid.New()
	deployedBy := uuid.New()
	require.NoError(t, svc.Promote(ctx, release.ID, envID1, deployedBy))

	// Verify it is now active.
	updated, err := svc.Get(ctx, release.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ReleaseStatusActive, updated.Status)
	firstReleasedAt := updated.ReleasedAt

	// Second promotion: already active, should still create env record.
	envID2 := uuid.New()
	err = svc.Promote(ctx, release.ID, envID2, deployedBy)
	require.NoError(t, err)

	// Verify ReleasedAt was not overwritten (status was already active, no re-transition).
	afterSecond, err := svc.Get(ctx, release.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ReleaseStatusActive, afterSecond.Status)
	assert.Equal(t, firstReleasedAt, afterSecond.ReleasedAt, "ReleasedAt should not change on second promote")

	// Verify two environment records exist.
	assert.Len(t, repo.releaseEnvs, 2)
	assert.Equal(t, envID1, repo.releaseEnvs[0].EnvironmentID)
	assert.Equal(t, envID2, repo.releaseEnvs[1].EnvironmentID)
}

func TestPromote_ReleaseNotFound(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	err := svc.Promote(ctx, uuid.New(), uuid.New(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting release for promotion")
}

func TestPromote_NonDraftRelease_SkipsTransition(t *testing.T) {
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	require.NoError(t, svc.Create(ctx, release))

	// Manually set the release to active (non-draft) to test that Promote
	// skips the draft->active transition and still creates the env record.
	stored := repo.releases[release.ID]
	stored.Status = models.ReleaseStatusActive

	envID := uuid.New()
	deployedBy := uuid.New()
	err := svc.Promote(ctx, release.ID, envID, deployedBy)
	require.NoError(t, err)

	// Verify the release environment was created.
	assert.Len(t, repo.releaseEnvs, 1)
	assert.Equal(t, envID, repo.releaseEnvs[0].EnvironmentID)
}

func TestPromote_CreateReleaseEnvironmentFails(t *testing.T) {
	envErr := errors.New("environment table locked")
	repo := newMockRepo()
	repo.createEnvErr = envErr
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	require.NoError(t, svc.Create(ctx, release))

	// Reset createErr to nil since we only want CreateReleaseEnvironment to fail.
	repo.createErr = nil

	err := svc.Promote(ctx, release.ID, uuid.New(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating release environment")
	assert.ErrorIs(t, err, envErr)
}

func TestPromote_UpdateReleaseFails(t *testing.T) {
	updateErr := errors.New("optimistic lock conflict")
	repo := newMockRepo()
	svc := NewReleaseService(repo)
	ctx := context.Background()

	release := validRelease()
	require.NoError(t, svc.Create(ctx, release))

	// Set the update error after creation to simulate failure during promotion.
	repo.updateErr = updateErr

	err := svc.Promote(ctx, release.ID, uuid.New(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "activating release")
	assert.ErrorIs(t, err, updateErr)
}
