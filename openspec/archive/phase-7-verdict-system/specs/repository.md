# Spec: Verdict Repository

The DynamoDB persistence contract for verdicts.

## VR-01: Verdict rows live in the existing `puzzle-pool` table under a new key family

**Rule.** Verdict rows share the `puzzle-pool` table with puzzles and configs. The key shape is:

- `PK = "VERDICT#{size}#{mode}#{puzzleId}"`
- `SK = "{raterRole}#{raterId}"`

No new DynamoDB table is created. No GSI is added in this phase. No Terraform change is required.

**Value.** Single-table convention (CLAUDE.md). The PK shape co-locates all verdicts for one puzzle in a single partition, so `ListVerdictsForPuzzle` is a single Query. The SK shape lets the row family scale to multiple rater roles without re-keying when the public-rater role lands.

**Verification.** Test: writing a verdict with `size=5, mode=standard, puzzleId=abc, raterId=user_123, raterRole=admin` produces a row with `PK="VERDICT#5#standard#abc"`, `SK="admin#user_123"`. `ListVerdictsForPuzzle(5, "standard", "abc")` returns it.

## VR-02: `VerdictRecord` carries the verdict value plus the calibration signal

**Rule.** The `VerdictRecord` struct stores:

| Attribute | Type | Source |
|---|---|---|
| `value` | string `"up"` or `"down"` | request body |
| `playTimeMs` | int64 | request body — wall-clock time on the producing attempt |
| `outcome` | string `"solved"` or `"skipped"` | request body |
| `clientVersion` | string | request body |
| `submittedAt` | string (RFC 3339 UTC) | server clock at write time |

`puzzleId`, `gridSize`, `mode`, `raterId`, `raterRole` are derived from PK / SK and not stored as separate attributes (`dynamodbav:"-"`), matching the `PuzzleRecord` and `ConfigRecord` patterns already in `repository/puzzle.go`.

**Value.** Captures everything R-084's blind calibration test wants — per-attempt play time, outcome (so a skip-downvote vs. a complete-downvote can be told apart), and client version (lets us correlate verdict patterns to UI revisions). Adding fields later is additive; subtracting fields later breaks the calibration corpus.

**Verification.** Round-trip test in `repository/puzzle_test.go`: marshal a `VerdictRecord` to a DynamoDB item, write, query, unmarshal — every field is preserved including PK/SK-derived fields.

## VR-03: `PutVerdict` is an unconditional overwrite

**Rule.** `PuzzleRepository.PutVerdict(ctx, *VerdictRecord)` writes the row via `PutItem` with no condition expression. A second call with the same PK / SK overwrites the prior row. `submittedAt` advances; the row count stays at one.

**Value.** PUT semantics. The locked decision says re-submission overwrites — no accumulation. An unconditional `PutItem` is the simplest implementation and matches the contract exactly.

**Verification.** Test: same `(puzzleId, raterId)` pair, three writes with values up / down / up. After all three writes, a Query on `PK="VERDICT#..."` returns exactly one item with `value: "up"`.

## VR-04: `ListVerdictsForPuzzle` returns every verdict row for a puzzle

**Rule.** `PuzzleRepository.ListVerdictsForPuzzle(ctx, size, mode, puzzleID)` issues a single Query against `PK="VERDICT#{size}#{mode}#{puzzleId}"`, unmarshals every item into a `VerdictRecord`, and returns the slice. No filter expression. Returns an empty slice when no rows exist.

**Value.** Source of truth for "what has been voted on this puzzle." Exposed publicly so the analysis agent (Phase 9) and any future audit tool reads from the row family directly without going through the summary.

**Verification.** Test: write three verdict rows for one puzzle, call `ListVerdictsForPuzzle`, get three records back with PK / SK round-tripped to the struct fields. Test: never-voted puzzle returns empty slice.

## VR-05: `RecomputeVerdictSummary` reads the row family and writes the projection

**Rule.** `PuzzleRepository.RecomputeVerdictSummary(ctx, size, mode, puzzleID)`:

1. Calls `ListVerdictsForPuzzle` to read every verdict row.
2. Counts `up` and `down` values into a `VerdictSummary{Up: int, Down: int, LastUpdatedAt: <now RFC 3339>}`.
3. Writes the summary onto `PuzzleRecord.verdictSummary` via `UpdateItem` with `attribute_exists(PK)` (so a missing puzzle row produces an error, not a silent upsert).
4. Returns the summary.

