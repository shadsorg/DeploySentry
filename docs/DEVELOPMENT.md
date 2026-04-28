# Development Guide

Welcome to DeploySentry development! This guide will help you get up and running quickly with a complete local development environment.

## Quick Start

```bash
# One-command setup for new developers
./scripts/dev-setup.sh

# Daily development workflow
./scripts/dev.sh start     # Start services
./scripts/dev.sh api       # Start API (separate terminal)
./scripts/dev.sh web       # Start web UI (separate terminal)
```

## Development CLI

The `./scripts/dev.sh` script provides a unified interface for all development tasks:

### Environment Management
```bash
./scripts/dev.sh setup              # Complete environment setup
./scripts/dev.sh start              # Start all services (PostgreSQL, Redis, NATS)
./scripts/dev.sh stop               # Stop all services
./scripts/dev.sh restart            # Restart all services
./scripts/dev.sh status             # Show service status and resource usage
```

### Development Servers
```bash
./scripts/dev.sh api                # Start API server with hot reload
./scripts/dev.sh web                # Start React dev server
./scripts/dev.sh debug 2345         # Start API with debugger on port 2345
```

### Database Operations
```bash
./scripts/dev.sh migrate            # Run database migrations
./scripts/dev.sh reset-db           # Reset database (destructive)
./scripts/dev.sh seed               # Seed with sample data
```

### Testing & Quality
```bash
./scripts/dev.sh test               # Run all tests
./scripts/dev.sh test unit          # Unit tests only
./scripts/dev.sh test integration   # Integration tests only
./scripts/dev.sh lint               # Run Go and TypeScript linters
./scripts/dev.sh format             # Format all code
./scripts/dev.sh bench              # Run benchmarks
./scripts/dev.sh profile            # Generate performance profiles
```

### Build & CLI
```bash
./scripts/dev.sh build              # Build API and CLI binaries
./scripts/dev.sh cli flags list     # Use CLI tool
./scripts/dev.sh clean              # Clean build artifacts
```

### Debugging & Monitoring
```bash
./scripts/dev.sh logs               # Show all service logs
./scripts/dev.sh logs postgres      # Show specific service logs
./scripts/dev.sh status             # Resource usage and health
```

## Architecture Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   React Web     │    │   Flutter       │    │   CLI Tool      │
│   Dashboard     │    │   Mobile App    │    │   (Go)          │
│   (TypeScript)  │    │   (Dart)        │    │                 │
└─────────┬───────┘    └─────────┬───────┘    └─────────┬───────┘
          │                      │                      │
          └──────────────────────┼──────────────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │   DeploySentry API      │
                    │   (Go + Gin)            │
                    │   Port: 8080            │
                    └──────────┬──────────────┘
                               │
        ┌──────────────────────┼──────────────────────┐
        │                      │                      │
┌───────▼───────┐    ┌─────────▼─────────┐    ┌───────▼───────┐
│  PostgreSQL    │    │     Redis         │    │     NATS      │
│  Port: 5432    │    │   Port: 6379      │    │  Port: 4222   │
│  (Database)    │    │   (Cache)         │    │ (Messaging)   │
└────────────────┘    └───────────────────┘    └───────────────┘
```

## Development Workflow

### 1. First-Time Setup

```bash
# Clone and setup (one time only)
git clone https://github.com/shadsorg/deploysentry.git
cd deploysentry
./scripts/dev-setup.sh

# This script will:
# - Check prerequisites (Docker, Go, Node.js)
# - Generate secure .env file
# - Start PostgreSQL, Redis, NATS
# - Run database migrations
# - Install dependencies
# - Build binaries
# - Run tests
```

### 2. Daily Development

```bash
# Start your day
./scripts/dev.sh start              # Start backing services
./scripts/dev.sh api                # Terminal 1: API server
./scripts/dev.sh web                # Terminal 2: Web dashboard

# Make changes, then test
./scripts/dev.sh test unit           # Quick unit tests
./scripts/dev.sh test                # Full test suite

# Format and lint before committing
./scripts/dev.sh format              # Auto-format code
./scripts/dev.sh lint                # Check for issues
```

### 3. Working with Features

```bash
# Create a feature flag via CLI
./scripts/dev.sh cli flags create \
  --key "new-feature" \
  --name "New Feature" \
  --type boolean \
  --category feature \
  --default false

# View flags in web dashboard
open http://localhost:3000/flags

# Test API directly
curl http://localhost:8080/health
curl -H "Authorization: ApiKey your-key" \
  http://localhost:8080/api/v1/flags
```

### 4. Database Development

```bash
# Create new migration
./scripts/dev.sh cli db create-migration "add_new_table"

# Apply migrations
./scripts/dev.sh migrate

# Reset for clean state (destructive!)
./scripts/dev.sh reset-db

# View schema
psql postgres://deploysentry:deploysentry@localhost:5432/deploysentry?search_path=deploy
```

## Environment Details

### Services Configuration

| Service      | Host      | Port | Purpose                    |
|--------------|-----------|------|----------------------------|
| API Server   | localhost | 8080 | Main DeploySentry API      |
| Web UI       | localhost | 3000 | React development server   |
| PostgreSQL   | localhost | 5432 | Primary database           |
| Redis        | localhost | 6379 | Cache and rate limiting    |
| NATS         | localhost | 4222 | Message queue              |

### Environment Variables

The development environment uses these key variables (auto-generated in `.env`):

```bash
DS_ENVIRONMENT=development          # Enables dev features
DS_DATABASE_SSL_MODE=disable        # Local development
DS_LOG_LEVEL=debug                  # Verbose logging
DS_LOG_FORMAT=text                  # Human-readable logs
DS_AUTH_JWT_SECRET=<generated>      # Secure random secret
```

### Database Schema

- **Namespace**: All tables in `deploy` schema (not `public`)
- **Connection**: `postgres://deploysentry:deploysentry@localhost:5432/deploysentry?search_path=deploy`
- **Migrations**: In `migrations/` directory, applied automatically

