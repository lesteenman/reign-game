package generator

import (
	"testing"
	"time"
)

// =============================================================================
// Consolidated Generator Benchmark Suite (GN-10)
// =============================================================================
//
// This file is the single source of truth for generator performance benchmarks.
// It covers the full matrix of viable pipeline x size x mode x concurrency
// combinations using sub-benchmarks for clean output.
//
// Naming convention:
//   BenchmarkPipeline/<Pipeline>/<Size>_<Mode>/<deducible|raw>/<cN>
//
// Run with:
//   go test -bench=BenchmarkPipeline -benchtime=3x -timeout=600s ./internal/generator/ -run='^$'
//   task bench:backend
// =============================================================================

// benchCase defines a single benchmark combination.
type benchCase struct {
	name           string
	pipeline       PipelineStrategy
	gridSize       int
	markersPerUnit int
	deducible      bool
	concurrency    int
	regionVariance float64
	timeout        time.Duration
}

// runBenchCase executes a single benchmark case with standard metric reporting.
func runBenchCase(b *testing.B, bc benchCase) {
	b.Helper()
	opts := GenerateOpts{
		Timeout:     bc.timeout,
		Deducible:   bc.deducible,
		Concurrency: bc.concurrency,
		RegionOpts: RegionOpts{
			Variance: bc.regionVariance,
			MinSize:  3,
		},
	}
	if bc.markersPerUnit >= 2 {
		opts.RegionOpts.MinSize = 4
	}

	for b.Loop() {
		start := time.Now()
		var err error
		var gridSize int
		if bc.concurrency > 1 {
			puzzle, genErr := GenerateConcurrent(bc.pipeline, bc.gridSize, bc.markersPerUnit, opts)
			err = genErr
			if puzzle != nil {
				gridSize = puzzle.GridSize
			}
		} else {
			puzzle, genErr := bc.pipeline.Generate(bc.gridSize, bc.markersPerUnit, opts)
			err = genErr
			if puzzle != nil {
				gridSize = puzzle.GridSize
			}
		}
		elapsed := time.Since(start)
		if err != nil {
			b.Fatalf("Generate(%d) failed: %v", bc.gridSize, err)
		}
		b.ReportMetric(elapsed.Seconds(), "sec/puzzle")
		b.ReportMetric(float64(gridSize), "gridSize")
	}
}

