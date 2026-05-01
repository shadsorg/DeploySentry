package staging

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shadsorg/deploysentry/internal/models"
)

// fakeRepo is the smallest possible Repository for sweeper unit tests. It
// records the cutoff passed to DeleteOlderThan and returns a configurable
// row count.
type fakeRepo struct {
	deletedCount  int64
	deleteErr     error
	receivedCutoff time.Time
}

func (f *fakeRepo) Upsert(context.Context, *models.StagedChange) error      { return nil }
func (f *fakeRepo) ListForUser(context.Context, uuid.UUID, uuid.UUID) ([]*models.StagedChange, error) { return nil, nil }
func (f *fakeRepo) ListForResource(context.Context, uuid.UUID, uuid.UUID, string) ([]*models.StagedChange, error) { return nil, nil }
func (f *fakeRepo) GetByIDs(context.Context, uuid.UUID, uuid.UUID, []uuid.UUID) ([]*models.StagedChange, error) { return nil, nil }
func (f *fakeRepo) DeleteByIDsTx(context.Context, pgx.Tx, uuid.UUID, uuid.UUID, []uuid.UUID) error { return nil }
func (f *fakeRepo) DeleteAllForUser(context.Context, uuid.UUID, uuid.UUID) (int64, error) { return 0, nil }
func (f *fakeRepo) CountForUser(context.Context, uuid.UUID, uuid.UUID) (int, error) { return 0, nil }

func (f *fakeRepo) DeleteOlderThan(_ context.Context, cutoff time.Time) (int64, error) {
	f.receivedCutoff = cutoff
	return f.deletedCount, f.deleteErr
}

func TestSweeper_UsesRetentionWindow(t *testing.T) {
	repo := &fakeRepo{deletedCount: 3}
	s := NewSweeper(repo, time.Hour, 30*24*time.Hour)
	before := time.Now().UTC()
	s.sweepOnce(context.Background())
	after := time.Now().UTC()

	expectedMin := before.Add(-30 * 24 * time.Hour)
	expectedMax := after.Add(-30 * 24 * time.Hour)
	if repo.receivedCutoff.Before(expectedMin) || repo.receivedCutoff.After(expectedMax) {
		t.Fatalf("cutoff outside expected window: got %s, want between %s and %s",
			repo.receivedCutoff, expectedMin, expectedMax)
	}
}
