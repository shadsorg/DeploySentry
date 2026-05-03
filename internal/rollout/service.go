package rollout

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shadsorg/deploysentry/internal/models"
)

// txBeginner is the minimal interface StrategyService requires from the
// connection pool. *pgxpool.Pool satisfies it; unit tests may inject a mock.
type txBeginner interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

// EffectiveStrategy is a strategy + metadata about where it came from.
type EffectiveStrategy struct {
	Strategy    *models.Strategy `json:"strategy"`
	OriginScope ScopeRef         `json:"origin_scope"`
	IsInherited bool             `json:"is_inherited"`
}

// AuditLogger is the minimal interface the services need to record audit trails.
// Concrete implementation wired at cmd/api level. A no-op is acceptable in tests.
type AuditLogger interface {
	Log(ctx context.Context, action string, actorID uuid.UUID, payload map[string]any) error
}

// StrategyService provides template CRUD + inheritance.
type StrategyService struct {
	pool  txBeginner
	repo  StrategyRepository
	audit AuditLogger
}

// NewStrategyService builds a StrategyService.
func NewStrategyService(pool *pgxpool.Pool, repo StrategyRepository, audit AuditLogger) *StrategyService {
	return newStrategyService(pool, repo, audit)
}

// newStrategyService is the internal constructor that accepts the txBeginner
// interface. Tests inject a lightweight mock; production code passes a *pgxpool.Pool.
func newStrategyService(pool txBeginner, repo StrategyRepository, audit AuditLogger) *StrategyService {
	return &StrategyService{pool: pool, repo: repo, audit: audit}
}

// ErrSystemStrategyImmutable is returned when a system template's delete/update is attempted.
var ErrSystemStrategyImmutable = errors.New("system strategy cannot be modified or deleted")

// ErrStrategyInUse is returned when Delete is blocked by a strategy_defaults reference.
var ErrStrategyInUse = errors.New("strategy is referenced by a default assignment")

// Create validates and persists a new strategy.
func (s *StrategyService) Create(ctx context.Context, st *models.Strategy) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("Create begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := s.CreateTx(ctx, tx, st); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("Create commit tx: %w", err)
	}
	return nil
}

// CreateTx is the tx-aware twin of Create. Used by the staging service so
// the create rides the same tx as the rest of the deploy batch.
func (s *StrategyService) CreateTx(ctx context.Context, tx pgx.Tx, st *models.Strategy) (uuid.UUID, error) {
	if err := ValidateStrategy(st); err != nil {
		return uuid.Nil, fmt.Errorf("validate: %w", err)
	}
	if err := s.repo.CreateTx(ctx, tx, st); err != nil {
		return uuid.Nil, fmt.Errorf("create: %w", err)
	}
	return st.ID, nil
}

// Update applies changes if the expected version matches the DB row.
func (s *StrategyService) Update(ctx context.Context, st *models.Strategy, expectedVersion int) error {
	existing, err := s.repo.Get(ctx, st.ID)
	if err != nil {
		return err
	}
	if existing.IsSystem {
		return ErrSystemStrategyImmutable
	}
	if err := ValidateStrategy(st); err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	return s.repo.Update(ctx, st, expectedVersion)
}

// Delete blocks on system templates or referenced strategies, then soft-deletes.
func (s *StrategyService) Delete(ctx context.Context, id uuid.UUID) error {
	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if existing.IsSystem {
		return ErrSystemStrategyImmutable
	}
	used, err := s.repo.IsReferenced(ctx, id)
	if err != nil {
		return err
	}
	if used {
		return ErrStrategyInUse
	}
	return s.repo.SoftDelete(ctx, id)
}

// EffectiveList returns all strategies visible at the leaf scope, including
// inherited ones. Names in more specific scopes shadow less-specific scopes.
func (s *StrategyService) EffectiveList(ctx context.Context, leaf ScopeRef, projectID, orgID *uuid.UUID) ([]*EffectiveStrategy, error) {
	ancestors := AncestorScopes(leaf, projectID, orgID)
	rows, err := s.repo.ListByAnyScope(ctx, ancestors)
	if err != nil {
		return nil, err
	}
	// Most-specific first in `ancestors`. Keep first occurrence per name.
	seen := map[string]bool{}
	byScope := map[ScopeRef][]*models.Strategy{}
	for _, r := range rows {
		byScope[ScopeRef{r.ScopeType, r.ScopeID}] = append(byScope[ScopeRef{r.ScopeType, r.ScopeID}], r)
	}
	var out []*EffectiveStrategy
	for _, anc := range ancestors {
		for _, r := range byScope[anc] {
			if seen[r.Name] {
				continue
			}
			seen[r.Name] = true
			out = append(out, &EffectiveStrategy{Strategy: r, OriginScope: anc, IsInherited: anc != leaf})
		}
	}
	return out, nil
}

