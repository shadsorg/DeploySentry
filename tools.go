//go:build tools
// +build tools

// Package deploysentry declares tool and future dependencies to prevent
// go mod tidy from removing them before they are used in application code.
package deploysentry

import (
	_ "github.com/golang-jwt/jwt/v5"
	_ "github.com/golang-migrate/migrate/v4"
	_ "github.com/google/uuid"
	_ "github.com/lib/pq"
	_ "github.com/stretchr/testify"
	_ "go.opentelemetry.io/otel"
	_ "golang.org/x/oauth2"
)
