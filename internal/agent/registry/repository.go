package registry

import (
	"context"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// Repository persists agent registrations and heartbeats.
type Repository interface {
	CreateAgent(ctx context.Context, a *models.Agent) error
	GetAgent(ctx context.Context, id uuid.UUID) (*models.Agent, error)
	ListAgentsByApp(ctx context.Context, appID uuid.UUID) ([]models.Agent, error)
	UpdateAgentStatus(ctx context.Context, id uuid.UUID, status models.AgentStatus) error
	UpdateLastSeen(ctx context.Context, id uuid.UUID) error
	DeleteAgent(ctx context.Context, id uuid.UUID) error
	MarkStaleAgents(ctx context.Context, staleDuration, disconnectDuration int) error

	InsertHeartbeat(ctx context.Context, hb *models.AgentHeartbeat) error
	ListHeartbeats(ctx context.Context, agentID uuid.UUID, deploymentID *uuid.UUID, limit int) ([]models.AgentHeartbeat, error)
	PruneHeartbeats(ctx context.Context, agentID uuid.UUID, deploymentID *uuid.UUID, keep int) error
}
