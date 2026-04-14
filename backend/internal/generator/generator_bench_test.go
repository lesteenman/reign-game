package generator

import (
	"testing"
	"time"
)

// BenchmarkGenerate5x5Standard benchmarks puzzle generation for a 5x5 grid.
// This is the baseline size — the backtrack solver handles it comfortably.
func BenchmarkGenerate5x5Standard(b *testing.B) {
	for b.Loop() {
		start := time.Now()
		puzzle, err := Generate(5, 60*time.Second)
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
	for b.Loop() {
		start := time.Now()
		puzzle, err := Generate(7, 120*time.Second)
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
	for b.Loop() {
		start := time.Now()
		puzzle, err := Generate(9, 300*time.Second)
		elapsed := time.Since(start)
		if err != nil {
			b.Fatalf("Generate(9) failed: %v", err)
		}
		b.ReportMetric(elapsed.Seconds(), "sec/puzzle")
		b.ReportMetric(float64(puzzle.GridSize), "gridSize")
	}
}
