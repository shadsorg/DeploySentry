package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
)

// RevertWriter is the persistence dependency for the revert endpoint.
// AuditLogRepository in internal/platform/database/postgres/audit.go satisfies
// it; we accept the interface here so tests can pass a fake.
type RevertWriter interface {
	GetAuditLogEntry(ctx context.Context, id uuid.UUID) (*models.AuditLogEntry, error)
	WriteAuditLog(ctx context.Context, entry *models.AuditLogEntry) error
}

// RevertHandlerHTTP exposes POST /audit-log/:entryId/revert.
type RevertHandlerHTTP struct {
	registry *RevertRegistry
	repo     RevertWriter
}

// NewRevertHandler creates a new RevertHandlerHTTP.
func NewRevertHandler(registry *RevertRegistry, repo RevertWriter) *RevertHandlerHTTP {
	return &RevertHandlerHTTP{registry: registry, repo: repo}
}

// RegisterRoutes mounts the revert endpoint on the given router group.
func (h *RevertHandlerHTTP) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/audit-log/:entryId/revert", h.revert)
}

func (h *RevertHandlerHTTP) revert(c *gin.Context) {
	entryID, err := uuid.Parse(c.Param("entryId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entry id"})
		return
	}

	var body struct {
		Force bool `json:"force"`
	}
	_ = json.NewDecoder(c.Request.Body).Decode(&body) // body is optional

	entry, err := h.repo.GetAuditLogEntry(c.Request.Context(), entryID)
	if err != nil {
		if isNotFoundErr(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "audit entry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load audit entry"})
		return
	}

	// Org isolation: the entry must belong to the caller's org.
	orgIDStr := c.GetString(ContextKeyOrgID)
	if orgIDStr == "" || entry.OrgID.String() != orgIDStr {
		c.JSON(http.StatusForbidden, gin.H{"error": "audit entry not in this org"})
		return
	}

	newAction, err := h.registry.Revert(c.Request.Context(), entry, body.Force)
	switch {
	case errors.Is(err, ErrNotRevertible):
		c.JSON(http.StatusBadRequest, gin.H{"error": "this action is not revertible"})
		return
	case errors.Is(err, ErrRevertRace):
		c.JSON(http.StatusConflict, gin.H{
			"error": "resource has changed since this entry; pass {\"force\":true} to revert anyway",
			"code":  "race",
		})
		return
	case err != nil:
		if strings.Contains(err.Error(), "malformed payload") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "revert failed"})
		return
	}

	// Append a new audit row recording the revert. The trail stays append-only.
	actorID := ActorUserID(c)
	revertEntry := &models.AuditLogEntry{
		ID:         uuid.New(),
		OrgID:      entry.OrgID,
		ProjectID:  entry.ProjectID,
		ActorID:    actorID,
		Action:     newAction,
		EntityType: entry.EntityType,
		EntityID:   entry.EntityID,
		OldValue:   entry.NewValue, // before revert (current state)
		NewValue:   entry.OldValue, // after revert (restored state)
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		CreatedAt:  time.Now(),
	}
	if err := h.repo.WriteAuditLog(c.Request.Context(), revertEntry); err != nil {
		// The revert succeeded but the audit write failed. Report error
		// but do not re-run the revert — that would be incorrect.
		c.JSON(http.StatusInternalServerError, gin.H{"error": "revert succeeded but audit write failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"reverted":       true,
		"audit_entry_id": revertEntry.ID,
		"action":         newAction,
	})
}

// isNotFoundErr returns true when the error indicates the audit entry was not
// found. pgx.ErrNoRows is the source; we string-match the wrapped message
// because importing pgx into the auth package would create a layer-violation
// the codebase otherwise avoids.
func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "no rows") || strings.Contains(s, "not found")
}
