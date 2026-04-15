# Spec: Backend

Covers R-040 (data model), R-042 (SQS generation), R-043 (replenish endpoint), and R-044 (serve endpoint).

## Requirements

### BE-01: Puzzle Repository

- New package `internal/repository/` with a `PuzzleRepository` struct
- Constructor accepts a DynamoDB client and table name
- Methods:
  - `PutPuzzle(ctx, puzzle PuzzleRecord) error` — writes a puzzle with `status = "ready"`
  - `NextReady(ctx, size int, mode string) (*PuzzleRecord, error)` — queries for one ready puzzle, returns nil if none
  - `MarkServed(ctx, pk string, sk string) error` — updates status to `served`, sets `servedAt`
  - `UpdateStatus(ctx, pk string, sk string, status string) error` — updates status (served → solved/skipped)
  - `CountReady(ctx, size int, mode string) (int, error)` — returns count of ready puzzles for a combo
- `PuzzleRecord` struct maps all DynamoDB attributes from the design doc
- Tests: table-driven tests against a mock/interface or LocalStack DynamoDB. Cover: put + query round-trip, NextReady returns nil on empty, MarkServed updates correctly, CountReady matches expected count

### BE-02: SQS Publisher

- New package `internal/queue/` with a `Publisher` struct
- Constructor accepts an SQS client and queue URL
- Method: `PublishGenerationRequest(ctx, req GenerationRequest) error`
- `GenerationRequest` struct matches the SQS message schema from the design doc
- Serializes to JSON, sends via `sqs.SendMessage`
- Tests: verify message body matches expected JSON shape (mock SQS client)

### BE-03: SQS Consumer (Generator Lambda + Local Poller)

- New package `internal/worker/` with generation logic
- `HandleSQSEvent(ctx, event events.SQSEvent) error` — Lambda SQS handler
  - Deserializes message body to `GenerationRequest`
  - Constructs pipeline via existing `buildPipeline` logic (extracted from handler package)
  - Records start time, calls `pipeline.Generate()` with timeout of 14 minutes
  - On success: constructs `PuzzleRecord` with all metadata, calls `PuzzleRepository.PutPuzzle()`
  - On failure: returns error (SQS retries)
- `RunLocalPoller(ctx, sqsClient, queueURL, handler)` — long-poll loop for local dev
  - Polls with 20s wait time
  - Processes one message at a time
  - Deletes message after successful processing
  - Exits on context cancellation
- Tests: HandleSQSEvent with valid message produces a puzzle record, invalid message returns error, timeout handling

### BE-04: Pipeline Builder Extraction

- Extract `buildPipeline` and `generateParams` from `internal/handler/generate.go` to a shared location (e.g., `internal/generator/` or a new `internal/pipeline/` package)
- Both the HTTP handler and the SQS consumer must use the same pipeline construction logic
- No behavior change — pure refactor
- Tests: existing handler tests still pass

### BE-05: Replenish Endpoint

- `POST /admin/replenish` handler in `internal/handler/`
- Iterates all size+mode combinations: (5, standard), (7, standard), (9, standard), (7, double), (9, double)
- Optional query params `size` and `mode` to target a specific combo
- For each combo: calls `PuzzleRepository.CountReady()`, if below threshold (3), publishes `(threshold - count)` SQS messages via `Publisher`
- Generation config: hardcoded defaults (pipeline=iterative, solver=propagation, regions=bfs, regionVariance=0.0, deducible=true, concurrency=1)
- Response: JSON with `triggered` and `skipped` arrays per design doc
- Tests: table-driven — all pools full (no messages), some pools below threshold (correct message count), specific combo filter, empty pools

### BE-06: Serve Endpoint

- `GET /puzzles/next` handler in `internal/handler/`
- Query params: `size` (required, int), `mode` (required, `standard` | `double`)
- Calls `PuzzleRepository.NextReady(size, mode)`
- If found: calls `MarkServed()`, returns puzzle with metadata (no solution field)
- If not found: returns 404 with `no_puzzles_available` error
- Response shape matches design doc (puzzleId, gridSize, mode, regionMap, metadata object)
- Tests: table-driven — puzzle available returns 200 with correct shape, no puzzle returns 404, invalid params return 400

### BE-07: Status Update Endpoint

- `PUT /puzzles/{id}/status` handler in `internal/handler/`
- Request body: `{"status": "solved"}` or `{"status": "skipped"}`
- Validates: status must be `solved` or `skipped`
- Requires `size` and `mode` query params (to construct PK for DynamoDB lookup)
- Calls `PuzzleRepository.UpdateStatus()`
- Response: 200 on success, 400 on invalid status, 404 if puzzle not found
- Tests: valid transitions, invalid status value, missing params

### BE-08: Main Entry Point Update

- `cmd/api/main.go` branches on `GENERATOR_MODE` environment variable
- `GENERATOR_MODE=sqs` + Lambda: starts SQS event handler
- `GENERATOR_MODE=sqs` + local: starts local SQS poller
- Default (no `GENERATOR_MODE`): starts HTTP server (existing behavior) with new routes added
- DynamoDB client and SQS client constructed from environment variables (`DYNAMODB_ENDPOINT`, `SQS_ENDPOINT`, `SQS_QUEUE_URL`, `PUZZLE_TABLE_NAME`, `AWS_REGION`)
- New routes registered: `POST /admin/replenish`, `GET /puzzles/next`, `PUT /puzzles/{id}/status`
- Tests: router includes new routes (existing router test pattern)

## Acceptance Criteria

All BE-01 through BE-08 pass. Generation pipeline produces puzzles in DynamoDB. Replenish endpoint triggers correct number of SQS messages. Serve endpoint returns puzzles with metadata. The existing `/puzzles/generate` endpoint continues to work (not removed this phase).
