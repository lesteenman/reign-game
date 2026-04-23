# Phase 5: Implementation Tasks

Implementation slices for the generator rework. Each slice is one reviewable PR. Every slice follows the project pipeline (OpenSpec -> TDD -> security scan -> review-local -> PR), but for the non-visual backend work the UI/UX and frontend-design skills do NOT apply.

The slices group `input-spec.md`'s 12 steps into 7 PR-sized units. The input spec's step numbers are in parentheses next to each slice.

## Slices at a glance

```
Layer 0: R-062 (scaffolding) — new generator package alongside old; deletion happens in R-068
    |
Layer 1: R-063 (sampler + brute solver + N-probe)     Steps 2 + 3
    |
Layer 2: R-064 (deductive solver all tiers)           Steps 4 + 5
    |   uses R-063's brute for cross-check fixtures
    |
Layer 3: R-065 (region grower + mutator + orchestrator + classifier)   Steps 6 + 7 + 8 + 9
    |   produces a working generator.Puzzle end-to-end
    |
Layer 4: R-066 (optional solver-guided growth)        Step 10 — CONDITIONAL on R-065 gate
    |
Layer 5: R-067 (consumer cleanup + drop-in replace)   swap old package for new everywhere
    |   deletes old pipeline/solver/regions files
    |   updates request/record/config/handlers/worker/frontend/LocalStack
    |
Layer 5a: R-067a (mutator upgrade)                    lift (N=12, k=1) Step 7 rate back over 80%
Layer 5b: R-067b (region-size balance)                enforce min region size of 3
Layer 5c: R-067c (relift N=12 k=1 after min-size)     restore the gate under the tighter regime
    |
Layer 6: R-068 (bench + distribution + soak + corpus) Steps 11 + 12
    |
Layer 7: R-069 (cutover + KI-007 close)               runbook: drain queue, flush pool, re-seed
    |
(Optional) R-06A: post-cutover cleanup if review-local turns up residue
    |
Layer 8: R-06B (e2e fixed-database harness)           full-game plays against seeded pool
```

Hard dependency rule: R-067 cannot merge before R-065 gates pass, because R-067 deletes the old pipeline and if the new one has <80% success the pool stops replenishing.

## Status

| ID    | Slice                                          | Layer | Spec steps       | Status |
|-------|------------------------------------------------|-------|------------------|--------|
| R-062 | Generator package scaffold                     | 0     | Step 1           | [ ]    |
| R-063 | Sampler + brute solver + N-feasibility probe   | 1     | Step 2, Step 3   | [ ]    |
| R-064 | Deductive solver (Tiers 1-4)                   | 2     | Step 4, Step 5   | [ ]    |
| R-065 | Region grower + mutator + orchestrator + classifier | 3 | Step 6, 7, 8, 9 | [ ]    |
| R-066 | Solver-guided growth (conditional)             | 4     | Step 10          | [ ]    |
| R-067 | Consumer cleanup + drop-in replacement         | 5     | (cross-cutting)  | [ ]    |
| R-067a| Mutator upgrade (close N=12 k=1 gate)          | 5a    | (follow-up)      | [ ]    |
| R-067b| Region-size balance (min size = 3)             | 5b    | (quality)        | [ ]    |
| R-067c| Relift N=12 k=1 gate under min-size regime     | 5c    | (follow-up)      | [x]    |
| R-068 | Benchmarks + distribution + soak + corpus + optional generator CI re-check | 6 | Step 11, Step 12 | [ ]    |
| R-069 | Cutover + KI-007 close                         | 7     | (operational)    | [x]    |
| R-06A | Post-cutover cleanup                           | 7     | (contingent)     | [x]    |
| R-06B | E2E fixed-database harness                     | 8     | (verification)   | [x]    |

## Tasks

### R-062: Generator package scaffold

- **Roadmap:** R-062
- **Spec step:** Step 1 (Package skeleton and types)
- **Agent:** backend-dev
- **OpenSpec:** `specs/puzzle-generation.md` (PG-01, PG-02)

**Work**

- Create `backend/internal/generator/v2/` package alongside the existing `generator` package (so the old code still builds and the pool stays alive during R-063..R-066).
- Define public types from `design.md` §2: `Difficulty`, `Mark`, `Metrics`, `Puzzle`, `Option`, `Generator`, `New`, `(*Generator).Generate` (unimplemented, returns `errors.New("not implemented")`).
- Validate `n` in [1, 16], `k` in {1, 2} in `New`. Typed errors: `ErrNOutOfRange`, `ErrKUnsupported`.
- JSON round-trip test for `Puzzle` (ensures tag names match spec).

**Gate**

- `go build ./...` passes (including the not-yet-used `v2` package).
- `generator.Puzzle` serializes with the exact JSON field names in `design.md` §2.
- `New(n, k, opts)` rejects out-of-range inputs with the typed errors.

**Files touched**

- `backend/internal/generator/v2/generator.go` (new)
- `backend/internal/generator/v2/generator_test.go` (new)
- `backend/internal/generator/v2/doc.go` (new, package GoDoc)

**Dependencies:** none.

**Commit after completion.**

