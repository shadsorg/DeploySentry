// Package models defines the core domain types for the DeploySentry platform.
package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Organization represents a top-level tenant in the platform.
// All projects, users, and resources are scoped to an organization.
type Organization struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Slug      string    `json:"slug" db:"slug"`
	OwnerID   uuid.UUID `json:"owner_id" db:"owner_id"`
	Plan      string    `json:"plan" db:"plan"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// OrgRole defines the roles a member can hold within an organization.
type OrgRole string

const (
	// OrgRoleOwner has full administrative control over the organization.
	OrgRoleOwner OrgRole = "owner"
	// OrgRoleAdmin can manage members and projects but cannot delete the org.
	OrgRoleAdmin OrgRole = "admin"
	// OrgRoleMember has standard read/write access to assigned projects.
	OrgRoleMember OrgRole = "member"
	// OrgRoleViewer has read-only access.
	OrgRoleViewer OrgRole = "viewer"
)

// OrgMember represents the association between a user and an organization,
// including their role and invitation status.
type OrgMember struct {
	ID        uuid.UUID `json:"id" db:"id"`
	OrgID     uuid.UUID `json:"org_id" db:"org_id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Role      OrgRole   `json:"role" db:"role"`
	InvitedBy uuid.UUID `json:"invited_by,omitempty" db:"invited_by"`
	JoinedAt  time.Time `json:"joined_at" db:"joined_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Validate checks that the Organization has all required fields populated
// and that they conform to platform constraints.
func (o *Organization) Validate() error {
	if o.Name == "" {
		return errors.New("organization name is required")
	}
	if len(o.Name) > 100 {
		return errors.New("organization name must be 100 characters or fewer")
	}
	if o.Slug == "" {
		return errors.New("organization slug is required")
	}
	if len(o.Slug) > 60 {
		return errors.New("organization slug must be 60 characters or fewer")
	}
	if o.OwnerID == uuid.Nil {
		return errors.New("organization owner_id is required")
	}
	return nil
}

// ValidRole reports whether the given OrgRole is one of the defined constants.
func ValidRole(r OrgRole) bool {
	switch r {
	case OrgRoleOwner, OrgRoleAdmin, OrgRoleMember, OrgRoleViewer:
		return true
	}
	return false
}

// Validate checks that the OrgMember has all required fields populated.
func (m *OrgMember) Validate() error {
	if m.OrgID == uuid.Nil {
		return errors.New("org_id is required")
	}
	if m.UserID == uuid.Nil {
		return errors.New("user_id is required")
	}
	if !ValidRole(m.Role) {
		return errors.New("invalid organization role")
	}
	return nil
}
