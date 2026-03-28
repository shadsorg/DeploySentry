package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// APIKeyScope defines the permission scope that an API key grants.
type APIKeyScope string

const (
	// APIKeyScopeReadFlags allows reading feature flag configurations.
	APIKeyScopeReadFlags APIKeyScope = "flags:read"
	// APIKeyScopeWriteFlags allows creating and updating feature flags.
	APIKeyScopeWriteFlags APIKeyScope = "flags:write"
	// APIKeyScopeReadDeploys allows reading deployment information.
	APIKeyScopeReadDeploys APIKeyScope = "deploys:read"
	// APIKeyScopeWriteDeploys allows creating and managing deployments.
	APIKeyScopeWriteDeploys APIKeyScope = "deploys:write"
	// APIKeyScopeReadReleases allows reading release information.
	APIKeyScopeReadReleases APIKeyScope = "releases:read"
	// APIKeyScopeWriteReleases allows creating and managing releases.
	APIKeyScopeWriteReleases APIKeyScope = "releases:write"
	// APIKeyScopeAdmin grants full administrative access.
	APIKeyScopeAdmin APIKeyScope = "admin"
)

// APIKey represents a machine-to-machine API key that grants scoped access
// to the platform without user authentication.
type APIKey struct {
	ID            uuid.UUID     `json:"id" db:"id"`
	OrgID         *uuid.UUID    `json:"org_id,omitempty" db:"org_id"`
	ProjectID     *uuid.UUID    `json:"project_id,omitempty" db:"project_id"`
	ApplicationID *uuid.UUID    `json:"application_id,omitempty" db:"application_id"`
	EnvironmentID *uuid.UUID    `json:"environment_id,omitempty" db:"environment_id"`
	Name          string        `json:"name" db:"name"`
	KeyPrefix     string        `json:"key_prefix" db:"key_prefix"`
	KeyHash       string        `json:"-" db:"key_hash"`
	Scopes        []APIKeyScope `json:"scopes" db:"-"`
	ExpiresAt     *time.Time    `json:"expires_at,omitempty" db:"expires_at"`
	LastUsedAt    *time.Time    `json:"last_used_at,omitempty" db:"last_used_at"`
	CreatedBy     uuid.UUID     `json:"created_by" db:"created_by"`
	CreatedAt     time.Time     `json:"created_at" db:"created_at"`
	RevokedAt     *time.Time    `json:"revoked_at,omitempty" db:"revoked_at"`
}

// Validate checks that the APIKey has all required fields populated.
func (k *APIKey) Validate() error {
	scopeCount := 0
	if k.OrgID != nil {
		scopeCount++
	}
	if k.ProjectID != nil {
		scopeCount++
	}
	if k.ApplicationID != nil {
		scopeCount++
	}
	if k.EnvironmentID != nil {
		scopeCount++
	}
	if scopeCount == 0 {
		return errors.New("exactly one scope (org_id, project_id, application_id, environment_id) must be set")
	}
	if scopeCount > 1 {
		return errors.New("only one scope (org_id, project_id, application_id, environment_id) may be set")
	}
	if k.Name == "" {
		return errors.New("name is required")
	}
	if len(k.Scopes) == 0 {
		return errors.New("at least one scope is required")
	}
	for _, scope := range k.Scopes {
		if !validScope(scope) {
			return errors.New("invalid scope: " + string(scope))
		}
	}
	return nil
}

// IsExpired reports whether the API key has passed its expiration time.
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().UTC().After(*k.ExpiresAt)
}

// IsRevoked reports whether the API key has been revoked.
func (k *APIKey) IsRevoked() bool {
	return k.RevokedAt != nil
}

// HasScope reports whether the API key grants the specified scope.
// An admin scope implicitly grants all other scopes.
func (k *APIKey) HasScope(scope APIKeyScope) bool {
	for _, s := range k.Scopes {
		if s == APIKeyScopeAdmin || s == scope {
			return true
		}
	}
	return false
}

// validScope reports whether the given scope is one of the defined constants.
func validScope(s APIKeyScope) bool {
	switch s {
	case APIKeyScopeReadFlags, APIKeyScopeWriteFlags,
		APIKeyScopeReadDeploys, APIKeyScopeWriteDeploys,
		APIKeyScopeReadReleases, APIKeyScopeWriteReleases,
		APIKeyScopeAdmin:
		return true
	}
	return false
}
