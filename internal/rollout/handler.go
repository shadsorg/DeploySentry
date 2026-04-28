package rollout

import (
	"errors"
	"io"
	"net/http"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ScopeResolver resolves URL slugs (orgSlug, projectSlug, appSlug) to scope IDs.
// The concrete implementation wraps the existing entities package at cmd/api wiring.
type ScopeResolver interface {
	ResolveOrg(c *gin.Context) (orgID uuid.UUID, err error)
	ResolveProject(c *gin.Context) (orgID, projectID uuid.UUID, err error)
	ResolveApp(c *gin.Context) (orgID, projectID, appID uuid.UUID, err error)
}

// Handler serves strategies, strategy-defaults, and rollout-policy endpoints.
type Handler struct {
	strategies *StrategyService
	defaults   *StrategyDefaultService
	policies   *RolloutPolicyService
	scope      ScopeResolver
}

// NewHandler builds a Handler.
func NewHandler(strat *StrategyService, defs *StrategyDefaultService, pols *RolloutPolicyService, scope ScopeResolver) *Handler {
	return &Handler{strategies: strat, defaults: defs, policies: pols, scope: scope}
}

// RegisterRoutes attaches all rollout endpoints to the given group.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	// Org scope
	org := api.Group("/orgs/:orgSlug")
	h.registerStrategyRoutes(org, "org")
	h.registerPolicyRoutes(org, "org")
	h.registerDefaultsRoutes(org, "org")
	// Project scope
	proj := api.Group("/orgs/:orgSlug/projects/:projectSlug")
	h.registerStrategyRoutes(proj, "project")
	h.registerPolicyRoutes(proj, "project")
	h.registerDefaultsRoutes(proj, "project")
	// App scope
	app := api.Group("/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug")
	h.registerStrategyRoutes(app, "app")
	h.registerPolicyRoutes(app, "app")
	h.registerDefaultsRoutes(app, "app")
}

func (h *Handler) registerStrategyRoutes(g *gin.RouterGroup, scope string) {
	g.GET("/strategies", h.listStrategies(scope))
	g.POST("/strategies", h.createStrategy(scope))
	g.GET("/strategies/:name", h.getStrategy(scope))
	g.PUT("/strategies/:name", h.updateStrategy(scope))
	g.DELETE("/strategies/:name", h.deleteStrategy(scope))
	g.POST("/strategies/import", h.importStrategy(scope))
	g.GET("/strategies/:name/export", h.exportStrategy(scope))
}

func (h *Handler) registerPolicyRoutes(g *gin.RouterGroup, scope string) {
	g.GET("/rollout-policy", h.listPolicies(scope))
	g.PUT("/rollout-policy", h.putPolicy(scope))
}

func (h *Handler) registerDefaultsRoutes(g *gin.RouterGroup, scope string) {
	g.GET("/strategy-defaults", h.listDefaults(scope))
	g.PUT("/strategy-defaults", h.putDefault(scope))
	g.DELETE("/strategy-defaults/:id", h.deleteDefault(scope))
}

// --- helpers ---

func (h *Handler) resolveLeaf(c *gin.Context, scope string) (ScopeRef, *uuid.UUID, *uuid.UUID, error) {
	switch scope {
	case "org":
		orgID, err := h.scope.ResolveOrg(c)
		if err != nil {
			return ScopeRef{}, nil, nil, err
		}
		return ScopeRef{models.ScopeOrg, orgID}, nil, nil, nil
	case "project":
		orgID, projID, err := h.scope.ResolveProject(c)
		if err != nil {
			return ScopeRef{}, nil, nil, err
		}
		return ScopeRef{models.ScopeProject, projID}, &projID, &orgID, nil
	case "app":
		orgID, projID, appID, err := h.scope.ResolveApp(c)
		if err != nil {
			return ScopeRef{}, nil, nil, err
		}
		return ScopeRef{models.ScopeApp, appID}, &projID, &orgID, nil
	}
	return ScopeRef{}, nil, nil, errors.New("unknown scope")
}

// --- strategies handlers ---

type createStrategyReq struct {
	Name                     string        `json:"name"`
	Description              string        `json:"description"`
	TargetType               string        `json:"target_type"`
	Steps                    []models.Step `json:"steps"`
	DefaultHealthThreshold   float64       `json:"default_health_threshold"`
	DefaultRollbackOnFailure bool          `json:"default_rollback_on_failure"`
}

func (h *Handler) createStrategy(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var req createStrategyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		s := &models.Strategy{
			ScopeType: leaf.Type, ScopeID: leaf.ID,
			Name: req.Name, Description: req.Description,
			TargetType:               models.TargetType(req.TargetType),
			Steps:                    req.Steps,
			DefaultHealthThreshold:   req.DefaultHealthThreshold,
			DefaultRollbackOnFailure: req.DefaultRollbackOnFailure,
		}
		if err := h.strategies.Create(c.Request.Context(), s); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, s)
	}
}

func (h *Handler) listStrategies(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, projID, orgID, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		items, err := h.strategies.EffectiveList(c.Request.Context(), leaf, projID, orgID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	}
}

func (h *Handler) getStrategy(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		s, err := h.strategies.GetByName(c.Request.Context(), leaf.Type, leaf.ID, c.Param("name"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, s)
	}
}

