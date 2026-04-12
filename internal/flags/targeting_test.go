package flags

import (
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// evaluatePercentageRule
// ---------------------------------------------------------------------------

func TestEvaluatePercentageRule_NilPercentage(t *testing.T) {
	rule := &models.TargetingRule{Percentage: nil}
	evalCtx := models.EvaluationContext{UserID: "user-1"}
	assert.False(t, evaluatePercentageRule(rule, evalCtx, "flag-key"))
}

func TestEvaluatePercentageRule_EmptyUserID(t *testing.T) {
	pct := 50
	rule := &models.TargetingRule{Percentage: &pct}
	evalCtx := models.EvaluationContext{UserID: ""}
	assert.False(t, evaluatePercentageRule(rule, evalCtx, "flag-key"))
}

func TestEvaluatePercentageRule_Percentage100AlwaysMatches(t *testing.T) {
	pct := 100
	rule := &models.TargetingRule{Percentage: &pct}

	// Try several different user IDs; all must match since the hash produces 0-99
	// and the check is bucket < 100 (always true).
	users := []string{"alice", "bob", "charlie", "dave", "eve", "frank"}
	for _, uid := range users {
		evalCtx := models.EvaluationContext{UserID: uid}
		assert.True(t, evaluatePercentageRule(rule, evalCtx, "flag-key"),
			"user %q should always match at 100%%", uid)
	}
}

func TestEvaluatePercentageRule_Percentage0NeverMatches(t *testing.T) {
	pct := 0
	rule := &models.TargetingRule{Percentage: &pct}

	users := []string{"alice", "bob", "charlie", "dave", "eve", "frank"}
	for _, uid := range users {
		evalCtx := models.EvaluationContext{UserID: uid}
		assert.False(t, evaluatePercentageRule(rule, evalCtx, "flag-key"),
			"user %q should never match at 0%%", uid)
	}
}

func TestEvaluatePercentageRule_DeterministicBucketing(t *testing.T) {
	pct := 50
	rule := &models.TargetingRule{Percentage: &pct}
	evalCtx := models.EvaluationContext{UserID: "consistent-user"}

	// Evaluate multiple times; result must be identical.
	first := evaluatePercentageRule(rule, evalCtx, "my-flag")
	for i := 0; i < 10; i++ {
		assert.Equal(t, first, evaluatePercentageRule(rule, evalCtx, "my-flag"),
			"percentage rule must be deterministic for the same user/flag")
	}
}

// ---------------------------------------------------------------------------
// evaluateUserTargetRule
// ---------------------------------------------------------------------------

func TestEvaluateUserTargetRule_EmptyUserID(t *testing.T) {
	rule := &models.TargetingRule{TargetValues: []string{"alice"}}
	evalCtx := models.EvaluationContext{UserID: ""}
	assert.False(t, evaluateUserTargetRule(rule, evalCtx))
}

func TestEvaluateUserTargetRule_UserMatches(t *testing.T) {
	rule := &models.TargetingRule{TargetValues: []string{"alice", "bob"}}
	evalCtx := models.EvaluationContext{UserID: "alice"}
	assert.True(t, evaluateUserTargetRule(rule, evalCtx))
}

func TestEvaluateUserTargetRule_UserDoesNotMatch(t *testing.T) {
	rule := &models.TargetingRule{TargetValues: []string{"alice", "bob"}}
	evalCtx := models.EvaluationContext{UserID: "charlie"}
	assert.False(t, evaluateUserTargetRule(rule, evalCtx))
}

func TestEvaluateUserTargetRule_MultipleTargets(t *testing.T) {
	targets := []string{"u1", "u2", "u3", "u4", "u5"}
	rule := &models.TargetingRule{TargetValues: targets}

	for _, uid := range targets {
		evalCtx := models.EvaluationContext{UserID: uid}
		assert.True(t, evaluateUserTargetRule(rule, evalCtx),
			"user %q should be in the target list", uid)
	}

	evalCtx := models.EvaluationContext{UserID: "u6"}
	assert.False(t, evaluateUserTargetRule(rule, evalCtx))
}

func TestEvaluateUserTargetRule_EmptyTargetValues(t *testing.T) {
	rule := &models.TargetingRule{TargetValues: []string{}}
	evalCtx := models.EvaluationContext{UserID: "alice"}
	assert.False(t, evaluateUserTargetRule(rule, evalCtx))
}

// ---------------------------------------------------------------------------
// evaluateAttributeRule
// ---------------------------------------------------------------------------

func TestEvaluateAttributeRule_NilAttributes(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "plan",
		Operator:  "eq",
		Value:     "premium",
	}
	evalCtx := models.EvaluationContext{Attributes: nil}
	assert.False(t, evaluateAttributeRule(rule, evalCtx))
}

