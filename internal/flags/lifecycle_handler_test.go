package flags

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestLifecycle_SmokeTestResult_Happy exercises the smoke-test endpoint via
// the HTTP plumbing. The mockFlagService round-trips status, so we can
// confirm the response JSON carries smoke_test_status = "pass" / "fail".
func TestLifecycle_SmokeTestResult_Happy(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})
	flagID := uuid.New().String()

	body := map[string]interface{}{"status": "pass", "notes": "all checks green"}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/flags/"+flagID+"/smoke-test-result", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "pass", resp["smoke_test_status"])
}

func TestLifecycle_SmokeTestResult_RejectsInvalidStatus(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})
	flagID := uuid.New().String()

	body := map[string]interface{}{"status": "bogus"}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/flags/"+flagID+"/smoke-test-result", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLifecycle_UserTestResult_FailRequiresNotes(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})
	flagID := uuid.New().String()

	body := map[string]interface{}{"status": "fail", "userId": "u1"}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/flags/"+flagID+"/user-test-result", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLifecycle_ScheduleRemoval_RejectsNonPositiveDays(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})
	flagID := uuid.New().String()

	body := map[string]interface{}{"days": 0}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/flags/"+flagID+"/schedule-removal", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLifecycle_ScheduleRemoval_Happy(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})
	flagID := uuid.New().String()

	body := map[string]interface{}{"days": 7}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/flags/"+flagID+"/schedule-removal", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestLifecycle_MarkExhausted_Happy(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})
	flagID := uuid.New().String()

	req := httptest.NewRequest(http.MethodPost, "/api/flags/"+flagID+"/mark-exhausted", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["iteration_exhausted"])
}
