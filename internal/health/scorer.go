package health

// Weight defines a named weight for a health check in the scoring formula.
type Weight struct {
	CheckName string  `json:"check_name"`
	Weight    float64 `json:"weight"`
}

// HealthScorer computes weighted health scores from a set of check results.
// Each check can have a configurable weight, and the final score is the
// weighted average of all check scores normalized to [0.0, 1.0].
type HealthScorer struct {
	weights map[string]float64
	defaultWeight float64
}

// NewHealthScorer creates a new HealthScorer with the given weight configuration.
// Checks not present in the weights map receive the default weight.
func NewHealthScorer(weights []Weight, defaultWeight float64) *HealthScorer {
	wm := make(map[string]float64, len(weights))
	for _, w := range weights {
		wm[w.CheckName] = w.Weight
	}
	if defaultWeight <= 0 {
		defaultWeight = 1.0
	}
	return &HealthScorer{
		weights:       wm,
		defaultWeight: defaultWeight,
	}
}

// ComputeScore calculates the weighted average health score from the given
// check results. Returns a value in [0.0, 1.0] where 1.0 is fully healthy.
// If no results are provided, returns 1.0 (healthy by default).
func (s *HealthScorer) ComputeScore(results []*CheckResult) float64 {
	if len(results) == 0 {
		return 1.0
	}

	var totalWeight float64
	var weightedSum float64

	for _, r := range results {
		w := s.weightFor(r.Name)
		totalWeight += w
		weightedSum += r.Score * w
	}

	if totalWeight == 0 {
		return 1.0
	}

	score := weightedSum / totalWeight
	return clamp(score, 0.0, 1.0)
}

// weightFor returns the configured weight for the named check,
// falling back to the default weight.
func (s *HealthScorer) weightFor(name string) float64 {
	if w, ok := s.weights[name]; ok {
		return w
	}
	return s.defaultWeight
}

// clamp restricts a value to the range [min, max].
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
