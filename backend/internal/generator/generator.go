package generator

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"
)

// NMin is the package-level floor for the grid size N.
//
// Empirically chosen from R-063's feasibility probe (bench/n-feasibility.md):
//   - N=4 k=1 has exactly 2 solutions.
//   - N=5 k=1 has exactly 14 solutions.
//   - N=6 k=1 has exactly 90 solutions (brute-verified, not capped).
//
// 14 unique base solutions at N=5 is too narrow a pool for long-term content
// variety (solutions × region maps grown over them). N=6 gives ~6× more.
// The constant is informational: New does not enforce n >= NMin; downstream
// orchestrators (R-065) respect it.
const NMin = 6

// nMax is the bitmask-width ceiling. The solver uses uint16 masks, so n > 16
// is unrepresentable.
const nMax = 16

// defaultMaxAttempts is the fallback attempt cap when a caller does not pass
// WithMaxAttempts. Tunable via the same option and, at the consumer layer,
// via ConfigRecord.MaxAttempts / GenerationRequest.MaxAttempts.
const defaultMaxAttempts = 20

// defaultMaxMutations is the default swap-mutation budget per attempt at
// k=1. The walker takes graded plateau and small-regression swaps, so it
// needs room to oscillate past local optima before reaching Solved. 300
// is the current empirical pick at k=1 where probe-solves are fastest.
const defaultMaxMutations = 300

// defaultMaxMutationsK2 is the per-attempt budget at k=2, where probe-
// solves are several times slower than at k=1 and the walker's success
// rate saturates early on reachable puzzles. Extra budget past ~100
// does not lift the Step 7 rate at any tested N, and the end-to-end
// N=12 k=2 wall time is dominated by probe-solve cost per attempt
// rather than per-step budget (R-068 will reduce that further).
const defaultMaxMutationsK2 = 100

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

// Metrics captures the classifier's view of a generated puzzle and
// any diagnostic counters the caller should surface (e.g. log).
type Metrics struct {
	MaxTier    int   `json:"max_tier"`
	TierCounts []int `json:"tier_counts"`
	TraceLen   int   `json:"trace_len"`
	// SafetyNetTrips counts how many attempts were rejected by the
	// regionsSatisfyMinSize guard before the successful attempt. Zero
	// is the expected case; non-zero means the grower or mutator
	// produced an under-size region that the guard caught.
	// The worker logs a WARN when this is > 0 so the generator package
	// stays silent (no log calls in the pure layer).
	SafetyNetTrips int `json:"safety_net_trips,omitempty"`
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
	n   int
	k   int
	rng *rand.Rand

	// Sampler scratch buffers. Pre-allocated in New so the inner loop is
	// allocation-free after warm-up. See sample.go for the invariants.
	rowMarks [nMax]uint16
	colCount [nMax]uint8
	solBuf   []Mark
	// rowCombos[row] is the filtered & shuffled list of candidate column
	// bitmasks considered for that row at the current backtracker depth.
	rowCombos [nMax][maxCombosPerRow]uint16

	// Grower / mutator scratch. Reused across attempts via reset, not
	// reallocated. regionOf is the authoritative cell-to-region assignment
	// for the current attempt (-1 = unclaimed during growth; no -1 values
	// after growRegions returns).
	regionOf [nMax][nMax]int8
	// solver holds the deductive solver state for the orchestrator's solve
	// pass. Pre-allocated so Generate does not allocate an 8KB state per
	// attempt.
	solver solverState
	// scoringSolver is the solver-guided grower's scratch state (R-066).
	// Pre-allocated on the Generator so solver-guided probes use value-copy
	// clone (*dst = *src) rather than reallocating. Trace recording is OFF
	// on this state (set and stays nil).
	scoringSolver solverState
	// scoringGrow is the solver-guided grower's grow-state scratch (R-066).
	// Holds the tentative completion while probing a candidate region
	// assignment. Pre-allocated to keep probes alloc-free.
	scoringGrow growState
	// growFrontierBuf is the scratch frontier list reused by every grower
	// invocation (cheap and solver-guided). Sized at New() to nMax*nMax so
	// it never needs to grow. Callers use g.growFrontierBuf[:0] to reset.
	growFrontierBuf []int
	// traceBuf backs solver.trace during the final classification pass. The
	// mutator's probe passes keep solver.trace == nil (NF3: zero alloc in
	// the hot loop).
	traceBuf ruleTrace
}

// maxCombosPerRow bounds the number of valid k-combinations of columns for a
// single row at N=nMax (16) with pairwise gap >= 2. For k in {1, 2} the
// upper bound is C(16, 2) = 120 (the k=1 case is nMax=16, strictly smaller).
// We size the pre-allocated scratch buffer to exactly this bound;
// enumerateKCombos panics if over-emission is ever attempted so that any
// future k=3 path (or off-by-one) fails loudly instead of silently
// reallocating onto the heap and defeating NF3 (zero-alloc hot loop).
const maxCombosPerRow = 120

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

	seed := cfg.seed
	if !cfg.seedSet {
		seed = time.Now().UnixNano()
	}
	g := &Generator{
		cfg:             cfg,
		n:               n,
		k:               marksPerUnit,
		rng:             rand.New(rand.NewPCG(uint64(seed), uint64(seed)^0x9E3779B97F4A7C15)),
		solBuf:          make([]Mark, 0, n*marksPerUnit),
		growFrontierBuf: make([]int, 0, nMax*nMax),
		traceBuf:        make(ruleTrace, 0, defaultTraceCap),
	}
	return g, nil
}

