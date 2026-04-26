# Queens-Variant Puzzle Generator — Input Specification

This document is the **external input** to the Phase 5 design-flow, produced by a separate LLM collaboration with the project owner. It captures the generator design ahead of engaging our own OpenSpec process.

The spec was written without visibility into our existing codebase. The **Locked Decisions** document (`locked-decisions.md`) in this same directory records how we are adapting the spec to our project — mode parameterization, replacement strategy, N ceiling, difficulty scope, output shape, thread-safety, and performance-reporting discipline. `design-flow` must honor those decisions.

---

**Audience:** an LLM coding assistant working through OpenSpec to add a generator to an existing Go project that already has frontend, API, and storage layers.

**Style note:** the human owner prefers honest reporting. When a step's gate fails or a measurement comes in worse than expected, surface the numbers — do not silently relax the gate.

---

## 1. Overview and goals

Build a Go package that generates puzzles for a Queens-style game with these rules:

- N×N grid
- N colored regions of arbitrary shape, partitioning the grid (every cell belongs to exactly one region; regions are 4-connected)
- Each row, column, and region contains **exactly 2 marks**
- No two marks may be adjacent (8-neighbor / king's move), regardless of row, column, or region

**N range:** upper bound **N=14**. The lower bound is **not pre-specified** — Step 2 of the implementation plan probes small N empirically and reports where viable puzzles start. Expected somewhere in N=5..8; the implementer reports the actual cutoff.

**Output:** the consuming code expects a 2D integer array of zone IDs. Specifically `[][]int` indexed as `regions[row][col]`, with values in `[0, N)` identifying the region each cell belongs to. This is a hard interface contract — the rest of the system already depends on it.

Each generated puzzle must:

- Have a **unique solution**
- Be solvable by **pure deduction** using a fixed rule set (no guessing)
- Carry a **difficulty classification**

This package is a generator only. It does not handle persistence, HTTP, or scheduling — the existing system handles all of that. The consumer decides how to call it (single calls, batch loops, goroutine pools, Lambda invocations, whatever).

---

## 2. Requirements

### 2.1 Functional

- F1. Generate puzzles for any N in [N_min, 14], where N_min is determined by Step 2.
- F2. Each puzzle output includes `regions [][]int`, the solution mark positions, a difficulty tier, and metrics.
- F3. The solution must satisfy all constraints (rows, cols, regions, adjacency).
- F4. The deductive solver must reach the solution using only rules from §4.2.
- F5. The puzzle must be unique (verified by an independent brute solver).
- F6. Difficulty classification into tiers: Easy, Medium, Hard, Expert (configurable thresholds).
- F7. Generation honors `context.Context` cancellation — the caller controls deadlines.
- F8. Optional difficulty filter: caller can request only puzzles in a specific tier; generator discards others and retries.

### 2.2 Non-functional

- NF1. Single-puzzle generation at N=12 should average **<2s on a modern x86 core** (initial target; revise after first real benchmarks).
- NF2. Generation must respect `ctx.Done()` promptly (check at least between attempts; ideally between mutation iterations).
- NF3. Zero allocations in the solver inner loop after warm-up.
- NF4. A `*Generator` is **not** safe for concurrent use. For parallelism, callers create one Generator per goroutine. This is documented, not enforced.

### 2.3 Out of scope

- Persistence, HTTP, CLIs, deployment.
- Difficulty *targeting* during generation (only classify-and-bucket — see §10).
- N > 14 or N below the empirically determined floor.

---

## 3. Public API

```go
package generator

import "context"

type Difficulty int

const (
    Easy Difficulty = iota + 1
    Medium
    Hard
    Expert
)

type Puzzle struct {
    N          int        `json:"n"`
    Regions    [][]int    `json:"regions"`    // regions[row][col] -> region id in [0, N)
    Solution   []Mark     `json:"solution"`   // 2N marks
    Difficulty Difficulty `json:"difficulty"`
    Metrics    Metrics    `json:"metrics"`
}

type Mark struct {
    Row int `json:"r"`
    Col int `json:"c"`
}

type Metrics struct {
    MaxTier    int   `json:"max_tier"`
    TierCounts []int `json:"tier_counts"` // index 0 unused; 1..4 = tier counts
    TraceLen   int   `json:"trace_len"`
}

type Option func(*config)

func WithSeed(seed int64) Option              // deterministic generation
func WithDifficulty(d Difficulty) Option      // discard puzzles outside this tier
func WithMaxAttempts(n int) Option            // cap retry attempts before giving up
func WithMaxMutations(n int) Option           // cap region-swap mutations per attempt

type Generator struct {
    // pre-allocated state, RNG, config — unexported
}

// New creates a Generator for the given N. Returns an error if N is out of range.
// The Generator owns pre-allocated buffers and is NOT safe for concurrent use.
func New(n int, opts ...Option) (*Generator, error)

// Generate produces a single puzzle, honoring context cancellation.
// Returns ctx.Err() if cancelled, or a domain error if max attempts exhausted.
func (g *Generator) Generate(ctx context.Context) (Puzzle, error)
```

The consumer wires this into whatever execution model they want. Calling `Generate` in a loop on one Generator gives serial throughput; running multiple Generators across goroutines gives parallel throughput.

---

## 4. Algorithm specification

### 4.1 Solution sampling

Generate a valid 2N-mark configuration satisfying row, col, and adjacency constraints. Regions come later.

**Approach:** row-by-row backtracking with bitmask filtering.

**State:**
- `rowMarks [16]uint16` — column-pair bitmask per row (only first N used)
- `colCount [16]uint8` — running mark count per column
- `prevRowMask uint16` — columns marked in the previous row, for adjacency filtering

**Per-row procedure** (rows visited in randomized order to diversify output):

1. Enumerate column pairs (c1, c2) with c1 < c2 and c2 − c1 ≥ 2 (intra-row adjacency).
2. Filter pairs where `colCount[c1] < 2 && colCount[c2] < 2`.
3. If a previous row exists, filter pairs where each chosen column is not within column-distance 1 of any column in `prevRowMask` (vertical + diagonal adjacency).
4. Forward-check: rows_remaining * 2 must still be ≥ marks_still_needed_per_column for every column.
5. Shuffle filtered pairs; recurse; backtrack on dead end.

**Termination:** all N rows placed AND `colCount[c] == 2` for all c.

**Performance gate:** must complete in <10ms at N=14. If not, switch to a SAT-based sampler. Surface the actual timing.

### 4.2 Deductive solver

The solver works on partial puzzle state and applies rules until fixed point, completion, or contradiction.

**State:**

```
N             int
Cands         [16]uint16   // candidate column mask per row
Marks         [16]uint16   // confirmed mark column mask per row
RowNeed       [16]uint8    // marks still needed per row (init 2)
ColNeed       [16]uint8
RegNeed       [16]uint8
RegCellsByRow [16][16]uint16  // [regionID][row] -> column mask of cells in that region
RegOf         [16][16]uint8   // (row, col) -> region ID
```

**Rule tiers.** Each rule is `func(s *SolverState) (changed bool, ev RuleEvent)`. The solver applies rules in tier order, restarting from Tier 1 on any change. Fixed point = no rule fires.

**Tier 1 — Trivial**

- **R1 Adjacency elimination**: when a mark is placed at (r, c), eliminate all 8 neighbors as candidates.
- **R2 Count saturation**: if a row/col/region already has 2 marks, eliminate all other candidates in it.
- **R3 Forced placement**: if a row/col/region needs 2 more marks and has exactly 2 candidates remaining, place both.

**Tier 2 — Basic line/region interaction**

- **R4 Single-line region**: if all of a region's remaining candidates lie in one row (or column), eliminate that line's other candidates outside the region.
- **R5 Single-region line**: symmetric — if all of a row/col's remaining candidates lie in one region, eliminate that region's other candidates outside the line.

**Tier 3 — Pair logic**

- **R6 Locked pair in line**: if exactly two cells are a row's only candidates AND share a region, those 2 are the row's marks; eliminate the rest of the region.
- **R7 Adjacency forcing**: if placing a mark in cell X would force, via R3, two adjacent marks in some line/region, eliminate X.

**Tier 4 — Subset reasoning**

- **R8 Two-line subset (X-wing analogue)**: across two rows, if their combined remaining candidates span only 2 columns, those 2 columns hold both rows' marks; eliminate other rows' candidates in those columns. Symmetric for columns.
- **R9 Region pair exclusion**: if two regions' combined candidates in some line span only 2 cells, no other region's marks lie in that line outside those cells.

**Output:** `Solved | Stalled | Contradiction`, plus a `RuleTrace []RuleEvent` (ordered list of every rule application with the cells affected).

**Implementation contract:** rules must be pure functions of state. No I/O, no globals. Trace recording must be toggleable for performance (off during region-grower scoring, on during final classification).

### 4.3 Region generation

Inputs: the 2N solution marks.

**Step A — pair marks into region seeds.** Default: greedy nearest-neighbor pairing by Manhattan distance (pick the closest unpaired pair, repeat). Hungarian algorithm is an option to compare against in §12 — don't implement it unless benchmarking shows greedy hurts downstream success.

**Step B — grow regions to tile the grid.** Maintain `regionOf [16][16]int8` (−1 = unclaimed) and per-region frontier sets (cells unclaimed and 4-adjacent to that region).

**Cheap variant (try first):**
1. Pick a random unclaimed frontier cell.
2. Among the regions whose frontier contains it, choose one weighted inversely by current region size (encourages balanced regions).
3. Assign and update frontiers.
4. Loop until all cells claimed.

**Expensive variant (only if cheap variant's success rate <50% at Step 7):**
1. Pick an unclaimed cell adjacent to ≥1 region.
2. For each candidate region assignment, run the deductive solver on the partial puzzle and count solved cells.
3. Assign the cell to the highest-scoring region; tie-break randomly.

**Step C — mutation on stall.** After full tiling, run the deductive solver to completion.

- `Solved` AND uniqueness check passes → done.
- `Stalled` → identify cells with >0 candidates remaining. For each, examine region boundaries within Manhattan distance 2. Try swapping a single boundary cell between its two regions (boundary cell ↔ adjacent region). Re-solve. Accept the first swap that strictly increases solved-cell count. Loop up to **K** mutations (default K=50, configurable via `WithMaxMutations`).
- `Contradiction` OR K exhausted → discard and restart from Step A.

**Region invariant during mutation:** every region must remain 4-connected and contain its 2 seed marks. Reject swaps that violate this.

**Uniqueness check:** after `Solved`, run the brute solver capped at 2 solutions. If 2+ found, treat like `Stalled`.

### 4.4 Classification

From the rule trace:
- `MaxTier` = highest tier of any rule fired
- `TierCounts` = firings per tier
- `TraceLen` = total firings

Default difficulty bucket (configurable):
- **Easy**: MaxTier ≤ 1
- **Medium**: MaxTier == 2
- **Hard**: MaxTier == 3
- **Expert**: MaxTier == 4

Store all three metrics in the puzzle record. This lets the consumer re-bucket later without regenerating.

### 4.5 Conversion to output format

Final step before return: convert internal `regionOf [16][16]int8` to `[][]int` of size `[N][N]`, indexed `[row][col]`. Region IDs in the output are normalized to `[0, N)`.

---

## 5. Internal data structures

Internal computation uses fixed-size arrays sized to 16 (Go requires compile-time array sizes; `[16]` allocated, only first N used — cheaper than slice indirection in hot paths). All bitmasks are `uint16` (sufficient for N ≤ 16). Use `math/bits` `OnesCount16` and `TrailingZeros16` — these compile to single instructions on amd64/arm64.

The output `Regions [][]int` is allocated once per puzzle at the conversion step (§4.5). This is the only unavoidable allocation in the public output.

---

## 6. Performance requirements and optimization

### 6.1 Targets (revise after first real benchmarks; do not blindly chase numbers)

- Solver fixed-point pass on a typical mid-generation state: <50µs at N=12
- End-to-end single puzzle: <2s at N=12, <10s at N=14

These are **initial** targets to flag pathological regressions. Real targets come from §11 Step 11's measurements.

### 6.2 Mandatory optimizations

- All hot paths use bitmask ops, not boolean arrays.
- Pre-allocate one `SolverState` per Generator; provide internal `Reset()`; never allocate in the solve loop.
- Region grower's solver-scoring loop: clone state with `*dst = *src` (value copy of fixed-size struct), not deep copy.
- Disable trace recording during region-grower scoring; enable only for final classification pass.

### 6.3 Profiling protocol

1. Add an internal benchmark harness that runs `go test -bench` with `-cpuprofile` and `-memprofile`.
2. Establish baseline numbers at Step 11; commit them to `bench/baseline.txt`.
3. After every optimization, re-run; commit only if `benchstat` shows ≥5% improvement at low variance.
4. Re-baseline every 5 commits — drift accumulates.

---

## 7. Testing strategy

### 7.1 Unit tests

- **Solution sampler**: 200 samples at each N in the supported range; assert all constraints (row, col count, adjacency, no duplicates) hold.
- **Each solver rule**: hand-crafted minimal state demonstrating exactly that rule firing and producing the expected change.
- **Region grower**: assert all regions are 4-connected and partition the grid; assert each region contains its 2 seed marks.
- **Mutation**: assert region invariants preserved after each swap.
- **Brute solver**: returns 1 for known-unique fixtures, 2+ for ambiguous, 0 for unsatisfiable.
- **Output conversion**: assert `len(regions) == N`, `len(regions[i]) == N`, all values in `[0, N)`, exactly N distinct values.

200 samples is enough for CI to be fast and still catch regressions. A separate `go test -tags=soak` target may run 10,000+ samples for deeper checks; not required for normal CI.

### 7.2 Property tests

- For any generated puzzle: deductive solver reaches the same solution as the brute solver.
- Brute solver finds exactly one solution.
- **Rule necessity (hand-crafted)**: each rule has a fixture where removing only that rule from the registry breaks the fixture.
- **Rule necessity (generated)**: across a corpus of 500 generated puzzles, each rule fires at least once. A rule that never fires is either redundant or buggy.

### 7.3 Regression corpus

Maintain `testdata/puzzles/*.json` with ~10 hand-verified puzzles per difficulty tier per N. Tests assert: solver still solves them, classification unchanged, solution unchanged.

### 7.4 Distribution test

Time-budgeted, not count-budgeted. Run `Generate` for a configurable wall-clock duration (default 60s for CI, longer locally) at a fixed N, with no difficulty filter. Print histogram of difficulties and total count. Soft assert: each tier has ≥1% representation. The histogram is the actual signal — humans read it.

---

## 8. Benchmarking

`go test -bench=. -benchmem ./...` must cover:

- `BenchmarkSolutionSample/N=Nmin..14`
- `BenchmarkSolverFixedPoint/N=Nmin..14`
- `BenchmarkRegionGrow/N=Nmin..14`
- `BenchmarkGenerateOne/N=Nmin..14`
- `BenchmarkGenerateParallel/N=12` (uses `b.RunParallel`, one Generator per goroutine, measures aggregate throughput)

Use `-benchtime=10s` for stable numbers on the throughput benchmarks. Track results in `bench/results-<date>.txt`. PRs touching hot paths must include `benchstat` output comparing before/after.

The throughput benchmark is the single number that drives the consumer's deployment decision (Lambda vs. local batch). Report it clearly.

---

## 9. Integration contract

The consumer's existing system handles persistence, HTTP, scheduling, and parallelism. This package provides:

- A factory: `New(n, opts...)` returns a Generator.
- A method: `(*Generator).Generate(ctx)` returns one puzzle.
- A documented thread-safety boundary: one Generator per goroutine.
- Context cancellation that is honored promptly between attempts (and between mutation iterations within an attempt, if feasible without slowing the inner loop).

Anything beyond that — pooling, retry policy, output destination, deadline negotiation — is the consumer's concern.

---

## 10. Difficulty handling

V1 implements **classify-and-bucket only**. Generation is uniform; classification labels each puzzle; the optional `WithDifficulty` filter discards mismatches and retries.

Do **not** implement difficulty-targeting / biased generation in v1. Add it in v2 only if Step 11's distribution data shows the Expert tier is starvation-bound to the point that the retry loop becomes the bottleneck. Surface the actual yield in the Step 11 handoff.

---

## 11. Step-by-step implementation plan

Follow this order. Each step has a verification gate; do not proceed until the gate passes. **If a gate fails, report the failure and propose a mitigation — do not relax the gate.**

**Step 1 — Package skeleton and types**
- Create the package with the public API from §3.
- Define internal `SolverState`, `RuleEvent`, `RuleTrace`, config types.
- **Gate:** `go build ./...` passes; `Puzzle` round-trips through JSON in a test; `New` validates N range.

**Step 2 — Solution sampler + N feasibility probe (§4.1)**
- Implement row-by-row backtracker with adjacency and forward-checking.
- Probe small N: run the sampler for a fixed time budget (e.g. 5s) at N=4, 5, 6, 7, 8 and count distinct solutions found.
- **Gate:** `BenchmarkSolutionSample/N=14` runs in <10ms/op; 200-sample test passes for N=8..14. **Report the feasibility table for N=4..8 and propose N_min based on the data.**

**Step 3 — Brute solver (uniqueness checker)**
- Plain backtracker on full puzzle; returns count of solutions, capped at 2 for early exit.
- **Gate:** returns 1 for known-unique fixtures, 2+ for known-ambiguous; <100ms at N=14 on the worst fixture.

**Step 4 — Deductive solver framework + Tier 1**
- `SolverState` with bitmask fields; rule registry; fixed-point loop; toggleable trace recording.
- Implement R1, R2, R3.
- **Gate:** solves handcrafted trivial puzzles using only Tier 1; trace is correct and ordered.

**Step 5 — Tier 2–4 rules**
- Implement R4–R9 in order; each with a unit test and a hand-crafted necessity fixture.
- **Gate:** every rule has a fixture proving it's needed (removing the rule from the registry breaks that fixture).

**Step 6 — Region grower, cheap variant (§4.3 Steps A & B)**
- Greedy nearest-neighbor pairing.
- Random-weighted growth without solver scoring.
- **Gate:** produces valid 4-connected partitions for 200 random solutions; all regions contain their 2 seeds.

**Step 7 — Mutation loop (§4.3 Step C)**
- Stall detection, boundary-swap mutation with region-invariant checks, retry up to K.
- **Gate:** ≥80% of attempts produce a deducible+unique puzzle within 50 mutations at N=12. **Report the actual rate at every supported N.**

**Step 8 — Generator orchestrator + output conversion**
- End-to-end pipeline: sample → grow → mutate → classify → convert to `[][]int`.
- Honor `ctx` cancellation between attempts.
- Discard-and-retry on failure with `WithMaxAttempts` cap.
- **Gate:** `BenchmarkGenerateOne/N=12` <5s (initial; tightened later); output passes the §7.1 conversion checks.

**Step 9 — Classifier**
- Compute `MaxTier`, `TierCounts`, `TraceLen` from trace; bucket per §4.4 thresholds.
- Wire `WithDifficulty` filter into the orchestrator.
- **Gate:** classifications stable across runs for fixed-seed puzzles.

**Step 10 — Solver-guided growth (§4.3 expensive variant) — CONDITIONAL**
- **Skip if Step 7 success rate ≥80%.** Otherwise implement and benchmark.
- **Gate:** success rate climbs to ≥90%; per-puzzle time penalty <3x.

**Step 11 — Profiling, benchmarking, and distribution measurement**
- Run all `go test -bench` benchmarks at every supported N with `-benchtime=10s`. Commit results to `bench/baseline.txt`.
- Run the time-budgeted distribution test for 1 hour at N=12 and N=14 locally; record difficulty histogram and Expert yield.
- Apply §6.2 optimizations not yet in place; re-benchmark.
- **Gate:** handoff document containing per-N throughput (puzzles/sec), per-N difficulty histograms, Expert yield rates, and a written recommendation on whether v2 difficulty-targeting is needed. **If Expert yield is below a level that makes the consumer's expected use case infeasible, propose a v2 approach explicitly. Do not silently accept it.**

**Step 12 — Property tests, regression corpus, soak target**
- All §7 tests; build the regression corpus by hand-verifying ≥10 puzzles per tier per supported N.
- Add `go test -tags=soak` target running larger sample counts.
- **Gate:** `go test ./...` passes; `go test -tags=soak ./...` passes.

---

## 12. Open questions for the implementer to surface, not silently resolve

These have no predetermined answer. Report findings honestly rather than picking a default that hides the data.

- **What is N_min?** Reported in Step 2.
- **Cheap grower success rate** at each N? Reported in Step 7. Drives whether Step 10 happens.
- **Expert tier yield** at N=12, 14? Reported in Step 11. Drives v2 scope.
- **Greedy vs. Hungarian pairing**: does Hungarian materially improve downstream success rate at any N? Run a controlled comparison with the same seed and report the delta. If the delta is small, stay with greedy.
- **`uint16` vs. `uint32` for masks**: is the narrower type actually faster on amd64 and arm64, or does the narrowing cost cancel the cache benefit? Microbenchmark before committing.

---

## 13. Non-goals and explicit anti-patterns

- **Do not** add CLIs, HTTP handlers, or persistence — the consumer has these.
- **Do not** implement difficulty-biased generation in v1.
- **Do not** add a third "intermediate" solver tier between brute and deductive — there are exactly two solvers, with distinct purposes (deduction proves human-solvability; brute proves uniqueness).
- **Do not** add a rule beyond Tier 4 without a corresponding difficulty bucket and a generated-corpus necessity test.
- **Do not** skip the brute uniqueness check on the assumption that "if the deductive solver solved it, it's unique." Deduction proves a solution exists and is reachable; it does not prove no other solution exists.
- **Do not** make `Generator` thread-safe via internal locking. Document the per-goroutine contract instead — locking would silently destroy the pre-allocation performance win.
