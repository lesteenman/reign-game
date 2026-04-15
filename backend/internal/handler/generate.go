package handler

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/generator"
	"github.com/eriksteenman/reign-game/backend/internal/model"
)

// generateTimeout returns the puzzle generation timeout based on grid size.
// Larger grids need more time for the backtracking solver.
func generateTimeout(size int) time.Duration {
	if size <= 5 {
		return 5 * time.Second
	}
	// API Gateway hard limit is 29s, so cap at 25s to leave margin for
	// response serialization and network overhead.
	if size <= 9 {
		return 25 * time.Second
	}
	return 25 * time.Second
}

// GenerateHandler handles GET /puzzles/generate.
// Query params:
//   - size: int 3-15 (required)
//   - mode: "standard" | "double" (required)
//   - deducible: "true" | "false" (optional, default "true" — only produce puzzles solvable without guessing)
//   - pipeline: "region-first" | "iterative" | "constraint-aware" (optional, default "iterative")
//   - solver: "backtrack" | "propagation" (optional, default "propagation")
//   - regions: "bfs" | "wfc" (optional, default "bfs")
//   - regionVariance: float 0.0-1.0 (optional, default 0.0)
//   - concurrency: int 1-8 (optional, default 1 — number of parallel generation goroutines)
func GenerateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse and validate all parameters.
	params, status, errCode, errMsg := ParseGenerateParams(r)
	if status != 0 {
		writeError(w, status, errCode, errMsg)
		return
	}

	// Build pipeline from parameters.
	pipeline := BuildPipeline(&params)

	// Build generation options.
	opts := generator.GenerateOpts{
		Timeout:   generateTimeout(params.Size),
		Ctx:       r.Context(),
		Deducible: params.Deducible,
		RegionOpts: generator.RegionOpts{
			Variance: params.RegionVariance,
			MinSize:  params.MinSize,
		},
	}

	// Generate the puzzle.
	var puzzle *model.Puzzle
	var err error
	if params.Concurrency > 1 {
		opts.Concurrency = params.Concurrency
		puzzle, err = generator.GenerateConcurrent(pipeline, params.Size, params.MarkersPerUnit, opts)
	} else {
		puzzle, err = pipeline.Generate(params.Size, params.MarkersPerUnit, opts)
	}
	if err != nil {
		log.Printf("puzzle generation failed: %v", err)
		writeError(w, http.StatusInternalServerError, "generation_failed", "Could not generate a puzzle. Please try again.")
		return
	}

	// Stamp puzzle mode (pipelines return Mode="", handler owns this).
	puzzle.Mode = params.Mode

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
