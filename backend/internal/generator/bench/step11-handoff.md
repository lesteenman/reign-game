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

**Trigger (per input-spec §11).** p99/median > 3× at any committed (N, k).

**Measured** (see `bench/latency-distribution.md` for full median/p99/max):

| (N, k) | p99/median |
|---|---|
| 6, 1 | 9.2× |
| 9, 1 | 7.8× |
| 12, 1 | 4.8× |
| 9, 2 | **73.3×** |
| 12, 2 | 6.1× |

Every bucket is past the trigger.

**Recommendation — do NOT ship `WithRacing`.** The spec's trigger is a latency-target rule; it assumes a synchronous caller waiting on a single `Generate()`. Reign does not have that caller.

Reign generates puzzles asynchronously into a DynamoDB pool. The frontend reads pre-generated puzzles from the pool on demand — no user waits on `Generate()`. In a pool-refill architecture:

- Tail latency is irrelevant. Nothing is blocked on any single call.
- Racing wastes compute. At M=2 a racing wrapper makes two `Generate()` calls per delivered puzzle — the loser's work is discarded.
- The only thing that matters is **aggregate throughput per dollar**, which scales by Lambda concurrency, not per-call latency reduction.

**What to do instead — size Lambda concurrency to throughput need.** `BenchmarkGenerateParallel/N=12/k=1` reports ~23 000 puzzles/hour at 12 cores on the dev box; a single-vCPU Lambda sits at ~2 000/hour. Run N concurrent Lambdas until the pool refill rate covers the consumption target.

If Reign ever adds a user-facing "generate one now" flow (e.g., custom-sized puzzle on request), revisit racing for that path specifically. The 7 s p99 at (12, 1) is too slow for a blocking UI. Pool reads are not that path.

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

Parallel throughput from `bench/baseline.txt` × per-difficulty yield from `bench/difficulty-distribution.md`, at 12 cores:

| tier | N=12, k=1 | N=12, k=2 |
|---|---|---|
| Easy | unreachable | unreachable |
| Medium | ~13 000/hr | ~80/hr (starved) |
| Hard | ~10 000/hr | ~16 400/hr |
| Expert | unreachable | unreachable |

Source numbers: `BenchmarkGenerateParallel/N=12/k=1` ≈ 23 000 total-puzzles/hr, `/k=2` ≈ 16 500/hr; yields from the distribution table in §2.

## 4. N=14 note

Not in the committed Step 7 gate — informational only. `(14, 2)` averages 47 s/puzzle single-threaded with a 39% single-attempt success rate. Even with racing and parallel execution this is not a viable production tier without targeted mutator work. Flagged for a dedicated follow-up if the consumer ever wants N=14 as a first-class ceiling.

## 5. Known gaps / follow-ups

- **R-068z (dead-rule investigation) is blocking the difficulty-targeting evaluation.** We can't make the v2 call until we know whether R6/R8/R9 are dead or dormant.
- **Soak cross-check is non-blocking.** `.github/workflows/generator-check.yml` (R-068c) runs `-tags=soak` on PRs touching the generator but doesn't gate merges. Tighten when the generator stabilizes.
- **Hand-verified corpus is machine-generated.** `testdata/puzzles/**` (R-068b's `TestCorpusGenerate`) produces deterministic fixtures, but tier labels are the classifier's opinion. A human pass over a handful per tier is a separate quality gate before trusting the corpus as a regression baseline.
- **Distribution budget is configurable but defaults to 1 hour per bucket.** This snapshot ran at 15 min per bucket (`REIGN_DIST_BUDGET_SEC=900`). A full 1-hour run before cutover would tighten the Expert-yield bound, though at 0% the conclusion does not change.
