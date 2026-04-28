package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AuditLogFilter defines the query parameters for filtering audit log entries.
type AuditLogFilter struct {
	OrgID        uuid.UUID  `json:"org_id"`
	ProjectID    *uuid.UUID `json:"project_id,omitempty"`
	UserID       *uuid.UUID `json:"user_id,omitempty"`
	Action       string     `json:"action,omitempty"`
	ResourceType string     `json:"resource_type,omitempty"`
	ResourceID   *uuid.UUID `json:"resource_id,omitempty"`
	StartDate    *time.Time `json:"start_date,omitempty"`
	EndDate      *time.Time `json:"end_date,omitempty"`
	Limit        int        `json:"limit"`
	Offset       int        `json:"offset"`
}

// AuditLogRepository defines the persistence interface for querying audit logs.
type AuditLogRepository interface {
	// QueryAuditLogs retrieves audit log entries matching the given filter.
	QueryAuditLogs(ctx context.Context, filter AuditLogFilter) ([]*models.AuditLogEntry, int, error)
}

// AuditHandler provides HTTP endpoints for querying audit logs.
type AuditHandler struct {
	repo AuditLogRepository
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(repo AuditLogRepository) *AuditHandler {
	return &AuditHandler{repo: repo}
}

// RegisterRoutes mounts all audit log query routes on the given router group.
func (h *AuditHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/audit-log", h.queryAuditLog)
}

func (h *AuditHandler) queryAuditLog(c *gin.Context) {
	orgIDValue, ok := c.Get("org_id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization context required"})
		return
	}

	orgIDStr, ok := orgIDValue.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization identity"})
		return
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}

	filter := AuditLogFilter{
		OrgID:        orgID,
		Action:       c.Query("action"),
		ResourceType: c.Query("resource_type"),
	}

	// Parse optional project_id filter.
	if pidStr := c.Query("project_id"); pidStr != "" {
		pid, err := uuid.Parse(pidStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
			return
		}
		filter.ProjectID = &pid
	}

	// Parse optional user_id filter.
	if uidStr := c.Query("user_id"); uidStr != "" {
		uid, err := uuid.Parse(uidStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
			return
		}
		filter.UserID = &uid
	}

	// Parse optional resource_id filter.
	if ridStr := c.Query("resource_id"); ridStr != "" {
		rid, err := uuid.Parse(ridStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource_id"})
			return
		}
		filter.ResourceID = &rid
	}

	// Parse optional date range.
	if startStr := c.Query("start_date"); startStr != "" {
		t, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_date format, use RFC3339"})
			return
		}
		filter.StartDate = &t
	}

	if endStr := c.Query("end_date"); endStr != "" {
		t, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_date format, use RFC3339"})
			return
		}
		filter.EndDate = &t
	}

	// Parse pagination.
	limit, offset := parsePagination(c)
	filter.Limit = limit
	filter.Offset = offset

	entries, total, err := h.repo.QueryAuditLogs(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query audit logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"entries": entries,
		"total":   total,
		"limit":   filter.Limit,
		"offset":  filter.Offset,
	})
}
