# Backend — Go Conventions

This file is auto-loaded by Claude Code when working on files under `backend/`. The project-wide rules live in `/CLAUDE.md`; this file is additive.

## Layered Architecture

The backend has two "edge" subsystems (`handler/` for HTTP, `worker/` for SQS), a `service/` application layer for orchestration, persistence (`repository/`, `queue/`), and pure domain (`domain/`, `mode/`, `generator/`). The `architecture` skill is the canonical spec.

| Layer | Directory | Allowed callees | Forbidden callees |
|---|---|---|---|
| **Edge: HTTP** | `internal/handler/` | service, mode, httperr, generator (debug only) | repository, queue, AWS SDK directly |
| **Edge: SQS consumer** | `internal/worker/` | service, mode, generator | handler, repository, queue directly |
| **Service** (application) | `internal/service/` | repository, queue, domain, mode, generator, awsclient | handler, worker |
| **Persistence** | `internal/repository/`, `internal/queue/` | AWS SDK, domain, mode | handler, worker, service |
| **Pure / domain** | `internal/mode/`, `internal/generator/` (and `internal/domain/` if created) | external libs only | anything else under `internal/` |
| **Infra adapters** | `internal/awsclient/`, `internal/auth/`, `internal/httperr/` | AWS SDK, external libs, domain | handler, worker (callable but not imported) |

Key rules:
- **Multi-leg DDB transactions live in `service/`, not `repository/`.** Repository methods are single transactions of single-row scope OR a single `TransactWriteItems`/`TransactGetItems` call with no orchestration logic.
- **`MarksPerUnitFromMode` and friends live in `internal/mode/`** — imported by both handler and worker. No worker → handler imports.

Drift detection: see `.claude/skills/architecture/SKILL.md` backend section for the full grep set.

## Go Conventions

- **Project layout.** `cmd/` for entry points, `internal/` for private packages.
- **Tests.** Table-driven preferred. Arrange-Act-Assert with explicit `// Arrange`, `// Act`, `// Assert` comments.
- **Doc comments.** Required on exported functions.
- **Comments describe current state only.** No issue/PR/slice references in code comments (`// added in #327`, `// was X before #N`, `// lesson 13`) — git history + the commit message carry provenance. The issue number belongs in the commit message and PR description, never in the source.
- **Error handling.** Wrap errors with context: `fmt.Errorf("doing X: %w", err)`.
- **No global mutable state.** Pass dependencies via struct fields.
- **Return directly.** When a value is computed and immediately returned, return it directly — don't assign to a local first. Exception: when the variable name aids readability of a complex expression.
- **Security.** Validate and sanitize all input from external sources. Use parameterized queries — never concatenate. Principle of least privilege. Watch for sensitive data in logs/errors/responses. No hardcoded secrets.

## DynamoDB Access

- Single-table design where practical; separate tables when access patterns diverge.
- AWS SDK for Go v2 directly — no ORM.
- All table definitions in Terraform (`infra/modules/database/`).
- Persisted data shapes live in `repository/`. Define the type once where it's saved, import from every consumer — don't redeclare in a service module (it will drift).

## Orchestration Services

When a service aggregates data from multiple sources:
- **Fetch shared data once.** If multiple methods in the same request need the same data, fetch it once and pass it as a parameter.
- **Consistent error handling across code paths.** If one path uses graceful degradation, ALL similar paths in the same class must use the same pattern.
- **Pass pre-fetched data down.** Builder/mapper methods should accept their data as parameters, not fetch it themselves.

## Logging

Stdlib `log` only — no `slog`, no third-party loggers. Small project, small surface.

- **Format.** Every log line starts with `<subsystem>: <what>`. Subsystem is the handler name, package role, or service (`admin pool`, `config modes`, `serve handler`, `generator`). Keeps grep-by-feature trivial.
- **Levels are implicit.** `log.Printf` for warn/error. `log.Fatal*` is reserved for "can't continue at all" — startup failures, missing required config. Never for request-path errors.
- **Warnings get an explicit `WARN:` prefix** so grep can find them. Example: `"WARN: generator: safety-net fired 2 times on puzzle X (seed=Y)"`.
- **Pure packages stay silent.** `backend/internal/generator/` has zero `log.` calls. Pure layers surface signals via return values or struct fields (e.g. `Metrics.SafetyNetTrips`) and a caller logs.
- **Per-message lines** use key=value pairs separated by commas: `key1=val1, key2=val2`.
- **Per-step timing on multi-call handlers, by default.** Any handler issuing more than one downstream call (DDB + Clerk, multiple DDB queries, fan-out) logs per-step latency on every request. Format: `<subsystem>: total_ms=N step1_ms=N step2_ms=N`. The next slow request shows the bottleneck in one log line. Examples:
  - `auth: allow path=/api/admin/pool sub=user_... verify_ms=12 get_user_ms=8`
  - `admin pool: total_ms=27 configs_ms=12 combos=3 count_breakdown=[7#standard=3ms 9#double=2ms 9#standard=3ms]`

## Testing (TDD — non-negotiable)

