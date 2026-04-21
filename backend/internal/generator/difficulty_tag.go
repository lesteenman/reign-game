//go:build corpus

package generator

// difficultyTag returns a short filesystem-safe label for a Difficulty
// value. Used by the corpus generator to name puzzle fixtures in
// testdata/puzzles/ without depending on an external Stringer.
//
// Kept package-internal; if a public String() ever lands on Difficulty,
// drop this and use that.
func difficultyTag(d Difficulty) string {
	switch d {
	case Easy:
		return "easy"
	case Medium:
		return "medium"
	case Hard:
		return "hard"
	case Expert:
		return "expert"
	default:
		return "unknown"
	}
}
