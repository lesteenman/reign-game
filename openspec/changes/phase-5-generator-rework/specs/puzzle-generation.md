# Capability: Puzzle Generation (v2)

New capability specification. Supersedes any pipeline/solver/region capabilities defined in `openspec/archive/phase-3-puzzle-pool/specs/backend.md` — those describe the old three-axis strategy matrix which Phase 5 deletes.

Section IDs (PG-XX) are referenced from `tasks.md`.

---

## PG-01: Generator package location and imports (INV-GEN-1)

**Requirement.** The generator lives at `backend/internal/generator` and does not import from `backend/internal/model`, `backend/internal/repository`, `backend/internal/queue`, `backend/internal/handler`, or `backend/internal/worker`.

**Invariant INV-GEN-1 (self-contained generator).** **All** generator logic — solution sampling, deductive solving, brute solving, region growing, mutation, classification, rule-trace recording, and the orchestrator — lives under `backend/internal/generator/` and nowhere else in the repository. Consumers (`handler/`, `worker/`, `queue/`, `repository/`, frontend) perform only validation, translation, plumbing, persistence, and rendering. See `design.md` §0 for the full statement.

**Why.** Locked decision #5. The generator is a pure algorithm; storage translation is the worker's responsibility. INV-GEN-1 additionally ensures that the optional path-filtered CI job (PG-17) can rely on a single directory as the trigger scope.

**Verification.**

- `grep -r "reign-game/backend/internal/\(model\|repository\|queue\|handler\|worker\)" backend/internal/generator/` returns zero results.
- `grep -rn 'uint16\|TrailingZeros16\|OnesCount16\|solverState\|bruteSolveAll' backend/internal/handler/ backend/internal/worker/ backend/internal/queue/ backend/internal/repository/` returns zero results. Any match is a review-blocking finding and must be moved into the generator package.
- Review-local's "reuse" agent explicitly asserts INV-GEN-1 on every Phase-5 slice PR.

---

## PG-02: Public API shape

**Requirement.** The generator package exposes these types and functions, and only these as its public surface:

```go
type Difficulty int

const (
    DifficultyUnknown Difficulty = 0
    Easy              Difficulty = 1
    Medium            Difficulty = 2
    Hard              Difficulty = 3
    Expert            Difficulty = 4
)

type Mark struct {
    Row int `json:"r"`
    Col int `json:"c"`
}

type Metrics struct {
    MaxTier    int   `json:"max_tier"`
    TierCounts []int `json:"tier_counts"`
    TraceLen   int   `json:"trace_len"`
}

type Puzzle struct {
    N            int        `json:"n"`
    MarksPerUnit int        `json:"marks_per_unit"`
    Regions      [][]int    `json:"regions"`
    Solution     []Mark     `json:"solution"`
    Difficulty   Difficulty `json:"difficulty"`
    Metrics      Metrics    `json:"metrics"`
}

type Option func(*config)

func WithSeed(seed int64) Option
func WithMaxAttempts(n int) Option
func WithMaxMutations(n int) Option
func WithDifficulty(d Difficulty) Option

type Generator struct { /* unexported */ }

func New(n, marksPerUnit int, opts ...Option) (*Generator, error)
func (g *Generator) Generate(ctx context.Context) (Puzzle, error)
```

