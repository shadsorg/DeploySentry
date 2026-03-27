#!/bin/bash
set -euo pipefail

# DeploySentry Development CLI
# Provides quick access to common development tasks

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}ℹ ${1}${NC}"
}

log_success() {
    echo -e "${GREEN}✅ ${1}${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️ ${1}${NC}"
}

log_error() {
    echo -e "${RED}❌ ${1}${NC}"
}

# Show usage information
usage() {
    cat << EOF
${CYAN}DeploySentry Development CLI${NC}

${BLUE}Usage:${NC} $0 <command> [options]

${BLUE}Commands:${NC}
  ${GREEN}setup${NC}              Set up complete development environment
  ${GREEN}start${NC}              Start all development services
  ${GREEN}stop${NC}               Stop all development services
  ${GREEN}restart${NC}            Restart all development services
  ${GREEN}status${NC}             Show status of development services
  ${GREEN}logs [service]${NC}     Show logs (postgres, redis, nats, or all)
  ${GREEN}reset-db${NC}           Reset database (drop and recreate)
  ${GREEN}migrate${NC}            Run database migrations
  ${GREEN}seed${NC}               Seed database with sample data
  ${GREEN}test [type]${NC}        Run tests (unit, integration, or all)
  ${GREEN}build${NC}              Build API and CLI binaries
  ${GREEN}lint${NC}               Run linters on Go and TypeScript code
  ${GREEN}format${NC}             Format code using gofmt and prettier
  ${GREEN}clean${NC}              Clean build artifacts and temporary files
  ${GREEN}docs${NC}               Generate and serve API documentation
  ${GREEN}api${NC}                Start API server in development mode
  ${GREEN}web${NC}                Start web development server
  ${GREEN}cli <args>${NC}         Run CLI tool with arguments
  ${GREEN}debug <port>${NC}       Start API with delve debugger
  ${GREEN}bench${NC}              Run benchmarks
  ${GREEN}profile${NC}            Generate performance profiles

${BLUE}Examples:${NC}
  $0 setup                    # Complete environment setup
  $0 start                    # Start all services
  $0 test unit               # Run unit tests only
  $0 logs postgres           # Show postgres logs
  $0 cli flags list          # Run CLI command
  $0 debug 2345              # Start debugger on port 2345

${BLUE}Environment:${NC}
  Development services run on:
  • PostgreSQL: localhost:5432
  • Redis: localhost:6379
  • NATS: localhost:4222
  • API: localhost:8080
  • Web: localhost:3000
EOF
}

# Check if docker-compose services are running
check_services() {
    cd "$PROJECT_ROOT"
    if ! docker-compose -f deploy/docker-compose.yml ps --services --filter "status=running" | grep -q .; then
        return 1
    fi
    return 0
}

# Start development services
cmd_start() {
    log_info "Starting development services..."
    cd "$PROJECT_ROOT"

    # Check if .env exists
    if [ ! -f ".env" ]; then
        log_warning "No .env file found. Run '$0 setup' first."
        exit 1
    fi

    make dev-up
    log_success "Development services started"
    log_info "Services available at: PostgreSQL :5432, Redis :6379, NATS :4222"
}

# Stop development services
cmd_stop() {
    log_info "Stopping development services..."
    cd "$PROJECT_ROOT"
    make dev-down
    log_success "Development services stopped"
}

# Restart development services
cmd_restart() {
    cmd_stop
    cmd_start
}

# Show service status
cmd_status() {
    cd "$PROJECT_ROOT"
    echo -e "${BLUE}Development Services Status:${NC}"
    docker-compose -f deploy/docker-compose.yml ps

    echo ""
    echo -e "${BLUE}Resource Usage:${NC}"
    docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}" | head -4
}

