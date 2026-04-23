package handler

// Puzzle mode names. Shared by handlers that validate the {mode} URL
// parameter and by the worker that translates it to the generator's k.
const (
	ModeStandard = "standard"
	ModeDouble   = "double"
)

// MarksPerUnitFromMode returns the generator's k parameter for a given mode.
// Standard puzzles have 1 mark per row/column/region; Double puzzles have 2.
// Callers must pre-validate mode against ModeStandard/ModeDouble.
func MarksPerUnitFromMode(mode string) int {
	if mode == ModeDouble {
		return 2
	}
	return 1
}
