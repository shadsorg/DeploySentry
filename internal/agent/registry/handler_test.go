package registry

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func setupTestRouter() (*gin.Engine, Service) {
	gin.SetMode(gin.TestMode)
	repo := newMockRepo()
	svc := NewService(repo)
	handler := NewHandler(svc)

	r := gin.New()
	handler.RegisterRoutes(&r.RouterGroup, nil)
	return r, svc
}

func TestHandler_RegisterAgent(t *testing.T) {
	r, _ := setupTestRouter()

	body := registerRequest{
		AppID:         uuid.New(),
		EnvironmentID: uuid.New(),
		Version:       "1.0.0",
		Upstreams:     json.RawMessage(`{"v1":"http://localhost:8081"}`),
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/agents/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var agent map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &agent); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if agent["id"] == nil || agent["id"] == "" {
		t.Error("expected non-empty agent id")
	}
	if agent["app_id"] != body.AppID.String() {
		t.Errorf("expected app_id %s, got %v", body.AppID, agent["app_id"])
	}
}

func TestHandler_Heartbeat(t *testing.T) {
	r, _ := setupTestRouter()

	// First register an agent.
	regBody := registerRequest{
		AppID:         uuid.New(),
		EnvironmentID: uuid.New(),
		Version:       "1.0.0",
	}
	b, _ := json.Marshal(regBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/agents/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", w.Code)
	}

	var agent map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &agent)
	agentID := agent["id"].(string)

	// Send heartbeat.
	hbBody := heartbeatRequest{
		ConfigVersion: 1,
		ActualTraffic: map[string]float64{"blue": 95, "green": 5},
		EnvoyHealthy:  true,
	}
	b, _ = json.Marshal(hbBody)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/agents/"+agentID+"/heartbeat", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("heartbeat: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ListAgents(t *testing.T) {
	r, _ := setupTestRouter()

	appID := uuid.New()

	// Register an agent for this app.
	regBody := registerRequest{
		AppID:         appID,
		EnvironmentID: uuid.New(),
		Version:       "1.0.0",
	}
	b, _ := json.Marshal(regBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/agents/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", w.Code)
	}

	// List agents for the app.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/applications/"+appID.String()+"/agents", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list agents: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string][]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp["agents"]) != 1 {
		t.Errorf("expected 1 agent, got %d", len(resp["agents"]))
	}
}

func TestHandler_ListAgents_Empty(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/applications/"+uuid.New().String()+"/agents", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string][]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp["agents"]) != 0 {
		t.Errorf("expected 0 agents, got %d", len(resp["agents"]))
	}
}

func TestHandler_Deregister(t *testing.T) {
	r, _ := setupTestRouter()

	// Register first.
	regBody := registerRequest{
		AppID:         uuid.New(),
		EnvironmentID: uuid.New(),
		Version:       "1.0.0",
	}
	b, _ := json.Marshal(regBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/agents/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var agent map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &agent)
	agentID := agent["id"].(string)

	// Delete.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodDelete, "/agents/"+agentID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("deregister: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