# Show logs
cmd_logs() {
    local service=${1:-}
    cd "$PROJECT_ROOT"

    if [ -z "$service" ] || [ "$service" = "all" ]; then
        docker-compose -f deploy/docker-compose.yml logs -f
    else
        case "$service" in
            postgres|postgresql|db)
                docker-compose -f deploy/docker-compose.yml logs -f postgres
                ;;
            redis)
                docker-compose -f deploy/docker-compose.yml logs -f redis
                ;;
            nats)
                docker-compose -f deploy/docker-compose.yml logs -f nats
                ;;
            *)
                log_error "Unknown service: $service"
                log_info "Available services: postgres, redis, nats, all"
                exit 1
                ;;
        esac
    fi
}

# Reset database
cmd_reset_db() {
    log_warning "This will DELETE all data in the development database!"
    read -p "Are you sure? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Database reset cancelled"
        exit 0
    fi

    log_info "Resetting database..."
    cd "$PROJECT_ROOT"

    # Stop services, remove volumes, restart
    docker-compose -f deploy/docker-compose.yml down -v
    docker-compose -f deploy/docker-compose.yml up -d postgres redis nats

    # Wait for postgres to be ready
    sleep 5

    # Run migrations
    make migrate-up

    log_success "Database reset complete"
}

# Run database migrations
cmd_migrate() {
    log_info "Running database migrations..."
    cd "$PROJECT_ROOT"
    make migrate-up
    log_success "Database migrations complete"
}

# Seed database with sample data
cmd_seed() {
    log_info "Seeding database with sample data..."
    cd "$PROJECT_ROOT"

    # This would run a seeding script when implemented
    if [ -f "scripts/seed-data.sh" ]; then
        ./scripts/seed-data.sh
    else
        log_warning "Seeding script not yet implemented"
        log_info "You can manually create data using the CLI or web interface"
    fi
}

# Run tests
cmd_test() {
    local test_type=${1:-all}
    cd "$PROJECT_ROOT"

    if ! check_services; then
        log_warning "Development services not running. Starting them..."
        cmd_start
        sleep 5
    fi

    case "$test_type" in
        unit)
            log_info "Running unit tests..."
            make test-unit
            ;;
        integration|int)
            log_info "Running integration tests..."
            make test-int
            ;;
        all)
            log_info "Running all tests..."
            make test
            ;;
        *)
            log_error "Unknown test type: $test_type"
            log_info "Available types: unit, integration, all"
            exit 1
            ;;
    esac

    log_success "Tests completed"
}

# Build project
cmd_build() {
    log_info "Building project..."
    cd "$PROJECT_ROOT"
    make build
    log_success "Build complete"
    log_info "Binaries available in bin/ directory"
}

# Run linters
cmd_lint() {
    log_info "Running linters..."
    cd "$PROJECT_ROOT"

    # Go linting
    if command -v golangci-lint >/dev/null 2>&1; then
        log_info "Running golangci-lint..."
        golangci-lint run
    else
        log_warning "golangci-lint not installed, using go vet..."
        go vet ./...
    fi

    # TypeScript linting
    if [ -d "web/node_modules" ]; then
        log_info "Running ESLint on web code..."
        cd web
        npm run lint
        cd ..
    else
        log_warning "Web dependencies not installed, skipping TypeScript linting"
    fi

    log_success "Linting complete"
}

# Format code
cmd_format() {
    log_info "Formatting code..."
    cd "$PROJECT_ROOT"

    # Format Go code
    log_info "Formatting Go code..."
    go fmt ./...

    # Format TypeScript/React code
    if [ -d "web/node_modules" ]; then
        log_info "Formatting TypeScript code..."
        cd web
        npx prettier --write "src/**/*.{ts,tsx,css,json}"
        cd ..
    fi

    log_success "Code formatting complete"
}

# Clean build artifacts
cmd_clean() {
    log_info "Cleaning build artifacts..."
    cd "$PROJECT_ROOT"

    # Remove Go build artifacts
    rm -rf bin/
    go clean -cache

    # Remove web build artifacts
    if [ -d "web/dist" ]; then
        rm -rf web/dist
    fi

    # Remove temporary files
    find . -name "*.log" -type f -delete
    find . -name ".DS_Store" -type f -delete

    log_success "Cleanup complete"
}

