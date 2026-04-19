# Phase 5: Generator Rework — Design Document

Authoritative technical design for Phase 5. Combines `input-spec.md` (algorithm) with `locked-decisions.md` (project adaptations) and the three grill points captured in `design-grill-summary.md`. Implementation slices live in `tasks.md`.

---

## 0. Invariants

Project-wide invariants this design commits to. Each is a review-blocking finding if violated.

### INV-GEN-1: Generator package is self-contained

**All** generator logic — solution sampling, deductive solving, brute solving, region growing, mutation, classification, rule-trace recording, and the orchestrator that stitches them together — lives under `backend/internal/generator/` and nowhere else in the repository.

Consumers of the generator (`backend/internal/handler/**`, `backend/internal/worker/**`, `backend/internal/queue/**`, `backend/internal/repository/**`, the frontend) perform only:

- **Validation** of incoming request parameters (handler).
- **Translation** from `generator.Puzzle` to domain types (`model.Puzzle` in the handler; `repository.PuzzleRecord` in the worker).
- **SQS plumbing** (worker dispatch).
- **Persistence** (repository writes).
- **UI rendering** (frontend).

A consumer MUST NOT contain: row-by-row backtracking, bitmask-level cell reasoning, rule R1..R9 implementations, region-grower frontier logic, mutation acceptance checks, or difficulty classification. If code matching any of those descriptions appears outside `backend/internal/generator/`, that is a review-blocking finding and must be moved into the generator package.

**Why.** Three reasons:

1. **Single source of truth for correctness.** The deductive solver, brute solver, and mutation loop cooperate via shared bitmask invariants. Splitting them across packages creates the same class of soundness bug that motivated the Phase 5 rewrite.
2. **Testability.** Unit tests, property tests, the soak target, and the optional CI job (§16) all target one directory. If logic leaks into `handler/` or `worker/`, the cross-check coverage goes with it.
3. **The optional CI job (§16) depends on it.** The path-filtered workflow re-runs the soak target on changes under `backend/internal/generator/**`. If generator logic lives elsewhere, a change that ought to trigger the cross-check silently slips past the path filter.

**Verification.**

- `grep -rn "uint16\|TrailingZeros16\|OnesCount16\|solverState\|bruteSolveAll" backend/internal/handler/ backend/internal/worker/ backend/internal/queue/ backend/internal/repository/` returns zero results.
- The generator package does not import `model`, `repository`, `queue`, `handler`, or `worker` (covered by PG-01).
- Review-local's "reuse" agent explicitly asserts INV-GEN-1 on every Phase-5 slice PR.

---

## 1. High-level structure

```
┌──────────────────────────────────────────────────────────────────────┐
│                     backend/internal/generator                        │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │ New(n, k, opts...) *Generator                                  │  │
│  │  (pre-allocates SolverState, RNG, buffers — NOT goroutine-safe)│  │
│  │                                                                │  │
│  │ (g *Generator).Generate(ctx) (Puzzle, error)                   │  │
│  │    Loop up to WithMaxAttempts:                                 │  │
│  │      ↳ sampler:    sample a 2N-mark solution (k-aware)         │  │
│  │      ↳ pairer:     group marks into N seeds of k marks each    │  │
│  │      ↳ grower:     grow N regions around seeds                 │  │
│  │      ↳ mutator:    swap boundary cells while stalled (≤ K)     │  │
│  │      ↳ solver:     deductive → Solved or Stalled               │  │
│  │      ↳ brute:      uniqueness check (cap 2 solutions)          │  │
│  │      ↳ classifier: MaxTier/TierCounts/TraceLen → Difficulty    │  │
│  │      ↳ (test)      cross-check deductive solution vs brute     │  │
│  │      ↳ convert:    [16][16]int8 regionOf → [N][N]int Regions   │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                  │                                    │
│                                  ▼  generator.Puzzle{N,Regions,       │
│                                      Solution,Difficulty,Metrics}     │
└──────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌──────────────────────────────────────────────────────────────────────┐
│                    backend/internal/worker                            │
│    processMessage(ctx, record):                                       │
│      req := decode(record.Body)                                       │
│      g, _ := generator.New(req.Size, k(req.Mode), options(req)...)    │
│      pz, _ := g.Generate(genCtx)                                      │
│      rec := translate(pz, req)   // see §5                            │
│      store.PutPuzzle(ctx, rec)                                        │
└──────────────────────────────────────────────────────────────────────┘
```