**Why.** `input-spec.md` §3, adapted for mode parameterization (locked decision #1).

**Verification.** Compile-time: `backend/internal/generator` builds with the types above. JSON round-trip test: `json.Marshal` / `json.Unmarshal` round-trip a non-trivial `Puzzle` without data loss.

---

## PG-03: Solution sampler

**Requirement.** The sampler generates a valid marker configuration satisfying row, column, and adjacency constraints. Each row holds exactly `k` marks; each column holds exactly `k` marks; no two marks are 8-neighbor adjacent.

**Implementation constraints.** Row-by-row backtracking using `uint16` bitmasks. Enumeration is **k-combinations** per row with pairwise column gap >= 2. Forward-check: `rowsRemaining * k >= (K - colCount[c])` for every column `c`. Rows are visited in **grid order** (0..n-1); diversity comes from per-row shuffling of the filtered k-combinations, driven by the Generator's RNG. This is a deliberate deviation from input-spec.md §4.1's "randomized row visit order" — shuffling the visit order breaks the prev-row adjacency pruning (because "previous row" then refers to a non-grid-adjacent row), which hangs sampling at high N + k=2. Grid-order visiting preserves the input-spec's diversity goal without the pruning regression. See design.md §4 for the full rationale.

**Performance.** <10ms/op at (N=14, k=1) and (N=14, k=2). If the gate fails, the failure is reported and a SAT-based sampler is proposed as a mitigation; the gate is not silently relaxed.

**Verification.** 200-sample unit test per (N, k) in the supported range asserting row/col k-counts, adjacency, no duplicate placements. `BenchmarkSolutionSample/N=14/k=1` and `/k=2` <10ms/op.

---

## PG-04: N-feasibility probe and NMin

**Requirement.** A `go test`-executable probe runs the sampler for a bounded time budget (default 5s) at N in [4, 5, 6, 7, 8] for each k in {1, 2} and reports distinct solutions found. An opt-in deep probe (`DEEP_PROBE=1`) extends to N∈[4..14] and cross-checks against `bruteSolveAll` with per-row regions to get exact solution counts up to a cap.

**Value.** `NMin = 6` is the k=1 package-level constant, determined empirically by R-063's deep probe: N=5 k=1 has exactly 14 solutions (too narrow for content variety), N=6 k=1 has exactly 90 solutions. For k=2 the orchestrator (R-065) enforces a separate floor of N=9: N=4..7 k=2 have 0 solutions, N=8 k=2 has exactly 2 solutions (content-dead), N=9 k=2 has 664+ distinct. Both floors are documented in `backend/internal/generator/v2/bench/n-feasibility.md` and `n-feasibility-deep.md`.

**Why.** `input-spec.md` §11 Step 2; locked decision #3 requires an empirically-grounded lower bound that considers both correctness (solutions must exist) and content-adequacy (the pool must be wide enough for long-term play).

**Verification.** `backend/internal/generator/v2/bench/n-feasibility.md` and `n-feasibility-deep.md` exist and record the tables. `NMin` is declared as a package-level constant with value 6. The generator's `New(n, k)` validates the absolute mask-width bounds ([1, 16]); content-adequacy floors are enforced by the orchestrator in R-065 with a typed error.

---

## PG-05: Brute solver

**Requirement.** A pure-function backtracker, `bruteSolveAll(regionMap [][]int, n, k, maxSolutions int) []Solution`, that returns up to `maxSolutions` valid solutions. Returns cap early (<100ms at N=14 on the worst known fixture).

**Why.** Uniqueness proof must be independent of the deductive solver (locked decision #8).

**Verification.** Returns 1 for known-unique fixtures; 2 for known-ambiguous; 0 for unsatisfiable. Benchmark `BenchmarkBruteSolver/N=14` <100ms/op.

---

## PG-06: Deductive solver state and fixed-point loop

**Requirement.** `solverState` holds the state described in `design.md` §3 (Cands, Marks, RowNeed, ColNeed, RegNeed, RegCellsByRow, RegOf, Trace). Solver applies rules in tier order; on any change, restart from Tier 1. Fixed point = no rule fires. Outcome is one of `Solved | Stalled | Contradiction`.

**Performance.** Zero allocations in the solve loop after warm-up. `BenchmarkSolverFixedPoint/N=14` <50us/op at a mid-generation state.

**Verification.** `go test -benchmem` shows 0 allocs/op on the solve loop benchmark. Handcrafted trivial puzzles solve using only Tier 1 with correct and ordered trace.

---

## PG-07: Deductive rules R1..R9

**Requirement.** All 9 rules from `input-spec.md` §4.2 are implemented, k-parameterized per `design.md` §4.2. Each rule is a pure function of `solverState`; no I/O, no globals.

**Verification.**

- Per-rule unit test: a minimal hand-crafted state where exactly that rule fires and produces the expected change.
- Per-rule necessity fixture: removing only that rule from the registry causes the fixture to fail. For outcome-subsumed rules (currently R6 and R8 — see "Subsumption" below), the necessity signal is trace-level: the rule's trace events are absent when removed. For all other rules the necessity signal is outcome-level (removed rule → state fails to make the rule's distinctive progress).
- Cross-check on a 50-puzzle corpus: deductive solution equals brute solution (on the Solved subset). Hard failure on divergence (locked decision #8). Corpus at R-064 uses D4 symmetries + hint variants of a known-unique 5x5 fixture; R-068 replaces this with a grower-generated corpus.
- Generated-corpus necessity: on a 500-puzzle corpus (R-068), every *non-subsumed* rule fires at least once. Subsumed rules (R6, R8) may never fire — this is expected, not a bug.

**Subsumption (implementation finding, R-064).** R6's precondition is a strict superset of R3's row-axis precondition; R8's preconditions at k=2 imply R3 firings on both involved rows. Because the fixed-point loop restarts from Tier 1 on any change, R3 always beats R6 and R8 to the same end-state. R6 and R8 remain in the registry for spec clarity; difficulty classification Tier 3 is effectively driven by R7 and Tier 4 by R9. See `design.md` §4.2 for the full rationale and the future-work options (tighten preconditions, move to Tier 1, or delete).

---

## PG-08: Pairer

**Requirement.**

- k=1: identity pairing (each mark is its own seed group).
- k=2: greedy nearest-neighbor Manhattan pairing on the 2N marks, producing N seed pairs.

Signature: `pair(solution []Mark, n, k int) [][]Mark` where each inner slice has length `k`.

**Verification.** For k=1: every mark in the input appears in its own singleton group. For k=2: every mark appears exactly once across the N pair groups; no pair shares a row or column (guaranteed by sampler, verified in the pairer tests).

---

## PG-09: Region grower (cheap variant)

**Requirement.** Given seed groups, tile the grid with N 4-connected regions such that each region contains all `k` of its seed marks. Growth uses random-weighted frontier selection, weighted inversely by current region size. `MinSize = k + 1` is an internal invariant.

**Verification.** 200 random solution inputs produce valid 4-connected partitions. Every region contains its `k` seed marks. No region smaller than `k + 1`.

---

## PG-10: Mutator

**Requirement.** On `Stalled` outcome, identify cells with >0 remaining candidates. For each, examine region boundaries within Manhattan distance 2 and try single-cell boundary swaps. Accept the first swap that strictly increases the deductive solver's solved-cell count. Loop up to `K` mutations (default K=50, `WithMaxMutations` adjusts).

**Region invariants during mutation.** Both affected regions must remain 4-connected and must still contain all their seed marks. Reject swaps that violate this.

**Failure modes handled.** `Contradiction` or K exhausted -> discard attempt, restart from sampler.

**Verification.** Region invariants preserved after every accepted swap. Generated corpus shows the mutator accepts swaps on Stalled states; rate-limited at K.

---

## PG-11: Orchestrator (`Generate`)

**Requirement.** `(*Generator).Generate(ctx)` runs up to `WithMaxAttempts` attempts of:

```
sample -> pair -> grow -> mutate -> deductiveSolve -> bruteUniqueCheck -> classify -> (crossCheck if debug build) -> convert
```

Returns `Puzzle` on success, `ctx.Err()` on cancellation, or a domain error after max attempts. Context is checked between attempts and (best-effort) between mutations.

`WithDifficulty` discards puzzles outside the requested tier and retries (counts against `WithMaxAttempts`).

**`MaxAttempts` is a config knob, not hardcoded.** The `WithMaxAttempts(n)` option is the single source of the cap inside the generator. Callers decide the value:

- The SQS worker reads `GenerationRequest.MaxAttempts` (which the replenish handler copied from `ConfigRecord.MaxAttempts`) and forwards it via `WithMaxAttempts` if non-zero.
- `GET /api/puzzles/generate` forwards the optional `maxAttempts` query param the same way.
- If the caller passes 0 (the protobuf/JSON zero value), the generator falls back to a package-level default constant (initial value 20, §13 tunable).

The generator itself does NOT read config, env vars, or any external state; it only honors the option value.

**Verification.** End-to-end test at (N=12, k=1) produces a puzzle passing §7.1 output-conversion checks. `ctx.Done()` propagates promptly (test with a 10ms context). Config-flow test: a `GenerationRequest` with `MaxAttempts=3` causes the generator to attempt at most 3 times (verified by injecting a seed that always produces unsolvable candidates).

**Gate (Step 7 for the orchestrator).** >=80% of attempts produce a deducible+unique puzzle within K=50 mutations at (N=12, k=1) AND (N=9, k=2). Report actuals at every supported (N, k) — do not silently relax.

---

## PG-12: Solver-guided growth (conditional)

**Requirement.** If PG-11 gate fails at either (N=12, k=1) or (N=9, k=2), implement `input-spec.md` §4.3 expensive variant: for each candidate region assignment of a frontier cell, clone the `solverState` (via `*dst = *src`), run the deductive solver on the partial puzzle, count solved cells, and choose the highest-scoring assignment.

Trace recording is **disabled** during scoring (hot-loop allocation hygiene).

**Gate.** Combined cheap+expensive success rate climbs to >=90%. Per-puzzle time penalty <3x.

**If this gate also fails:** escalate to the human. Do not ship with <90% combined rate at committed sizes (locked decision #7).

---

## PG-13: Classifier

**Requirement.** From the rule trace, compute:

- `MaxTier` = highest tier of any rule fired (1..4)
- `TierCounts` = firings per tier, as `[]int` of length 5 (index 0 unused)
- `TraceLen` = total firings

Bucket into `Difficulty`:

- `MaxTier <= 1` -> Easy
- `MaxTier == 2` -> Medium
- `MaxTier == 3` -> Hard
- `MaxTier == 4` -> Expert

**Persistence contract.** All four values (`Difficulty`, `MaxTier`, `TierCounts`, `TraceLen`) are stored on `PuzzleRecord` by the worker (see `consumer-surface.md` CS-01). No filtering on difficulty in the v1 replenish flow (locked decision #4).

**Verification.** Stable across runs for fixed-seed inputs. Classification of the 40-puzzle regression corpus (10 per tier) is exact.

---

## PG-14: Benchmarks, soak, regression corpus

**Requirement.**

- Benchmarks in `go test -bench` covering: `BenchmarkSolutionSample`, `BenchmarkSolverFixedPoint`, `BenchmarkRegionGrow`, `BenchmarkGenerateOne`, `BenchmarkGenerateParallel` — each per (N, k) in the supported range.
- Baseline committed to `backend/internal/generator/bench/baseline.txt`.
- Median + P99 `Generate()` latency per (N, k) committed to `bench/latency-distribution.md`.
- Difficulty histogram + Expert yield at (N=12, k in {1,2}) and (N=14, k in {1,2}) committed to `bench/difficulty-distribution.md`.
- Step 11 handoff document `bench/step11-handoff.md` with throughput, histograms, latency ratios, and v2 recommendations.
- Regression corpus `testdata/puzzles/*.json` with >=10 hand-verified puzzles per difficulty tier per supported (N, k).
- Soak target `go test -tags=soak` running 10,000+ samples. Not in CI.

**Verification.** `go test ./backend/internal/generator/...` passes. `go test -tags=soak ./backend/internal/generator/...` passes locally. Every listed file exists and is non-empty. Step 11 handoff contains an explicit v2 recommendation on `WithRacing` and difficulty-targeting.

---

## PG-15: Thread-safety contract

**Requirement.** `*Generator` is NOT safe for concurrent use. Each goroutine that generates puzzles must hold its own `*Generator`. This is documented (GoDoc on `New`) but not enforced by the package.

**Why.** Locked decision #6. Internal locking would destroy the pre-allocation performance win.

**Verification.** GoDoc on `New` states the contract verbatim. `BenchmarkGenerateParallel/N=12` uses `b.RunParallel` with one Generator per goroutine and demonstrates aggregate throughput scaling.

---

## PG-16: Deductive/brute cross-check (test-only)

**Requirement.** The deductive-vs-brute *match* check runs only in three places (never on the release hot path):

1. **Debug builds.** Behind the `//go:build debug` tag (or an unexported `DEBUG` const) in the orchestrator's per-attempt loop, for opt-in local development.
2. **Property tests.** `property_test.go` `TestDeductiveBruteAgree` runs the cross-check on a 500-puzzle corpus.
3. **Soak tests.** `soak_test.go` (`//go:build soak`) runs the cross-check on 10,000+ generations.

After the deductive solver returns `Solved` and the brute solver has run for uniqueness, the cross-check compares the two solutions. Divergence is a hard failure — not a retry trigger — because it means a rule is unsound (locked decision #8).

**Release hot path.** The release build runs the brute solver for uniqueness (always — two-solution early exit) but does NOT compare its solution against the deductive solver's. Keeping the cross-check off the hot path preserves generator throughput.

**Why.** Locked decision #8 and the owner's Q5 answer — the cross-check is an expensive correctness guarantee that only needs to run in test builds. Release builds rely on (a) the unit/property test suite catching unsound rules before release, and (b) the optional CI job (PG-17) re-running the soak target on every generator-touching PR.

**Verification.** `TestDeductiveBruteAgree` (500-puzzle corpus) and the soak target both execute the cross-check and fail loudly on any mismatch. Release-build benchmarks (`BenchmarkGenerateOne`) do NOT show cross-check overhead in their flame graphs.

---

## PG-17: Optional generator CI re-check workflow

**Requirement.** A GitHub Actions workflow at `.github/workflows/generator-check.yml` re-runs the soak target (which exercises PG-16's cross-check on 10,000+ generations) whenever a PR changes generator code.

**Trigger.**

```yaml
on:
  pull_request:
    branches: [main]
    paths:
      - 'backend/internal/generator/**'
      - '.github/workflows/generator-check.yml'
```

**Prerequisite.** INV-GEN-1 (PG-01) holds. If generator logic leaks into `handler/` or `worker/`, the path filter silently misses changes that should trigger the cross-check.

**Job shape.**

- Single job `generator-cross-check`.
- `continue-on-error: true` — informational, not a merge gate.
- `timeout-minutes: 30`.
- Runs `go test -tags=soak -timeout=25m ./internal/generator/...` with `working-directory: backend`.
- Uses `actions/setup-go@v6` with `go-version: "1.26"` (matching `ci.yml`).

**Status semantics.**

- Non-blocking — the job MUST NOT be listed in branch-protection required-status-checks.
- A red status flags a regression but does not block merge; reviewers decide whether to investigate.

**Why.** Owner's Q5 answer. The soak target and debug-tagged cross-check exist for correctness; exposing them in CI on every generator-touching PR converts "ran locally once" into "ran on every change" without making long-running tests a merge gate. INV-GEN-1 makes the path filter reliable.

**Verification.**

- The workflow file exists, parses as valid YAML, and its `paths:` filter matches `backend/internal/generator/**`.
- Smoke test: run `gh workflow run generator-check.yml` on a branch with a trivial generator-file change; confirm the job executes. Run the same on a branch that changes only `frontend/`; confirm the workflow is NOT triggered.
- The job is not in the repo's required-status-checks list (confirm via `gh api repos/{owner}/{repo}/branches/main/protection` or equivalent UI check).
