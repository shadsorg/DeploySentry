package deploysentry

// EvaluationContext holds targeting attributes used during flag evaluation.
// Use NewEvaluationContext to construct one with the builder pattern.
type EvaluationContext struct {
	UserID     string                 `json:"user_id,omitempty"`
	OrgID      string                 `json:"org_id,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// EvaluationContextBuilder provides a fluent API for constructing an
// EvaluationContext.
type EvaluationContextBuilder struct {
	ctx EvaluationContext
}

// NewEvaluationContext returns a new builder for constructing an
// EvaluationContext.
func NewEvaluationContext() *EvaluationContextBuilder {
	return &EvaluationContextBuilder{
		ctx: EvaluationContext{
			Attributes: make(map[string]interface{}),
		},
	}
}

// UserID sets the user identifier for targeting.
func (b *EvaluationContextBuilder) UserID(id string) *EvaluationContextBuilder {
	b.ctx.UserID = id
	return b
}

// OrgID sets the organization identifier for targeting.
func (b *EvaluationContextBuilder) OrgID(id string) *EvaluationContextBuilder {
	b.ctx.OrgID = id
	return b
}

// Set adds a custom attribute to the evaluation context.
func (b *EvaluationContextBuilder) Set(key string, value interface{}) *EvaluationContextBuilder {
	b.ctx.Attributes[key] = value
	return b
}

// Build returns the constructed EvaluationContext.
func (b *EvaluationContextBuilder) Build() *EvaluationContext {
	return &b.ctx
}
