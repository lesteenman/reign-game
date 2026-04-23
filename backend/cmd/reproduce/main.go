// Command reproduce regenerates a single puzzle from an explicit seed
// and prints its region map, solution, and invariant check results.
// Used to debug a specific production puzzle that the pool worker
// generated with a recorded seed (stored on PuzzleRecord.Seed in
// DynamoDB; also returned in the /api/puzzles/next response metadata).
//
// Usage:
//
//	go run ./cmd/reproduce --seed=1234567890 --n=9 --k=1
//	task reproduce -- --seed=1234567890 --n=9 --k=1
//
// Exit codes:
//
//	0  success — all invariants hold.
//	1  runtime failure — Generate errored or exhausted attempts.
//	2  usage — required flag missing.
//	3  invariant violation — a region is below the min-size floor.
//
// A CI caller that wants to gate only on invariant failures (not
// runtime) checks specifically for exit 3.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/eriksteenman/reign-game/backend/internal/generator"
)

func main() {
	var (
		seed        = flag.Int64("seed", 0, "RNG seed (from PuzzleRecord.Seed)")
		n           = flag.Int("n", 0, "grid size N (required)")
		k           = flag.Int("k", 0, "marks per unit: 1 (Standard) or 2 (Double)")
		maxAttempts = flag.Int("max-attempts", 20, "WithMaxAttempts override")
	)
	flag.Parse()

	if *seed == 0 || *n == 0 || *k == 0 {
		fmt.Fprintln(os.Stderr, "usage: reproduce --seed=<int64> --n=<N> --k=<1|2> [--max-attempts=N]")
		os.Exit(2)
	}

	g, err := generator.New(*n, *k,
		generator.WithSeed(*seed),
		generator.WithMaxAttempts(*maxAttempts),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "constructing generator: %v\n", err)
		os.Exit(1)
	}

	p, err := g.Generate(context.Background())
	if err != nil {
		if errors.Is(err, generator.ErrMaxAttemptsExhausted) {
			fmt.Fprintln(os.Stderr, "generator exhausted its attempt budget — try a larger --max-attempts")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Generate: %v\n", err)
		os.Exit(1)
	}

	// Print the region map as a grid.
	fmt.Printf("N=%d k=%d seed=%d difficulty=%v\n\n", *n, *k, *seed, p.Difficulty)
	fmt.Println("region map:")
	for _, row := range p.Regions {
		for _, gid := range row {
			fmt.Printf("%2d ", gid)
		}
		fmt.Println()
	}

	// Per-region size check.
	sizes := make([]int, *n)
	for _, row := range p.Regions {
		for _, gid := range row {
			sizes[gid]++
		}
	}
	fmt.Println("\nregion sizes:")
	violations := 0
	for gid, sz := range sizes {
		marker := ""
		if sz < 3 {
			marker = "  <-- violates min-size (should be >= 3)"
			violations++
		}
		fmt.Printf("  region %d: %d cells%s\n", gid, sz, marker)
	}

	fmt.Printf("\nsolution marks: %d (expected N*k = %d)\n", len(p.Solution), *n*(*k))

	if violations > 0 {
		fmt.Fprintf(os.Stderr, "\nFAIL: %d region(s) below min-size\n", violations)
		os.Exit(3)
	}
	fmt.Println("\nOK — all invariants hold.")
}
