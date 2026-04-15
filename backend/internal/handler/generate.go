package handler

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/generator"
	"github.com/eriksteenman/reign-game/backend/internal/model"
)

// Pipeline strategy names.
const (
	PipelineRegionFirst     = "region-first"
	PipelineIterative       = "iterative"
	PipelineConstraintAware = "constraint-aware"
)

// Solver strategy names.
const (
	SolverBacktrack   = "backtrack"
	SolverPropagation = "propagation"
)

// Region strategy names.
const (
	RegionsBFS = "bfs"
	RegionsWFC = "wfc"
)

// Puzzle mode names.
const (
	ModeStandard = "standard"
	ModeDouble   = "double"
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

// generateParams holds all validated parameters for puzzle generation.
type generateParams struct {
	size           int
	mode           string
	pipeline       string
	solver         string
	regions        string
	regionVariance float64
	markersPerUnit int
	minSize        int
	deducible      bool
	concurrency    int
}

// parseGenerateParams validates and extracts all query parameters from a
// generate request. Returns an error message tuple (status, code, message)
// on validation failure.
func parseGenerateParams(r *http.Request) (params generateParams, status int, errCode, errMsg string) {
	var p generateParams

	// Validate size parameter.
	sizeStr := r.URL.Query().Get("size")
	if sizeStr == "" {
		return p, http.StatusBadRequest, "invalid_params", "size parameter is required"
	}

	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		return p, http.StatusBadRequest, "invalid_params", "size must be an integer"
	}

	if size < 3 || size > 15 {
		return p, http.StatusBadRequest, "invalid_params", "size must be between 3 and 15"
	}
	p.size = size

	// Validate mode parameter.
	p.mode = r.URL.Query().Get("mode")
	if p.mode == "" {
		return p, http.StatusBadRequest, "invalid_params", "mode parameter is required"
	}

	if p.mode != ModeStandard && p.mode != ModeDouble {
		return p, http.StatusBadRequest, "invalid_params", "mode must be 'standard' or 'double'"
	}

	// Validate deducible parameter (default true).
	deducibleStr := r.URL.Query().Get("deducible")
	switch deducibleStr {
	case "", "true":
		p.deducible = true
	case "false":
		p.deducible = false
	default:
		return p, http.StatusBadRequest, "invalid_params", "deducible must be 'true' or 'false'"
	}

	// Validate pipeline parameter.
	p.pipeline = r.URL.Query().Get("pipeline")
	if p.pipeline == "" {
		p.pipeline = PipelineIterative
	}
	if p.pipeline != PipelineRegionFirst && p.pipeline != PipelineIterative && p.pipeline != PipelineConstraintAware {
		return p, http.StatusBadRequest, "invalid_params", "pipeline must be 'region-first', 'iterative', or 'constraint-aware'"
	}

	// Validate solver parameter.
	p.solver = r.URL.Query().Get("solver")
	if p.solver == "" {
		p.solver = SolverPropagation
	}
	if p.solver != SolverBacktrack && p.solver != SolverPropagation {
		return p, http.StatusBadRequest, "invalid_params", "solver must be 'backtrack' or 'propagation'"
	}

	// Validate regions parameter.
	p.regions = r.URL.Query().Get("regions")
	if p.regions == "" {
		p.regions = RegionsBFS
	}
	if p.regions != RegionsBFS && p.regions != RegionsWFC {
		return p, http.StatusBadRequest, "invalid_params", "regions must be 'bfs' or 'wfc'"
	}

	// Validate regionVariance parameter.
	regionVarianceStr := r.URL.Query().Get("regionVariance")
	if regionVarianceStr != "" {
		p.regionVariance, err = strconv.ParseFloat(regionVarianceStr, 64)
		if err != nil {
			return p, http.StatusBadRequest, "invalid_params", "regionVariance must be a number"
		}
		if p.regionVariance < 0.0 || p.regionVariance > 1.0 || math.IsNaN(p.regionVariance) {
			return p, http.StatusBadRequest, "invalid_params", "regionVariance must be between 0.0 and 1.0"
		}
	}

	// Validate concurrency parameter (default 1).
	concurrencyStr := r.URL.Query().Get("concurrency")
	if concurrencyStr == "" {
		p.concurrency = 1
	} else {
		p.concurrency, err = strconv.Atoi(concurrencyStr)
		if err != nil {
			return p, http.StatusBadRequest, "invalid_params", "concurrency must be an integer"
		}
		if p.concurrency < 1 || p.concurrency > 8 {
			return p, http.StatusBadRequest, "invalid_params", "concurrency must be between 1 and 8"
		}
	}

	// Determine markersPerUnit and minSize based on mode.
	p.markersPerUnit = 1
	p.minSize = 3
	if p.mode == ModeDouble {
		p.markersPerUnit = 2
		p.minSize = 4
	}

	return p, 0, "", ""
}

// buildPipeline constructs the appropriate pipeline strategy from validated
// parameters.
func buildPipeline(p *generateParams) generator.PipelineStrategy {
	// Construct solver strategy.
	var solver generator.SolverStrategy
	switch p.solver {
	case SolverBacktrack:
		solver = generator.NewBacktrackSolver()
	case SolverPropagation:
		solver = generator.NewPropagationSolver()
	}

	// Construct region strategy (used by iterative pipeline).
	var regions generator.RegionStrategy
	switch p.regions {
	case RegionsBFS:
		if p.mode == ModeDouble {
			regions = generator.NewBFSRegionGeneratorDouble()
		} else {
			regions = generator.NewBFSRegionGenerator()
		}
	case RegionsWFC:
		if p.mode == ModeDouble {
			regions = generator.NewWFCRegionGeneratorDouble()
		} else {
			regions = generator.NewWFCRegionGenerator()
		}
	}

	// Construct pipeline strategy.
	switch p.pipeline {
	case PipelineIterative:
		return generator.NewIterativeRefinementPipeline(solver, regions)
	case PipelineConstraintAware:
		return generator.NewConstraintAwarePipeline(solver)
	default:
		return generator.NewRegionFirstPipeline(solver)
	}
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
	params, status, errCode, errMsg := parseGenerateParams(r)
	if status != 0 {
		writeError(w, status, errCode, errMsg)
		return
	}

	// Build pipeline from parameters.
	pipeline := buildPipeline(&params)

	// Build generation options.
	opts := generator.GenerateOpts{
		Timeout:   generateTimeout(params.size),
		Ctx:       r.Context(),
		Deducible: params.deducible,
		RegionOpts: generator.RegionOpts{
			Variance: params.regionVariance,
			MinSize:  params.minSize,
		},
	}

	// Generate the puzzle.
	var puzzle *model.Puzzle
	var err error
	if params.concurrency > 1 {
		opts.Concurrency = params.concurrency
		puzzle, err = generator.GenerateConcurrent(pipeline, params.size, params.markersPerUnit, opts)
	} else {
		puzzle, err = pipeline.Generate(params.size, params.markersPerUnit, opts)
	}
	if err != nil {
		log.Printf("puzzle generation failed: %v", err)
		writeError(w, http.StatusInternalServerError, "generation_failed", "Could not generate a puzzle. Please try again.")
		return
	}

	// Stamp puzzle mode (pipelines return Mode="", handler owns this).
	puzzle.Mode = params.mode

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