// BenchmarkPipeline runs the comprehensive pipeline benchmark matrix.
func BenchmarkPipeline(b *testing.B) {
	solver := NewPropagationSolver()
	regionsBFS := NewBFSRegionGenerator()
	regionsBFSDouble := NewBFSRegionGeneratorDouble()

	regionFirst := NewRegionFirstPipeline(solver)
	iterative := NewIterativeRefinementPipeline(solver, regionsBFS)
	iterativeDouble := NewIterativeRefinementPipeline(solver, regionsBFSDouble)

	cases := []benchCase{
		// -----------------------------------------------------------------
		// Standard mode (deducible=true): RegionFirst
		// -----------------------------------------------------------------
		{
			name: "RegionFirst/5x5_Standard/deducible/c1",
			pipeline: regionFirst, gridSize: 5, markersPerUnit: 1,
			deducible: true, concurrency: 1, timeout: 60 * time.Second,
		},
		{
			name: "RegionFirst/7x7_Standard/deducible/c1",
			pipeline: regionFirst, gridSize: 7, markersPerUnit: 1,
			deducible: true, concurrency: 1, timeout: 60 * time.Second,
		},
		{
			name: "RegionFirst/9x9_Standard/deducible/c1",
			pipeline: regionFirst, gridSize: 9, markersPerUnit: 1,
			deducible: true, concurrency: 1, timeout: 120 * time.Second,
		},
		{
			name: "RegionFirst/9x9_Standard/deducible/c4",
			pipeline: regionFirst, gridSize: 9, markersPerUnit: 1,
			deducible: true, concurrency: 4, timeout: 120 * time.Second,
		},

		// -----------------------------------------------------------------
		// Standard mode (deducible=true): Iterative
		// -----------------------------------------------------------------
		{
			name: "Iterative/5x5_Standard/deducible/c1",
			pipeline: iterative, gridSize: 5, markersPerUnit: 1,
			deducible: true, concurrency: 1, timeout: 60 * time.Second,
		},
		{
			name: "Iterative/7x7_Standard/deducible/c1",
			pipeline: iterative, gridSize: 7, markersPerUnit: 1,
			deducible: true, concurrency: 1, timeout: 60 * time.Second,
		},
		{
			name: "Iterative/9x9_Standard/deducible/c1",
			pipeline: iterative, gridSize: 9, markersPerUnit: 1,
			deducible: true, concurrency: 1, timeout: 120 * time.Second,
		},
		{
			name: "Iterative/9x9_Standard/deducible/c4",
			pipeline: iterative, gridSize: 9, markersPerUnit: 1,
			deducible: true, concurrency: 4, timeout: 120 * time.Second,
		},

		// -----------------------------------------------------------------
		// Standard mode (deducible=false): RegionFirst baseline
		// -----------------------------------------------------------------
		{
			name: "RegionFirst/9x9_Standard/raw/c1",
			pipeline: regionFirst, gridSize: 9, markersPerUnit: 1,
			deducible: false, concurrency: 1, timeout: 120 * time.Second,
		},

		// -----------------------------------------------------------------
		// Double Queens (deducible=false, 9x9): RegionFirst
		// -----------------------------------------------------------------
		{
			name: "RegionFirst/9x9_Double/raw/c1",
			pipeline: regionFirst, gridSize: 9, markersPerUnit: 2,
			deducible: false, concurrency: 1, timeout: 120 * time.Second,
		},
		{
			name: "RegionFirst/9x9_Double/raw/c4",
			pipeline: regionFirst, gridSize: 9, markersPerUnit: 2,
			deducible: false, concurrency: 4, timeout: 120 * time.Second,
		},

		// -----------------------------------------------------------------
		// Double Queens (deducible=false, 9x9): Iterative
		// -----------------------------------------------------------------
		{
			name: "Iterative/9x9_Double/raw/c1",
			pipeline: iterativeDouble, gridSize: 9, markersPerUnit: 2,
			deducible: false, concurrency: 1, timeout: 120 * time.Second,
		},
		{
			name: "Iterative/9x9_Double/raw/c4",
			pipeline: iterativeDouble, gridSize: 9, markersPerUnit: 2,
			deducible: false, concurrency: 4, timeout: 120 * time.Second,
		},

		// -----------------------------------------------------------------
		// Region variance impact (9x9 standard, deducible=true): Iterative
		// -----------------------------------------------------------------
		{
			name: "Iterative/9x9_Standard/deducible/variance0.00/c1",
			pipeline: iterative, gridSize: 9, markersPerUnit: 1,
			deducible: true, concurrency: 1, regionVariance: 0.0, timeout: 120 * time.Second,
		},
		{
			name: "Iterative/9x9_Standard/deducible/variance0.25/c1",
			pipeline: iterative, gridSize: 9, markersPerUnit: 1,
			deducible: true, concurrency: 1, regionVariance: 0.25, timeout: 120 * time.Second,
		},
	}

	for _, bc := range cases {
		b.Run(bc.name, func(b *testing.B) {
			runBenchCase(b, bc)
		})
	}
}

// =============================================================================
// BenchmarkCombo — targeted parameter combination benchmarks
// =============================================================================
//
// Run with:
//   go test -bench=BenchmarkCombo -benchtime=3x -timeout=300s ./internal/generator/ -run='^$'

