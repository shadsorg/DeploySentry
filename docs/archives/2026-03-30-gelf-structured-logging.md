# GELF Structured Logging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add GELF structured logging as an additional transport alongside existing console logging, with auto-selected UDP/HTTP transport based on Cloudflare Access credentials.

**Architecture:** A standalone `internal/platform/gelf/` package handles GELF message construction and transport (UDP or HTTP). The existing `StructuredLogger` and `ErrorHandler` middleware accept an optional `*gelf.Client` and send structured messages alongside existing console output. Transport is selected at startup based on the presence of `CF_ACCESS_CLIENT_ID`.

**Tech Stack:** Go standard library (`net`, `net/http`, `encoding/json`, `os`), Gin framework, existing middleware infrastructure.

**Spec:** `docs/superpowers/specs/2026-03-30-gelf-structured-logging-design.md`

---

## File Structure

| Action | Path | Responsibility |
|---|---|---|
| Create | `internal/platform/gelf/gelf.go` | Client, Message struct, convenience methods, level mapping |
| Create | `internal/platform/gelf/transport_udp.go` | UDP transport to localhost:12201 |
| Create | `internal/platform/gelf/transport_http.go` | HTTP transport with Cloudflare Access headers |
| Create | `internal/platform/gelf/gelf_test.go` | Tests for Client, Message, level mapping, nil-safety |
| Create | `internal/platform/gelf/transport_udp_test.go` | UDP transport tests |
| Create | `internal/platform/gelf/transport_http_test.go` | HTTP transport tests |
| Modify | `internal/platform/middleware/logging.go` | Accept `*gelf.Client`, send request/response GELF messages, trace-level skip-path override |
| Modify | `internal/platform/middleware/error_handler.go` | Accept `*gelf.Client`, send error/fatal GELF messages |
| Modify | `internal/platform/middleware/middleware_test.go` | Add tests for GELF integration in middleware |
| Modify | `cmd/api/main.go` | Instantiate GELF client, wire into middleware, send startup message |

---

### Task 1: GELF Message struct and level mapping

**Files:**
- Create: `internal/platform/gelf/gelf.go`
- Create: `internal/platform/gelf/gelf_test.go`

- [ ] **Step 1: Write failing tests for Message JSON serialization and level mapping**

```go
// internal/platform/gelf/gelf_test.go
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
		{"unknown", 6}, // default to info
	}
	for _, tc := range tests {
		t.Run(tc.logLevel, func(t *testing.T) {
			assert.Equal(t, tc.want, gelfLevel(tc.logLevel))
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/platform/gelf/ -v`
Expected: FAIL — package does not exist

- [ ] **Step 3: Implement Message struct and level mapping**

