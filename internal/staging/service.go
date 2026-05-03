package staging

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shadsorg/deploysentry/internal/models"
)

// txBeginner abstracts pgxpool.Pool for unit tests that don't have a real DB.
type txBeginner interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
}

// AuditWriter is satisfied by the existing Postgres audit repository so the
// staging package doesn't need to import internal/auth.
type AuditWriter interface {
	WriteAuditLog(ctx context.Context, entry *models.AuditLogEntry) error
}

// Service orchestrates staging-layer operations: stage a row, list a user's
// pending changes, deploy (commit) selected rows in one transaction, discard.
type Service struct {
	repo    Repository
	reg     *CommitRegistry
	creates *CreateRegistry
	pool    txBeginner
	audit   AuditWriter
}

// NewService wires the staging service. pool is the same pool used by the
// rest of the app — Service.Commit opens a transaction on it so the create
// + mutation handlers and the staged_changes DELETE all ride the same
// boundary.
//
// audit may be nil for unit tests; production should wire it so committed
// rows leave an audit trail.
func NewService(repo Repository, reg *CommitRegistry, creates *CreateRegistry, pool *pgxpool.Pool, audit AuditWriter) *Service {
	return &Service{repo: repo, reg: reg, creates: creates, pool: pool, audit: audit}
}

// Stage upserts a single staged change. Caller is responsible for setting
// UserID, OrgID, ResourceType, Action; everything else is optional. ID is
// minted if zero.
func (s *Service) Stage(ctx context.Context, row *models.StagedChange) error {
	if row.UserID == uuid.Nil || row.OrgID == uuid.Nil {
		return errors.New("staging.Stage: user_id and org_id are required")
	}
	if row.ResourceType == "" || row.Action == "" {
		return errors.New("staging.Stage: resource_type and action are required")
	}
	if row.ProvisionalID != nil && !IsProvisional(*row.ProvisionalID) {
		return errors.New("staging.Stage: provisional_id must use the provisional UUID variant byte")
	}
	if row.ResourceID != nil && IsProvisional(*row.ResourceID) {
		return errors.New("staging.Stage: resource_id must not be provisional")
	}
	return s.repo.Upsert(ctx, row)
}

// ListForUser proxies to the repository — used by the handler's GET endpoint.
func (s *Service) ListForUser(ctx context.Context, userID, orgID uuid.UUID) ([]*models.StagedChange, error) {
	return s.repo.ListForUser(ctx, userID, orgID)
}

// CountForUser proxies to the repository — used by the header-banner count.
func (s *Service) CountForUser(ctx context.Context, userID, orgID uuid.UUID) (int, error) {
	return s.repo.CountForUser(ctx, userID, orgID)
}

// CommitResult reports per-row outcomes from Deploy. Successful rows include
// the audit-log action recorded; failed rows include the error message and
// abort the transaction.
type CommitResult struct {
	CommittedIDs []uuid.UUID `json:"committed_ids"`
	FailedID     *uuid.UUID  `json:"failed_id,omitempty"`
	FailedReason string      `json:"failed_reason,omitempty"`
}

