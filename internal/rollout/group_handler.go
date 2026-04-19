package rollout

import (
	"net/http"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RolloutGroupScopeResolver resolves :orgSlug to an org ID.
type RolloutGroupScopeResolver interface {
	ResolveOrg(c *gin.Context) (uuid.UUID, error)
}

// RolloutGroupHandler serves rollout-group endpoints.
type RolloutGroupHandler struct {
	svc   *RolloutGroupService
	scope RolloutGroupScopeResolver
}

// NewRolloutGroupHandler builds a RolloutGroupHandler.
func NewRolloutGroupHandler(svc *RolloutGroupService, scope RolloutGroupScopeResolver) *RolloutGroupHandler {
	return &RolloutGroupHandler{svc: svc, scope: scope}
}

// RegisterRoutes mounts routes under /orgs/:orgSlug/.
func (h *RolloutGroupHandler) RegisterRoutes(api *gin.RouterGroup) {
	org := api.Group("/orgs/:orgSlug")
	org.GET("/rollout-groups", h.list)
	org.POST("/rollout-groups", h.create)
	org.GET("/rollout-groups/:id", h.get)
	org.PUT("/rollout-groups/:id", h.update)
	org.POST("/rollout-groups/:id/attach", h.attach)
}

func (h *RolloutGroupHandler) list(c *gin.Context) {
	orgID, err := h.scope.ResolveOrg(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rows, err := h.svc.List(c.Request.Context(), models.ScopeOrg, orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

type createGroupReq struct {
	Name               string `json:"name"`
	Description        string `json:"description"`
	CoordinationPolicy string `json:"coordination_policy,omitempty"`
}

func (h *RolloutGroupHandler) create(c *gin.Context) {
	orgID, err := h.scope.ResolveOrg(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var req createGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	policy := models.CoordinationPolicy(req.CoordinationPolicy)
	if policy == "" {
		policy = models.CoordinationIndependent
	}
	switch policy {
	case models.CoordinationIndependent, models.CoordinationPauseOnSiblingAbort, models.CoordinationCascadeAbort:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid coordination_policy"})
		return
	}
	g := &models.RolloutGroup{
		ScopeType:          models.ScopeOrg,
		ScopeID:            orgID,
		Name:               req.Name,
		Description:        req.Description,
		CoordinationPolicy: policy,
	}
	if err := h.svc.Create(c.Request.Context(), g); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, g)
}

func (h *RolloutGroupHandler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	g, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	members, _ := h.svc.Members(c.Request.Context(), id)
	c.JSON(http.StatusOK, gin.H{"group": g, "members": members})
}

type updateGroupReq struct {
	Name               string `json:"name"`
	Description        string `json:"description"`
	CoordinationPolicy string `json:"coordination_policy"`
}

func (h *RolloutGroupHandler) update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	existing, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	var req updateGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	policy := models.CoordinationPolicy(req.CoordinationPolicy)
	switch policy {
	case models.CoordinationIndependent, models.CoordinationPauseOnSiblingAbort, models.CoordinationCascadeAbort:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid coordination_policy"})
		return
	}
	existing.Name = req.Name
	existing.Description = req.Description
	existing.CoordinationPolicy = policy
	if err := h.svc.Update(c.Request.Context(), existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, existing)
}

type groupAttachReq struct {
	RolloutID uuid.UUID `json:"rollout_id"`
}

func (h *RolloutGroupHandler) attach(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req groupAttachReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.RolloutID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rollout_id is required"})
		return
	}
	if err := h.svc.Attach(c.Request.Context(), id, req.RolloutID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