// Get returns a single strategy by ID.
func (s *StrategyService) Get(ctx context.Context, id uuid.UUID) (*models.Strategy, error) {
	return s.repo.Get(ctx, id)
}

// GetByName returns a strategy by (scope, name), not searching ancestors.
func (s *StrategyService) GetByName(ctx context.Context, st models.ScopeType, sid uuid.UUID, name string) (*models.Strategy, error) {
	return s.repo.GetByName(ctx, st, sid, name)
}

// StrategyDefaultService encapsulates defaults CRUD + scope-inheritance resolution.
type StrategyDefaultService struct {
	repo StrategyDefaultRepository
}

// NewStrategyDefaultService builds a StrategyDefaultService.
func NewStrategyDefaultService(repo StrategyDefaultRepository) *StrategyDefaultService {
	return &StrategyDefaultService{repo: repo}
}

// Upsert writes the default row.
func (s *StrategyDefaultService) Upsert(ctx context.Context, d *models.StrategyDefault) error {
	return s.repo.Upsert(ctx, d)
}

// List returns rows defined directly on the scope (no inheritance).
func (s *StrategyDefaultService) List(ctx context.Context, st models.ScopeType, sid uuid.UUID) ([]*models.StrategyDefault, error) {
	return s.repo.ListByScope(ctx, st, sid)
}

// Resolve walks ancestors and returns the most-specific matching default.
func (s *StrategyDefaultService) Resolve(ctx context.Context, leaf ScopeRef, projectID, orgID *uuid.UUID, env *string, target *models.TargetType) (*models.StrategyDefault, error) {
	ancestors := AncestorScopes(leaf, projectID, orgID)
	var allRows []*models.StrategyDefault
	for _, anc := range ancestors {
		rows, err := s.repo.ListByScope(ctx, anc.Type, anc.ID)
		if err != nil {
			return nil, err
		}
		allRows = append(allRows, rows...)
	}
	return ResolveDefault(allRows, ancestors, env, target), nil
}

// Delete removes a default row by ID.
func (s *StrategyDefaultService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// RolloutPolicyService encapsulates policy CRUD + scope-inheritance resolution.
type RolloutPolicyService struct {
	repo RolloutPolicyRepository
}

// NewRolloutPolicyService builds a RolloutPolicyService.
func NewRolloutPolicyService(repo RolloutPolicyRepository) *RolloutPolicyService {
	return &RolloutPolicyService{repo: repo}
}

// Upsert writes the policy row.
func (s *RolloutPolicyService) Upsert(ctx context.Context, p *models.RolloutPolicy) error {
	return s.repo.Upsert(ctx, p)
}

// List returns rows defined directly on the scope (no inheritance).
func (s *RolloutPolicyService) List(ctx context.Context, st models.ScopeType, sid uuid.UUID) ([]*models.RolloutPolicy, error) {
	return s.repo.ListByScope(ctx, st, sid)
}

// Resolve walks ancestors and returns the most-specific matching policy.
// If no row matches, returns nil (caller treats as "off" = immediate-apply).
func (s *RolloutPolicyService) Resolve(ctx context.Context, leaf ScopeRef, projectID, orgID *uuid.UUID, env *string, target *models.TargetType) (*models.RolloutPolicy, error) {
	ancestors := AncestorScopes(leaf, projectID, orgID)
	var allRows []*models.RolloutPolicy
	for _, anc := range ancestors {
		rows, err := s.repo.ListByScope(ctx, anc.Type, anc.ID)
		if err != nil {
			return nil, err
		}
		allRows = append(allRows, rows...)
	}
	return ResolvePolicy(allRows, ancestors, env, target), nil
}

// Delete removes a policy row by ID.
func (s *RolloutPolicyService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
