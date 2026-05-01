package staging

import (
	"context"
	"log"
	"time"
)

// Sweeper periodically deletes staged_changes rows older than retention.
// Mirrors internal/flags/RetentionSweeper. Run as a goroutine started from
// cmd/api/main.go.
type Sweeper struct {
	repo      Repository
	interval  time.Duration
	retention time.Duration
}

// NewSweeper builds a sweeper with the given interval (how often it wakes)
// and retention (rows older than now-retention are deleted).
func NewSweeper(repo Repository, interval, retention time.Duration) *Sweeper {
	return &Sweeper{repo: repo, interval: interval, retention: retention}
}

// Run blocks until ctx is cancelled, sweeping once on startup and then every
// interval.
func (s *Sweeper) Run(ctx context.Context) {
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

func (s *Sweeper) sweepOnce(ctx context.Context) {
	cutoff := time.Now().UTC().Add(-s.retention)
	n, err := s.repo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		log.Printf("staging.sweep: delete failed: %v", err)
		return
	}
	if n > 0 {
		log.Printf("staging.sweep: deleted %d staged row(s) older than %s", n, cutoff.Format(time.RFC3339))
	}
}
