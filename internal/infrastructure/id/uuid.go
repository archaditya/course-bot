// Package id provides the production IDGenerator backed by crypto/rand UUIDs.
// Everything above this package (application/, interfaces/) imports only the
// domain interface provider.IDGenerator — not this package.
package id

import (
	"crypto/rand"
	"fmt"

	"archadilm/internal/domain/provider"
)

// UUIDGenerator generates RFC 4122 UUID v4 identifiers using crypto/rand.
// It is stateless and safe for concurrent use.
type UUIDGenerator struct{}

// Verify compile-time that UUIDGenerator satisfies the interface.
var _ provider.IDGenerator = UUIDGenerator{}

// New returns a new random UUID v4 string formatted as:
//   xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
func (UUIDGenerator) New() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is unrecoverable — panic is correct here.
		// In practice this only happens on OS-level entropy exhaustion.
		panic(fmt.Errorf("uuid: crypto/rand failed: %w", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:],
	)
}
