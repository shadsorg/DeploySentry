// internal/members/handler.go
package members

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/shadsorg/deploysentry/internal/auth"
	"github.com/shadsorg/deploysentry/internal/entities"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AuditWriter persists audit log entries. Mirrors flags.AuditWriter so we
// avoid an internal/auth import cycle.
type AuditWriter interface {
	WriteAuditLog(ctx context.Context, entry *models.AuditLogEntry) error
}

// Handler provides HTTP endpoints for member management.
type Handler struct {
	service  Service
	entities entities.EntityService
	rbac     *auth.RBACChecker
	audit    AuditWriter
}

// NewHandler creates a new members HTTP handler. audit may be nil — when
// provided, member lifecycle changes write audit_log rows (member.added,
// member.removed, member.role_changed) so they show up in the org audit
// page and the MembersPage activity panel.
func NewHandler(service Service, entityService entities.EntityService, rbac *auth.RBACChecker, audit AuditWriter) *Handler {
	return &Handler{service: service, entities: entityService, rbac: rbac, audit: audit}
}

// RegisterRoutes mounts member management routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	// Org members: /orgs/:orgSlug/members
	orgMembers := rg.Group("/orgs/:orgSlug/members")
	{
		orgMembers.GET("", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.listOrgMembers)
		orgMembers.POST("", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.addOrgMember)
		orgMembers.PUT("/:userId", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.updateOrgMemberRole)
		orgMembers.DELETE("/:userId", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.removeOrgMember)
	}
}

// resolveOrgID looks up the org by slug and returns its ID.
func (h *Handler) resolveOrgID(c *gin.Context) (uuid.UUID, bool) {
	org, err := h.entities.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return uuid.Nil, false
	}
	return org.ID, true
}

// ---------------------------------------------------------------------------
// Org member handlers
// ---------------------------------------------------------------------------

func (h *Handler) listOrgMembers(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	rows, err := h.service.ListOrgMembers(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list members"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"members": rows})
}

type addMemberRequest struct {
	Email string `json:"email" binding:"required"`
	Role  string `json:"role" binding:"required"`
}

func (h *Handler) addOrgMember(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	var req addMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("user_id")
	addedBy, _ := uid.(uuid.UUID)

	row, err := h.service.AddOrgMember(c.Request.Context(), orgID, req.Email, models.OrgRole(req.Role), addedBy)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found with that email"})
		case errors.Is(err, ErrAlreadyMember):
			c.JSON(http.StatusConflict, gin.H{"error": "user is already a member"})
		case errors.Is(err, ErrInvalidRole):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid role"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add member"})
		}
		return
	}
	newVal, _ := json.Marshal(map[string]string{
		"user_id": row.UserID.String(),
		"email":   row.Email,
		"role":    string(row.Role),
	})
	h.writeAudit(c, orgID, "member.added", row.UserID, "", string(newVal))
	c.JSON(http.StatusCreated, gin.H{"member": row})
}

type updateRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

func (h *Handler) updateOrgMemberRole(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	var req updateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	priorRole := ""
	if prior, err := h.service.GetOrgMember(c.Request.Context(), orgID, userID); err == nil && prior != nil {
		priorRole = string(prior.Role)
	}

	if err := h.service.UpdateOrgMemberRole(c.Request.Context(), orgID, userID, models.OrgRole(req.Role)); err != nil {
		switch {
		case errors.Is(err, ErrInvalidRole):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid role"})
		case errors.Is(err, ErrOwnerAssign):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case errors.Is(err, ErrLastOwner):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update role"})
		}
		return
	}
	oldVal, _ := json.Marshal(map[string]string{"role": priorRole})
	newVal, _ := json.Marshal(map[string]string{"role": req.Role})
	h.writeAudit(c, orgID, "member.role_changed", userID, string(oldVal), string(newVal))
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *Handler) removeOrgMember(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	priorRole := ""
	if prior, err := h.service.GetOrgMember(c.Request.Context(), orgID, userID); err == nil && prior != nil {
		priorRole = string(prior.Role)
	}

	if err := h.service.RemoveOrgMember(c.Request.Context(), orgID, userID); err != nil {
		switch {
		case errors.Is(err, ErrLastOwner):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove member"})
		}
		return
	}
	oldVal, _ := json.Marshal(map[string]string{
		"user_id": userID.String(),
		"role":    priorRole,
	})
	h.writeAudit(c, orgID, "member.removed", userID, string(oldVal), "")
	c.Status(http.StatusNoContent)
}

// writeAudit records an audit log entry for a member lifecycle change.
// Failures are logged but don't fail the request.
func (h *Handler) writeAudit(c *gin.Context, orgID uuid.UUID, action string, userID uuid.UUID, oldValue, newValue string) {
	if h.audit == nil {
		return
	}
	var actorID uuid.UUID
	if uid, exists := c.Get("user_id"); exists {
		actorID, _ = uid.(uuid.UUID)
	}
	entry := &models.AuditLogEntry{
		OrgID:      orgID,
		ActorID:    actorID,
		Action:     action,
		EntityType: "user",
		EntityID:   userID,
		OldValue:   oldValue,
		NewValue:   newValue,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		CreatedAt:  time.Now(),
	}
	if err := h.audit.WriteAuditLog(c.Request.Context(), entry); err != nil {
		log.Printf("members: failed to write audit log: %v", err)
	}
}