# Start API server
cmd_api() {
    log_info "Starting API server in development mode..."
    cd "$PROJECT_ROOT"

    if ! check_services; then
        log_warning "Development services not running. Starting them..."
        cmd_start
        sleep 5
    fi

    # Load environment variables
    set -a
    [ -f .env ] && source .env
    set +a

    # Set development environment
    export DS_ENVIRONMENT=development
    export DS_LOG_LEVEL=debug
    export DS_LOG_FORMAT=text

    log_info "API server starting on http://localhost:${DS_SERVER_PORT:-8080}"
    go run ./cmd/api
}

# Start web development server
cmd_web() {
    log_info "Starting web development server..."
    cd "$PROJECT_ROOT/web"

    if [ ! -d "node_modules" ]; then
        log_info "Installing web dependencies..."
        npm ci --ignore-scripts
    fi

    npm run dev
}

# Run CLI tool
cmd_cli() {
    cd "$PROJECT_ROOT"

    if [ ! -f "bin/deploysentry" ]; then
        log_info "CLI binary not found, building..."
        go build -o bin/deploysentry ./cmd/cli
    fi

    ./bin/deploysentry "$@"
}

# Start API with debugger
cmd_debug() {
    local port=${1:-2345}
    log_info "Starting API with delve debugger on port $port..."
    cd "$PROJECT_ROOT"

    if ! command -v dlv >/dev/null 2>&1; then
        log_info "Installing delve debugger..."
        go install github.com/go-delve/delve/cmd/dlv@latest
    fi

    if ! check_services; then
        log_warning "Development services not running. Starting them..."
        cmd_start
        sleep 5
    fi

    # Load environment variables
    set -a
    [ -f .env ] && source .env
    set +a

    export DS_ENVIRONMENT=development
    export DS_LOG_LEVEL=debug

    log_info "Debugger will listen on :$port"
    log_info "Connect your IDE debugger to localhost:$port"
    dlv debug --headless --listen=":$port" --api-version=2 ./cmd/api
}

# Run benchmarks
cmd_bench() {
    log_info "Running benchmarks..."
    cd "$PROJECT_ROOT"
    go test -bench=. -benchmem ./...
}

# Generate performance profiles
cmd_profile() {
    log_info "Generating performance profiles..."
    cd "$PROJECT_ROOT"

    # CPU profile
    go test -cpuprofile=cpu.prof -bench=. ./...

    # Memory profile
    go test -memprofile=mem.prof -bench=. ./...

    log_success "Profiles generated: cpu.prof, mem.prof"
    log_info "View with: go tool pprof cpu.prof"
}

# Main command dispatcher
main() {
    if [ $# -eq 0 ]; then
        usage
        exit 0
    fi

    local cmd=$1
    shift

    case "$cmd" in
        setup)
            "$SCRIPT_DIR/dev-setup.sh"
            ;;
        start)
            cmd_start
            ;;
        stop)
            cmd_stop
            ;;
        restart)
            cmd_restart
            ;;
        status)
            cmd_status
            ;;
        logs)
            cmd_logs "$@"
            ;;
        reset-db)
            cmd_reset_db
            ;;
        migrate)
            cmd_migrate
            ;;
        seed)
            cmd_seed
            ;;
        test)
            cmd_test "$@"
            ;;
        build)
            cmd_build
            ;;
        lint)
            cmd_lint
            ;;
        format)
            cmd_format
            ;;
        clean)
            cmd_clean
            ;;
        api)
            cmd_api
            ;;
        web)
            cmd_web
            ;;
        cli)
            cmd_cli "$@"
            ;;
        debug)
            cmd_debug "$@"
            ;;
        bench)
            cmd_bench
            ;;
        profile)
            cmd_profile
            ;;
        help|-h|--help)
            usage
            ;;
        *)
            log_error "Unknown command: $cmd"
            echo ""
            usage
            exit 1
            ;;
    esac
}

# Run main function
main "$@"