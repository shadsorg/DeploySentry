package flags

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"sync/atomic"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
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

	// GetSegment returns a cached segment, or nil if not found.
	GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error)

	// SetSegment stores a segment in the cache with a TTL.
	SetSegment(ctx context.Context, segment *models.Segment, ttl time.Duration) error
}

// TelemetryLogger defines the interface for logging flag evaluation telemetry.
// Implementations may write to a database, message queue, or observability system.
type TelemetryLogger interface {
	// LogEvaluation records the result of a flag evaluation.
	LogEvaluation(ctx context.Context, result *models.FlagEvaluationResult, evalCtx models.EvaluationContext)
}

// CacheMetrics provides atomic counters for tracking cache hit and miss rates
// during flag evaluation. These counters can be read by monitoring systems.
type CacheMetrics struct {
	Hits   atomic.Int64
	Misses atomic.Int64
}

// Evaluator is the flag evaluation engine. It resolves feature flag values
// for a given evaluation context by checking the cache, falling back to the
// database, and applying targeting rules in priority order.
type Evaluator struct {
	repo       FlagRepository
	cache      Cache
	cacheTTL   time.Duration
	telemetry  TelemetryLogger
	sampleRate float64
	Metrics    CacheMetrics
	sfFlags    singleflight.Group
	sfRules    singleflight.Group
}

// NewEvaluator creates a new flag evaluation engine.
func NewEvaluator(repo FlagRepository, cache Cache) *Evaluator {
	return &Evaluator{
		repo:       repo,
		cache:      cache,
		cacheTTL:   30 * time.Second,
		sampleRate: 0.0,
	}
}

// SetTelemetry configures the telemetry logger and sample rate for evaluation
// logging. The sample rate must be between 0.0 (disabled) and 1.0 (log every
// evaluation). When set to 0.1, approximately 10% of evaluations are logged.
func (e *Evaluator) SetTelemetry(logger TelemetryLogger, sampleRate float64) {
	e.telemetry = logger
	if sampleRate < 0 {
		sampleRate = 0
	}
	if sampleRate > 1 {
		sampleRate = 1
	}
	e.sampleRate = sampleRate
}

// shouldSample returns true if this evaluation should be logged, based on the
// configured sample rate.
func (e *Evaluator) shouldSample() bool {
	if e.telemetry == nil || e.sampleRate <= 0 {
		return false
	}
	if e.sampleRate >= 1.0 {
		return true
	}
	return rand.Float64() < e.sampleRate
}

