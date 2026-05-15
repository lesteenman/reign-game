# backend/ Index — Phase 0 Sweep (Track 3)

This is a read-only index of the Go backend that powers the Reign puzzle game. The backend is a single Go module (`github.com/eriksteenman/reign-game/backend`, Go 1.26) compiled into three Lambda binaries plus two dev/debug CLIs. The HTTP API uses chi + `aws-lambda-go-api-proxy`, DynamoDB via `aws-sdk-go-v2`, SQS for asynchronous puzzle generation, and Clerk for admin auth.

Top-level layout:

```
backend/
  cmd/        Lambda + CLI entrypoints (one per binary)
  internal/   Private packages — handler, service-like, repository, generator, etc.
  CLAUDE.md   Backend-specific conventions (layered architecture, logging, TDD)
  README.md   Local dev setup
  .golangci.yml  Lint config
  go.mod / go.sum
```

The documented layered architecture (`backend/CLAUDE.md`) is **Handler → Service → Domain/Repository**. In practice there is no `internal/service/` directory; orchestration code lives in `internal/daily/`, `internal/replenish/`, `internal/worker/`, and helper packages. Handlers also import `internal/repository` directly (see FINDINGS.md for the architecture verdict on this).

## Cross-reference index

- Per-package READMEs:
  - [`internal/generator/README.md`](internal/generator/README.md) — the puzzle-generation pipeline (dedicated, longer)
  - [`internal/handler/README.md`](internal/handler/README.md) — HTTP layer
  - [`internal/repository/README.md`](internal/repository/README.md) — DynamoDB layer
  - [`internal/auth/README.md`](internal/auth/README.md) — Clerk-backed admin middleware
  - [`internal/daily/README.md`](internal/daily/README.md) — daily-puzzle orchestration
- Per-package doc.go (Go-style):
  - `internal/auth/doc.go`, `internal/generator/doc.go`, `internal/awsclient/awsclient.go` (header), `internal/queue/publisher.go` (header), `internal/replenish/replenish.go` (header), `internal/repository/puzzle.go` (header), `internal/repository/daily.go` (header), `internal/worker/generator.go` (header), `internal/model/puzzle.go` (header), `internal/httperr/httperr.go` (header)
- Findings report: [`FINDINGS.md`](FINDINGS.md)

## File tree (one-line summary per file)

