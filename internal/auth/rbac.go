package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Permission represents a specific action that can be performed on a resource.
type Permission string

const (
	// PermDeployCreate allows creating deployments.
	PermDeployCreate Permission = "deploy:create"
	// PermDeployRead allows reading deployment data.
	PermDeployRead Permission = "deploy:read"
	// PermDeployPromote allows promoting deployments.
	PermDeployPromote Permission = "deploy:promote"
	// PermDeployRollback allows rolling back deployments.
	PermDeployRollback Permission = "deploy:rollback"

	// PermFlagCreate allows creating feature flags.
	PermFlagCreate Permission = "flag:create"
	// PermFlagRead allows reading feature flag data.
	PermFlagRead Permission = "flag:read"
	// PermFlagUpdate allows updating feature flags.
	PermFlagUpdate Permission = "flag:update"
	// PermFlagToggle allows toggling feature flags.
	PermFlagToggle Permission = "flag:toggle"
	// PermFlagArchive allows archiving feature flags.
	PermFlagArchive Permission = "flag:archive"

	// PermReleaseCreate allows creating releases.
	PermReleaseCreate Permission = "release:create"
	// PermReleaseRead allows reading release data.
	PermReleaseRead Permission = "release:read"
	// PermReleasePromote allows promoting releases.
	PermReleasePromote Permission = "release:promote"

	// PermProjectManage allows managing project settings and members.
	PermProjectManage Permission = "project:manage"
	// PermOrgManage allows managing organization settings and members.
	PermOrgManage Permission = "org:manage"

	// PermAuditRead allows reading audit logs.
	PermAuditRead Permission = "audit:read"
)

// Role defines a named set of permissions.
type Role string

const (
	// RoleOwner has full access to all resources.
	RoleOwner Role = "owner"
	// RoleAdmin can manage projects, members, and perform all deployment operations.
	RoleAdmin Role = "admin"
	// RoleDeveloper can create and manage deployments, flags, and releases.
	RoleDeveloper Role = "developer"
	// RoleViewer has read-only access.
	RoleViewer Role = "viewer"
)

// rolePermissions maps each role to its set of allowed permissions.
var rolePermissions = map[Role][]Permission{
	RoleOwner: {
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle, PermFlagArchive,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
		PermProjectManage, PermOrgManage, PermAuditRead,
	},
	RoleAdmin: {
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle, PermFlagArchive,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
		PermProjectManage, PermAuditRead,
	},
	RoleDeveloper: {
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
	},
	RoleViewer: {
		PermDeployRead, PermFlagRead, PermReleaseRead,
	},
}

// RBACChecker provides role-based access control verification.
type RBACChecker struct{}

// NewRBACChecker creates a new RBACChecker.
func NewRBACChecker() *RBACChecker {
	return &RBACChecker{}
}

// HasPermission reports whether the given role includes the specified permission.
func (r *RBACChecker) HasPermission(role Role, perm Permission) bool {
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}

// GetPermissions returns all permissions granted to a role.
func (r *RBACChecker) GetPermissions(role Role) []Permission {
	perms, ok := rolePermissions[role]
	if !ok {
		return nil
	}
	result := make([]Permission, len(perms))
	copy(result, perms)
	return result
}

// RoleResolver looks up a user's role within a project or organization.
type RoleResolver interface {
	// GetProjectRole returns the user's role in the specified project.
	GetProjectRole(c *gin.Context, userID, projectID string) (Role, error)

	// GetOrgRole returns the user's role in the specified organization.
	GetOrgRole(c *gin.Context, userID, orgID string) (Role, error)
}

// RequirePermission returns a Gin middleware that checks whether the
// authenticated user has the specified permission for the target resource.
// It expects "user_id" and "role" to be set on the Gin context by a prior
// authentication middleware or role resolver.
func RequirePermission(rbac *RBACChecker, perm Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleValue, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "role not determined"})
			return
		}

		role, ok := roleValue.(Role)
		if !ok {
			roleStr, ok := roleValue.(string)
			if !ok {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid role format"})
				return
			}
			role = Role(roleStr)
		}

		if !rbac.HasPermission(role, perm) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":      "insufficient permissions",
				"required":   string(perm),
			})
			return
		}

		c.Next()
	}
}
