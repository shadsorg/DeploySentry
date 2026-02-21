package models

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AuthProvider identifies the external authentication provider a user signed in with.
type AuthProvider string

const (
	// AuthProviderGitHub represents GitHub OAuth authentication.
	AuthProviderGitHub AuthProvider = "github"
	// AuthProviderGoogle represents Google OAuth authentication.
	AuthProviderGoogle AuthProvider = "google"
	// AuthProviderEmail represents email/password authentication.
	AuthProviderEmail AuthProvider = "email"
)

// User represents an authenticated user of the platform.
type User struct {
	ID            uuid.UUID    `json:"id" db:"id"`
	Email         string       `json:"email" db:"email"`
	Name          string       `json:"name" db:"name"`
	AvatarURL     string       `json:"avatar_url,omitempty" db:"avatar_url"`
	AuthProvider  AuthProvider `json:"auth_provider" db:"auth_provider"`
	ProviderID    string       `json:"provider_id,omitempty" db:"provider_id"`
	PasswordHash  string       `json:"-" db:"password_hash"`
	EmailVerified bool         `json:"email_verified" db:"email_verified"`
	LastLoginAt   *time.Time   `json:"last_login_at,omitempty" db:"last_login_at"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at" db:"updated_at"`
}

// Validate checks that the User has all required fields and that they conform
// to platform constraints.
func (u *User) Validate() error {
	if u.Email == "" {
		return errors.New("email is required")
	}
	if !strings.Contains(u.Email, "@") {
		return errors.New("email must be a valid email address")
	}
	if u.Name == "" {
		return errors.New("name is required")
	}
	if len(u.Name) > 200 {
		return errors.New("name must be 200 characters or fewer")
	}
	switch u.AuthProvider {
	case AuthProviderGitHub, AuthProviderGoogle, AuthProviderEmail:
		// valid
	default:
		return errors.New("unsupported auth provider")
	}
	return nil
}
