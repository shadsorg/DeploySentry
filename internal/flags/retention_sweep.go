package flags

import (
	"context"
	"log"
	"time"
)

// RetentionSweeper periodically tombstones flags whose delete_after has elapsed.
// It runs as a goroutine started by cmd/api/main.go.
type RetentionSweeper struct {
	svc       FlagService
	repo      FlagRepository
	interval  time.Duration
	retention time.Duration
	batchSize int
}

// NewRetentionSweeper constructs a sweeper with the given interval and retention
// window. interval controls how often the sweep wakes up; retention is the
// archive-to-tombstone window (passed through to repo.HardDeleteFlag for the
// SQL guard).
func NewRetentionSweeper(svc FlagService, repo FlagRepository, interval, retention time.Duration) *RetentionSweeper {
	return &RetentionSweeper{
		svc:       svc,
		repo:      repo,
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

// sweepOnce fetches the next batch of flags eligible for tombstoning and
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
	log.Printf("retention_sweep: tombstoning %d flag(s)", len(ids))
	for _, id := range ids {
		if err := s.svc.HardDeleteFlag(ctx, id, s.retention); err != nil {
			log.Printf("retention_sweep: tombstone %s failed: %v", id, err)
		}
	}
}
