// Package mobilepwa serves the built Mobile PWA static assets from the
// API binary. The dist/ directory is populated by `make build-mobile` (or
// `make embed-mobile-pwa`) before `go build`. A .gitkeep placeholder lives
// in dist/ so the embed has at least one file to attach to in dev builds.
package mobilepwa

import "embed"

//go:embed all:dist
var assets embed.FS
