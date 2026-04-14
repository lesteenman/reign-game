package generator

import (
	"testing"
	"time"
)

// BenchmarkGenerate5x5Standard benchmarks puzzle generation for a 5x5 grid.
// This is the baseline size — the backtrack solver handles it comfortably.
func BenchmarkGenerate5x5Standard(b *testing.B) {
	solver := NewBacktrackSolver()
	regions := NewBFSRegionGenerator()
	opts := GenerateOpts{Timeout: 60 * time.Second, Mode: "standard"}
	for b.Loop() {
		start := time.Now()
		puzzle, err := Generate(5, 1, solver, regions, opts)
		elapsed := time.Since(start)
		if err != nil {
			b.Fatalf("Generate(5) failed: %v", err)
		}
		b.ReportMetric(elapsed.Seconds(), "sec/puzzle")
		b.ReportMetric(float64(puzzle.GridSize), "gridSize")
	}
}

// BenchmarkGenerate7x7Standard benchmarks puzzle generation for a 7x7 grid.
// The backtrack solver is expected to struggle at this size — generation
// regularly takes 30-60+ seconds per puzzle.
func BenchmarkGenerate7x7Standard(b *testing.B) {
	solver := NewBacktrackSolver()
	regions := NewBFSRegionGenerator()
	opts := GenerateOpts{Timeout: 120 * time.Second, Mode: "standard"}
	for b.Loop() {
		start := time.Now()
		puzzle, err := Generate(7, 1, solver, regions, opts)
		elapsed := time.Since(start)
		if err != nil {
			b.Fatalf("Generate(7) failed: %v", err)
		}
		b.ReportMetric(elapsed.Seconds(), "sec/puzzle")
		b.ReportMetric(float64(puzzle.GridSize), "gridSize")
	}
}

// BenchmarkGenerate9x9Standard benchmarks puzzle generation for a 9x9 grid.
// The brute-force solver is expected to be very slow or time out at this size.
// This benchmark exists to quantify the gap that a constraint propagation
// solver (GN-03) needs to close.
func BenchmarkGenerate9x9Standard(b *testing.B) {
	solver := NewBacktrackSolver()
	regions := NewBFSRegionGenerator()
	opts := GenerateOpts{Timeout: 300 * time.Second, Mode: "standard"}
	for b.Loop() {
		start := time.Now()
		puzzle, err := Generate(9, 1, solver, regions, opts)
		elapsed := time.Since(start)
		if err != nil {
			b.Fatalf("Generate(9) failed: %v", err)
		}
		b.ReportMetric(elapsed.Seconds(), "sec/puzzle")
		b.ReportMetric(float64(puzzle.GridSize), "gridSize")
	}
}

// BenchmarkGeneratePropagation5x5 benchmarks full pipeline with PropagationSolver at 5x5.
func BenchmarkGeneratePropagation5x5(b *testing.B) {
	solver := NewPropagationSolver()
	regions := NewBFSRegionGenerator()
	opts := GenerateOpts{Timeout: 60 * time.Second, Mode: "standard"}
	for b.Loop() {
		start := time.Now()
		puzzle, err := Generate(5, 1, solver, regions, opts)
		elapsed := time.Since(start)
		if err != nil {
			b.Fatalf("Generate(5) failed: %v", err)
		}
		b.ReportMetric(elapsed.Seconds(), "sec/puzzle")
		b.ReportMetric(float64(puzzle.GridSize), "gridSize")
	}
}

// BenchmarkGeneratePropagation7x7 benchmarks full pipeline with PropagationSolver at 7x7.
func BenchmarkGeneratePropagation7x7(b *testing.B) {
	solver := NewPropagationSolver()
	regions := NewBFSRegionGenerator()
	opts := GenerateOpts{Timeout: 60 * time.Second, Mode: "standard"}
	for b.Loop() {
		start := time.Now()
		puzzle, err := Generate(7, 1, solver, regions, opts)
		elapsed := time.Since(start)
		if err != nil {
			b.Fatalf("Generate(7) failed: %v", err)
		}
		b.ReportMetric(elapsed.Seconds(), "sec/puzzle")
		b.ReportMetric(float64(puzzle.GridSize), "gridSize")
	}
}

// BenchmarkGeneratePropagation9x9 benchmarks full pipeline with PropagationSolver at 9x9.
func BenchmarkGeneratePropagation9x9(b *testing.B) {
	solver := NewPropagationSolver()
	regions := NewBFSRegionGenerator()
	opts := GenerateOpts{Timeout: 60 * time.Second, Mode: "standard"}
	for b.Loop() {
		start := time.Now()
		puzzle, err := Generate(9, 1, solver, regions, opts)
		elapsed := time.Since(start)
		if err != nil {
			b.Fatalf("Generate(9) failed: %v", err)
		}
		b.ReportMetric(elapsed.Seconds(), "sec/puzzle")
		b.ReportMetric(float64(puzzle.GridSize), "gridSize")
	}
}