The recompute is read-then-write, not in-place increment. Reasoning: in-place is correct for "first-ever vote" but wrong on overwrite (admin flips up → down → up). A read-then-write is O(votes-per-puzzle) which is bounded in practice.

**Value.** The summary is always derivable from the row family. If it ever drifts, calling `RecomputeVerdictSummary` reconciles it.

**Verification.** Test: write `up`, write `down`, write `up` from same admin → summary is `{up: 1, down: 0}` (overwrite, not accumulate). Test: two admins vote `up` and `down` → summary is `{up: 1, down: 1}`. Test: no votes → summary is `{up: 0, down: 0}`. Test: missing puzzle row → returns error.

## VR-06: `PuzzleRecord` schema swaps `Verdict string` for `VerdictSummary`

**Rule.** The `PuzzleRecord` struct in `repository/puzzle.go` is modified:

- The `Verdict string \`dynamodbav:"verdict"\`` field is removed.
- A `VerdictSummary VerdictSummary \`dynamodbav:"verdictSummary"\`` field is added.

Every Go-side write of `verdict: "none"` (in `worker/generator.go`, `cmd/genfixtures/main.go`, and tests) is removed. Every read of the legacy field (none today, confirmed by grep) is removed.

**Value.** The schema reflects the multi-rater storage model. The legacy field was unused at read time and pre-committed the codebase to a single-voter design.

**Verification.** Grep sweep in R-081's PR confirms zero remaining read or write references to the legacy `Verdict` string field. New puzzles serialize with `verdictSummary: {up: 0, down: 0, lastUpdatedAt: ""}` instead of `verdict: "none"`.

## VR-07: Legacy rows tolerate the schema change without backfill

**Rule.** Existing puzzle rows in DynamoDB carry a top-level `verdict: "none"` attribute. After the schema change, that attribute is ignored on unmarshal (the Go struct has no field for it). New writes do not include it. The `verdictSummary` field on legacy rows is absent on first read; `attributevalue.UnmarshalMap` decodes a missing struct attribute as the zero value, producing `VerdictSummary{Up: 0, Down: 0, LastUpdatedAt: ""}`.

**Value.** No backfill script. No migration window. The change is read-tolerant in both directions during deploy.

**Verification.** Test: marshal a legacy item with `verdict: "none"` and no `verdictSummary` attribute, unmarshal as `PuzzleRecord` → struct's `VerdictSummary` is the zero value, no decode error. Test: new write does not include the `verdict` attribute in the marshaled item map.

## VR-08: Verdict writes do not block on transactional consistency with the summary

**Rule.** The handler calls `PutVerdict` and then `RecomputeVerdictSummary` as two independent DynamoDB operations. Neither is wrapped in `TransactWriteItems`. A crash between the two writes leaves the row family canonical and the summary stale by one vote — recoverable on the next call.

**Value.** Single-admin scale doesn't see the lag. Adding a transaction doubles write cost and introduces partial-failure modes that are themselves harder to test than the lag.

**Verification.** Code review: no `TransactWriteItems` call in `verdict.go` or the new repository methods. Test (failure injection): if the second write errors, the row family is still correct, and the next successful write reconciles the summary.

## VR-09: No GSI for verdict-bucketed queries this phase

**Rule.** The repository ships no GSI keyed on `(verdictBucket, createdAt)` or any verdict-aware secondary key. "Find all puzzles where the summary is downvote-heavy" is implemented by callers via `Query` + `FilterExpression` against the puzzle partition (small at current scale).

**Value.** Avoids a Terraform change and a GSI backfill. Phase 9's analysis agent decides whether the scan-with-filter latency justifies a GSI based on observed query patterns.

**Verification.** `infra/modules/database/` Terraform diff in R-081's PR is empty. Code review: no `IndexName` parameter on any new `Query` call.

## VR-10: Repository methods are independently exposed; the handler composes them

**Rule.** `PutVerdict`, `ListVerdictsForPuzzle`, and `RecomputeVerdictSummary` are three separate methods. The handler is the only caller that composes the first and the third in the standard happy path. Tests can exercise each method in isolation.

**Value.** Repository methods stay narrow and testable. The summary recompute can be re-run by future tooling (Phase 9 reconciliation script) without going through the handler.

**Verification.** Code review: `repository/puzzle.go` exports the three methods with the signatures named above. Each has dedicated tests in `repository/puzzle_test.go` that do not invoke the handler.
