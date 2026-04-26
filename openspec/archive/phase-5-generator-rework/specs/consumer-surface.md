# Capability Delta: Consumer Surface (backend)

This is a **delta spec**. It amends capabilities defined in `openspec/archive/phase-3-puzzle-pool/specs/backend.md` and `openspec/changes/phase-4-admin-pool-management/specs/backend.md`. When Phase 5 lands, both of those specs are superseded for the strategy-matrix fields listed below.

Section IDs (CS-XX) are referenced from `tasks.md`.

---

## CS-01: PuzzleRecord schema change

**Supersedes:** Phase 3 `BE-05` (PuzzleRecord definition), Phase 4 implicit assumptions.

**Removed fields** (Phase 5 deletion):

- `Pipeline string`
- `Solver string`
- `Regions string`
- `RegionVariance float64`
- `Concurrency int`

**Added fields** (Phase 5 addition):

- `Difficulty int` — generator-assigned tier, 0 (unknown) .. 4 (Expert).
- `MaxTier int` — highest rule tier fired during deductive solve.
- `TierCounts []int` — firings per tier, `[]int` of length 5 (index 0 unused).
- `TraceLen int` — total rule firings.

**Retained fields:** `GridSize`, `Mode`, `ID`, `Status`, `Verdict`, `RegionMap`, `Solution`, `Deducible` (always `true` by construction), `GenerationDurationMs`, `CreatedAt`, `ServedAt`.

**Data migration.** None. At cutover, all items with `PK != CONFIG` are deleted and replenish refills the pool (see R-069).

**Verification.** `PuzzleRecord` struct matches this shape. Worker writes the four new fields on every new puzzle. An end-to-end integration test against LocalStack proves round-trip fidelity.

---

## CS-02: ConfigRecord schema change

**Supersedes:** Phase 4 `BE-10` (ConfigRecord definition).

**Removed fields:**

- `Pipeline string`
- `Solver string`
- `Regions string`
- `RegionVariance float64`
- `Concurrency int`

**Added fields:**

- `MaxAttempts int` — optional; 0 means "use generator package default".

**Retained fields:** `Size`, `Mode`, `Deducible`, `Threshold`, `Enabled`.

**Why.** Locked decision #2 — strategy matrix disappears; generator picks its own strategy.

**Verification.** `ConfigRecord` struct matches this shape. `GetAllConfigs`, `GetConfig`, `PutConfig`, `CreateConfig` round-trip the new shape. LocalStack init seeds the new shape.

---

## CS-03: GenerationRequest schema change

**Supersedes:** Phase 3 `BE-06` (GenerationRequest JSON).

**Removed fields:**

- `Pipeline string`
- `Solver string`
- `Regions string`
- `RegionVariance float64`
- `Concurrency int`

**Added fields:**

- `MaxAttempts int` — optional; mirrors `ConfigRecord.MaxAttempts`.

**Retained fields:** `Size`, `Mode`, `Deducible`.

**Backward compatibility.** None. In-flight SQS messages carrying the removed fields are drained before deploy (R-069 cutover step 1).

**Verification.** `GenerationRequest` struct matches. Publisher serializes new shape; worker deserializes new shape.

---

## CS-04: Remove `BuildPipeline`, `GenerateConcurrent`, and the `pipeline` query param

**Supersedes:** Phase 3 debug endpoint contract.

**Changes:**

- `handler.BuildPipeline` is deleted entirely.
- `generator.GenerateConcurrent` is deleted entirely.
- `GET /api/puzzles/generate` query params lose: `pipeline`, `solver`, `regions`, `regionVariance`, `concurrency`.
- `GET /api/puzzles/generate` query params retain: `size`, `mode`, `deducible`.
- `GET /api/puzzles/generate` calls `generator.New(size, k, opts...)` and `(*Generator).Generate(ctx)` directly.

**Why.** Locked decisions #2 and #6.

**Verification.** `grep -r "BuildPipeline\|GenerateConcurrent" backend/` returns zero results. `grep -r "pipeline\s*:\|solver\s*:\|regions\s*:" frontend/src/` returns zero results (after Phase 5 frontend cleanup lands).

---

## CS-05: Worker translates generator shape to storage shape

**New requirement.**

The SQS worker (`backend/internal/worker/generator.go`) is the sole translation point from `generator.Puzzle` to `repository.PuzzleRecord`. The pseudocode in `design.md` §14 is authoritative.

Translation responsibilities:

