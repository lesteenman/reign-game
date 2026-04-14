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
// Query params:
//   - size: int 3-15 (required)
//   - mode: "standard" | "double" (required)
//   - pipeline: "region-first" | "iterative" | "constraint-aware" (optional, default "region-first")
//   - solver: "backtrack" | "propagation" (optional, default "propagation")
//   - regions: "bfs" | "wfc" (optional, default "bfs")
//   - regionVariance: float 0.0-1.0 (optional, default 0.0)
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

	if mode != "standard" && mode != "double" {
		writeError(w, http.StatusBadRequest, "invalid_params", "mode must be 'standard' or 'double'")
		return
	}

	// Validate pipeline parameter.
	pipelineStr := r.URL.Query().Get("pipeline")
	if pipelineStr == "" {
		pipelineStr = "region-first"
	}
	if pipelineStr != "region-first" && pipelineStr != "iterative" && pipelineStr != "constraint-aware" {
		writeError(w, http.StatusBadRequest, "invalid_params", "pipeline must be 'region-first', 'iterative', or 'constraint-aware'")
		return
	}

	// Validate solver parameter.
	solverStr := r.URL.Query().Get("solver")
	if solverStr == "" {
		solverStr = "propagation"
	}
	if solverStr != "backtrack" && solverStr != "propagation" {
		writeError(w, http.StatusBadRequest, "invalid_params", "solver must be 'backtrack' or 'propagation'")
		return
	}

	// Validate regions parameter.
	regionsStr := r.URL.Query().Get("regions")
	if regionsStr == "" {
		regionsStr = "bfs"
	}
	if regionsStr != "bfs" && regionsStr != "wfc" {
		writeError(w, http.StatusBadRequest, "invalid_params", "regions must be 'bfs' or 'wfc'")
		return
	}

	// Validate regionVariance parameter.
	var regionVariance float64
	regionVarianceStr := r.URL.Query().Get("regionVariance")
	if regionVarianceStr != "" {
		regionVariance, err = strconv.ParseFloat(regionVarianceStr, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_params", "regionVariance must be a number")
			return
		}
		if regionVariance < 0.0 || regionVariance > 1.0 {
			writeError(w, http.StatusBadRequest, "invalid_params", "regionVariance must be between 0.0 and 1.0")
			return
		}
	}

	// Determine markersPerUnit and minSize based on mode.
	markersPerUnit := 1
	minSize := 3
	if mode == "double" {
		markersPerUnit = 2
		minSize = 4
	}

	// Construct solver strategy.
	var solver generator.SolverStrategy
	switch solverStr {
	case "backtrack":
		solver = generator.NewBacktrackSolver()
	case "propagation":
		solver = generator.NewPropagationSolver()
	}

	// Construct region strategy (used by iterative pipeline).
	var regions generator.RegionStrategy
	switch regionsStr {
	case "bfs":
		if mode == "double" {
			regions = generator.NewBFSRegionGeneratorDouble()
		} else {
			regions = generator.NewBFSRegionGenerator()
		}
	case "wfc":
		if mode == "double" {
			regions = generator.NewWFCRegionGeneratorDouble()
		} else {
			regions = generator.NewWFCRegionGenerator()
		}
	}

	// Construct pipeline strategy.
	var pipeline generator.PipelineStrategy
	switch pipelineStr {
	case "region-first":
		pipeline = generator.NewRegionFirstPipeline(solver)
	case "iterative":
		pipeline = generator.NewIterativeRefinementPipeline(solver, regions)
	case "constraint-aware":
		pipeline = generator.NewConstraintAwarePipeline(solver)
	}

	// Build generation options.
	opts := generator.GenerateOpts{
		Timeout: generateTimeout(size),
		Mode:    mode,
		RegionOpts: generator.RegionOpts{
			Variance: regionVariance,
			MinSize:  minSize,
		},
	}

	// Generate the puzzle.
	puzzle, err := pipeline.Generate(size, markersPerUnit, opts)
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
