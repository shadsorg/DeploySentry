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
	defer results.Close()

	for range logs {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("postgres.WriteEvaluationLog: %w", err)
		}
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