```
backend/
  cmd/
    api/
      main.go              — API Lambda + local-dev HTTP server entry point; also doubles as the SQS-consumer entry when GENERATOR_MODE=sqs. Wires Clerk, AWS, the chi router, and the reactive-replenish hook.
      main_test.go         — Smoke tests for the main package: router wiring + signal handling.
    daily-cron/
      main.go              — EventBridge-driven Lambda dispatcher for T-6h ensure / T=0 finalize daily-puzzle crons.
      main_test.go         — Dispatcher unit tests against a mock dailyService.
    genfixtures/
      main.go              — Dev CLI that produces deterministic puzzle fixtures (DynamoDB-JSON Item files) for the Playwright e2e suite.
    reproduce/
      main.go              — Dev CLI that regenerates a single puzzle from a recorded seed for debugging.

  internal/
    auth/
      doc.go               — Package documentation for the Clerk auth middleware.
      middleware.go        — RequireAuth / RequireAdmin / OptionalAuth chi middleware + cached *clerk.User lookup.
      middleware_test.go   — Middleware tests with a fake sessionVerifier.
      secret.go            — One-shot Clerk-secret loader (env-var first, SSM Parameter Store fallback).
      secret_test.go       — Secret-loader tests covering both resolution branches.
    awsclient/
      awsclient.go         — AWS SDK bootstrap helpers (LoadAWSConfig with transport isolation, NewDynamoDBClient).
      awsclient_test.go    — Env-driven config + endpoint override tests.
    daily/
      cron.go              — T-6h "ensure" algorithm: pick a candidate, conditionally put it.
      cron_test.go         — Cron-flow tests with a hand-rolled repository fake.
      sync.go              — T=0 "finalize" sync algorithm shared by handler sync-fallback + cron.
      sync_test.go         — Decision-tree + race-loser tests.
    generator/              ← See dedicated README.md (largest subsystem)
      bench/                 markdown reports from latency/feasibility benchmarks; no Go code
      brute.go              — Deterministic brute-force solver used for uniqueness check.
      brute_test.go         — Brute solver unit tests.
      classify.go           — Difficulty bucketing from a solver rule trace.
      classify_test.go      — Classify mapping tests.
      corpus_roundtrip_test.go, corpus_test.go — Corpus regression / roundtrip suites.
      diag_test.go          — Diagnostic / soak helpers (build-tag gated).
      distribution_test.go  — Difficulty distribution benchmark (build-tagged).
      doc.go                — Package overview (purity invariant INV-GEN-1).
      generator.go          — Public surface: Generator, Option, Generate orchestrator.
      generator_test.go, generator_bench_test.go — Public-API tests + benches.
      grower.go             — Cheap (random-weighted-frontier) region grower + initGrowState + bridging.
      grower_scored.go      — R-066 solver-guided grower variant (per-cell probe scoring).
      grower_test.go, grower_bench_test.go — Grower unit tests + bench.
      kcombos.go            — Canonical k-column-combo enumerator shared by sampler + brute solver.
      latency_distribution_test.go — Per-(N,k) latency benchmark (build-tagged).
      min_size_test.go      — Per-region min-size invariant tests.
      mutate.go             — Boundary-swap walker that escapes deductive-solver stalls.
      mutate_connectivity_test.go, mutate_test.go — Mutator + connectivity tests.
      neighbors.go          — 4-neighbor offsets + bfsRegionVisit BFS helper.
      output.go             — Region-array → [][]int conversion at the package boundary.
      output_test.go        — Conversion tests + panic-on-malformed-input.
      pair.go               — Mark grouping (k=1 identity, k=2 nearest-neighbor) for seed pairing.
      pair_test.go          — Pairing tests.
      probe_test.go         — End-to-end probe / solver round-trips.
      property_test.go      — Property-based tests (build-tagged).
      rules.go              — R1..R9 deductive-rule implementations (1034 lines; see FINDINGS).
      rules_test.go         — Per-rule fixtures + necessity tests.
      sample.go             — Solution sampler — row-by-row k-mark backtracker.
      sampler_bench_test.go, sample_test.go — Sampler tests + benches.
      soak_test.go          — Long-running soak (build-tagged).
      solver.go             — solve / solveWith fixed-point loop over the rule tiers.
      solver_bench_test.go, solver_cross_test.go, solver_state_test.go — Solver tests + benches.
      solver_state.go       — solverState type, init/reset/place/eliminate primitives, contradicts/solved.
      step7_test.go         — Step-7-gate regression suite (N=12 k=1).
      testhelpers_test.go   — Shared test helpers.
    handler/                ← See README.md
      admin_config.go       — PUT / POST admin config endpoints.
      admin_pool.go         — GET /admin/pool — per-combo ready counts + config view.
      auth_test.go          — Shared test helpers for admin-route tests (helpers only, no test methods).
      config_dto.go         — Handler-layer DTOs and validators for CONFIG endpoints.
      config_modes.go       — Public GET /api/config/modes endpoint for the landing page.
      daily.go              — Daily-puzzle GET + POST handlers (650 lines; see FINDINGS).
      generate.go           — Debug GET /api/puzzles/generate that runs the generator inline.
      health.go             — Health endpoint.
      params.go             — Shared mode constants + MarksPerUnitFromMode helper.
      replenish.go          — POST /admin/replenish — delegates to replenish.Sweep.
      serve.go              — GET /api/puzzles/next — pool-served puzzles with reactive replenish.
      status.go             — PUT /puzzles/{id}/status — solved / skipped lifecycle update.
      verdict.go            — PUT /api/admin/puzzles/{id}/verdict — admin verdict submission.
      *_test.go             — Per-handler unit tests.
    httperr/
      httperr.go            — Single WriteError helper for the canonical JSON error response shape.
      httperr_test.go       — Output-shape tests.
    model/
      puzzle.go             — Slim transport struct for the debug generate endpoint (single file).
    queue/
      publisher.go          — SQS Publisher + GenerationRequest schema.
      publisher_test.go     — Publisher tests against a mock SQSAPI.
    replenish/
      replenish.go          — Sweep + TryReactiveTopUp + NewAsyncHook (admin + reactive top-up).
      replenish_test.go     — Sweep + reactive top-up + async hook tests.
    repository/             ← See README.md
      daily.go              — Daily-puzzle DDB layer: schedule, candidate, play, leaderboard, transactional submit (805 lines; see FINDINGS).
      daily_test.go         — Daily-row layer tests.
      puzzle.go             — Puzzle / config / verdict DDB layer (681 lines; see FINDINGS).
      puzzle_test.go        — Repo tests.
      puzzle_auto_replenish_test.go — TryClaimAutoReplenish-specific tests.
    worker/
      generator.go          — SQS consumer + local poller; constructs the generator per message and writes the result.
      generator_test.go     — Worker unit tests against a fake store + SQS client.
```

