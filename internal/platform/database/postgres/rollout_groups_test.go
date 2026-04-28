package postgres

import (
	"context"
	"testing"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

func sampleGroup(orgID uuid.UUID) *models.RolloutGroup {
	return &models.RolloutGroup{
		ScopeType:          models.ScopeOrg,
		ScopeID:            orgID,
		Name:               "v1.1-" + uuid.NewString()[:8],
		Description:        "test bundle",
		CoordinationPolicy: models.CoordinationIndependent,
	}
}

func TestRolloutGroupRepo_CreateGet(t *testing.T) {
	ctx := context.Background()
	repo := NewRolloutGroupRepo(testDB(t))
	g := sampleGroup(uuid.New())
	if err := repo.Create(ctx, g); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.Get(ctx, g.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != g.Name || got.CoordinationPolicy != models.CoordinationIndependent {
		t.Fatalf("mismatch: %+v", got)
	}
}

func TestRolloutGroupRepo_UpdatePolicy(t *testing.T) {
	ctx := context.Background()
	repo := NewRolloutGroupRepo(testDB(t))
	g := sampleGroup(uuid.New())
	if err := repo.Create(ctx, g); err != nil {
		t.Fatal(err)
	}
	g.CoordinationPolicy = models.CoordinationCascadeAbort
	g.Description = "updated"
	if err := repo.Update(ctx, g); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.Get(ctx, g.ID)
	if got.CoordinationPolicy != models.CoordinationCascadeAbort || got.Description != "updated" {
		t.Fatalf("update not applied: %+v", got)
	}
}

func TestRolloutGroupRepo_ListByScope(t *testing.T) {
	ctx := context.Background()
	repo := NewRolloutGroupRepo(testDB(t))
	orgID := uuid.New()
	g1 := sampleGroup(orgID)
	g2 := sampleGroup(orgID)
	g3 := sampleGroup(uuid.New())
	if err := repo.Create(ctx, g1); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, g2); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, g3); err != nil {
		t.Fatal(err)
	}
	got, err := repo.ListByScope(ctx, models.ScopeOrg, orgID)
	if err != nil || len(got) != 2 {
		t.Fatalf("list: err=%v len=%d", err, len(got))
	}
}

func TestRolloutGroupRepo_Delete(t *testing.T) {
	ctx := context.Background()
	repo := NewRolloutGroupRepo(testDB(t))
	g := sampleGroup(uuid.New())
	if err := repo.Create(ctx, g); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, g.ID); err == nil {
		t.Fatalf("expected not-found after delete")
	}
}
