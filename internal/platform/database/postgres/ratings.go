package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RatingRepository implements ratings.RatingRepository using PostgreSQL.
type RatingRepository struct {
	pool *pgxpool.Pool
}

// NewRatingRepository creates a new RatingRepository backed by the given pool.
func NewRatingRepository(pool *pgxpool.Pool) *RatingRepository {
	return &RatingRepository{pool: pool}
}

func (r *RatingRepository) UpsertRating(ctx context.Context, rating *models.FlagRating) error {
	query := `
		INSERT INTO flag_ratings (id, flag_id, user_id, org_id, rating, comment, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (flag_id, user_id) DO UPDATE SET
			rating = EXCLUDED.rating,
			comment = EXCLUDED.comment,
			updated_at = EXCLUDED.updated_at
		RETURNING id, created_at, updated_at`
	return r.pool.QueryRow(ctx, query,
		rating.FlagID, rating.UserID, rating.OrgID,
		rating.Rating, rating.Comment,
		rating.CreatedAt, rating.UpdatedAt,
	).Scan(&rating.ID, &rating.CreatedAt, &rating.UpdatedAt)
}

func (r *RatingRepository) GetRating(ctx context.Context, flagID, userID uuid.UUID) (*models.FlagRating, error) {
	query := `SELECT id, flag_id, user_id, org_id, rating, comment, created_at, updated_at
		FROM flag_ratings WHERE flag_id = $1 AND user_id = $2`
	var fr models.FlagRating
	err := r.pool.QueryRow(ctx, query, flagID, userID).Scan(
		&fr.ID, &fr.FlagID, &fr.UserID, &fr.OrgID,
		&fr.Rating, &fr.Comment, &fr.CreatedAt, &fr.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &fr, nil
}

func (r *RatingRepository) ListRatings(ctx context.Context, flagID uuid.UUID, limit, offset int) ([]*models.FlagRating, error) {
	query := `SELECT id, flag_id, user_id, org_id, rating, comment, created_at, updated_at
		FROM flag_ratings WHERE flag_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, query, flagID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*models.FlagRating
	for rows.Next() {
		var fr models.FlagRating
		if err := rows.Scan(&fr.ID, &fr.FlagID, &fr.UserID, &fr.OrgID,
			&fr.Rating, &fr.Comment, &fr.CreatedAt, &fr.UpdatedAt); err != nil {
			return nil, err
		}
		results = append(results, &fr)
	}
	return results, rows.Err()
}

func (r *RatingRepository) DeleteRating(ctx context.Context, flagID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM flag_ratings WHERE flag_id = $1 AND user_id = $2`, flagID, userID)
	return err
}

func (r *RatingRepository) GetRatingSummary(ctx context.Context, flagID uuid.UUID) (*models.RatingSummary, error) {
	query := `SELECT COALESCE(AVG(rating), 0), COUNT(*) FROM flag_ratings WHERE flag_id = $1`
	summary := &models.RatingSummary{Distribution: make(map[int16]int)}
	err := r.pool.QueryRow(ctx, query, flagID).Scan(&summary.Average, &summary.Count)
	if err != nil {
		return nil, err
	}
	if summary.Count == 0 {
		return summary, nil
	}

	distQuery := `SELECT rating, COUNT(*) FROM flag_ratings WHERE flag_id = $1 GROUP BY rating`
	rows, err := r.pool.Query(ctx, distQuery, flagID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var star int16
		var count int
		if err := rows.Scan(&star, &count); err != nil {
			return nil, err
		}
		summary.Distribution[star] = count
	}
	return summary, rows.Err()
}

func (r *RatingRepository) UpsertErrorStats(ctx context.Context, stat *models.FlagErrorStat) error {
	query := `
		INSERT INTO flag_error_stats (id, flag_id, environment_id, org_id, period_start, total_evaluations, error_count)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)
		ON CONFLICT (flag_id, environment_id, org_id, period_start) DO UPDATE SET
			total_evaluations = flag_error_stats.total_evaluations + EXCLUDED.total_evaluations,
			error_count = flag_error_stats.error_count + EXCLUDED.error_count`
	_, err := r.pool.Exec(ctx, query,
		stat.FlagID, stat.EnvironmentID, stat.OrgID,
		stat.PeriodStart, stat.TotalEvaluations, stat.ErrorCount,
	)
	return err
}

func (r *RatingRepository) GetErrorSummary(ctx context.Context, flagID uuid.UUID, period time.Duration) (*models.ErrorSummary, error) {
	query := `SELECT COALESCE(SUM(total_evaluations), 0), COALESCE(SUM(error_count), 0)
		FROM flag_error_stats
		WHERE flag_id = $1 AND period_start >= $2`
	since := time.Now().UTC().Add(-period)
	var totalEvals, errorCount int64
	err := r.pool.QueryRow(ctx, query, flagID, since).Scan(&totalEvals, &errorCount)
	if err != nil {
		return nil, err
	}
	pct := 0.0
	if totalEvals > 0 {
		pct = float64(errorCount) / float64(totalEvals) * 100
	}

	switch {
	case period <= 24*time.Hour:
		periodStr = "24h"
	case period <= 7*24*time.Hour:
		periodStr = "7d"
	default:
		periodStr = "30d"
	}
	return &models.ErrorSummary{Percentage: pct, Period: periodStr}, nil
}

func (r *RatingRepository) GetErrorsByOrg(ctx context.Context, flagID uuid.UUID, period time.Duration) ([]*models.OrgErrorBreakdown, error) {
	query := `SELECT org_id, SUM(total_evaluations), SUM(error_count)
		FROM flag_error_stats
		WHERE flag_id = $1 AND period_start >= $2
		GROUP BY org_id
		ORDER BY SUM(error_count) DESC`
	since := time.Now().UTC().Add(-period)
	rows, err := r.pool.Query(ctx, query, flagID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*models.OrgErrorBreakdown
	for rows.Next() {
		var b models.OrgErrorBreakdown
		if err := rows.Scan(&b.OrgID, &b.TotalEvaluations, &b.ErrorCount); err != nil {
			return nil, err
		}
		if b.TotalEvaluations > 0 {
			b.Percentage = float64(b.ErrorCount) / float64(b.TotalEvaluations) * 100
		}
		results = append(results, &b)
	}
	return results, rows.Err()
}

func (r *RatingRepository) ResolveFlagID(ctx context.Context, projectID uuid.UUID, flagKey string) (uuid.UUID, error) {
	query := `SELECT id FROM feature_flags WHERE project_id = $1 AND key = $2`
	var flagID uuid.UUID
	err := r.pool.QueryRow(ctx, query, projectID, flagKey).Scan(&flagID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("flag not found: project=%s key=%s", projectID, flagKey)
	}
	return flagID, nil
}

func (r *RatingRepository) GetSettingValue(ctx context.Context, orgID uuid.UUID, key string) (*models.Setting, error) {
	query := `SELECT id, org_id, key, value, updated_by, updated_at
		FROM settings WHERE org_id = $1 AND key = $2`
	var s models.Setting
	err := r.pool.QueryRow(ctx, query, orgID, key).Scan(
		&s.ID, &s.OrgID, &s.Key, &s.Value, &s.UpdatedBy, &s.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}
