# internal/handler/

HTTP layer for the Reign backend. Every handler is a chi-compatible `http.HandlerFunc` (or `http.Handler` for those whose factory signature is mandated by the chi `r.Method` form). Each handler:

- decodes inputs (path params via `chi.URLParam`, query via `r.URL.Query()`, body via `json.NewDecoder`),
- validates with package-local helpers (`validateSize`, `validateMode`, `validateConfigBody`, …),
- delegates to a narrow interface that's satisfied by the relevant `internal/service/*` package in production (e.g. `serveservice.Service`, `verdictservice.Service`) and a stub in tests,
- writes JSON responses with `application/json` Content-Type via either `json.NewEncoder(w).Encode(...)` or `httperr.WriteError(w, status, code, message)`.

## Data flow

- **In** — JSON over HTTP (API Gateway → Lambda → chi via `aws-lambda-go-api-proxy` in production; chi directly in local dev).
- **Calls** — Service interfaces (one per concern; see `internal/service/`) and, for the SQS-touching paths, `queue.Publisher` via the `MessagePublisher` interface. The reactive-replenish goroutine is injected as a `func(size int, mode string)` closure constructed in `cmd/api/main.go::buildReplenishHook` from `replenish.NewAsyncHook`.
- **Out** — JSON responses. Errors go through `httperr.WriteError`, which emits `{"error":"<code>","message":"<message>"}`.

## Auth integration

- Admin routes live behind a chi group whose `Use` chain is `auth.RequireAuth(NewClerkSessionVerifier()) → auth.RequireAdmin`. The chain is wired once in `cmd/api/main.go::newRouter`.
- The daily endpoints use `auth.OptionalAuth` and branch on `auth.UserFromContextOK` (signed-in users) vs `X-Device-Id` header (anonymous).
- `VerdictHandler` re-asserts `auth.UserFromContext(r.Context()) != nil` defensively (rater ID must come from the session, never the request body).

## Key files

| File | Responsibility |
|---|---|
| `health.go` | `GET /api/health`. |
| `generate.go` | `GET /api/puzzles/generate` (debug; inline generation). Also hosts `parseSizeMode` and `newUUIDv4` (consolidated in `internal/uuid/`). |
| `serve.go` | `GET /api/puzzles/next`. Fetches a ready puzzle, marks served, fires reactive replenish. |
| `status.go` | `PUT /api/puzzles/{id}/status` — accept `solved` / `skipped`. |
| `daily.go` | `GET /api/daily/{date}` + `POST /api/daily/{date}/result`. ~230 lines after Track 3 refactor moved orchestration into `internal/service/daily/`. Parses HTTP inputs and translates `daily.Err*` sentinels to status codes. |
| `verdict.go` | `PUT /api/admin/puzzles/{id}/verdict`. |
| `admin_pool.go` | `GET /api/admin/pool` — config + per-combo ready counts with per-step timing. |
| `admin_config.go` | `PUT /api/admin/config/{size}/{mode}` + `POST /api/admin/config`. |
| `config_modes.go` | `GET /api/config/modes` — public enabled-modes listing for the landing page. |
| `replenish.go` | `POST /api/admin/replenish` — delegates to `replenish.Sweep`. |
| `config_dto.go` | DTOs (`ConfigBody`, `ConfigView`, `ConfigCreateRequest`, `ConfigUpdateRequest`) + mapper functions + validators. |
| `params.go` | Shared `ModeStandard / ModeDouble` constants + `MarksPerUnitFromMode`. |
| `auth_test.go` | Helpers (no Test* methods) for mounting an admin route with a fake `sessionVerifier`. |

## Layer rules specific to this directory

The documented architecture (`backend/CLAUDE.md`) says handlers should call **services** and never the repository / queue / AWS SDK directly. After the Track 3 refactor, `internal/service/` exists (containing `config`, `daily`, `pool`, `replenish`, `serve`, `status`, `verdict` packages) and the verdict, daily, serve, status, and pool handlers now depend on service interfaces. A handful of handlers (`config_modes`, `serve`, `replenish`, `config_dto`, `admin_config`, `worker`) still import repository for shared DTOs — that drift is tracked as a follow-up (43-48 sweep).

## Common patterns

- **Reactive replenish hook** — `ServeHandler` and `DailyGetHandler` both accept a `func(size int, mode string)` plumbed through from `cmd/api/main.go`. `nil` is treated as no-op so local dev without SQS still works.
- **Per-step timing** — Multi-call handlers (`AdminPoolHandler`, `DailyGetHandler`, `DailySubmitHandler`) log `total_ms=N step1_ms=N step2_ms=N` per request, matching the convention in `backend/CLAUDE.md`.
- **Logging prefix** — Every log line starts with a subsystem prefix: `daily get:`, `daily submit:`, `admin pool:`, `verdict:`, `auth:`, etc.
- **Race-loser handling** — `ServeHandler` maps `repository.ErrPuzzleNotFound` from `MarkServed` to 404; `DailySubmitHandler` maps `repository.ErrPlayNotInStartedState` to 409 with `"already solved"`.
