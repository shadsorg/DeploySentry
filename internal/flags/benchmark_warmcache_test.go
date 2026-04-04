package flags

import (
	"context"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

func BenchmarkWarmCache(b *testing.B) {
	ctx := context.Background()
	projectID := uuid.New()

	repo := &mockFlagRepo{
		flags: make(map[uuid.UUID]*models.FeatureFlag),
		rules: make(map[uuid.UUID][]*models.TargetingRule),
	}

	// Create 100 active flags
	for i := 0; i < 100; i++ {
		flag := &models.FeatureFlag{
			ID:        uuid.New(),
			ProjectID: projectID,
			Enabled:   true,
		}
		repo.flags[flag.ID] = flag

		// 3 rules per flag
		for j := 0; j < 3; j++ {
			repo.rules[flag.ID] = append(repo.rules[flag.ID], &models.TargetingRule{
				ID:     uuid.New(),
				FlagID: flag.ID,
			})
		}
	}

	service := NewFlagService(repo, newMockCache(), nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := service.WarmCache(ctx, projectID)
		if err != nil {
			b.Fatal(err)
		}
	}
}
