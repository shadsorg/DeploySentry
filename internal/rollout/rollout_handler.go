package rollout

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RolloutHandler serves rollout list, detail, and 6 runtime control endpoints.
type RolloutHandler struct {
	svc *RolloutService
}

// NewRolloutHandler builds the handler.
func NewRolloutHandler(svc *RolloutService) *RolloutHandler {
	return &RolloutHandler{svc: svc}
}

// RegisterRoutes mounts routes under /orgs/:orgSlug/.
func (h *RolloutHandler) RegisterRoutes(api *gin.RouterGroup) {
	org := api.Group("/orgs/:orgSlug")
	org.GET("/rollouts", h.list)
	org.GET("/rollouts/:id", h.get)
	org.POST("/rollouts/:id/pause", h.pause)
	org.POST("/rollouts/:id/resume", h.resume)
	org.POST("/rollouts/:id/rollback", h.rollback)
	org.POST("/rollouts/:id/promote", h.promote)
	org.POST("/rollouts/:id/approve", h.approve)
	org.POST("/rollouts/:id/force-promote", h.forcePromote)
	org.GET("/rollouts/:id/events", h.events)
}

func (h *RolloutHandler) list(c *gin.Context) {
	opts := RolloutListOptions{}
	if s := c.Query("status"); s != "" {
		st := models.RolloutStatus(s)
		opts.Status = &st
	}
	if t := c.Query("target_type"); t != "" {
		tt := models.TargetType(t)
		opts.TargetType = &tt
	}
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			opts.Limit = n
		}
	}
	rows, err := h.svc.List(c.Request.Context(), opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (h *RolloutHandler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	ro, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ro)
}

type reasonBody struct {
	Reason string `json:"reason"`
}

func actorFromContext(c *gin.Context) uuid.UUID {
	// Auth middleware sets "user_id" in the context. Fallback to uuid.Nil.
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(uuid.UUID); ok {
			return id
		}
	}
	return uuid.Nil
}

// ctx is a type alias used inside runControl closures to keep signatures terse.
type ctx = *gin.Context

func (h *RolloutHandler) runControl(c *gin.Context, fn func(c ctx, id uuid.UUID, actor uuid.UUID, reason string) error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body reasonBody
	_ = c.ShouldBindJSON(&body)
	actor := actorFromContext(c)
	if err := fn(c, id, actor, body.Reason); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func mapErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrReasonRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidStateForOp):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (h *RolloutHandler) pause(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.Pause(c.Request.Context(), id, actor, reason)
	})
}
func (h *RolloutHandler) resume(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.Resume(c.Request.Context(), id, actor, reason)
	})
}
func (h *RolloutHandler) rollback(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.Rollback(c.Request.Context(), id, actor, reason)
	})
}
func (h *RolloutHandler) promote(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.Promote(c.Request.Context(), id, actor, reason)
	})
}
func (h *RolloutHandler) approve(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.Approve(c.Request.Context(), id, actor, reason)
	})
}
func (h *RolloutHandler) forcePromote(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.ForcePromote(c.Request.Context(), id, actor, reason)
	})
}

// events streams rollout events (simple long-poll for now; SSE upgrade can be
// added without API change).
func (h *RolloutHandler) events(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	limit := 100
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	events, err := h.svc.Events(c.Request.Context(), id, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": events})
}
