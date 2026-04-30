package flags

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRetentionSweeper_SweepsExpiredFlags(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()

	repo := newMockFlagRepo()
	repo.listFlagsToHardDeleteFn = func(_ context.Context, limit int) ([]uuid.UUID, error) {
		return []uuid.UUID{id1, id2}, nil
	}

	var hardDeleted []uuid.UUID
	svcRepo := newMockFlagRepo() // separate repo for the service
	svcRepo.hardDeleteFlagFn = func(_ context.Context, id uuid.UUID, _ time.Duration) error {
		hardDeleted = append(hardDeleted, id)
		return nil
	}
	svc := NewFlagService(svcRepo, newMockCache(), nil)

	sweeper := NewRetentionSweeper(svc, repo, 1*time.Hour, 30*24*time.Hour)
	sweeper.sweepOnce(context.Background())

	if len(hardDeleted) != 2 {
		t.Fatalf("expected 2 tombstones, got %d", len(hardDeleted))
	}
	if hardDeleted[0] != id1 || hardDeleted[1] != id2 {
		t.Errorf("unexpected ids: %v", hardDeleted)
	}
}

func TestRetentionSweeper_NoOpWhenEmpty(t *testing.T) {
	repo := newMockFlagRepo()
	repo.listFlagsToHardDeleteFn = func(_ context.Context, _ int) ([]uuid.UUID, error) {
		return nil, nil
	}
	svc := NewFlagService(newMockFlagRepo(), newMockCache(), nil)
	sweeper := NewRetentionSweeper(svc, repo, 1*time.Hour, 30*24*time.Hour)
	sweeper.sweepOnce(context.Background())
	// No assertion needed; just confirm it doesn't panic.
}

func TestRetentionSweeper_LogsButContinuesOnSingleFailure(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()

	repo := newMockFlagRepo()
	repo.listFlagsToHardDeleteFn = func(_ context.Context, _ int) ([]uuid.UUID, error) {
		return []uuid.UUID{id1, id2, id3}, nil
	}

	var attempted []uuid.UUID
	svcRepo := newMockFlagRepo()
	svcRepo.hardDeleteFlagFn = func(_ context.Context, id uuid.UUID, _ time.Duration) error {
		attempted = append(attempted, id)
		if id == id2 {
			return errors.New("simulated failure")
		}
		return nil
	}
	svc := NewFlagService(svcRepo, newMockCache(), nil)

	sweeper := NewRetentionSweeper(svc, repo, 1*time.Hour, 30*24*time.Hour)
	sweeper.sweepOnce(context.Background())

	if len(attempted) != 3 {
		t.Errorf("expected 3 attempts (failure should not stop the loop), got %d", len(attempted))
	}
}