// BenchmarkCombo benchmarks specific parameter combinations to find optimal defaults.
func BenchmarkCombo(b *testing.B) {
	propSolver := NewPropagationSolver()
	backtrackSolver := NewBacktrackSolver()

	bfsRegions := NewBFSRegionGenerator()
	bfsRegionsDouble := NewBFSRegionGeneratorDouble()
	wfcRegions := NewWFCRegionGenerator()

	iterPropBFS := NewIterativeRefinementPipeline(propSolver, bfsRegions)
	iterPropWFC := NewIterativeRefinementPipeline(propSolver, wfcRegions)
	iterBacktrackBFS := NewIterativeRefinementPipeline(backtrackSolver, bfsRegions)
	iterPropBFSDouble := NewIterativeRefinementPipeline(propSolver, bfsRegionsDouble)
	regionFirstProp := NewRegionFirstPipeline(propSolver)

	cases := []benchCase{
		// -----------------------------------------------------------------
		// BFS vs WFC regions (Iterative, Propagation, 9x9 Standard, deducible)
		// -----------------------------------------------------------------
		{
			name: "Iterative_Prop_BFS_9x9_ded",
			pipeline: iterPropBFS, gridSize: 9, markersPerUnit: 1,
			deducible: true, concurrency: 1, timeout: 120 * time.Second,
		},
		{
			name: "Iterative_Prop_WFC_9x9_ded",
			pipeline: iterPropWFC, gridSize: 9, markersPerUnit: 1,
			deducible: true, concurrency: 1, timeout: 120 * time.Second,
		},

		// -----------------------------------------------------------------
		// Solver comparison (Iterative, BFS, 9x9 Standard, deducible)
		// -----------------------------------------------------------------
		{
			name: "Iterative_Backtrack_BFS_9x9_ded",
			pipeline: iterBacktrackBFS, gridSize: 9, markersPerUnit: 1,
			deducible: true, concurrency: 1, timeout: 120 * time.Second,
		},

		// -----------------------------------------------------------------
		// Double Queens combinations (9x9, c=1)
		// -----------------------------------------------------------------
		{
			name: "Iterative_Prop_BFS_9x9DQ_ded",
			pipeline: iterPropBFSDouble, gridSize: 9, markersPerUnit: 2,
			deducible: true, concurrency: 1, timeout: 120 * time.Second,
		},
		{
			name: "Iterative_Prop_BFS_9x9DQ_raw",
			pipeline: iterPropBFSDouble, gridSize: 9, markersPerUnit: 2,
			deducible: false, concurrency: 1, timeout: 120 * time.Second,
		},
		{
			name: "RegionFirst_Prop_9x9DQ_ded",
			pipeline: regionFirstProp, gridSize: 9, markersPerUnit: 2,
			deducible: true, concurrency: 1, timeout: 120 * time.Second,
		},
		{
			name: "RegionFirst_Prop_9x9DQ_raw",
			pipeline: regionFirstProp, gridSize: 9, markersPerUnit: 2,
			deducible: false, concurrency: 1, timeout: 120 * time.Second,
		},
	}

	for _, bc := range cases {
		b.Run(bc.name, func(b *testing.B) {
			runBenchCase(b, bc)
		})
	}
}

// ---------------------------------------------------------------------------
// Solver comparison — BacktrackSolver vs PropagationSolver (5x5 only)
// ---------------------------------------------------------------------------

// BenchmarkSolver_Backtrack_CountSolutions_5x5 benchmarks BacktrackSolver's
// CountSolutions on the reference 5x5 region map.
func BenchmarkSolver_Backtrack_CountSolutions_5x5(b *testing.B) {
	solver := NewBacktrackSolver()
	for b.Loop() {
		count := solver.CountSolutions(validRegionMap5x5, 5, 1, 2)
		if count != 1 {
			b.Fatalf("expected 1 solution, got %d", count)
		}
	}
}

// BenchmarkSolver_Propagation_CountSolutions_5x5 benchmarks PropagationSolver's
// CountSolutions on the same reference 5x5 region map.
func BenchmarkSolver_Propagation_CountSolutions_5x5(b *testing.B) {
	solver := NewPropagationSolver()
	for b.Loop() {
		count := solver.CountSolutions(validRegionMap5x5, 5, 1, 2)
		if count != 1 {
			b.Fatalf("expected 1 solution, got %d", count)
		}
	}
}

// ---------------------------------------------------------------------------
// Legacy reference — BacktrackSolver full pipeline at 5x5
// ---------------------------------------------------------------------------

// BenchmarkPipeline_Legacy_Backtrack_5x5_Standard benchmarks the
// SolutionFirstPipeline with BacktrackSolver + BFS regions at 5x5.
func BenchmarkPipeline_Legacy_Backtrack_5x5_Standard(b *testing.B) {
	solver := NewBacktrackSolver()
	regions := NewBFSRegionGenerator()
	pipeline := NewSolutionFirstPipeline(solver, regions)
	opts := GenerateOpts{Timeout: 60 * time.Second}
	for b.Loop() {
		start := time.Now()
		puzzle, err := pipeline.Generate(5, 1, opts)
		elapsed := time.Since(start)
		if err != nil {
			b.Fatalf("Generate(5) failed: %v", err)
		}
		b.ReportMetric(elapsed.Seconds(), "sec/puzzle")
		b.ReportMetric(float64(puzzle.GridSize), "gridSize")
	}
}
