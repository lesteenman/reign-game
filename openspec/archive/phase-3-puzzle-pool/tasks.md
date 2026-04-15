# Phase 3: Implementation Tasks

## Milestones

```
Milestone A (Infrastructure)       → DynamoDB table, SQS queue, Terraform, LocalStack, puzzle repository
Milestone B (Generation Pipeline)  → SQS consumer, replenish endpoint, generation with metadata
Milestone C (Serving + Frontend)   → /puzzles/next endpoint, frontend integration, metadata display
```

## Dependency Layers

```
Layer 0: T-300 (Terraform)
    ↓
Layer 1: T-301 (LocalStack) + T-302 (Puzzle Repository) — parallel
    ↓
Layer 2: T-303 (Pipeline extraction) + T-304 (SQS Publisher) — parallel
    ↓
Layer 3: T-305 (SQS Consumer / Generator)
    ↓
Layer 4: T-306 (Main entry point) + T-307 (Replenish endpoint) — parallel
    ↓
Layer 5: T-308 (Serve endpoint) + T-309 (Status endpoint) — parallel
    ↓
Layer 6: T-310 (Frontend API client) + T-311 (PuzzleSelector simplification) — parallel
    ↓
Layer 7: T-312 (GamePage integration) + T-313 (Game state update) — parallel
    ↓
Layer 8: T-314 (Integration test)
```

## Status

| Task | Title | Milestone | Status |
|------|-------|-----------|--------|
| T-300 | Terraform: DynamoDB + SQS + Generator Lambda | A | [ ] |
| T-301 | LocalStack setup | A | [ ] |
| T-302 | Puzzle repository | A | [ ] |
| T-303 | Pipeline builder extraction | B | [ ] |
| T-304 | SQS publisher | B | [ ] |
| T-305 | SQS consumer (Generator) | B | [ ] |
| T-306 | Main entry point update | B | [ ] |
| T-307 | Replenish endpoint | B | [ ] |
| T-308 | Serve endpoint | C | [ ] |
| T-309 | Status update endpoint | C | [ ] |
| T-310 | Frontend API client update | C | [ ] |
| T-311 | PuzzleSelector simplification | C | [ ] |
| T-312 | GamePage integration | C | [ ] |
| T-313 | Game state update | C | [ ] |
| T-314 | Integration test | C | [ ] |

## Tasks

### Milestone A: Infrastructure + Data Layer

#### T-300: Terraform — DynamoDB + SQS + Generator Lambda

- **Roadmap:** R-041
- **Agent:** devops-engineer
- **Spec:** specs/infrastructure.md (TF-01 through TF-04)
- **Work:**
  - Create `infra/modules/database/` with DynamoDB `puzzle-pool` table (PK/SK, PAY_PER_REQUEST)
  - Create SQS queue + DLQ (visibility timeout 900s, maxReceiveCount 3)
  - Create Generator Lambda function (same zip, GENERATOR_MODE=sqs, 900s timeout, 512MB)
  - SQS event source mapping (batch size 1)
  - IAM: API Lambda gets sqs:SendMessage + DynamoDB read/write; Generator Lambda gets SQS consume + DynamoDB write + CloudWatch Logs
  - Wire modules in `infra/main.tf` — pass table name and queue URL as outputs
- **Acceptance:** TF-01 through TF-04 pass. `terraform plan` shows all resources.
- **Commit after completion.**

#### T-301: LocalStack Setup

- **Roadmap:** R-045
- **Agent:** devops-engineer
- **Spec:** specs/infrastructure.md (TF-05)
- **Work:**
  - Update `docker-compose.yml`: add SQS to LocalStack services
  - Create init script to create puzzle-pool table and puzzle-generation queue on startup
  - Add `task dev:generator` to Taskfile (starts SQS consumer with LocalStack env vars)
  - Update `task dev:backend` with DynamoDB/SQS environment variables
- **Acceptance:** TF-05 passes. `docker compose up localstack` creates both table and queue.
- **Depends on:** T-300
- **Commit after completion.**

#### T-302: Puzzle Repository

