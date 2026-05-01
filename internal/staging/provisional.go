// Package staging implements the per-user staging layer described in
// docs/superpowers/specs/2026-04-30-staged-changes-and-deploy-workflow-design.md.
package staging

import (
	"fmt"

	"github.com/google/uuid"
)

// Provisional UUIDs are minted with a non-RFC-4122 variant so they are
// constant-time distinguishable from any production-minted UUID. Any leak of
// a provisional id past the staging boundary (audit log, webhook payload,
// SSE broadcast, analytics emit) is a bug — call MustNotBeProvisional at
// every egress surface to guarantee that.
//
// RFC 4122 variant lives in the top bits of byte 8:
//   10xx_xxxx — RFC 4122 (what uuid.New() always produces)
//   11xx_xxxx — reserved (Microsoft + future). We use this for provisional.
const (
	provisionalVariantMask byte = 0xc0
	provisionalVariantBits byte = 0xc0
)

// NewProvisional mints a fresh provisional UUID. It begins life as a v4 UUID
// and then has its variant bits forced to the reserved range.
func NewProvisional() uuid.UUID {
	u := uuid.New()
	u[8] = (u[8] & 0x3f) | provisionalVariantBits
	return u
}

// IsProvisional reports whether id was minted by NewProvisional.
// uuid.Nil is treated as not provisional (callers test for Nil separately).
func IsProvisional(id uuid.UUID) bool {
	if id == uuid.Nil {
		return false
	}
	return id[8]&provisionalVariantMask == provisionalVariantBits
}

// MustNotBeProvisional panics with an egress-site label if id is provisional.
// Use at every boundary where a UUID leaves the staging package: audit
// writes, webhook payloads, SSE broadcasts, analytics events.
func MustNotBeProvisional(id uuid.UUID, where string) {
	if IsProvisional(id) {
		panic(fmt.Sprintf("staging: provisional id leaked into %s: %s", where, id))
	}
}
