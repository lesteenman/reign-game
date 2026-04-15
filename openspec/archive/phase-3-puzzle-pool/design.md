# Phase 3: Design Document

Authoritative design reference for Phase 3 implementation. See design-grill-summary.md for the full decision log and rationale.

## DynamoDB Table (R-040)

### Table: `puzzle-pool`

On-demand (pay-per-request) billing. No GSI.

| Attribute | Type | Description |
|-----------|------|-------------|
| `PK` | String | `{gridSize}#{mode}` (e.g., `7#standard`) |
| `SK` | String | Puzzle UUID |
| `status` | String | `ready`, `served`, `solved`, `skipped` |
| `verdict` | String | `none`, `upvote`, `downvote`, `skip` (not writable this phase) |
| `regionMap` | List | 2D array of region IDs |
| `solution` | List | 2D boolean array (correct placement) |
| `pipeline` | String | `region-first`, `iterative`, `constraint-aware` |
| `solver` | String | `backtrack`, `propagation` |
| `regions` | String | `bfs`, `wfc` |
| `regionVariance` | Number | 0.0 - 1.0 |
| `deducible` | Boolean | Always `true` this phase |
| `concurrency` | Number | Goroutines used during generation |
| `generationDurationMs` | Number | Wall-clock generation time in milliseconds |
| `createdAt` | String | ISO 8601 timestamp |
| `servedAt` | String | ISO 8601 timestamp, null until served |

### Access Patterns

1. **Serve next ready puzzle:** `Query PK = "7#standard"` with `FilterExpression status = "ready"`, `Limit 1`. At current scale (~60 items per partition), filter cost is negligible.
2. **Count ready puzzles per combo:** `Query PK = "7#standard"` with `FilterExpression status = "ready"`, `Select COUNT`.
3. **Write new puzzle:** `PutItem` with all attributes, `status = "ready"`.
4. **Mark as served:** `UpdateItem` set `status = "served"`, `servedAt = now()`.
5. **Mark as solved/skipped:** `UpdateItem` set `status = "solved"` or `status = "skipped"`.

## SQS Queue (R-041, R-042)

### Queue: `puzzle-generation`

Standard queue (not FIFO — order irrelevant). Visibility timeout = 900 seconds (matches Generator Lambda max execution time). Dead-letter queue after 3 failed attempts.

### Message Schema

```json
{
  "size": 7,
  "mode": "standard",
  "pipeline": "iterative",
  "solver": "propagation",
  "regions": "bfs",
  "regionVariance": 0.0,
  "deducible": true,
  "concurrency": 1
}
```

One message = one puzzle generation request. The Generator Lambda processes one message at a time (batch size 1).

## Lambda Architecture (R-041, R-042)

### Two Lambda functions, one binary

The same Go binary serves both functions. The `main()` function branches on an environment variable:

```go
func main() {
    if os.Getenv("GENERATOR_MODE") == "sqs" {
        // SQS consumer: receive messages, generate puzzles, write to DynamoDB
        lambda.Start(handleSQSEvent)
    } else if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
        // API handler: HTTP routes via chi + API Gateway proxy
        lambda.Start(chiadapter.New(newRouter()).ProxyWithContext)
    } else {
        // Local dev: HTTP server on port 5181
        http.ListenAndServe(...)
    }
}
```

**API Lambda:** 29s timeout, 512MB memory, API Gateway trigger. Handles all HTTP routes.

**Generator Lambda:** 900s (15min) timeout, 512MB memory, SQS trigger. Processes one generation request per invocation. Writes the completed puzzle to DynamoDB with full metadata.

### Generation Flow

1. Generator Lambda receives SQS message
2. Constructs pipeline from message params (same `buildPipeline` logic as current handler)
3. Records `startTime`
4. Calls `pipeline.Generate()` with `Deducible: true` and a timeout of 14 minutes (leaving 1 minute for SQS overhead + DynamoDB write)
5. On success: calculates `generationDurationMs`, writes puzzle to DynamoDB with `status = "ready"`
6. On failure: returns error (SQS retries up to 3x, then dead-letter)

## API Endpoints (R-043, R-044)

### POST /admin/replenish

Checks pool levels for all size+mode combinations. For each combo below the pool threshold, publishes one SQS message per needed puzzle.

**Size+mode combinations:** 5x5 standard, 7x7 standard, 9x9 standard, 7x7 double, 9x9 double.

**Pool threshold:** 3 (hardcoded this phase).

**Request:** No body required. Optional query params `size` and `mode` to replenish a specific combo only.

