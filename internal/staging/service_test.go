package staging

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shadsorg/deploysentry/internal/models"
)

// --- fake repository (named memRepo to avoid collision with sweep_test.go's fakeRepo) ---

type memRepo struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*models.StagedChange
}

func newFakeRepo() *memRepo { return &memRepo{rows: map[uuid.UUID]*models.StagedChange{}} }

func (f *memRepo) Upsert(_ context.Context, row *models.StagedChange) error {
	if row.ID == uuid.Nil {
		row.ID = uuid.New()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows[row.ID] = row
	return nil
}

func (f *memRepo) ListForUser(_ context.Context, userID, orgID uuid.UUID) ([]*models.StagedChange, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*models.StagedChange
	for _, r := range f.rows {
		if r.UserID == userID && r.OrgID == orgID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *memRepo) ListForResource(_ context.Context, userID, orgID uuid.UUID, resourceType string) ([]*models.StagedChange, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*models.StagedChange
	for _, r := range f.rows {
		if r.UserID == userID && r.OrgID == orgID && r.ResourceType == resourceType {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *memRepo) GetByIDs(_ context.Context, userID, orgID uuid.UUID, ids []uuid.UUID) ([]*models.StagedChange, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*models.StagedChange
	for _, id := range ids {
		r, ok := f.rows[id]
		if ok && r.UserID == userID && r.OrgID == orgID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *memRepo) DeleteByIDsTx(_ context.Context, _ pgx.Tx, userID, orgID uuid.UUID, ids []uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		r, ok := f.rows[id]
		if ok && r.UserID == userID && r.OrgID == orgID {
			delete(f.rows, id)
		}
	}
	return nil
}

func (f *memRepo) DeleteAllForUser(_ context.Context, userID, orgID uuid.UUID) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for id, r := range f.rows {
		if r.UserID == userID && r.OrgID == orgID {
			delete(f.rows, id)
			n++
		}
	}
	return n, nil
}

func (f *memRepo) DeleteOlderThan(_ context.Context, _ time.Time) (int64, error) { return 0, nil }

func (f *memRepo) CountForUser(_ context.Context, userID, orgID uuid.UUID) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.rows {
		if r.UserID == userID && r.OrgID == orgID {
			n++
		}
	}
	return n, nil
}

// --- mock tx / pool ---

// mockTx implements pgx.Tx. Only Commit and Rollback are needed by Service.
type mockTx struct{}

func (t *mockTx) Begin(_ context.Context) (pgx.Tx, error)      { return &mockTx{}, nil }
func (t *mockTx) Commit(_ context.Context) error               { return nil }
func (t *mockTx) Rollback(_ context.Context) error             { return nil }
func (t *mockTx) Conn() *pgx.Conn                              { return nil }
func (t *mockTx) LargeObjects() pgx.LargeObjects               { panic("mockTx.LargeObjects not implemented") }
func (t *mockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	panic("mockTx.Exec not implemented")
}
func (t *mockTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	panic("mockTx.CopyFrom not implemented")
}
func (t *mockTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults {
	panic("mockTx.SendBatch not implemented")
}
func (t *mockTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	panic("mockTx.Prepare not implemented")
}
func (t *mockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	panic("mockTx.Query not implemented")
}
func (t *mockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	panic("mockTx.QueryRow not implemented")
}

type mockTxBeginner struct{}

func (m *mockTxBeginner) BeginTx(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
	return &mockTx{}, nil
}

// --- existing tests (stagedNewValue / buildAuditEntry) ---

func TestStagedNewValue_AnnotatesObjectInPlace(t *testing.T) {
	stagedAt := time.Date(2026, 5, 1, 10, 30, 0, 0, time.UTC)
	got := stagedNewValue([]byte(`{"enabled":true}`), stagedAt)
	if !strings.Contains(got, `"_staged_at":"2026-05-01T10:30:00Z"`) {
		t.Fatalf("expected _staged_at in object, got %s", got)
	}
	if !strings.Contains(got, `"enabled":true`) {
		t.Fatalf("expected original payload preserved, got %s", got)
	}
	// Output must be valid JSON.
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, got)
	}
}

func TestStagedNewValue_HandlesEmptyObject(t *testing.T) {
	got := stagedNewValue([]byte(`{}`), time.Now())
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("empty-object splice produced invalid JSON: %v\n%s", err, got)
	}
}

