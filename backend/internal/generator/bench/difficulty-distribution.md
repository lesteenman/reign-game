# Generate() difficulty distribution (R-068d)

Each bucket ran for 15m0s wall-clock. Override with REIGN_DIST_BUDGET_SEC. Captured by `TestDifficultyDistribution` under `-tags=distribution`.

| (N, k) | attempts | ok | ok/min | easy | medium | hard | expert | expert yield |
|---|---|---|---|---|---|---|---|---|
| 12, 1 | 512 | 505 | 33.6 | 0 | 279 | 226 | 0 | 0.0% |
| 12, 2 | 587 | 564 | 37.6 | 0 | 2 | 562 | 0 | 0.0% |
| 14, 1 | 104 | 51 | 3.4 | 0 | 28 | 23 | 0 | 0.0% |
| 14, 2 | 67 | 26 | 1.3 | 0 | 1 | 25 | 0 | 0.0% |

## Interpretation

- **Expert yield** is `expert_count / total_successes`. Per input-spec §10 the `WithDifficulty(Expert)` filter is a retry-until-match loop. If yield is far below the consumer's throughput target the retry budget dominates wall time and v2 difficulty-targeting (biased generation) becomes necessary.
- **Throughput (ok/min)** at N=12 drives Lambda concurrency sizing. Combine with `BenchmarkGenerateParallel` (baseline.txt) to project aggregate pool refill rate.
- Row-by-row quirks (a bucket starving in one tier) are usually a classifier issue rather than a generator issue; cross-check against the rule trace distribution from the property-corpus logs.
