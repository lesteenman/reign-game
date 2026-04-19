package generator

import (
	"testing"
)

func TestConvertRegionsToSlicesShape(t *testing.T) {
	t.Parallel()

	// Arrange
	const n = 5
	var src [nMax][nMax]int8
	for r := range nMax {
		for c := range nMax {
			src[r][c] = -1 // should NOT be copied for r >= n or c >= n
		}
	}
	for r := range n {
		for c := range n {
			src[r][c] = int8((r + c) % n)
		}
	}

	// Act
	out := convertRegionsToSlices(&src, n)

	// Assert
	if len(out) != n {
		t.Fatalf("len(out) = %d, want %d", len(out), n)
	}
	for r := range n {
		if len(out[r]) != n {
			t.Errorf("len(out[%d]) = %d, want %d", r, len(out[r]), n)
		}
		for c := range n {
			if out[r][c] != (r+c)%n {
				t.Errorf("out[%d][%d] = %d, want %d", r, c, out[r][c], (r+c)%n)
			}
		}
	}
}

func TestConvertRegionsToSlicesIDRange(t *testing.T) {
	t.Parallel()

	// Arrange
	const n = 9
	var src [nMax][nMax]int8
	for r := range n {
		for c := range n {
			// Assign each cell to its row's region.
			src[r][c] = int8(r)
		}
	}

	// Act
	out := convertRegionsToSlices(&src, n)

	// Assert — all values in [0, n); exactly n distinct IDs.
	seen := make(map[int]bool)
	for r := range n {
		for c := range n {
			v := out[r][c]
			if v < 0 || v >= n {
				t.Errorf("out[%d][%d] = %d, expected [0, %d)", r, c, v, n)
			}
			seen[v] = true
		}
	}
	if len(seen) != n {
		t.Errorf("expected %d distinct region ids, got %d: %v", n, len(seen), seen)
	}
}

func TestConvertRegionsToSlicesPanicsOnOutOfRange(t *testing.T) {
	t.Parallel()

	// Arrange
	const n = 3
	var src [nMax][nMax]int8
	src[0][0] = -1

	// Act / Assert
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic on out-of-range region id, got none")
		}
	}()
	_ = convertRegionsToSlices(&src, n)
}
