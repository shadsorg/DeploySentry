package auth

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	// PermDeployManage allows managing deployment lifecycle (pause, resume).
	PermDeployManage Permission = "deploy:manage"

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

	// PermAPIKeyManage allows creating, revoking, and rotating API keys.
	PermAPIKeyManage Permission = "apikey:manage"
	// PermUserManage allows managing user accounts and memberships.
	PermUserManage Permission = "user:manage"
	// PermEnvDeploy allows deploying to specific environments.
	PermEnvDeploy Permission = "env:deploy"
	// PermBillingManage allows managing billing and subscription settings.
	PermBillingManage Permission = "billing:manage"

	// PermSettingsRead allows reading hierarchical settings.
	PermSettingsRead Permission = "settings:read"
	// PermSettingsWrite allows creating, updating, and deleting hierarchical settings.
	PermSettingsWrite Permission = "settings:write"

	// PermGroupManage allows managing groups and group memberships.
	PermGroupManage Permission = "group:manage"
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

	// RoleOrgOwner has full access to all projects and settings within an organization.
	RoleOrgOwner Role = "org:owner"
	// RoleOrgAdmin can manage projects, users, and billing within an organization.
	RoleOrgAdmin Role = "org:admin"
	// RoleProjectAdmin has full access within a project.
	RoleProjectAdmin Role = "project:admin"
	// RoleProjectEditor can create/edit deploys, flags, and releases within a project.
	RoleProjectEditor Role = "project:editor"
	// RoleProjectViewer has read-only access within a project.
	RoleProjectViewer Role = "project:viewer"
	// RoleEnvDeployer can deploy to specific environments only.
	RoleEnvDeployer Role = "environment:deployer"
)

// rolePermissions maps each role to its set of allowed permissions.
var rolePermissions = map[Role][]Permission{
	RoleOwner: {
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback, PermDeployManage,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle, PermFlagArchive,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
		PermProjectManage, PermOrgManage, PermAuditRead,
		PermSettingsRead, PermSettingsWrite,
		PermGroupManage,
	},
	RoleAdmin: {
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback, PermDeployManage,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle, PermFlagArchive,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
		PermProjectManage, PermAuditRead,
		PermSettingsRead, PermSettingsWrite,
		PermGroupManage,
	},
	RoleDeveloper: {
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback, PermDeployManage,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
		PermSettingsRead,
	},
	RoleViewer: {
		PermDeployRead, PermFlagRead, PermReleaseRead,
		PermSettingsRead,
	},

	// Granular organization-level roles.
	RoleOrgOwner: {
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback, PermDeployManage,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle, PermFlagArchive,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
		PermProjectManage, PermOrgManage, PermAuditRead,
		PermAPIKeyManage, PermUserManage, PermEnvDeploy, PermBillingManage,
		PermSettingsRead, PermSettingsWrite,
		PermGroupManage,
	},
	RoleOrgAdmin: {
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback, PermDeployManage,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle, PermFlagArchive,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
		PermProjectManage, PermAuditRead,
		PermAPIKeyManage, PermUserManage, PermEnvDeploy, PermBillingManage,
		PermSettingsRead, PermSettingsWrite,
		PermGroupManage,
	},

	// Granular project-level roles.
	RoleProjectAdmin: {
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback, PermDeployManage,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle, PermFlagArchive,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
		PermProjectManage, PermAuditRead, PermAPIKeyManage, PermUserManage, PermEnvDeploy,
	},
	RoleProjectEditor: {
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback, PermDeployManage,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
		PermEnvDeploy,
	},
	RoleProjectViewer: {
		PermDeployRead, PermFlagRead, PermReleaseRead,
	},

	// Environment-level role.
	RoleEnvDeployer: {
		PermDeployCreate, PermDeployRead, PermEnvDeploy,
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

	// GetEnvironmentRole returns the user's role for a specific environment.
	GetEnvironmentRole(c *gin.Context, userID, projectID, environmentID string) (Role, error)
}

// ResourceOwnershipChecker validates that a user or org owns a given resource.
type ResourceOwnershipChecker interface {
	// IsResourceOwner checks whether the given user or org owns the resource.
	IsResourceOwner(ctx context.Context, resourceType string, resourceID, userID, orgID uuid.UUID) (bool, error)
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
				"error":    "insufficient permissions",
				"required": string(perm),
			})
			return
		}

		c.Next()
	}
}

// RequireProjectPermission returns a Gin middleware that resolves the user's
// project-level role and checks whether they have the required permission.
// It reads "user_id" from the Gin context and "project_id" from the URL
// parameter. On success it sets "role" and "project_id" on the context.
func RequireProjectPermission(rbac *RBACChecker, resolver RoleResolver, perm Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDValue, exists := c.Get("user_id")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		userID, ok := userIDValue.(uuid.UUID)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid user identity"})
			return
		}

		projectID := c.Param("project_id")
		if projectID == "" {
			projectID = c.Query("project_id")
		}
		if projectID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "project_id is required"})
			return
		}

		if _, err := uuid.Parse(projectID); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
			return
		}

		role, err := resolver.GetProjectRole(c, userID.String(), projectID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "unable to determine project role"})
			return
		}

		if !rbac.HasPermission(role, perm) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":    "insufficient project permissions",
				"required": string(perm),
			})
			return
		}

		c.Set("role", role)
		c.Set("project_id", projectID)
		c.Next()
	}
}

