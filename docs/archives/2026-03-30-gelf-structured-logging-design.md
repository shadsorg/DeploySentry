# GELF Structured Logging

## Overview

Add structured JSON logging to Graylog via GELF as an additional transport alongside existing console logging. The GELF client auto-selects UDP (local) or HTTP (remote/cloud) based on whether Cloudflare Access credentials are present in the environment.

## Transport Selection

No configuration fields required. Transport is derived from environment variables at startup:

| Condition | Transport | Target |
|---|---|---|
| `CF_ACCESS_CLIENT_ID` is set | GELF HTTP POST | `https://ingest.crowdsoftapps.com/gelf` |
| `CF_ACCESS_CLIENT_ID` is not set | GELF UDP | `localhost:12201` |

HTTP transport includes `CF-Access-Client-Id` and `CF-Access-Client-Secret` headers from the corresponding environment variables.

## New Package: `internal/platform/gelf/`

### Files

- **`gelf.go`** — `Client` struct, `Message` struct, `NewClient()` constructor
- **`transport_udp.go`** — UDP sender to `localhost:12201`
- **`transport_http.go`** — HTTP sender with Cloudflare Access headers

### Client

```go
type Client struct {
    transport transport
    appName   string // fixed at construction, set as _appName
    env       string // fixed at construction, set as _env
    hostname  string // resolved once via os.Hostname()
}

func NewClient(appName string) (*Client, error)
```

`NewClient` reads:
- `CF_ACCESS_CLIENT_ID` and `CF_ACCESS_CLIENT_SECRET` from environment (transport selection)
- `DS_ENVIRONMENT` from environment (default: `"development"`) for the `_env` field
- `os.Hostname()` for the GELF `host` field

Returns an error only if the selected transport cannot be initialized (e.g., UDP dial failure). The caller decides whether to treat this as fatal or log a warning and continue without GELF.

### Message

```go
type Message struct {
    // Standard GELF fields
    Version      string  `json:"version"`       // always "1.1"
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
```

The `Version` field is always set to `"1.1"` by the client. Callers never set it.

### Convenience Methods on Client

```go
func (c *Client) Info(shortMessage string)
func (c *Client) Error(shortMessage string, errorName string, errorStack string)
func (c *Client) Request(requestID, method, path, userID string)
func (c *Client) Response(requestID, method, path, userID string, status int, durationMs int64)
```

Each method builds a `Message` with the fixed fields (`version`, `host`, `_appName`, `_env`, `timestamp`) pre-filled, then calls the transport. All methods are nil-safe (no-op if client is nil).

**`_logType` and `_logLevel` mapping by method:**

| Method | `_logType` | `_logLevel` | `short_message` format |
|---|---|---|---|
| `Info` | `info` | `info` | passed as argument |
| `Error` | `error` | `error` | passed as argument |
| `Request` | `request` | `info` | `"<method> <path>"` |
| `Response` | `response` | varies* | `"<status> <method> <path> <durationMs>ms"` |

*Response `_logLevel`: `"error"` if status >= 500, `"warn"` if status >= 400, `"info"` otherwise.

### GELF Level Mapping

| `_logLevel` | GELF `level` |
|---|---|
| `fatal` | 2 |
| `error` | 3 |
| `warn` | 4 |
| `info` | 6 |
| `debug` | 7 |
| `trace` | 7 |

### Transport Interface

```go
type transport interface {
    Send(data []byte) error
    Close() error
}
```

### UDP Transport (`transport_udp.go`)

- Dials `localhost:12201` via `net.DialUDP`
- Writes JSON-encoded message as a single UDP datagram
- If the serialized message exceeds 8192 bytes, truncate the `_errorStack` field (the only variable-length field that can realistically hit this limit) before serialization to bring the payload under the limit. GELF UDP chunking is not implemented.
- Send errors are logged to stderr via `log.Printf` and swallowed — never block the caller
- `Close()` closes the underlying `net.UDPConn`

### HTTP Transport (`transport_http.go`)

- POST JSON to `https://ingest.crowdsoftapps.com/gelf`
- Headers: `Content-Type: application/json`, `CF-Access-Client-Id`, `CF-Access-Client-Secret`
- 2-second timeout per request
- Sends asynchronously via a buffered channel (capacity 256) drained by a background goroutine, so logging never blocks request handling
- Dropped messages (full buffer) log a warning to stderr
- Send failures (network errors, non-2xx responses) are logged to stderr and discarded — no retry
- `Close()` drains the buffer and shuts down the goroutine

## Middleware Changes

### `StructuredLogger` (`internal/platform/middleware/logging.go`)

