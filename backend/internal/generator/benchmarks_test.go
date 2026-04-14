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
// It covers the full matrix of viable pipeline x size x mode combinations.
//
// Naming convention:
//   BenchmarkPipeline_<Pipeline>_<Size>_<Mode>[_<Variant>]
//
// Pipelines benchmarked:
//   - RegionFirst:      regions via BFS, then solve. Reliable across all sizes.
//   - Iterative:        iterative refinement with BFS regions. Reliable across all sizes.
//   - ConstraintAware:  constraint-guided region growth. Works at 5x5/7x7; times out at 9x9.
//
// Modes:
//   - Standard:  markersPerUnit=1 (sizes 5x5, 7x7, 9x9)
//   - Double:    markersPerUnit=2 (9x9 only, a.k.a. "Double Queens")
//
// Variants:
//   - Variance05: RegionOpts.Variance=0.5 (variable region sizes)
//
// Solver comparison:
//   - BacktrackSolver vs PropagationSolver for CountSolutions (5x5 only)
//
// Run with:
//   go test -bench=BenchmarkPipeline -benchtime=3x -timeout=600s ./internal/generator/ -run='^$'
//   task bench:backend
// =============================================================================

// benchConfig holds the parameters for a single pipeline benchmark case.
type benchConfig struct {
	pipeline       PipelineStrategy
	gridSize       int
	markersPerUnit int
	timeout        time.Duration
}

// runPipelineBench executes a pipeline benchmark with standard metric reporting.
func runPipelineBench(b *testing.B, cfg benchConfig) {
	b.Helper()
	opts := GenerateOpts{
		Timeout: cfg.timeout,
	}
	for b.Loop() {
		start := time.Now()
		puzzle, err := cfg.pipeline.Generate(cfg.gridSize, cfg.markersPerUnit, opts)
		elapsed := time.Since(start)
		if err != nil {
			b.Fatalf("Generate(%d) failed: %v", cfg.gridSize, err)
		}
		b.ReportMetric(elapsed.Seconds(), "sec/puzzle")
		b.ReportMetric(float64(puzzle.GridSize), "gridSize")
	}
}

// runPipelineBenchWithVariance executes a pipeline benchmark with region variance.
func runPipelineBenchWithVariance(b *testing.B, cfg benchConfig, variance float64) {
	b.Helper()
	opts := GenerateOpts{
		Timeout:    cfg.timeout,
		RegionOpts: RegionOpts{Variance: variance, MinSize: 3},
	}
	for b.Loop() {
		start := time.Now()
		puzzle, err := cfg.pipeline.Generate(cfg.gridSize, cfg.markersPerUnit, opts)
		elapsed := time.Since(start)
		if err != nil {
			b.Fatalf("Generate(%d) failed: %v", cfg.gridSize, err)
		}
		b.ReportMetric(elapsed.Seconds(), "sec/puzzle")
		b.ReportMetric(float64(puzzle.GridSize), "gridSize")
	}
}

// ---------------------------------------------------------------------------
// RegionFirst Pipeline — Standard mode (5x5, 7x7, 9x9)
// ---------------------------------------------------------------------------

// BenchmarkPipeline_RegionFirst_5x5_Standard benchmarks RegionFirst at the
// baseline 5x5 size. Fast and reliable.
func BenchmarkPipeline_RegionFirst_5x5_Standard(b *testing.B) {
	runPipelineBench(b, benchConfig{
		pipeline:       NewRegionFirstPipeline(NewPropagationSolver()),
		gridSize:       5,
		markersPerUnit: 1,
		timeout:        60 * time.Second,
	})
}

// BenchmarkPipeline_RegionFirst_7x7_Standard benchmarks RegionFirst at 7x7.
func BenchmarkPipeline_RegionFirst_7x7_Standard(b *testing.B) {
	runPipelineBench(b, benchConfig{
		pipeline:       NewRegionFirstPipeline(NewPropagationSolver()),
		gridSize:       7,
		markersPerUnit: 1,
		timeout:        60 * time.Second,
	})
}

