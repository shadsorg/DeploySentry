package flags

import (
	"strconv"
	"strings"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
)

// CombineOperator defines how multiple conditions in a compound rule are combined.
type CombineOperator string

const (
	// CombineAND requires all conditions to match.
	CombineAND CombineOperator = "AND"
	// CombineOR requires at least one condition to match.
	CombineOR CombineOperator = "OR"
)

// CompoundCondition represents a single condition within a compound rule.
// Each condition evaluates an attribute against a value using an operator.
type CompoundCondition struct {
	Attribute string `json:"attribute"`
	Operator  string `json:"operator"`
	Value     string `json:"value"`
}

// evaluatePercentageRule uses deterministic hashing to determine whether a user
// falls within the configured traffic percentage. The hash is based on the flag
// key and user ID, ensuring consistent assignment across evaluations.
func evaluatePercentageRule(rule *models.TargetingRule, evalCtx models.EvaluationContext, flagKey string) bool {
	if rule.Percentage == nil {
		return false
	}
	if evalCtx.UserID == "" {
		return false
	}
	bucket := HashPercentage(flagKey, evalCtx.UserID)
	return bucket < *rule.Percentage
}

// evaluateUserTargetRule checks whether the evaluation context's user ID
// matches any of the target values configured in the rule.
func evaluateUserTargetRule(rule *models.TargetingRule, evalCtx models.EvaluationContext) bool {
	if evalCtx.UserID == "" {
		return false
	}
	for _, target := range rule.TargetValues {
		if evalCtx.UserID == target {
			return true
		}
	}
	return false
}

// evaluateAttributeRule checks whether the evaluation context contains an
// attribute matching the rule's criteria using the configured operator.
//
// Supported operators:
//   - eq: exact string equality
//   - neq: string inequality
//   - contains: substring match
//   - starts_with: prefix match
//   - ends_with: suffix match
//   - in: membership in a comma-separated list of values
//   - not_in: non-membership in a comma-separated list of values
//   - gt, gte, lt, lte: numeric comparison
func evaluateAttributeRule(rule *models.TargetingRule, evalCtx models.EvaluationContext) bool {
	if evalCtx.Attributes == nil {
		return false
	}
	attrValue, ok := evalCtx.Attributes[rule.Attribute]
	if !ok {
		return false
	}

	ruleValue := rule.Value

	switch rule.Operator {
	case "eq":
		return attrValue == ruleValue
	case "neq":
		return attrValue != ruleValue
	case "contains":
		return strings.Contains(attrValue, ruleValue)
	case "starts_with":
		return strings.HasPrefix(attrValue, ruleValue)
	case "ends_with":
		return strings.HasSuffix(attrValue, ruleValue)
	case "in":
		values := strings.Split(ruleValue, ",")
		for _, v := range values {
			if strings.TrimSpace(v) == attrValue {
				return true
			}
		}
		return false
	case "not_in":
		values := strings.Split(ruleValue, ",")
		for _, v := range values {
			if strings.TrimSpace(v) == attrValue {
				return false
			}
		}
		return true
	case "gt":
		return compareNumeric(attrValue, ruleValue) > 0
	case "gte":
		return compareNumeric(attrValue, ruleValue) >= 0
	case "lt":
		return compareNumeric(attrValue, ruleValue) < 0
	case "lte":
		return compareNumeric(attrValue, ruleValue) <= 0
	default:
		return false
	}
}

// evaluateScheduleRule checks whether the current time falls within the
// rule's configured start and end time window.
func evaluateScheduleRule(rule *models.TargetingRule) bool {
	now := time.Now().UTC()
	if rule.StartTime != nil && now.Before(*rule.StartTime) {
		return false
	}
	if rule.EndTime != nil && now.After(*rule.EndTime) {
		return false
	}
	return true
}

// evaluateCompoundRule evaluates multiple conditions combined with either AND
// or OR logic. For AND, all conditions must match. For OR, at least one
// condition must match.
//
//nolint:unused
func evaluateCompoundRule(operator CombineOperator, conditions []CompoundCondition, evalCtx models.EvaluationContext) bool {
	if len(conditions) == 0 {
		return false
	}

	switch operator {
	case CombineAND:
		for _, cond := range conditions {
			if !evaluateSingleCondition(cond, evalCtx) {
				return false
			}
		}
		return true
	case CombineOR:
		for _, cond := range conditions {
			if evaluateSingleCondition(cond, evalCtx) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// evaluateSingleCondition evaluates a single compound condition against the
// evaluation context. It reuses the same operator semantics as
// evaluateAttributeRule by constructing a temporary targeting rule.
//
//nolint:unused
func evaluateSingleCondition(cond CompoundCondition, evalCtx models.EvaluationContext) bool {
	rule := &models.TargetingRule{
		Attribute: cond.Attribute,
		Operator:  cond.Operator,
		Value:     cond.Value,
	}
	return evaluateAttributeRule(rule, evalCtx)
}

// compareNumeric parses two strings as float64 values and returns:
//
//	-1 if a < b, 0 if a == b, 1 if a > b.
//
// Returns 0 if either value cannot be parsed.
func compareNumeric(a, b string) int {
	av, errA := strconv.ParseFloat(a, 64)
	bv, errB := strconv.ParseFloat(b, 64)
	if errA != nil || errB != nil {
		return 0
	}
	switch {
	case av < bv:
		return -1
	case av > bv:
		return 1
	default:
		return 0
	}
}
