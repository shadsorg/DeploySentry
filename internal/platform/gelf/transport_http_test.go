package gelf

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPTransport_SendsToEndpoint(t *testing.T) {
	var mu sync.Mutex
	var received []byte
	var headers http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		headers = r.Header.Clone()
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	tr, err := newHTTPTransport(server.URL, "test-client-id", "test-client-secret")
	require.NoError(t, err)

	msg := map[string]string{"short_message": "hello from http"}
	data, _ := json.Marshal(msg)
	require.NoError(t, tr.Send(data))

	// Close flushes the buffer, ensuring the message is sent
	tr.Close()

	mu.Lock()
	defer mu.Unlock()

	require.NotEmpty(t, received)
	var parsed map[string]string
	require.NoError(t, json.Unmarshal(received, &parsed))
	assert.Equal(t, "hello from http", parsed["short_message"])

	assert.Equal(t, "application/json", headers.Get("Content-Type"))
	assert.Equal(t, "test-client-id", headers.Get("CF-Access-Client-Id"))
	assert.Equal(t, "test-client-secret", headers.Get("CF-Access-Client-Secret"))
}

func TestHTTPTransport_CloseFlushesBuffer(t *testing.T) {
	var mu sync.Mutex
	var count int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	tr, err := newHTTPTransport(server.URL, "id", "secret")
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		require.NoError(t, tr.Send([]byte(`{"short_message":"msg"}`)))
	}

	tr.Close()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 5, count)
}