- **Roadmap:** R-040
- **Agent:** backend-dev
- **Spec:** specs/backend.md (BE-01)
- **Work:**
  - Create `internal/repository/puzzle.go` with PuzzleRepository struct
  - Implement PutPuzzle, NextReady, MarkServed, UpdateStatus, CountReady
  - Define PuzzleRecord struct with all DynamoDB attributes
  - TDD: write tests first against a DynamoDB interface/mock
- **Acceptance:** BE-01 passes.
- **Depends on:** T-300 (needs table name convention, but code can be written against an interface)
- **Commit after completion.**

### Milestone B: Generation Pipeline

#### T-303: Pipeline Builder Extraction

- **Roadmap:** R-042
- **Agent:** backend-dev
- **Spec:** specs/backend.md (BE-04)
- **Work:**
  - Extract `buildPipeline` logic from `internal/handler/generate.go` to a shared location
  - Both HTTP handler and SQS consumer will import the same builder
  - Existing handler tests must continue to pass (no behavior change)
- **Acceptance:** BE-04 passes.
- **Depends on:** T-302
- **Commit after completion.**

#### T-304: SQS Publisher

- **Roadmap:** R-042
- **Agent:** backend-dev
- **Spec:** specs/backend.md (BE-02)
- **Work:**
  - Create `internal/queue/publisher.go` with Publisher struct
  - Implement PublishGenerationRequest: serialize to JSON, send via sqs.SendMessage
  - Define GenerationRequest struct matching SQS message schema
  - TDD: mock SQS client, verify message body
- **Acceptance:** BE-02 passes.
- **Depends on:** T-302
- **Commit after completion.**

#### T-305: SQS Consumer (Generator)

- **Roadmap:** R-042
- **Agent:** backend-dev
- **Spec:** specs/backend.md (BE-03)
- **Work:**
  - Create `internal/worker/generator.go` with HandleSQSEvent and RunLocalPoller
  - HandleSQSEvent: deserialize message, build pipeline, generate puzzle, write to DynamoDB
  - RunLocalPoller: long-poll loop for local dev
  - Record generation duration, construct full PuzzleRecord
  - TDD: valid message produces puzzle, invalid message returns error
- **Acceptance:** BE-03 passes.
- **Depends on:** T-303, T-302
- **Commit after completion.**

#### T-306: Main Entry Point Update

- **Roadmap:** R-042
- **Agent:** backend-dev
- **Spec:** specs/backend.md (BE-08)
- **Work:**
  - Update `cmd/api/main.go` to branch on GENERATOR_MODE
  - Construct DynamoDB and SQS clients from environment variables
  - Wire new routes: /admin/replenish, /puzzles/next, /puzzles/{id}/status
  - SQS mode + Lambda: start SQS event handler
  - SQS mode + local: start local poller
  - Default: HTTP server with all routes
- **Acceptance:** BE-08 passes.
- **Depends on:** T-305, T-304
- **Commit after completion.**

#### T-307: Replenish Endpoint

- **Roadmap:** R-043
- **Agent:** backend-dev
- **Spec:** specs/backend.md (BE-05)
- **Work:**
  - Create `POST /admin/replenish` handler
  - Iterate all 5 size+mode combos, count ready puzzles, publish SQS messages for gaps
  - Hardcoded defaults: iterative/propagation/bfs/0.0/deducible/concurrency=1
  - Optional size+mode filter via query params
  - Return triggered/skipped arrays
  - TDD: all pools full, some below threshold, specific combo filter
- **Acceptance:** BE-05 passes.
- **Depends on:** T-304, T-302
- **Commit after completion.**

### Milestone C: Serving + Frontend

#### T-308: Serve Endpoint

- **Roadmap:** R-044
- **Agent:** backend-dev
- **Spec:** specs/backend.md (BE-06)
- **Work:**
  - Create `GET /puzzles/next` handler
  - Query NextReady, MarkServed, return puzzle with metadata (no solution)
  - 404 with no_puzzles_available on empty pool
  - TDD: puzzle available returns 200, empty pool returns 404, invalid params return 400
- **Acceptance:** BE-06 passes.
- **Depends on:** T-302
- **Commit after completion.**

#### T-309: Status Update Endpoint