func TestStagedNewValue_WrapsNonObjectPayload(t *testing.T) {
	got := stagedNewValue([]byte(`"control"`), time.Now())
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("non-object wrap produced invalid JSON: %v\n%s", err, got)
	}
	if parsed["value"] != "control" {
		t.Fatalf("expected wrapped value=control, got %v", parsed["value"])
	}
}

func TestStagedNewValue_HandlesEmptyInput(t *testing.T) {
	got := stagedNewValue(nil, time.Now())
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("nil input produced invalid JSON: %v\n%s", err, got)
	}
	if _, ok := parsed["_staged_at"]; !ok {
		t.Fatal("expected _staged_at key for nil input")
	}
}

func TestBuildAuditEntry_PassthroughResourceID(t *testing.T) {
	rid := uuid.New()
	row := &models.StagedChange{
		ID:           uuid.New(),
		OrgID:        uuid.New(),
		ResourceType: "flag",
		ResourceID:   &rid,
		Action:       "toggle",
		NewValue:     json.RawMessage(`{"enabled":true}`),
		CreatedAt:    time.Now(),
	}
	entry := buildAuditEntry(row, uuid.New(), "flag.toggled")
	if entry.EntityID != rid {
		t.Fatalf("expected EntityID=%s, got %s", rid, entry.EntityID)
	}
	if entry.EntityType != "flag" {
		t.Fatalf("expected EntityType=flag, got %s", entry.EntityType)
	}
}

// --- new tests ---