**Signature change:**

```go
// Before
func StructuredLogger(config LoggingConfig) gin.HandlerFunc

// After
func StructuredLogger(config LoggingConfig, gelf *gelf.Client) gin.HandlerFunc
```

The `gelf` parameter is nil-safe. When nil, behavior is identical to today.

**Request start** (before `c.Next()`):
- Send GELF `_logType: "request"` with `_requestId`, `_httpMethod`, `_httpPath`, `_logLevel: "info"`

**Request end** (after `c.Next()`):
- Send GELF `_logType: "response"` with `_requestId`, `_httpMethod`, `_httpPath`, `_httpStatus`, `_requestTimeMs`
- Extract `_userId` from Gin context (`user_id` key set by auth middleware) — available after `c.Next()` returns since auth middleware runs during `Next()`
- `_logLevel`: `"error"` if status >= 500, `"warn"` if status >= 400, `"info"` otherwise

**Note on `_userId`:** The `StructuredLogger` runs as global middleware, before auth middleware. The `_userId` field is only available after `c.Next()` returns (i.e., on response logs, not request logs), because the auth middleware sets it during request processing. Request-start logs will not include `_userId`.

**Unchanged:** Existing `log.Println` console output remains as-is.

### Health Check / Skip-Paths Behavior

The existing `SkipPaths` config (`/health`, `/ready`, `/metrics`) controls which paths are excluded from logging. This applies to **both** console and GELF output.

**Log-level gating:** When `DS_LOG_LEVEL` is set to `trace`, skip-paths filtering is disabled — health check endpoints are logged to both console and GELF. At all other log levels, skip-paths are excluded from both. This enables debugging health check issues in development or troubleshooting without changing the skip-paths config.

Implementation: the `StructuredLogger` middleware receives the current log level (passed via `LoggingConfig` or as an additional parameter) and only applies skip-paths filtering when the level is not `trace`.

### `ErrorHandler` (`internal/platform/middleware/error_handler.go`)

**Signature change:**

```go
// Before
func ErrorHandler(config ErrorHandlingConfig) gin.HandlerFunc

// After
func ErrorHandler(config ErrorHandlingConfig, gelf *gelf.Client) gin.HandlerFunc
```

On panic recovery or error handling, send a GELF message:
- `_logType: "error"`, `_logLevel: "fatal"` for panics, `"error"` for request errors
- `_errorName`: error type/message
- `_errorStack`: stack trace string (truncated if needed for UDP transport)
- `_requestId`, `_httpMethod`, `_httpPath`, `_userId` from context

## Startup Wiring (`cmd/api/main.go`)

```go
gelfClient, err := gelf.NewClient("deploysentry-api")
if err != nil {
    log.Printf("warning: GELF logging disabled: %v", err)
}
defer func() {
    if gelfClient != nil {
        gelfClient.Close()
    }
}()

// Pass to middleware
router.Use(middleware.ErrorHandler(errorConfig, gelfClient))
router.Use(middleware.StructuredLogger(middleware.DefaultLoggingConfig(), gelfClient))

// Startup confirmation
if gelfClient != nil {
    gelfClient.Info("deploysentry-api started")
}
```

The `defer gelfClient.Close()` runs after the server's graceful shutdown completes (after the `select` block exits), ensuring in-flight requests finish before the HTTP transport's buffer is drained.

## Field Summary by Log Type

Every message includes: `version`, `host`, `short_message`, `timestamp`, `level`, `_appName`, `_env`, `_logType`, `_logLevel`.

| `_logType` | Additional Fields |
|---|---|
| `request` | `_requestId`, `_httpMethod`, `_httpPath` |
| `response` | `_requestId`, `_httpMethod`, `_httpPath`, `_httpStatus`, `_requestTimeMs`, `_userId`* |
| `error` | `_requestId`*, `_httpMethod`*, `_httpPath`*, `_errorName`, `_errorStack`, `_userId`* |
| `info` | *(base fields + short_message only)* |

*\* omitted when not available*

## What Does Not Change

- All existing `log.Println` / `log.Printf` calls throughout the codebase
- `RequestID` middleware (already generates UUIDs, already in context)
- `Config` struct (no new fields — transport derived from env vars)
- Request/response body content is never sent to GELF (PII risk)

## Dependencies

No new third-party dependencies. GELF is a simple JSON-over-UDP/HTTP protocol. Standard library `net`, `net/http`, and `encoding/json` are sufficient.

## Error Handling Philosophy

GELF logging is best-effort. A failure to send a log message must never:
- Block a request
- Cause a request to fail
- Panic the application

Send failures are logged to stderr and discarded.
