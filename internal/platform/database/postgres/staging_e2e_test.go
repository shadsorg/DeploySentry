package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shadsorg/deploysentry/internal/flags"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/staging"
)

// seedProject inserts a minimal projects row and registers cleanup. Returns projectID.
func seedProject(t *testing.T, ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID) uuid.UUID {
	t.Helper()
	projectID := uuid.New()
	slug := "e2e-proj-" + projectID.String()[:8]
	_, err := pool.Exec(ctx,
		`INSERT INTO projects (id, org_id, name, slug, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (id) DO NOTHING`,
		projectID, orgID, "e2e-project", slug, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id = $1`, projectID)
	})
	return projectID
}

// seedEnvironment inserts a minimal environments row scoped to the org. Returns envID.
func seedEnvironment(t *testing.T, ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID) uuid.UUID {
	t.Helper()
	envID := uuid.New()
	slug := "e2e-env-" + envID.String()[:8]
	_, err := pool.Exec(ctx,
		`INSERT INTO environments (id, org_id, name, slug, is_production, sort_order, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (id) DO NOTHING`,
		envID, orgID, "e2e-environment", slug, false, 0, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		t.Fatalf("seed environment: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM environments WHERE id = $1`, envID)
	})
	return envID
}

// TestE2E_FlagCreateProvisionalResolution stages a flag.create with a
// provisional id, commits it through the real staging service wired to real
// flag + staging repositories, and asserts that:
//   - the flag lands in feature_flags with a non-provisional real id
//   - staged_changes is empty for the user/org after commit
func TestE2E_FlagCreateProvisionalResolution(t *testing.T) {
	pool := testDB(t) // skips when DS_TEST_DATABASE_DSN is unset or unreachable
	ctx := context.Background()

	// Seed user + org (reuse existing staging test helper).
	stagingRepo := NewStagedChangesRepository(pool)
	userID, orgID := stageTestSetup(t, ctx, stagingRepo)

	// Seed project + environment to satisfy FK constraints on feature_flags.
	projectID := seedProject(t, ctx, pool, orgID)
	envID := seedEnvironment(t, ctx, pool, orgID)

	// Wire real services against the same pool.
	flagRepo := NewFlagRepository(pool)
	flagSvc := flags.NewFlagService(pool, flagRepo, nil, nil)

	commitReg := staging.NewCommitRegistry()
	for _, tup := range flags.FlagCommitHandlers(flagSvc) {
		commitReg.Register(tup.ResourceType, tup.Action, tup.Handler)
	}
	createReg := staging.NewCreateRegistry()
	for _, tup := range flags.FlagCreateHandlers(flagSvc) {
		createReg.Register(tup.ResourceType, tup.Action, tup.Handler)
	}
	stagingSvc := staging.NewService(stagingRepo, commitReg, createReg, pool, nil)

	// Build a staged flag.create with a provisional id.
	provFlag := staging.NewProvisional()
	flagPayload, err := json.Marshal(map[string]any{
		"key":            "e2e-provisional-flag",
		"name":           "E2E Provisional Flag",
		"project_id":     projectID.String(),
		"environment_id": envID.String(),
		"flag_type":      "boolean",
		"default_value":  "false",
		"category":       "feature",
		"created_by":     userID.String(),
	})
	if err != nil {
		t.Fatalf("marshal flag payload: %v", err)
	}

	flagRow := &models.StagedChange{
		ID:            uuid.New(),
		UserID:        userID,
		OrgID:         orgID,
		ResourceType:  "flag",
		Action:        "create",
		ProvisionalID: &provFlag,
		NewValue:      flagPayload,
	}
	if err := stagingSvc.Stage(ctx, flagRow); err != nil {
		t.Fatalf("stage flag.create: %v", err)
	}

	// Commit the staged flag.create.
	res, err := stagingSvc.Commit(ctx, userID, orgID, userID, []uuid.UUID{flagRow.ID})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if res.FailedID != nil {
		t.Fatalf("commit failed: id=%v reason=%s", res.FailedID, res.FailedReason)
	}
	if len(res.CommittedIDs) != 1 {
		t.Fatalf("expected 1 committed id, got %d: %v", len(res.CommittedIDs), res.CommittedIDs)
	}

	// Assert: flag exists in production with a non-provisional real id.
	var realFlagID uuid.UUID
	err = pool.QueryRow(ctx,
		`SELECT id FROM feature_flags WHERE key = $1 AND project_id = $2`,
		"e2e-provisional-flag", projectID).Scan(&realFlagID)
	if err != nil {
		t.Fatalf("query feature_flags: %v", err)
	}
	if realFlagID == provFlag {
		t.Fatal("real flag id equals provisional id — resolution did not happen")
	}
	if staging.IsProvisional(realFlagID) {
		t.Fatal("real flag id still carries the provisional variant byte")
	}

	// Assert: staged_changes empty for this user/org after commit.
	var stagedCount int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM staged_changes WHERE user_id = $1 AND org_id = $2`,
		userID, orgID).Scan(&stagedCount)
	if err != nil {
		t.Fatalf("count staged_changes: %v", err)
	}
	if stagedCount != 0 {
		t.Errorf("expected 0 staged rows after commit, got %d", stagedCount)
	}

	// Cleanup: remove the flag that was committed into production.
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM feature_flags WHERE key = $1 AND project_id = $2`,
			"e2e-provisional-flag", projectID)
	})
}