- **Roadmap:** R-044
- **Agent:** backend-dev
- **Spec:** specs/backend.md (BE-07)
- **Work:**
  - Create `PUT /puzzles/{id}/status` handler
  - Validate status transitions (served → solved/skipped)
  - Requires size+mode query params for PK construction
  - TDD: valid transitions, invalid status, missing params
- **Acceptance:** BE-07 passes.
- **Depends on:** T-302
- **Commit after completion.**

#### T-310: Frontend API Client Update

- **Roadmap:** R-046
- **Agent:** frontend-dev
- **Spec:** specs/frontend.md (FE-01)
- **Work:**
  - Add `fetchNextPuzzle(size, mode)` to puzzleService.ts
  - Add PuzzleMetadata and PuzzleResponse types
  - Add NoPuzzlesAvailableError for 404 handling
  - Remove generatePuzzle() and GenerateOptions
  - TDD: mock fetch for 200/404/500 scenarios
- **Acceptance:** FE-01 passes.
- **Depends on:** T-308 (needs API contract, but can code against spec)
- **Commit after completion.**

#### T-311: PuzzleSelector Simplification

- **Roadmap:** R-046
- **Agent:** frontend-dev
- **Spec:** specs/frontend.md (FE-02)
- **Work:**
  - Remove advanced toggle, advanced section, all advanced state
  - Keep four preset buttons
  - Simplify onSelect to `{ size, mode }` only
  - TDD: presets render, no advanced section, onSelect sends correct shape
- **Acceptance:** FE-02 passes.
- **Depends on:** None (independent of backend)
- **Commit after completion.**

#### T-312: GamePage Integration

- **Roadmap:** R-046, R-047, R-048
- **Agent:** frontend-dev
- **Spec:** specs/frontend.md (FE-03, FE-04, FE-05)
- **Work:**
  - Replace generatePuzzle() with fetchNextPuzzle()
  - Handle NoPuzzlesAvailableError → pool-empty UI with retry
  - Show metadata below the grid (compact format)
  - Update buildPlayUrl to omit removed params
  - TDD: fetch success loads puzzle with metadata, 404 shows pool-empty, retry works, metadata displays
- **Acceptance:** FE-03, FE-04, FE-05 pass.
- **Depends on:** T-310, T-311, T-313
- **Commit after completion.**

#### T-313: Game State Update

- **Roadmap:** R-047
- **Agent:** frontend-dev
- **Spec:** specs/frontend.md (FE-06)
- **Work:**
  - Add optional metadata field to PuzzleData type
  - Update GameState and createFreshGameState to carry metadata
  - Ensure backward compatibility with saved games without metadata
  - TDD: save/load with metadata, old state without metadata loads cleanly
- **Acceptance:** FE-06 passes.
- **Depends on:** None (type changes only)
- **Commit after completion.**

#### T-314: Integration Test

- **Roadmap:** All R-040 through R-048
- **Agent:** tester
- **Work:**
  - Start LocalStack, API server, and generator process
  - Call POST /admin/replenish → verify SQS messages published
  - Wait for generator to process → verify puzzles appear in DynamoDB
  - Call GET /puzzles/next → verify puzzle returned with metadata
  - Call PUT /puzzles/{id}/status → verify status updated
  - Call GET /puzzles/next when pool is empty → verify 404
  - Verify frontend loads puzzle from pool (manual or Playwright if available)
- **Acceptance:** Full flow works end-to-end locally.
- **Depends on:** T-306, T-308, T-309, T-312
- **Commit after completion.**

## Execution Summary

| Layer | Tasks | Agents | Parallel? |
|-------|-------|--------|-----------|
| 0 | T-300 | devops-engineer | — |
| 1 | T-301, T-302 | devops-engineer, backend-dev | Yes |
| 2 | T-303, T-304 | backend-dev | Yes |
| 3 | T-305 | backend-dev | — |
| 4 | T-306, T-307 | backend-dev | Yes |
| 5 | T-308, T-309 | backend-dev | Yes |
| 6 | T-310, T-311, T-313 | frontend-dev | Yes |
| 7 | T-312 | frontend-dev | — |
| 8 | T-314 | tester | — |