// defaultTraceCap caps the rule-trace backing slice. Even Expert puzzles
// rarely exceed a few dozen rule firings — 512 is ample headroom and lets
// the append in (*solverState).record never reallocate.
const defaultTraceCap = 512

// ErrMaxAttemptsExhausted is returned by Generate when the configured
// WithMaxAttempts cap is hit without producing a valid, unique,
// deducible puzzle that matches the optional WithDifficulty filter.
var ErrMaxAttemptsExhausted = errors.New("generator: max attempts exhausted")

// cheapAttemptsBeforeEscalation is the number of cheap-grower attempts
// per Generate call before the orchestrator escalates to the R-066
// solver-guided variant. Cheap combos (most N at k=1) hit >90% in one
// attempt; hard combos (N>=10 k=1, all k=2) usually need the guided
// variant. Escalating after 2 failures avoids paying the guidance cost
// for cheap combos that succeed on attempt 1.
const cheapAttemptsBeforeEscalation = 2

// shouldUseSolverGuided picks between the cheap grower (fast, >90% on
// most (N, k)) and the R-066 solver-guided variant (expensive per attempt
// but dramatically better on hard combos).
//
// The predicate short-circuits on known-hard combos so we don't burn the
// cheap-first attempts on cases where cheap empirically succeeds <30%:
//   - k=2 at any N: cheap fails near-100% (see R-065 Step 7 data).
//   - N>=11 at k=1: cheap fails >70% (same data).
//
// Otherwise, the first cheapAttemptsBeforeEscalation attempts use cheap.
func shouldUseSolverGuided(attempt, n, k int) bool {
	if k == 2 {
		return true
	}
	if n >= 11 {
		return true
	}
	return attempt >= cheapAttemptsBeforeEscalation
}

// Generate produces one puzzle. Runs up to g.cfg.maxAttempts iterations of
// the pipeline described in PG-11 / input-spec §11 Step 8:
//
//	sample -> pair -> grow -> solve+mutate -> brute-uniqueness ->
//	  classify -> (optional difficulty filter) -> convert
//
// Honors ctx cancellation between attempts; returns ctx.Err() unchanged so
// callers can distinguish Canceled/DeadlineExceeded from domain errors.
//
// On total failure (budget exhausted) returns ErrMaxAttemptsExhausted.
func (g *Generator) Generate(ctx context.Context) (Puzzle, error) {
	maxAttempts := g.cfg.maxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}

	// safetyNetTrips counts guard-rejected attempts across this Generate
	// call. Copied into Metrics on return so the caller (worker) can
	// surface a WARN when > 0.
	safetyNetTrips := 0

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return Puzzle{}, err
		}

		marks, ok := g.sampleSolution()
		if !ok {
			continue
		}
		seeds := pairSeeds(marks, g.n, g.k)
		var grew bool
		if shouldUseSolverGuided(attempt, g.n, g.k) {
			grew = g.growRegionsSolverGuided(seeds, &g.regionOf)
		} else {
			grew = g.growRegions(seeds, &g.regionOf)
		}
		if !grew {
			continue
		}
		if outcome := g.solveAndMutate(seeds); outcome != mutationSolved {
			continue
		}
		// Brute-uniqueness check. The solver in g.solver holds the
		// deductive solution; we also want to confirm no alternative
		// solution exists.
		rm := convertRegionsToSlices(&g.regionOf, g.n)
		sols, err := bruteSolveAll(rm, g.n, g.k, 2)
		if err != nil || len(sols) != 1 {
			continue
		}

		// R-067b safety net (R-06C). Kept as belt-and-suspenders.
		// KI-021 (R-06D) traced the original under-size region report
		// to an orphan pre-R-067b worker, not a real rule leak, but
		// the guard stays because it's cheap and catches regressions.
		// The counter is surfaced via Metrics.SafetyNetTrips and the
		// worker emits a WARN line when > 0 (the generator package
		// intentionally does no logging of its own).
		if !regionsSatisfyMinSize(rm, g.n) {
			safetyNetTrips++
			continue
		}

		// Trace-enabled classification pass. Reset the solver state with
		// trace recording, re-solve, then classify.
		g.solver.trace = g.traceBuf[:0]
		if err := g.solver.initFromRegionMap(rm, g.n, g.k); err != nil {
			continue
		}
		if solve(&g.solver) != OutcomeSolved {
			// Extremely unlikely — the just-solved puzzle should be
			// deducible. Skip on any mismatch.
			continue
		}
		difficulty, metrics := classify(g.solver.trace)
		g.traceBuf = g.solver.trace // cache back for reuse

		if g.cfg.difficulty != DifficultyUnknown && difficulty != g.cfg.difficulty {
			continue
		}

		solution := g.solver.appendSolutionMarks(make([]Mark, 0, g.n*g.k))
		metrics.SafetyNetTrips = safetyNetTrips
		return Puzzle{
			N:            g.n,
			MarksPerUnit: g.k,
			Regions:      convertRegionsToSlices(&g.regionOf, g.n),
			Solution:     solution,
			Difficulty:   difficulty,
			Metrics:      metrics,
		}, nil
	}

	return Puzzle{}, ErrMaxAttemptsExhausted
}

// regionsSatisfyMinSize returns true iff every region in the given
// region map has at least regionMinSize cells. Caller supplies a
// normalized [n][n] map whose region ids cover [0, n) so this function
// can index its bucket slice by id directly.
func regionsSatisfyMinSize(rm [][]int, n int) bool {
	sizes := make([]int, n)
	for r := range rm {
		for _, gid := range rm[r] {
			sizes[gid]++
		}
	}
	for _, sz := range sizes {
		if sz < regionMinSize {
			return false
		}
	}
	return true
}
