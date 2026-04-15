# Design Grill Summary: Puzzle Pool + Generation Pipeline

## Final Design

Pre-generate puzzles into a DynamoDB `puzzle-pool` table, served to players on demand. An SQS queue decouples the serve path (29s API Lambda) from the generation path (15min Generator Lambda, same binary). The frontend requests the next ready puzzle by size and mode instead of generating on the fly. Generation metadata is stored per puzzle and shown in the UI. Pool size is 3 per size+mode combination, hardcoded for now.

## Decisions

### Data Storage

**DynamoDB `puzzle-pool` table.** PK = `{size}#{mode}`, SK = `{puzzleId}`. Simple purpose-built table, no GSI. FilterExpression on status is sufficient at current scale (~60 items per partition). Revisit if partitions exceed ~200 items. Verdict field included in schema (default `none`) but not writable via API this phase.

**Stored attributes per puzzle:** puzzleId, gridSize, mode, regionMap, solution, status, verdict, pipeline, solver, regions, regionVariance, deducible, generationDurationMs, concurrency, createdAt, servedAt.

### Generation Architecture

**Two Lambda functions from one binary.** API Lambda (29s timeout, API Gateway) handles HTTP routes. Generator Lambda (15min timeout, SQS trigger) generates one puzzle per message. The binary distinguishes its role via environment variable (`GENERATOR_MODE`).

**SQS queue** bridges the two. Standard queue with dead-letter queue. One message = one puzzle generation request containing size, mode, and generation config.

**`POST /admin/replenish`** checks all pool levels, publishes one SQS message per needed puzzle. Returns immediately with what was triggered.

**Locally:** LocalStack provides both DynamoDB and SQS. Two processes: `task dev:backend` (HTTP server) and `task dev:generator` (SQS consumer). Full fidelity with production.

### Serving

**`GET /puzzles/next?size=N&mode=M`** queries the pool for a ready puzzle, marks it as served, returns the puzzle with generation metadata. When the pool is empty, returns an error; the frontend shows "no puzzles available" with a retry button.

**Response includes a `metadata` object** with pipeline, solver, regions, regionVariance, generationDurationMs, createdAt. Shown subtly in the UI.

### Configuration

**Hardcoded defaults this phase.** Pipeline: iterative. Solver: propagation. Regions: bfs. Region variance: 0.0. Deducible: true. Concurrency: 1. Pool size: 3 per size+mode combo. Per-combo config via DynamoDB deferred to admin UI phase.

### Status Lifecycle

`ready` (generated, in pool) -> `served` (given to a player) -> `solved` or `skipped`. Served puzzles are consumed and not re-served from the pool. Replay by ID is deferred.

### Frontend Changes

Replace `generatePuzzle()` call with `GET /puzzles/next`. Remove advanced options (pipeline, solver, regions, variance) from PuzzleSelector. Show generation metadata on the puzzle page. "No puzzles available" + retry when pool is empty.

## Deferred Items

- **Admin pool management UI + config endpoints** -- per-combo generation settings, pool size tuning, pool counts (Phase 4).
- **Verdict endpoint + UI** -- upvote/downvote/skip buttons. Schema ready, API and UI deferred (Phase 5).
- **Puzzle replay** -- load any puzzle by ID, browse played puzzles in admin UI (Phase 6).
- **Analysis agent + endpoints** -- dedicated agent for querying and interpreting puzzle data (Phase 7).
- **Difficulty rating algorithm** -- deducibility-based rating, UI selector (Phase 8).
- **Daily puzzles / scheduling** -- assign puzzles to dates, daily challenge flow (Phase 9+).
- **Separate production puzzle table** -- approved puzzles copied from pool for numbered/daily serving (Phase 9+).

## Constraints and Assumptions

- API Gateway hard limit is 29s. Generation that exceeds 29s must go through SQS + Generator Lambda.
- Generator Lambda timeout set to 15min as a safe upper bound until real generation durations are measured.
- LocalStack must support both DynamoDB and SQS for local dev parity.
- Pool size of 3 is sufficient for current usage (one developer + occasional playtesters).
- Single verdict per puzzle, last write wins. No per-player tracking until identity system exists.
- All generated puzzles are deducible (solvable without guessing). Non-deducible puzzles are a future difficulty tier.
