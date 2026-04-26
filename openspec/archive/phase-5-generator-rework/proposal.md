# Phase 5: Generator Rework

## What

Replace the existing `backend/internal/generator` package — pipelines, solvers, and region strategies alike — with a single parameterized generator built around the row-by-row bitmask sampler, tiered deductive solver, and brute uniqueness check specified in `input-spec.md`. The new generator is a **drop-in replacement**: the three-axis strategy matrix (`pipeline` × `solver` × `regions`) goes away, and with it the corresponding fields on `GenerationRequest`, `PuzzleRecord`, `ConfigRecord`, and the admin UI.

The new generator exposes a narrow API — `generator.New(n, marksPerUnit, opts...)` returning a `*Generator` whose `Generate(ctx)` produces one puzzle at a time — and emits puzzles in its own shape (`generator.Puzzle{N, Regions [][]int, Solution []Mark, Difficulty, Metrics}`). The SQS worker translates that shape into `model.Puzzle` / `repository.PuzzleRecord` before persisting.

## Why

Three concrete problems the current strategy matrix creates:

1. **Double Queens is too slow to ship** (KI-007). The `iterative + propagation + bfs` combination chews through 12+ minutes to produce a deducible 7x7 Double. The replenish config has Double combos disabled-by-default. Without a new algorithm, Phase 9's difficulty rating and Phase 6's verdict flow don't have a fast enough pipeline to feed them.
2. **The strategy matrix surfaces implementation choices to users.** `ConfigRecord` stores `pipeline`, `solver`, `regions`, `regionVariance`, and `concurrency`; the admin UI lets an operator mix them. Every combination that is not the default is either redundant, broken, or untested. This is leaky abstraction — the user is choosing from a menu only the authors understand.
3. **No uniqueness guarantee.** The current solver's deducibility check counts solutions via the same propagation solver that *generated* the trace. If the solver has a soundness bug, both the trace and the uniqueness check agree. The new generator mandates a separate brute solver for uniqueness and cross-checks its answer against the deductive trace in test builds (locked decision #8).

The algorithm to replace the matrix is fixed in `input-spec.md` (§4). This proposal records the *nine adaptations* our codebase imposes on that spec (see `locked-decisions.md`) and shapes them into implementation slices.

## Summary of Locked Decisions

See `locked-decisions.md` for the authoritative statements. In one-line form:

1. **Mode parameterization.** `marksPerUnit` is a first-class parameter through sampler, solver, region grower, and classifier. The spec's "2" is treated as `k`.
2. **Drop-in replacement.** Delete the full strategy matrix (pipelines × solvers × regions) and its pass-through fields on request/record/config.
3. **N range.** Commit to N=6..12 for k=1 and N=9..12 for k=2 (R-063 feasibility data); measure N=13..14 as stretch in Step 11.
4. **Difficulty v1 is compute-and-store.** `MaxTier`, `TierCounts`, `TraceLen`, `Difficulty` land on `PuzzleRecord`; no frontend selector, no replenish filter.
5. **Output translation lives in the worker.** Generator returns spec shape; `worker/generator.go` translates to `model.Puzzle` / `repository.PuzzleRecord`.
6. **One Generator per SQS invocation.** Lambda-level parallelism only; `GenerateConcurrent` and the `concurrency` field are deleted.
7. **Performance numbers are measurements, not commitments.** Only hard external gate: Step 7 ≥80% at (N=12, k=1) and (N=9, k=2).
8. **Deductive/brute cross-check baked into test builds.** A divergence is a hard failure, not a retry trigger.
9. **Honesty on tuning knobs.** K=50, 80%, 200-sample count are starting points; report and adjust on measurement.

## Acceptance Criteria

A single cycle of Phase 5 is done when:

