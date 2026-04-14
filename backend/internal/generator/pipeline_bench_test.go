package generator

import (
	"testing"
	"time"
)

// benchmarkPipeline runs a pipeline benchmark, reporting time per puzzle.
func benchmarkPipeline(b *testing.B, pipeline PipelineStrategy, gridSize int, timeout time.Duration) {
	b.Helper()
	opts := GenerateOpts{Timeout: timeout, Mode: "standard"}
	for b.Loop() {
		start := time.Now()
		puzzle, err := pipeline.Generate(gridSize, 1, opts)
		elapsed := time.Since(start)
		if err != nil {
			b.Fatalf("Generate(%d) failed: %v", gridSize, err)
		}
		b.ReportMetric(elapsed.Seconds(), "sec/puzzle")
		b.ReportMetric(float64(puzzle.GridSize), "gridSize")
	}
}

// --- SolutionFirstPipeline benchmarks ---

func BenchmarkPipelineSolutionFirst_5x5(b *testing.B) {
	pipeline := NewSolutionFirstPipeline(NewPropagationSolver(), NewBFSRegionGenerator())
	benchmarkPipeline(b, pipeline, 5, 60*time.Second)
}

func BenchmarkPipelineSolutionFirst_7x7(b *testing.B) {
	pipeline := NewSolutionFirstPipeline(NewPropagationSolver(), NewBFSRegionGenerator())
	benchmarkPipeline(b, pipeline, 7, 60*time.Second)
}

func BenchmarkPipelineSolutionFirst_9x9(b *testing.B) {
	pipeline := NewSolutionFirstPipeline(NewPropagationSolver(), NewBFSRegionGenerator())
	benchmarkPipeline(b, pipeline, 9, 60*time.Second)
}

// --- RegionFirstPipeline benchmarks ---

func BenchmarkPipelineRegionFirst_5x5(b *testing.B) {
	pipeline := NewRegionFirstPipeline(NewPropagationSolver())
	benchmarkPipeline(b, pipeline, 5, 60*time.Second)
}

func BenchmarkPipelineRegionFirst_7x7(b *testing.B) {
	pipeline := NewRegionFirstPipeline(NewPropagationSolver())
	benchmarkPipeline(b, pipeline, 7, 60*time.Second)
}

func BenchmarkPipelineRegionFirst_9x9(b *testing.B) {
	pipeline := NewRegionFirstPipeline(NewPropagationSolver())
	benchmarkPipeline(b, pipeline, 9, 60*time.Second)
}

// --- IterativeRefinementPipeline benchmarks ---

func BenchmarkPipelineIterative_5x5(b *testing.B) {
	pipeline := NewIterativeRefinementPipeline(NewPropagationSolver(), NewBFSRegionGenerator())
	benchmarkPipeline(b, pipeline, 5, 60*time.Second)
}

func BenchmarkPipelineIterative_7x7(b *testing.B) {
	pipeline := NewIterativeRefinementPipeline(NewPropagationSolver(), NewBFSRegionGenerator())
	benchmarkPipeline(b, pipeline, 7, 60*time.Second)
}

func BenchmarkPipelineIterative_9x9(b *testing.B) {
	pipeline := NewIterativeRefinementPipeline(NewPropagationSolver(), NewBFSRegionGenerator())
	benchmarkPipeline(b, pipeline, 9, 60*time.Second)
}

// --- ConstraintAwarePipeline benchmarks ---

func BenchmarkPipelineConstraintAware_5x5(b *testing.B) {
	pipeline := NewConstraintAwarePipeline(NewPropagationSolver())
	benchmarkPipeline(b, pipeline, 5, 60*time.Second)
}

func BenchmarkPipelineConstraintAware_7x7(b *testing.B) {
	pipeline := NewConstraintAwarePipeline(NewPropagationSolver())
	benchmarkPipeline(b, pipeline, 7, 60*time.Second)
}

func BenchmarkPipelineConstraintAware_9x9(b *testing.B) {
	pipeline := NewConstraintAwarePipeline(NewPropagationSolver())
	benchmarkPipeline(b, pipeline, 9, 60*time.Second)
}
