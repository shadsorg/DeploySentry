package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// FlagRating represents a user's star rating and optional comment on a feature flag.
type FlagRating struct {
	ID        uuid.UUID `json:"id" db:"id"`
	FlagID    uuid.UUID `json:"flag_id" db:"flag_id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	OrgID     uuid.UUID `json:"org_id" db:"org_id"`
	Rating    int16     `json:"rating" db:"rating"`
	Comment   string    `json:"comment,omitempty" db:"comment"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Validate checks that the FlagRating has all required fields and valid values.
func (r *FlagRating) Validate() error {
	if r.FlagID == uuid.Nil {
		return errors.New("flag_id is required")
	}
	if r.UserID == uuid.Nil {
		return errors.New("user_id is required")
	}
	if r.OrgID == uuid.Nil {
		return errors.New("org_id is required")
	}
	if r.Rating < 1 || r.Rating > 5 {
		return errors.New("rating must be between 1 and 5")
	}
	if len(r.Comment) > 2000 {
		return errors.New("comment must be 2000 characters or fewer")
	}
	return nil
}

// RatingSummary is the aggregate rating data returned in API responses.
type RatingSummary struct {
	Average      float64       `json:"average"`
	Count        int           `json:"count"`
	Distribution map[int16]int `json:"distribution"`
}
