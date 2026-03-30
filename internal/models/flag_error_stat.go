package models

import (
	"time"

	"github.com/google/uuid"
)

// FlagErrorStat represents aggregated error counts for a flag in a given
// environment, org, and hourly time bucket.
type FlagErrorStat struct {
	ID               uuid.UUID `json:"id" db:"id"`
	FlagID           uuid.UUID `json:"flag_id" db:"flag_id"`
	EnvironmentID    uuid.UUID `json:"environment_id" db:"environment_id"`
	OrgID            uuid.UUID `json:"org_id" db:"org_id"`
	PeriodStart      time.Time `json:"period_start" db:"period_start"`
	TotalEvaluations int64     `json:"total_evaluations" db:"total_evaluations"`
	ErrorCount       int64     `json:"error_count" db:"error_count"`
}

// ErrorSummary is the aggregate error data returned in API responses.
type ErrorSummary struct {
	Percentage float64 `json:"percentage"`
	Period     string  `json:"period"`
}

// OrgErrorBreakdown is the per-org error data returned for admin endpoints.
type OrgErrorBreakdown struct {
	OrgID            uuid.UUID `json:"org_id"`
	TotalEvaluations int64     `json:"total_evaluations"`
	ErrorCount       int64     `json:"error_count"`
	Percentage       float64   `json:"percentage"`
}
