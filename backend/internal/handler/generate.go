package handler

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/generator"
)

// generateTimeout returns the puzzle generation timeout based on grid size.
// Larger grids need more time for the backtracking solver.
func generateTimeout(size int) time.Duration {
	if size <= 5 {
		return 5 * time.Second
	}
	if size <= 9 {
		return 30 * time.Second
	}
	return 60 * time.Second
}

// GenerateHandler handles GET /puzzles/generate.
// Query params: size (int 3-15, required), mode (string, required).
// Currently only mode=standard is accepted.
func GenerateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Validate size parameter.
	sizeStr := r.URL.Query().Get("size")
	if sizeStr == "" {
		writeError(w, http.StatusBadRequest, "invalid_params", "size parameter is required")
		return
	}

	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_params", "size must be an integer")
		return
	}

	if size < 3 || size > 15 {
		writeError(w, http.StatusBadRequest, "invalid_params", "size must be between 3 and 15")
		return
	}

	// Validate mode parameter.
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		writeError(w, http.StatusBadRequest, "invalid_params", "mode parameter is required")
		return
	}

	if mode != "standard" {
		writeError(w, http.StatusBadRequest, "invalid_params", "only mode=standard is currently supported")
		return
	}

	// Generate puzzle using default pipeline.
	pipeline := generator.NewDefaultPipeline()
	opts := generator.GenerateOpts{
		Timeout: generateTimeout(size),
		Mode:    mode,
	}
	puzzle, err := pipeline.Generate(size, 1, opts)
	if err != nil {
		log.Printf("puzzle generation failed: %v", err)
		writeError(w, http.StatusInternalServerError, "generation_failed", "Could not generate a puzzle. Please try again.")
		return
	}

	// Assign a UUID v4.
	puzzle.ID, err = newUUIDv4()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "generation_failed", "failed to generate puzzle ID")
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(puzzle); err != nil {
		log.Printf("generate handler write failed: %v", err)
	}
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

// writeError sends a JSON error response.
func writeError(w http.ResponseWriter, status int, errCode, message string) {
	w.WriteHeader(status)
	body := map[string]string{"error": errCode, "message": message}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("error writing error response: %v", err)
	}
}
