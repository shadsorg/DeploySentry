package gelf

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessage_JSON_IncludesRequiredFields(t *testing.T) {
	msg := Message{
		Version:      "1.1",
		Host:         "testhost",
		ShortMessage: "test message",
		Timestamp:    1711843200.0,
		Level:        6,
		AppName:      "test-app",
		Env:          "development",
		LogType:      "info",
		LogLevel:     "info",
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &parsed))

	assert.Equal(t, "1.1", parsed["version"])
	assert.Equal(t, "testhost", parsed["host"])
	assert.Equal(t, "test message", parsed["short_message"])
	assert.Equal(t, float64(6), parsed["level"])
	assert.Equal(t, "test-app", parsed["_appName"])
	assert.Equal(t, "development", parsed["_env"])
	assert.Equal(t, "info", parsed["_logType"])
	assert.Equal(t, "info", parsed["_logLevel"])
}

func TestMessage_JSON_OmitsEmptyOptionalFields(t *testing.T) {
	msg := Message{
		Version:      "1.1",
		Host:         "testhost",
		ShortMessage: "test",
		Level:        6,
		AppName:      "app",
		Env:          "dev",
		LogType:      "info",
		LogLevel:     "info",
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &parsed))

	_, hasRequestID := parsed["_requestId"]
	_, hasUserID := parsed["_userId"]
	_, hasHTTPMethod := parsed["_httpMethod"]
	assert.False(t, hasRequestID)
	assert.False(t, hasUserID)
	assert.False(t, hasHTTPMethod)
}

func TestGELFLevel(t *testing.T) {
	tests := []struct {
		logLevel string
		want     int
	}{
		{"fatal", 2},
		{"error", 3},
		{"warn", 4},
		{"info", 6},
		{"debug", 7},
		{"trace", 7},
		{"unknown", 6},
	}
	for _, tc := range tests {
		t.Run(tc.logLevel, func(t *testing.T) {
			assert.Equal(t, tc.want, gelfLevel(tc.logLevel))
		})
	}
}

func TestClient_NilSafety(t *testing.T) {
	var c *Client
	// None of these should panic
	c.Info("test")
	c.Error("test", "err", "stack")
	c.Request("req-1", "GET", "/test", "user-1")
	c.Response("req-1", "GET", "/test", "user-1", 200, 42)
	assert.NoError(t, c.Close())
}

type mockTransport struct {
	messages [][]byte
	closed   bool
}

func (m *mockTransport) Send(data []byte) error {
	cp := make([]byte, len(data))
	copy(cp, data)
	m.messages = append(m.messages, cp)
	return nil
}

func (m *mockTransport) Close() error {
	m.closed = true
	return nil
}

func newTestClient(t *testing.T) (*Client, *mockTransport) {
	t.Helper()
	mt := &mockTransport{}
	c := &Client{
		transport: mt,
		appName:   "test-app",
		env:       "development",
		hostname:  "testhost",
	}
	return c, mt
}

func TestClient_Info_SendsCorrectFields(t *testing.T) {
	c, mt := newTestClient(t)
	c.Info("server started")

	require.Len(t, mt.messages, 1)
	var msg map[string]interface{}
	require.NoError(t, json.Unmarshal(mt.messages[0], &msg))

	assert.Equal(t, "1.1", msg["version"])
	assert.Equal(t, "testhost", msg["host"])
	assert.Equal(t, "server started", msg["short_message"])
	assert.Equal(t, "info", msg["_logType"])
	assert.Equal(t, "info", msg["_logLevel"])
	assert.Equal(t, float64(6), msg["level"])
	assert.Equal(t, "test-app", msg["_appName"])
	assert.Equal(t, "development", msg["_env"])
}

func TestClient_Request_SendsCorrectFields(t *testing.T) {
	c, mt := newTestClient(t)
	c.Request("req-123", "POST", "/api/v1/orgs", "user-456")

	require.Len(t, mt.messages, 1)
	var msg map[string]interface{}
	require.NoError(t, json.Unmarshal(mt.messages[0], &msg))

	assert.Equal(t, "request", msg["_logType"])
	assert.Equal(t, "POST /api/v1/orgs", msg["short_message"])
	assert.Equal(t, "req-123", msg["_requestId"])
	assert.Equal(t, "POST", msg["_httpMethod"])
	assert.Equal(t, "/api/v1/orgs", msg["_httpPath"])
	assert.Equal(t, "user-456", msg["_userId"])
}

func TestClient_Response_LogLevelByStatus(t *testing.T) {
	tests := []struct {
		status    int
		wantLevel string
	}{
		{200, "info"},
		{301, "info"},
		{400, "warn"},
		{404, "warn"},
		{500, "error"},
		{503, "error"},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			c, mt := newTestClient(t)
			c.Response("req-1", "GET", "/test", "", tc.status, 42)

			require.Len(t, mt.messages, 1)
			var msg map[string]interface{}
			require.NoError(t, json.Unmarshal(mt.messages[0], &msg))

			assert.Equal(t, tc.wantLevel, msg["_logLevel"])
		})
	}
}

func TestClient_Response_ShortMessageFormat(t *testing.T) {
	c, mt := newTestClient(t)
	c.Response("req-1", "GET", "/api/v1/flags", "", 200, 42)

	require.Len(t, mt.messages, 1)
	var msg map[string]interface{}
	require.NoError(t, json.Unmarshal(mt.messages[0], &msg))

	assert.Equal(t, "200 GET /api/v1/flags 42ms", msg["short_message"])
}

func TestClient_Error_SendsCorrectFields(t *testing.T) {
	c, mt := newTestClient(t)
	c.Error("request failed", "NullPointerError", "goroutine 1 [running]:\nmain.go:42")

	require.Len(t, mt.messages, 1)
	var msg map[string]interface{}
	require.NoError(t, json.Unmarshal(mt.messages[0], &msg))

	assert.Equal(t, "error", msg["_logType"])
	assert.Equal(t, "error", msg["_logLevel"])
	assert.Equal(t, float64(3), msg["level"])
	assert.Equal(t, "NullPointerError", msg["_errorName"])
	assert.Contains(t, msg["_errorStack"], "goroutine 1")
}

func TestClient_Close(t *testing.T) {
	c, mt := newTestClient(t)
	assert.NoError(t, c.Close())
	assert.True(t, mt.closed)
}

func TestClient_TruncatesLargeErrorStack(t *testing.T) {
	mt := &mockTransport{}
	c := &Client{
		transport: mt,
		appName:   "test-app",
		env:       "dev",
		hostname:  "testhost",
	}

	bigStack := make([]byte, 16000)
	for i := range bigStack {
		bigStack[i] = 'x'
	}
	c.Error("big error", "StackOverflow", string(bigStack))

	require.Len(t, mt.messages, 1)
	assert.LessOrEqual(t, len(mt.messages[0]), maxUDPPayload)
	assert.Contains(t, string(mt.messages[0]), "...[truncated]")
}