```go
// internal/platform/gelf/gelf.go
package gelf

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// Message represents a GELF 1.1 log message.
type Message struct {
	// Standard GELF fields
	Version      string  `json:"version"`
	Host         string  `json:"host"`
	ShortMessage string  `json:"short_message"`
	Timestamp    float64 `json:"timestamp"`
	Level        int     `json:"level"`

	// Required custom fields (always present)
	AppName  string `json:"_appName"`
	Env      string `json:"_env"`
	LogType  string `json:"_logType"`
	LogLevel string `json:"_logLevel"`

	// Optional custom fields (omitted when zero-value)
	RequestID     string `json:"_requestId,omitempty"`
	UserID        string `json:"_userId,omitempty"`
	HTTPMethod    string `json:"_httpMethod,omitempty"`
	HTTPPath      string `json:"_httpPath,omitempty"`
	HTTPStatus    int    `json:"_httpStatus,omitempty"`
	RequestTimeMs int64  `json:"_requestTimeMs,omitempty"`
	ErrorName     string `json:"_errorName,omitempty"`
	ErrorStack    string `json:"_errorStack,omitempty"`
}

// Transport defines how GELF messages are delivered.
// Exported so middleware tests can provide mock implementations.
type Transport interface {
	Send(data []byte) error
	Close() error
}

// Client sends structured log messages via GELF.
type Client struct {
	transport Transport
	appName   string
	env       string
	hostname  string
}

// NewClient creates a GELF client. Transport is selected automatically:
// - CF_ACCESS_CLIENT_ID set → HTTP to ingest.crowdsoftapps.com
// - Otherwise → UDP to localhost:12201
func NewClient(appName string) (*Client, error) {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	env := os.Getenv("DS_ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	var t transport
	var err error

	if os.Getenv("CF_ACCESS_CLIENT_ID") != "" {
		t, err = newHTTPTransport(
			"https://ingest.crowdsoftapps.com/gelf",
			os.Getenv("CF_ACCESS_CLIENT_ID"),
			os.Getenv("CF_ACCESS_CLIENT_SECRET"),
		)
	} else {
		t, err = newUDPTransport("localhost:12201")
	}
	if err != nil {
		return nil, fmt.Errorf("gelf: %w", err)
	}

	return &Client{
		transport: t,
		appName:   appName,
		env:       env,
		hostname:  hostname,
	}, nil
}

// Close shuts down the transport.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	return c.transport.Close()
}

// send serializes and dispatches a message. If the payload exceeds maxUDPPayload
// and the message has an ErrorStack, the stack is truncated before re-serializing.
// Errors are logged to stderr, never returned.
func (c *Client) send(msg Message) {
	msg.Version = "1.1"
	msg.Host = c.hostname
	msg.AppName = c.appName
	msg.Env = c.env
	if msg.Timestamp == 0 {
		msg.Timestamp = float64(time.Now().UnixNano()) / 1e9
	}
	msg.Level = gelfLevel(msg.LogLevel)

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("gelf: marshal error: %v", err)
		return
	}

	// Truncate _errorStack if payload is too large for UDP
	if len(data) > maxUDPPayload && msg.ErrorStack != "" {
		overhead := len(data) - len(msg.ErrorStack)
		maxStack := maxUDPPayload - overhead - 20 // safety margin
		if maxStack < 0 {
			maxStack = 0
		}
		msg.ErrorStack = msg.ErrorStack[:maxStack] + "...[truncated]"
		data, err = json.Marshal(msg)
		if err != nil {
			log.Printf("gelf: marshal error after truncation: %v", err)
			return
		}
	}

	if err := c.transport.Send(data); err != nil {
		log.Printf("gelf: send error: %v", err)
	}
}

const maxUDPPayload = 8192

// Info sends an informational log message.
func (c *Client) Info(shortMessage string) {
	if c == nil {
		return
	}
	c.send(Message{
		ShortMessage: shortMessage,
		LogType:      "info",
		LogLevel:     "info",
	})
}

// Error sends an error log message.
func (c *Client) Error(shortMessage, errorName, errorStack string) {
	if c == nil {
		return
	}
	c.send(Message{
		ShortMessage: shortMessage,
		LogType:      "error",
		LogLevel:     "error",
		ErrorName:    errorName,
		ErrorStack:   errorStack,
	})
}

// Request sends a request-start log message.
func (c *Client) Request(requestID, method, path, userID string) {
	if c == nil {
		return
	}
	c.send(Message{
		ShortMessage: method + " " + path,
		LogType:      "request",
		LogLevel:     "info",
		RequestID:    requestID,
		HTTPMethod:   method,
		HTTPPath:     path,
		UserID:       userID,
	})
}

// Response sends a request-complete log message.
func (c *Client) Response(requestID, method, path, userID string, status int, durationMs int64) {
	if c == nil {
		return
	}
	logLevel := "info"
	if status >= 500 {
		logLevel = "error"
	} else if status >= 400 {
		logLevel = "warn"
	}
	c.send(Message{
		ShortMessage:  fmt.Sprintf("%d %s %s %dms", status, method, path, durationMs),
		LogType:       "response",
		LogLevel:      logLevel,
		RequestID:     requestID,
		HTTPMethod:    method,
		HTTPPath:      path,
		UserID:        userID,
		HTTPStatus:    status,
		RequestTimeMs: durationMs,
	})
}

// gelfLevel maps a log level string to a GELF numeric severity.
func gelfLevel(level string) int {
	switch level {
	case "fatal":
		return 2
	case "error":
		return 3
	case "warn":
		return 4
	case "info":
		return 6
	case "debug", "trace":
		return 7
	default:
		return 6
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/platform/gelf/ -v -run "TestMessage|TestGELFLevel"`
Expected: PASS (transport tests will be added in subsequent tasks)