Only the worker and the `/api/puzzles/generate` debug handler instantiate Generators. Nothing else imports `generator` after the rework.

---

## 2. Public API

```go
package generator

import "context"

// Difficulty tiers, computed from rule trace. Persisted on PuzzleRecord;
// not surfaced to players in v1 (locked decision #4).
type Difficulty int

const (
    DifficultyUnknown Difficulty = 0
    Easy              Difficulty = 1
    Medium            Difficulty = 2
    Hard              Difficulty = 3
    Expert            Difficulty = 4
)

// Mark is a cell coordinate. Row and Col are zero-indexed.
type Mark struct {
    Row int `json:"r"`
    Col int `json:"c"`
}

// Metrics captures the classifier's view of the puzzle.
type Metrics struct {
    MaxTier    int   `json:"max_tier"`     // 1..4
    TierCounts []int `json:"tier_counts"`  // len 5; index 0 unused
    TraceLen   int   `json:"trace_len"`
}

// Puzzle is the generator's output shape. Storage types are built from it.
type Puzzle struct {
    N            int        `json:"n"`
    MarksPerUnit int        `json:"marks_per_unit"` // k ∈ {1, 2}
    Regions      [][]int    `json:"regions"`        // regions[row][col], len N×N, values [0, N)
    Solution     []Mark     `json:"solution"`       // N*k marks
    Difficulty   Difficulty `json:"difficulty"`
    Metrics      Metrics    `json:"metrics"`
}

// Option configures a Generator.
type Option func(*config)

func WithSeed(seed int64) Option              // deterministic RNG
func WithMaxAttempts(n int) Option            // sample+grow attempts per Generate call
func WithMaxMutations(n int) Option           // swap mutations per attempt (default K=50)
func WithDifficulty(d Difficulty) Option      // discard-and-retry filter; off by default
// WithRacing(n int) — deliberately NOT introduced in v1 (locked decision #6, grill point c)

// Generator owns pre-allocated SolverState, brute-solver buffers, and RNG.
// NOT safe for concurrent use. One Generator per goroutine (per SQS invocation).
type Generator struct { /* unexported */ }

// New constructs a Generator. Returns an error if n or marksPerUnit is out of
// range. Supported ranges: n in [N_min, 16] (N_min determined by Step 2);
// marksPerUnit in {1, 2}.
func New(n, marksPerUnit int, opts ...Option) (*Generator, error)

// Generate produces one puzzle. Honors ctx cancellation between attempts
// and, where feasible without slowing the hot loop, between mutation
// iterations within an attempt. Returns ctx.Err() if cancelled, or a domain
// error after WithMaxAttempts is exhausted.
func (g *Generator) Generate(ctx context.Context) (Puzzle, error)
```