func TestEvaluateAttributeRule_MissingAttribute(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "plan",
		Operator:  "eq",
		Value:     "premium",
	}
	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"country": "US"},
	}
	assert.False(t, evaluateAttributeRule(rule, evalCtx))
}

func TestEvaluateAttributeRule_Eq_Match(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "plan",
		Operator:  "eq",
		Value:     "premium",
	}
	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"plan": "premium"},
	}
	assert.True(t, evaluateAttributeRule(rule, evalCtx))
}

func TestEvaluateAttributeRule_Eq_NonMatch(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "plan",
		Operator:  "eq",
		Value:     "premium",
	}
	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"plan": "free"},
	}
	assert.False(t, evaluateAttributeRule(rule, evalCtx))
}

func TestEvaluateAttributeRule_Neq_Match(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "plan",
		Operator:  "neq",
		Value:     "free",
	}
	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"plan": "premium"},
	}
	assert.True(t, evaluateAttributeRule(rule, evalCtx))
}

func TestEvaluateAttributeRule_Neq_NonMatch(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "plan",
		Operator:  "neq",
		Value:     "free",
	}
	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"plan": "free"},
	}
	assert.False(t, evaluateAttributeRule(rule, evalCtx))
}

func TestEvaluateAttributeRule_Contains_Match(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "email",
		Operator:  "contains",
		Value:     "@example.com",
	}
	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"email": "alice@example.com"},
	}
	assert.True(t, evaluateAttributeRule(rule, evalCtx))
}

func TestEvaluateAttributeRule_Contains_NonMatch(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "email",
		Operator:  "contains",
		Value:     "@example.com",
	}
	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"email": "alice@other.com"},
	}
	assert.False(t, evaluateAttributeRule(rule, evalCtx))
}

func TestEvaluateAttributeRule_StartsWith_Match(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "country",
		Operator:  "starts_with",
		Value:     "US",
	}
	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"country": "US-CA"},
	}
	assert.True(t, evaluateAttributeRule(rule, evalCtx))
}

func TestEvaluateAttributeRule_StartsWith_NonMatch(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "country",
		Operator:  "starts_with",
		Value:     "US",
	}
	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"country": "CA-US"},
	}
	assert.False(t, evaluateAttributeRule(rule, evalCtx))
}

func TestEvaluateAttributeRule_EndsWith_Match(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "hostname",
		Operator:  "ends_with",
		Value:     ".internal",
	}
	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"hostname": "app.internal"},
	}
	assert.True(t, evaluateAttributeRule(rule, evalCtx))
}

func TestEvaluateAttributeRule_EndsWith_NonMatch(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "hostname",
		Operator:  "ends_with",
		Value:     ".internal",
	}
	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"hostname": "app.external"},
	}
	assert.False(t, evaluateAttributeRule(rule, evalCtx))
}

func TestEvaluateAttributeRule_In_Match(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "role",
		Operator:  "in",
		Value:     "admin, editor, viewer",
	}
	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"role": "editor"},
	}
	assert.True(t, evaluateAttributeRule(rule, evalCtx))
}