- [ ] **Step 5: Commit**

```bash
git add internal/platform/gelf/gelf.go internal/platform/gelf/gelf_test.go
git commit -m "feat: add GELF message struct, client, and level mapping"
```

---

### Task 2: Nil-safety and convenience method tests

**Files:**
- Modify: `internal/platform/gelf/gelf_test.go`

- [ ] **Step 1: Write tests for nil-safety and convenience methods**

```go
// Append to internal/platform/gelf/gelf_test.go

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
	m.messages = append(m.messages, data)
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
		status   int
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
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `go test ./internal/platform/gelf/ -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/platform/gelf/gelf_test.go
git commit -m "test: add nil-safety and convenience method tests for GELF client"
```

---

### Task 3: UDP transport

**Files:**
- Create: `internal/platform/gelf/transport_udp.go`
- Create: `internal/platform/gelf/transport_udp_test.go`

- [ ] **Step 1: Write failing tests for UDP transport**

```go
// internal/platform/gelf/transport_udp_test.go
package gelf

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUDPTransport_SendReceive(t *testing.T) {
	// Start a UDP listener on a random port
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	require.NoError(t, err)
	conn, err := net.ListenUDP("udp", addr)
	require.NoError(t, err)
	defer conn.Close()

	// Create transport pointed at the listener
	tr, err := newUDPTransport(conn.LocalAddr().String())
	require.NoError(t, err)
	defer tr.Close()

	// Send a message
	msg := map[string]string{"short_message": "hello"}
	data, _ := json.Marshal(msg)
	require.NoError(t, tr.Send(data))

	// Read it back
	buf := make([]byte, 8192)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := conn.ReadFromUDP(buf)
	require.NoError(t, err)

	var received map[string]string
	require.NoError(t, json.Unmarshal(buf[:n], &received))
	assert.Equal(t, "hello", received["short_message"])
}

func TestClient_TruncatesLargeErrorStack(t *testing.T) {
	mt := &mockTransport{}
	c := &Client{
		transport: mt,
		appName:   "test-app",
		env:       "dev",
		hostname:  "testhost",
	}

	// Build a huge error stack that will exceed maxUDPPayload
	bigStack := make([]byte, 16000)
	for i := range bigStack {
		bigStack[i] = 'x'
	}
	c.Error("big error", "StackOverflow", string(bigStack))

	require.Len(t, mt.messages, 1)
	assert.LessOrEqual(t, len(mt.messages[0]), maxUDPPayload)
	assert.Contains(t, string(mt.messages[0]), "...[truncated]")
}

func TestUDPTransport_Close(t *testing.T) {
	tr, err := newUDPTransport("127.0.0.1:12201")
	require.NoError(t, err)
	assert.NoError(t, tr.Close())
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/platform/gelf/ -v -run TestUDP`
Expected: FAIL — `newUDPTransport` undefined

- [ ] **Step 3: Implement UDP transport**

```go
// internal/platform/gelf/transport_udp.go
package gelf

import (
	"fmt"
	"net"
)

type udpTransport struct {
	conn *net.UDPConn
}

func newUDPTransport(address string) (*udpTransport, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, fmt.Errorf("resolve udp address: %w", err)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("dial udp: %w", err)
	}
	return &udpTransport{conn: conn}, nil
}

// Send writes pre-serialized JSON data as a single UDP datagram.
// Truncation of oversized messages is handled by Client.send before calling this.
func (t *udpTransport) Send(data []byte) error {
	_, err := t.conn.Write(data)
	return err
}

