package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/flags"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FlagRepository implements flags.FlagRepository using a PostgreSQL connection pool.
type FlagRepository struct {
	pool *pgxpool.Pool
}

// NewFlagRepository creates a new FlagRepository backed by the given pool.
func NewFlagRepository(pool *pgxpool.Pool) *FlagRepository {
	return &FlagRepository{pool: pool}
}

// ---------------------------------------------------------------------------
// Scan helpers
// ---------------------------------------------------------------------------

// scanFeatureFlag reads a single FeatureFlag row from the given pgx.Row.
// The SELECT must include columns in the order defined by flagSelectCols.
func scanFeatureFlag(row pgx.Row) (*models.FeatureFlag, error) {
	var f models.FeatureFlag
	var defaultValueBytes []byte
	var archivedAt *time.Time

	err := row.Scan(
		&f.ID,
		&f.ProjectID,
		&f.EnvironmentID,
		&f.Key,
		&f.Name,
		&f.Description,
		&f.FlagType,
		&defaultValueBytes,
		&f.Enabled,
		&f.Tags,
		&f.CreatedBy,
		&f.CreatedAt,
		&f.UpdatedAt,
		&archivedAt,
		&f.Category,
		&f.Purpose,
		&f.Owners,
		&f.IsPermanent,
		&f.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if len(defaultValueBytes) > 0 {
		f.DefaultValue = string(defaultValueBytes)
	}
	f.Archived = archivedAt != nil

	return &f, nil
}

// scanTargetingRule reads a single TargetingRule row from the given pgx.Row.
// The SELECT must include columns in the order defined by ruleSelectCols.
func scanTargetingRule(row pgx.Row) (*models.TargetingRule, error) {
	var r models.TargetingRule

	err := row.Scan(
		&r.ID,
		&r.FlagID,
		&r.RuleType,
		&r.Priority,
		&r.Value,
		&r.Percentage,
		&r.Attribute,
		&r.Operator,
		&r.TargetValues,
		&r.SegmentID,
		&r.StartTime,
		&r.EndTime,
		&r.Enabled,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &r, nil
}

// ---------------------------------------------------------------------------
// Column lists
// ---------------------------------------------------------------------------

const flagSelectCols = `
	id, project_id, environment_id,
	key, name,
	COALESCE(description, ''),
	flag_type,
	default_value,
	enabled,
	COALESCE(tags, '{}'),
	created_by,
	created_at, updated_at,
	archived_at,
	COALESCE(category, ''),
	COALESCE(purpose, ''),
	COALESCE(owners, '{}'),
	is_permanent,
	expires_at`

const ruleSelectCols = `
	id, flag_id, rule_type, priority,
	COALESCE(value, ''),
	percentage,
	COALESCE(attribute, ''),
	COALESCE(operator, ''),
	COALESCE(target_values, '{}'),
	segment_id,
	start_time, end_time,
	enabled,
	created_at, updated_at`

// ---------------------------------------------------------------------------
// FeatureFlag methods
// ---------------------------------------------------------------------------

// CreateFlag inserts a new feature flag into the database.
func (r *FlagRepository) CreateFlag(ctx context.Context, flag *models.FeatureFlag) error {
	if flag.ID == uuid.Nil {
		flag.ID = uuid.New()
	}
	now := time.Now().UTC()
	flag.CreatedAt = now
	flag.UpdatedAt = now

	defaultValueJSON := flag.DefaultValue
	if defaultValueJSON == "" {
		defaultValueJSON = "null"
	}

	const q = `
		INSERT INTO feature_flags
			(id, project_id, environment_id, key, name, description, flag_type,
			 default_value, enabled, tags, created_by, created_at, updated_at,
			 category, purpose, owners, is_permanent, expires_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7,
			 $8, $9, $10, $11, $12, $13,
			 $14, $15, $16, $17, $18)`

	_, err := r.pool.Exec(ctx, q,
		flag.ID,
		flag.ProjectID,
		flag.EnvironmentID,
		flag.Key,
		flag.Name,
		flag.Description,
		flag.FlagType,
		[]byte(defaultValueJSON),
		flag.Enabled,
		flag.Tags,
		flag.CreatedBy,
		flag.CreatedAt,
		flag.UpdatedAt,
		flag.Category,
		flag.Purpose,
		flag.Owners,
		flag.IsPermanent,
		flag.ExpiresAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.CreateFlag: %w", err)
	}
	return nil
}

// GetFlag retrieves a feature flag by its unique identifier.
func (r *FlagRepository) GetFlag(ctx context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
	q := `SELECT` + flagSelectCols + ` FROM feature_flags WHERE id = $1`
	f, err := scanFeatureFlag(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetFlag: %w", err)
	}
	return f, nil
}

// GetFlagByKey retrieves a feature flag by its project, environment, and key.
func (r *FlagRepository) GetFlagByKey(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*models.FeatureFlag, error) {
	q := `SELECT` + flagSelectCols + ` FROM feature_flags WHERE project_id = $1 AND environment_id = $2 AND key = $3`
	f, err := scanFeatureFlag(r.pool.QueryRow(ctx, q, projectID, environmentID, key))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetFlagByKey: %w", err)
	}
	return f, nil
}

// ListFlags returns feature flags for a project, with optional filtering.
func (r *FlagRepository) ListFlags(ctx context.Context, projectID uuid.UUID, opts flags.ListOptions) ([]*models.FeatureFlag, error) {
	var w whereBuilder
	w.Add("project_id = $%d", projectID)

	if opts.EnvironmentID != nil {
		w.Add("environment_id = $%d", *opts.EnvironmentID)
	}
	if opts.Tag != "" {
		w.Add("$%d = ANY(tags)", opts.Tag)
	}

	whereClause, args := w.Build()

	archivedFilter := ""
	if opts.Archived != nil {
		if *opts.Archived {
			archivedFilter = " AND archived_at IS NOT NULL"
		} else {
			archivedFilter = " AND archived_at IS NULL"
		}
	}

	pagClause, args := paginationClause(opts.Limit, opts.Offset, args)
	q := `SELECT` + flagSelectCols + ` FROM feature_flags` + whereClause + archivedFilter + ` ORDER BY created_at DESC` + pagClause

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListFlags: %w", err)
	}
	defer rows.Close()

	var result []*models.FeatureFlag
	for rows.Next() {
		f, err := scanFeatureFlag(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListFlags: %w", err)
		}
		result = append(result, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListFlags: %w", err)
	}
	return result, nil
}

// UpdateFlag persists changes to an existing feature flag.
func (r *FlagRepository) UpdateFlag(ctx context.Context, flag *models.FeatureFlag) error {
	flag.UpdatedAt = time.Now().UTC()

	defaultValueJSON := flag.DefaultValue
	if defaultValueJSON == "" {
		defaultValueJSON = "null"
	}

	const q = `
		UPDATE feature_flags SET
			name           = $2,
			description    = $3,
			flag_type      = $4,
			default_value  = $5,
			enabled        = $6,
			tags           = $7,
			category       = $8,
			purpose        = $9,
			owners         = $10,
			is_permanent   = $11,
			expires_at     = $12,
			updated_at     = $13
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q,
		flag.ID,
		flag.Name,
		flag.Description,
		flag.FlagType,
		[]byte(defaultValueJSON),
		flag.Enabled,
		flag.Tags,
		flag.Category,
		flag.Purpose,
		flag.Owners,
		flag.IsPermanent,
		flag.ExpiresAt,
		flag.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateFlag: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteFlag soft-deletes a feature flag by setting archived_at.
func (r *FlagRepository) DeleteFlag(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE feature_flags SET archived_at = now() WHERE id = $1 AND archived_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.DeleteFlag: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// TargetingRule methods
// ---------------------------------------------------------------------------

// CreateRule inserts a new targeting rule into the database.
func (r *FlagRepository) CreateRule(ctx context.Context, rule *models.TargetingRule) error {
	if rule.ID == uuid.Nil {
		rule.ID = uuid.New()
	}
	now := time.Now().UTC()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	const q = `
		INSERT INTO flag_targeting_rules
			(id, flag_id, environment, rule_type, priority, value, percentage,
			 attribute, operator, target_values, segment_id, start_time, end_time,
			 enabled, conditions, serve_value, created_at, updated_at)
		VALUES
			($1, $2, '', $3, $4, $5, $6,
			 $7, $8, $9, $10, $11, $12,
			 $13, '{}', '{}', $14, $15)`

	_, err := r.pool.Exec(ctx, q,
		rule.ID,
		rule.FlagID,
		rule.RuleType,
		rule.Priority,
		rule.Value,
		rule.Percentage,
		rule.Attribute,
		rule.Operator,
		rule.TargetValues,
		rule.SegmentID,
		rule.StartTime,
		rule.EndTime,
		rule.Enabled,
		rule.CreatedAt,
		rule.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateRule: %w", err)
	}
	return nil
}

// GetRule retrieves a targeting rule by ID.
func (r *FlagRepository) GetRule(ctx context.Context, id uuid.UUID) (*models.TargetingRule, error) {
	q := `SELECT` + ruleSelectCols + ` FROM flag_targeting_rules WHERE id = $1`
	rule, err := scanTargetingRule(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetRule: %w", err)
	}
	return rule, nil
}

// ListRules returns all targeting rules for a flag, ordered by priority.
func (r *FlagRepository) ListRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error) {
	q := `SELECT` + ruleSelectCols + ` FROM flag_targeting_rules WHERE flag_id = $1 ORDER BY priority ASC`

	rows, err := r.pool.Query(ctx, q, flagID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListRules: %w", err)
	}
	defer rows.Close()

	var result []*models.TargetingRule
	for rows.Next() {
		rule, err := scanTargetingRule(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListRules: %w", err)
		}
		result = append(result, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListRules: %w", err)
	}
	return result, nil
}

// UpdateRule persists changes to an existing targeting rule.
func (r *FlagRepository) UpdateRule(ctx context.Context, rule *models.TargetingRule) error {
	rule.UpdatedAt = time.Now().UTC()

	const q = `
		UPDATE flag_targeting_rules SET
			rule_type     = $2,
			priority      = $3,
			value         = $4,
			percentage    = $5,
			attribute     = $6,
			operator      = $7,
			target_values = $8,
			segment_id    = $9,
			start_time    = $10,
			end_time      = $11,
			enabled       = $12,
			updated_at    = $13
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q,
		rule.ID,
		rule.RuleType,
		rule.Priority,
		rule.Value,
		rule.Percentage,
		rule.Attribute,
		rule.Operator,
		rule.TargetValues,
		rule.SegmentID,
		rule.StartTime,
		rule.EndTime,
		rule.Enabled,
		rule.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateRule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteRule hard-deletes a targeting rule.
func (r *FlagRepository) DeleteRule(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM flag_targeting_rules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres.DeleteRule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// FlagEnvironmentState methods
// ---------------------------------------------------------------------------

// ListFlagEnvStates returns all per-environment states for a given flag.
func (r *FlagRepository) ListFlagEnvStates(ctx context.Context, flagID uuid.UUID) ([]*models.FlagEnvironmentState, error) {
	const q = `
		SELECT id, flag_id, environment_id, enabled, value, updated_by, updated_at
		FROM flag_environment_state
		WHERE flag_id = $1
		ORDER BY environment_id`
	rows, err := r.pool.Query(ctx, q, flagID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListFlagEnvStates: %w", err)
	}
	defer rows.Close()
	var result []*models.FlagEnvironmentState
	for rows.Next() {
		var s models.FlagEnvironmentState
		if err := rows.Scan(&s.ID, &s.FlagID, &s.EnvironmentID, &s.Enabled, &s.Value, &s.UpdatedBy, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("postgres.ListFlagEnvStates: %w", err)
		}
		result = append(result, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListFlagEnvStates: %w", err)
	}
	return result, nil
}

// UpsertFlagEnvState creates or updates a per-environment flag state using
// ON CONFLICT on the (flag_id, environment_id) unique constraint.
func (r *FlagRepository) UpsertFlagEnvState(ctx context.Context, state *models.FlagEnvironmentState) error {
	if state.ID == uuid.Nil {
		state.ID = uuid.New()
	}
	state.UpdatedAt = time.Now().UTC()

	const q = `
		INSERT INTO flag_environment_state (id, flag_id, environment_id, enabled, value, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (flag_id, environment_id)
		DO UPDATE SET enabled = EXCLUDED.enabled, value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = EXCLUDED.updated_at
		RETURNING id`

	err := r.pool.QueryRow(ctx, q,
		state.ID, state.FlagID, state.EnvironmentID, state.Enabled, state.Value, state.UpdatedBy, state.UpdatedAt,
	).Scan(&state.ID)
	if err != nil {
		return fmt.Errorf("postgres.UpsertFlagEnvState: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Evaluation log
// ---------------------------------------------------------------------------

// WriteEvaluationLog persists a batch of flag evaluation log entries using
// pgx.Batch for efficient bulk inserts.
func (r *FlagRepository) WriteEvaluationLog(ctx context.Context, logs []flags.EvaluationLog) error {
	if len(logs) == 0 {
		return nil
	}

	const q = `
		INSERT INTO flag_evaluation_log
			(id, flag_key, environment, context_hash, result_value, rule_matched, evaluated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT DO NOTHING`

	batch := &pgx.Batch{}
	for _, entry := range logs {
		id := entry.ID
		if id == uuid.Nil {
			id = uuid.New()
		}

		// Derive environment from EvalCtx attributes or use empty string.
		env := ""
		if entry.EvalCtx.Attributes != nil {
			if e, ok := entry.EvalCtx.Attributes["environment"]; ok {
				env = e
			}
		}

		// Compute a simple context hash from the EvalCtx.
		ctxHash := entry.EvalCtx.UserID + ":" + entry.EvalCtx.OrgID

		// Marshal the result value as JSON.
		resultValueJSON, err := json.Marshal(entry.Value)
		if err != nil {
			resultValueJSON = []byte(`""`)
		}

		// rule_matched is a UUID; use nil if RuleID is empty.
		var ruleMatched *uuid.UUID
		if entry.RuleID != "" {
			if parsed, parseErr := uuid.Parse(entry.RuleID); parseErr == nil {
				ruleMatched = &parsed
			}
		}

		ts := entry.Timestamp
		if ts.IsZero() {
			ts = time.Now().UTC()
		}

		batch.Queue(q,
			id,
			entry.FlagKey,
			env,
			ctxHash,
			resultValueJSON,
			ruleMatched,
			ts,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer func() { _ = results.Close() }()

	for range logs {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("postgres.WriteEvaluationLog: %w", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Segment methods
// ---------------------------------------------------------------------------

// scanSegment reads a single Segment row from the given pgx.Row.
// The SELECT must include columns: id, project_id, key, name, description, combine_op, created_at, updated_at.
func scanSegment(row pgx.Row) (*models.Segment, error) {
	var s models.Segment
	err := row.Scan(
		&s.ID,
		&s.ProjectID,
		&s.Key,
		&s.Name,
		&s.Description,
		&s.CombineOp,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

const segmentSelectCols = `
	id, project_id, key, name,
	COALESCE(description, ''),
	COALESCE(combine_op, 'and'),
	created_at, updated_at`

// loadSegmentConditions fetches all conditions for the given segment ID and
// attaches them to the segment.
func (r *FlagRepository) loadSegmentConditions(ctx context.Context, seg *models.Segment) error {
	const q = `
		SELECT id, segment_id, attribute, operator, value, priority, created_at
		FROM segment_conditions
		WHERE segment_id = $1
		ORDER BY priority ASC`

	rows, err := r.pool.Query(ctx, q, seg.ID)
	if err != nil {
		return fmt.Errorf("postgres.loadSegmentConditions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var c models.SegmentCondition
		if err := rows.Scan(&c.ID, &c.SegmentID, &c.Attribute, &c.Operator, &c.Value, &c.Priority, &c.CreatedAt); err != nil {
			return fmt.Errorf("postgres.loadSegmentConditions: %w", err)
		}
		seg.Conditions = append(seg.Conditions, c)
	}
	return rows.Err()
}

// CreateSegment inserts a new segment and its conditions into the database
// within a single transaction.
func (r *FlagRepository) CreateSegment(ctx context.Context, segment *models.Segment) error {
	if segment.ID == uuid.Nil {
		segment.ID = uuid.New()
	}
	now := time.Now().UTC()
	segment.CreatedAt = now
	segment.UpdatedAt = now

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres.CreateSegment: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const q = `
		INSERT INTO segments (id, project_id, key, name, description, combine_op, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err = tx.Exec(ctx, q,
		segment.ID, segment.ProjectID, segment.Key, segment.Name,
		segment.Description, segment.CombineOp, segment.CreatedAt, segment.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.CreateSegment: %w", err)
	}

	for i := range segment.Conditions {
		c := &segment.Conditions[i]
		if c.ID == uuid.Nil {
			c.ID = uuid.New()
		}
		c.SegmentID = segment.ID
		c.CreatedAt = now

		const cq = `
			INSERT INTO segment_conditions (id, segment_id, attribute, operator, value, priority, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`
		if _, err := tx.Exec(ctx, cq, c.ID, c.SegmentID, c.Attribute, c.Operator, c.Value, c.Priority, c.CreatedAt); err != nil {
			return fmt.Errorf("postgres.CreateSegment condition: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres.CreateSegment: commit: %w", err)
	}
	return nil
}

// GetSegment retrieves a segment by its unique identifier, including conditions.
func (r *FlagRepository) GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error) {
	q := `SELECT` + segmentSelectCols + ` FROM segments WHERE id = $1`
	seg, err := scanSegment(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetSegment: %w", err)
	}
	if err := r.loadSegmentConditions(ctx, seg); err != nil {
		return nil, fmt.Errorf("postgres.GetSegment: %w", err)
	}
	return seg, nil
}

// GetSegmentByKey retrieves a segment by project ID and key, including conditions.
func (r *FlagRepository) GetSegmentByKey(ctx context.Context, projectID uuid.UUID, key string) (*models.Segment, error) {
	q := `SELECT` + segmentSelectCols + ` FROM segments WHERE project_id = $1 AND key = $2`
	seg, err := scanSegment(r.pool.QueryRow(ctx, q, projectID, key))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetSegmentByKey: %w", err)
	}
	if err := r.loadSegmentConditions(ctx, seg); err != nil {
		return nil, fmt.Errorf("postgres.GetSegmentByKey: %w", err)
	}
	return seg, nil
}

// ListSegments returns all segments for a project, each with their conditions.
func (r *FlagRepository) ListSegments(ctx context.Context, projectID uuid.UUID) ([]*models.Segment, error) {
	q := `SELECT` + segmentSelectCols + ` FROM segments WHERE project_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListSegments: %w", err)
	}
	defer rows.Close()

	var result []*models.Segment
	for rows.Next() {
		seg, err := scanSegment(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListSegments: %w", err)
		}
		result = append(result, seg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListSegments: %w", err)
	}

	for _, seg := range result {
		if err := r.loadSegmentConditions(ctx, seg); err != nil {
			return nil, fmt.Errorf("postgres.ListSegments: %w", err)
		}
	}
	return result, nil
}

// UpdateSegment persists changes to an existing segment within a transaction:
// UPDATE segments, DELETE old conditions, INSERT new conditions.
func (r *FlagRepository) UpdateSegment(ctx context.Context, segment *models.Segment) error {
	segment.UpdatedAt = time.Now().UTC()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres.UpdateSegment: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const q = `
		UPDATE segments SET
			name        = $2,
			description = $3,
			combine_op  = $4,
			updated_at  = $5
		WHERE id = $1`

	tag, err := tx.Exec(ctx, q, segment.ID, segment.Name, segment.Description, segment.CombineOp, segment.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres.UpdateSegment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM segment_conditions WHERE segment_id = $1`, segment.ID); err != nil {
		return fmt.Errorf("postgres.UpdateSegment delete conditions: %w", err)
	}

	now := time.Now().UTC()
	for i := range segment.Conditions {
		c := &segment.Conditions[i]
		if c.ID == uuid.Nil {
			c.ID = uuid.New()
		}
		c.SegmentID = segment.ID
		c.CreatedAt = now

		const cq = `
			INSERT INTO segment_conditions (id, segment_id, attribute, operator, value, priority, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`
		if _, err := tx.Exec(ctx, cq, c.ID, c.SegmentID, c.Attribute, c.Operator, c.Value, c.Priority, c.CreatedAt); err != nil {
			return fmt.Errorf("postgres.UpdateSegment condition: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres.UpdateSegment: commit: %w", err)
	}
	return nil
}

// DeleteSegment hard-deletes a segment. Conditions are removed via FK cascade.
func (r *FlagRepository) DeleteSegment(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM segments WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres.DeleteSegment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// isUniqueViolation returns true when err is a PostgreSQL unique-constraint
// violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return containsSQLState(err, "23505")
}

// containsSQLState checks whether the error message contains the given
// PostgreSQL SQLSTATE code. This is a lightweight alternative to importing
// pgconn just for the PgError type.
func containsSQLState(err error, code string) bool {
	type sqlStater interface {
		SQLState() string
	}
	var se sqlStater
	if errors.As(err, &se) {
		return se.SQLState() == code
	}
	return false
}
