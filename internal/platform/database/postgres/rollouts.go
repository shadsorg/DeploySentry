package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/rollout"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrRolloutNotFound is returned when a rollout lookup finds no row.
var ErrRolloutNotFound = errors.New("rollout not found")

// ---------------------------------------------------------------------------
// RolloutRepo
// ---------------------------------------------------------------------------

// RolloutRepo is a Postgres-backed rollout.RolloutRepository.
type RolloutRepo struct {
	db *pgxpool.Pool
}

// NewRolloutRepo returns a new RolloutRepo.
func NewRolloutRepo(db *pgxpool.Pool) *RolloutRepo {
	return &RolloutRepo{db: db}
}

var _ rollout.RolloutRepository = (*RolloutRepo)(nil)

const selectRolloutCols = `SELECT
    id, release_id, target_type, target_ref, strategy_snapshot, signal_source,
    status, current_phase_index, current_phase_started_at, last_healthy_since,
    rollback_reason, created_by, created_at, completed_at`

func (r *RolloutRepo) Create(ctx context.Context, ro *models.Rollout) error {
	if ro.ID == uuid.Nil {
		ro.ID = uuid.New()
	}
	ro.CreatedAt = time.Now().UTC()

	targetRefJSON, err := json.Marshal(ro.TargetRef)
	if err != nil {
		return fmt.Errorf("marshal target_ref: %w", err)
	}
	snapshotJSON, err := json.Marshal(ro.StrategySnapshot)
	if err != nil {
		return fmt.Errorf("marshal strategy_snapshot: %w", err)
	}
	signalJSON, err := json.Marshal(ro.SignalSource)
	if err != nil {
		return fmt.Errorf("marshal signal_source: %w", err)
	}

	_, err = r.db.Exec(ctx, `
        INSERT INTO rollouts (
            id, release_id, target_type, target_ref, strategy_snapshot, signal_source,
            status, current_phase_index, current_phase_started_at, last_healthy_since,
            rollback_reason, created_by, created_at, completed_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		ro.ID, ro.ReleaseID, ro.TargetType, targetRefJSON, snapshotJSON, signalJSON,
		ro.Status, ro.CurrentPhaseIndex, ro.CurrentPhaseStartedAt, ro.LastHealthySince,
		ro.RollbackReason, ro.CreatedBy, ro.CreatedAt, ro.CompletedAt,
	)
	return err
}

func (r *RolloutRepo) Get(ctx context.Context, id uuid.UUID) (*models.Rollout, error) {
	return r.scanOne(ctx, `WHERE id=$1`, id)
}

func (r *RolloutRepo) GetActiveByDeployment(ctx context.Context, deploymentID uuid.UUID) (*models.Rollout, error) {
	depStr := deploymentID.String()
	return r.scanOne(ctx,
		`WHERE target_type='deploy' AND target_ref->>'deployment_id'=$1 AND status IN ('pending','active','paused','awaiting_approval')`,
		depStr,
	)
}

func (r *RolloutRepo) GetActiveByConfig(ctx context.Context, flagKey, env string) (*models.Rollout, error) {
	return r.scanOne(ctx,
		`WHERE target_type='config' AND target_ref->>'flag_key'=$1 AND target_ref->>'env'=$2 AND status IN ('pending','active','paused','awaiting_approval')`,
		flagKey, env,
	)
}

func (r *RolloutRepo) GetActiveByRule(ctx context.Context, ruleID uuid.UUID) (*models.Rollout, error) {
	return r.scanOne(ctx,
		`WHERE target_type='config' AND target_ref->>'rule_id'=$1 AND status IN ('pending','active','paused','awaiting_approval')`,
		ruleID.String(),
	)
}

func (r *RolloutRepo) List(ctx context.Context, opts rollout.RolloutListOptions) ([]*models.Rollout, error) {
	w := whereBuilder{}
	if opts.TargetType != nil {
		w.Add("target_type=$%d", string(*opts.TargetType))
	}
	if opts.Status != nil {
		w.Add("status=$%d", string(*opts.Status))
	}
	if opts.ReleaseID != nil {
		w.Add("release_id=$%d", *opts.ReleaseID)
	}
	if opts.Since != nil {
		w.Add("created_at >= $%d", *opts.Since)
	}
	where, args := w.Build()

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	pagination, args := paginationClause(limit, 0, args)

	rows, err := r.db.Query(ctx, selectRolloutCols+" FROM rollouts"+where+" ORDER BY created_at DESC"+pagination, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRollouts(rows)
}

func (r *RolloutRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status models.RolloutStatus, rollbackReason *string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE rollouts SET status=$1, rollback_reason=COALESCE($2, rollback_reason) WHERE id=$3`,
		status, rollbackReason, id,
	)
	return err
}

