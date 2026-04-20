package generator

import (
	"context"
	"testing"
)

// TestGenerateProducesConnectedRegions: end-to-end invariant that every
// Generate output has 4-connected regions. Regression for the R-067-era
// bug where the mutator's fromRegionConnectedWithoutCell only verified
// seed reachability, allowing non-seed cells to be orphaned by a swap.
// Observed live on 9x9 Standard puzzles served from the dev pool.
//
// We exercise every supported (N, k) with enough samples that any
// non-vanishing disconnection rate will trip a failure.
func TestGenerateProducesConnectedRegions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		n, k, samples int
	}{
		{n: 6, k: 1, samples: 20},
		{n: 7, k: 1, samples: 20},
		{n: 8, k: 1, samples: 20},
		{n: 9, k: 1, samples: 20},
		{n: 9, k: 2, samples: 20},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(namef(tc.n, tc.k), func(t *testing.T) {
			t.Parallel()

			// Arrange
			g, err := New(tc.n, tc.k, WithSeed(int64(9000*tc.n+tc.k)))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			ctx := context.Background()

			for i := 0; i < tc.samples; i++ {
				// Act
				puzzle, err := g.Generate(ctx)
				if err != nil {
					t.Fatalf("sample %d: Generate: %v", i, err)
				}

				// Assert — every region 4-connected.
				if bad := firstDisconnectedRegion(puzzle.Regions, tc.n); bad >= 0 {
					t.Fatalf("sample %d (N=%d, k=%d): region %d is not 4-connected\nregion map:\n%s",
						i, tc.n, tc.k, bad, formatRegionMap(puzzle.Regions))
				}
			}
		})
	}
}

// firstDisconnectedRegion returns the id of a region that is not
// 4-connected, or -1 if every region is connected.
func firstDisconnectedRegion(regions [][]int, n int) int {
	if len(regions) != n {
		return -1
	}
	// Count cells per region.
	sizes := make(map[int]int)
	start := make(map[int][2]int)
	for r := range n {
		for c := range n {
			rid := regions[r][c]
			sizes[rid]++
			if _, ok := start[rid]; !ok {
				start[rid] = [2]int{r, c}
			}
		}
	}
	// BFS from the first cell of each region; compare visited count to
	// total cells.
	for rid, total := range sizes {
		s := start[rid]
		visited := map[[2]int]bool{{s[0], s[1]}: true}
		queue := [][2]int{{s[0], s[1]}}
		reached := 1
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, d := range [4][2]int{{-1, 0}, {0, -1}, {0, 1}, {1, 0}} {
				nr, nc := cur[0]+d[0], cur[1]+d[1]
				if nr < 0 || nr >= n || nc < 0 || nc >= n {
					continue
				}
				if visited[[2]int{nr, nc}] {
					continue
				}
				if regions[nr][nc] != rid {
					continue
				}
				visited[[2]int{nr, nc}] = true
				queue = append(queue, [2]int{nr, nc})
				reached++
			}
		}
		if reached != total {
			return rid
		}
	}
	return -1
}

func formatRegionMap(regions [][]int) string {
	var s string
	for _, row := range regions {
		for _, v := range row {
			s += " "
			if v < 10 {
				s += " "
			}
			s += itoa(v)
		}
		s += "\n"
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