## Testing Strategy

### Unit Tests
```bash
./scripts/dev.sh test unit

# Tests individual functions and components
# No external dependencies
# Fast execution (< 30 seconds)
```

### Integration Tests
```bash
./scripts/dev.sh test integration

# Tests complete workflows
# Uses real PostgreSQL, Redis, NATS
# Slower execution (1-2 minutes)
```

### Manual Testing
```bash
# API Health Check
curl http://localhost:8080/health

# Create test flag
./scripts/dev.sh cli flags create --key test --type boolean

# Web UI testing
open http://localhost:3000
```

## Debugging

### API Server Debugging

```bash
# Run with delve debugger
./scripts/dev.sh debug 2345

# Connect from VS Code launch.json:
{
  "name": "Connect to Delve",
  "type": "go",
  "request": "attach",
  "mode": "remote",
  "remotePath": "${workspaceFolder}",
  "port": 2345,
  "host": "127.0.0.1"
}
```

### Database Debugging

```bash
# Connect to database
psql postgres://deploysentry:deploysentry@localhost:5432/deploysentry?search_path=deploy

# View tables
\dt

# Query flags
SELECT key, name, enabled FROM feature_flags;

# Monitor queries
SELECT query FROM pg_stat_activity WHERE state = 'active';
```

### Service Monitoring

```bash
# View all service logs
./scripts/dev.sh logs

# Monitor resource usage
./scripts/dev.sh status

# Check API metrics
curl http://localhost:8080/metrics
```

## Performance Profiling

### CPU & Memory Profiles

```bash
# Generate profiles
./scripts/dev.sh profile

# Analyze CPU usage
go tool pprof cpu.prof

# Analyze memory usage
go tool pprof mem.prof
```

### Benchmarks

```bash
# Run all benchmarks
./scripts/dev.sh bench

# Run specific benchmarks
go test -bench=BenchmarkFlagEvaluation ./internal/flags/
```

### Load Testing

```bash
# Install hey (HTTP load testing tool)
go install github.com/rakyll/hey@latest

# Test API under load
hey -n 1000 -c 10 http://localhost:8080/health

# Test flag evaluation
hey -n 1000 -c 10 \
  -H "Authorization: ApiKey your-key" \
  http://localhost:8080/api/v1/flags/evaluate
```

## Code Standards

### Go Guidelines
- Use `gofmt` for formatting (automated via `./scripts/dev.sh format`)
- Follow effective Go conventions
- Write tests for all public functions
- Use context for timeouts and cancellation
- Handle errors explicitly

### TypeScript Guidelines
- Use ESLint and Prettier (automated via `./scripts/dev.sh lint`)
- Prefer functional components with hooks
- Use TypeScript strict mode
- Write type definitions for all props

### Database Guidelines
- All migrations must have up and down versions
- Use descriptive migration names
- Never modify existing migrations
- Add database constraints for data integrity

### API Guidelines
- RESTful endpoints with consistent patterns
- Use proper HTTP status codes
- Include request IDs for tracing
- Validate input and sanitize output

## Troubleshooting

### Common Issues

**"Port already in use"**
```bash
# Kill processes on conflicting ports
lsof -ti:8080 | xargs kill -9
lsof -ti:3000 | xargs kill -9
```

**"Database connection failed"**
```bash
# Restart database service
./scripts/dev.sh stop
./scripts/dev.sh start

# Check database logs
./scripts/dev.sh logs postgres
```

**"Migration failed"**
```bash
# Reset database (destructive)
./scripts/dev.sh reset-db

# Or check migration status manually
./scripts/dev.sh cli db status
```

**"Tests failing"**
```bash
# Ensure services are running
./scripts/dev.sh status

# Run tests with verbose output
go test -v ./...

# Run specific test
go test -v -run TestFlagEvaluation ./internal/flags/
```

### Getting Help

1. **Check logs**: `./scripts/dev.sh logs`
2. **Verify services**: `./scripts/dev.sh status`
3. **Reset environment**: `./scripts/dev.sh stop && ./scripts/dev-setup.sh`
4. **Check documentation**: This file and `README.md`
5. **Ask the team**: Create issue or discussion on GitHub

## IDE Configuration

### VS Code

Recommended extensions:
- Go (golang.go)
- ES7+ React/Redux/React-Native snippets
- Prettier - Code formatter
- GitLens

Settings (`.vscode/settings.json`):
```json
{
  "go.toolsManagement.checkForUpdates": "local",
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "package",
  "editor.formatOnSave": true,
  "[typescript]": {
    "editor.defaultFormatter": "esbenp.prettier-vscode"
  },
  "[typescriptreact]": {
    "editor.defaultFormatter": "esbenp.prettier-vscode"
  }
}
```

Launch configuration (`.vscode/launch.json`):
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch API Server",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/cmd/api",
      "env": {
        "DS_ENVIRONMENT": "development"
      }
    },
    {
      "name": "Attach to Delve",
      "type": "go",
      "request": "attach",
      "mode": "remote",
      "remotePath": "${workspaceFolder}",
      "port": 2345,
      "host": "127.0.0.1"
    }
  ]
}
```

Happy coding! 🚀