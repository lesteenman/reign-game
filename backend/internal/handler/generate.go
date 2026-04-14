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

const generateTimeout = 5 * time.Second

// GenerateHandler handles GET /puzzles/generate.
// Query params: size (int, required), mode (string, required).
// Phase 1: only size=5 and mode=standard are accepted.
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

	if size != 5 {
		writeError(w, http.StatusBadRequest, "invalid_params", "only size=5 is supported in Phase 1")
		return
	}

	// Validate mode parameter.
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		writeError(w, http.StatusBadRequest, "invalid_params", "mode parameter is required")
		return
	}

	if mode != "standard" {
		writeError(w, http.StatusBadRequest, "invalid_params", "only mode=standard is supported in Phase 1")
		return
	}

	// Generate puzzle.
	puzzle, err := generator.Generate(size, generateTimeout)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "generation_failed", fmt.Sprintf("puzzle generation failed: %v", err))
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
