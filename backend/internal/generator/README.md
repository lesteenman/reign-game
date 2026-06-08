# internal/generator/

Pure, self-contained puzzle generator for the Reign Queens-style puzzle. Per `doc.go`'s INV-GEN-1, this package does not import `model`, `repository`, `queue`, `handler`, or `worker`. Consumers translate `generator.Puzzle` into storage and transport shapes themselves.

A `*Generator` is **not** safe for concurrent use. Each goroutine that generates puzzles must construct and own its own `*Generator`. Pre-allocated scratch fields on the struct make `Generate` allocation-free after warm-up (with the documented exception of the per-call solution slice and `convertRegionsToSlices`).

## Pipeline (per Generate call)

```
                                                                       ┌─ regionsSatisfyMinSize  (R-067b safety net)
                                                                       │   safetyNetTrips++ on reject
sampleSolution ── pairSeeds ──► growRegions* ── solveAndMutate ──► bruteSolveAll(≤ 2) ──► classify ──► (filter?) ──► Puzzle
                                  (cheap or solver-guided)             unique?
```

1. **`sampleSolution`** (`sample.go`) — Row-by-row backtracker that picks `n*k` mark positions satisfying row/column/region (implicit) + 8-neighbor-adjacency constraints. Diversity comes from per-row shuffling of the candidate k-combinations.
2. **`pairSeeds`** (`pair.go`) — Groups the `n*k` marks into `n` seed groups of `k` marks each. At `k=1` it's the identity; at `k=2` it's greedy nearest-neighbor by Manhattan distance.
3. **`growRegions`** / **`growRegionsSolverGuided`** (`grower.go`, `grower_scored.go`) — Tiles the grid with `n` 4-connected regions grown out of the seeds. The cheap variant uses random-weighted-frontier growth with inverse-size weighting. The solver-guided variant (R-066) probes each multi-candidate frontier cell by tentatively assigning it, completing the tiling cheaply, running the deductive solver, and scoring by solved-cell count. `shouldUseSolverGuided` short-circuits the cheap path on known-hard combos (`k==2`, or `n>=11`) and after `cheapAttemptsBeforeEscalation = 2` cheap-grower failures.
4. **`solveAndMutate`** (`mutate.go`) — Runs the deductive solver. On `OutcomeStalled`, applies up to `budgetForCurrent()` boundary swaps in a two-pass greedy/plateau walker. Acceptance rules: strict improvement (always), same-score plateau (`1/plateauAcceptInvProb` = 1/10), one-cell regression (`1/regressionAcceptInvProb` = 1/20). On `OutcomeSolved` or `OutcomeContradiction` it returns directly.
5. **`bruteSolveAll(rm, n, k, 2)`** (`brute.go`) — Confirms uniqueness: if the brute solver finds more than one solution, the attempt is rejected.
6. **R-067b safety net** (`generator.go::regionsSatisfyMinSize`) — A post-uniqueness guard rejects attempts whose region map violates the `regionMinSize = 3` floor. KI-021 (R-06D) traced the original report to an orphan pre-R-067b worker (not a rule leak), but the guard stays as belt-and-suspenders. Each rejection increments `safetyNetTrips`; the worker logs `WARN: generator: safety-net fired ... times on puzzle X (seed=Y)` when `> 0`. The pure generator package itself emits no logs.
7. **`classify`** (`classify.go`) — Walks the solver's rule trace and returns `(Difficulty, Metrics)`. Optional `WithDifficulty` filter discards puzzles whose tier doesn't match and the orchestrator retries.

The orchestrator (`Generator.Generate`) loops up to `WithMaxAttempts` (default `20`). On budget exhaustion it returns `ErrMaxAttemptsExhausted` so callers (specifically `cmd/reproduce`) can distinguish budget exhaustion from runtime failures.

## Deductive rules (`rules.go`)

Rules are organized by tier; `solveWith` runs them in tier order and restarts from Tier 1 whenever any rule fires. The fixed point is one full pass over Tiers 1-4 with no firings. Tier 4 is empty, so classification emits only Tier 1-3 events.

| Rule | Tier | Description |
|---|---|---|
| R1 `ruleAdjacencyElimination` | 1 | Eliminate 8-neighbors of every placed mark. |
| R2 `ruleCountSaturation` | 1 | Row/col/region whose need == 0 eliminates its remaining cands. |
| R3 `ruleForcedPlacement` | 1 | Row/col/region with need == cand-count places all marks (with adjacency soundness check at k=2). |
| R4 `ruleSingleLineRegion` | 2 | All of a region's cands lie in one row/col → eliminate non-region cands on that line. |
| R5 `ruleSingleRegionLine` | 2 | All of a row's/col's cands lie in one region → eliminate region cands outside that line. |
| R7 `ruleAdjacencyForcing` | 3 | Eliminate cells whose placement would force an 8-adjacent mark elsewhere. |