// BenchmarkPipeline_RegionFirst_9x9_Standard benchmarks RegionFirst at the
// largest standard size. This is the primary production benchmark.
func BenchmarkPipeline_RegionFirst_9x9_Standard(b *testing.B) {
	runPipelineBench(b, benchConfig{
		pipeline:       NewRegionFirstPipeline(NewPropagationSolver()),
		gridSize:       9,
		markersPerUnit: 1,
		timeout:        120 * time.Second,
	})
}

// ---------------------------------------------------------------------------
// RegionFirst Pipeline — Double Queens mode (9x9)
// ---------------------------------------------------------------------------

// BenchmarkPipeline_RegionFirst_9x9_Double benchmarks RegionFirst for
// Double Queens (markersPerUnit=2) at 9x9.
func BenchmarkPipeline_RegionFirst_9x9_Double(b *testing.B) {
	runPipelineBench(b, benchConfig{
		pipeline:       NewRegionFirstPipeline(NewPropagationSolver()),
		gridSize:       9,
		markersPerUnit: 2,
		timeout:        120 * time.Second,
	})
}

// ---------------------------------------------------------------------------
// Iterative Pipeline — Standard mode (5x5, 7x7, 9x9)
// ---------------------------------------------------------------------------

// BenchmarkPipeline_Iterative_5x5_Standard benchmarks IterativeRefinement at 5x5.
func BenchmarkPipeline_Iterative_5x5_Standard(b *testing.B) {
	runPipelineBench(b, benchConfig{
		pipeline:       NewIterativeRefinementPipeline(NewPropagationSolver(), NewBFSRegionGenerator()),
		gridSize:       5,
		markersPerUnit: 1,
		timeout:        60 * time.Second,
	})
}

// BenchmarkPipeline_Iterative_7x7_Standard benchmarks IterativeRefinement at 7x7.
func BenchmarkPipeline_Iterative_7x7_Standard(b *testing.B) {
	runPipelineBench(b, benchConfig{
		pipeline:       NewIterativeRefinementPipeline(NewPropagationSolver(), NewBFSRegionGenerator()),
		gridSize:       7,
		markersPerUnit: 1,
		timeout:        60 * time.Second,
	})
}

// BenchmarkPipeline_Iterative_9x9_Standard benchmarks IterativeRefinement at 9x9.
func BenchmarkPipeline_Iterative_9x9_Standard(b *testing.B) {
	runPipelineBench(b, benchConfig{
		pipeline:       NewIterativeRefinementPipeline(NewPropagationSolver(), NewBFSRegionGenerator()),
		gridSize:       9,
		markersPerUnit: 1,
		timeout:        120 * time.Second,
	})
}

// ---------------------------------------------------------------------------
// Iterative Pipeline — Double Queens mode (9x9)
// ---------------------------------------------------------------------------

// BenchmarkPipeline_Iterative_9x9_Double benchmarks IterativeRefinement for
// Double Queens (markersPerUnit=2) at 9x9.
func BenchmarkPipeline_Iterative_9x9_Double(b *testing.B) {
	solver := NewPropagationSolver()
	regions := NewBFSRegionGeneratorDouble()
	pipeline := NewIterativeRefinementPipeline(solver, regions)
	opts := GenerateOpts{Timeout: 60 * time.Second, RegionOpts: RegionOpts{MinSize: 4}}
	for b.Loop() {
		start := time.Now()
		puzzle, err := pipeline.Generate(9, 2, opts)
		elapsed := time.Since(start)
		if err != nil {
			b.Fatalf("Generate(9) failed: %v", err)
		}
		b.ReportMetric(elapsed.Seconds(), "sec/puzzle")
		b.ReportMetric(float64(puzzle.GridSize), "gridSize")
	}
}

// ---------------------------------------------------------------------------
// ConstraintAware Pipeline — Standard mode (5x5, 7x7 only; 9x9 times out)
// ---------------------------------------------------------------------------