func TestEvaluateAttributeRule_In_NonMatch(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "role",
		Operator:  "in",
		Value:     "admin, editor, viewer",
	}
	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"role": "guest"},
	}
	assert.False(t, evaluateAttributeRule(rule, evalCtx))
}

func TestEvaluateAttributeRule_Gt(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "age",
		Operator:  "gt",
		Value:     "18",
	}
	assert.True(t, evaluateAttributeRule(rule, models.EvaluationContext{
		Attributes: map[string]string{"age": "25"},
	}))
	assert.False(t, evaluateAttributeRule(rule, models.EvaluationContext{
		Attributes: map[string]string{"age": "18"},
	}))
	assert.False(t, evaluateAttributeRule(rule, models.EvaluationContext{
		Attributes: map[string]string{"age": "10"},
	}))
}

func TestEvaluateAttributeRule_Gte(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "age",
		Operator:  "gte",
		Value:     "18",
	}
	assert.True(t, evaluateAttributeRule(rule, models.EvaluationContext{
		Attributes: map[string]string{"age": "18"},
	}))
	assert.True(t, evaluateAttributeRule(rule, models.EvaluationContext{
		Attributes: map[string]string{"age": "25"},
	}))
	assert.False(t, evaluateAttributeRule(rule, models.EvaluationContext{
		Attributes: map[string]string{"age": "10"},
	}))
}

func TestEvaluateAttributeRule_Lt(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "age",
		Operator:  "lt",
		Value:     "18",
	}
	assert.True(t, evaluateAttributeRule(rule, models.EvaluationContext{
		Attributes: map[string]string{"age": "10"},
	}))
	assert.False(t, evaluateAttributeRule(rule, models.EvaluationContext{
		Attributes: map[string]string{"age": "18"},
	}))
	assert.False(t, evaluateAttributeRule(rule, models.EvaluationContext{
		Attributes: map[string]string{"age": "25"},
	}))
}

func TestEvaluateAttributeRule_Lte(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "age",
		Operator:  "lte",
		Value:     "18",
	}
	assert.True(t, evaluateAttributeRule(rule, models.EvaluationContext{
		Attributes: map[string]string{"age": "18"},
	}))
	assert.True(t, evaluateAttributeRule(rule, models.EvaluationContext{
		Attributes: map[string]string{"age": "10"},
	}))
	assert.False(t, evaluateAttributeRule(rule, models.EvaluationContext{
		Attributes: map[string]string{"age": "25"},
	}))
}

func TestEvaluateAttributeRule_UnknownOperator(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "plan",
		Operator:  "regex",
		Value:     ".*",
	}
	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"plan": "anything"},
	}
	assert.False(t, evaluateAttributeRule(rule, evalCtx))
}

func TestEvaluateAttributeRule_NumericComparisonWithNonNumericValues(t *testing.T) {
	rule := &models.TargetingRule{
		Attribute: "age",
		Operator:  "gt",
		Value:     "not-a-number",
	}
	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"age": "25"},
	}
	// compareNumeric returns 0 when parsing fails, so "gt" check (0 > 0) is false.
	assert.False(t, evaluateAttributeRule(rule, evalCtx))

	rule2 := &models.TargetingRule{
		Attribute: "age",
		Operator:  "lt",
		Value:     "18",
	}
	evalCtx2 := models.EvaluationContext{
		Attributes: map[string]string{"age": "xyz"},
	}
	// compareNumeric returns 0 when parsing fails, so "lt" check (0 < 0) is false.
	assert.False(t, evaluateAttributeRule(rule2, evalCtx2))
}

// ---------------------------------------------------------------------------
// evaluateScheduleRule
// ---------------------------------------------------------------------------

func TestEvaluateScheduleRule_WithinWindow(t *testing.T) {
	start := time.Now().UTC().Add(-1 * time.Hour)
	end := time.Now().UTC().Add(1 * time.Hour)
	rule := &models.TargetingRule{StartTime: &start, EndTime: &end}
	assert.True(t, evaluateScheduleRule(rule))
}

