package auth

import (
	"context"
	"net/http"
	"strings"

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

	// PermStatusWrite allows an app (or agent) to push self-reported
	// version + health status for an application/environment.
	PermStatusWrite Permission = "status:write"
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
		PermStatusWrite,
	},
	RoleAdmin: {
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback, PermDeployManage,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle, PermFlagArchive,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
		PermProjectManage, PermAuditRead,
		PermSettingsRead, PermSettingsWrite,
		PermGroupManage,
		PermStatusWrite,
	},
	RoleDeveloper: {
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback, PermDeployManage,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
		PermSettingsRead,
		PermStatusWrite,
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
		PermStatusWrite,
	},
	RoleOrgAdmin: {
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback, PermDeployManage,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle, PermFlagArchive,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
		PermProjectManage, PermAuditRead,
		PermAPIKeyManage, PermUserManage, PermEnvDeploy, PermBillingManage,
		PermSettingsRead, PermSettingsWrite,
		PermGroupManage,
		PermStatusWrite,
	},

	// Granular project-level roles.
	RoleProjectAdmin: {
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback, PermDeployManage,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle, PermFlagArchive,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
		PermProjectManage, PermAuditRead, PermAPIKeyManage, PermUserManage, PermEnvDeploy,
		PermStatusWrite,
	},
	RoleProjectEditor: {
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback, PermDeployManage,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
		PermEnvDeploy,
		PermStatusWrite,
	},
	RoleProjectViewer: {
		PermDeployRead, PermFlagRead, PermReleaseRead,
	},

	// Environment-level role.
	RoleEnvDeployer: {
		PermDeployCreate, PermDeployRead, PermEnvDeploy,
		PermStatusWrite,
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

// ResourceOwnershipChecker validates that a user or org owns a given resource.
type ResourceOwnershipChecker interface {
	// IsResourceOwner checks whether the given user or org owns the resource.
	IsResourceOwner(ctx context.Context, resourceType string, resourceID, userID, orgID uuid.UUID) (bool, error)
}

// permissionSatisfyingScopes maps each RBAC Permission to the API-key
// scopes that imply it. When a request authenticates via API key (no user
// role), RequirePermission consults this table instead of the role matrix.
// The `admin` scope is a universal superset — always sufficient — and
// handled separately in scopeSatisfies.
//
// Naming reconciliation: RBAC uses singular+action (deploy:create),
// scopes use plural+access (deploys:write). This table is the only place
// those two naming schemes cross.
var permissionSatisfyingScopes = map[Permission][]string{
	PermDeployCreate:    {"deploys:write"},
	PermDeployRead:      {"deploys:read", "deploys:write"},
	PermDeployPromote:   {"deploys:write"},
	PermDeployRollback:  {"deploys:write"},
	PermDeployManage:    {"deploys:write"},
	PermFlagCreate:      {"flags:write"},
	PermFlagRead:        {"flags:read", "flags:write"},
	PermFlagUpdate:      {"flags:write"},
	PermFlagToggle:      {"flags:write"},
	PermFlagArchive:     {"flags:write"},
	PermReleaseCreate:   {"releases:write"},
	PermReleaseRead:     {"releases:read", "releases:write"},
	PermReleasePromote:  {"releases:write"},
	PermStatusWrite:     {"status:write"},
	PermAPIKeyManage:    {"apikey:manage"},
	PermSettingsRead:    {"deploys:read", "flags:read"},
	PermSettingsWrite:   {"deploys:write", "flags:write"},
	// PermProjectManage / PermOrgManage / PermUserManage / PermBillingManage /
	// PermGroupManage / PermEnvDeploy / PermAuditRead are session-only; no API
	// key scope currently grants them. Listed explicitly so future scope
	// additions have an obvious home.
}

// scopeSatisfies reports whether the set of API key scopes includes one
// that satisfies the given permission. The `admin` scope is always
// sufficient (matches the model's HasScope superset semantics).
func scopeSatisfies(scopes []string, perm Permission) bool {
	for _, s := range scopes {
		if s == "admin" {
			return true
		}
	}
	for _, want := range permissionSatisfyingScopes[perm] {
		for _, have := range scopes {
			if have == want {
				return true
			}
		}
	}
	return false
}

// RequirePermission returns a Gin middleware that checks whether the
// authenticated caller has the specified permission.
//
// Two authorization paths are honored, in this order:
//  1. API-key auth: the calling key's scopes (set by AuthMiddleware as
//     `api_key_scopes`) are checked against permissionSatisfyingScopes.
//     `admin` scope is a universal superset.
//  2. Session / JWT auth: the user's role (set by the role resolver as
//     `role`) is checked against the RBAC permission matrix.
//
// Either path succeeding lets the request through.
func RequirePermission(rbac *RBACChecker, perm Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Path 1: API key scopes.
		if method, _ := c.Get("auth_method"); method == "api_key" {
			scopesVal, _ := c.Get("api_key_scopes")
			scopes, _ := scopesVal.([]string)
			if scopeSatisfies(scopes, perm) {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":          "insufficient permissions",
				"required":       string(perm),
				"api_key_scopes": scopes,
				"hint":           "add one of these scopes to the api key: " + scopeHintFor(perm),
			})
			return
		}

		// Path 2: session role.
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

// OrgRoleLookup resolves a user's org role by org slug or by user's default org.
type OrgRoleLookup interface {
	GetOrgIDBySlug(ctx context.Context, slug string) (uuid.UUID, error)
	GetOrgMemberRole(ctx context.Context, orgID, userID uuid.UUID) (string, error)
	GetUserDefaultOrgRole(ctx context.Context, userID uuid.UUID) (uuid.UUID, string, error)
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
		orgID, role, err := lookup.GetUserDefaultOrgRole(c.Request.Context(), userID)
		if err != nil {
			c.Next()
			return
		}

		c.Set("role", Role(role))
		c.Set("org_id", orgID.String())
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

// scopeHintFor returns a short comma-separated list of scopes that
// would satisfy the given permission, for inclusion in the 403 error
// body. Always mentions `admin` as the universal fallback.
func scopeHintFor(perm Permission) string {
	scopes := permissionSatisfyingScopes[perm]
	if len(scopes) == 0 {
		return "admin (this permission has no non-admin scope mapping yet)"
	}
	return strings.Join(append(scopes, "admin"), ", ")
}
