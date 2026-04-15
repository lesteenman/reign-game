package handler

import (
	"math"
	"net/http"
	"strconv"

	"github.com/eriksteenman/reign-game/backend/internal/generator"
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

// GenerateParams holds all validated parameters for puzzle generation.
// Exported so other packages (e.g., worker) can construct pipelines from
// deserialized SQS messages.
type GenerateParams struct {
	Size           int
	Mode           string
	Pipeline       string
	Solver         string
	Regions        string
	RegionVariance float64
	MarkersPerUnit int
	MinSize        int
	Deducible      bool
	Concurrency    int
}

// ParseGenerateParams validates and extracts all query parameters from a
// generate request. Returns an error message tuple (status, code, message)
// on validation failure.
func ParseGenerateParams(r *http.Request) (params GenerateParams, status int, errCode, errMsg string) {
	var p GenerateParams

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
	p.Size = size

	// Validate mode parameter.
	p.Mode = r.URL.Query().Get("mode")
	if p.Mode == "" {
		return p, http.StatusBadRequest, "invalid_params", "mode parameter is required"
	}

	if p.Mode != ModeStandard && p.Mode != ModeDouble {
		return p, http.StatusBadRequest, "invalid_params", "mode must be 'standard' or 'double'"
	}

	// Validate deducible parameter (default true).
	deducibleStr := r.URL.Query().Get("deducible")
	switch deducibleStr {
	case "", "true":
		p.Deducible = true
	case "false":
		p.Deducible = false
	default:
		return p, http.StatusBadRequest, "invalid_params", "deducible must be 'true' or 'false'"
	}

	// Validate pipeline parameter.
	p.Pipeline = r.URL.Query().Get("pipeline")
	if p.Pipeline == "" {
		p.Pipeline = PipelineIterative
	}
	if p.Pipeline != PipelineRegionFirst && p.Pipeline != PipelineIterative && p.Pipeline != PipelineConstraintAware {
		return p, http.StatusBadRequest, "invalid_params", "pipeline must be 'region-first', 'iterative', or 'constraint-aware'"
	}

	// Validate solver parameter.
	p.Solver = r.URL.Query().Get("solver")
	if p.Solver == "" {
		p.Solver = SolverPropagation
	}
	if p.Solver != SolverBacktrack && p.Solver != SolverPropagation {
		return p, http.StatusBadRequest, "invalid_params", "solver must be 'backtrack' or 'propagation'"
	}

	// Validate regions parameter.
	p.Regions = r.URL.Query().Get("regions")
	if p.Regions == "" {
		p.Regions = RegionsBFS
	}
	if p.Regions != RegionsBFS && p.Regions != RegionsWFC {
		return p, http.StatusBadRequest, "invalid_params", "regions must be 'bfs' or 'wfc'"
	}

	// Validate regionVariance parameter.
	regionVarianceStr := r.URL.Query().Get("regionVariance")
	if regionVarianceStr != "" {
		p.RegionVariance, err = strconv.ParseFloat(regionVarianceStr, 64)
		if err != nil {
			return p, http.StatusBadRequest, "invalid_params", "regionVariance must be a number"
		}
		if p.RegionVariance < 0.0 || p.RegionVariance > 1.0 || math.IsNaN(p.RegionVariance) {
			return p, http.StatusBadRequest, "invalid_params", "regionVariance must be between 0.0 and 1.0"
		}
	}

	// Validate concurrency parameter (default 1).
	concurrencyStr := r.URL.Query().Get("concurrency")
	if concurrencyStr == "" {
		p.Concurrency = 1
	} else {
		p.Concurrency, err = strconv.Atoi(concurrencyStr)
		if err != nil {
			return p, http.StatusBadRequest, "invalid_params", "concurrency must be an integer"
		}
		if p.Concurrency < 1 || p.Concurrency > 8 {
			return p, http.StatusBadRequest, "invalid_params", "concurrency must be between 1 and 8"
		}
	}

	// Determine markersPerUnit and minSize based on mode.
	p.MarkersPerUnit = 1
	p.MinSize = 3
	if p.Mode == ModeDouble {
		p.MarkersPerUnit = 2
		p.MinSize = 4
	}

	return p, 0, "", ""
}

// BuildPipeline constructs the appropriate pipeline strategy from validated
// parameters.
func BuildPipeline(p *GenerateParams) generator.PipelineStrategy {
	// Construct solver strategy.
	var solver generator.SolverStrategy
	switch p.Solver {
	case SolverBacktrack:
		solver = generator.NewBacktrackSolver()
	case SolverPropagation:
		solver = generator.NewPropagationSolver()
	}

	// Construct region strategy (used by iterative pipeline).
	var regions generator.RegionStrategy
	switch p.Regions {
	case RegionsBFS:
		if p.Mode == ModeDouble {
			regions = generator.NewBFSRegionGeneratorDouble()
		} else {
			regions = generator.NewBFSRegionGenerator()
		}
	case RegionsWFC:
		if p.Mode == ModeDouble {
			regions = generator.NewWFCRegionGeneratorDouble()
		} else {
			regions = generator.NewWFCRegionGenerator()
		}
	}

	// Construct pipeline strategy.
	switch p.Pipeline {
	case PipelineIterative:
		return generator.NewIterativeRefinementPipeline(solver, regions)
	case PipelineConstraintAware:
		return generator.NewConstraintAwarePipeline(solver)
	default:
		return generator.NewRegionFirstPipeline(solver)
	}
}