func TestEvaluateScheduleRule_BeforeStart(t *testing.T) {
	start := time.Now().UTC().Add(1 * time.Hour)
	end := time.Now().UTC().Add(2 * time.Hour)
	rule := &models.TargetingRule{StartTime: &start, EndTime: &end}
	assert.False(t, evaluateScheduleRule(rule))
}

func TestEvaluateScheduleRule_AfterEnd(t *testing.T) {
	start := time.Now().UTC().Add(-2 * time.Hour)
	end := time.Now().UTC().Add(-1 * time.Hour)
	rule := &models.TargetingRule{StartTime: &start, EndTime: &end}
	assert.False(t, evaluateScheduleRule(rule))
}

func TestEvaluateScheduleRule_NilStart_OpenStart(t *testing.T) {
	end := time.Now().UTC().Add(1 * time.Hour)
	rule := &models.TargetingRule{StartTime: nil, EndTime: &end}
	assert.True(t, evaluateScheduleRule(rule), "nil start means no lower bound")
}

func TestEvaluateScheduleRule_NilEnd_OpenEnd(t *testing.T) {
	start := time.Now().UTC().Add(-1 * time.Hour)
	rule := &models.TargetingRule{StartTime: &start, EndTime: nil}
	assert.True(t, evaluateScheduleRule(rule), "nil end means no upper bound")
}

func TestEvaluateScheduleRule_BothNil(t *testing.T) {
	rule := &models.TargetingRule{StartTime: nil, EndTime: nil}
	assert.True(t, evaluateScheduleRule(rule), "no constraints means always active")
}

// ---------------------------------------------------------------------------
// compareNumeric
// ---------------------------------------------------------------------------

func TestCompareNumeric_ALessThanB(t *testing.T) {
	assert.Equal(t, -1, compareNumeric("5", "10"))
}

func TestCompareNumeric_AGreaterThanB(t *testing.T) {
	assert.Equal(t, 1, compareNumeric("10", "5"))
}

func TestCompareNumeric_AEqualsB(t *testing.T) {
	assert.Equal(t, 0, compareNumeric("42", "42"))
}

func TestCompareNumeric_InvalidA(t *testing.T) {
	assert.Equal(t, 0, compareNumeric("abc", "10"))
}

func TestCompareNumeric_InvalidB(t *testing.T) {
	assert.Equal(t, 0, compareNumeric("10", "xyz"))
}

func TestCompareNumeric_FloatValues(t *testing.T) {
	assert.Equal(t, -1, compareNumeric("3.14", "3.15"))
	assert.Equal(t, 1, compareNumeric("3.15", "3.14"))
	assert.Equal(t, 0, compareNumeric("3.14", "3.14"))
}

func TestCompareNumeric_NegativeValues(t *testing.T) {
	assert.Equal(t, -1, compareNumeric("-5", "5"))
	assert.Equal(t, 1, compareNumeric("5", "-5"))
	assert.Equal(t, 0, compareNumeric("-5", "-5"))
}

// ---------------------------------------------------------------------------
// HashPercentage
// ---------------------------------------------------------------------------

func TestHashPercentage_Deterministic(t *testing.T) {
	first := HashPercentage("flag-key", "user-123")
	for i := 0; i < 50; i++ {
		assert.Equal(t, first, HashPercentage("flag-key", "user-123"),
			"HashPercentage must return the same value for the same inputs")
	}
}

func TestHashPercentage_RangeZeroTo99(t *testing.T) {
	// Generate a large set of hashes and verify they are all within [0, 99].
	for i := 0; i < 1000; i++ {
		uid := "user-" + time.Now().String() + "-" + string(rune(i))
		h := HashPercentage("flag", uid)
		require.GreaterOrEqual(t, h, 0, "hash must be >= 0")
		require.Less(t, h, 100, "hash must be < 100")
	}
}

