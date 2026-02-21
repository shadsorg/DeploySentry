package flags

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// Cache defines the interface for a flag evaluation cache.
type Cache interface {
	// GetFlag returns a cached flag, or nil if not found.
	GetFlag(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*models.FeatureFlag, error)

	// SetFlag stores a flag in the cache with a TTL.
	SetFlag(ctx context.Context, flag *models.FeatureFlag, ttl time.Duration) error

	// GetRules returns cached targeting rules for a flag, or nil if not found.
	GetRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error)

	// SetRules stores targeting rules in the cache with a TTL.
	SetRules(ctx context.Context, flagID uuid.UUID, rules []*models.TargetingRule, ttl time.Duration) error

	// Invalidate removes a flag and its rules from the cache.
	Invalidate(ctx context.Context, flagID uuid.UUID) error
}

// Evaluator is the flag evaluation engine. It resolves feature flag values
// for a given evaluation context by checking the cache, falling back to the
// database, and applying targeting rules in priority order.
type Evaluator struct {
	repo     FlagRepository
	cache    Cache
	cacheTTL time.Duration
}

// NewEvaluator creates a new flag evaluation engine.
func NewEvaluator(repo FlagRepository, cache Cache) *Evaluator {
	return &Evaluator{
		repo:     repo,
		cache:    cache,
		cacheTTL: 30 * time.Second,
	}
}

// Evaluate resolves the value of a feature flag for the given evaluation context.
// It first checks the cache for the flag and rules, falls back to the database,
// then applies targeting rules in priority order.
func (e *Evaluator) Evaluate(ctx context.Context, projectID, environmentID uuid.UUID, key string, evalCtx models.EvaluationContext) (*models.FlagEvaluationResult, error) {
	// Try cache first.
	flag, err := e.cache.GetFlag(ctx, projectID, environmentID, key)
	if err != nil || flag == nil {
		// Cache miss or error: fall back to database.
		flag, err = e.repo.GetFlagByKey(ctx, projectID, environmentID, key)
		if err != nil {
			return nil, fmt.Errorf("flag %q not found: %w", key, err)
		}
		// Populate cache asynchronously (best-effort).
		_ = e.cache.SetFlag(ctx, flag, e.cacheTTL)
	}

	// If the flag is disabled or archived, return the default value.
	if !flag.Enabled || flag.Archived {
		return &models.FlagEvaluationResult{
			FlagKey: flag.Key,
			Enabled: false,
			Value:   flag.DefaultValue,
			Reason:  "flag_disabled",
		}, nil
	}

	// Load targeting rules.
	rules, err := e.cache.GetRules(ctx, flag.ID)
	if err != nil || rules == nil {
		rules, err = e.repo.ListRules(ctx, flag.ID)
		if err != nil {
			return nil, fmt.Errorf("loading rules for flag %q: %w", key, err)
		}
		_ = e.cache.SetRules(ctx, flag.ID, rules, e.cacheTTL)
	}

	// Apply rules in priority order (lower priority number = higher precedence).
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		match, err := e.evaluateRule(rule, evalCtx, flag.Key)
		if err != nil {
			// Rule evaluation errors are non-fatal; skip the rule.
			continue
		}
		if match {
			return &models.FlagEvaluationResult{
				FlagKey: flag.Key,
				Enabled: true,
				Value:   rule.Value,
				Reason:  "rule_match",
				RuleID:  rule.ID.String(),
			}, nil
		}
	}

	// No rules matched: return the default value with the flag enabled.
	return &models.FlagEvaluationResult{
		FlagKey: flag.Key,
		Enabled: true,
		Value:   flag.DefaultValue,
		Reason:  "default",
	}, nil
}

// evaluateRule dispatches to the appropriate rule evaluator based on rule type.
func (e *Evaluator) evaluateRule(rule *models.TargetingRule, evalCtx models.EvaluationContext, flagKey string) (bool, error) {
	switch rule.RuleType {
	case models.RuleTypePercentage:
		return evaluatePercentageRule(rule, evalCtx, flagKey), nil
	case models.RuleTypeUserTarget:
		return evaluateUserTargetRule(rule, evalCtx), nil
	case models.RuleTypeAttribute:
		return evaluateAttributeRule(rule, evalCtx), nil
	case models.RuleTypeSchedule:
		return evaluateScheduleRule(rule), nil
	case models.RuleTypeSegment:
		// Segment evaluation would require loading segment membership data.
		// Returning false as a stub.
		return false, nil
	default:
		return false, fmt.Errorf("unknown rule type: %s", rule.RuleType)
	}
}

// HashPercentage computes a deterministic hash-based percentage (0-99) for a
// given flag key and user ID. This ensures consistent bucketing across
// evaluations for the same user and flag.
func HashPercentage(flagKey, userID string) int {
	h := sha256.New()
	h.Write([]byte(flagKey))
	h.Write([]byte(":"))
	h.Write([]byte(userID))
	sum := h.Sum(nil)
	// Use the first 4 bytes as a uint32 and mod by 100.
	n := binary.BigEndian.Uint32(sum[:4])
	return int(n % 100)
}
