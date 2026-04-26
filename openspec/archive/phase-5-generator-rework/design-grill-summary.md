# Design Grill Summary: Generator Rework (Phase 5)

## Context

Replace `backend/internal/generator` with a parameterized row-by-row sampler + tiered deductive solver + brute-uniqueness pipeline, per `input-spec.md`. The 9 locked decisions in `locked-decisions.md` were set by the project owner before this grill and are not re-litigated here. This document records the three grill points the owner explicitly requested, plus a handful of secondary challenges that surfaced during exploration.

## Final Design

- Generator package exposes `New(n, marksPerUnit, opts...)` and `(*Generator).Generate(ctx)`. Output shape is `Puzzle{N, MarksPerUnit, Regions, Solution, Difficulty, Metrics}`.
- Worker translates generator output into `repository.PuzzleRecord`, adding `ID`, `Status`, `Verdict`, `CreatedAt`, and expanding `Solution []Mark` into `Solution [][]bool`.
- The three-axis strategy matrix (pipeline × solver × regions) and `concurrency` are deleted from request/record/config/admin-UI.
- Difficulty is computed and stored but not surfaced to players or the replenish filter in v1.
- Deductive/brute cross-check runs in test builds only; a divergence is a hard failure.

## Grill Point A — Mode parameterization vs bitmask width

**Challenge:** at N=14 with k=2 each row has 2 marks in a `uint16` mask. The mask is fine, but does anything downstream (pair enumeration, region frontier counts, fixed-size-16 arrays) break? What about the N=15 stretch?

**Resolution:**

- `uint16` has 16 bits, so it holds N up to 16. `[16]`-sized arrays hold N up to 16 exactly. N=15 is safe; N=17 would break both.
- Pair enumeration must be **k-combinations**, not hardcoded `C(N, 2)`. For k=1 the enumerator yields singletons, for k=2 it yields pairs. Generalize — no loop body may contain a literal `2` for the k-dimension.
- Region frontier storage is cell-count-based, not N-bit-mask-based; it carries over from the spec unchanged.
- `RegCellsByRow [16][16]uint16` (regionID × row → col mask) is 196 `uint16` entries at N=14 — negligible.
- Popcount/trailing-zero operations must use `math/bits.OnesCount16` / `TrailingZeros16` exclusively. No manual shifts by literal 16 (`>> 16` on a `uint16` returns 0 — a latent foot-gun).

**Guardrails baked into the design:**

1. `New(n, k, opts)` validates `n in [N_min, 16]` and `k in {1, 2}`. Explicit rejection of out-of-range inputs.
2. Every k-loop uses the generator's `g.k`, never a literal `2`.
3. Mask width assumptions are asserted once at construction time; hot-path code trusts them.

**Status:** Resolved. No architectural change vs. the spec; the grill confirms the spec's `[16]`/`uint16` choice is sound for the locked N range (5..12 committed, up to 15 as a measurement stretch).

## Grill Point B — Mutation loop local-minima risk

**Challenge:** greedy "accept first swap that strictly increases solved-cell count" with K=50 will stall below threshold in degenerate cases. Identify ≥2 of them; propose mitigations without committing to one (Step 11 data decides).

**Degenerate cases identified:**

1. **Plateau trap.** Solved-cell count is a step function. Two adjacent boundary swaps X and Y are symmetric (strict increase on X, strict decrease on Y); the hill-climb accepts X, then the new state has no single-cell swap that strictly improves. Greedy stalls before K=50; attempt is discarded despite being close to feasible.

2. **Two-swap bottleneck.** Feasibility is 95% away with one region's seeds creating an ambiguity that only a **simultaneous** pair of non-adjacent boundary swaps resolves. No single-cell swap ever increases the count, so greedy cannot reach the required state.

3. **Flat-plateau regression.** A swap that keeps solved-count flat but shifts region shape such that a second swap *would* break through is rejected (strict-increase rule). The path to feasibility is blocked.

**Mitigation catalog (not committed — Step 11 picks):**

- **Random restart inside the attempt:** on no-improving-swap, roll back the last m swaps and try a different prefix.
- **Plateau acceptance (simulated-annealing-lite):** accept non-strict improvements with probability p, decaying. Directly addresses cases 1 and 3.
- **Widened neighborhood:** allow swaps at boundary distance ≤ 2. Larger neighborhood per step, higher cost, enriches search.
- **Pair-swaps:** two simultaneous single-cell swaps per step. Addresses case 2 directly. O(boundary²) neighborhood.
- **Small beam search (k=3):** keep top-3 neighbors each step rather than first-improvement.

