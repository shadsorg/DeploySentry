package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Group represents a named collection of users within an organization,
// used to assign resource-level permissions in bulk.
type Group struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	OrgID       uuid.UUID  `json:"org_id" db:"org_id"`
	Name        string     `json:"name" db:"name"`
	Slug        string     `json:"slug" db:"slug"`
	Description string     `json:"description" db:"description"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty" db:"created_by"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// Validate checks that the Group has all required fields.
func (g *Group) Validate() error {
	if g.OrgID == uuid.Nil {
		return errors.New("org_id is required")
	}
	if g.Name == "" {
		return errors.New("name is required")
	}
	if g.Slug == "" {
		return errors.New("slug is required")
	}
	return nil
}

// GroupMember represents the membership of a user in a group.
type GroupMember struct {
	GroupID   uuid.UUID `json:"group_id" db:"group_id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ResourcePermission defines the level of access granted to a resource.
type ResourcePermission string

const (
	// PermissionRead allows viewing the resource.
	PermissionRead ResourcePermission = "read"
	// PermissionWrite allows modifying the resource.
	PermissionWrite ResourcePermission = "write"
)

// ResourceGrant assigns a permission on a specific project or application
// to either a single user or a group.
type ResourceGrant struct {
	ID            uuid.UUID          `json:"id" db:"id"`
	OrgID         uuid.UUID          `json:"org_id" db:"org_id"`
	ProjectID     *uuid.UUID         `json:"project_id,omitempty" db:"project_id"`
	ApplicationID *uuid.UUID         `json:"application_id,omitempty" db:"application_id"`
	UserID        *uuid.UUID         `json:"user_id,omitempty" db:"user_id"`
	GroupID       *uuid.UUID         `json:"group_id,omitempty" db:"group_id"`
	Permission    ResourcePermission `json:"permission" db:"permission"`
	GrantedBy     *uuid.UUID         `json:"granted_by,omitempty" db:"granted_by"`
	CreatedAt     time.Time          `json:"created_at" db:"created_at"`
}

// Validate checks that the ResourceGrant has all required fields and exactly-one constraints.
func (rg *ResourceGrant) Validate() error {
	if rg.OrgID == uuid.Nil {
		return errors.New("org_id is required")
	}
	if rg.ProjectID == nil && rg.ApplicationID == nil {
		return errors.New("exactly one of project_id or application_id is required")
	}
	if rg.ProjectID != nil && rg.ApplicationID != nil {
		return errors.New("exactly one of project_id or application_id is required")
	}
	if rg.UserID == nil && rg.GroupID == nil {
		return errors.New("exactly one of user_id or group_id is required")
	}
	if rg.UserID != nil && rg.GroupID != nil {
		return errors.New("exactly one of user_id or group_id is required")
	}
	if rg.Permission != PermissionRead && rg.Permission != PermissionWrite {
		return errors.New("permission must be 'read' or 'write'")
	}
	return nil
}
