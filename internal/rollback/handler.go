package rollback

import (
	"net/http"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for managing rollbacks.
type Handler struct {
	controller *RollbackController
	rbac       *auth.RBACChecker
}

// NewHandler creates a new rollback HTTP handler.
func NewHandler(controller *RollbackController) *Handler {
	return &Handler{
		controller: controller,
		rbac:       auth.NewRBACChecker(),
	}
}

// RegisterRoutes mounts all rollback API routes on the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	// Manual rollback trigger on a specific deployment.
	deployments := rg.Group("/deployments")
	{
		deployments.POST("/:id/rollback",
			auth.RequirePermission(h.rbac, auth.PermDeployRollback),
			h.triggerRollback,
		)
	}

	// Rollback history listing.
	rollbacks := rg.Group("/rollbacks")
	{
		rollbacks.GET("",
			auth.RequirePermission(h.rbac, auth.PermDeployRead),
			h.listRollbacks,
		)
	}
}

// triggerRollbackRequest is the JSON body for triggering a manual rollback.
type triggerRollbackRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// triggerRollback handles POST /api/v1/deployments/:id/rollback.
// It initiates a manual rollback for the specified deployment.
func (h *Handler) triggerRollback(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}

	var req triggerRollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.controller.TriggerManualRollback(c.Request.Context(), id, req.Reason); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "rolled_back", "deployment_id": id})
}

// listRollbacks handles GET /api/v1/rollbacks.
// It returns the history of rollback events.
func (h *Handler) listRollbacks(c *gin.Context) {
	history := h.controller.GetRollbackHistory()
	c.JSON(http.StatusOK, gin.H{"rollbacks": history})
}