type updateStrategyReq struct {
	Description              string        `json:"description"`
	TargetType               string        `json:"target_type"`
	Steps                    []models.Step `json:"steps"`
	DefaultHealthThreshold   float64       `json:"default_health_threshold"`
	DefaultRollbackOnFailure bool          `json:"default_rollback_on_failure"`
	ExpectedVersion          int           `json:"expected_version"`
}

func (h *Handler) updateStrategy(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		existing, err := h.strategies.GetByName(c.Request.Context(), leaf.Type, leaf.ID, c.Param("name"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		var req updateStrategyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		existing.Description = req.Description
		existing.TargetType = models.TargetType(req.TargetType)
		existing.Steps = req.Steps
		existing.DefaultHealthThreshold = req.DefaultHealthThreshold
		existing.DefaultRollbackOnFailure = req.DefaultRollbackOnFailure
		if err := h.strategies.Update(c.Request.Context(), existing, req.ExpectedVersion); err != nil {
			if errors.Is(err, ErrSystemStrategyImmutable) {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, existing)
	}
}

func (h *Handler) deleteStrategy(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		existing, err := h.strategies.GetByName(c.Request.Context(), leaf.Type, leaf.ID, c.Param("name"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err := h.strategies.Delete(c.Request.Context(), existing.ID); err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, ErrSystemStrategyImmutable) {
				status = http.StatusForbidden
			} else if errors.Is(err, ErrStrategyInUse) {
				status = http.StatusConflict
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func (h *Handler) importStrategy(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		s, err := UnmarshalStrategyYAML(body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		s.ScopeType, s.ScopeID = leaf.Type, leaf.ID
		existing, err := h.strategies.GetByName(c.Request.Context(), leaf.Type, leaf.ID, s.Name)
		if err == nil && existing != nil {
			// Update path: carry over existing ID+version.
			existing.Description = s.Description
			existing.TargetType = s.TargetType
			existing.Steps = s.Steps
			existing.DefaultHealthThreshold = s.DefaultHealthThreshold
			existing.DefaultRollbackOnFailure = s.DefaultRollbackOnFailure
			if err := h.strategies.Update(c.Request.Context(), existing, existing.Version); err != nil {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, existing)
			return
		}
		if err := h.strategies.Create(c.Request.Context(), s); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, s)
	}
}

func (h *Handler) exportStrategy(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		s, err := h.strategies.GetByName(c.Request.Context(), leaf.Type, leaf.ID, c.Param("name"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		b, err := MarshalStrategyYAML(s)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Data(http.StatusOK, "application/yaml", b)
	}
}

// --- policies handlers ---

type putPolicyReq struct {
	Environment *string `json:"environment,omitempty"`
	TargetType  *string `json:"target_type,omitempty"`
	Enabled     bool    `json:"enabled"`
	Policy      string  `json:"policy"`
}

func (h *Handler) listPolicies(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		rows, err := h.policies.List(c.Request.Context(), leaf.Type, leaf.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": rows})
	}
}

func (h *Handler) putPolicy(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var req putPolicyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		pk := models.PolicyKind(req.Policy)
		switch pk {
		case models.PolicyOff, models.PolicyPrompt, models.PolicyMandate:
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "policy must be off, prompt, or mandate"})
			return
		}
		var ttPtr *models.TargetType
		if req.TargetType != nil {
			tt := models.TargetType(*req.TargetType)
			ttPtr = &tt
		}
		p := &models.RolloutPolicy{
			ScopeType: leaf.Type, ScopeID: leaf.ID,
			Environment: req.Environment, TargetType: ttPtr,
			Enabled: req.Enabled, Policy: pk,
		}
		if err := h.policies.Upsert(c.Request.Context(), p); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, p)
	}
}

// --- defaults handlers ---

type putDefaultReq struct {
	Environment  *string   `json:"environment,omitempty"`
	TargetType   *string   `json:"target_type,omitempty"`
	StrategyName string    `json:"strategy_name"` // friendlier than StrategyID in write path
	StrategyID   uuid.UUID `json:"strategy_id,omitempty"`
}

func (h *Handler) listDefaults(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		rows, err := h.defaults.List(c.Request.Context(), leaf.Type, leaf.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": rows})
	}
}

func (h *Handler) putDefault(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		leaf, _, _, err := h.resolveLeaf(c, scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var req putDefaultReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		sid := req.StrategyID
		if sid == uuid.Nil {
			s, err := h.strategies.GetByName(c.Request.Context(), leaf.Type, leaf.ID, req.StrategyName)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "strategy not found in scope"})
				return
			}
			sid = s.ID
		}
		var ttPtr *models.TargetType
		if req.TargetType != nil {
			tt := models.TargetType(*req.TargetType)
			ttPtr = &tt
		}
		d := &models.StrategyDefault{
			ScopeType: leaf.Type, ScopeID: leaf.ID,
			Environment: req.Environment, TargetType: ttPtr,
			StrategyID: sid,
		}
		if err := h.defaults.Upsert(c.Request.Context(), d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, d)
	}
}

func (h *Handler) deleteDefault(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		if err := h.defaults.Delete(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}