// BenchmarkPipeline_ConstraintAware_5x5_Standard benchmarks ConstraintAware at 5x5.
func BenchmarkPipeline_ConstraintAware_5x5_Standard(b *testing.B) {
	runPipelineBench(b, benchConfig{
		pipeline:       NewConstraintAwarePipeline(NewPropagationSolver()),
		gridSize:       5,
		markersPerUnit: 1,
		timeout:        60 * time.Second,
	})
}

// BenchmarkPipeline_ConstraintAware_7x7_Standard benchmarks ConstraintAware at 7x7.
// This is the largest size where ConstraintAware completes reliably.
func BenchmarkPipeline_ConstraintAware_7x7_Standard(b *testing.B) {
	runPipelineBench(b, benchConfig{
		pipeline:       NewConstraintAwarePipeline(NewPropagationSolver()),
		gridSize:       7,
		markersPerUnit: 1,
		timeout:        60 * time.Second,
	})
}

// NOTE: ConstraintAware at 9x9 is intentionally excluded — it consistently
// times out due to the exponential cost of constraint-guided region growth
// at that grid size.

// ---------------------------------------------------------------------------
// Region Variance benchmarks — RegionFirst + Iterative at 9x9
// ---------------------------------------------------------------------------
// These show the impact of variable region sizes (Variance=0.5) vs uniform
// (Variance=0.0, the default) on generation time at the largest standard size.

// BenchmarkPipeline_RegionFirst_9x9_Standard_Variance00 benchmarks RegionFirst
// at 9x9 with explicitly uniform region sizes (Variance=0.0).
func BenchmarkPipeline_RegionFirst_9x9_Standard_Variance00(b *testing.B) {
	runPipelineBenchWithVariance(b, benchConfig{
		pipeline:       NewRegionFirstPipeline(NewPropagationSolver()),
		gridSize:       9,
		markersPerUnit: 1,
		timeout:        120 * time.Second,
	}, 0.0)
}

// BenchmarkPipeline_RegionFirst_9x9_Standard_Variance05 benchmarks RegionFirst
// at 9x9 with variable region sizes (Variance=0.5).
func BenchmarkPipeline_RegionFirst_9x9_Standard_Variance05(b *testing.B) {
	runPipelineBenchWithVariance(b, benchConfig{
		pipeline:       NewRegionFirstPipeline(NewPropagationSolver()),
		gridSize:       9,
		markersPerUnit: 1,
		timeout:        120 * time.Second,
	}, 0.5)
}

// BenchmarkPipeline_Iterative_9x9_Standard_Variance00 benchmarks Iterative
// at 9x9 with explicitly uniform region sizes (Variance=0.0).
func BenchmarkPipeline_Iterative_9x9_Standard_Variance00(b *testing.B) {
	runPipelineBenchWithVariance(b, benchConfig{
		pipeline:       NewIterativeRefinementPipeline(NewPropagationSolver(), NewBFSRegionGenerator()),
		gridSize:       9,
		markersPerUnit: 1,
		timeout:        120 * time.Second,
	}, 0.0)
}

// BenchmarkPipeline_Iterative_9x9_Standard_Variance05 benchmarks Iterative
// at 9x9 with variable region sizes (Variance=0.5).
func BenchmarkPipeline_Iterative_9x9_Standard_Variance05(b *testing.B) {
	runPipelineBenchWithVariance(b, benchConfig{
		pipeline:       NewIterativeRefinementPipeline(NewPropagationSolver(), NewBFSRegionGenerator()),
		gridSize:       9,
		markersPerUnit: 1,
		timeout:        120 * time.Second,
	}, 0.5)
}

// ---------------------------------------------------------------------------
// Solver comparison — BacktrackSolver vs PropagationSolver (5x5 only)
// ---------------------------------------------------------------------------
// The BacktrackSolver is too slow for CountSolutions at 7x7+, so we only
// compare at 5x5 to document the improvement from constraint propagation.

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
// Kept as a historical reference point. The BacktrackSolver times out at
// 7x7+ in full pipeline mode, so only 5x5 is benchmarked here.

// BenchmarkPipeline_Legacy_Backtrack_5x5_Standard benchmarks the
// SolutionFirstPipeline with BacktrackSolver + BFS regions at 5x5.
// This is the pre-pipeline baseline for comparison.
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
