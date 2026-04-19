package flags

import (
	"context"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLifecycleScheduler_Tick_MarksFiredOnce verifies the scheduler marks a
// due flag as fired so a follow-up tick doesn't re-emit the webhook.
func TestLifecycleScheduler_Tick_MarksFiredOnce(t *testing.T) {
	repo := newMockFlagRepo()
	cache := newMockCache()
	svc := NewFlagService(repo, cache, nil)

	flag := &models.FeatureFlag{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Key:       "retire-me",
		Name:      "Retire Me",
		FlagType:  models.FlagTypeBoolean,
	}
	due := time.Now().UTC().Add(-time.Second)
	flag.ScheduledRemovalAt = &due
	repo.flags[flag.ID] = flag

	sched := NewLifecycleScheduler(svc, nil, time.Minute)

	// Override the fire-marker mock: once marked, subsequent ticks must skip.
	var markCalls int
	originalRepo := repo
	wrappedRepo := &markCountingRepo{mockFlagRepo: originalRepo, markCount: &markCalls}
	svc = NewFlagService(wrappedRepo, cache, nil)
	sched = NewLifecycleScheduler(svc, nil, time.Minute)

	require.NoError(t, sched.Tick(context.Background()))
	require.NoError(t, sched.Tick(context.Background()))

	assert.Equal(t, 1, markCalls, "second tick must not re-mark an already-fired flag")
}

// markCountingRepo counts MarkFlagRemovalFired invocations and simulates the
// real repo's idempotency: after the first call, subsequent calls on the same
// flag ID return ErrNotFound (mimicking the partial-index filter).
type markCountingRepo struct {
	*mockFlagRepo
	markCount *int
	fired     map[uuid.UUID]bool
}

func (r *markCountingRepo) MarkFlagRemovalFired(ctx context.Context, id uuid.UUID, firedAt time.Time) error {
	if r.fired == nil {
		r.fired = make(map[uuid.UUID]bool)
	}
	if r.fired[id] {
		// Idempotent — simulate the partial-index constraint: zero rows
		// affected triggers ErrNotFound in the real repo, so callers skip.
		return errNotFoundStub
	}
	*r.markCount++
	r.fired[id] = true
	// Flip the flag's scheduled_removal so ListFlagsDueForRemoval also excludes it.
	if f, ok := r.mockFlagRepo.flags[id]; ok {
		f.ScheduledRemovalAt = nil
	}
	return nil
}

type stubErr string

func (e stubErr) Error() string { return string(e) }

const errNotFoundStub stubErr = "not found"
