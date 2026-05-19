# Taskfile dev/e2e Service Dedup — Design

**Issue:** [#121 — Dedup Taskfile.yml dev:up/down blocks via helper task](https://github.com/lesteenman/reign-game/issues/121)
**Date:** 2026-05-19
**Status:** Approved (awaiting writing-plans)
**Type:** Refactor — no behavior change, no new tests required.

## Context

`Taskfile.yml` has eight near-duplicate service-lifecycle blocks (four port-based starts, two PID-based starts, two PID-based stops). The four port-based stops already share an internal Task helper (`_dev:down:port`, ~line 475) — partial prior dedup. The PID-based and start-side blocks were never deduplicated.

The shapes:

| Task | Start shape | Stop shape | LOC (start+stop) |
|---|---|---|---|
| `dev:up:backend` / `dev:down:backend` | Port + HTTP health + wrapper-PID-for-orphan-detection | Port (existing helper) | ~60 + ~7 |
| `dev:up:frontend` / `dev:down:frontend` | Port-listen only | Port (existing helper) | ~23 + ~4 |
| `dev:up:generator` / `dev:down:generator` | PID file + ready-via-log-grep | PID-based (inline ~33 lines) | ~50 + ~33 |
| `e2e:up:backend` / `e2e:down:backend` | Port + HTTP health + wrapper-PID | Port (existing helper) | ~50 + ~6 |
| `e2e:up:frontend` / `e2e:down:frontend` | Port-listen only | Port (existing helper) | ~27 + ~3 |
| `e2e:up:generator` / `e2e:down:generator` | PID file + ready-via-log-grep | PID-based (inline ~33 lines) | ~50 + ~33 |

Net duplication: ~370 lines of bash-in-YAML across six service shapes.

## Decisions

### Mechanism: sourced shell library, not internal Task helper

Each service-specific task keeps its `env:`/`dotenv:`/`dir:` block (genuinely service-specific) but its `cmds:` collapse to a single `bash <<'BASH' ... BASH` heredoc that sources `scripts/dev/lib.sh` and calls one of four functions: `start_port_service`, `start_pid_service`, `stop_port_service`, `stop_pid_service`.

Alternative considered and rejected: pure-Task internal helpers via `task: _dev:up:port-service` + `vars:`. Hits Task's env-doesn't-propagate-to-callee limitation — each caller would either re-declare env on the helper (defeats dedup) or wrap the call in an awkward env-export line. Sourced shell lib avoids the issue because the caller's `env:` is already exported into its shell when the heredoc runs.

The existing `_dev:down:port` Task-internal helper is removed; its callers switch to `stop_port_service` via the lib. One mechanism, not two.

### Function API

`scripts/dev/lib.sh` exposes:

```
start_port_service NAME PORT COMMAND LOG_FILE \
  [--health-url URL] [--pid-file FILE] [--timeout SECONDS]

start_pid_service NAME COMMAND LOG_FILE PID_FILE \
  --ready-grep STRING [--timeout SECONDS]

stop_port_service NAME PORT [--pid-file FILE]

stop_pid_service NAME PID_FILE
```

All functions are idempotent and verify their post-condition before returning success — matching the existing inline behaviour (e.g. `_dev:down:port` refuses to claim "stopped" while the port is still in use; the lib preserves that semantic).

Path resolution: lib sets `DEV_LIB_ROOT="$(git rev-parse --show-toplevel)"` at source-time so callers can pass paths relative to the project root OR relative to the caller's `dir:` interchangeably.

### Backward compatibility — preserve PID/log paths byte-identical

`_dev:preflight:orphans` and `dev:status` introspect `logs/dev-backend.pid`, `logs/e2e-backend.pid`, `logs/generator.pid`, `logs/e2e-generator.pid`, `logs/backend.log`, `logs/frontend.log`, `logs/generator.log` etc. The lib preserves these paths verbatim so neither task needs modification.

The ready-grep strings stay identical too (e.g. `'starting local SQS poller'` for the generator).

### Per-task new shape (illustrative)

Port-based start with HTTP health + orphan-PID tracking:

```yaml
dev:up:backend:
  desc: Start backend in background (logs -> logs/backend.log)
  deps: [dev:up:localstack, _dev:preflight:orphans]
  dir: backend
  dotenv: ['.env.local']
  env: { DYNAMODB_ENDPOINT: '{{.LOCALSTACK_URL}}', ... }
  cmds:
    - mkdir -p ../logs
    - |
      bash <<'BASH'
      source "$(git rev-parse --show-toplevel)/scripts/dev/lib.sh"
      start_port_service backend 5181 'go run ./cmd/api' ../logs/backend.log \
        --health-url http://localhost:5181/api/health \
        --pid-file ../logs/dev-backend.pid \
        --timeout 60
      BASH
```

PID-based start:

```yaml
dev:up:generator:
  desc: Start SQS generator worker in background (logs -> logs/generator.log)
  deps: [dev:up:localstack, _dev:preflight:orphans]
  dir: backend
  env: { GENERATOR_MODE: sqs, ... }
  cmds:
    - mkdir -p ../logs
    - |
      bash <<'BASH'
      source "$(git rev-parse --show-toplevel)/scripts/dev/lib.sh"
      start_pid_service generator 'go run ./cmd/api' ../logs/generator.log \
        ../logs/generator.pid \
        --ready-grep 'starting local SQS poller' \
        --timeout 30
      BASH
```

Port-based stop:

```yaml
dev:down:backend:
  desc: Stop the backend. Verifies the port is free before returning success.
  cmds:
    - |
      bash <<'BASH'
      source "$(git rev-parse --show-toplevel)/scripts/dev/lib.sh"
      stop_port_service backend 5181 --pid-file logs/dev-backend.pid
      BASH
```

PID-based stop:

```yaml
dev:down:generator:
  desc: Stop the generator worker. Verifies the PID is gone before returning success.
  cmds:
    - |
      bash <<'BASH'
      source "$(git rev-parse --show-toplevel)/scripts/dev/lib.sh"
      stop_pid_service generator logs/generator.pid
      BASH
```

## Migration order

Refactor in steps that keep the dev stack working continuously — no big-bang cutover.

1. Add `scripts/dev/lib.sh` with all four functions. `bash -n` parse-check. No Taskfile change.
2. Convert `dev:up:generator` (PID-based, higher-risk because of stale-PID + race ordering). Smoke-test manually: start, idempotent re-start, ready signal, log output.
3. Convert `dev:down:generator`. Smoke-test stop + idempotent re-stop.
4. Convert `dev:up:backend` (port-based with HTTP health + orphan-PID). Smoke-test health-check path + verify `_dev:preflight:orphans` still detects the PID file correctly.
5. Mirror to the remaining four: `dev:up:frontend`, `e2e:up:backend`, `e2e:up:generator`, `e2e:up:frontend` (plus their `dev:down:*` / `e2e:down:*` counterparts).
6. Delete `_dev:down:port`. Update `dev:down:{backend,frontend}` and `e2e:down:{backend,frontend}` to call `stop_port_service` via the lib directly.
7. Sweep: grep for any remaining inline `nohup`, `lsof -ti:`, `kill -0` in the dev:/e2e: sections. Should be zero on migrated tasks.

## Verification

The Taskfile's behaviour isn't unit-testable at the level needed (background processes + OS port state). Verification is parse-check + manual smoke:

- `bash -n scripts/dev/lib.sh` after each lib edit.
- `shellcheck scripts/dev/lib.sh` if installed locally; non-blocking (project doesn't use shellcheck in CI today and adding it is out of scope).
- Per-task: `task <task-name>` start, run again to confirm idempotent message, `task dev:status` to confirm state, then `task dev:down:<service>`.
- End-to-end: `task dev:up && task dev:status && task dev:down && task dev:status && task e2e:up && task e2e:down` — clean state after.
- An existing Playwright e2e (e.g. `daily-flow.spec.ts`) — confirms the rebuilt e2e stack serves correctly under real traffic.

No new unit tests for the lib. At this scale, bats/shellspec ceremony exceeds the value.

## Risks

- **`_dev:preflight:orphans`** keys on PID file paths and shell-tested invariants. The lib preserves paths byte-identical; verify during step 4 that the orphan task still sees the dev-backend.pid file in the same place.
- **`dev:status`** introspects the same PID + port surface. Same guarantee.
- **Task's mvdan/sh heredoc handling.** The existing `bash <<'BASH'` pattern already works in dev:up:backend etc., so no new risk introduced.

## Out of scope

- Adding shellcheck to CI/pre-commit (separate decision; surgical scope here).
- Rewriting `_dev:preflight:orphans` or `dev:status`.
- `dev:up:localstack` (docker-based, doesn't fit the port/PID shape).
- `e2e:seed` / `e2e:genfixtures` (different concern).
- Unit-testing the lib functions.

## PR scope

Single branch `refactor/121-taskfile-dev-dedup`, single PR. Key Decisions section in the PR body lists: sourced-shell-lib vs Task-internal-helper choice, scope widened to include e2e + PID-based dedup (not just dev:up/down per literal issue title), `_dev:down:port` removed in favour of `stop_port_service`. No frontend/backend changes, so no `requesting-code-review` security trigger (the only thing this touches is `Taskfile.yml` + `scripts/dev/lib.sh`).
