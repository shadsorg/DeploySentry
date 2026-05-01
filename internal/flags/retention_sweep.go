package flags

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// AuditWriter is the same interface the HTTP handler uses; redeclared here
// to avoid a circular import. cmd/api/main.go passes the same concrete
// repository to both.
type retentionAuditWriter interface {
	WriteAuditLog(ctx context.Context, entry *models.AuditLogEntry) error
}

// RetentionSweeper periodically hard-deletes flags whose delete_after has elapsed.
// It runs as a goroutine started by cmd/api/main.go and writes a system
// audit row (actor_id = uuid.Nil) for each successful deletion.
type RetentionSweeper struct {
	svc       FlagService
	repo      FlagRepository
	audit     retentionAuditWriter
	interval  time.Duration
	retention time.Duration
	batchSize int
}

// NewRetentionSweeper constructs a sweeper with the given interval and retention
// window. interval controls how often the sweep wakes up; retention is the
// archive-to-delete window (passed through to repo.HardDeleteFlag for the SQL guard).
// audit may be nil — when provided, each successful deletion writes a
// flag.hard_deleted audit row with the deleted flag's prior state in old_value.
func NewRetentionSweeper(svc FlagService, repo FlagRepository, audit retentionAuditWriter, interval, retention time.Duration) *RetentionSweeper {
	return &RetentionSweeper{
		svc:       svc,
		repo:      repo,
		audit:     audit,
		interval:  interval,
		retention: retention,
		batchSize: 100,
	}
}

// Run blocks until ctx is cancelled. Sweeps once at startup, then every interval.
func (s *RetentionSweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	s.sweepOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweepOnce(ctx)
		}
	}
}

// sweepOnce fetches the next batch of flags eligible for hard-delete and
// calls service.HardDeleteFlag on each. Errors are logged; a single failure
// doesn't stop the rest of the batch.
func (s *RetentionSweeper) sweepOnce(ctx context.Context) {
	ids, err := s.repo.ListFlagsToHardDelete(ctx, s.batchSize)
	if err != nil {
		log.Printf("retention_sweep: list failed: %v", err)
		return
	}
	if len(ids) == 0 {
		return
	}
	log.Printf("retention_sweep: hard-deleting %d flag(s)", len(ids))
	for _, id := range ids {
		// Capture flag state before deletion for the audit row.
		var oldValBytes []byte
		if flag, err := s.svc.GetFlag(ctx, id); err == nil && flag != nil {
			oldValBytes, _ = json.Marshal(flag)
		}

		if err := s.svc.HardDeleteFlag(ctx, id, s.retention); err != nil {
			log.Printf("retention_sweep: hard-delete %s failed: %v", id, err)
			continue
		}

		if s.audit != nil {
			entry := &models.AuditLogEntry{
				ActorID:    uuid.Nil,
				Action:     "flag.hard_deleted",
				EntityType: "flag",
				EntityID:   id,
				OldValue:   string(oldValBytes),
				CreatedAt:  time.Now(),
			}
			if err := s.audit.WriteAuditLog(ctx, entry); err != nil {
				log.Printf("retention_sweep: audit write for %s failed: %v", id, err)
			}
		}
	}
}
