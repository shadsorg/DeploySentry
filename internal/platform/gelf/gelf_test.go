package gelf

import (
	"encoding/json"
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