// Commit deploys the requested staged rows in one transaction using a
// four-phase pipeline: load → preflight → tx + topo-ordered dispatch →
// tx.Commit → post-commit hooks.
//
// actorID is the user committing (for the audit row). It may differ from the
// staged row's user_id in a future delegated-deploy scenario; in Phase A the
// commit endpoint requires (userID == actorID).
func (s *Service) Commit(ctx context.Context, userID, orgID, actorID uuid.UUID, ids []uuid.UUID) (*CommitResult, error) {
	if len(ids) == 0 {
		return &CommitResult{}, nil
	}

	rows, err := s.repo.GetByIDs(ctx, userID, orgID, ids)
	if err != nil {
		return nil, fmt.Errorf("staging.Commit: load rows: %w", err)
	}
	if len(rows) != len(ids) {
		return nil, fmt.Errorf("staging.Commit: %d of %d rows not found or not owned by user", len(ids)-len(rows), len(ids))
	}

	plan, err := planBatch(rows)
	if err != nil {
		var unresolved *ErrUnresolvedProvisional
		if errors.As(err, &unresolved) {
			rid := unresolved.RowID
			return &CommitResult{FailedID: &rid, FailedReason: unresolved.Error()}, nil
		}
		return nil, fmt.Errorf("staging.Commit: preflight: %w", err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("staging.Commit: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op once Commit succeeds

	resolver := NewResolver()
	committed := make([]uuid.UUID, 0, len(plan.ordered))
	auditEntries := make([]*models.AuditLogEntry, 0, len(plan.ordered))
	postCommitHooks := make([]func(context.Context), 0)

	for _, r := range plan.ordered {
		// Create branch: a row that minted a provisional id and has a
		// create-handler registered. Resolves provisional → real and binds
		// it for downstream rows. Audit row uses the resolved real id.
		if r.ProvisionalID != nil && s.creates != nil && s.creates.IsCreatable(r.ResourceType, r.Action) {
			realID, auditAction, hook, err := s.creates.Dispatch(ctx, tx, r)
			if err != nil {
				rid := r.ID
				return &CommitResult{CommittedIDs: committed, FailedID: &rid, FailedReason: err.Error()}, nil
			}
			resolver.Bind(*r.ProvisionalID, realID)
			if hook != nil {
				postCommitHooks = append(postCommitHooks, hook)
			}
			entry := buildAuditEntry(r, actorID, auditAction)
			entry.EntityID = realID
			MustNotBeProvisional(entry.EntityID, "audit_log.entity_id (create)")
			auditEntries = append(auditEntries, entry)
			committed = append(committed, r.ID)
			continue
		}

		// Mutation branch: rewrite any provisional references using the
		// resolver state built from earlier creates in this batch, then
		// dispatch through the existing CommitRegistry.
		if err := resolver.RewriteRow(r); err != nil {
			rid := r.ID
			return &CommitResult{CommittedIDs: committed, FailedID: &rid, FailedReason: err.Error()}, nil
		}
		auditAction, err := s.reg.Dispatch(ctx, tx, r)
		if err != nil {
			rid := r.ID
			return &CommitResult{CommittedIDs: committed, FailedID: &rid, FailedReason: err.Error()}, nil
		}
		entry := buildAuditEntry(r, actorID, auditAction)
		MustNotBeProvisional(entry.EntityID, "audit_log.entity_id")
		auditEntries = append(auditEntries, entry)
		committed = append(committed, r.ID)
	}

	if err := s.repo.DeleteByIDsTx(ctx, tx, userID, orgID, committed); err != nil {
		return nil, fmt.Errorf("staging.Commit: delete staged rows: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("staging.Commit: tx commit: %w", err)
	}

	// Post-commit hooks fire only after tx.Commit succeeds — we never publish
	// events or invalidate caches for production rows that got rolled back.
	for _, hook := range postCommitHooks {
		hook(ctx)
	}

	if s.audit != nil {
		for _, e := range auditEntries {
			if writeErr := s.audit.WriteAuditLog(ctx, e); writeErr != nil {
				err = errors.Join(err, fmt.Errorf("audit row for %s: %w", e.EntityID, writeErr))
			}
		}
	}

	return &CommitResult{CommittedIDs: committed}, err
}

// DiscardOne removes one staged row owned by (userID, orgID).
func (s *Service) DiscardOne(ctx context.Context, userID, orgID, id uuid.UUID) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("staging.DiscardOne: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := s.repo.DeleteByIDsTx(ctx, tx, userID, orgID, []uuid.UUID{id}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DiscardAll drops every staged row for (userID, orgID). Returns how many.
func (s *Service) DiscardAll(ctx context.Context, userID, orgID uuid.UUID) (int64, error) {
	return s.repo.DeleteAllForUser(ctx, userID, orgID)
}

// buildAuditEntry materialises an audit row from a committed staged change.
// The spec adds metadata.staged_at; the existing AuditLogEntry model has no
// metadata column, so the staged_at timestamp is appended into NewValue's
// JSON envelope under the key "_staged_at" until the audit schema grows a
// proper metadata field. This keeps Phase A self-contained.
func buildAuditEntry(r *models.StagedChange, actorID uuid.UUID, action string) *models.AuditLogEntry {
	entityID := uuid.Nil
	if r.ResourceID != nil {
		entityID = *r.ResourceID
	}
	return &models.AuditLogEntry{
		ID:         uuid.New(),
		OrgID:      r.OrgID,
		ActorID:    actorID,
		Action:     action,
		EntityType: r.ResourceType,
		EntityID:   entityID,
		OldValue:   string(r.OldValue),
		NewValue:   stagedNewValue(r.NewValue, r.CreatedAt),
		CreatedAt:  time.Now().UTC(),
	}
}

// stagedNewValue annotates the new_value JSON with the original staging
// timestamp so the audit trail can show "queued at X, deployed at Y".
func stagedNewValue(newVal []byte, stagedAt time.Time) string {
	if len(newVal) == 0 {
		return fmt.Sprintf(`{"_staged_at":%q}`, stagedAt.UTC().Format(time.RFC3339Nano))
	}
	// If newVal is a JSON object, splice _staged_at in. Otherwise wrap it.
	if len(newVal) > 0 && newVal[0] == '{' {
		// Cheap splice: insert "_staged_at": "..." right after the opening brace.
		s := string(newVal)
		annot := fmt.Sprintf(`{"_staged_at":%q,`, stagedAt.UTC().Format(time.RFC3339Nano))
		// If body is "{}" the resulting "{ann_at": ..., }" would have a trailing
		// comma — guard that case.
		if s == "{}" {
			return annot[:len(annot)-1] + "}"
		}
		return annot + s[1:]
	}
	return fmt.Sprintf(`{"_staged_at":%q,"value":%s}`, stagedAt.UTC().Format(time.RFC3339Nano), string(newVal))
}
