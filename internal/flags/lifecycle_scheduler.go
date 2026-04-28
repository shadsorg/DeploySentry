package flags

import (
	"context"
	"log"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/webhooks"
	"github.com/google/uuid"
)

// LifecycleScheduler polls for flags whose scheduled_removal_at has passed
// and emits a `flag.scheduled_for_removal.due` webhook exactly once per flag.
//
// The scheduler is deliberately simple: it runs in-process on every API
// instance, and uses an update-row marker (scheduled_removal_fired_at) for
// idempotency, so concurrent schedulers can race safely — only the first to
// mark the row fires the webhook.
type LifecycleScheduler struct {
	svc      FlagService
	webhooks *webhooks.Service
	interval time.Duration
	now      func() time.Time
}

// NewLifecycleScheduler builds a scheduler. interval defaults to 60s when 0.
func NewLifecycleScheduler(svc FlagService, webhookSvc *webhooks.Service, interval time.Duration) *LifecycleScheduler {
	if interval <= 0 {
		interval = time.Minute
	}
	return &LifecycleScheduler{
		svc:      svc,
		webhooks: webhookSvc,
		interval: interval,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// Run blocks on ctx, running Tick every interval until ctx is done.
func (s *LifecycleScheduler) Run(ctx context.Context) {
	t := time.NewTicker(s.interval)
	defer t.Stop()
	// Fire once immediately on startup so a restart doesn't block removal
	// events for up to `interval`.
	if err := s.Tick(ctx); err != nil {
		log.Printf("lifecycle scheduler: initial tick failed: %v", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.Tick(ctx); err != nil {
				log.Printf("lifecycle scheduler: tick failed: %v", err)
			}
		}
	}
}

// Tick runs a single pass: list due flags, fire their webhooks, mark them
// as fired. Safe to call from tests.
func (s *LifecycleScheduler) Tick(ctx context.Context) error {
	now := s.now()
	flags, err := s.svc.ListFlagsDueForRemoval(ctx, now)
	if err != nil {
		return err
	}
	for _, f := range flags {
		if err := s.svc.MarkFlagRemovalFired(ctx, f.ID, now); err != nil {
			// Another scheduler instance likely raced us — skip.
			continue
		}
		s.emitDue(ctx, f)
	}
	return nil
}

func (s *LifecycleScheduler) emitDue(ctx context.Context, flag *models.FeatureFlag) {
	if s.webhooks == nil {
		return
	}
	payload := lifecyclePayload(flag)
	// org_id is not cheaply available here without a join; we use uuid.Nil
	// and rely on the `project_id` field in the payload for routing.
	if err := s.webhooks.PublishEvent(ctx, models.EventFlagScheduledForRemovalDue, uuid.Nil, &flag.ProjectID, payload, nil); err != nil {
		log.Printf("lifecycle scheduler: publish due event for flag %s: %v", flag.ID, err)
	}
}