The generator produces unique solutions via R1..R5, R7 at both k=1 and k=2.

## Difficulty classifier (`classify.go`)

| MaxTier fired | Difficulty bucket |
|---|---|
| 0 (no rules) | Easy (trivial by construction) |
| 1 | Easy |
| 2 | Medium |
| 3 | Hard |
| 4 | Expert |

Hard is the difficulty ceiling. Tier 4 has no rules, so the Expert bucket is unreachable; the `maxTier == 4` branch is retained defensively and `New` rejects `WithDifficulty(Expert)` with `ErrExpertUnreachable`. See `bench/difficulty-distribution.md`.

## Seed-capture mechanism

`WithSeed(int64)` pins the RNG to `rand.NewPCG(uint64(seed), uint64(seed)^0x9E3779B97F4A7C15)`. Without it, `Generator.New` seeds itself from `time.Now().UnixNano()`. The worker picks a fresh `int64` via `crypto/rand` (`worker/generator.go::newSeed` masks the sign bit) and stores it on `PuzzleRecord.Seed` so any production puzzle can be replayed deterministically via `task reproduce -- --seed=<int> --n=<N> --k=<1|2>` (`cmd/reproduce/main.go`).

Two implementation details to know about seed handling:

- `Seed=0` is the "unrecorded — generated pre-R-06C" sentinel. The serve handler omits the seed from the JSON metadata when the value is zero.
- The seed is encoded as a JSON **string** (`metadata.seed`) so JavaScript clients don't lose precision beyond `2^53`. Don't change the encoding without updating the frontend.

## Pre-allocated scratch (Generator struct)

`Generator` carries fixed-size scratch buffers so the inner generation loop allocates zero after warm-up:

- `rowMarks, colCount` (sampler)
- `solBuf, rowCombos` (sampler combos)
- `regionOf` (current region assignment)
- `solver` (final classification solver state)
- `scoringSolver, scoringGrow` (solver-guided grower probes; cloned via `*dst = *src` value copy)
- `growFrontierBuf` (frontier list, reset with `[:0]`)
- `traceBuf` (rule trace, reset with `[:0]`)

The only allocation per successful `Generate` call is the output `solution []Mark` and `convertRegionsToSlices` for the public `Regions [][]int` field.

## Purity contract (INV-GEN-1)

- No `log.` calls anywhere in the package. Signals are returned via `Metrics` (specifically `SafetyNetTrips`) and surfaced by the worker.
- No imports of `model`, `repository`, `queue`, `handler`, or `worker`.
- No global mutable state — `Generator` carries everything.

## Bench reports

Markdown files under `bench/` are committed outputs from the build-tagged benchmark suites:

- `n-feasibility.md` / `n-feasibility-deep.md` — Sampler feasibility per `(N, k)`. Justifies `NMin = 6`.
- `latency-distribution.md` — Per-`(N, k)` `Generate` latency percentiles.
- `difficulty-distribution.md` — Per-`(N, k)` difficulty bucket yields.
- `step11-handoff.md` — The two Step-11 decisions (whether `WithRacing` is needed; whether `WithDifficulty` is viable) based on the other three reports.

Regenerate by running the corresponding test with `-tags=feasibility|latency|distribution|...`.

## Key types and exported symbols

- `Generator` — non-concurrent puzzle generator. Construct with `New(n, marksPerUnit, opts...)`.
- `Option`, `WithSeed`, `WithMaxAttempts`, `WithMaxMutations`, `WithDifficulty` — construction options.
- `Puzzle` — output shape (`N`, `MarksPerUnit`, `Regions`, `Solution`, `Difficulty`, `Metrics`).
- `Mark` — `{Row, Col}` cell coordinate.
- `Difficulty` — `DifficultyUnknown / Easy / Medium / Hard / Expert`.
- `Metrics` — `{MaxTier, TierCounts, TraceLen, SafetyNetTrips}`.
- `NMin = 6` — informational floor (not enforced by `New`).
- `ErrNOutOfRange`, `ErrKUnsupported`, `ErrMaxAttemptsExhausted` — sentinel errors.

Unexported (but worth knowing):

- `solverState` — solver working state; field-pack designed for cheap `*dst = *src` cloning during probe loops.
- `ruleset` / `defaultRuleset()` — tiered rule registry; `solveWith(s, rs)` accepts a pruned ruleset for necessity tests.
- `bruteSolveAll` — deterministic uniqueness solver used in the orchestrator loop and in tests.
