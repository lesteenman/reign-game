package generator

import (
	"context"
	"errors"
	"fmt"
)

// NMin is the interim package-level floor for the grid size N.
//
// R-063's feasibility probe may raise this value but must never lower it below
// 5 (proposal AC-10). The constant is informational at the R-062 scaffold
// slice: New does not yet enforce n >= NMin — only the absolute mask-width
// bounds are checked here.
const NMin = 5

// nMax is the bitmask-width ceiling. The solver uses uint16 masks, so n > 16
// is unrepresentable.
const nMax = 16

// defaultMaxAttempts is the fallback attempt cap when a caller does not pass
// WithMaxAttempts. Tunable via the same option and, at the consumer layer,
// via ConfigRecord.MaxAttempts / GenerationRequest.MaxAttempts.
const defaultMaxAttempts = 20

// defaultMaxMutations is the default swap-mutation budget per attempt.
// Locked decision #9; adjustable via WithMaxMutations.
const defaultMaxMutations = 50

// Package-level typed errors. Consumers use errors.Is against these sentinels.
var (
	// ErrNOutOfRange is returned by New when n is outside [1, 16].
	ErrNOutOfRange = errors.New("generator: n out of range (expected 1..16)")

	// ErrKUnsupported is returned by New when marksPerUnit is not in {1, 2}.
	ErrKUnsupported = errors.New("generator: marksPerUnit unsupported (expected 1 or 2)")
)

// Difficulty buckets derived from the rule trace (design.md §8). Zero is the
// unset / invalid sentinel so an uninitialized value is never mistaken for a
// valid tier.
type Difficulty int

const (
	// DifficultyUnknown is the zero value and represents an unset tier.
	DifficultyUnknown Difficulty = iota
	// Easy puzzles require only Tier-1 rules.
	Easy
	// Medium puzzles require up to Tier-2 rules.
	Medium
	// Hard puzzles require up to Tier-3 rules.
	Hard
	// Expert puzzles require Tier-4 rules.
	Expert
)

// Mark is a zero-indexed cell coordinate.
type Mark struct {
	Row int `json:"r"`
	Col int `json:"c"`
}

// Metrics captures the classifier's view of a generated puzzle.
type Metrics struct {
	MaxTier    int   `json:"max_tier"`
	TierCounts []int `json:"tier_counts"`
	TraceLen   int   `json:"trace_len"`
}

// Puzzle is the generator's output shape. Storage types are built from it by
// the worker (see backend/internal/worker for the translation boundary).
type Puzzle struct {
	N            int        `json:"n"`
	MarksPerUnit int        `json:"marks_per_unit"`
	Regions      [][]int    `json:"regions"`
	Solution     []Mark     `json:"solution"`
	Difficulty   Difficulty `json:"difficulty"`
	Metrics      Metrics    `json:"metrics"`
}

// config holds the options applied to a Generator at construction. All fields
// are unexported; callers set them via Option functions.
type config struct {
	seed         int64
	seedSet      bool
	maxAttempts  int
	maxMutations int
	difficulty   Difficulty
}

// Option configures a Generator at construction.
type Option func(*config)

// WithSeed seeds the generator's RNG for deterministic output. If unset, the
// generator seeds itself from a non-deterministic source.
func WithSeed(seed int64) Option {
	return func(c *config) {
		c.seed = seed
		c.seedSet = true
	}
}

// WithMaxAttempts sets the sample+grow attempt cap per Generate call. Values
// <= 0 are ignored (the package default applies).
func WithMaxAttempts(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.maxAttempts = n
		}
	}
}

// WithMaxMutations sets the swap-mutation budget per attempt. Values <= 0 are
// ignored (the package default applies).
func WithMaxMutations(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.maxMutations = n
		}
	}
}

// WithDifficulty enables the discard-and-retry difficulty filter. Puzzles that
// classify outside the requested tier are rejected and Generate retries
// (counted against WithMaxAttempts). DifficultyUnknown disables the filter.
func WithDifficulty(d Difficulty) Option {
	return func(c *config) {
		c.difficulty = d
	}
}

// Generator owns pre-allocated solver state, RNG, and trace buffers. A
// Generator is NOT safe for concurrent use — one Generator per goroutine.
type Generator struct {
	cfg *config
}

// New constructs a Generator. n must be in [1, 16] and marksPerUnit must be 1
// or 2; otherwise a typed error is returned.
func New(n, marksPerUnit int, opts ...Option) (*Generator, error) {
	if n < 1 || n > nMax {
		return nil, fmt.Errorf("n=%d: %w", n, ErrNOutOfRange)
	}
	if marksPerUnit != 1 && marksPerUnit != 2 {
		return nil, fmt.Errorf("marksPerUnit=%d: %w", marksPerUnit, ErrKUnsupported)
	}

	cfg := &config{
		maxAttempts:  defaultMaxAttempts,
		maxMutations: defaultMaxMutations,
		difficulty:   DifficultyUnknown,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return &Generator{cfg: cfg}, nil
}

// Generate produces one puzzle. The scaffold slice (R-062) returns
// "not implemented"; the real implementation lands in R-065.
func (g *Generator) Generate(_ context.Context) (Puzzle, error) {
	return Puzzle{}, errors.New("generator: not implemented")
}
