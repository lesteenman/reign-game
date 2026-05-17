// Package mode defines the puzzle Mode value type — the two playable
// modes (Standard, Double) and the translation from Mode to the
// generator's marks-per-unit parameter (k).
//
// Both the HTTP edge (handler) and the SQS edge (worker) import from
// here. Putting these in handler/ would make worker→handler the
// dependency direction, which is wrong — worker is a peer, not a
// caller of, handler.
package mode

// Mode names. Used in URL paths, JSON payloads, and DDB partition keys.
const (
	Standard = "standard"
	Double   = "double"
)

// IsValid reports whether s is one of the two known modes.
func IsValid(s string) bool {
	return s == Standard || s == Double
}

// MarksPerUnit returns the generator's k parameter for the given mode:
// 1 for Standard, 2 for Double. Callers must pre-validate via IsValid.
func MarksPerUnit(s string) int {
	if s == Double {
		return 2
	}
	return 1
}