## API endpoints (live, sourced from `cmd/api/main.go`)

| Method | Path | Auth | Handler |
|---|---|---|---|
| GET | `/api/health` | public | `handler.HealthCheck` |
| GET | `/api/puzzles/generate` | public | `handler.GenerateHandler` (debug; inline generation) |
| GET | `/api/puzzles/next` | public | `handler.ServeHandler` |
| PUT | `/api/puzzles/{id}/status` | public | `handler.StatusHandler` |
| GET | `/api/config/modes` | public | `handler.ConfigModesHandler` |
| GET | `/api/daily/{date}` | optional (cookie OR `X-Device-Id`) | `handler.DailyGetHandler` |
| POST | `/api/daily/{date}/result` | optional | `handler.DailySubmitHandler` |
| GET | `/api/admin/pool` | admin | `handler.AdminPoolHandler` |
| POST | `/api/admin/config` | admin | `handler.CreateConfigHandler` |
| PUT | `/api/admin/config/{size}/{mode}` | admin | `handler.UpdateConfigHandler` |
| PUT | `/api/admin/puzzles/{id}/verdict` | admin | `handler.VerdictHandler` |
| POST | `/api/admin/replenish` | admin | `handler.ReplenishHandler` |

## Subsystems at a glance

- **HTTP / chi router** — `cmd/api/main.go::newRouter` plus `internal/handler/*`. Routes are mounted under `/api`; admin routes inside a `r.Route("/admin", ...)` group that wires `RequireAuth + RequireAdmin` once.
- **Pool generation** — Producer/consumer split:
  - `internal/queue/publisher.go` publishes `GenerationRequest` messages.
  - `internal/worker/generator.go` consumes them (via `lambda.Start(w.HandleSQSEvent)` in prod, `RunLocalPoller` in local dev) and writes generated puzzles to DynamoDB.
- **Reactive replenish** — `internal/replenish/replenish.go::NewAsyncHook` builds a closure used by the serve handler, the daily handler, and `cmd/daily-cron` to fire fire-and-forget top-ups after a pool drain.
- **Daily puzzle** — `internal/daily/cron.go` (T-6h) + `internal/daily/sync.go` (T=0 finalize) compose the algorithms; the EventBridge Lambda `cmd/daily-cron/main.go` dispatches by `detail-type`; the API GET handler uses the same `SyncFinalizeForToday` as a synchronous fallback if no schedule row exists yet.
- **Auth** — `internal/auth/middleware.go` (chi middleware + Clerk SDK wrapper, with a 60s in-memory user cache); `internal/auth/secret.go` (one-shot SSM-backed Clerk secret loader with `sync.Once`).
- **AWS bootstrap** — `internal/awsclient/awsclient.go` clones `http.DefaultTransport` to isolate AWS from Clerk's `SetKey` global-transport mutation (documented lesson #4 in `CLAUDE.md`).
- **Generator** — `internal/generator/` is the algorithm core; see `internal/generator/README.md` for the pipeline and rule tiers.

## Build / lint

- `go.mod` pinned to Go 1.26.
- `.golangci.yml`: errcheck, govet, staticcheck, unused, ineffassign, gocritic, gofmt, goimports — gocritic's diagnostic + style + performance tags are all enabled.
- Tests use Go's testing package + table-driven style; arrange/act/assert comments are convention.
- Several test files use build tags (`distribution`, `latency`, `soak`, `property`) so the default `go test ./...` finishes quickly; the gated suites generate the markdown reports under `internal/generator/bench/`.
