# internal/repository/

DynamoDB data-access layer. `*PuzzleRepository` is the single struct exposed; it wraps a `DynamoDBAPI` interface (matching the subset of `*dynamodb.Client` we actually call) plus a table name. Production binaries construct one via `repository.NewPuzzleRepository(awsclient.NewDynamoDBClient(&cfg), tableName)`; tests inject a fake `DynamoDBAPI`.

The whole project uses a single DynamoDB table (`puzzle-pool` in prod; `puzzle-pool-e2e` in the e2e environment) with multiple key shapes overlaid via the `PK` partition.

## Data flow

- **In** — Method calls from handlers / orchestrators (`internal/daily`, `internal/replenish`). Inputs are domain values (`PuzzleRecord`, `ConfigRecord`, etc.).
- **Calls** — `aws-sdk-go-v2/service/dynamodb` + `attributevalue` directly. No ORM.
- **Out** — Domain structs with `dynamodbav` tags. PK and SK components stored as `dynamodbav:"-"` and reconstructed from the keys at read time so the storage shape is denormalized (one source of truth per key component).

## Row shapes (PK / SK)

| Row family | PK | SK | Defining file |
|---|---|---|---|
| Puzzle | `{size}#{mode}` | `{puzzleId}` | `puzzle.go::PuzzleRecord` |
| Config | `CONFIG` | `{size}#{mode}` | `puzzle.go::ConfigRecord` |
| Verdict | `VERDICT#{size}#{mode}#{puzzleId}` | `{raterRole}#{raterId}` | `puzzle.go::VerdictRecord` |
| Daily schedule | `DAILY#{YYYY-MM-DD}` | `<single>` | `daily.go::ScheduleRecord` |
| Daily candidate | `DAILY-CANDIDATE` | `<single>` | `daily.go::CandidateRecord` |
| Play | `PLAY#{playerId}` | `DAILY#{YYYY-MM-DD}` | `daily.go::PlayRecord` |
| Daily leaderboard | `DAILY-LEADERBOARD#{YYYY-MM-DD}` | `{paddedMs:8d}#{userId}` | `daily.go` (no struct; inline `Put`) |

## Key types and exported symbols

- `PuzzleRepository` — the entry point. Implements every method called by handlers / orchestrators.
- `DynamoDBAPI` — narrow interface (the SDK calls we actually need).
- `PuzzleRecord`, `ConfigRecord`, `VerdictRecord`, `VerdictSummary` — the puzzle/config/verdict row shapes (in `puzzle.go`).
- `ScheduleRecord`, `ScheduleCounters`, `CandidateRecord`, `PlayRecord`, `SubmitInput`, `FinalizeMode` — daily-puzzle row shapes (in `daily.go`).
- Sentinel errors — `ErrPuzzleNotFound`, `ErrCandidateAlreadyExists`, `ErrScheduleAlreadyFinalized`, `ErrPlayNotInStartedState`. Plus `ConfigAlreadyExistsError` (typed; carries Size+Mode).
- Constants — `DailyRecycleWindowDays = 14`, `ScheduleCounterStarted/Solved`, `PlayOutcomeStarted/Solved`, `FinalizeModeConfirm/Recycle`.

## Methods grouped by family

- **Puzzles** — `PutPuzzle`, `NextReady`, `MarkServed`, `UpdateStatus`, `GetPuzzle`, `CountReady`.
- **Configs** — `GetAllConfigs`, `GetConfig`, `PutConfig`, `CreateConfig`, `TryClaimAutoReplenish` (uses the CONFIG row's `last_auto_replenish_ts` attribute as a debounce mutex).
- **Verdicts** — `PutVerdict`, `ListVerdictsForPuzzle`, `RecomputeVerdictSummary`.
- **Daily schedule / candidate** — `GetSchedule`, `GetCandidate`, `PutCandidateIfAbsent`, `DeleteCandidate`, `FinalizeSchedule`, `IncrementScheduleCounter`, `FinalizeDailyTransaction`, `ListApprovedPool`, `MarkPuzzleAsDailyOn`.
- **Daily play / leaderboard** — `GetPlay`, `SubmitPlayTransactionally` (3-leg `TransactWriteItems`), `LeaderboardRank`. PLAY-row creation is performed by a 2-leg `TransactWriteItems` composed in `service/daily/` (PUT + `counters.started` bump).

## Rules specific to this directory

- **Single-table design.** Don't add a second table unless access patterns truly diverge — the design treats `puzzle-pool` as the only data store.
- **Conditional writes for races.** `attribute_not_exists(PK)` for "create if absent" and `attribute_exists(PK)` for "update if present" — never let DDB silently upsert orphan rows on attacker-supplied keys.
- **`Limit` + `FilterExpression` is a footgun.** DDB applies `Limit` *before* the filter. `NextReady`, `ListApprovedPool`, and `CountReady` all omit `Limit` and rely on small partition sizes. The rule is repeated in source comments and in `backend/CLAUDE.md` lesson 2.
- **PK/SK components are `dynamodbav:"-"`.** The components live as struct fields for handler convenience but are NOT stored as separate attributes. The repository re-derives them on read (`fmt.Sscanf` against the `PK`/`SK` value).
- **Sentinel errors over generic errors.** Conditional-check failures are mapped to package-level sentinels callers test with `errors.Is`. The `isConditionalCheckFailureOnLeg` helper unpacks `TransactionCanceledException` to identify which leg failed.

## Subtleties worth flagging during refactor work

- `MarkPuzzleAsDailyOn` (in `daily.go`) performs an extra `GetPuzzle` after a conditional failure to distinguish "row missing" from "newer date present". An optimistic check, not free — see the inline comment.
- `RecomputeVerdictSummary` is read-list-then-write; transactional cost is intentionally O(rows). Not transactional with `PutVerdict` itself because verdicts are the canonical row family and the summary is a cached projection (VR-05).
- `LeaderboardRank` is a single `Query` with `Select=COUNT` and no pagination guard — fine while leaderboards stay under DDB's 1 MB query result cap, but watch this if Phase 9 expands the surface (TODO is documented in the source).