func (t *udpTransport) Close() error {
	return t.conn.Close()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/platform/gelf/ -v -run TestUDP`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/platform/gelf/transport_udp.go internal/platform/gelf/transport_udp_test.go
git commit -m "feat: add GELF UDP transport with error stack truncation"
```

---

### Task 4: HTTP transport

**Files:**
- Create: `internal/platform/gelf/transport_http.go`
- Create: `internal/platform/gelf/transport_http_test.go`

- [ ] **Step 1: Write failing tests for HTTP transport**

```go
// internal/platform/gelf/transport_http_test.go
package gelf

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

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

	// Wait for async send
	time.Sleep(200 * time.Millisecond)
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/platform/gelf/ -v -run TestHTTP`
Expected: FAIL — `newHTTPTransport` undefined

- [ ] **Step 3: Implement HTTP transport**

```go
// internal/platform/gelf/transport_http.go
package gelf

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	httpBufferSize = 256
	httpTimeout    = 2 * time.Second
)

type httpTransport struct {
	url          string
	clientID     string
	clientSecret string
	httpClient   *http.Client
	ch           chan []byte
	wg           sync.WaitGroup
	once         sync.Once
}

func newHTTPTransport(url, clientID, clientSecret string) (*httpTransport, error) {
	t := &httpTransport{
		url:          url,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: httpTimeout},
		ch:           make(chan []byte, httpBufferSize),
	}
	t.wg.Add(1)
	go t.drain()
	return t, nil
}

func (t *httpTransport) drain() {
	defer t.wg.Done()
	for data := range t.ch {
		t.post(data)
	}
}

func (t *httpTransport) post(data []byte) {
	req, err := http.NewRequest(http.MethodPost, t.url, bytes.NewReader(data))
	if err != nil {
		log.Printf("gelf http: create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("CF-Access-Client-Id", t.clientID)
	req.Header.Set("CF-Access-Client-Secret", t.clientSecret)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		log.Printf("gelf http: send error: %v", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("gelf http: unexpected status %d", resp.StatusCode)
	}
}

func (t *httpTransport) Send(data []byte) error {
	select {
	case t.ch <- data:
		return nil
	default:
		return fmt.Errorf("gelf http: buffer full, message dropped")
	}
}

func (t *httpTransport) Close() error {
	t.once.Do(func() {
		close(t.ch)
	})
	t.wg.Wait()
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/platform/gelf/ -v -run TestHTTP`
Expected: PASS

- [ ] **Step 5: Run all GELF tests**

Run: `go test ./internal/platform/gelf/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/platform/gelf/transport_http.go internal/platform/gelf/transport_http_test.go
git commit -m "feat: add GELF HTTP transport with async buffered send"
```

---

### Task 5: Wire GELF into StructuredLogger middleware

**Files:**
- Modify: `internal/platform/middleware/logging.go`
- Modify: `internal/platform/middleware/middleware_test.go`

- [ ] **Step 1: Write failing tests for GELF integration in StructuredLogger**

Append to `internal/platform/middleware/middleware_test.go`:

```go
// ---------------------------------------------------------------------------
// 8. StructuredLogger GELF integration
// ---------------------------------------------------------------------------

func TestStructuredLogger_SendsGELFRequestAndResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())

	mt := &gelfMockTransport{}
	gelfClient := newTestGELFClient(mt)

	router.Use(StructuredLogger(DefaultLoggingConfig(), gelfClient))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.GreaterOrEqual(t, len(mt.messages), 2)

	// First message should be request
	var reqMsg map[string]interface{}
	require.NoError(t, json.Unmarshal(mt.messages[0], &reqMsg))
	assert.Equal(t, "request", reqMsg["_logType"])
	assert.Equal(t, "GET", reqMsg["_httpMethod"])
	assert.Equal(t, "/test", reqMsg["_httpPath"])
	assert.NotEmpty(t, reqMsg["_requestId"])

	// Second message should be response
	var respMsg map[string]interface{}
	require.NoError(t, json.Unmarshal(mt.messages[1], &respMsg))
	assert.Equal(t, "response", respMsg["_logType"])
	assert.Equal(t, float64(200), respMsg["_httpStatus"])
	assert.NotZero(t, respMsg["_requestTimeMs"])
}

func TestStructuredLogger_SkipsHealthByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())

	mt := &gelfMockTransport{}
	gelfClient := newTestGELFClient(mt)

	router.Use(StructuredLogger(DefaultLoggingConfig(), gelfClient))
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, mt.messages)
}

func TestStructuredLogger_LogsHealthAtTraceLevel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())

	mt := &gelfMockTransport{}
	gelfClient := newTestGELFClient(mt)

	cfg := DefaultLoggingConfig()
	cfg.LogLevel = "trace"
	router.Use(StructuredLogger(cfg, gelfClient))
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.GreaterOrEqual(t, len(mt.messages), 2)
}

func TestStructuredLogger_NilGELFClientDoesNotPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.Use(StructuredLogger(DefaultLoggingConfig(), nil))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// Test helpers — add near top of file with other test imports/utilities.
// Add imports: "encoding/json", "github.com/stretchr/testify/require",
//              "github.com/shadsorg/deploysentry/internal/platform/gelf"

type gelfMockTransport struct {
	messages [][]byte
}

func (m *gelfMockTransport) Send(data []byte) error {
	cp := make([]byte, len(data))
	copy(cp, data)
	m.messages = append(m.messages, cp)
	return nil
}

func (m *gelfMockTransport) Close() error { return nil }

func newTestGELFClient(mt *gelfMockTransport) *gelf.Client {
	return gelf.NewTestClient(mt)
}
```

Note: `newTestGELFClient` will call a `gelf.NewTestClient` helper that accepts a transport, added in step 3.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/platform/middleware/ -v -run "TestStructuredLogger_(SendsGELF|Skips|LogsHealth|NilGELF)"`
Expected: FAIL — signature mismatch, missing fields

- [ ] **Step 3: Add `NewTestClient` helper to gelf package and `LogLevel` field to `LoggingConfig`, then update `StructuredLogger`**

Add to `internal/platform/gelf/gelf.go`:

```go
// NewTestClient creates a Client with the given transport for testing.
func NewTestClient(t Transport) *Client {
	return &Client{
		transport: t,
		appName:   "test",
		env:       "test",
		hostname:  "testhost",
	}
}
```

Add `LogLevel` field to `LoggingConfig` in `internal/platform/middleware/logging.go`:

```go
type LoggingConfig struct {
	SkipPaths       []string `json:"skip_paths"`
	LogRequestBody  bool     `json:"log_request_body"`
	LogResponseBody bool     `json:"log_response_body"`
	MaxBodyLogSize  int64    `json:"max_body_log_size"`
	LogLevel        string   `json:"log_level"` // when "trace", skip-paths filtering is disabled
}
```

Update `StructuredLogger` signature and body in `internal/platform/middleware/logging.go`:

```go
func StructuredLogger(config LoggingConfig, gelfClient *gelf.Client) gin.HandlerFunc {
```

The existing middleware already builds a `skipPaths` map and extracts `path`. Update the skip-paths check to respect trace level:

```go
// Existing code builds: skipPaths map[string]bool from config.SkipPaths
// Replace the skip check with:
if config.LogLevel != "trace" && skipPaths[path] {
    c.Next()
    return
}
```

Before `c.Next()`, add GELF request log (use existing `c.Request.Method` and `path` local var from the existing code):

```go
method := c.Request.Method
requestID := GetRequestID(c)
if gelfClient != nil {
    gelfClient.Request(requestID, method, path, "")
}
```

After `c.Next()`, add GELF response log (use existing `status` and `duration` local vars):

```go
if gelfClient != nil {
    userID := ""
    if uid, exists := c.Get("user_id"); exists {
        if id, ok := uid.(uuid.UUID); ok {
            userID = id.String()
        }
    }
    gelfClient.Response(requestID, method, path, userID, status, int64(duration.Milliseconds()))
}
```

Add imports: `"github.com/google/uuid"` and `"github.com/shadsorg/deploysentry/internal/platform/gelf"`.

**Note on `_userId`:** The auth middleware sets `user_id` as `uuid.UUID` in the Gin context. The type assertion `uid.(uuid.UUID)` is correct for production. In tests that need to verify `_userId` propagation, set `user_id` as `uuid.UUID`, not `string`.

- [ ] **Step 4: Update test helpers in middleware_test.go**

Add the `newTestGELFClient` helper using the gelf package:

```go
import "github.com/shadsorg/deploysentry/internal/platform/gelf"

func newTestGELFClient(mt *gelfMockTransport) *gelf.Client {
	return gelf.NewTestClient(mt)
}
```

Note: `gelfMockTransport` implements `gelf.Transport` (the exported interface). The middleware test creates a mock via `gelf.NewTestClient(mt)` where `mt` satisfies `gelf.Transport`.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/platform/middleware/ -v -run "TestStructuredLogger"`
Expected: PASS

Run: `go test ./internal/platform/gelf/ -v`
Expected: PASS (no regressions)

- [ ] **Step 6: Commit**

```bash
git add internal/platform/gelf/gelf.go internal/platform/middleware/logging.go internal/platform/middleware/middleware_test.go
git commit -m "feat: wire GELF client into StructuredLogger middleware"
```

---

### Task 6: Wire GELF into ErrorHandler middleware

**Files:**
- Modify: `internal/platform/middleware/error_handler.go`
- Modify: `internal/platform/middleware/middleware_test.go`

- [ ] **Step 1: Write failing test for GELF error logging**

Append to `internal/platform/middleware/middleware_test.go`:

```go
// ---------------------------------------------------------------------------
// 9. ErrorHandler GELF integration
// ---------------------------------------------------------------------------

func TestErrorHandler_SendsGELFOnPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())

	mt := &gelfMockTransport{}
	gelfClient := newTestGELFClient(mt)

	router.Use(ErrorHandler(DefaultErrorHandlingConfig(), gelfClient))
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	require.GreaterOrEqual(t, len(mt.messages), 1)

	var msg map[string]interface{}
	require.NoError(t, json.Unmarshal(mt.messages[0], &msg))
	assert.Equal(t, "error", msg["_logType"])
	assert.Equal(t, "fatal", msg["_logLevel"])
	assert.Contains(t, msg["_errorName"], "test panic")
	assert.NotEmpty(t, msg["_errorStack"])
	assert.Equal(t, "GET", msg["_httpMethod"])
	assert.Equal(t, "/panic", msg["_httpPath"])
}

func TestErrorHandler_NilGELFClientDoesNotPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.Use(ErrorHandler(DefaultErrorHandlingConfig(), nil))
	router.GET("/panic", func(c *gin.Context) {
		panic("test")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/platform/middleware/ -v -run "TestErrorHandler_(SendsGELF|NilGELF)"`
Expected: FAIL — signature mismatch

- [ ] **Step 3: Update ErrorHandler to accept and use GELF client**

Update signature in `internal/platform/middleware/error_handler.go`:

```go
func ErrorHandler(config ErrorHandlingConfig, gelfClient *gelf.Client) gin.HandlerFunc {
```

In the panic recovery block, after existing `log.Println(logEntry)`, add:

```go
if gelfClient != nil {
    gelfClient.send(gelf.Message{
        ShortMessage: fmt.Sprintf("PANIC: %v", err),
        LogType:      "error",
        LogLevel:     "fatal",
        RequestID:    requestID,
        HTTPMethod:   c.Request.Method,
        HTTPPath:     c.Request.URL.Path,
        ErrorName:    fmt.Sprintf("%v", err),
        ErrorStack:   string(debug.Stack()),
    })
}
```

Note: Since `gelf.send` is unexported, use a new exported method instead. Add to `gelf.go`:

```go
// Fatal sends a fatal-level error log message (used for panics).
func (c *Client) Fatal(shortMessage, errorName, errorStack, requestID, method, path, userID string) {
	if c == nil {
		return
	}
	c.send(Message{
		ShortMessage: shortMessage,
		LogType:      "error",
		LogLevel:     "fatal",
		ErrorName:    errorName,
		ErrorStack:   errorStack,
		RequestID:    requestID,
		HTTPMethod:   method,
		HTTPPath:     path,
		UserID:       userID,
	})
}
```

Then in the error handler panic recovery:

```go
if gelfClient != nil {
    userID := ""
    if uid, exists := c.Get("user_id"); exists {
        if id, ok := uid.(uuid.UUID); ok {
            userID = id.String()
        }
    }
    gelfClient.Fatal(
        fmt.Sprintf("PANIC: %v", err),
        fmt.Sprintf("%v", err),
        string(debug.Stack()),
        requestID,
        c.Request.Method,
        c.Request.URL.Path,
        userID,
    )
}
```

In the request error handling block (the `c.Errors` loop), after existing `log.Println(logEntry)`, add:

```go
if gelfClient != nil {
    userID := ""
    if uid, exists := c.Get("user_id"); exists {
        if id, ok := uid.(uuid.UUID); ok {
            userID = id.String()
        }
    }
    gelfClient.ErrorWithContext(
        fmt.Sprintf("ERROR: %v", lastError.Err),
        fmt.Sprintf("%v", lastError.Err),
        "", // no stack trace for request errors
        GetRequestID(c),
        c.Request.Method,
        c.Request.URL.Path,
        userID,
    )
}
```

Add `ErrorWithContext` method to `gelf.go` (alongside `Fatal`):

```go
// ErrorWithContext sends an error log message with request context.
func (c *Client) ErrorWithContext(shortMessage, errorName, errorStack, requestID, method, path, userID string) {
    if c == nil {
        return
    }
    c.send(Message{
        ShortMessage: shortMessage,
        LogType:      "error",
        LogLevel:     "error",
        ErrorName:    errorName,
        ErrorStack:   errorStack,
        RequestID:    requestID,
        HTTPMethod:   method,
        HTTPPath:     path,
        UserID:       userID,
    })
}
```

Add imports: `"github.com/google/uuid"` and `"github.com/shadsorg/deploysentry/internal/platform/gelf"`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/platform/middleware/ -v -run "TestErrorHandler"`
Expected: PASS

- [ ] **Step 5: Run all middleware tests**

Run: `go test ./internal/platform/middleware/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/platform/gelf/gelf.go internal/platform/middleware/error_handler.go internal/platform/middleware/middleware_test.go
git commit -m "feat: wire GELF client into ErrorHandler middleware"
```

---

### Task 7: Wire GELF client in main.go and send startup message

**Files:**
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Add GELF client initialization after router creation**

In `cmd/api/main.go`, after the router and before middleware registration (~line 114), add:

```go
// Initialize GELF structured logging
gelfClient, err := gelf.NewClient("deploysentry-api")
if err != nil {
	log.Printf("warning: GELF logging disabled: %v", err)
}
defer func() {
	if gelfClient != nil {
		gelfClient.Close()
	}
}()
```

Add import: `"github.com/shadsorg/deploysentry/internal/platform/gelf"`

- [ ] **Step 2: Update middleware registration to pass GELF client**

Change:
```go
router.Use(middleware.ErrorHandler(errorConfig))
router.Use(middleware.StructuredLogger(middleware.DefaultLoggingConfig()))
```

To:
```go
loggingConfig := middleware.DefaultLoggingConfig()
loggingConfig.LogLevel = cfg.Log.Level
router.Use(middleware.ErrorHandler(errorConfig, gelfClient))
router.Use(middleware.StructuredLogger(loggingConfig, gelfClient))
```

- [ ] **Step 3: Add startup confirmation after route registration**

After the route dump loop (near the "Start HTTP Server" section), add:

```go
if gelfClient != nil {
	gelfClient.Info("deploysentry-api started")
}
```

- [ ] **Step 4: Verify the project builds**

Run: `go build ./cmd/api/`
Expected: Build succeeds with no errors

- [ ] **Step 5: Run all tests**

Run: `go test ./... 2>&1 | tail -30`
Expected: All packages pass (or pre-existing failures only)

- [ ] **Step 6: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat: wire GELF client into API server startup and middleware"
```

---

### Task 8: Final integration verification

**Files:**
- No new files

- [ ] **Step 1: Run all GELF package tests**

Run: `go test ./internal/platform/gelf/ -v -count=1`
Expected: ALL PASS

- [ ] **Step 2: Run all middleware tests**

Run: `go test ./internal/platform/middleware/ -v -count=1`
Expected: ALL PASS

- [ ] **Step 3: Run full project test suite**

Run: `go test ./... 2>&1 | grep -E "^(ok|FAIL|---)" | head -30`
Expected: No new failures introduced

- [ ] **Step 4: Verify clean build**

Run: `go build ./cmd/api/ && echo "BUILD OK"`
Expected: `BUILD OK`

- [ ] **Step 5: Commit any remaining fixes**

If any tests needed fixing:
```bash
git add -A
git commit -m "fix: address test failures from GELF integration"
```