**Decision ordering:**

1. Implement greedy + K=50 (spec-compliant) in Step 7.
2. Step 7 gate reports actual success rate at every (N, k). Gate target: ≥80% at (N=12, k=1) and (N=9, k=2) (locked decision #7).
3. If Step 7 fails, **Step 10** (solver-guided growth) is the first mitigation — it is already in the input spec and is a variant of the grower, not the mutator.
4. If Step 10 also fails, pick from the catalog based on Step 11 telemetry. Favor plateau acceptance + widened neighborhood before pair-swaps (latter is a big complexity jump).

**Status:** Resolved as an ordered-mitigation plan. No mutation mitigation ships in v1 beyond the spec-defined greedy. Step 7 and Step 11 telemetry drives any v2 change.

## Grill Point C — "No internal racing" vs generator variance

**Challenge:** locked decision #6 drops `GenerateConcurrent`. Risk: one unlucky seed exhausts `WithMaxAttempts` while a racing peer would finish instantly. What is the tell, and what is the smallest hedge to add back if Step 11 says we need it?

**The tell:**

- Per-(N, k), measure not just mean puzzles/sec but **P95/P99 `Generate()` latency**.
- If **P99/median > 3**, variance is high enough that racing would materially help.
- If P99/median ≤ 2, SQS-level parallelism across messages amortizes the variance; racing would add complexity without a visible win.

**Why racing may not matter at all:**

- SQS gives us message-level parallelism via concurrent Lambda invocations. Variance at the per-message level is smoothed across the fleet — each message is independent.
- Racing only helps when a **single message** is critical-path, e.g., the `/api/puzzles/generate` debug handler under its own timeout. That is not the replenish flow.

**Smallest hedge (if Step 11 data demands it):**

- Add `WithRacing(n int)` as a **Generator-level** option (default 1). Internally, `Generate(ctx)` fans out n goroutines, each with its own pre-allocated `solverState`, racing via `context.WithCancel` + `sync.Once`-gated return.
- Keep the per-goroutine pre-allocation contract: one Generator allocates n state blocks at construction, hands one to each racing goroutine. Consumer still sees `one Generator, one Generate() call`.
- **Not** a re-export of the old `GenerateConcurrent`. The old version lived outside the package and forced the consumer to reason about goroutines; the new hedge is inside the package, consumer-transparent, and determinism-friendly (each goroutine gets a seed derived from the base seed).

**Status:** Resolved as "do not add in v1; report the signal in Step 11." The locked decision stands. Step 11 handoff must include per-N per-k median and P99 latency; if variance ratio > 3 anywhere, the handoff explicitly recommends `WithRacing` for a v2 Phase.

## Secondary challenges raised during exploration

### S1 — Spec says regions are "4-connected"; what about single-cell regions at k=1?

At k=1, a 3x3 grid with 3 regions requires 3 cells per region on average. Single-cell regions violate the spec's `MinSize` idea but don't violate 4-connectivity (trivially connected). The current code enforces `MinSize=3` at k=1 and `MinSize=4` at k=2 via `RegionOpts`. The new generator does not expose `MinSize` (it is not in the spec's API). Instead, `MinSize = k + 1` is an internal constant, because the region must hold k seed marks plus at least one non-seed cell (to avoid the trivial single-cell case). Noted as an internal invariant; the new Generator documents this in `New`'s GoDoc.

**Status:** Internal invariant, no API impact. Documented in the design.

### S2 — Cutover: in-place migration vs. flush-and-refill?

The shape of `PuzzleRecord` changes (five fields removed, four added). DynamoDB is schemaless so old records remain queryable, but the app logic will see `Pipeline=""`, `Difficulty=0`, etc. on legacy records.

**Options:**

- (a) In-place migration script: walk every `PK != CONFIG` item, remove old fields, add default new fields.
- (b) Flush the pool: delete every `PK != CONFIG` item and let replenish refill.

**Decision:** (b). Zero production users today; the pool is dev/LocalStack data. Far simpler than (a). Documented as a cutover runbook step in the final slice's tasks.md entry.

**Status:** Resolved as flush-and-refill.

### S3 — `concurrency` and `regionVariance` in SQS messages in flight at deploy time

During deploy, there may be un-processed SQS messages carrying the old `GenerationRequest` shape. The new worker cannot decode them (strict JSON unmarshal will error on unknown fields only if we tag-enforce; current code ignores unknown fields, so decode will succeed but fields will be zero).

**Decision:** Drain the queue before deploy. In dev/LocalStack, `task dev:down:generator` stops the consumer; `task dev:up:generator` starts the new one. In prod, set consumer concurrency to 0 via Terraform, wait for in-flight to drain, then deploy. Documented in the cutover runbook.

**Status:** Resolved via operational procedure, not code.

### S4 — KI-013 (config payload duplicated 4 times) — does Phase 5 close it?

The config shape currently lives in four places: `repository.ConfigRecord`, `handler.configRequest`, `handler.configResponse`, `handler.buildConfigResponseMap`. Phase 5 shrinks the config (5 fewer fields), which makes the duplication *less* painful but does not unify it.

**Decision:** Not in Phase 5 scope. The shrink is a natural unblocker for a follow-up cleanup PR (call it a P5.5 cleanup slice if anyone wants it). Left as KI-013.

**Status:** Explicitly deferred.

### S5 — KI-015, KI-016 become moot

`AdminPage.tsx` `ConfigForm` has 9 primitive props, 4 of which are create-only; the pipeline/solver/regions/mode literals are string[] rather than typed unions. When Phase 5 deletes the strategy matrix fields from config, `ConfigForm` shrinks to (threshold, enabled) plus maybe (deducible). KI-015 and KI-016 resolve naturally — there's nothing left to refactor.

**Status:** Will close with Phase 5 merge. Note in the final slice's tasks.md.

### S6 — Where do `difficulty` / `maxTier` / `tierCounts` / `traceLen` live on the DynamoDB record?

Four new scalar-ish fields per puzzle. `traceLen` is int; `tierCounts` is `[]int` of length 5. DynamoDB stores lists natively. No schema migration needed — just start writing them on new records.

**Status:** Resolved. `PuzzleRecord` Go struct gains four fields; the `attributevalue.MarshalMap` call handles them without changes.

## Alignment check

Every resolved decision, restated:

1. **Bitmask safety:** `N in [N_min, 16]`, `k in {1, 2}`, no hardcoded 2 in k-loops, math/bits exclusively.
2. **Mutation loop:** greedy+strict+K=50 baseline; Step 10 is the first mitigation; Step 11 telemetry drives any further change.
3. **No internal racing:** `WithRacing` not added in v1; Step 11 must report P99/median latency; hedge documented for v2 if the ratio > 3.
4. **Cutover:** flush the pool; drain the queue before deploy.
5. **Deferred:** KI-013 (config DTO unification); difficulty surfacing; replenish difficulty filter; `WithRacing`.
6. **Closes naturally:** KI-007 (Double Queens slow), KI-015, KI-016.
7. **N_min interim = 5** (owner Q2). Package constant at R-063 time; R-063 probe may raise it, never lower below 5.
8. **MaxAttempts is a config knob** (owner Q4). Lives on `ConfigRecord` + `GenerationRequest`; generator exposes `WithMaxAttempts`; worker forwards.
9. **Generator package is self-contained — INV-GEN-1** (owner Q5, hard prerequisite for the CI job). All sampling, solving, growing, mutating, and classifying lives in `backend/internal/generator/`. Handler/worker do only validation + translation. Enforced via review-local sweep and `grep` verification in PG-01.
10. **Cross-check is test-only + optional CI job** (owner Q5). Deductive/brute match runs only in `//go:build debug`, property tests, and the soak target. A new path-filtered workflow (`.github/workflows/generator-check.yml`) re-runs the soak target on generator-touching PRs as a non-blocking informational status. Added to R-068 as a devops sub-slice.

## Deferred / Out of Scope

- Difficulty UI surfacing (R-034, Phase 9)
- Replenish difficulty filter (Phase 9)
- Verdict endpoint and UI (R-063, R-064, Phase 6)
- Admin-route auth (KI-009, R-075)
- In-place data migration of existing `PuzzleRecord`s (we flush instead)
- `WithRacing` option (v2 only if Step 11 data justifies it)
- Config DTO unification across `ConfigRecord` + handler types (KI-013)

## Constraints and Assumptions

- `uint16` and `[16]` arrays hold N up to 16. New puzzle sizes beyond 16 require a separate architecture.
- `marksPerUnit ∈ {1, 2}`. k=3+ is not part of the game design and is not supported.
- The pre-push hook (`.githooks/pre-push`) will run `go test ./...` and `golangci-lint run` on every push; CI runs the same. Soak and distribution tests are manual-only.
- Generator package has no imports from `model`, `repository`, or `queue`. Breaking this is a review-blocking finding.
- One Generator per goroutine — concurrent use is UB, not an error. The design trusts callers here.
