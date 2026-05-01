package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/staging"
)

// stageTestSetup ensures staging requires fresh users + organizations rows
// because the FKs are NOT DEFERRABLE. Returns (userID, orgID) ready for use.
func stageTestSetup(t *testing.T, ctx context.Context, repo *StagedChangesRepository) (uuid.UUID, uuid.UUID) {
	t.Helper()
	userID := uuid.New()
	orgID := uuid.New()
	// Insert minimal user + org rows the FKs will accept. Use a name field
	// that's NOT NULL on most schemas; coalesce to a sane default.
	_, err := repo.pool.Exec(ctx,
		`INSERT INTO users (id, email, name, password_hash) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (id) DO NOTHING`,
		userID, userID.String()+"@e.test", "stage-test", "x")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	_, err = repo.pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO NOTHING`,
		orgID, "stage-test-"+orgID.String()[:8], "stage-test-"+orgID.String()[:8])
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	t.Cleanup(func() {
		// FK on staged_changes is ON DELETE CASCADE, so deleting user/org
		// also drops the test rows.
		_, _ = repo.pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, orgID)
		_, _ = repo.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	return userID, orgID
}

func TestStagedChangesRepository_UpsertAndList(t *testing.T) {
	pool := testDB(t)
	repo := NewStagedChangesRepository(pool)
	ctx := context.Background()
	userID, orgID := stageTestSetup(t, ctx, repo)

	resourceID := uuid.New()
	row := &models.StagedChange{
		UserID:       userID,
		OrgID:        orgID,
		ResourceType: "flag",
		ResourceID:   &resourceID,
		Action:       "toggle",
		NewValue:     json.RawMessage(`{"enabled":true}`),
	}
	if err := repo.Upsert(ctx, row); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	rows, err := repo.ListForUser(ctx, userID, orgID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].ResourceType != "flag" || rows[0].Action != "toggle" {
		t.Fatalf("row mismatch: %+v", rows[0])
	}
}

func TestStagedChangesRepository_UpsertCollapsesSameField(t *testing.T) {
	pool := testDB(t)
	repo := NewStagedChangesRepository(pool)
	ctx := context.Background()
	userID, orgID := stageTestSetup(t, ctx, repo)

	resourceID := uuid.New()
	for _, val := range []string{`"first"`, `"second"`, `"third"`} {
		row := &models.StagedChange{
			UserID:       userID,
			OrgID:        orgID,
			ResourceType: "flag",
			ResourceID:   &resourceID,
			Action:       "update",
			FieldPath:    "name",
			NewValue:     json.RawMessage(val),
		}
		if err := repo.Upsert(ctx, row); err != nil {
			t.Fatalf("upsert %s: %v", val, err)
		}
	}

	rows, err := repo.ListForUser(ctx, userID, orgID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected upserts to collapse to 1 row, got %d", len(rows))
	}
	if string(rows[0].NewValue) != `"third"` {
		t.Fatalf("expected latest value 'third', got %s", rows[0].NewValue)
	}
}

func TestStagedChangesRepository_ProvisionalCreatesCoexist(t *testing.T) {
	pool := testDB(t)
	repo := NewStagedChangesRepository(pool)
	ctx := context.Background()
	userID, orgID := stageTestSetup(t, ctx, repo)

	for i := 0; i < 3; i++ {
		prov := staging.NewProvisional()
		row := &models.StagedChange{
			UserID:        userID,
			OrgID:         orgID,
			ResourceType:  "flag",
			ProvisionalID: &prov,
			Action:        "create",
			NewValue:      json.RawMessage(`{"name":"x"}`),
		}
		if err := repo.Upsert(ctx, row); err != nil {
			t.Fatalf("provisional create %d: %v", i, err)
		}
	}
	rows, err := repo.ListForUser(ctx, userID, orgID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 distinct creates, got %d", len(rows))
	}
}

func TestStagedChangesRepository_DeleteOlderThan(t *testing.T) {
	pool := testDB(t)
	repo := NewStagedChangesRepository(pool)
	ctx := context.Background()
	userID, orgID := stageTestSetup(t, ctx, repo)

	resourceID := uuid.New()
	row := &models.StagedChange{
		UserID:       userID,
		OrgID:        orgID,
		ResourceType: "flag",
		ResourceID:   &resourceID,
		Action:       "toggle",
		NewValue:     json.RawMessage(`{"enabled":true}`),
	}
	if err := repo.Upsert(ctx, row); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Backdate the row so DeleteOlderThan finds it.
	if _, err := pool.Exec(ctx, `UPDATE staged_changes SET created_at = $1 WHERE id = $2`,
		time.Now().Add(-31*24*time.Hour), row.ID); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	deleted, err := repo.DeleteOlderThan(ctx, time.Now().Add(-30*24*time.Hour))
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 row deleted, got %d", deleted)
	}
}

func TestStagedChangesRepository_GetByIDsScopesToUserOrg(t *testing.T) {
	pool := testDB(t)
	repo := NewStagedChangesRepository(pool)
	ctx := context.Background()
	userA, orgA := stageTestSetup(t, ctx, repo)
	userB, _ := stageTestSetup(t, ctx, repo)

	rid := uuid.New()
	rowA := &models.StagedChange{
		UserID: userA, OrgID: orgA, ResourceType: "flag", ResourceID: &rid,
		Action: "toggle", NewValue: json.RawMessage(`{"enabled":true}`),
	}
	if err := repo.Upsert(ctx, rowA); err != nil {
		t.Fatalf("upsert A: %v", err)
	}

	// userB asking for userA's row id must return nothing.
	got, err := repo.GetByIDs(ctx, userB, orgA, []uuid.UUID{rowA.ID})
	if err != nil {
		t.Fatalf("get by ids: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected GetByIDs to refuse cross-user access, got %d rows", len(got))
	}
}