No imports from `model`, `repository`, or `queue` (locked decision #5). The generator stays a pure algorithm package.

---

## 3. Internal data structures

All hot-path state is fixed-size `[16]` arrays, `uint16` bitmasks, and `math/bits` for popcount/trailing-zeros. Mask width 16 covers N up to 16; the constructor rejects N > 16 explicitly.

```go
// Compact solver state; cloned via value copy (*dst = *src) in the region grower.
type solverState struct {
    N, K           int
    Cands          [16]uint16   // candidate column mask per row
    Marks          [16]uint16   // confirmed marks per row
    RowNeed        [16]uint8    // marks still needed per row (init K)
    ColNeed        [16]uint8
    RegNeed        [16]uint8
    RegCellsByRow  [16][16]uint16 // [regionID][row] -> column mask
    RegOf          [16][16]uint8  // (row, col) -> region ID
    Trace          ruleTrace     // pooled; recording toggleable
}
```

`RuleTrace` is backed by a preallocated slice owned by the Generator and `[:0]`-reset per Generate call. Trace recording is off during region-grower scoring (Step 10) and on for the final classification pass.

---

## 4. Mode parameterization (`k` as first-class parameter)

Locked decision #1. Every module that the spec describes in terms of "2 marks per row" is implemented in terms of `k`:

### 4.1 Sampler (spec §4.1, k-generalized)

- Enumerate **k-combinations** of columns per row with minimum intra-row column gap ≥ 2 (adjacency). For k=1 this is "each of N columns"; for k=2 this is the spec's pair enumeration.
- `colCount[c]` runs to K per column (not always 2).
- Forward-check: `rowsRemaining * k >= (K - colCount[c])` for every column `c`.
- The spec's "c1 < c2 with c2 − c1 ≥ 2" pair filter generalizes to **strictly increasing column sequences with pairwise column gap ≥ 2**.

**Implementation note — grid-order row visiting (deviation from input-spec §4.1):** the spec calls for "rows visited in randomized order to diversify output." We do NOT do that. Instead we visit rows 0..n-1 in grid order and shuffle the *filtered k-combinations per row* before recursion. Rationale: the prev-row adjacency mask (`adjacentColumnsMask(rowMarks[row-1], n)`) prunes aggressively only when "previous row" means "grid-adjacent previous row." Shuffling the visit order turns that pruning into dead code (non-grid-adjacent rows don't forbid each other's columns), which blew up to multi-minute sampler hangs at N=13 k=2 during R-063 smoke. Grid-order visiting plus per-row combo shuffling preserves the spec's diversity goal (verified by `TestSampleDistinct`) and keeps N=14 benchmarks under 100µs for both k. PG-03 has been updated to reflect the implementation.

### 4.2 Rule tiers

Every rule statement that references "2" in the spec is templated on `k`:

- **R2 Count saturation:** "row/col/region already has `K` marks" (not 2).
- **R3 Forced placement:** "needs `m` more marks and has exactly `m` candidates" — works for any residual `m` ∈ [1, K].
- **R6 Locked pair in line:** "the row's only candidates are exactly `K` cells AND they all share a region."
- **R8 Two-line subset (X-wing analogue):** "across `K` rows, if combined candidates span only `K` columns, those columns hold all `K²` marks." For k=2, the spec's 2-row/2-col X-wing is k=2; the k=1 collapse is the traditional 1-row/1-col Naked Single.
- **R9 Region pair exclusion:** templated identically on `k`.

**Implementation note — R6 / R8 subsumption (found in R-064):** R6's precondition ("the row's only candidates are exactly K cells AND they share a region") is a strict superset of R3's row-axis firing precondition ("rowNeed equals cands count"). In the restart-from-Tier-1 fixed-point loop, R3 (Tier 1) always fires before R6 (Tier 3); after R3 places the marks, R2 eliminates the region's stale candidates. End-state matches R6's intended progress. Same for R8 at k=2: R8's preconditions imply R3-row firings for both rows involved, so R3 beats R8 to the same outcome. Both rules are retained in the registry for spec clarity and to keep their preconditions documentable, but they never contribute deductively distinct progress on reachable states. Their necessity tests are trace-level (removing the rule produces a trace without its events) rather than outcome-level. **Consequence for difficulty classification:** `MaxTier` will never reach 3 via R6 alone or 4 via R8 alone — Tier 3 remains populated by R7 (genuine), Tier 4 by R9 (genuine). R-068's 500-puzzle corpus will show the actual per-rule firing rate; if R6/R8 are truly unreachable, a future slice may move them to Tier 1 (tighter supersets of R3) or delete them with a spec update.

### 4.3 Pair-into-seeds (spec §4.3 Step A)

- For k=1: degenerates to "every mark is its own region seed." N seeds, one each. Pairer is skipped.
- For k=2: greedy nearest-neighbor Manhattan pairing on 2N marks → N seed-pairs. Hungarian deferred to Step 11 comparison.
- Shared interface: `pair(solution []Mark, n, k int) [][]Mark` returns N seed groups of k marks each.

### 4.4 Region grower and mutator

- Seed count is N regardless of `k`; each region carries `k` seed marks.
- Frontier/growth logic is cell-level and not k-aware.
- Mutation validates **per-region** invariants: region remains 4-connected AND contains all `k` of its seed marks (not "its 2 seed marks" as the spec says).

### 4.5 Bitmask-width safety

- `n` in `New(n, k, opts)` must be in [N_min, 16]. Reject otherwise with a typed error.
- Pair enumeration is **k-combinations**, not hardcoded pair-enumeration. No loop body contains a literal `2` for k-dimension; use `g.k`.
- All popcount/trailing-zero operations use `math/bits.OnesCount16` / `TrailingZeros16`. No manual shifts by literal 16.
- Arrays are `[16]…`; any reference to a width other than `g.n` in a mask op is a bug.

---

## 5. Output translation layer (worker-side)

Locked decision #5. The generator returns `generator.Puzzle`. The worker builds a `repository.PuzzleRecord`:

```go
// worker/generator.go (pseudocode for the translation portion)

g, err := generator.New(req.Size, kFromMode(req.Mode),
    generator.WithMaxAttempts(attempts(req.Size, req.Mode)),
)
if err != nil { return err }

startTime := time.Now()
pz, err := g.Generate(genCtx)
if err != nil { return err }
durationMs := time.Since(startTime).Milliseconds()

puzzleID, err := w.newUUID()
if err != nil { return err }

// Translate generator.Puzzle → repository.PuzzleRecord
solutionGrid := make([][]bool, pz.N)
for i := range solutionGrid {
    solutionGrid[i] = make([]bool, pz.N)
}
for _, m := range pz.Solution {
    solutionGrid[m.Row][m.Col] = true
}

rec := &repository.PuzzleRecord{
    GridSize:             req.Size,
    Mode:                 req.Mode,
    ID:                   puzzleID,
    Status:               "ready",
    Verdict:              "none",
    RegionMap:            pz.Regions,       // [][]int direct copy
    Solution:             solutionGrid,     // [][]bool built from []Mark
    Deducible:            true,             // always true by construction
    Difficulty:           int(pz.Difficulty),
    MaxTier:              pz.Metrics.MaxTier,
    TierCounts:           pz.Metrics.TierCounts,
    TraceLen:             pz.Metrics.TraceLen,
    GenerationDurationMs: durationMs,
    CreatedAt:            time.Now().UTC().Format(time.RFC3339),
}

if err := w.store.PutPuzzle(ctx, rec); err != nil { return err }
```

`kFromMode` is a small helper in `worker/generator.go` (or a shared constant in `handler`):

```go
func kFromMode(mode string) int {
    if mode == handler.ModeDouble { return 2 }
    return 1
}
```

The generator is ignorant of `"standard"`/`"double"` strings; those remain in the handler/worker layer.

---

## 6. Consumer-side cleanup (delete-list)

Locked decision #2. The following fields and files are removed. Every field here is traced to at least one caller in the current codebase (grep sweep is the Step T-06x.1 task).

### 6.1 `repository.PuzzleRecord` — field changes

| Field | Action |
|-------|--------|
| `Pipeline` | **Remove** |
| `Solver` | **Remove** |
| `Regions` | **Remove** |
| `RegionVariance` | **Remove** |
| `Concurrency` | **Remove** |
| `Deducible` | **Keep** (true by construction; record-keeping) |
| `Difficulty` (new) | `int` — generator-assigned tier |
| `MaxTier` (new) | `int` |
| `TierCounts` (new) | `[]int` |
| `TraceLen` (new) | `int` |

Schema note: DynamoDB is schemaless; the new fields appear on new records. Existing `ready` records in the pool at cutover are flushed (see proposal "Out of Scope / data migration").

### 6.2 `repository.ConfigRecord` — field changes

| Field | Action |
|-------|--------|
| `Pipeline` | **Remove** |
| `Solver` | **Remove** |
| `Regions` | **Remove** |
| `RegionVariance` | **Remove** |
| `Concurrency` | **Remove** |
| `Deducible` | **Keep** (always true, for future flexibility) |
| `Threshold` | **Keep** |
| `Enabled` | **Keep** |

The config schema collapses to `{size, mode, deducible, threshold, enabled}`. Add `maxAttempts int` with default 0 (meaning "use generator package default") — keeps a tunable knob for Double at higher N without re-opening the full strategy matrix.

### 6.3 `queue.GenerationRequest` — field changes

| Field | Action |
|-------|--------|
| `Size` | Keep |
| `Mode` | Keep |
| `Deducible` | Keep |
| `Pipeline`, `Solver`, `Regions`, `RegionVariance`, `Concurrency` | **Remove** |
| `MaxAttempts int` (new, optional) | **Add** — mirrors `ConfigRecord.MaxAttempts` |

Old in-flight SQS messages carrying the removed fields will fail to decode after deploy. Mitigation: drain the queue before deploy (it is a dev-only queue today, and at cutover we flush the pool anyway).

### 6.4 `handler.GenerateParams` + `handler.BuildPipeline` + `handler.ParseGenerateParams`

- `BuildPipeline` is **deleted entirely**.
- `GenerateParams` loses `Pipeline`, `Solver`, `Regions`, `RegionVariance`, `Concurrency`. `MarkersPerUnit`, `MinSize` stay (derived from `Mode`).
- `ParseGenerateParams` no longer reads `pipeline`, `solver`, `regions`, `regionVariance`, `concurrency`. Unknown query params are ignored (backward-tolerant for bookmarked URLs).

### 6.5 Handlers and worker

- `handler/generate.go` (`GET /api/puzzles/generate`) calls `generator.New(...)` directly — no pipeline indirection.
- `worker/generator.go` calls `generator.New(...)` directly.
- `handler/admin_config.go` validation drops pipeline/solver/regions/regionVariance/concurrency field checks.

### 6.6 Frontend

- `frontend/src/services/adminService.ts` — `ConfigData` loses the five strategy fields and `concurrency`. Gains nothing (difficulty is not surfaced yet).
- `frontend/src/pages/AdminPage.tsx` — `PIPELINE_OPTIONS`, `SOLVER_OPTIONS`, `REGIONS_OPTIONS`, region-variance/concurrency inputs, and related props in `ConfigForm` are removed. This also closes **KI-015** and **KI-016** (they become moot).
- Admin UI may display read-only `difficulty`, `maxTier`, `traceLen` per puzzle in a future richer-pool phase — **not in Phase 5 scope**.

### 6.7 LocalStack seed

`.localstack/init-aws.sh` CONFIG items drop `pipeline`, `solver`, `regions`, `regionVariance`, `concurrency`. Five items become:

```json
{ "PK": "CONFIG", "SK": "5#standard", "deducible": true, "threshold": 3, "enabled": true }
{ "PK": "CONFIG", "SK": "7#standard", "deducible": true, "threshold": 3, "enabled": true }
{ "PK": "CONFIG", "SK": "9#standard", "deducible": true, "threshold": 3, "enabled": true }
{ "PK": "CONFIG", "SK": "7#double",   "deducible": true, "threshold": 3, "enabled": true }
{ "PK": "CONFIG", "SK": "9#double",   "deducible": true, "threshold": 3, "enabled": true }
```

Double combos can finally be `enabled=true` in local dev — that is the primary KI-007 close condition (backend Lambda timeout enforces the same 14-minute budget in prod).

---

## 7. Deductive/brute cross-check (locked decision #8)

**Release hot path.** In release builds the generator runs the brute solver to prove uniqueness (two solutions = reject). It does NOT compare the brute solution against the deductive solver's solution — that check is test-only. Keeping the cross-check off the hot path preserves the ~<100ms N=14 brute budget without paying for an equality walk of two N*k-mark solutions per generated puzzle.

**Test-only cross-check.** The deductive-vs-brute *match* runs only:

1. Behind the `//go:build debug` tag (or an unexported `DEBUG` const) in the orchestrator's per-attempt loop, so developers can opt in locally.
2. In `property_test.go` against a 500-puzzle corpus.
3. In `soak_test.go` (`//go:build soak`) against 10,000+ generations.

```go
// pseudocode inside the generator loop, guarded by the debug build tag
//go:build debug

if debug.GeneratorCrossCheck {
    brute := g.bruteSolveAll(regionMap, maxSolutions=2)
    if len(brute) != 1 {
        return Puzzle{}, fmt.Errorf("uniqueness check failed: %d solutions", len(brute))
    }
    if !equalSolutions(brute[0], g.deductiveSolver.Solution()) {
        return Puzzle{}, fmt.Errorf("deductive/brute mismatch — unsound rule")
    }
}
```

A mismatch is a hard failure — no retry — because it means a rule is unsound. The release build never sees this branch; the soak/property tests exercise it on every generation.

**CI exposure.** Because the cross-check is test-only, the release CI job (`.github/workflows/ci.yml`) does not exercise it. The optional path-filtered workflow described in §16 re-runs the soak target whenever a PR touches generator code, giving us CI-visible evidence on every generator-touching change without making the check a merge gate.

---

## 8. Difficulty metrics (locked decision #4)

- Generator **always** computes `Difficulty`, `MaxTier`, `TierCounts`, `TraceLen` for every produced puzzle.
- Worker **always** writes them to `PuzzleRecord`.
- Replenish flow **does not** filter on difficulty (the optional `WithDifficulty` Generator option exists but is never called by replenish in v1).
- Frontend **does not** surface difficulty to players (that's R-034, Phase 9).
- Admin UI **may** surface difficulty read-only — not part of Phase 5 scope, but the data is there the moment Phase 5 lands.

Classification thresholds (`MaxTier ≤ 1 → Easy`, `=2 → Medium`, `=3 → Hard`, `=4 → Expert`) are configurable via an unexported-for-now constant block in the generator; exposing them via config is a Phase 9 decision.

---

## 9. Thread safety (locked decision #6)

- `*Generator` is per-goroutine. Documented in the `New` GoDoc.
- No internal locking. No `GenerateConcurrent` helper.
- Lambda-level parallelism is the only parallelism: each SQS message spawns one Lambda invocation, each invocation creates one Generator, calls `Generate` once, and exits.
- Local dev (`/api/puzzles/generate`): one Generator per request, created inline in the handler.

**Grill point (c) outcome:** `WithRacing(n int)` is *not* added in v1. Step 11 must report per-(N,k) median and P99 latency for `Generate()`. If P99/median > 3 anywhere, the Step 11 handoff recommends `WithRacing` as a v2 Generator-level option — keeping it inside the package so the consumer still sees a single `Generate()` call. That recommendation is written down in the handoff document; it is *not* implemented in v1.

---

## 10. Mutation loop (locked decision #9, grill point b)

- **Default K = 50** swap mutations per attempt (`WithMaxMutations` adjusts).
- **Default neighborhood:** single-cell boundary swap between a region and a 4-adjacent region, where the swap preserves 4-connectivity of both regions and their seed-mark invariant.
- **Acceptance rule:** strict improvement in solved-cell count (spec-compliant).
- **Step 7 reports** the actual success rate at every (N, k). If Step 7 gate fails:
  - First mitigation: **Step 10** (solver-guided growth variant).
  - If Step 10 also fails: pick from the grill-point-(b) catalog (plateau acceptance, widened neighborhood, pair-swaps, random restart, k=3 beam) using Step 11 data.
  - No mitigation is committed in v1 beyond Step 10; all alternatives are Step-11 fodder.

---

## 11. Testing and benchmarking

- **Per-slice TDD** on all algorithmic modules (sampler, brute, deductive rules, region grower, mutator, classifier). Unit tests use Arrange-Act-Assert with explicit comments (CLAUDE.md convention).
- **Integration tests:** worker + LocalStack + DynamoDB round-trip; `generator.Puzzle` → `PuzzleRecord` translation fidelity.
- **Regression corpus:** `backend/internal/generator/testdata/puzzles/*.json`, ~10 hand-verified puzzles per tier per supported N per mode. Committed in the final slice.
- **Property tests:** deductive solution == brute solution on a 500-puzzle corpus; every rule fires at least once; every rule has a hand-crafted necessity fixture.
- **Benchmarks** (`go test -bench`): per the spec, plus per-N per-k matrix. Committed to `backend/internal/generator/bench/baseline.txt`.
- **Soak target:** `go test -tags=soak ./backend/internal/generator/...` — long-running, not run in CI.
- **Distribution test:** `-bench=BenchmarkGenerateOne` with histogram output at N=12 and (stretch) N=14 for both k. Step 11 handoff includes the histogram and the Expert yield number.

The pre-push hook (`.githooks/pre-push`) already runs `go test ./...`. Soak and the distribution test are manual-only.

---

## 12. Cutover / rollout

Because Phase 5 replaces the generator wholesale and changes the shape of `PuzzleRecord`, `ConfigRecord`, and `GenerationRequest`, the cutover is:

1. **Deploy new backend + new frontend + new LocalStack seed together** (single PR or a coordinated chain — see `tasks.md` slices).
2. **Drain the SQS queue** before deploy (in prod: set the consumer's concurrency to 0, wait for in-flight drain, deploy).
3. **Flush the pool** on first boot after deploy: a one-off admin action deletes all items with `PK != CONFIG`. Easier than in-place migration and there are zero production users today. Document as a cutover runbook step in the final slice's tasks.md entry.
4. **Re-seed configs** with the new CONFIG shape. LocalStack seed handles dev; for prod a one-off `PutConfig` loop or AWS CLI script writes the reduced configs.
5. **Trigger replenish.** New puzzles land in the pool with the new shape. KI-007 closes when Double combos can be `enabled=true` and produce puzzles within the 14-minute Lambda budget.

---

## 13. Open knobs (decide before or during implementation)

| Knob | Default | Where | Decision timing |
|------|---------|-------|-----------------|
| `N_min` | unknown | generator package constant | **Step 2 gate** (R-062) |
| `K` (max mutations) | 50 | generator package constant | Step 7 gate (R-064) — adjust if rate <80% |
| `WithMaxAttempts` default | 20 | generator package constant | Step 8 gate (R-065) — adjust per N if median attempts > 5 |
| Difficulty tier thresholds | as spec §4.4 | generator package constants | Step 11 (R-068) — may shift after histogram |
| Whether to keep `bench/baseline.txt` in-repo long-term | yes | convention | Retro at end of phase |
| `WithRacing` introduction | no | — | Step 11 handoff recommendation; Phase 6+ if data says so |

All of these are unexported constants in the generator package at first; they become `Option`s only if/when a caller actually needs them.

---

## 14. Pseudocode sketch: worker-side translation boundary

Full end-to-end flow in the worker after Phase 5 lands:

```go
// backend/internal/worker/generator.go
func (w *GeneratorWorker) processMessage(ctx context.Context, record *events.SQSMessage) error {
    var req queue.GenerationRequest
    if err := json.Unmarshal([]byte(record.Body), &req); err != nil {
        return fmt.Errorf("deserializing generation request: %w", err)
    }

    k := 1
    if req.Mode == handler.ModeDouble {
        k = 2
    }

    genCtx, cancel := context.WithTimeout(ctx, generationTimeout) // 14 min
    defer cancel()

    // One Generator per SQS message. No pool, no sharing, no racing.
    opts := []generator.Option{}
    if req.MaxAttempts > 0 {
        opts = append(opts, generator.WithMaxAttempts(req.MaxAttempts))
    }

    g, err := generator.New(req.Size, k, opts...)
    if err != nil {
        return fmt.Errorf("constructing generator (n=%d, k=%d): %w", req.Size, k, err)
    }

    start := time.Now()
    pz, err := g.Generate(genCtx)
    if err != nil {
        return fmt.Errorf("generating puzzle (size=%d, mode=%s): %w", req.Size, req.Mode, err)
    }
    durationMs := time.Since(start).Milliseconds()

    id, err := w.newUUID()
    if err != nil {
        return fmt.Errorf("generating puzzle ID: %w", err)
    }

    // Translate generator shape → repository shape.
    solution := make([][]bool, pz.N)
    for i := range solution {
        solution[i] = make([]bool, pz.N)
    }
    for _, m := range pz.Solution {
        solution[m.Row][m.Col] = true
    }

    rec := &repository.PuzzleRecord{
        GridSize:             req.Size,
        Mode:                 req.Mode,
        ID:                   id,
        Status:               "ready",
        Verdict:              "none",
        RegionMap:            pz.Regions,
        Solution:             solution,
        Deducible:            true,
        Difficulty:           int(pz.Difficulty),
        MaxTier:              pz.Metrics.MaxTier,
        TierCounts:           pz.Metrics.TierCounts,
        TraceLen:             pz.Metrics.TraceLen,
        GenerationDurationMs: durationMs,
        CreatedAt:            time.Now().UTC().Format(time.RFC3339),
    }

    if err := w.store.PutPuzzle(ctx, rec); err != nil {
        return fmt.Errorf("storing generated puzzle: %w", err)
    }

    log.Printf("generated puzzle %s (size=%d, mode=%s, difficulty=%d, duration=%dms)",
        id, req.Size, req.Mode, pz.Difficulty, durationMs)
    return nil
}
```

This is the **entire** footprint of the generator-consumer contract. Compare against the current `worker/generator.go`: the `BuildPipeline` / `GenerateConcurrent` indirection, the `RegionOpts` wiring, and the five strategy-matrix fields on the request all disappear.

---

## 15. Cross-references

- Spec algorithm: `input-spec.md` §4.
- Project adaptations: `locked-decisions.md` (all 9).
- Grill outcomes: `design-grill-summary.md` §§A, B, C.
- Implementation slices: `tasks.md` (R-062..R-06A).
- Capability deltas: `specs/puzzle-generation.md`, `specs/consumer-surface.md`, `specs/frontend-admin.md`.

---

## 16. Operations / CI

### 16.1 Existing CI coverage

The `.github/workflows/ci.yml` workflow already runs on every PR:

- `backend` job: `go build ./...`, `go test ./... -v`, `golangci-lint run`.
- `frontend` job: `npm run build`, `npm test`.
- `terraform-plan` job: terraform validate + fmt + plan.
- `security` job: gitleaks, govulncheck, npm audit.

This is the **release hot path** CI. It does NOT run the soak target or the debug-tagged deductive/brute cross-check, because those are long-running and not tagged by default. The existing jobs continue to gate merges unchanged.

### 16.2 New: optional generator re-check workflow

**File path:** `.github/workflows/generator-check.yml` (created as part of R-068 — see `tasks.md` slice `R-068.CI`).

**Trigger.**

```yaml
on:
  pull_request:
    branches: [main]
    paths:
      - 'backend/internal/generator/**'
      - '.github/workflows/generator-check.yml'
```

The path filter ensures the job only runs when generator code or the workflow itself changes. PRs touching only `handler/`, `worker/`, `frontend/`, infra, or docs do not trigger it. **INV-GEN-1 is the hard prerequisite for this path filter working** — if generator logic leaks into `handler/` or `worker/` the filter silently misses changes that ought to run the cross-check.

**Jobs.** A single job runs the soak target (which is where the full deductive/brute cross-check lives — see §7):

```yaml
jobs:
  generator-cross-check:
    name: Generator cross-check (optional)
    runs-on: ubuntu-latest
    continue-on-error: true
    timeout-minutes: 30
    defaults:
      run:
        working-directory: backend
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v6
        with:
          go-version: "1.26"
          cache-dependency-path: backend/go.sum
      - name: Run soak target
        run: go test -tags=soak -timeout=25m ./internal/generator/...
```

**Status semantics.**

- `continue-on-error: true` — a failure marks the check red but does NOT block PR merge.
- GitHub branch protection rules MUST NOT include this job in the list of required status checks. It is informational.
- `timeout-minutes: 30` bounds the wall-clock; the `go test -timeout=25m` bound sits under the GitHub timeout so a hang surfaces as a `go test` timeout failure rather than an ambiguous job cancellation.

**Why optional and non-blocking.**

- The soak target runs 10,000+ generations at multiple (N, k). Even on a generous CI runner it can take 10–25 minutes; occasional variance can push a legitimate run to 30 minutes.
- A red status on a long-running cross-check that sometimes flakes would train reviewers to ignore it; non-blocking keeps the signal honest.
- The release CI job is the merge gate; this job is the "did the generator change move the cross-check needle?" signal.

**What the job covers.**

- The deductive/brute match check (§7), because the soak target runs the debug-tagged cross-check.
- A much larger sample than the 500-puzzle property corpus in `go test`, so soundness bugs that escape the regular test run have a second chance to surface.
- Per-(N, k) smoke across the supported range.

**What the job does NOT cover.**

- Latency/throughput — that is `R-068`'s `bench/step11-handoff.md`, which is run locally, not in CI.
- The 1-hour distribution tests — too long for any CI cadence.

### 16.3 Cutover runbook

Covered in §12 and `tasks.md` R-069. No CI change required at cutover.