**Response:**
```json
{
  "triggered": [
    {"size": 9, "mode": "standard", "count": 2},
    {"size": 9, "mode": "double", "count": 3}
  ],
  "skipped": [
    {"size": 5, "mode": "standard", "ready": 3},
    {"size": 7, "mode": "standard", "ready": 3}
  ]
}
```

### GET /puzzles/next

Serves the next ready puzzle for a given size and mode.

**Query params:**
- `size`: integer, required
- `mode`: `standard` | `double`, required

**Success response (200):**
```json
{
  "puzzleId": "550e8400-...",
  "gridSize": 7,
  "mode": "standard",
  "regionMap": [[...]],
  "metadata": {
    "pipeline": "iterative",
    "solver": "propagation",
    "regions": "bfs",
    "regionVariance": 0.0,
    "generationDurationMs": 4200,
    "createdAt": "2026-04-15T10:30:00Z"
  }
}
```

Note: `solution` is never sent to the client. `metadata` is a new field not present in the current generate endpoint response.

**No puzzles available (404):**
```json
{
  "error": "no_puzzles_available",
  "message": "No puzzles available for this size and mode. Try again shortly."
}
```

**Invalid params (400):** Same error format as current generate endpoint.

### PUT /puzzles/{id}/status

Updates a puzzle's status after the player finishes or skips.

**Request body:**
```json
{
  "status": "solved"
}
```

Valid transitions: `served` → `solved`, `served` → `skipped`.

**Response (200):** Empty body, or echo the updated status.

## Local Development (R-045)

### LocalStack

The existing `docker-compose.yml` adds SQS to the LocalStack services. The `puzzle-pool` DynamoDB table and `puzzle-generation` SQS queue are created on container startup via an init script.

### Two Processes

**`task dev:backend`** — Starts the API HTTP server on port 5181. Configured to use LocalStack DynamoDB and SQS endpoints via environment variables.

**`task dev:generator`** — Starts the SQS consumer as a long-running poller against LocalStack SQS. Uses the same binary with `GENERATOR_MODE=sqs` but in local mode, polls SQS instead of using Lambda event source mapping.

The SQS consumer in local mode:
1. Long-polls the queue (20s wait time)
2. Receives a message
3. Generates the puzzle (same code path as Lambda)
4. Writes to LocalStack DynamoDB
5. Deletes the message
6. Loops

### Environment Variables

| Variable | API Server | Generator | Description |
|----------|-----------|-----------|-------------|
| `DYNAMODB_ENDPOINT` | `http://localhost:4566` | `http://localhost:4566` | LocalStack DynamoDB |
| `SQS_ENDPOINT` | `http://localhost:4566` | `http://localhost:4566` | LocalStack SQS |
| `SQS_QUEUE_URL` | set | set | Queue URL for publish/consume |
| `AWS_REGION` | `us-east-1` | `us-east-1` | LocalStack default |
| `GENERATOR_MODE` | unset | `sqs` | Selects SQS consumer mode |
| `PUZZLE_TABLE_NAME` | `puzzle-pool` | `puzzle-pool` | DynamoDB table name |

## Frontend (R-046, R-047, R-048)

### API Client Changes

Replace `generatePuzzle()` with `fetchNextPuzzle()`:

```typescript
interface PuzzleMetadata {
  pipeline: string;
  solver: string;
  regions: string;
  regionVariance: number;
  generationDurationMs: number;
  createdAt: string;
}

interface PuzzleResponse {
  puzzleId: string;
  gridSize: number;
  mode: string;
  regionMap: number[][];
  metadata: PuzzleMetadata;
}

function fetchNextPuzzle(size: number, mode: string): Promise<PuzzleResponse>
```

### PuzzleSelector Changes

Remove advanced options (pipeline, solver, regions, variance controls and the "Advanced" toggle). Keep the four preset buttons: 5x5 Standard, 7x7 Standard, 9x9 Standard, 9x9 Double Queens.

### GamePage Changes

Replace the `generatePuzzle()` call with `fetchNextPuzzle()`. Store the metadata alongside the puzzle in game state.

**New state: pool empty.** When `/puzzles/next` returns 404, show a message: "No puzzles available for this size and mode" with a "Retry" button. Do not fall back to on-demand generation.

**Metadata display.** Show generation metadata subtly below or beside the grid. Format: `iterative / propagation / 4.2s` or similar compact representation. Always visible (no toggle needed — the current UI is effectively the admin/curation flow).

### Game State Changes

The `PuzzleData` type gains an optional `metadata` field. Persisted in IndexedDB alongside the puzzle so metadata survives page reload.

### URL Parameter Changes

Remove generator-specific URL params (`pipeline`, `solver`, `regions`, `regionVariance`) from the play URL. The play URL becomes: `/play?new=true&size=7&mode=standard`. The puzzle comes from the pool, not from on-demand generation.