---

### R-063: Sampler + brute solver + N-feasibility probe

- **Roadmap:** R-063
- **Spec steps:** Step 2 (Solution sampler + N feasibility probe), Step 3 (Brute solver)
- **Agent:** backend-dev
- **OpenSpec:** `specs/puzzle-generation.md` (PG-03, PG-04, PG-05)

**Work**

- Implement row-by-row sampler in `v2/sample.go` with adjacency, forward-checking, k-combination enumeration per row (design.md §4.1, k-aware).
- Implement brute solver in `v2/brute.go`: full-puzzle backtracker returning up to maxSolutions (cap 2 for early exit). Pure function of a `RegionMap [][]int`.
- **N-feasibility probe:** a `TestMain`-style or `go test -run TestFeasibility` that runs the sampler for 5 seconds at each N in [4, 5, 6, 7, 8] for each k in {1, 2}, counts distinct solutions found, and prints the table.
- Declare `N_min = 5` as the interim package-level constant in `v2/generator.go` (proposal AC-10). The probe may recommend raising it; it may never lower it below 5. Record the final value in the PR description and — if it differs from 5 — update the constant and `specs/puzzle-generation.md` PG-04 verification.
- 200-sample unit tests per (N, k) for N in [N_min, 14] asserting row/col/k-count, adjacency, no duplicates.
- `BenchmarkSolutionSample/N=14/k=1` and `/k=2`.

**Gate**

- `BenchmarkSolutionSample/N=14` runs in <10ms/op for both k (if not, propose SAT-based sampler as mitigation — do not silently proceed).
- 200-sample tests pass at every N in [N_min, 14] for both k.
- Feasibility table committed to `backend/internal/generator/v2/bench/n-feasibility.md`; `N_min` proposed based on the data (report in the PR description, do not silently pick).
- Brute solver returns 1 for known-unique fixtures, 2 for ambiguous, 0 for unsatisfiable. <100ms at N=14 on the worst fixture.

**Files touched**

- `backend/internal/generator/v2/sample.go` (new)
- `backend/internal/generator/v2/sample_test.go` (new)
- `backend/internal/generator/v2/brute.go` (new)
- `backend/internal/generator/v2/brute_test.go` (new)
- `backend/internal/generator/v2/bench/n-feasibility.md` (new, committed data)
- `backend/internal/generator/v2/testdata/fixtures/` (new, hand-crafted brute fixtures)

**Dependencies:** R-062.

**Commit after completion.**

---

### R-064: Deductive solver (Tiers 1-4)

- **Roadmap:** R-064
- **Spec steps:** Step 4 (Tier 1), Step 5 (Tiers 2-4)
- **Agent:** backend-dev
- **OpenSpec:** `specs/puzzle-generation.md` (PG-06, PG-07)

**Work**