- Convert `Solution []Mark` to `Solution [][]bool` of size N×N.
- Direct-copy `Regions [][]int` to `RegionMap [][]int`.
- Assign `ID` (UUID v4), `Status="ready"`, `Verdict="none"`, `Deducible=true`, `CreatedAt=time.Now().UTC()`.
- Copy `Difficulty`, `MaxTier`, `TierCounts`, `TraceLen` from `Metrics`.
- Record `GenerationDurationMs` as wall-clock time of the `(*Generator).Generate` call.

**Why.** Locked decision #5 — generator stays ignorant of storage types.

**Verification.** `backend/internal/worker/generator.go` imports `generator` and `repository` but does not import `queue` into the translation logic. Integration test: SQS message in -> `PuzzleRecord` in DynamoDB with all new fields populated.

---

## CS-06: Replenish handler reads reduced config

**Supersedes:** Phase 4 `BE-13` (Replenish Refactor) for the strategy-matrix fields.

**Changes:**

- `ReplenishHandler` no longer copies `Pipeline`, `Solver`, `Regions`, `RegionVariance`, `Concurrency` from config into `GenerationRequest`.
- Copies `Size`, `Mode`, `Deducible`, `MaxAttempts` (new).
- Uses `config.Threshold` for pool level check (unchanged from Phase 4).
- Skips `enabled=false` configs (unchanged from Phase 4).

**Verification.** `backend/internal/handler/replenish.go` only references the six retained `ConfigRecord` fields (Size, Mode, Deducible, Threshold, Enabled, MaxAttempts). Existing replenish tests updated to match.

---

## CS-07: Admin config validation reduced

**Supersedes:** Phase 4 `BE-12` (Admin Config Handlers) for the strategy-matrix validation.

**Removed validations** in `handler/admin_config.go` and `handler/pipeline.go` (`ParseGenerateParams`):

- `pipeline` in {region-first, iterative, constraint-aware}
- `solver` in {backtrack, propagation}
- `regions` in {bfs, wfc}
- `regionVariance` in [0.0, 1.0] (and NaN/Inf check)
- `concurrency` in [1, 8]

**Retained validations:**

- `size` in [3, 15]
- `mode` in {standard, double}
- `deducible` is boolean
- `threshold` >= 1
- `enabled` is boolean

**Added validation:**

- `maxAttempts` >= 0 (0 means default; positive means override). Reject negative.

**Why.** Locked decision #2 plus the natural closure of KI-013 duplication — five validators vanish.

**Verification.** Admin config handler tests updated. All PUT/POST requests carrying the removed fields are accepted but the fields are silently ignored (backward-tolerant for bookmarked admin UI sessions during deploy).

---

## CS-08: File deletion list

**New requirement.** The following files MUST NOT exist after Phase 5 lands:

- `backend/internal/generator/pipeline_region_first.go`
- `backend/internal/generator/pipeline_iterative.go`
- `backend/internal/generator/pipeline_constraint.go`
- `backend/internal/generator/pipeline_test.go`
- `backend/internal/generator/pipeline_bench_test.go`
- `backend/internal/generator/regions_bfs.go`
- `backend/internal/generator/regions_random.go`
- `backend/internal/generator/regions_wfc.go`
- `backend/internal/generator/regions_wfc_test.go`
- `backend/internal/generator/solver_backtrack.go`
- `backend/internal/generator/solver_propagation.go`
- `backend/internal/generator/solver_propagation_test.go`
- `backend/internal/generator/strategy.go`
- `backend/internal/generator/concurrent.go`
- `backend/internal/generator/concurrent_test.go`
- `backend/internal/generator/benchmarks_test.go`
- `backend/internal/generator/deducibility_test.go`
- `backend/internal/generator/generator_bench_test.go` (the OLD one; the new one replaces it)
- `backend/internal/generator/region.go` (the OLD one)
- `backend/internal/generator/region_test.go` (the OLD one)
- `backend/internal/generator/solver.go` (the OLD one)
- `backend/internal/generator/solver_test.go` (the OLD one)
- `backend/internal/generator/generator.go` (the OLD one)
- `backend/internal/generator/generator_test.go` (the OLD one)
- `backend/internal/handler/pipeline.go`

**Verification.** `ls backend/internal/generator/` returns only Phase-5 files (from R-067's move of `v2/*` up). `ls backend/internal/handler/pipeline.go` returns "no such file."

The Phase-5 generator directory should contain roughly: `generator.go`, `sample.go`, `brute.go`, `solver_state.go`, `solver.go`, `rules.go`, `pair.go`, `grower.go`, `mutate.go`, `classify.go`, `output.go`, their `_test.go` siblings, `doc.go`, `soak_test.go`, `property_test.go`, `bench/`, `testdata/`. Plus optionally `grower_scored.go` if R-066 ran.