func (r *RolloutRepo) UpdatePhasePointer(ctx context.Context, id uuid.UUID, phaseIndex int, startedAt *time.Time, lastHealthy *time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE rollouts SET current_phase_index=$1, current_phase_started_at=$2, last_healthy_since=COALESCE($3, last_healthy_since) WHERE id=$4`,
		phaseIndex, startedAt, lastHealthy, id,
	)
	return err
}

func (r *RolloutRepo) MarkCompleted(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx,
		`UPDATE rollouts SET status='succeeded', completed_at=$1 WHERE id=$2`,
		now, id,
	)
	return err
}

// ListByRelease returns all rollouts (any status) attached to a rollout group.
func (r *RolloutRepo) ListByRelease(ctx context.Context, releaseID uuid.UUID) ([]*models.Rollout, error) {
	rows, err := r.db.Query(ctx, selectRolloutCols+" FROM rollouts WHERE release_id=$1 ORDER BY created_at ASC", releaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRollouts(rows)
}

// SetReleaseID attaches (or detaches with nil) a rollout to a group.
func (r *RolloutRepo) SetReleaseID(ctx context.Context, rolloutID uuid.UUID, releaseID *uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `UPDATE rollouts SET release_id=$1 WHERE id=$2`, releaseID, rolloutID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRolloutNotFound
	}
	return nil
}

func (r *RolloutRepo) scanOne(ctx context.Context, where string, args ...any) (*models.Rollout, error) {
	rows, err := r.db.Query(ctx, selectRolloutCols+" FROM rollouts "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanRollouts(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrRolloutNotFound
	}
	return list[0], nil
}

func scanRollouts(rows pgx.Rows) ([]*models.Rollout, error) {
	var out []*models.Rollout
	for rows.Next() {
		var ro models.Rollout
		var targetRefJSON, snapshotJSON, signalJSON []byte
		var releaseID *uuid.UUID
		var createdBy *uuid.UUID
		var phaseStartedAt sql.NullTime
		var lastHealthy sql.NullTime
		var rollbackReason sql.NullString
		var completedAt sql.NullTime

		if err := rows.Scan(
			&ro.ID, &releaseID, &ro.TargetType, &targetRefJSON, &snapshotJSON, &signalJSON,
			&ro.Status, &ro.CurrentPhaseIndex, &phaseStartedAt, &lastHealthy,
			&rollbackReason, &createdBy, &ro.CreatedAt, &completedAt,
		); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(targetRefJSON, &ro.TargetRef); err != nil {
			return nil, fmt.Errorf("decode target_ref: %w", err)
		}
		if err := json.Unmarshal(snapshotJSON, &ro.StrategySnapshot); err != nil {
			return nil, fmt.Errorf("decode strategy_snapshot: %w", err)
		}
		if err := json.Unmarshal(signalJSON, &ro.SignalSource); err != nil {
			return nil, fmt.Errorf("decode signal_source: %w", err)
		}

		ro.ReleaseID = releaseID
		ro.CreatedBy = createdBy
		if phaseStartedAt.Valid {
			ro.CurrentPhaseStartedAt = &phaseStartedAt.Time
		}
		if lastHealthy.Valid {
			ro.LastHealthySince = &lastHealthy.Time
		}
		if rollbackReason.Valid {
			ro.RollbackReason = &rollbackReason.String
		}
		if completedAt.Valid {
			ro.CompletedAt = &completedAt.Time
		}

		out = append(out, &ro)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// RolloutPhaseRepo
// ---------------------------------------------------------------------------

// RolloutPhaseRepo is a Postgres-backed rollout.RolloutPhaseRepository.
type RolloutPhaseRepo struct {
	db *pgxpool.Pool
}

// NewRolloutPhaseRepo returns a new RolloutPhaseRepo.
func NewRolloutPhaseRepo(db *pgxpool.Pool) *RolloutPhaseRepo {
	return &RolloutPhaseRepo{db: db}
}

var _ rollout.RolloutPhaseRepository = (*RolloutPhaseRepo)(nil)

func (r *RolloutPhaseRepo) BulkInsert(ctx context.Context, phases []*models.RolloutPhase) error {
	if len(phases) == 0 {
		return nil
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, p := range phases {
		if p.ID == uuid.Nil {
			p.ID = uuid.New()
		}
		stepJSON, err := json.Marshal(p.StepSnapshot)
		if err != nil {
			return fmt.Errorf("marshal step_snapshot: %w", err)
		}
		_, err = tx.Exec(ctx, `
            INSERT INTO rollout_phases (id, rollout_id, phase_index, step_snapshot, status, entered_at, exited_at, applied_percent, health_score_at_exit, notes)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			p.ID, p.RolloutID, p.PhaseIndex, stepJSON, p.Status,
			p.EnteredAt, p.ExitedAt, p.AppliedPercent, p.HealthScoreAtExit, p.Notes,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *RolloutPhaseRepo) ListByRollout(ctx context.Context, rolloutID uuid.UUID) ([]*models.RolloutPhase, error) {
	rows, err := r.db.Query(ctx, `
        SELECT id, rollout_id, phase_index, step_snapshot, status, entered_at, exited_at, applied_percent, health_score_at_exit, notes
        FROM rollout_phases WHERE rollout_id=$1 ORDER BY phase_index`,
		rolloutID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.RolloutPhase
	for rows.Next() {
		var p models.RolloutPhase
		var stepJSON []byte
		var enteredAt, exitedAt sql.NullTime
		var appliedPct, healthScore sql.NullFloat64

		if err := rows.Scan(
			&p.ID, &p.RolloutID, &p.PhaseIndex, &stepJSON, &p.Status,
			&enteredAt, &exitedAt, &appliedPct, &healthScore, &p.Notes,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(stepJSON, &p.StepSnapshot); err != nil {
			return nil, fmt.Errorf("decode step_snapshot: %w", err)
		}
		if enteredAt.Valid {
			p.EnteredAt = &enteredAt.Time
		}
		if exitedAt.Valid {
			p.ExitedAt = &exitedAt.Time
		}
		if appliedPct.Valid {
			p.AppliedPercent = &appliedPct.Float64
		}
		if healthScore.Valid {
			p.HealthScoreAtExit = &healthScore.Float64
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

func (r *RolloutPhaseRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status models.PhaseStatus, enteredAt, exitedAt *time.Time, appliedPct, healthScore *float64, notes string) error {
	_, err := r.db.Exec(ctx, `
        UPDATE rollout_phases SET
            status=$1,
            entered_at=COALESCE($2, entered_at),
            exited_at=COALESCE($3, exited_at),
            applied_percent=COALESCE($4, applied_percent),
            health_score_at_exit=COALESCE($5, health_score_at_exit),
            notes=CASE WHEN $6 = '' THEN notes ELSE $6 END
        WHERE id=$7`,
		status, enteredAt, exitedAt, appliedPct, healthScore, notes, id,
	)
	return err
}

// ---------------------------------------------------------------------------
// RolloutEventRepo
// ---------------------------------------------------------------------------

// RolloutEventRepo is a Postgres-backed rollout.RolloutEventRepository.
type RolloutEventRepo struct {
	db *pgxpool.Pool
}

// NewRolloutEventRepo returns a new RolloutEventRepo.
func NewRolloutEventRepo(db *pgxpool.Pool) *RolloutEventRepo {
	return &RolloutEventRepo{db: db}
}

var _ rollout.RolloutEventRepository = (*RolloutEventRepo)(nil)

func (r *RolloutEventRepo) Insert(ctx context.Context, e *models.RolloutEvent) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	payloadJSON, err := json.Marshal(e.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	_, err = r.db.Exec(ctx, `
        INSERT INTO rollout_events (id, rollout_id, event_type, actor_type, actor_id, reason, payload, occurred_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		e.ID, e.RolloutID, e.EventType, e.ActorType, e.ActorID, e.Reason, payloadJSON, e.OccurredAt,
	)
	return err
}

func (r *RolloutEventRepo) ListByRollout(ctx context.Context, rolloutID uuid.UUID, limit int) ([]*models.RolloutEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `
        SELECT id, rollout_id, event_type, actor_type, actor_id, reason, payload, occurred_at
        FROM rollout_events WHERE rollout_id=$1 ORDER BY occurred_at DESC LIMIT $2`,
		rolloutID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.RolloutEvent
	for rows.Next() {
		var e models.RolloutEvent
		var actorID *uuid.UUID
		var reason sql.NullString
		var payloadJSON []byte

		if err := rows.Scan(
			&e.ID, &e.RolloutID, &e.EventType, &e.ActorType, &actorID, &reason, &payloadJSON, &e.OccurredAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payloadJSON, &e.Payload); err != nil {
			return nil, fmt.Errorf("decode payload: %w", err)
		}
		e.ActorID = actorID
		if reason.Valid {
			e.Reason = &reason.String
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}