Project-wide TDD rule in `/CLAUDE.md`. Backend-specific:
- Structure every test Arrange-Act-Assert with explicit `// Arrange`, `// Act`, `// Assert` comments.
- Run a single test: `go test -run TestName ./internal/<pkg>/...`.
- Aim for above 90% coverage on services and repositories.
- Test happy + unhappy + edge cases. Boundary values, NaN, Inf, empty strings, max values.
- Mock external dependencies (AWS clients, Clerk client), not the unit under test.
- Use `testing.Short()` to guard long-running tests; pre-push and CI must pass `-short` (see lesson #6 in infra/CLAUDE.md).
- **Verify with `-short`, matching CI** — full `go test ./internal/generator/...` (no `-short`) runs the property corpus + heavy solver tests and **exceeds the 600s Bash timeout**. Use `go test -short ./internal/generator/...` for the standard gate; run the corpus/soak separately and unbounded (`go test -run TestPropertyCorpus -timeout 30m ./internal/generator/`, or `task soak`). Never tell a subagent to run the non-`-short` generator suite as a check — it will time out before committing.

## Lessons (Reign-specific)

1. **Float API params: test NaN and Inf explicitly.** `strconv.ParseFloat` accepts both as valid. Compound checks like `x < 0 || x > 1` evaluate to false for NaN, letting it through. Use `math.IsNaN` explicitly.
2. **DynamoDB `Limit` applies BEFORE `FilterExpression`.** With `Query`, DDB reads up to Limit items *then* filters. `Limit=1` with a status filter can return 0 results even when matching items exist further in the partition. Either omit Limit (small partitions) or paginate.
3. **Standalone reproducer first when perf-bisecting an SDK/framework issue.** A 30-line standalone Go program that varies one suspect at a time beats three guesses at the wrong layer. Write the probe FIRST. The R-7-02 perf hunt spent ~30 min guessing at IPv6 / IMDS / connection-pool / DNS before a tiny standalone program identified `clerk.SetKey` mutating `http.DefaultTransport` in 5 minutes.
4. **Go SDKs that mutate `http.DefaultTransport` contaminate every other SDK in the same process.** Clerk's Go SDK v2's `clerk.SetKey()` wraps `http.DefaultClient`; the AWS Go SDK pays a multi-second cost on its first call as a result. Insulate each SDK at construction time with a dedicated `http.Client` backed by `http.DefaultTransport.(*http.Transport).Clone()`. The clone snapshots the underlying TCP transport into an independent state, detached from later global mutations. Documented in `backend/cmd/api/main.go::loadAWSConfig`. Measured cost: ~1.8s vs ~9ms on the first DDB Query when both SDKs share the default transport. **When integrating any new third-party Go SDK, audit whether it mutates the default transport; if yes, isolate every other SDK explicitly.**
5. **When an interface grows, grep every site that mirrors it.** When an exported interface in package A gains a method (or a re-derived interface in package B is meant to be a "subset" of A's), the mirror in B does NOT pick up the new method automatically. The compiler only fires when both packages are imported together (e.g. by `cmd/api/main.go`). Per-package tests pass at each layer in isolation while a latent type-mismatch waits to bite at integration time. After any interface change: `grep -rn 'interface' --include='*.go' <pkg>` to find every duck-typed twin. A compile-time `var _ A.Iface = (*B.MyType)(nil)` assert at the bottom of `B`'s file fails fast at unit-test compile.
6. **When two layers defend against the same condition, pick ONE policy.** Don't have layer A clamp while layer B rejects — layer A's clamp becomes a no-op and the user gets a generic 500 instead of the intended UX. Defense-in-depth is fine, but each layer must agree on the outcome. R-8-01 had a clock-skew bug: handler clamped negative `serverElapsedMs` to 0; repo rejected with an error. On the rare clock-skew path the handler's clamp was a no-op — the repo errored on the same input the handler thought it was defending against.
7. **`go vet` ≠ `golangci-lint`.** When asked to lint, run `golangci-lint run` (or invoke `git commit` to fire the pre-commit hook). `go vet` is a strict subset — it won't catch gocritic, unused, errcheck, staticcheck, or the 40+ other rules golangci-lint v2 enables. CI's `golangci-lint run` is the real gate.

## Pre-Commit Quality Checklist

Verify before declaring a backend task done:

1. **Unauthenticated test coverage.** For each controller test you create or modify, verify there is an unauthenticated access test expecting 401/403.
2. **Resource ownership on nested endpoints.** `/{userId}/items/{itemId}` must use scoped queries. Validate at the DB level.
3. **Test happy + unhappy + edge cases.** Every service method: success, error/rejection, boundary values.
4. **Remove imports YOUR changes orphaned.** If your edit made an import unused, remove it. Don't remove pre-existing dead code that wasn't already in scope of your change.
5. **Every bug fix needs a regression test.** No fix without a test that would have caught it.
6. **Null-check DTO fields before setting.** Partial updates must not erase fields the caller didn't send.
7. **Generic error messages in auth.** "Invalid credentials" — never reveal whether an account exists.