// RequireEnvironmentPermission returns a Gin middleware that resolves the user's
// environment-level role and checks whether they have the required permission.
// It reads "user_id" from the Gin context and "project_id" / "environment_id"
// from URL parameters or query strings.
func RequireEnvironmentPermission(rbac *RBACChecker, resolver RoleResolver, perm Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDValue, exists := c.Get("user_id")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		userID, ok := userIDValue.(uuid.UUID)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid user identity"})
			return
		}

		projectID := c.Param("project_id")
		if projectID == "" {
			projectID = c.Query("project_id")
		}
		if projectID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "project_id is required"})
			return
		}

		environmentID := c.Param("environment_id")
		if environmentID == "" {
			environmentID = c.Query("environment_id")
		}
		if environmentID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "environment_id is required"})
			return
		}

		if _, err := uuid.Parse(projectID); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
			return
		}
		if _, err := uuid.Parse(environmentID); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid environment_id"})
			return
		}

		role, err := resolver.GetEnvironmentRole(c, userID.String(), projectID, environmentID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "unable to determine environment role"})
			return
		}

		if !rbac.HasPermission(role, perm) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":    "insufficient environment permissions",
				"required": string(perm),
			})
			return
		}

		c.Set("role", role)
		c.Set("project_id", projectID)
		c.Set("environment_id", environmentID)
		c.Next()
	}
}

// OrgRoleLookup resolves a user's org role by org slug or by user's default org.
type OrgRoleLookup interface {
	GetOrgIDBySlug(ctx context.Context, slug string) (uuid.UUID, error)
	GetOrgMemberRole(ctx context.Context, orgID, userID uuid.UUID) (string, error)
	GetUserDefaultOrgRole(ctx context.Context, userID uuid.UUID) (string, error)
}

// ResolveOrgRole returns a Gin middleware that looks up the user's org role
// and sets "role" on the Gin context. When :orgSlug is in the URL, it resolves
// by slug. Otherwise, it falls back to the user's highest org role.
// This must run before RequirePermission on all authenticated routes.
func ResolveOrgRole(lookup OrgRoleLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		// API key authentication does not carry a user_id, so the
		// user-centric lookups below don't apply. Map the key's scopes
		// directly to a role instead. This lets SDK clients that
		// authenticate with an API key reach the read-only flag
		// endpoints (listFlags, streamFlags, evaluate) which are
		// guarded by RequirePermission(PermFlagRead).
		if method, _ := c.Get("auth_method"); method == "api_key" {
			if _, exists := c.Get("role"); !exists {
				scopes, _ := c.Get("api_key_scopes")
				if scopeSlice, ok := scopes.([]string); ok {
					role := apiKeyScopesToRole(scopeSlice)
					if role != "" {
						c.Set("role", role)
					}
				}
			}
			c.Next()
			return
		}

		userIDValue, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		userID, ok := userIDValue.(uuid.UUID)
		if !ok {
			c.Next()
			return
		}

		orgSlug := c.Param("orgSlug")
		if orgSlug != "" {
			orgID, err := lookup.GetOrgIDBySlug(c.Request.Context(), orgSlug)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "organization not found"})
				return
			}

			role, err := lookup.GetOrgMemberRole(c.Request.Context(), orgID, userID)
			if err != nil {
				c.Next()
				return
			}

			c.Set("role", Role(role))
			c.Set("org_id", orgID.String())
			c.Next()
			return
		}

		// No orgSlug in URL — resolve user's default (highest) org role
		role, err := lookup.GetUserDefaultOrgRole(c.Request.Context(), userID)
		if err != nil {
			c.Next()
			return
		}

		c.Set("role", Role(role))
		c.Next()
	}
}

// apiKeyScopesToRole maps a set of API key scopes to the least-privilege
// org role that satisfies them. Used by ResolveOrgRole when authenticating
// with an API key so scope-checked SDK endpoints can also clear the
// RequirePermission role-based checks.
func apiKeyScopesToRole(scopes []string) Role {
	var write bool
	var read bool
	for _, s := range scopes {
		switch s {
		case "admin":
			return RoleOwner
		case "flag:toggle", "flags:toggle", "flag:create", "flags:create",
			"flag:update", "flags:update", "flag:write", "flags:write":
			write = true
		case "flag:read", "flags:read":
			read = true
		}
	}
	if write {
		return RoleDeveloper
	}
	if read {
		return RoleViewer
	}
	return ""
}

// ValidateResourceOwnership returns a Gin middleware that checks whether the
// authenticated user's organization owns the specified resource. It reads
// "user_id" and "org_id" from the Gin context and the resource ID from the
// URL parameter named by the resourceIDParam argument.
func ValidateResourceOwnership(checker ResourceOwnershipChecker, resourceType, resourceIDParam string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDValue, exists := c.Get("user_id")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		userID, ok := userIDValue.(uuid.UUID)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid user identity"})
			return
		}

		orgIDValue, exists := c.Get("org_id")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "organization context required"})
			return
		}

		orgIDStr, ok := orgIDValue.(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid organization identity"})
			return
		}

		orgID, err := uuid.Parse(orgIDStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid organization identity"})
			return
		}

		resourceIDStr := c.Param(resourceIDParam)
		if resourceIDStr == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "resource id is required"})
			return
		}

		resourceID, err := uuid.Parse(resourceIDStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid resource id"})
			return
		}

		isOwner, err := checker.IsResourceOwner(c.Request.Context(), resourceType, resourceID, userID, orgID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to validate resource ownership"})
			return
		}

		if !isOwner {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "resource does not belong to your organization"})
			return
		}

		c.Next()
	}
}
