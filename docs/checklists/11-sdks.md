# 11 — SDKs (Feature Flag Client Libraries)

## SDK Core Responsibilities (All SDKs)
- [ ] Local flag cache with configurable TTL
- [ ] Streaming updates via SSE / gRPC streams
- [ ] Offline mode with stale cache
- [ ] Context enrichment hooks
- [ ] Evaluation telemetry (opt-in)
- [ ] Graceful degradation on API failure
- [ ] Thread safety / concurrency safety

## Go SDK (`sdk/go/`) — Priority P0
- [ ] Client initialization with API key and base URL
- [ ] gRPC transport + HTTP fallback
- [ ] Flag evaluation: `client.BoolValue(key, defaultVal, context)`
- [ ] All flag types: boolean, string, number, JSON
- [ ] Local cache (in-memory with Redis optional)
- [ ] gRPC streaming for real-time flag updates
- [ ] Context builder for evaluation context
- [ ] Evaluation telemetry reporting
- [ ] Offline mode (serve from cache when API unavailable)
- [ ] Client shutdown / cleanup
- [ ] Unit tests with testify
- [ ] Integration tests
- [ ] README and usage examples

## Node.js / TypeScript SDK (`sdk/node/`) — Priority P0
- [ ] Client initialization with API key and base URL
- [ ] HTTP transport + SSE for streaming updates
- [ ] Flag evaluation: `client.boolValue(key, defaultVal, context)`
- [ ] All flag types: boolean, string, number, JSON
- [ ] In-memory cache with TTL
- [ ] SSE listener for real-time flag updates
- [ ] TypeScript type definitions
- [ ] Promise-based API
- [ ] Evaluation telemetry reporting
- [ ] Offline mode
- [ ] Unit tests with Jest
- [ ] README and usage examples
- [ ] npm package configuration

## React SDK (`sdk/react/`) — Priority P0
- [ ] React context provider (`<DeploySentryProvider>`)
- [ ] `useFlag(key, defaultValue)` hook
- [ ] `useFlags()` hook for all flags
- [ ] `withFlag(key)` HOC (optional)
- [ ] Automatic re-render on flag updates (SSE subscription)
- [ ] SSR support
- [ ] TypeScript type definitions
- [ ] Unit tests with React Testing Library
- [ ] README and usage examples

## Python SDK (`sdk/python/`) — Priority P1
- [ ] Client initialization with API key and base URL
- [ ] HTTP transport
- [ ] Flag evaluation: `client.bool_value(key, default_val, context)`
- [ ] All flag types: boolean, string, number, JSON
- [ ] In-memory cache with TTL
- [ ] Thread-safe evaluation
- [ ] Evaluation telemetry reporting
- [ ] Offline mode
- [ ] Unit tests with pytest
- [ ] README and usage examples
- [ ] PyPI package configuration

## Java / Kotlin SDK (`sdk/java/`) — Priority P1
- [ ] Client initialization with API key and base URL
- [ ] gRPC transport
- [ ] Flag evaluation: `client.boolValue(key, defaultVal, context)`
- [ ] All flag types: boolean, string, number, JSON
- [ ] In-memory cache with TTL
- [ ] gRPC streaming for real-time flag updates
- [ ] Thread-safe evaluation
- [ ] Evaluation telemetry reporting
- [ ] Offline mode
- [ ] Unit tests with JUnit
- [ ] README and usage examples
- [ ] Maven/Gradle configuration

## Flutter / Dart SDK (`sdk/flutter/`) — Priority P1
- [ ] Client initialization with API key and base URL
- [ ] HTTP transport (using `http` or `dio` package)
- [ ] Flag evaluation: `client.boolValue(key, defaultValue, context)`
- [ ] All flag types: boolean, string, number, JSON
- [ ] In-memory cache with TTL
- [ ] SSE listener for real-time flag updates
- [ ] `DeploySentryProvider` InheritedWidget for reactive flag access
- [ ] `DeploySentry.of(context)` accessor for evaluating flags in widgets
- [ ] Automatic widget rebuild on flag change via `ChangeNotifier`
- [ ] Offline mode (serve from cache when API unavailable)
- [ ] Exponential backoff reconnection for SSE stream
- [ ] Evaluation telemetry reporting (opt-in)
- [ ] Platform-aware context enrichment (iOS/Android/web)
- [ ] Unit tests with `flutter_test`
- [ ] Integration tests with `integration_test`
- [ ] README and usage examples
- [ ] pub.dev package configuration

## Ruby SDK (`sdk/ruby/`) — Priority P2
- [ ] Client initialization
- [ ] HTTP transport
- [ ] Flag evaluation
- [ ] Caching and offline mode
- [ ] Unit tests with RSpec

## Contract Tests (All SDKs)
- [ ] Pact contract tests for SDK ↔ API compatibility
- [ ] Shared test fixtures across SDK languages
- [ ] CI integration for contract verification
