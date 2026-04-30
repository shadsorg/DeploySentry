package postgres

import (
	"context"
	"fmt"

	"github.com/shadsorg/deploysentry/internal/auth"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditLogRepository implements auth.AuditLogRepository using a PostgreSQL connection pool.
type AuditLogRepository struct {
	pool *pgxpool.Pool
}

// NewAuditLogRepository creates a new AuditLogRepository backed by the given pool.
func NewAuditLogRepository(pool *pgxpool.Pool) *AuditLogRepository {
	return &AuditLogRepository{pool: pool}
}

// QueryAuditLogs retrieves audit log entries matching the given filter along
// with the total count of matching rows (ignoring pagination).
func (r *AuditLogRepository) QueryAuditLogs(ctx context.Context, filter auth.AuditLogFilter) ([]*models.AuditLogEntry, int, error) {
	var wb whereBuilder
	wb.Add("a.org_id = $%d", filter.OrgID)

	if filter.ProjectID != nil {
		wb.Add("a.project_id = $%d", *filter.ProjectID)
	}
	if filter.UserID != nil {
		wb.Add("a.user_id = $%d", *filter.UserID)
	}
	if filter.Action != "" {
		wb.Add("a.action = $%d", filter.Action)
	}
	if filter.ResourceType != "" {
		wb.Add("a.resource_type = $%d", filter.ResourceType)
	}
	if filter.ResourceID != nil {
		wb.Add("a.resource_id = $%d", *filter.ResourceID)
	}
	if filter.StartDate != nil {
		wb.Add("a.created_at >= $%d", *filter.StartDate)
	}
	if filter.EndDate != nil {
		wb.Add("a.created_at <= $%d", *filter.EndDate)
	}

	where, filterArgs := wb.Build()

	// COUNT query — reuse the same WHERE args.
	countQ := `SELECT COUNT(*) FROM audit_log a` + where
	var total int
	if err := r.pool.QueryRow(ctx, countQ, filterArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres.QueryAuditLogs count: %w", err)
	}

	// Paginated SELECT query.
	pagClause, selectArgs := paginationClause(filter.Limit, filter.Offset, filterArgs)
	selectQ := `
		SELECT
			a.id, a.org_id,
			COALESCE(a.project_id, '00000000-0000-0000-0000-000000000000'::uuid),
			a.user_id, COALESCE(u.name, ''), a.action, a.resource_type,
			COALESCE(a.resource_id, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(a.old_value::text, ''),
			COALESCE(a.new_value::text, ''),
			COALESCE(a.ip_address::text, ''),
			COALESCE(a.user_agent, ''),
			a.created_at
		FROM audit_log a
		LEFT JOIN users u ON u.id = a.user_id` + where + ` ORDER BY a.created_at DESC` + pagClause

	rows, err := r.pool.Query(ctx, selectQ, selectArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres.QueryAuditLogs: %w", err)
	}
	defer rows.Close()

	var entries []*models.AuditLogEntry
	for rows.Next() {
		var e models.AuditLogEntry
		if err := rows.Scan(
			&e.ID,
			&e.OrgID,
			&e.ProjectID,
			&e.ActorID,
			&e.ActorName,
			&e.Action,
			&e.EntityType,
			&e.EntityID,
			&e.OldValue,
			&e.NewValue,
			&e.IPAddress,
			&e.UserAgent,
			&e.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("postgres.QueryAuditLogs scan: %w", err)
		}
		entries = append(entries, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("postgres.QueryAuditLogs: %w", err)
	}

	return entries, total, nil
}

// GetAuditLogEntry retrieves a single audit log entry by id.
func (r *AuditLogRepository) GetAuditLogEntry(ctx context.Context, id uuid.UUID) (*models.AuditLogEntry, error) {
	const q = `
		SELECT
			a.id, a.org_id,
			COALESCE(a.project_id, '00000000-0000-0000-0000-000000000000'::uuid),
			a.user_id, COALESCE(u.name, ''), a.action, a.resource_type,
			COALESCE(a.resource_id, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(a.old_value::text, ''),
			COALESCE(a.new_value::text, ''),
			COALESCE(a.ip_address::text, ''),
			COALESCE(a.user_agent, ''),
			a.created_at
		FROM audit_log a
		LEFT JOIN users u ON u.id = a.user_id
		WHERE a.id = $1`
	row := r.pool.QueryRow(ctx, q, id)
	var e models.AuditLogEntry
	if err := row.Scan(
		&e.ID, &e.OrgID, &e.ProjectID, &e.ActorID, &e.ActorName,
		&e.Action, &e.EntityType, &e.EntityID,
		&e.OldValue, &e.NewValue, &e.IPAddress, &e.UserAgent, &e.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("postgres.GetAuditLogEntry: %w", err)
	}
	return &e, nil
}

// WriteAuditLog inserts a single audit log entry.
func (r *AuditLogRepository) WriteAuditLog(ctx context.Context, entry *models.AuditLogEntry) error {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	const q = `
		INSERT INTO audit_log (id, org_id, project_id, user_id, action, resource_type, resource_id, old_value, new_value, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10::inet, $11, $12)`
	_, err := r.pool.Exec(ctx, q,
		entry.ID, entry.OrgID, entry.ProjectID, entry.ActorID,
		entry.Action, entry.EntityType, entry.EntityID,
		nullIfEmpty(entry.OldValue), nullIfEmpty(entry.NewValue),
		nullIfEmpty(entry.IPAddress), entry.UserAgent, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.WriteAuditLog: %w", err)
	}
	return nil
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
