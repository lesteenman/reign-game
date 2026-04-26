# Phase 5 — Locked Decisions (project-side adaptations of the input spec)

The input spec in `input-spec.md` was written without visibility into our codebase. These are the nine decisions the project owner locked **before** OpenSpec explore begins. design-flow, backend-dev, and every later agent must honor them.

---

## 1. Mode parameterization — not Double-only

**Decision:** parameterize `marksPerUnit` throughout. Both `standard` (k=1) and `double` (k=2) are supported by the same pipeline. The input spec hardcodes "2 marks" — that is a spec bug. The same algorithm applies for k=1; the seed-pairing step in §4.3 Step A degenerates to "each mark is its own region seed" and rule tier logic treats "2" as a parameter, not a constant.

**Implications:**
- Solution sampler enumerates combinations of k columns per row instead of pairs.
- Rule tiers R2/R3 talk about "the line/region needs *k* more marks" and "has exactly *k* candidates" instead of "2".
- Region grower seeds are `N * k` marks grouped into `N` regions of `k` seeds each.
- Public API accepts `marksPerUnit` alongside `N` in `New(...)`.
- Benchmark matrix multiplies by k ∈ {1, 2}.

## 2. Drop-in replacement — delete the existing strategy matrix

**Decision:** the new generator replaces `backend/internal/generator` entirely. Delete `pipeline_region_first.go`, `pipeline_iterative.go`, `pipeline_constraint.go`, `regions_bfs.go`, `regions_random.go`, `regions_wfc.go`, `solver_backtrack.go`, `solver_propagation.go`, and the `strategy.go` abstraction. No sibling coexistence.

**During performance tuning only:** the old variants remain available in git history as reference for A/B comparisons. The handoff at Step 11 should include benchmarks against the old `constraint-aware + propagation + bfs` combination at the sizes where that combination actually terminates, so we can quantify the win.

**Consumer-side cleanup required:**
- `handler.GenerateParams` loses `Pipeline`, `Solver`, `Regions`, `RegionVariance`, `Concurrency` fields.
- `handler.BuildPipeline` is deleted.
- `handler.ParseGenerateParams` loses the corresponding query params.
- `repository.PuzzleRecord` loses `Pipeline`, `Solver`, `Regions`, `RegionVariance`, `Concurrency` fields. `Deducible` stays (true by construction; keep as a record-keeping bool).
- `repository.ConfigRecord` loses `Pipeline`, `Solver`, `Regions`, `RegionVariance`. Size+mode+threshold remain. (Pick-a-config logic collapses.)
- `queue.GenerationRequest` loses the same fields.
- `frontend/src/services/adminService.ts` types + `AdminPage.tsx` strategy dropdowns are removed (see KI-015, KI-016 — those become moot).
- LocalStack seed in `.localstack/init-aws.sh` updates to the simplified CONFIG shape.

## 3. N range — probe up to 12, measure higher as a stretch

**Decision:** target **N=6..12 for k=1**, **N=9..12 for k=2** as the committed ranges. These floors emerged from R-063's feasibility probe (`backend/internal/generator/v2/bench/n-feasibility.md` and `n-feasibility-deep.md`): N=5 k=1 has exactly 14 solutions (too narrow for long-term content variety), N=6 k=1 has 90; N=8 k=2 has exactly 2 solutions (content-dead), N=9 k=2 has 664+. Run Step 11 benchmarks up to N=14 out of curiosity; if they stay within a usable envelope (decision: what counts as "usable" is post-Step-11), expose them as desktop-only puzzle sizes. Do not commit to N>12 until the data is in.

**Performance:** the "<2s at N=12" number in the input spec is an estimate, not a contract. **Measure first, then decide what sizes ship.**

## 4. Difficulty — compute and store, no filter or UI yet

**Decision:** v1 computes `MaxTier`, `TierCounts`, `TraceLen`, and the `Difficulty` tier for every generated puzzle and stores them on `repository.PuzzleRecord`. **Do not** wire `WithDifficulty` filtering into the pool replenish flow in v1. **Do not** add a difficulty selector to the frontend in v1 (that is Phase 9, R-034).