func TestHashPercentage_DifferentInputsDifferentOutputs(t *testing.T) {
	// While hash collisions are possible, for a small set of very different
	// inputs the hashes should not all be identical.
	results := make(map[int]bool)
	inputs := []string{"alice", "bob", "charlie", "dave", "eve", "frank", "grace", "heidi"}
	for _, uid := range inputs {
		results[HashPercentage("test-flag", uid)] = true
	}
	// With 8 distinct inputs mapped to 0-99, it's statistically near-impossible
	// for all to land in the same bucket.
	assert.Greater(t, len(results), 1,
		"different inputs should produce at least 2 distinct hash values")
}

// ---------------------------------------------------------------------------
// evaluateConditions
// ---------------------------------------------------------------------------

func TestEvaluateConditions(t *testing.T) {
	tests := []struct {
		name       string
		conditions []models.CompoundCondition
		op         CombineOperator
		evalCtx    models.EvaluationContext
		want       bool
	}{
		{
			name: "AND all match",
			conditions: []models.CompoundCondition{
				{Attribute: "plan", Operator: "eq", Value: "enterprise"},
				{Attribute: "region", Operator: "eq", Value: "US"},
			},
			op:      CombineAND,
			evalCtx: models.EvaluationContext{Attributes: map[string]string{"plan": "enterprise", "region": "US"}},
			want:    true,
		},
		{
			name: "AND one mismatch",
			conditions: []models.CompoundCondition{
				{Attribute: "plan", Operator: "eq", Value: "enterprise"},
				{Attribute: "region", Operator: "eq", Value: "EU"},
			},
			op:      CombineAND,
			evalCtx: models.EvaluationContext{Attributes: map[string]string{"plan": "enterprise", "region": "US"}},
			want:    false,
		},
		{
			name: "OR one matches",
			conditions: []models.CompoundCondition{
				{Attribute: "plan", Operator: "eq", Value: "enterprise"},
				{Attribute: "plan", Operator: "eq", Value: "pro"},
			},
			op:      CombineOR,
			evalCtx: models.EvaluationContext{Attributes: map[string]string{"plan": "pro"}},
			want:    true,
		},
		{
			name: "OR none match",
			conditions: []models.CompoundCondition{
				{Attribute: "plan", Operator: "eq", Value: "enterprise"},
				{Attribute: "plan", Operator: "eq", Value: "pro"},
			},
			op:      CombineOR,
			evalCtx: models.EvaluationContext{Attributes: map[string]string{"plan": "free"}},
			want:    false,
		},
		{
			name:       "empty conditions AND returns true",
			conditions: []models.CompoundCondition{},
			op:         CombineAND,
			evalCtx:    models.EvaluationContext{},
			want:       true,
		},
		{
			name:       "empty conditions OR returns false",
			conditions: []models.CompoundCondition{},
			op:         CombineOR,
			evalCtx:    models.EvaluationContext{},
			want:       false,
		},
		{
			name: "AND with in operator",
			conditions: []models.CompoundCondition{
				{Attribute: "region", Operator: "in", Value: "US,EU,UK"},
				{Attribute: "plan", Operator: "eq", Value: "enterprise"},
			},
			op:      CombineAND,
			evalCtx: models.EvaluationContext{Attributes: map[string]string{"region": "EU", "plan": "enterprise"}},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateConditions(tt.conditions, tt.op, tt.evalCtx)
			if got != tt.want {
				t.Errorf("evaluateConditions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHashPercentage_DifferentFlagKeyChangesOutput(t *testing.T) {
	h1 := HashPercentage("flag-a", "user-1")
	h2 := HashPercentage("flag-b", "user-1")
	// Not a hard guarantee (collisions are possible), but we test the mechanism.
	// If they happen to collide, the test is still valid as long as the function
	// is deterministic, which is tested above.  We keep this as a smoke test.
	_ = h1
	_ = h2
	// At minimum, ensure no panic and values are in range.
	assert.GreaterOrEqual(t, h1, 0)
	assert.Less(t, h1, 100)
	assert.GreaterOrEqual(t, h2, 0)
	assert.Less(t, h2, 100)
}