- **AC-1. Old generator removed.** The files listed in locked decision #2 no longer exist; `backend/internal/generator` contains only the new parameterized package. `go build ./...` passes.
- **AC-2. Parameterized API in place.** `generator.New(n, marksPerUnit, opts...)` returns a `*Generator`; `(*Generator).Generate(ctx)` returns a `generator.Puzzle`. Both k=1 and k=2 are covered by unit tests.
- **AC-3. Consumer surface updated.** `GenerationRequest`, `PuzzleRecord`, `ConfigRecord`, `GenerateParams` have lost `Pipeline`, `Solver`, `Regions`, `RegionVariance`, `Concurrency`. The admin UI, `adminService.ts`, and the LocalStack seed match the new shape.
- **AC-4. Worker translates correctly.** `worker/generator.go` builds a `repository.PuzzleRecord` from a `generator.Puzzle`, including `Difficulty`, `MaxTier`, `TierCounts`, `TraceLen`. Integration test (LocalStack + SQS + DynamoDB) proves round-trip.
- **AC-5. Uniqueness guaranteed.** Every puzzle emitted to the pool has exactly one solution, proven by the brute solver. The deductive/brute cross-check is test-only (behind `//go:build debug` and in property + soak tests); it does NOT run on the release hot path.
- **AC-6. Deducibility guaranteed.** Every puzzle emitted to the pool is solvable by the tiered deductive solver alone. No stalled-with-candidates puzzles reach the pool.
- **AC-7. Step 7 gate meets spec.** Cheap region-grower success rate ≥80% at (N=12, k=1) and (N=9, k=2). If not, Step 10 (solver-guided growth) is implemented and the gate is re-run.
- **AC-8. Step 11 handoff produced.** Benchmarks for every supported (N, k) combination, difficulty histograms, Expert yield, median + P99 `Generate()` latency, and a written v2 recommendation — all committed to `backend/internal/generator/bench/`.
- **AC-9. KI-007 resolvable.** Double Queens combos can be `enabled=true` in the LocalStack seed and the admin UI with generation that completes under the 14-minute SQS worker timeout. KI-007 closes at the end of this phase.
- **AC-10. NMin = 6 (k=1), 9 (k=2).** R-063's feasibility probe (and brute cross-check in `TestDeepFeasibility`) established exact solution counts at low N: N=5 k=1 has only 14 solutions, N=6 k=1 has 90; N=4..7 k=2 have 0, N=8 k=2 has exactly 2, N=9 k=2 has 664+. Package constant `NMin = 6` reflects the k=1 content-adequacy floor. The k=2 floor (9) is enforced by the orchestrator (R-065) via `sample_test.go`'s `minFeasibleN`. Evidence in `backend/internal/generator/bench/n-feasibility.md` and `n-feasibility-deep.md`.
- **AC-11. MaxAttempts is a config knob.** `MaxAttempts` lives on `ConfigRecord` and `GenerationRequest` as an optional `int` (0 means "use generator package default"). It is NOT hardcoded inside the generator; the generator exposes `WithMaxAttempts(n int)` as an `Option` and the worker forwards the config value.
- **AC-12. Generator package is self-contained (INV-GEN-1).** All generation logic lives under `backend/internal/generator/`. `backend/internal/handler/**` performs only validation and translation-to-domain-model; `backend/internal/worker/**` performs only SQS plumbing and translation-to-`repository.PuzzleRecord`. Neither imports any sampling, solving, region-growing, mutation, or classification logic that could equivalently live in the generator package. This is verified in review-local and (optionally) in the CI job described in AC-13.
- **AC-13. Optional generator CI job exists.** A path-filtered GitHub Actions workflow re-runs the deductive-vs-brute cross-check (soak tag) whenever a PR touches `backend/internal/generator/**`. The job is non-blocking (`continue-on-error: true`); it is informational, not a merge gate, because the soak target is long-running. The workflow file and timeout are described in `design.md` §16 and created as part of R-068.

## Scope

### In Scope

- Full replacement of `backend/internal/generator`.
- Updates to `backend/internal/handler/generate.go`, `handler/pipeline.go`, `handler/replenish.go`, `worker/generator.go`, `queue/publisher.go`, `repository/puzzle.go` needed to drop strategy-matrix fields and wire the new generator end-to-end.
- Updates to `frontend/src/services/adminService.ts` and `frontend/src/pages/AdminPage.tsx` to match the simplified config.
- LocalStack seed (`.localstack/init-aws.sh`) updated to the new CONFIG shape.
- Benchmarks, soak target, regression corpus as spec'd.

### Out of Scope

- Difficulty selector in the frontend (Phase 9, R-034).
- Replenish filter on difficulty (Phase 9).
- Verdict endpoint and UI (Phase 6, R-063/R-064).
- Auth on admin routes (KI-009, tied to R-075).
- Data migration of existing `PuzzleRecord`s. The pool is currently populated by the old generator; after Phase 5 we flush the pool (delete all `PK != CONFIG` items via a one-off LocalStack/DynamoDB admin command at cutover) and let replenish refill it. No in-place transform.

## References

- `input-spec.md` — external algorithm specification
- `locked-decisions.md` — project-side adaptations of the input spec
- `ROADMAP.md` — R-061 (this) and R-062..R-06A proposed in `tasks.md`
- `GLOSSARY.md` — Marker / Region / Region Map / Puzzle Solver / Deduction Chain
- `openspec/archive/phase-3-puzzle-pool/` — superseded pipeline/solver/regions capability specs
- `openspec/changes/phase-4-admin-pool-management/` — superseded CONFIG shape
