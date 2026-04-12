#!/bin/bash
# Build the DeploySentry API server binary
set -e
cd "$(dirname "$0")/.."
go build -o bin/deploysentry-api ./cmd/api/main.go
echo "Built bin/deploysentry-api"
