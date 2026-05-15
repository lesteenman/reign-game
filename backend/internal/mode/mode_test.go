package mode

import "testing"

func TestIsValid(t *testing.T) {
	// Arrange + Act + Assert
	tests := []struct {
		mode string
		want bool
	}{
		{Standard, true},
		{Double, true},
		{"", false},
		{"standard ", false},
		{"STANDARD", false},
		{"triple", false},
	}
	for _, tt := range tests {
		if got := IsValid(tt.mode); got != tt.want {
			t.Errorf("IsValid(%q) = %v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestMarksPerUnit(t *testing.T) {
	// Arrange + Act + Assert
	tests := []struct {
		mode string
		want int
	}{
		{Standard, 1},
		{Double, 2},
		// MarksPerUnit's contract is "callers pre-validate"; on unknown
		// input it falls back to Standard's 1. Documented behavior.
		{"unknown", 1},
		{"", 1},
	}
	for _, tt := range tests {
		if got := MarksPerUnit(tt.mode); got != tt.want {
			t.Errorf("MarksPerUnit(%q) = %d, want %d", tt.mode, got, tt.want)
		}
	}
}
