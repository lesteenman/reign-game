package handler

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/eriksteenman/reign-game/backend/internal/generator"
	"github.com/eriksteenman/reign-game/backend/internal/httperr"
	"github.com/eriksteenman/reign-game/backend/internal/model"
)

// GenerateHandler handles GET /api/puzzles/generate. The debug endpoint
// generates one puzzle inline and returns it without persisting to the pool.
//
// Query params:
//   - size: int 3..15 (required; generator also rejects below NMin)
//   - mode: "standard" | "double" (required)
//
// Other query params (pipeline, solver, regions, regionVariance, concurrency,
// deducible) from the legacy strategy matrix are silently ignored so
// bookmarked URLs continue to work during and after the Phase 5 cutover.
func GenerateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	size, mode, status, errCode, errMsg := parseSizeMode(r)
	if status != 0 {
		httperr.WriteError(w, status, errCode, errMsg)
		return
	}

	g, err := generator.New(size, MarksPerUnitFromMode(mode))
	if err != nil {
		log.Printf("generator construction failed: %v", err)
		httperr.WriteError(w, http.StatusBadRequest, "invalid_params", err.Error())
		return
	}

	pz, err := g.Generate(r.Context())
	if err != nil {
		if errors.Is(err, r.Context().Err()) {
			// Context canceled — client gave up. Nothing to return.
			return
		}
		log.Printf("puzzle generation failed (size=%d, mode=%s): %v", size, mode, err)
		httperr.WriteError(w, http.StatusInternalServerError, "generation_failed", "Could not generate a puzzle. Please try again.")
		return
	}

	puzzleID, err := newUUIDv4()
	if err != nil {
		httperr.WriteError(w, http.StatusInternalServerError, "generation_failed", "failed to generate puzzle ID")
		return
	}

	solution := make([][]bool, pz.N)
	for i := range solution {
		solution[i] = make([]bool, pz.N)
	}
	for _, m := range pz.Solution {
		solution[m.Row][m.Col] = true
	}

	puzzle := &model.Puzzle{
		ID:        puzzleID,
		GridSize:  pz.N,
		Mode:      mode,
		RegionMap: pz.Regions,
		Solution:  solution,
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(puzzle); err != nil {
		log.Printf("generate handler write failed: %v", err)
	}
}

// parseSizeMode validates the shared size+mode params used by
// /api/puzzles/generate, /api/puzzles/next, and /api/puzzles/{id}/status.
// Returns (0, "", "") on success.
func parseSizeMode(r *http.Request) (size int, mode string, status int, errCode, errMsg string) {
	sizeStr := r.URL.Query().Get("size")
	if sizeStr == "" {
		return 0, "", http.StatusBadRequest, "invalid_params", "size parameter is required"
	}
	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		return 0, "", http.StatusBadRequest, "invalid_params", "size must be an integer"
	}
	if size < 3 || size > 15 {
		return 0, "", http.StatusBadRequest, "invalid_params", "size must be between 3 and 15"
	}

	mode = r.URL.Query().Get("mode")
	if mode == "" {
		return 0, "", http.StatusBadRequest, "invalid_params", "mode parameter is required"
	}
	if mode != ModeStandard && mode != ModeDouble {
		return 0, "", http.StatusBadRequest, "invalid_params", "mode must be 'standard' or 'double'"
	}

	return size, mode, 0, "", ""
}

// newUUIDv4 generates a UUID v4 string using crypto/rand.
func newUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}

	// Set version 4 (bits 12-15 of time_hi_and_version).
	b[6] = (b[6] & 0x0f) | 0x40
	// Set variant bits (bits 6-7 of clk_seq_hi_res).
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
