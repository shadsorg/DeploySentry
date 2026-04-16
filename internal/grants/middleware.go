package grants

import (
	"net/http"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequireResourceAccess returns a Gin middleware that checks whether the
// authenticated user has the required permission on the target resource.
// It uses the grant-based access resolution system, returning 404 (not 403)
// when access is denied to avoid leaking resource existence.
func RequireResourceAccess(svc Service, requiredPerm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract user_id.
		uidVal, exists := c.Get("user_id")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		userID, ok := uidVal.(uuid.UUID)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid user identity"})
			return
		}

		// Extract org role (may be auth.Role or string).
		var orgRole string
		if roleVal, exists := c.Get("role"); exists {
			switch r := roleVal.(type) {
			case auth.Role:
				orgRole = string(r)
			case string:
				orgRole = r
			}
		}

		// Extract project_id (may be uuid.UUID or string).
		var projectID *uuid.UUID
		if val, exists := c.Get("project_id"); exists {
			switch v := val.(type) {
			case uuid.UUID:
				projectID = &v
			case string:
				if parsed, err := uuid.Parse(v); err == nil {
					projectID = &parsed
				}
			}
		}

		// Extract application_id (may be uuid.UUID or string).
		var applicationID *uuid.UUID
		if val, exists := c.Get("application_id"); exists {
			switch v := val.(type) {
			case uuid.UUID:
				applicationID = &v
			case string:
				if parsed, err := uuid.Parse(v); err == nil {
					applicationID = &parsed
				}
			}
		}

		// If neither resource ID is present, let the handler deal with it.
		if projectID == nil && applicationID == nil {
			c.Next()
			return
		}

		perm, err := svc.ResolveAccess(c.Request.Context(), userID, orgRole, projectID, applicationID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve access"})
			return
		}

		// Denied — return 404 to hide resource existence.
		if perm == nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// Check write requirement.
		if requiredPerm == "write" && *perm == models.PermissionRead {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "write access required"})
			return
		}

		c.Set("resource_permission", *perm)
		c.Next()
	}
}