func TestCommit_ResolvesProvisionalAcrossBatch(t *testing.T) {
	repo := newFakeRepo()
	commitReg := NewCommitRegistry()
	createReg := NewCreateRegistry()

	wantRealFlag := uuid.New()
	createReg.Register("flag", "create", func(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (uuid.UUID, string, func(context.Context), error) {
		return wantRealFlag, "flag.created", nil, nil
	})
	var seenFlagID uuid.UUID
	commitReg.Register("flag_rule", "update", func(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (string, error) {
		var p map[string]any
		_ = json.Unmarshal(row.NewValue, &p)
		seenFlagID, _ = uuid.Parse(p["flag_id"].(string))
		return "flag.rule.updated", nil
	})

	svc := &Service{repo: repo, reg: commitReg, creates: createReg, pool: &mockTxBeginner{}, audit: nil}
	user, org := uuid.New(), uuid.New()
	prov := NewProvisional()
	realRule := uuid.New()
	createRow := &models.StagedChange{ID: uuid.New(), UserID: user, OrgID: org, ResourceType: "flag", Action: "create", ProvisionalID: &prov, NewValue: []byte(`{}`)}
	mutateRow := &models.StagedChange{ID: uuid.New(), UserID: user, OrgID: org, ResourceType: "flag_rule", Action: "update", ResourceID: &realRule, NewValue: []byte(`{"flag_id":"` + prov.String() + `"}`)}
	if err := repo.Upsert(context.Background(), createRow); err != nil {
		t.Fatal(err)
	}
	if err := repo.Upsert(context.Background(), mutateRow); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Commit(context.Background(), user, org, user, []uuid.UUID{createRow.ID, mutateRow.ID})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if len(res.CommittedIDs) != 2 {
		t.Fatalf("committed: %v", res.CommittedIDs)
	}
	if seenFlagID != wantRealFlag {
		t.Errorf("rule handler saw flag_id %v, want resolved %v", seenFlagID, wantRealFlag)
	}
}

func TestCommit_RefusesUnresolvedProvisional(t *testing.T) {
	repo := newFakeRepo()
	svc := &Service{repo: repo, reg: NewCommitRegistry(), creates: NewCreateRegistry(), pool: &mockTxBeginner{}, audit: nil}
	user, org := uuid.New(), uuid.New()
	dangling := NewProvisional()
	realRule := uuid.New()
	row := &models.StagedChange{ID: uuid.New(), UserID: user, OrgID: org, ResourceType: "flag_rule", Action: "update", ResourceID: &realRule, NewValue: []byte(`{"flag_id":"` + dangling.String() + `"}`)}
	if err := repo.Upsert(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Commit(context.Background(), user, org, user, []uuid.UUID{row.ID})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if res.FailedID == nil || *res.FailedID != row.ID {
		t.Fatalf("expected FailedID = %v, got %v", row.ID, res.FailedID)
	}
	if !strings.Contains(res.FailedReason, dangling.String()) {
		t.Errorf("FailedReason should name the dangling provisional, got: %s", res.FailedReason)
	}
}

func TestStage_RejectsBadProvisionalVariant(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, NewCommitRegistry(), NewCreateRegistry(), nil, nil)
	bad := uuid.New() // RFC-4122 variant, not provisional
	row := &models.StagedChange{
		UserID: uuid.New(), OrgID: uuid.New(),
		ResourceType: "flag", Action: "create",
		ProvisionalID: &bad,
	}
	if err := svc.Stage(context.Background(), row); err == nil {
		t.Fatal("expected Stage to reject non-provisional variant byte")
	}
}

func TestStage_RejectsProvisionalResourceID(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, NewCommitRegistry(), NewCreateRegistry(), nil, nil)
	prov := NewProvisional()
	row := &models.StagedChange{
		UserID: uuid.New(), OrgID: uuid.New(),
		ResourceType: "flag", Action: "toggle",
		ResourceID: &prov,
	}
	if err := svc.Stage(context.Background(), row); err == nil {
		t.Fatal("expected Stage to reject provisional resource_id")
	}
}

// TestCommit_FallsThroughToMutationWhenNoCreateHandler verifies that when a
// row has a provisional id but no create handler is registered, Commit falls
// through to the CommitRegistry path and surfaces the resulting
// ErrNoCommitHandler as a per-row failure. This is the safety net for
// misconfiguration of the create registry.
func TestCommit_FallsThroughToMutationWhenNoCreateHandler(t *testing.T) {
	repo := newFakeRepo()
	reg := NewCommitRegistry()       // empty — no flag handler either
	createReg := NewCreateRegistry() // empty — no create handler

	svc := &Service{repo: repo, reg: reg, creates: createReg, pool: &mockTxBeginner{}}
	user, org := uuid.New(), uuid.New()
	prov := NewProvisional()
	row := &models.StagedChange{
		ID: uuid.New(), UserID: user, OrgID: org,
		ResourceType: "flag", Action: "create",
		ProvisionalID: &prov,
		NewValue:      []byte(`{}`),
	}
	if err := repo.Upsert(context.Background(), row); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Commit(context.Background(), user, org, user, []uuid.UUID{row.ID})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if res.FailedID == nil || *res.FailedID != row.ID {
		t.Fatalf("expected FailedID = %v, got %v", row.ID, res.FailedID)
	}
	if res.FailedReason == "" {
		t.Error("expected non-empty FailedReason")
	}
}

// TestCommit_PartialOnMidBatchFailure verifies that when a mutation handler
// errors mid-batch, the rows that completed before the failure are reported
// in CommittedIDs (and the tx rolls back, so they're not durably committed
// — the IDs are reported for caller-side bookkeeping only).
func TestCommit_PartialOnMidBatchFailure(t *testing.T) {
	repo := newFakeRepo()
	reg := NewCommitRegistry()
	createReg := NewCreateRegistry()

	wantRealFlag := uuid.New()
	createReg.Register("flag", "create", func(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (uuid.UUID, string, func(context.Context), error) {
		return wantRealFlag, "flag.created", nil, nil
	})
	reg.Register("flag_rule", "update", func(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (string, error) {
		return "", fmt.Errorf("simulated rule failure")
	})

	svc := &Service{repo: repo, reg: reg, creates: createReg, pool: &mockTxBeginner{}}
	user, org := uuid.New(), uuid.New()
	prov := NewProvisional()
	realRule := uuid.New()
	createRow := &models.StagedChange{ID: uuid.New(), UserID: user, OrgID: org, ResourceType: "flag", Action: "create", ProvisionalID: &prov, NewValue: []byte(`{}`)}
	mutateRow := &models.StagedChange{ID: uuid.New(), UserID: user, OrgID: org, ResourceType: "flag_rule", Action: "update", ResourceID: &realRule, NewValue: []byte(`{}`)}
	if err := repo.Upsert(context.Background(), createRow); err != nil {
		t.Fatal(err)
	}
	if err := repo.Upsert(context.Background(), mutateRow); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Commit(context.Background(), user, org, user, []uuid.UUID{createRow.ID, mutateRow.ID})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if len(res.CommittedIDs) != 1 || res.CommittedIDs[0] != createRow.ID {
		t.Errorf("CommittedIDs: got %v, want [%v]", res.CommittedIDs, createRow.ID)
	}
	if res.FailedID == nil || *res.FailedID != mutateRow.ID {
		t.Errorf("FailedID: got %v, want %v", res.FailedID, mutateRow.ID)
	}
	if !strings.Contains(res.FailedReason, "simulated rule failure") {
		t.Errorf("FailedReason: %q", res.FailedReason)
	}
}
