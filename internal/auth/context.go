package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// OrgIDFromContext reads the org_id value set by the auth middleware and
// returns it as a uuid.UUID. Both JWT and API-key auth paths store the
// org id as a string, so a direct .(uuid.UUID) assertion on the raw
// context value panics. This helper is the safe way to read it.
//
// Returns (uuid.Nil, false) when the value is missing, not a string, or
// not a valid UUID.
func OrgIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	raw, exists := c.Get(ContextKeyOrgID)
	if !exists {
		return uuid.Nil, false
	}
	s, ok := raw.(string)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}
