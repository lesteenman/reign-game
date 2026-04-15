# Phase 3: Puzzle Pool + Generation Pipeline

## What

Pre-generate puzzles into a DynamoDB pool table, served to players on demand instead of generating on the fly. An SQS queue decouples the API from a dedicated Generator Lambda (15-minute timeout) that handles slow generation workloads. The frontend switches from calling `/puzzles/generate` to `/puzzles/next`, and shows generation metadata on each puzzle.

## Why

On-demand generation is too slow for 9x9 and Double Queens puzzles — Lambda timeouts at 29 seconds are insufficient. Pre-generating into a pool decouples generation time from user experience. Storing generation metadata (engine, duration, parameters) enables future analysis of generator performance across strategies and grid sizes.

## Scope

- **R-040** — DynamoDB `puzzle-pool` table (PK=size#mode, SK=puzzleId, status/verdict/metadata)
- **R-041** — Terraform: DynamoDB table, SQS queue + DLQ, Generator Lambda (15min timeout)
- **R-042** — SQS-based generation: API Lambda publishes messages, Generator Lambda consumes and writes puzzles to DB
- **R-043** — `POST /admin/replenish` endpoint: check pool levels, publish SQS messages for gaps (pool size = 3 per size+mode)
- **R-044** — `GET /puzzles/next?size=N&mode=M` endpoint: serve next ready puzzle with generation metadata, mark as served
- **R-045** — LocalStack SQS setup: local dev parity with two processes (API server + SQS consumer)
- **R-046** — Frontend: replace `generatePuzzle()` with `/puzzles/next`, remove advanced options from PuzzleSelector
- **R-047** — Frontend: show generation metadata (pipeline, solver, duration) subtly on puzzle page
- **R-048** — Frontend: "no puzzles available" state with retry button when pool is empty

## Not in Scope

- Verdict endpoint + UI — upvote/downvote/skip (Phase 5, schema included but not writable)
- Admin config endpoint + UI — per-combo generation settings, pool size tuning (Phase 4)
- Replay by ID — load any puzzle regardless of status (Phase 6)
- Analysis endpoint + agent — querying and interpreting puzzle data (Phase 7)
- Difficulty rating algorithm (Phase 8)
- Daily puzzles, production puzzle table (Phase 9+)

## Implementation Milestones

- **A: Infrastructure + Data Layer** — DynamoDB table, SQS queue, Terraform, LocalStack setup, puzzle repository
- **B: Generation Pipeline** — Generator Lambda (SQS consumer), replenish endpoint, generation with metadata tracking
- **C: Serving + Frontend** — `/puzzles/next` endpoint, frontend integration, metadata display, pool-empty state

## References

- ROADMAP.md: R-040 through R-048
- design-grill-summary.md (this directory)
- GLOSSARY.md: puzzle pool, candidate puzzle, region map, deducible
- PROJECT_STRUCTURE.md
