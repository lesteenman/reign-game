# Generate() latency distribution (R-068a)

Per-call `Generate()` timings at each committed (N, k). Captured by `TestLatencyDistribution` under `-tags=latency`. Samples vary by N because wall-clock at N=14 is high; sample counts are chosen so every bucket fits in a reasonable local run.

| (N, k) | samples | ok | median | p90 | p99 | max | p99/median |
|---|---|---|---|---|---|---|---|
| 6, 1 | 500 | 500 | 994.417µs | 3.795459ms | 9.169875ms | 12.044209ms | 9.22x |
| 9, 1 | 300 | 300 | 57.439875ms | 200.454542ms | 449.820333ms | 589.971625ms | 7.83x |
| 12, 1 | 200 | 193 | 1.487423375s | 4.526940917s | 7.074050458s | 7.400300375s | 4.76x |
| 9, 2 | 300 | 300 | 2.728708ms | 44.965625ms | 200.057792ms | 282.1705ms | 73.32x |
| 12, 2 | 100 | 97 | 1.055628875s | 3.230015042s | 6.465423417s | 6.465423417s | 6.12x |

## Interpretation

Per input-spec §11, p99/median > 3× at any committed (N, k) is the trigger for recommending `WithRacing` in Step 11's handoff. A row with ratio <= 3× is within the single-stream budget; >= 3× means the slow-tail attempts are blocking a P99-sensitive consumer (Lambda response, user-facing generate-on-demand).

Every committed bucket above sits past 3×. The explicit `WithRacing` recommendation lives in `bench/step11-handoff.md`, produced by R-068d alongside the 1-hour difficulty-distribution run at N=12/14.