The admin UI may surface the stored metrics read-only for observation. Generator API keeps the `WithDifficulty` option for future use, and for ad-hoc admin calls.

## 5. Output shape — generator returns spec shape, worker translates

**Decision:** `generator.Puzzle{N, Regions [][]int, Solution []Mark, Difficulty, Metrics}` is the generator's public return type. The worker (`backend/internal/worker/generator.go`) performs the translation to `model.Puzzle` / `repository.PuzzleRecord`, including:
- `Mark{Row,Col}` → `Solution [][]bool` (allocate NxN, set true for each mark)
- `Regions [][]int` → `RegionMap [][]int` (direct copy)
- Generator does not emit `ID`, `Mode`, `Status`, `Verdict`, `CreatedAt` — those are the worker's concern.

Keep the generator ignorant of our storage types. No imports from `model`, `repository`, or `queue` into the generator package.

## 6. Thread-safety — one Generator per SQS invocation, drop internal racing

**Decision:** `*Generator` is per-goroutine, as the spec dictates. In our system:
- The SQS worker creates exactly one `*Generator` per message and calls `Generate(ctx)` once. The `WithMaxAttempts` retry loop inside the Generator replaces the old `GenerateConcurrent` racing pattern.
- Lambda-level parallelism (multiple concurrent SQS invocations) is the only parallelism we use.
- Delete `generator.GenerateConcurrent`, the `Concurrency` field in `GenerationRequest` / `PuzzleRecord` / `ConfigRecord`, and the `concurrency` URL param in `/api/puzzles/generate`.
- The `/api/puzzles/generate` debug endpoint calls the generator directly with a single Generator instance.

If Step 11 shows single-Generator throughput is too variable at high N and racing genuinely helps, we reconsider — but the default is **no internal concurrency**.

## 7. Performance targets — measure, don't promise

**Decision:** the "<2s at N=12" and "<10s at N=14" numbers in §6.1 are estimates for catching pathological regressions, not commitments. Report actuals at every gate. Do not silently relax a failing gate; report it with a proposed mitigation (as the spec itself says).

The only hard gate we commit to externally is: Step 7 success rate ≥80% at N=12 for Standard (k=1) and N=9 for Double (k=2). If those fail, Step 10 (solver-guided growth) triggers; if *that* fails, escalate to the human.

## 8. Deductive/brute cross-check baked into the pipeline

**Decision:** after deductive solve returns `Solved`, assert (in test builds) that the reached solution matches the brute-solver's single solution. A divergence means a deductive rule is unsound — treat as a hard failure, not a retry trigger. In release builds, skip the comparison on the hot path but run it in property tests and the soak target.

## 9a. Region MinSize = 3 (R-067b)

**Decision:** every region in a generated puzzle must have at least 3 cells, at both k=1 and k=2. The grower enforces this during growth — while any region is under 3 cells, the frontier-pick step is restricted to cells adjacent to under-size regions and the assignment always goes to the smallest under-size region (smallest among under-3 candidates in the cheap variant; probe-scored among under-3 candidates in the solver-guided variant). If no under-size region has an unclaimed neighbor, the grow fails and the orchestrator resamples.

The maximum region size is **deliberately unbounded**. Culling overly large regions is a curation concern (see GAME_DESIGN.md "Planned Work") and lives outside the generator.

The earlier internal note of `MinSize = k + 1` (implied by the seed count) is superseded. 3 cells at k=1 is a puzzle-quality floor, not a structural one.

The rule is measurable: `TestGrowRegionsMinSize` samples 200 grows per committed (N, k) combo across both grower variants and fails on any under-3 region.

## 9. Honesty priors on the spec's guesses

**Decision:** K=50 mutation cap, 80% Step 7 success rate, 200-sample unit test count — accept as starting points. If measurements say otherwise, report and adjust. No externally-promised perf numbers until Step 11's data is in.

---

## OpenSpec scope note

R-061 (design-flow for the generator rework) is this proposal. Subsequent roadmap items (R-062 … R-06A) are implementation slices that design-flow *proposes* in its tasks.md output. The user confirms the slice split before backend-dev starts on any of them.
