// Package deploys implements the agentless deploy-event ingestion path.
// Provider-specific adapters normalize inbound webhooks into a canonical
// DeployEvent, which the shared service turns into a mode=record
// deployment via the existing deploy service.
package deploys

import (
	"errors"
	"net/http"

	"github.com/deploysentry/deploysentry/internal/models"
)

// ErrInvalidSignature is returned by an adapter when a webhook's signature
// does not match. It maps to HTTP 401.
var ErrInvalidSignature = errors.New("invalid webhook signature")

// DeployEventAdapter converts a provider's native webhook payload into the
// shared DeployEvent shape and verifies the payload's signature.
type DeployEventAdapter interface {
	Provider() string
	VerifySignature(r *http.Request, body []byte, secret string) error
	ParsePayload(body []byte, integration *models.DeployIntegration) (models.DeployEvent, error)
}

// Registry holds the adapter instances known to the handler.
type Registry struct {
	byProvider map[string]DeployEventAdapter
}

// NewRegistry constructs an empty registry.
func NewRegistry() *Registry {
	return &Registry{byProvider: map[string]DeployEventAdapter{}}
}

// Register adds an adapter. Overwrites any previous adapter for the same provider.
func (r *Registry) Register(a DeployEventAdapter) {
	r.byProvider[a.Provider()] = a
}

// Lookup returns the adapter for a provider, or (nil, false) if unregistered.
func (r *Registry) Lookup(provider string) (DeployEventAdapter, bool) {
	a, ok := r.byProvider[provider]
	return a, ok
}
