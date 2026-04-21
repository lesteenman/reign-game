# Step 11 Handoff (R-068d)

Reads the three measurement artifacts committed by the R-068 slices and makes the two decisions input-spec §10 and §11 flag as Step-11-gated:

1. Does the consumer need `WithRacing` to stay under a P99 latency budget?
2. Does it need v2 difficulty-targeting to keep the Expert tier from starving?

## Measurement sources

| Artifact | Committed file | How to refresh |
|---|---|---|
| Throughput baseline | `bench/baseline.txt` | `task bench:backend:baseline` |
| Latency distribution | `bench/latency-distribution.md` | `task bench:backend:latency` |
| Difficulty distribution | `bench/difficulty-distribution.md` | `task bench:backend:distribution` |

## 1. WithRacing recommendation

**Trigger.** p99/median > 3× at any committed (N, k).

**Evidence.** Every committed bucket is past the trigger, some by a lot:

| (N, k) | median | p99 | ratio |
|---|---|---|---|
| 6, 1 | 994 µs | 9.17 ms | 9.2× |
| 9, 1 | 57 ms | 450 ms | 7.8× |
| 12, 1 | 1.49 s | 7.07 s | 4.8× |
| 9, 2 | 2.73 ms | 200 ms | **73.3×** |
| 12, 2 | 1.06 s | 6.47 s | 6.1× |

`(9, 2)` is the pathological case — median is a few ms, tail is hundreds of ms. The same slow-attempt-blocks-fast-attempt shape appears at every N. At `(12, 1)` median-7× means 10 s of wait for a p99 call while 50 median calls would have already finished.

**Recommendation — ship `WithRacing` as part of the consumer cutover.** A racing wrapper issues M concurrent `Generate` calls, returns the first to finish, and cancels the rest. M = 2–3 captures most of the tail-cut at minor Generator-memory cost.

The wrapper itself is a follow-up slice post-R-068. The thread-safety contract in §9 (one Generator per goroutine) already supports it; implementation is one file.

## 2. v2 difficulty-targeting recommendation

**Trigger.** Expert yield at the committed ceiling falls below `consumer_demand / retry_budget` — at which point the `WithDifficulty(Expert)` retry loop dominates wall-time.

**Evidence.** Expert yield is **zero** at every bucket, and Easy yield is zero too:

| (N, k) | attempts | ok | easy | medium | hard | expert |
|---|---|---|---|---|---|---|
| 12, 1 | 512 | 505 | 0 | 279 | 226 | **0** |
| 12, 2 | 587 | 564 | 0 | 2 | 562 | **0** |
| 14, 1 | 104 | 51 | 0 | 28 | 23 | **0** |
| 14, 2 | 67 | 26 | 0 | 1 | 25 | **0** |

This is not a statistical artifact. The classifier routes a puzzle to Expert only when a Tier-4 rule fires (R8 or R9). Both rules are listed in `propertyCorpusKnownDead` (see `property_test.go`): they never fire across the R-068b 500-puzzle corpus either. So:

**Expert is unreachable via generation in the current state.** `WithRacing` does not help; retry-until-match cannot succeed. The consumer must not ship `WithDifficulty(Expert)` until the Tier-4 classifier path is fixed.

**Recommendation — R-068z (dead-rule investigation) must land before difficulty-targeting is even evaluated.** That slice resolves whether R6/R8/R9 are dead-and-retireable (classifier's tier max should drop to 3) or reachable-but-rare (generator needs a targeted mode). Only after R-068z does "does the consumer need v2 difficulty-targeting?" become answerable.

In the meantime, the classifier produces a binary Medium/Hard split at N=12 k=1 (55/45) and an effectively-Hard-only output at N=12 k=2. Coarser than the spec assumes, but not itself a bug — it is a consequence of the dead-rule finding.

## 3. Pool refill capacity

From `bench/baseline.txt`:

- `BenchmarkGenerateParallel/N=12/k=1` at 12 cores: 466 ms/op aggregate for 3 iterations → **≈ 6.4 puzzles/sec ≈ 23 000 puzzles/hour**.
- `BenchmarkGenerateParallel/N=12/k=2` at 12 cores: 653 ms/op aggregate → **≈ 4.6 puzzles/sec ≈ 16 500 puzzles/hour**.

From `bench/difficulty-distribution.md` (single-threaded, 15-min buckets):

- `(12, 1)`: 33.6 puzzles/min ≈ 2 000/hour per goroutine.
- `(12, 2)`: 37.6 puzzles/min ≈ 2 250/hour per goroutine.

Per-difficulty refill at 12 cores (yield × parallel throughput):

| tier | N=12, k=1 | N=12, k=2 |
|---|---|---|
| Easy | unreachable | unreachable |
| Medium | ~13 000/hr | ~80/hr (starved) |
| Hard | ~10 000/hr | ~16 400/hr |
| Expert | unreachable | unreachable |

## 4. N=14 note

Not in the committed Step 7 gate — informational only. `(14, 2)` averages 47 s/puzzle single-threaded with a 39% single-attempt success rate. Even with racing and parallel execution this is not a viable production tier without targeted mutator work. Flagged for a dedicated follow-up if the consumer ever wants N=14 as a first-class ceiling.

## 5. Known gaps / follow-ups

- **R-068z (dead-rule investigation) is blocking the difficulty-targeting evaluation.** We can't make the v2 call until we know whether R6/R8/R9 are dead or dormant.
- **Soak cross-check is non-blocking.** `.github/workflows/generator-check.yml` (R-068c) runs `-tags=soak` on PRs touching the generator but doesn't gate merges. Tighten when the generator stabilizes.
- **Hand-verified corpus is machine-generated.** `testdata/puzzles/**` (R-068b's `TestCorpusGenerate`) produces deterministic fixtures, but tier labels are the classifier's opinion. A human pass over a handful per tier is a separate quality gate before trusting the corpus as a regression baseline.
- **Distribution budget is configurable but defaults to 1 hour per bucket.** This snapshot ran at 15 min per bucket (`REIGN_DIST_BUDGET_SEC=900`). A full 1-hour run before cutover would tighten the Expert-yield bound, though at 0% the conclusion does not change.
