package postgres

import (
	"context"
	"fmt"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
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
	wb.Add("org_id = $%d", filter.OrgID)

	if filter.ProjectID != nil {
		wb.Add("project_id = $%d", *filter.ProjectID)
	}
	if filter.UserID != nil {
		wb.Add("user_id = $%d", *filter.UserID)
	}
	if filter.Action != "" {
		wb.Add("action = $%d", filter.Action)
	}
	if filter.ResourceType != "" {
		wb.Add("resource_type = $%d", filter.ResourceType)
	}
	if filter.StartDate != nil {
		wb.Add("created_at >= $%d", *filter.StartDate)
	}
	if filter.EndDate != nil {
		wb.Add("created_at <= $%d", *filter.EndDate)
	}

	where, filterArgs := wb.Build()

	// COUNT query — reuse the same WHERE args.
	countQ := `SELECT COUNT(*) FROM audit_log` + where
	var total int
	if err := r.pool.QueryRow(ctx, countQ, filterArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres.QueryAuditLogs count: %w", err)
	}

	// Paginated SELECT query.
	pagClause, selectArgs := paginationClause(filter.Limit, filter.Offset, filterArgs)
	selectQ := `
		SELECT
			id, org_id,
			COALESCE(project_id, '00000000-0000-0000-0000-000000000000'::uuid),
			user_id, action, resource_type,
			COALESCE(resource_id, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(old_value::text, ''),
			COALESCE(new_value::text, ''),
			COALESCE(ip_address::text, ''),
			COALESCE(user_agent, ''),
			created_at
		FROM audit_log` + where + ` ORDER BY created_at DESC` + pagClause

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