// Evaluate resolves the value of a feature flag for the given evaluation context.
// It first checks the cache for the flag and rules, falls back to the database,
// then applies targeting rules in priority order.
func (e *Evaluator) Evaluate(ctx context.Context, projectID, environmentID uuid.UUID, key string, evalCtx models.EvaluationContext) (*models.FlagEvaluationResult, error) {
	// Try cache first.
	flag, err := e.cache.GetFlag(ctx, projectID, environmentID, key)
	if err != nil || flag == nil {
		// Cache miss: fall back to database, coalescing concurrent requests.
		e.Metrics.Misses.Add(1)
		sfKey := fmt.Sprintf("%s:%s:%s", projectID, environmentID, key)
		val, sfErr, _ := e.sfFlags.Do(sfKey, func() (interface{}, error) {
			return e.repo.GetFlagByKey(ctx, projectID, environmentID, key)
		})
		if sfErr != nil {
			return nil, fmt.Errorf("flag %q not found: %w", key, sfErr)
		}
		flag = val.(*models.FeatureFlag)
		_ = e.cache.SetFlag(ctx, flag, e.cacheTTL)
	} else {
		e.Metrics.Hits.Add(1)
	}

	// Check archived first — archived flags are always off.
	if flag.Archived {
		result := &models.FlagEvaluationResult{
			FlagKey: flag.Key,
			Enabled: false,
			Value:   flag.DefaultValue,
			Reason:  "flag_disabled",
		}
		e.logTelemetry(ctx, result, evalCtx)
		return result, nil
	}

	// Determine enabled state: per-environment state takes precedence over
	// the global flag.Enabled field.
	enabled := flag.Enabled
	envState, err := e.repo.GetFlagEnvState(ctx, flag.ID, environmentID)
	if err == nil && envState != nil {
		enabled = envState.Enabled
	}

	if !enabled {
		result := &models.FlagEvaluationResult{
			FlagKey: flag.Key,
			Enabled: false,
			Value:   flag.DefaultValue,
			Reason:  "flag_disabled",
		}
		e.logTelemetry(ctx, result, evalCtx)
		return result, nil
	}

	// Load targeting rules.
	rules, err := e.cache.GetRules(ctx, flag.ID)
	if err != nil || rules == nil {
		e.Metrics.Misses.Add(1)
		val, sfErr, _ := e.sfRules.Do(flag.ID.String(), func() (interface{}, error) {
			return e.repo.ListRules(ctx, flag.ID)
		})
		if sfErr != nil {
			return nil, fmt.Errorf("loading rules for flag %q: %w", key, sfErr)
		}
		rules = val.([]*models.TargetingRule)
		_ = e.cache.SetRules(ctx, flag.ID, rules, e.cacheTTL)
	} else {
		e.Metrics.Hits.Add(1)
	}

	// Load per-environment rule activation states.
	enabledRules, err := e.repo.ListRuleEnvironmentStatesByEnv(ctx, flag.ID, environmentID)
	if err != nil {
		enabledRules = make(map[uuid.UUID]bool)
	}

	// Apply rules in priority order (lower priority number = higher precedence).
	for _, rule := range rules {
		if enabled, ok := enabledRules[rule.ID]; !ok || !enabled {
			continue
		}

		match, err := e.evaluateRule(ctx, rule, evalCtx, flag.Key)
		if err != nil {
			// Rule evaluation errors are non-fatal; skip the rule.
			continue
		}
		if match {
			result := &models.FlagEvaluationResult{
				FlagKey: flag.Key,
				Enabled: true,
				Value:   rule.Value,
				Reason:  "rule_match",
				RuleID:  rule.ID.String(),
			}
			e.logTelemetry(ctx, result, evalCtx)
			return result, nil
		}
	}

	// No rules matched: use per-environment value if set, else the global default.
	value := flag.DefaultValue
	if envState != nil && envState.Value != nil {
		// Strip surrounding quotes from JSON string values (e.g. `"true"` → `true`).
		raw := string(*envState.Value)
		if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
			raw = raw[1 : len(raw)-1]
		}
		value = raw
	}
	result := &models.FlagEvaluationResult{
		FlagKey: flag.Key,
		Enabled: true,
		Value:   value,
		Reason:  "default",
	}
	e.logTelemetry(ctx, result, evalCtx)
	return result, nil
}

// logTelemetry logs an evaluation result if telemetry is configured and the
// current evaluation passes the sampling check.
func (e *Evaluator) logTelemetry(ctx context.Context, result *models.FlagEvaluationResult, evalCtx models.EvaluationContext) {
	if e.shouldSample() {
		e.telemetry.LogEvaluation(ctx, result, evalCtx)
	}
}

// evaluateRule dispatches to the appropriate rule evaluator based on rule type.
func (e *Evaluator) evaluateRule(ctx context.Context, rule *models.TargetingRule, evalCtx models.EvaluationContext, flagKey string) (bool, error) {
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
		segment, err := e.loadSegment(ctx, rule.SegmentID)
		if err != nil {
			return false, err
		}
		conditions := make([]models.CompoundCondition, len(segment.Conditions))
		for i, sc := range segment.Conditions {
			conditions[i] = models.CompoundCondition{
				Attribute: sc.Attribute,
				Operator:  sc.Operator,
				Value:     sc.Value,
			}
		}
		return evaluateConditions(conditions, CombineOperator(segment.CombineOp), evalCtx), nil
	case models.RuleTypeCompound:
		return evaluateConditions(rule.Conditions, CombineOperator(rule.CombineOp), evalCtx), nil
	default:
		return false, fmt.Errorf("unknown rule type: %s", rule.RuleType)
	}
}

// loadSegment retrieves a segment from the cache, falling back to the repository.
// The segment is written back to cache after a repo load.
func (e *Evaluator) loadSegment(ctx context.Context, segmentID *uuid.UUID) (*models.Segment, error) {
	if segmentID == nil {
		return nil, fmt.Errorf("segment rule missing segment_id")
	}
	segment, err := e.cache.GetSegment(ctx, *segmentID)
	if err == nil && segment != nil {
		return segment, nil
	}
	segment, err = e.repo.GetSegment(ctx, *segmentID)
	if err != nil {
		return nil, fmt.Errorf("loading segment %s: %w", segmentID, err)
	}
	_ = e.cache.SetSegment(ctx, segment, e.cacheTTL)
	return segment, nil
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
