package generator

import "fmt"

// convertRegionsToSlices copies the first n rows and cols of a
// [nMax][nMax]int8 region map into a freshly allocated [][]int of shape
// [N][N]. Region IDs are normalized to int (they are already in [0, n) by
// grower contract — this function asserts that).
//
// This is the only unavoidable allocation in the public output
// (input-spec §4.5); it runs once per successful Generate call. The src
// array is passed by pointer to avoid a 256-byte value copy.
func convertRegionsToSlices(src *[nMax][nMax]int8, n int) [][]int {
	out := make([][]int, n)
	for r := range n {
		out[r] = make([]int, n)
		for c := range n {
			v := int(src[r][c])
			if v < 0 || v >= n {
				// grower's contract is "all cells in [0, N)"; this is a
				// programmer bug, not a data condition — panic loudly.
				panic(fmt.Sprintf(
					"convertRegionsToSlices: out-of-range region id at (%d, %d) = %d, expected [0, %d)",
					r, c, v, n))
			}
			out[r][c] = v
		}
	}
	return out
}