- Define `solverState` (design.md §3) in `v2/solver_state.go`. Pre-allocated buffers; value-copy clone via `*dst = *src`.
- Implement `v2/rules.go`: R1..R9 as `func(s *solverState) (changed bool, ev ruleEvent)` pure functions. All rule statements are k-parameterized (design.md §4.2) — no literal `2` in rule bodies.
- Fixed-point `Solve(s *solverState) Outcome` loop: apply rules in tier order, restart from Tier 1 on any change. Toggleable trace recording.
- For each rule R1..R9: (a) unit test with a hand-crafted minimal state proving exactly that rule fires; (b) necessity fixture where removing only that rule breaks the test.
- Cross-check fixture: on a 50-puzzle hand-crafted corpus, assert deductive solver's solution matches brute solver's single solution. Hard failure on divergence (per locked decision #8).

**Gate**

- All 9 rules pass their necessity fixture (removing the rule from the registry breaks the fixture).
- Deductive/brute cross-check passes on the 50-puzzle fixture corpus for both k=1 and k=2.
- `BenchmarkSolverFixedPoint/N=14` <50us/op at a "typical mid-generation state".
- Zero allocations in the solve loop after warm-up (verify via `-benchmem`).

**Files touched**

- `backend/internal/generator/v2/solver_state.go` (new)
- `backend/internal/generator/v2/solver.go` (new) — the fixed-point loop and `Outcome` enum
- `backend/internal/generator/v2/rules.go` (new) — R1..R9
- `backend/internal/generator/v2/rules_test.go` (new) — per-rule fixtures + necessity
- `backend/internal/generator/v2/solver_bench_test.go` (new)
- `backend/internal/generator/v2/testdata/corpus50/` (new, cross-check corpus)

**Dependencies:** R-063 (uses brute solver for cross-check).

**Commit after completion.**

---

### R-065: Region grower + mutator + orchestrator + classifier

- **Roadmap:** R-065
- **Spec steps:** Step 6 (Region grower cheap variant), Step 7 (Mutation loop), Step 8 (Orchestrator + output conversion), Step 9 (Classifier)
- **Agent:** backend-dev
- **OpenSpec:** `specs/puzzle-generation.md` (PG-08, PG-09, PG-10, PG-11)

**Work**

- **Pairer** (`v2/pair.go`): k=1 identity pairing; k=2 greedy nearest-neighbor Manhattan pairing. `pair(solution, n, k) [][]Mark`.
- **Grower cheap variant** (`v2/grower.go`): random-weighted frontier growth, inverse-size weighting, k-agnostic cell assignment. Respects `MinSize = k + 1` internal invariant.
- **Mutator** (`v2/mutate.go`): single-cell boundary swap, 4-connectivity + seed-mark invariant check, strict-improvement acceptance, up to K mutations. `WithMaxMutations` adjusts.
- **Orchestrator** (`v2/generator.go`, implementing `(*Generator).Generate`): sample -> pair -> grow -> mutate -> solve -> brute-unique -> classify -> convert. `WithMaxAttempts` caps attempts. Honors `ctx.Done()` between attempts and (best-effort) between mutations.
- **Classifier** (`v2/classify.go`): `MaxTier`, `TierCounts`, `TraceLen` from the trace; bucket per design.md §8 thresholds. `WithDifficulty` filters (retry-on-mismatch, respecting `WithMaxAttempts`).
- **Output conversion** (`v2/output.go`): `[16][16]int8 regionOf` -> `[][]int` of size [N][N]. Region IDs normalized to [0, N).

**Gate (Step 7 pattern)**

- >=80% of attempts produce a deducible+unique puzzle within K=50 mutations at (N=12, k=1) AND (N=9, k=2). Report actual rate at every (N, k) in the PR description, even the ones that pass. If either gate fails, proceed to R-066.
- `BenchmarkGenerateOne/N=12/k=1` <5s/op (initial — tighten in R-068).
- Output `Regions [][]int` has `len(regions)==N`, `len(regions[i])==N`, values in [0, N), exactly N distinct values.
- Classification is stable across runs for fixed-seed inputs.

**Files touched**

- `backend/internal/generator/v2/pair.go`, `pair_test.go` (new)
- `backend/internal/generator/v2/grower.go`, `grower_test.go` (new)
- `backend/internal/generator/v2/mutate.go`, `mutate_test.go` (new)
- `backend/internal/generator/v2/generator.go` (update Generate body)
- `backend/internal/generator/v2/generator_test.go` (add end-to-end tests)
- `backend/internal/generator/v2/classify.go`, `classify_test.go` (new)
- `backend/internal/generator/v2/output.go`, `output_test.go` (new)

**Dependencies:** R-064 (uses deductive solver); R-063 (uses brute solver and sampler).

**Commit after completion.**

---

### R-066: Solver-guided growth (CONDITIONAL)

- **Roadmap:** R-066
- **Spec step:** Step 10 (Solver-guided growth — conditional)
- **Agent:** backend-dev
- **OpenSpec:** `specs/puzzle-generation.md` (PG-12)

**Work** — only if R-065 gate fails (<80% at either (N=12, k=1) or (N=9, k=2))

- Implement the expensive grower variant from input-spec §4.3: for each candidate region assignment, clone the solver state, run the deductive solver on partial state, count solved cells; assign to the highest-scoring region.
- Disable trace recording during scoring (hot-loop allocation hygiene).
- Wire as a fallback inside the orchestrator: on repeated Step 7 stalls, switch to the expensive grower.

**Gate**

- Cheap + expensive combined success rate climbs to >=90% at the committed (N, k).
- Per-puzzle time penalty <3x vs. cheap grower alone.
- If this gate also fails, STOP and escalate to the human (per locked decision #7).

**Files touched**

- `backend/internal/generator/v2/grower_scored.go`, `grower_scored_test.go` (new)
- `backend/internal/generator/v2/generator.go` (update to switch on stall)

**Dependencies:** R-065.

**Commit after completion. Skip this slice entirely if R-065 passes — document the skip in R-067's PR description.**

---

### R-067: Consumer cleanup + drop-in replacement

- **Roadmap:** R-067
- **Spec step:** cross-cutting (no single step — this is the locked-decisions-#2 execution)
- **Agent:** backend-dev + frontend-dev (parallel within this slice), devops-engineer for LocalStack
- **OpenSpec:** `specs/consumer-surface.md` (CS-01..CS-08), `specs/frontend-admin.md` (FA-01..FA-03)

**Work**

**Sweep before deleting** — run these commands from the repo root and fix every match:

```
grep -rn "Pipeline\s*string\|Solver\s*string\|Regions\s*string\|RegionVariance\|Concurrency" backend/
grep -rn "pipeline\|solver\|regions\|regionVariance\|concurrency" frontend/src/
grep -rn "PipelineRegionFirst\|PipelineIterative\|PipelineConstraintAware\|SolverBacktrack\|SolverPropagation\|RegionsBFS\|RegionsWFC" backend/
grep -rn "BuildPipeline\|GenerateConcurrent" backend/ frontend/
```

**Backend deletions**

- Delete: `backend/internal/generator/pipeline_region_first.go`, `pipeline_iterative.go`, `pipeline_constraint.go`, `regions_bfs.go`, `regions_random.go`, `regions_wfc.go`, `solver_backtrack.go`, `solver_propagation.go`, `strategy.go`, `concurrent.go`, and their `_test.go` siblings.
- Move `backend/internal/generator/v2/*` up to `backend/internal/generator/`. The `v2` subdirectory vanishes.
- Delete: `handler/pipeline.go` — consolidate what remains (`ParseGenerateParams` without the strategy fields, `ModeStandard`/`ModeDouble` constants, a new `MarksPerUnitFromMode(mode) int`) into `handler/generate.go` or a new minimal `handler/params.go`.

**Backend modifications**

- `repository/puzzle.go` `PuzzleRecord`: remove Pipeline, Solver, Regions, RegionVariance, Concurrency. Add Difficulty (int), MaxTier (int), TierCounts ([]int), TraceLen (int).
- `repository/puzzle.go` `ConfigRecord`: remove Pipeline, Solver, Regions, RegionVariance, Concurrency. Add MaxAttempts (int, optional — 0 means package default).
- `queue/publisher.go` `GenerationRequest`: remove the same five; add optional MaxAttempts (int).
- `handler/generate.go`: rewrite to call `generator.New(size, k, opts...)` and `(*Generator).Generate(ctx)` directly. Translate `generator.Puzzle` to `model.Puzzle` in the handler (distinct from the worker, which translates to `repository.PuzzleRecord`).
- `handler/admin_config.go`: validation drops Pipeline/Solver/Regions/RegionVariance/Concurrency. Accepts Threshold, Enabled, Deducible, MaxAttempts.
- `worker/generator.go`: rewrite per design.md §14 pseudocode. Uses `generator.New` directly; builds `repository.PuzzleRecord` including the new difficulty fields.
- `handler/replenish.go`: remove the five fields from the `GenerationRequest` it constructs. Pass `MaxAttempts` from the config.

**Frontend modifications**

- `frontend/src/services/adminService.ts`: `ConfigData` collapses to `{ deducible, threshold, enabled, maxAttempts? }`. Remove `pipeline`, `solver`, `regions`, `regionVariance`, `concurrency` fields.
- `frontend/src/pages/AdminPage.tsx`: remove `PIPELINE_OPTIONS`, `SOLVER_OPTIONS`, `REGIONS_OPTIONS`, `regionVariance` input, `concurrency` input. `ConfigForm` shrinks. **KI-015 and KI-016 close with this change** — record that in the commit message and the ROADMAP update.
- Update `adminService.test.ts` and `AdminPage.test.tsx` to match.

**LocalStack / devops**

- `.localstack/init-aws.sh`: rewrite the CONFIG put-item blocks to the reduced shape (design.md §6.7).
- Verify `task dev:up` brings up the stack with the new shape and `task dev:logs:generator` shows no field-decode errors.

**Cross-cutting**

- `PROJECT_STRUCTURE.md`: update the generator subtree if files shift.
- Update `ROADMAP.md`: mark R-061 done (once R-069 lands), add R-062..R-069 entries with actual status, close KI-007, KI-015, KI-016 with this phase.

**Gate**

- `task build` passes.
- `task test` passes (all packages, all tiers).
- `golangci-lint run` clean.
- `npx tsc -b` clean in frontend.
- Integration test via `task dev:up` + LocalStack: submit a generation request, verify a `PuzzleRecord` with the new shape lands in DynamoDB and has non-empty `difficulty` / `maxTier` / `tierCounts`.
- Review-local's 4-agent review completes without CRITICAL/HIGH findings on this PR.
- Full-repo sweep of the removed identifiers returns zero hits (per the commands above).

**Files touched** — see design.md §6 for the authoritative delete-list.

**Dependencies:** R-065, plus R-066 if it ran.

**Commit after completion.**

---

### R-067a: Mutator upgrade (close N=12 k=1 gate)

- **Roadmap:** R-067a
- **Spec step:** (follow-up to Step 7)
- **Agent:** backend-dev
- **OpenSpec:** none (implementation follow-up); update `step7_test.go` comment on promotion.

**Work**

After the R-067-era mutator connectivity fix (PR #35), the (N=12, k=1) Step 7 rate dropped to 34% because many previously-accepted swaps produced orphaned cells. `step7_test.go` flipped that combo to `enforce=false`. This slice lifts the rate back over 80% and re-enforces the gate.

Start with the cheapest mutator change that moves the rate and stop when it does. Design-grill (b) catalogued four tactics; pick in this order:

1. Weighted plateau acceptance. Today the plateau phase accepts same-score swaps at p=0.5. Try a score-delta-aware rule: accept strict improvements always, equal-score with p=0.5, and small regressions (delta = -1) with p=0.1. Gives the walker a narrow escape from local optima without random wandering.
2. Widened neighborhoods. Today the scan sweeps Manhattan <= 2 around stalled cells, then a global pass. Try Manhattan <= 3 first, or start directly on a global pass once the first sweep finds nothing.
3. Pair-swaps. Swap two boundary cells at the same time, one in each direction, so the walker can cross a cut vertex in a single step. Connectivity check runs twice.
4. Random restart. When the budget is near exhausted, rewind the last N accepted swaps and try a different branch. Expensive per attempt, so only reach for this if 1 to 3 combined don't clear the gate.

Each tactic is reviewable on its own; if (1) hits the gate, stop there and note (2, 3, 4) as future levers.

**Gate**

- `TestStep7Gate` passes with `{n: 12, k: 1, enforce: true}`. Both committed combos back to LIVE.
- `TestGenerateProducesConnectedRegions` still passes (the tighter swap guard from #35 is preserved).
- `TestGenerateDeterministic` still passes (any new RNG consumption is done via `g.rng` so fixed seeds stay reproducible).
- `BenchmarkGenerateOne/N=12/k=1` stays under the 2 s/op budget.

**Files touched**

- `backend/internal/generator/mutate.go`
- `backend/internal/generator/mutate_test.go`
- `backend/internal/generator/step7_test.go` (promote enforce flag + drop the TODO comment)

**Dependencies:** R-067 + the connectivity fix (PR #35).

**Commit after completion.**

---

### R-067b: Region-size balance (min size = 3)

- **Roadmap:** R-067b
- **Spec step:** (quality follow-up)
- **Agent:** backend-dev
- **OpenSpec:** update `locked-decisions.md` MinSize note; add a PG-0x invariant if the new rule is large enough to warrant it.

**Work**

The cheap grower's inverse-size weighting is weak. Post-R-066 pool samples show region sizes like `[1, 4, 5, 6, 6, 8, 8, 12, 31]` at N=9 k=1. One-cell regions violate the MinSize invariant noted in `locked-decisions.md` (MinSize = k + 1 = 2 at k=1).

New rule: **every region must have at least 3 cells.** 3 at both k=1 and k=2. Unbounded maximum by design — the curation flow (out of scope for this phase, see "Planned Work" in GAME_DESIGN.md) rejects regions that are too large or ugly.

Enforce during growth, not post-hoc:

1. While any region has fewer than 3 cells, the frontier-pick step biases hard toward small regions. The cleanest rule: if a region has size < 3 and one of its frontier cells is in the candidate list, assign to the smallest such region. Only fall back to inverse-size weighting once every region has size >= 3.
2. If no under-size region has a frontier cell in reach (rare; can happen when bridging traps a k=2 seed pair), fail the attempt and let the orchestrator resample. This matches the existing "fail fast on bridge collision" pattern.

The solver-guided variant inherits the same priority rule — the probe still scores each candidate via the deductive solver, but only candidates that satisfy the min-size rule are probed.

**Gate**

- New `TestGrowRegionsMinSize` fails if any of 200 sampled grows produces a region with fewer than 3 cells.
- `TestGenerateProducesConnectedRegions` still passes.
- Step 7 rates logged for every combo. The min-size rule may cost a few percentage points; note the drop but do NOT relax the gate threshold. If a committed combo drops below its gate, open a follow-up rather than widen this slice.
- `BenchmarkGenerateOne` stays under 2 s/op at the committed ceiling.

**Files touched**

- `backend/internal/generator/grower.go`
- `backend/internal/generator/grower_scored.go`
- `backend/internal/generator/grower_test.go`
- `openspec/changes/phase-5-generator-rework/locked-decisions.md` (MinSize note)

**Dependencies:** R-067.

**Commit after completion.**

---

### R-067c: Relift (N=12, k=1) Step 7 gate under the min-size regime

- **Roadmap:** R-067c
- **Spec step:** (quality follow-up to R-067b)
- **Agent:** backend-dev
- **OpenSpec:** no spec change; this is a mutator-tuning slice.

**Work done**

Root-caused via `TestDiagPipelineStages` and a build-tagged exit-reason probe: at N=12 k=1 post-R-067b, 194/200 attempts exhausted the mutator budget (noSwap=0, budgetDone=194). The walker was spending too much budget on plateau-acceptance random walks that don't converge under the tighter boundary surface the min-size rule leaves behind. A pure budget increase (300 → 1000) scaled success rate sub-linearly and pushed wall time past the 2 s ceiling.

Two surgical changes:

1. `mutate.go` `tryOneSwap`: plateau acceptance cut from p=0.5 → p=0.1 (same-score) and p=0.1 → p=0.05 (one-cell regression). Greedier walker commits to improving swaps instead of diffusing.
2. `mutate.go` `tryOneSwap`: the "region must not empty" swap guard was `>= 1`; raised to `>= regionMinSize` so the mutator honors the R-067b floor (previously the mutator could undo what the grower built, which the Step 7 gate was partly hiding).

Step 7 rates on 100 × WithMaxAttempts=10:

| (N, k) | pre-R-067b | post-R-067b | R-067c |
|---|---|---|---|
| 12, 1 | 81% | 50% | **84%** |
| 12, 2 | 46% | 46% | **83%** |
| 11, 1 | 97% | 91% | 98% |
| 11, 2 | 89% | 89% | 100% |
| 10, 1 | 100% | 95% | 98% |
| 10, 2 | 98% | 98% | 100% |

`BenchmarkGenerateOne` at N=12 k=1: 1.05 s/op (under 2 s ceiling). N=12 k=2 at 2.37 s/op, not introduced here; R-068 owns that reduction.

**Outcome**

- N=12 k=1 gate back to `enforce: true`. Step 7 comment rewritten (no history accumulation — retro lesson 19).
- N=12 k=2 rose 37 pp as a bonus (the greedier walker helps there too), though it is not a committed gate.
- No regression on any other combo.

**Files touched**

- `backend/internal/generator/mutate.go` — plateau probabilities + min-size swap guard + docstring updates.
- `backend/internal/generator/step7_test.go` — (N=12, k=1) `enforce: true`; history-narrating rationale block rewritten.

**Dependencies:** R-067b.

**Status:** done.

---

### R-068: Benchmarks + distribution + soak + corpus + optional generator CI re-check

- **Roadmap:** R-068
- **Spec steps:** Step 11 (Profiling + benchmarking + distribution), Step 12 (Property tests, regression corpus, soak)
- **Agent:** backend-dev + tester + devops-engineer (for the CI workflow)
- **OpenSpec:** `specs/puzzle-generation.md` (PG-13, PG-14, PG-17)

**Work**

- Add all `go test -bench` benchmarks from input-spec §8 plus the per-k variants at every supported N. Commit baseline to `backend/internal/generator/bench/baseline.txt`.
- **Measure median AND P99 `Generate()` latency per (N, k)** — required by design-grill point (c). Write it to `bench/latency-distribution.md`.
- Run the distribution test for 1 hour at (N=12, k=1), (N=12, k=2), (N=14, k=1), (N=14, k=2) locally. Record difficulty histogram and Expert yield. Write to `bench/difficulty-distribution.md`.
- Apply any §6.2 optimizations not yet in place from §11 telemetry; re-benchmark.
- Hand-verify >=10 puzzles per difficulty tier per supported (N, k) and commit to `backend/internal/generator/testdata/puzzles/*.json`.
- Add the `go test -tags=soak` target running 10,000+ sample counts. Runs locally by default; also invoked by the new optional CI job (below).
- Add property tests: deductive solution == brute solution on a 500-puzzle corpus; every rule fires at least once.
- Write **`bench/step11-handoff.md`**: per-N throughput, per-N difficulty histograms, Expert yield, P99/median ratio, and a written recommendation on whether v2 difficulty-targeting OR `WithRacing` is needed.

**Work (optional generator CI re-check — devops sub-slice)**

Prerequisite: **INV-GEN-1** must hold (all generation logic lives under `backend/internal/generator/`). R-067's review-local sweep is where this is enforced; confirm before creating the workflow.

- Create `.github/workflows/generator-check.yml` per `design.md` §16.2.
- Trigger: `pull_request` with `paths: ['backend/internal/generator/**', '.github/workflows/generator-check.yml']`.
- Single job `generator-cross-check`:
  - `continue-on-error: true` (non-blocking, informational).
  - `timeout-minutes: 30`.
  - Checkout + setup-go (Go `1.26`, matching `ci.yml`).
  - Step: `go test -tags=soak -timeout=25m ./internal/generator/...` (working-directory: `backend`).
- Verify the workflow does NOT land in the branch-protection required-status-checks list. If the repo has branch protection configured, leave it untouched — new jobs are not required checks by default.
- Sanity-test the path filter by running `gh workflow run generator-check.yml` against a throwaway branch with and without a generator-directory change.

**Gate**

- `go test ./...` passes (including property tests).
- `go test -tags=soak ./backend/internal/generator/...` passes locally.
- `bench/step11-handoff.md` exists and contains every required metric.
- If P99/median > 3 at any supported (N, k), the handoff explicitly recommends `WithRacing` for a follow-up phase.
- If Expert yield at (N=12, k=1) is low enough to starve a difficulty-selector flow, the handoff explicitly recommends v2 difficulty-targeting.
- `.github/workflows/generator-check.yml` exists, is valid YAML, and triggers correctly on a path-filter smoke test.
- INV-GEN-1 sweep (`grep -rn 'uint16\|TrailingZeros16\|OnesCount16\|solverState\|bruteSolveAll' backend/internal/handler/ backend/internal/worker/ backend/internal/queue/ backend/internal/repository/`) returns zero results.

**Files touched**

- `backend/internal/generator/*_bench_test.go` (new, across modules)
- `backend/internal/generator/bench/baseline.txt` (new)
- `backend/internal/generator/bench/latency-distribution.md` (new)
- `backend/internal/generator/bench/difficulty-distribution.md` (new)
- `backend/internal/generator/bench/step11-handoff.md` (new)
- `backend/internal/generator/testdata/puzzles/**` (new)
- `backend/internal/generator/soak_test.go` (new, `//go:build soak`)
- `backend/internal/generator/property_test.go` (new)
- `.github/workflows/generator-check.yml` (new) — optional generator CI re-check

**Dependencies:** R-067.

**Commit after completion.**

---

### R-069: Cutover + KI-007 close

- **Roadmap:** R-069
- **Spec step:** (operational — no input-spec step)
- **Agent:** devops-engineer
- **OpenSpec:** runbook entry; no capability delta

**Work**

- `docs/runbooks/phase-5-cutover.md` — end-to-end cutover runbook covering the eight operational steps (drain, flush, deploy, re-seed, re-enable consumer, verify, KI close, project-structure refresh) with local-dev and prod-equivalent paths.
- `scripts/flush-pool.sh` — portable over LocalStack and real AWS (via `AWS_ENDPOINT_URL`). Refuses to run without `CONFIRM=YES`. Preserves CONFIG rows.
- `scripts/seed-configs.sh` — idempotent re-seed of the three CONFIG rows (7#standard, 9#standard, 9#double). Matches `.localstack/init-aws.sh`.
- `PROJECT_STRUCTURE.md` — adds `scripts/` and `docs/runbooks/` entries at the repository-root level.
- KI-007 close-out is already captured in ROADMAP.md (struck through with R-062..R-067 references); no additional change needed.

**Deferred to R-06A** (frontend refactor scope, not operational cutover):

- KI-015 (ConfigForm prop sprawl)
- KI-016 (AdminPage MODE_OPTIONS typed-const tightening)

Both were originally listed under R-069 in an earlier version of this slice but are cleanup rather than cutover work. They belong in R-06A.

**Gate**

- `./scripts/flush-pool.sh` and `./scripts/seed-configs.sh` exist, are executable, and pass `bash -n`.
- Runbook covers all eight steps with both dev and prod paths.
- KI-007 struck through in ROADMAP.md.
- Step 11 handoff (`bench/step11-handoff.md`) is the referenced source for post-cutover concurrency / yield decisions.

**Files touched**

- `scripts/flush-pool.sh` (new)
- `scripts/seed-configs.sh` (new)
- `docs/runbooks/phase-5-cutover.md` (new)
- `PROJECT_STRUCTURE.md` (scripts/ + docs/runbooks/ entries)

**Dependencies:** R-068.

**Commit after completion.**

---

### R-06A: Post-cutover cleanup + dynamic mode buttons

- **Roadmap:** R-06A
- **Agent:** backend-dev + frontend-dev
- **OpenSpec:** `design-grill-r06a-r06b.md` for decision record

**Work** — scope grew during design-grill (2026-04-22) from pure frontend cleanup to a full-stack slice driving the landing page from actual enabled combos.

**Backend:**

- **New public endpoint `GET /api/config/modes`** returning `[{size, mode}]` for every `enabled=true` CONFIG row. No thresholds, no ready counts. Keeps free-user traffic off `/api/admin/*`.
- **KI-013 / KI-015 type split.** `repository.ConfigRecord` is the domain shape. The handler layer owns explicit DTOs — `ConfigView` for reads, `ConfigCreateRequest` for POST, `ConfigUpdateRequest` for PUT — with mapping functions at the boundary. Drops the hand-rolled `buildConfigResponseMap` and the four-way config redeclaration.

**Frontend:**

- **Dynamic mode buttons.** `PuzzleSelector` reads `/api/config/modes` on mount and renders one button per enabled combo. Fallback UI when the endpoint returns zero combos: "no puzzles available, try again."
- **KI-015 component split.** `ConfigForm` splits into `EditConfigForm` + `CreateConfigForm` sharing a `ConfigFields` child. Each consumes its matching DTO.
- **KI-016.** `MODE_OPTIONS` becomes a typed `as const` union exported from `adminService.ts` so invalid literals are compile errors.

**Docs:**

- **`GLOSSARY.md`** new "Testing" section with `End-to-end test` and `Integration test` entries. Records the project-wide terminology R-06B will use.
- **`PROJECT_STRUCTURE.md` full refresh.** Backend generator subtree is wildly stale (claims `generator.go`, `solver.go`, `difficulty.go`, `region.go` — none match reality). Frontend + infra sections also visited in the same sweep.
- **`ROADMAP.md`.** Strike through KI-013, KI-015, KI-016 with R-06A as the close-out.

**Gate**

- `GET /api/config/modes` served by the backend with tests covering enabled-only filtering and the empty-set response.
- Landing page renders dynamic buttons from the endpoint; mocked-backend Playwright suite still passes on the new behavior.
- KI-013, KI-015, KI-016 struck through in ROADMAP.md.
- `PROJECT_STRUCTURE.md` backend tree matches the actual `backend/internal/generator/` file list.
- `GLOSSARY.md` Testing section exists and both terms are defined.
- Zero outstanding review-local CRITICAL/HIGH.

**Dependencies:** R-069.

**Commit after completion.**

---

### R-06B: E2E infrastructure + minimum viable coverage

- **Roadmap:** R-06B
- **Spec step:** (verification — full-stack)
- **Agent:** tester + backend-dev + frontend-dev
- **OpenSpec:** `design-grill-r06a-r06b.md`

**Work**

Today's Playwright suite lives under `frontend/e2e/` but uses `page.route` to mock the backend. Under the R-06A glossary, that's actually an **integration test**. This slice sets up the real e2e infrastructure and ships two validating tests. Broader coverage goes to ROADMAP.md Phase 10+ as "R-06B follow-up."

**Infrastructure:**

- **LocalStack isolation.** `init-aws.sh` creates `puzzle-pool-e2e` alongside `puzzle-pool`. Same schema, same CONFIG seeds. Dev pool is never touched.
- **Second backend.** New `task e2e:up` starts a backend instance on `:5182` with `PUZZLE_TABLE_NAME=puzzle-pool-e2e`. Matching `task e2e:down` + `task e2e:status`.
- **Fixture generator.** New `backend/cmd/genfixtures/main.go` generates deterministic puzzle fixtures via `generator.Generate(WithSeed(...))` and writes JSON to `frontend/playwright/e2e/fixtures/puzzles/`. Re-running produces byte-identical output.
- **Seed task.** `task e2e:seed` writes the committed fixture rows to `puzzle-pool-e2e` via `aws dynamodb put-item`. Idempotent.
- **Playwright rename.** `frontend/e2e/` → `frontend/playwright/` with `integration/` and `e2e/` subfolders. Existing `grid-interaction.spec.ts` moves to `integration/`. `playwright.config.ts` defines two projects (`integration`, `e2e`) with matching `testMatch` paths.
- **README.** `frontend/playwright/README.md` documents which suite is which and the Task-dependency flow for `e2e`.

**Minimum test set (ships in this slice):**

1. **`play-to-completion.spec.ts`** — seed a Standard 5×5 puzzle, click Play, place marks, exercise undo mid-play, complete. Validates the full pipeline + undo in one flow.
2. **`dynamic-modes.spec.ts`** — seed CONFIG rows where `9#double` is `enabled=false`, confirm the button doesn't render. Validates R-06A's new `/api/config/modes` endpoint wiring.

**Gate**

- `task e2e:up` + `task e2e:seed` + `npm run test:playwright --project=e2e` passes locally against committed fixtures.
- `npm run test:playwright --project=integration` still passes (migrated spec unchanged except for imports).
- `frontend/playwright/README.md` exists and explains the two projects.
- Remaining coverage (Double 9×9 play-through, serve-lifecycle, pool-empty UI, generation-path tests) documented in ROADMAP.md Phase 10+ under "R-06B follow-up — full e2e coverage."

**Files touched**

- `.localstack/init-aws.sh` (add `puzzle-pool-e2e`)
- `Taskfile.yml` (`e2e:up`, `e2e:down`, `e2e:status`, `e2e:seed`)
- `backend/cmd/genfixtures/main.go` (new)
- `frontend/playwright.config.ts` (two projects)
- `frontend/playwright/integration/grid-interaction.spec.ts` (moved)
- `frontend/playwright/e2e/play-to-completion.spec.ts` (new)
- `frontend/playwright/e2e/dynamic-modes.spec.ts` (new)
- `frontend/playwright/e2e/fixtures/puzzles/*.json` (new, generated)
- `frontend/playwright/README.md` (new)
- `ROADMAP.md` (add R-06B follow-up bullet)

**Dependencies:** R-06A.

**Commit after completion.**

---

## Execution Summary

| Layer | Slices | Agents | Parallel? |
|-------|--------|--------|-----------|
| 0 | R-062 | backend-dev | — |
| 1 | R-063 | backend-dev | — |
| 2 | R-064 | backend-dev | depends on R-063 |
| 3 | R-065 | backend-dev | depends on R-064 |
| 4 | R-066 (conditional) | backend-dev | depends on R-065 gate |
| 5 | R-067 | backend-dev + frontend-dev + devops-engineer | backend+frontend+devops run in parallel within the PR |
| 5a | R-067a | backend-dev | depends on R-067 + PR #35 |
| 5b | R-067b | backend-dev | depends on R-067 |
| 6 | R-068 | backend-dev + tester + devops-engineer | depends on R-067a and R-067b |
| 7 | R-069 | devops-engineer | depends on R-068 |
| 7+ | R-06A | TBD | contingent |
| 8 | R-06B | tester + backend-dev + frontend-dev | depends on R-06A (or R-069 if A is skipped) |

Every slice lands as its own PR. The `v2` subdirectory is a scaffolding convenience that disappears when R-067 swaps it into place — the `/opsx:apply` orchestrator should plan for R-067 as the most disruptive PR (largest delete-list, cross-package, frontend + LocalStack changes).

## Out-of-scope for this phase (captured so we don't drift)

- Difficulty selector in the frontend (R-034, Phase 9).
- Verdict endpoint and UI (R-063 in roadmap naming collision — that is the *Phase 6* R-063, not this R-063; see ROADMAP.md for disambiguation).
- Auth on admin routes (KI-009 / R-075).
- `WithRacing` or v2 difficulty-targeting — only triggered by Step 11 handoff recommendations.
- KI-013 (config DTO duplication) — Phase 5 shrinks the config so this is less painful, but unifying the DTO is a Phase 5.x cleanup not covered here.

---

**Roadmap-ID decision (owner-confirmed):** Phase 5 keeps the contiguous range **R-062..R-06A** (directly following the existing R-061 design-flow item). When Phase 6 (Verdict) begins, its originally-listed `R-063`/`R-064` entries will be renumbered to **R-06B** and **R-06C** respectively. No code or spec inside Phase 5 depends on Phase 6's numbers, so the renumber is a documentation-only change at the start of Phase 6. This is a resolved decision, not an open flag.
