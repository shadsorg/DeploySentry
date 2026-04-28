package registry

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// ErrAgentNotFound is returned when an agent lookup finds no match.
var ErrAgentNotFound = errors.New("agent not found")

// Service defines the business operations for agent registration and heartbeats.
type Service interface {
	Register(ctx context.Context, appID, envID uuid.UUID, version string, upstreams json.RawMessage) (*models.Agent, error)
	Heartbeat(ctx context.Context, agentID uuid.UUID, deploymentID *uuid.UUID, payload models.HeartbeatPayload) error
	Deregister(ctx context.Context, agentID uuid.UUID) error
	ListByApp(ctx context.Context, appID uuid.UUID) ([]models.Agent, error)
	GetAgent(ctx context.Context, agentID uuid.UUID) (*models.Agent, error)
	LatestHeartbeats(ctx context.Context, agentID uuid.UUID, deploymentID *uuid.UUID, limit int) ([]models.AgentHeartbeat, error)
}

type agentService struct {
	repo Repository
}

// NewService returns a new agent registry service.
func NewService(repo Repository) Service {
	return &agentService{repo: repo}
}

func (s *agentService) Register(ctx context.Context, appID, envID uuid.UUID, version string, upstreams json.RawMessage) (*models.Agent, error) {
	now := time.Now().UTC()
	agent := &models.Agent{
		ID:             uuid.New(),
		AppID:          appID,
		EnvironmentID:  envID,
		Status:         models.AgentStatusConnected,
		Version:        version,
		UpstreamConfig: upstreams,
		LastSeenAt:     now,
		RegisteredAt:   now,
	}
	if err := s.repo.CreateAgent(ctx, agent); err != nil {
		return nil, err
	}
	return agent, nil
}

func (s *agentService) Heartbeat(ctx context.Context, agentID uuid.UUID, deploymentID *uuid.UUID, payload models.HeartbeatPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	hb := &models.AgentHeartbeat{
		ID:           uuid.New(),
		AgentID:      agentID,
		DeploymentID: deploymentID,
		Payload:      data,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.repo.InsertHeartbeat(ctx, hb); err != nil {
		return err
	}
	if err := s.repo.UpdateLastSeen(ctx, agentID); err != nil {
		return err
	}
	return s.repo.PruneHeartbeats(ctx, agentID, deploymentID, 100)
}

func (s *agentService) Deregister(ctx context.Context, agentID uuid.UUID) error {
	return s.repo.DeleteAgent(ctx, agentID)
}

func (s *agentService) ListByApp(ctx context.Context, appID uuid.UUID) ([]models.Agent, error) {
	return s.repo.ListAgentsByApp(ctx, appID)
}

func (s *agentService) GetAgent(ctx context.Context, agentID uuid.UUID) (*models.Agent, error) {
	return s.repo.GetAgent(ctx, agentID)
}

func (s *agentService) LatestHeartbeats(ctx context.Context, agentID uuid.UUID, deploymentID *uuid.UUID, limit int) ([]models.AgentHeartbeat, error) {
	return s.repo.ListHeartbeats(ctx, agentID, deploymentID, limit)
}
