// Package uuid produces RFC 4122 §4.4 version-4 UUIDs from crypto/rand.
// Centralised so callers in cmd/ and internal/handler/ don't drift on
// the bit-set convention.
package uuid

import (
	"crypto/rand"
	"fmt"
)

// NewV4 returns a freshly-generated UUID v4 string.
func NewV4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
