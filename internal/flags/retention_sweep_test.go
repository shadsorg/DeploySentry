package flags

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

type stubAuditWriter struct {
	mu      sync.Mutex
	entries []*models.AuditLogEntry
}

func (s *stubAuditWriter) WriteAuditLog(_ context.Context, entry *models.AuditLogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry)
	return nil
}

func TestRetentionSweeper_SweepsExpiredFlags(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()

	repo := newMockFlagRepo()
	repo.listFlagsToHardDeleteFn = func(_ context.Context, _ int) ([]uuid.UUID, error) {
		return []uuid.UUID{id1, id2}, nil
	}

	var hardDeleted []uuid.UUID
	svcRepo := newMockFlagRepo()
	svcRepo.flags[id1] = &models.FeatureFlag{ID: id1, Key: "flag-1"}
	svcRepo.flags[id2] = &models.FeatureFlag{ID: id2, Key: "flag-2"}
	svcRepo.hardDeleteFlagFn = func(_ context.Context, id uuid.UUID, _ time.Duration) error {
		hardDeleted = append(hardDeleted, id)
		return nil
	}
	svc := NewFlagService(svcRepo, newMockCache(), nil)
	audit := &stubAuditWriter{}

	sweeper := NewRetentionSweeper(svc, repo, audit, 1*time.Hour, 30*24*time.Hour)
	sweeper.sweepOnce(context.Background())

	if len(hardDeleted) != 2 {
		t.Fatalf("expected 2 deletions, got %d", len(hardDeleted))
	}
	if hardDeleted[0] != id1 || hardDeleted[1] != id2 {
		t.Errorf("unexpected ids: %v", hardDeleted)
	}
	if got := len(audit.entries); got != 2 {
		t.Fatalf("expected 2 audit entries, got %d", got)
	}
	for _, e := range audit.entries {
		if e.Action != "flag.hard_deleted" {
			t.Errorf("expected action flag.hard_deleted, got %q", e.Action)
		}
		if e.ActorID != uuid.Nil {
			t.Errorf("expected system actor (uuid.Nil), got %s", e.ActorID)
		}
		if e.OldValue == "" {
			t.Errorf("expected non-empty old_value capturing flag state")
		}
	}
}

func TestRetentionSweeper_NoOpWhenEmpty(t *testing.T) {
	repo := newMockFlagRepo()
	repo.listFlagsToHardDeleteFn = func(_ context.Context, _ int) ([]uuid.UUID, error) {
		return nil, nil
	}
	svc := NewFlagService(newMockFlagRepo(), newMockCache(), nil)
	audit := &stubAuditWriter{}
	sweeper := NewRetentionSweeper(svc, repo, audit, 1*time.Hour, 30*24*time.Hour)
	sweeper.sweepOnce(context.Background())
	if len(audit.entries) != 0 {
		t.Errorf("expected no audit entries on empty sweep, got %d", len(audit.entries))
	}
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
	svcRepo.flags[id1] = &models.FeatureFlag{ID: id1, Key: "flag-1"}
	svcRepo.flags[id2] = &models.FeatureFlag{ID: id2, Key: "flag-2"}
	svcRepo.flags[id3] = &models.FeatureFlag{ID: id3, Key: "flag-3"}
	svcRepo.hardDeleteFlagFn = func(_ context.Context, id uuid.UUID, _ time.Duration) error {
		attempted = append(attempted, id)
		if id == id2 {
			return errors.New("simulated failure")
		}
		return nil
	}
	svc := NewFlagService(svcRepo, newMockCache(), nil)
	audit := &stubAuditWriter{}

	sweeper := NewRetentionSweeper(svc, repo, audit, 1*time.Hour, 30*24*time.Hour)
	sweeper.sweepOnce(context.Background())

	if len(attempted) != 3 {
		t.Errorf("expected 3 attempts (failure should not stop the loop), got %d", len(attempted))
	}
	if got := len(audit.entries); got != 2 {
		t.Errorf("expected 2 audit entries (only successes), got %d", got)
	}
}
