package uuid_test

import (
	"regexp"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/uuid"
)

// canonicalUUID matches the 8-4-4-4-12 hex pattern.
var canonicalUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func TestNewV4(t *testing.T) {
	const iterations = 100

	for i := range iterations {
		// Arrange
		// (nothing to arrange — NewV4 takes no input)

		// Act
		got, err := uuid.NewV4()

		// Assert
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if !canonicalUUID.MatchString(got) {
			t.Fatalf("iteration %d: %q does not match 8-4-4-4-12 hex pattern", i, got)
		}

		// Version nibble: character at index 14 must be '4'.
		if got[14] != '4' {
			t.Fatalf("iteration %d: version nibble = %c, want 4 (full UUID: %s)", i, got[14], got)
		}

		// Variant high bits: first hex digit of the 4th group must be 8, 9, a, or b.
		variantChar := got[19]
		switch variantChar {
		case '8', '9', 'a', 'b':
			// correct
		default:
			t.Fatalf("iteration %d: variant char = %c, want one of 8/9/a/b (full UUID: %s)", i, variantChar, got)
		}
	}
}

func TestNewV4_NonZeroEntropy(t *testing.T) {
	// Arrange
	const iterations = 100
	seen := make(map[string]struct{}, iterations)

	// Act + Assert
	for i := range iterations {
		v, err := uuid.NewV4()
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if _, dup := seen[v]; dup {
			t.Fatalf("duplicate UUID generated at iteration %d: %s", i, v)
		}
		seen[v] = struct{}{}
	}
}
