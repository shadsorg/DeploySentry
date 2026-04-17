package registry

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// mockRepo implements Repository with in-memory maps.
type mockRepo struct {
	mu         sync.Mutex
	agents     map[uuid.UUID]*models.Agent
	heartbeats map[uuid.UUID][]models.AgentHeartbeat
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		agents:     make(map[uuid.UUID]*models.Agent),
		heartbeats: make(map[uuid.UUID][]models.AgentHeartbeat),
	}
}

func (m *mockRepo) CreateAgent(_ context.Context, a *models.Agent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents[a.ID] = a
	return nil
}

func (m *mockRepo) GetAgent(_ context.Context, id uuid.UUID) (*models.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.agents[id]
	if !ok {
		return nil, ErrAgentNotFound
	}
	return a, nil
}

func (m *mockRepo) ListAgentsByApp(_ context.Context, appID uuid.UUID) ([]models.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Agent
	for _, a := range m.agents {
		if a.AppID == appID {
			out = append(out, *a)
		}
	}
	return out, nil
}

func (m *mockRepo) UpdateAgentStatus(_ context.Context, id uuid.UUID, status models.AgentStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.agents[id]; ok {
		a.Status = status
	}
	return nil
}

func (m *mockRepo) UpdateLastSeen(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockRepo) DeleteAgent(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.agents, id)
	return nil
}

func (m *mockRepo) MarkStaleAgents(_ context.Context, _, _ int) error {
	return nil
}

func (m *mockRepo) InsertHeartbeat(_ context.Context, hb *models.AgentHeartbeat) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.heartbeats[hb.AgentID] = append(m.heartbeats[hb.AgentID], *hb)
	return nil
}

func (m *mockRepo) ListHeartbeats(_ context.Context, agentID uuid.UUID, _ *uuid.UUID, limit int) ([]models.AgentHeartbeat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hbs := m.heartbeats[agentID]
	if limit > 0 && limit < len(hbs) {
		hbs = hbs[len(hbs)-limit:]
	}
	return hbs, nil
}

func (m *mockRepo) PruneHeartbeats(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _ int) error {
	return nil
}

func TestRegisterAgent(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	appID := uuid.New()
	envID := uuid.New()
	upstreams := json.RawMessage(`{"v1":"http://localhost:8081"}`)

	agent, err := svc.Register(context.Background(), appID, envID, "1.0.0", upstreams)
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if agent.AppID != appID {
		t.Errorf("expected AppID %s, got %s", appID, agent.AppID)
	}
	if agent.Status != models.AgentStatusConnected {
		t.Errorf("expected status connected, got %s", agent.Status)
	}
	if agent.ID == uuid.Nil {
		t.Error("expected non-nil agent ID")
	}

	// Verify agent is in the repo.
	got, err := svc.GetAgent(context.Background(), agent.ID)
	if err != nil {
		t.Fatalf("GetAgent returned error: %v", err)
	}
	if got.ID != agent.ID {
		t.Errorf("expected agent ID %s, got %s", agent.ID, got.ID)
	}
}

func TestHeartbeat(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	agent, err := svc.Register(context.Background(), uuid.New(), uuid.New(), "1.0.0", nil)
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	payload := models.HeartbeatPayload{
		AgentID:       agent.ID,
		ConfigVersion: 1,
		EnvoyHealthy:  true,
	}
	if err := svc.Heartbeat(context.Background(), agent.ID, nil, payload); err != nil {
		t.Fatalf("Heartbeat returned error: %v", err)
	}

	hbs, err := svc.LatestHeartbeats(context.Background(), agent.ID, nil, 10)
	if err != nil {
		t.Fatalf("LatestHeartbeats returned error: %v", err)
	}
	if len(hbs) != 1 {
		t.Fatalf("expected 1 heartbeat, got %d", len(hbs))
	}
	if hbs[0].AgentID != agent.ID {
		t.Errorf("expected heartbeat agent ID %s, got %s", agent.ID, hbs[0].AgentID)
	}
}

func TestDeregister(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	agent, err := svc.Register(context.Background(), uuid.New(), uuid.New(), "1.0.0", nil)
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	if err := svc.Deregister(context.Background(), agent.ID); err != nil {
		t.Fatalf("Deregister returned error: %v", err)
	}

	_, err = svc.GetAgent(context.Background(), agent.ID)
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound after deregister, got %v", err)
	}
}
