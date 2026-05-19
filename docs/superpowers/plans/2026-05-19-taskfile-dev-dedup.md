# Taskfile dev/e2e Service Dedup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse six near-duplicate service-lifecycle blocks in `Taskfile.yml` (4 port-based, 2 PID-based) into thin callers of a shared `scripts/dev/lib.sh`.

**Architecture:** Sourced shell library with four functions (`start_port_service`, `start_pid_service`, `stop_port_service`, `stop_pid_service`). Each per-service Taskfile entry keeps its `env:`/`dotenv:`/`dir:` (service-specific) but its `cmds:` shrinks to a single `bash <<'BASH'` heredoc that sources the lib and calls one function. Env propagation works because the caller's shell already has the env set when the heredoc runs — no Task-internal env-passing hackery needed.

**Tech Stack:** bash, [Taskfile](https://taskfile.dev/) (mvdan/sh-based runner).

**Spec:** `docs/superpowers/specs/2026-05-19-taskfile-dev-dedup-design.md`
**Issue:** [#121](https://github.com/lesteenman/reign-game/issues/121)
**Branch:** `refactor/121-taskfile-dev-dedup` (worktree at `.claude/worktrees/refactor+121-taskfile-dev-dedup`).

---

## File Structure

**Created:**
- `scripts/dev/lib.sh` — the shared library. ~150 lines, four exported functions + one private `_dev_lib_die` helper.

**Modified:**
- `Taskfile.yml`:
  - `dev:up:{backend,generator,frontend}` — rewrite cmds.
  - `dev:down:{backend,generator,frontend}` — rewrite cmds.
  - `e2e:up:{backend,generator,frontend}` — rewrite cmds.
  - `e2e:down:{backend,generator,frontend}` — rewrite cmds.
  - `_dev:down:port` (the existing internal helper, ~line 475) — deleted.

**Untouched (intentional):**
- `_dev:preflight:orphans` (~line 213) — keys on PID file paths the lib preserves byte-identical.
- `dev:status` (~line 630) — same PID/port introspection contract preserved.
- `dev:up:localstack`, `e2e:*` non-service tasks (seed, genfixtures) — out of scope.

---

## Task 1 — Create `scripts/dev/lib.sh`

**Files:**
- Create: `scripts/dev/lib.sh`

- [ ] **Step 1.1: Create the script**

```bash
mkdir -p scripts/dev
```

Then create `scripts/dev/lib.sh` with EXACTLY this content:

```bash
#!/usr/bin/env bash
# Shared bash functions for service lifecycle blocks in Taskfile.yml.
# Each function is idempotent and verifies its post-condition before
# returning success. Source from inside a `bash <<'BASH' ... BASH`
# heredoc inside a Taskfile cmd:
#
#   source "$(git rev-parse --show-toplevel)/scripts/dev/lib.sh"
#
# The caller's `env:` block (set on the calling Taskfile task) is
# inherited because the source happens inside the caller's shell —
# no env propagation hackery needed.

# Deliberately no `set -e` — callers (Taskfile cmds) control flow via
# function return codes.
set -u

DEV_LIB_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

# Private: emit error to stderr.
_dev_lib_die() {
  echo "$1" >&2
}

# start_port_service NAME PORT COMMAND LOG_FILE \
#   [--health-url URL] [--pid-file FILE] [--timeout SECONDS]
#
# Idempotent on PORT having a listener. Launches COMMAND in background,
# appends output to LOG_FILE, waits up to TIMEOUT seconds (default 60)
# for either:
#   - HEALTH_URL to return 2xx (when provided), OR
#   - lsof to see a listener on PORT (otherwise).
# Optionally writes the wrapper PID to PID_FILE for orphan detection.
# On timeout: kills wrapper (if PID_FILE), prints error, returns 1.
start_port_service() {
  local name=$1 port=$2 command=$3 log_file=$4
  shift 4
  local health_url="" pid_file="" timeout=60
  while [ $# -gt 0 ]; do
    case "$1" in
      --health-url) health_url=$2; shift 2 ;;
      --pid-file)   pid_file=$2;   shift 2 ;;
      --timeout)    timeout=$2;    shift 2 ;;
      *) _dev_lib_die "start_port_service: unknown flag $1"; return 1 ;;
    esac
  done

  local existing
  existing=$(lsof -ti:"$port" -sTCP:LISTEN 2>/dev/null || true)
  if [ -n "$existing" ]; then
    echo "$name already running on :$port (PID $existing)"
    return 0
  fi

  : > "$log_file"
  nohup bash -c "$command" >> "$log_file" 2>&1 </dev/null &
  local wrapper_pid=$!
  if [ -n "$pid_file" ]; then
    echo "$wrapper_pid" > "$pid_file"
  fi
  echo "$name starting, logs: $log_file"

  local i
  for i in $(seq 1 "$timeout"); do
    if [ -n "$health_url" ]; then
      if curl -sf "$health_url" >/dev/null 2>&1; then
        echo "$name ready on :$port (PID $(lsof -ti:"$port" -sTCP:LISTEN 2>/dev/null || echo '?'))"
        return 0
      fi
    else
      if lsof -ti:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
        echo "$name ready on :$port"
        return 0
      fi
    fi
    sleep 1
  done

  echo "$name did not become ready in ${timeout}s — check $log_file" >&2
  if [ -n "$pid_file" ]; then
    local wrapper
    wrapper=$(cat "$pid_file" 2>/dev/null || true)
    [ -n "$wrapper" ] && kill "$wrapper" 2>/dev/null || true
    rm -f "$pid_file"
  fi
  return 1
}

# start_pid_service NAME COMMAND LOG_FILE PID_FILE \
#   --ready-grep STRING [--timeout SECONDS]
#
# Idempotent via PID_FILE. Launches COMMAND in background, writes
# wrapper PID to PID_FILE, waits up to TIMEOUT seconds (default 30) for
# either:
#   - process to die early (return 1), OR
#   - LOG_FILE to contain READY_GREP string (return 0).
start_pid_service() {
  local name=$1 command=$2 log_file=$3 pid_file=$4
  shift 4
  local ready_grep="" timeout=30
  while [ $# -gt 0 ]; do
    case "$1" in
      --ready-grep) ready_grep=$2; shift 2 ;;
      --timeout)    timeout=$2;    shift 2 ;;
      *) _dev_lib_die "start_pid_service: unknown flag $1"; return 1 ;;
    esac
  done
  if [ -z "$ready_grep" ]; then
    _dev_lib_die "start_pid_service: --ready-grep is required"
    return 1
  fi

  if [ -f "$pid_file" ] && kill -0 "$(cat "$pid_file")" 2>/dev/null; then
    echo "$name already running (PID $(cat "$pid_file"))"
    return 0
  fi
  rm -f "$pid_file"

  : > "$log_file"
  nohup bash -c "$command" >> "$log_file" 2>&1 </dev/null &
  local pid=$!
  echo "$pid" > "$pid_file"
  echo "$name starting (PID $pid), logs: $log_file"

  local i
  for i in $(seq 1 "$timeout"); do
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "$name exited during startup — check $log_file" >&2
      rm -f "$pid_file"
      return 1
    fi
    if grep -q "$ready_grep" "$log_file" 2>/dev/null; then
      echo "$name ready (PID $pid)"
      return 0
    fi
    sleep 1
  done

  echo "$name did not report ready in ${timeout}s — check $log_file" >&2
  return 1
}

# stop_port_service NAME PORT [--pid-file FILE]
#
# SIGTERM → 5s wait → SIGKILL → verify port free. Optionally removes
# the orphan-detection PID file. Refuses to claim "stopped" while a
# listener remains (matches the existing _dev:down:port semantic).
stop_port_service() {
  local name=$1 port=$2
  shift 2
  local pid_file=""
  while [ $# -gt 0 ]; do
    case "$1" in
      --pid-file) pid_file=$2; shift 2 ;;
      *) _dev_lib_die "stop_port_service: unknown flag $1"; return 1 ;;
    esac
  done

  local pids
  pids=$(lsof -ti:"$port" 2>/dev/null || true)
  if [ -z "$pids" ]; then
    echo "$name not running"
    [ -n "$pid_file" ] && rm -f "$pid_file"
    return 0
  fi

  kill $pids 2>/dev/null || true
  local i
  for i in 1 2 3 4 5; do
    if ! lsof -ti:"$port" >/dev/null 2>&1; then break; fi
    sleep 1
  done

  local remaining
  remaining=$(lsof -ti:"$port" 2>/dev/null || true)
  if [ -n "$remaining" ]; then
    kill -9 $remaining 2>/dev/null || true
    sleep 1
  fi

  if lsof -ti:"$port" >/dev/null 2>&1; then
    echo "ERROR: $name still on :$port after SIGKILL (PID $(lsof -ti:"$port"))" >&2
    return 1
  fi

  [ -n "$pid_file" ] && rm -f "$pid_file"
  echo "$name stopped (was $pids)"
  return 0
}

# stop_pid_service NAME PID_FILE
#
# Reads PID, kills the process tree (pkill -P + kill, then -9
# escalation), verifies process is gone, removes PID_FILE. Refuses to
# claim "stopped" while the process is still alive.
stop_pid_service() {
  local name=$1 pid_file=$2

  if [ ! -f "$pid_file" ]; then
    echo "$name not running"
    return 0
  fi
  local pid
  pid=$(cat "$pid_file")
  if ! kill -0 "$pid" 2>/dev/null; then
    echo "$name not running (stale PID file removed)"
    rm -f "$pid_file"
    return 0
  fi

  pkill -P "$pid" 2>/dev/null || true
  kill "$pid" 2>/dev/null || true
  local i
  for i in 1 2 3 4 5; do
    if ! kill -0 "$pid" 2>/dev/null; then break; fi
    sleep 1
  done
  pkill -9 -P "$pid" 2>/dev/null || true
  kill -9 "$pid" 2>/dev/null || true
  sleep 1

  if kill -0 "$pid" 2>/dev/null; then
    echo "ERROR: $name PID $pid still alive after SIGKILL" >&2
    return 1
  fi

  rm -f "$pid_file"
  echo "$name stopped (was PID $pid)"
  return 0
}
```

- [ ] **Step 1.2: Make executable**

```bash
chmod +x scripts/dev/lib.sh
```

(Sourced scripts don't strictly need the exec bit, but it's tidy.)

- [ ] **Step 1.3: Parse-check**

```bash
bash -n scripts/dev/lib.sh
```

Expected: no output, exit 0.

- [ ] **Step 1.4: Smoke-test each function in isolation (no Taskfile yet)**

These commands exercise each function against trivial commands to verify the shell logic works. Run from project root.

```bash
mkdir -p /tmp/devlib-smoke
cd /tmp/devlib-smoke

# Sanity: source resolves
( source "$(git -C /Users/eriksteenman/Projects/reign-game rev-parse --show-toplevel)/scripts/dev/lib.sh"; type start_port_service start_pid_service stop_port_service stop_pid_service ) | head -8
```

Expected: each shows `<name> is a function`.

```bash
# Test stop_port_service no-op path (nothing on port 59999)
( source "$(git -C /Users/eriksteenman/Projects/reign-game rev-parse --show-toplevel)/scripts/dev/lib.sh"; stop_port_service noop 59999 )
```

Expected: `noop not running` + exit 0.

```bash
# Test stop_pid_service no-op path (no pid file)
( source "$(git -C /Users/eriksteenman/Projects/reign-game rev-parse --show-toplevel)/scripts/dev/lib.sh"; stop_pid_service noop /tmp/devlib-smoke/no-such-file )
```

Expected: `noop not running` + exit 0.

Don't try to exercise start_*service against fake commands in step 1 — those need the full Taskfile context to be meaningful, and tasks 2/4 verify them properly.

- [ ] **Step 1.5: Commit**

```bash
cd /Users/eriksteenman/Projects/reign-game/.claude/worktrees/refactor+121-taskfile-dev-dedup
git add scripts/dev/lib.sh
git commit -m "$(cat <<'EOF'
feat(infra): add scripts/dev/lib.sh for service lifecycle dedup (#121)

Four exported functions (start_port_service, start_pid_service,
stop_port_service, stop_pid_service) ready for Taskfile.yml to
adopt. Not yet wired into any task — that lands in subsequent
commits, one service at a time.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

Pre-commit hook should pass (no Go/TS staged).

---

## Task 2 — Convert `dev:up:generator` to use `start_pid_service`

**Files:**
- Modify: `Taskfile.yml` (the `dev:up:generator` block, around lines 384-433)

- [ ] **Step 2.1: Replace the cmds block**

Open `Taskfile.yml`, find the `dev:up:generator:` task (around line 384). REPLACE the entire task block (from `dev:up:generator:` to the end of the `BASH` heredoc) with EXACTLY:

```yaml
  dev:up:generator:
    desc: Start SQS generator worker in background (logs -> logs/generator.log)
    deps:
      - dev:up:localstack
      - _dev:preflight:orphans
    dir: backend
    env:
      GENERATOR_MODE: sqs
      DYNAMODB_ENDPOINT: '{{.LOCALSTACK_URL}}'
      SQS_ENDPOINT: '{{.LOCALSTACK_URL}}'
      SQS_QUEUE_URL: '{{.SQS_QUEUE_HOST}}/000000000000/puzzle-generation'
      AWS_REGION: us-east-1
      PUZZLE_TABLE_NAME: puzzle-pool
      AWS_ACCESS_KEY_ID: test
      AWS_SECRET_ACCESS_KEY: test
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

(The `env:` block is preserved verbatim from the original — same vars, same values.)

- [ ] **Step 2.2: Smoke-test cold start**

Make sure no generator is running:

```bash
task dev:down:generator 2>&1 | tail -3
```

Expected: either `generator not running` or `generator stopped (was PID …)`.

Now start fresh and watch the output:

```bash
task dev:up:generator 2>&1 | tail -10
```

Expected output ends with `generator ready (PID <number>)`.

Verify by inspecting state:

```bash
ls -la logs/generator.pid && cat logs/generator.pid && \
  ps -p "$(cat logs/generator.pid)" -o pid,command 2>&1 | tail -3
```

Expected: PID file exists, process is alive, `command` shows the `go run ./cmd/api` wrapper.

- [ ] **Step 2.3: Smoke-test idempotence**

Run the task a second time:

```bash
task dev:up:generator 2>&1 | tail -5
```

Expected output contains `generator already running (PID <same number>)`.

- [ ] **Step 2.4: Verify `dev:status` still sees it**

```bash
task dev:status 2>&1 | grep generator
```

Expected: `generator: running (PID <same number>)`.

- [ ] **Step 2.5: Tear down (cleanup before next task)**

```bash
task dev:down:generator 2>&1 | tail -3
```

Expected: `generator stopped (was PID …)`.

(Yes, the old inline `dev:down:generator` block still works fine — it's a separate task; we convert it in Task 3.)

- [ ] **Step 2.6: Commit**

```bash
git add Taskfile.yml
git commit -m "$(cat <<'EOF'
refactor(taskfile): dev:up:generator uses scripts/dev/lib.sh (#121)

First conversion: the PID-based generator start. Same env, same
output, same PID/log paths. Verified cold-start, idempotent restart,
dev:status still introspects correctly, and dev:down still tears
down (it's still on the old inline shape — converted in Task 3).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3 — Convert `dev:down:generator` to use `stop_pid_service`

**Files:**
- Modify: `Taskfile.yml` (the `dev:down:generator` block, around lines 531-563)

- [ ] **Step 3.1: Replace the cmds block**

Find `dev:down:generator:` (around line 531). REPLACE the entire task block with:

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

- [ ] **Step 3.2: Smoke-test stop (when running)**

```bash
task dev:up:generator >/dev/null 2>&1
task dev:down:generator 2>&1 | tail -3
```

Expected output ends with `generator stopped (was PID <number>)`.

Verify PID file removed and process is gone:

```bash
ls logs/generator.pid 2>&1
```

Expected: `ls: cannot access 'logs/generator.pid': No such file or directory`.

- [ ] **Step 3.3: Smoke-test stop (already stopped — idempotence)**

```bash
task dev:down:generator 2>&1 | tail -3
```

Expected: `generator not running`.

- [ ] **Step 3.4: Smoke-test stale PID file cleanup**

```bash
echo "99999" > logs/generator.pid   # PID guaranteed not to exist
task dev:down:generator 2>&1 | tail -3
ls logs/generator.pid 2>&1
```

Expected: `generator not running (stale PID file removed)`; then `No such file or directory`.

- [ ] **Step 3.5: Commit**

```bash
git add Taskfile.yml
git commit -m "$(cat <<'EOF'
refactor(taskfile): dev:down:generator uses scripts/dev/lib.sh (#121)

Drops the ~33-line inline pkill/kill escalation block in favour of
stop_pid_service. Same SIGTERM → 5s wait → SIGKILL semantic, same
post-condition verification, same PID file path.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4 — Convert `dev:up:backend` to use `start_port_service`

**Files:**
- Modify: `Taskfile.yml` (the `dev:up:backend` block, around lines 329-382)

This task adds the HTTP-health-check path and the wrapper-PID-for-orphan-detection path. Two new surface areas to verify.

- [ ] **Step 4.1: Replace the cmds block**

Find `dev:up:backend:` (around line 329). REPLACE the entire task block with:

```yaml
  dev:up:backend:
    desc: Start backend in background (logs -> logs/backend.log)
    deps:
      - dev:up:localstack
      - _dev:preflight:orphans
    dir: backend
    # Load dev secrets (e.g. CLERK_SECRET_KEY) from backend/.env.local.
    # See backend/.env.local.example for the template. Missing file is
    # fine for tests but backend/cmd/api will log.Fatal at startup if
    # no Clerk secret is resolvable from env or SSM.
    dotenv: ['.env.local']
    env:
      DYNAMODB_ENDPOINT: '{{.LOCALSTACK_URL}}'
      SQS_ENDPOINT: '{{.LOCALSTACK_URL}}'
      SQS_QUEUE_URL: '{{.SQS_QUEUE_HOST}}/000000000000/puzzle-generation'
      AWS_REGION: us-east-1
      PUZZLE_TABLE_NAME: puzzle-pool
      AWS_ACCESS_KEY_ID: test
      AWS_SECRET_ACCESS_KEY: test
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

(env + dotenv preserved verbatim.)

- [ ] **Step 4.2: Cold-start smoke**

Make sure no backend running:

```bash
task dev:down:backend 2>&1 | tail -3
```

Then:

```bash
task dev:up:backend 2>&1 | tail -10
```

Expected output ends with `backend ready on :5181 (PID <number>)`.

- [ ] **Step 4.3: Verify the orphan-PID file exists**

```bash
ls -la logs/dev-backend.pid && cat logs/dev-backend.pid
```

Expected: file exists, contains the wrapper PID (matches the `go run` process tree).

- [ ] **Step 4.4: Verify health endpoint serves**

```bash
curl -sf http://localhost:5181/api/health && echo " (ok)"
```

Expected output ends with `(ok)`.

- [ ] **Step 4.5: Verify `_dev:preflight:orphans` still recognizes the dev backend**

This is the critical regression check — the preflight task reads `logs/dev-backend.pid` to exclude the dev backend's process tree when e2e starts alongside.

```bash
# Start e2e backend WHILE dev backend is running.
# If the preflight breaks, this will refuse to start with an orphan error.
task e2e:up:backend 2>&1 | tail -10
```

Expected output ends with `e2e backend ready on :5182 …`. NOT `ERROR: orphan cmd/api process(es) detected`.

Tear down e2e to clean up:

```bash
task e2e:down:backend 2>&1 | tail -3
```

- [ ] **Step 4.6: Idempotence**

```bash
task dev:up:backend 2>&1 | tail -3
```

Expected: `backend already running on :5181 (PID …)`.

- [ ] **Step 4.7: Tear down**

```bash
task dev:down:backend 2>&1 | tail -3
```

Expected: `backend stopped (was …)`.

- [ ] **Step 4.8: Commit**

```bash
git add Taskfile.yml
git commit -m "$(cat <<'EOF'
refactor(taskfile): dev:up:backend uses scripts/dev/lib.sh (#121)

Port-based start with HTTP health-check + wrapper-PID-for-orphan-
detection. Verified cold-start, idempotence, health endpoint, and
that _dev:preflight:orphans still recognizes the dev backend's PID
when e2e starts alongside (regression-critical: GH #161).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5 — Convert `dev:up:frontend` to use `start_port_service`

**Files:**
- Modify: `Taskfile.yml` (the `dev:up:frontend` block, around lines 435-456)

Port-based but no HTTP health, no PID file. Simpler than backend.

- [ ] **Step 5.1: Replace the cmds block**

```yaml
  dev:up:frontend:
    desc: Start frontend in background (logs -> logs/frontend.log)
    dir: frontend
    cmds:
      - mkdir -p ../logs
      - |
        bash <<'BASH'
        source "$(git rev-parse --show-toplevel)/scripts/dev/lib.sh"
        start_port_service frontend 5180 'npm run dev -- --host 0.0.0.0' ../logs/frontend.log \
          --timeout 60
        BASH
```

- [ ] **Step 5.2: Cold-start smoke**

```bash
task dev:down:frontend 2>&1 | tail -3
task dev:up:frontend 2>&1 | tail -5
```

Expected output ends with `frontend ready on :5180`.

- [ ] **Step 5.3: Verify HMR is alive**

```bash
curl -sf http://localhost:5180/ | head -5
```

Expected: Vite's index.html (visible `<title>` or `<script type="module">`).

- [ ] **Step 5.4: Idempotence**

```bash
task dev:up:frontend 2>&1 | tail -3
```

Expected: `frontend already running on :5180 (PID …)`.

- [ ] **Step 5.5: Tear down**

```bash
task dev:down:frontend 2>&1 | tail -3
```

Expected: `frontend stopped (was …)`.

- [ ] **Step 5.6: Commit**

```bash
git add Taskfile.yml
git commit -m "$(cat <<'EOF'
refactor(taskfile): dev:up:frontend uses scripts/dev/lib.sh (#121)

Port-listen-only start (no health URL, no PID file). Verified
cold-start, HMR serves, idempotence.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6 — Convert all three `e2e:up:*` tasks

**Files:**
- Modify: `Taskfile.yml`:
  - `e2e:up:backend` (around lines 665-715)
  - `e2e:up:generator` (around lines 724-773)
  - `e2e:up:frontend` (around lines 809-835)

Mirror pattern to dev:up:* but with e2e-specific ports, paths, env, and PID file names.

- [ ] **Step 6.1: Replace `e2e:up:backend`**

Find `e2e:up:backend:` (around line 665). Replace entire task block with:

```yaml
  e2e:up:backend:
    desc: Start e2e backend on :5182 reading puzzle-pool-e2e
    deps:
      - dev:up:localstack
      - _dev:preflight:orphans
    dir: backend
    # Shares backend/.env.local with the dev backend so the e2e stack
    # picks up the same Clerk dev secret.
    dotenv: ['.env.local']
    env:
      PORT: "5182"
      DYNAMODB_ENDPOINT: '{{.LOCALSTACK_URL}}'
      SQS_ENDPOINT: '{{.LOCALSTACK_URL}}'
      # Publish replenish messages to the e2e queue so e2e:up:generator
      # (which polls puzzle-generation-e2e) can pick them up. Using the
      # dev `puzzle-generation` queue here would route messages to the
      # dev generator and starve the e2e generator — silent test break.
      SQS_QUEUE_URL: '{{.SQS_QUEUE_HOST}}/000000000000/puzzle-generation-e2e'
      AWS_REGION: us-east-1
      PUZZLE_TABLE_NAME: puzzle-pool-e2e
      AWS_ACCESS_KEY_ID: test
      AWS_SECRET_ACCESS_KEY: test
    cmds:
      - mkdir -p ../logs
      - |
        bash <<'BASH'
        source "$(git rev-parse --show-toplevel)/scripts/dev/lib.sh"
        start_port_service "e2e backend" 5182 'go run ./cmd/api' ../logs/e2e-backend.log \
          --health-url http://localhost:5182/api/health \
          --pid-file ../logs/e2e-backend.pid \
          --timeout 60
        BASH
```

(env + dotenv preserved.)

- [ ] **Step 6.2: Replace `e2e:up:generator`**

```yaml
  e2e:up:generator:
    desc: Start the e2e SQS generator worker (logs -> logs/e2e-generator.log, PID -> logs/e2e-generator.pid)
    deps:
      - dev:up:localstack
      - _dev:preflight:orphans
    dir: backend
    env:
      GENERATOR_MODE: sqs
      DYNAMODB_ENDPOINT: '{{.LOCALSTACK_URL}}'
      SQS_ENDPOINT: '{{.LOCALSTACK_URL}}'
      SQS_QUEUE_URL: '{{.SQS_QUEUE_HOST}}/000000000000/puzzle-generation-e2e'
      AWS_REGION: us-east-1
      PUZZLE_TABLE_NAME: puzzle-pool-e2e
      AWS_ACCESS_KEY_ID: test
      AWS_SECRET_ACCESS_KEY: test
    cmds:
      - mkdir -p ../logs
      - |
        bash <<'BASH'
        source "$(git rev-parse --show-toplevel)/scripts/dev/lib.sh"
        start_pid_service "e2e generator" 'go run ./cmd/api' ../logs/e2e-generator.log \
          ../logs/e2e-generator.pid \
          --ready-grep 'starting local SQS poller' \
          --timeout 30
        BASH
```

- [ ] **Step 6.3: Replace `e2e:up:frontend`**

```yaml
  e2e:up:frontend:
    desc: Start a second Vite on :5183 that proxies /api/* to the e2e backend at :5182
    dir: frontend
    env:
      REIGN_API_TARGET: http://localhost:5182
    cmds:
      - mkdir -p ../logs
      - |
        bash <<'BASH'
        source "$(git rev-parse --show-toplevel)/scripts/dev/lib.sh"
        start_port_service "e2e frontend" 5183 'npm run dev -- --port 5183 --host 0.0.0.0' ../logs/e2e-frontend.log \
          --timeout 60
        BASH
```

- [ ] **Step 6.4: Smoke-test all three e2e ups**

```bash
task e2e:up:backend 2>&1 | tail -5
task e2e:up:generator 2>&1 | tail -5
task e2e:up:frontend 2>&1 | tail -5
```

Expected output endings: `e2e backend ready on :5182 …`, `e2e generator ready (PID …)`, `e2e frontend ready on :5183`.

Verify all three:

```bash
curl -sf http://localhost:5182/api/health && echo " (e2e backend ok)"
curl -sf http://localhost:5183/ | head -1
ls logs/e2e-backend.pid logs/e2e-generator.pid
```

- [ ] **Step 6.5: Tear down e2e (still using the old `e2e:down:*` blocks)**

```bash
task e2e:down:frontend
task e2e:down:generator
task e2e:down:backend
```

(All three should still work — they're on the old shape until Task 7.)

- [ ] **Step 6.6: Commit**

```bash
git add Taskfile.yml
git commit -m "$(cat <<'EOF'
refactor(taskfile): e2e:up:{backend,generator,frontend} use scripts/dev/lib.sh (#121)

Mirrors the dev:up:* conversions to e2e:up:*. Same lib functions,
e2e-specific ports/paths/env preserved verbatim. e2e:down:* still
on the old inline shape — converted in Task 7.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7 — Convert all `*:down:*` port + pid tasks, delete `_dev:down:port`

**Files:**
- Modify: `Taskfile.yml`:
  - `dev:down:backend` (around lines 518-523)
  - `dev:down:frontend` (around lines 525-529)
  - `e2e:down:backend` (around lines 717-722)
  - `e2e:down:frontend` (around lines 837-841)
  - `e2e:down:generator` (around lines 775-807)
  - DELETE `_dev:down:port` (around lines 475-516)

- [ ] **Step 7.1: Replace `dev:down:backend`**

```yaml
  dev:down:backend:
    desc: Stop the backend (kills whatever is on :5181). Verifies the port is free before returning success.
    cmds:
      - |
        bash <<'BASH'
        source "$(git rev-parse --show-toplevel)/scripts/dev/lib.sh"
        stop_port_service backend 5181 --pid-file logs/dev-backend.pid
        BASH
```

- [ ] **Step 7.2: Replace `dev:down:frontend`**

```yaml
  dev:down:frontend:
    desc: Stop the frontend (kills whatever is on :5180). Verifies the port is free before returning success.
    cmds:
      - |
        bash <<'BASH'
        source "$(git rev-parse --show-toplevel)/scripts/dev/lib.sh"
        stop_port_service frontend 5180
        BASH
```

- [ ] **Step 7.3: Replace `e2e:down:backend`**

```yaml
  e2e:down:backend:
    desc: Stop the e2e backend. Verifies the port is free before returning success.
    cmds:
      - |
        bash <<'BASH'
        source "$(git rev-parse --show-toplevel)/scripts/dev/lib.sh"
        stop_port_service "e2e backend" 5182 --pid-file logs/e2e-backend.pid
        BASH
```

- [ ] **Step 7.4: Replace `e2e:down:frontend`**

```yaml
  e2e:down:frontend:
    desc: Stop the e2e frontend. Verifies the port is free before returning success.
    cmds:
      - |
        bash <<'BASH'
        source "$(git rev-parse --show-toplevel)/scripts/dev/lib.sh"
        stop_port_service "e2e frontend" 5183
        BASH
```

- [ ] **Step 7.5: Replace `e2e:down:generator`**

```yaml
  e2e:down:generator:
    desc: Stop the e2e generator worker. Verifies the PID is gone before returning success.
    cmds:
      - |
        bash <<'BASH'
        source "$(git rev-parse --show-toplevel)/scripts/dev/lib.sh"
        stop_pid_service "e2e generator" logs/e2e-generator.pid
        BASH
```

- [ ] **Step 7.6: Delete `_dev:down:port`**

Find the `_dev:down:port:` task (around line 475). Delete the entire block from `_dev:down:port:` through the `BASH` heredoc terminator (around line 516).

Verify nothing else references it:

```bash
grep -n "_dev:down:port" Taskfile.yml
```

Expected: no results.

- [ ] **Step 7.7: Smoke each stop in isolation**

```bash
# Start everything
task dev:up >/dev/null 2>&1
task e2e:up:backend >/dev/null 2>&1
task e2e:up:generator >/dev/null 2>&1
task e2e:up:frontend >/dev/null 2>&1

# Stop each via its own command and confirm
task dev:down:backend 2>&1 | tail -3       # → backend stopped
task dev:down:frontend 2>&1 | tail -3      # → frontend stopped
task e2e:down:backend 2>&1 | tail -3       # → e2e backend stopped
task e2e:down:frontend 2>&1 | tail -3      # → e2e frontend stopped
task e2e:down:generator 2>&1 | tail -3     # → e2e generator stopped
```

Each should print `<name> stopped (was …)`. Then idempotent re-run:

```bash
task dev:down:backend 2>&1 | tail -3       # → backend not running
```

- [ ] **Step 7.8: Commit**

```bash
git add Taskfile.yml
git commit -m "$(cat <<'EOF'
refactor(taskfile): all dev/e2e *:down:* use scripts/dev/lib.sh; remove _dev:down:port (#121)

Five down tasks (dev:down:{backend,frontend}, e2e:down:{backend,
frontend,generator}) switch to stop_port_service / stop_pid_service
via the lib. The _dev:down:port internal Task helper is deleted —
the lib is now the single mechanism for service shutdown.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8 — Full end-to-end smoke + sweep + PR

**Files:** none (verification + PR).

- [ ] **Step 8.1: Sweep grep for residual inline patterns**

```bash
grep -nE "nohup|lsof -ti:|kill -0|kill -9" Taskfile.yml
```

Expected results: ONLY in `_dev:preflight:orphans` (~line 200-330) and `dev:status` (~line 630). All other matches should be gone. If anything else surfaces, investigate before continuing.

- [ ] **Step 8.2: Full dev stack lifecycle**

```bash
task dev:down 2>&1 | tail -5     # baseline clean state
task dev:up 2>&1 | tail -15       # cold start
task dev:status 2>&1               # all running
task dev:restart:backend 2>&1 | tail -5    # exercise restart
task dev:restart:generator 2>&1 | tail -5
task dev:down 2>&1 | tail -5
task dev:status 2>&1               # all stopped
```

Each `dev:up`/`dev:down`/`dev:restart:*` command should complete cleanly. `dev:status` shows all-running then all-stopped.

- [ ] **Step 8.3: Full e2e stack lifecycle**

```bash
task e2e:up:backend 2>&1 | tail -5
task e2e:up:generator 2>&1 | tail -5
task e2e:up:frontend 2>&1 | tail -5
task e2e:down:frontend 2>&1 | tail -3
task e2e:down:generator 2>&1 | tail -3
task e2e:down:backend 2>&1 | tail -3
```

- [ ] **Step 8.4: Real-traffic e2e smoke**

Run one of the existing Playwright e2e specs to confirm the rebuilt e2e stack serves traffic correctly.

```bash
task e2e:up >/dev/null 2>&1 || task e2e:up:backend && task e2e:up:generator && task e2e:up:frontend
cd frontend && npx playwright test playwright/e2e/daily-flow.spec.ts 2>&1 | tail -20
cd ..
task e2e:down:frontend && task e2e:down:generator && task e2e:down:backend
```

Expected: Playwright test passes. If it fails because of an unrelated regression (not introduced here), note in the PR body; otherwise, fix surgically and re-run.

- [ ] **Step 8.5: Push**

```bash
git push -u origin refactor/121-taskfile-dev-dedup 2>&1 | tail -10
```

(Note: the harness-generated branch was named `worktree-refactor+121-taskfile-dev-dedup`; rename if desired with `git branch -m refactor/121-taskfile-dev-dedup` BEFORE push for a cleaner PR ref. If the rename causes any worktree-list weirdness, push as-is — the branch name is internal.)

Pre-push hook fires gitleaks; should pass cleanly.

- [ ] **Step 8.6: Open PR**

```bash
gh pr create --repo lesteenman/reign-game \
  --title "refactor(taskfile): dedup dev/e2e service lifecycle via scripts/dev/lib.sh (#121)" \
  --body "$(cat <<'EOF'
## Summary

Closes #121. Six near-duplicate service-lifecycle blocks in `Taskfile.yml` (4 port-based starts, 2 PID-based starts, plus matching stops) collapse to thin callers of a new `scripts/dev/lib.sh`.

## Changes

- `scripts/dev/lib.sh` — new. Four functions: `start_port_service`, `start_pid_service`, `stop_port_service`, `stop_pid_service`. Each is idempotent and verifies its post-condition before returning success.
- `Taskfile.yml` — all 11 service-lifecycle tasks (`dev:up:{backend,generator,frontend}`, `dev:down:{backend,generator,frontend}`, `e2e:up:{backend,generator,frontend}`, `e2e:down:{backend,generator,frontend}`) rewritten as thin bash heredocs that source the lib and call one function.
- `Taskfile.yml` — `_dev:down:port` internal helper deleted; superseded by `stop_port_service` in the lib.

Net: ~370 lines of bash-in-YAML → ~80 lines + a single ~150-line library file. One mechanism for service lifecycle, not three.

## Key Decisions

- **Sourced shell lib, not internal Task helper.** Task's `env:` block doesn't propagate to callees via `task: _foo`, which would have forced each caller to re-declare env on the helper (defeating the dedup) or wrap calls in awkward env-export commands. Sourcing a shell lib from inside `bash <<'BASH'` inherits the caller's env naturally.
- **Scope widened beyond the issue's literal title.** #121 named `dev:up/down`; this PR also includes `e2e:up/down` since the duplication pattern was identical and partial dedup invites drift. Discussed in spec.
- **`_dev:down:port` removed.** Old mechanism replaced by `stop_port_service`. One way to do it.
- **No unit tests for the lib.** Behaviour requires background processes + OS port state; bats/shellspec ceremony exceeds the value at this scale. Manual smoke + parse-check is the verification (see test plan).

## Spec / plan

- Spec: `docs/superpowers/specs/2026-05-19-taskfile-dev-dedup-design.md`
- Plan: `docs/superpowers/plans/2026-05-19-taskfile-dev-dedup.md`

## Test plan

- [x] `bash -n scripts/dev/lib.sh` parses clean.
- [x] Per-task smoke: cold start, idempotent re-run, stop, idempotent re-stop. Each task verified individually as it was converted.
- [x] `_dev:preflight:orphans` still recognizes the dev backend when e2e starts alongside (GH #161 regression-critical).
- [x] `dev:status` still introspects all services correctly.
- [x] Full dev stack lifecycle (`task dev:up && task dev:status && task dev:down`).
- [x] Full e2e stack lifecycle.
- [x] Playwright e2e (`daily-flow.spec.ts`) passes on the rebuilt e2e stack.
- [x] Sweep grep — no residual inline `nohup`/`lsof -ti:`/`kill -0` outside `_dev:preflight:orphans` and `dev:status`.

## Out of scope

- shellcheck in CI / pre-commit.
- Rewriting `_dev:preflight:orphans` or `dev:status`.
- `dev:up:localstack` (docker-based, doesn't fit the port/PID shape).

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)" 2>&1 | tail -3
```

Return the PR URL.

- [ ] **Step 8.7: Wait for CI**

```bash
gh pr checks <PR_NUMBER> --repo lesteenman/reign-game 2>&1 | tail -10
```

Expected: all checks pass. CI doesn't exercise Taskfile lifecycle directly (it runs `go test` + `npm test`), so the risk surface is mostly local manual verification — already done above.

---

## Out-of-plan housekeeping (only if hit)

- If the pre-push gitleaks scan reports a false positive in the lib (e.g. an example URL that triggers the scan), update `.gitleaks.toml` or the lib comment to avoid the trigger. Do NOT bypass with `--no-verify`.
- If `_dev:preflight:orphans` breaks on the new PID file shape (it shouldn't — paths are preserved byte-identical), STOP and re-read both files side-by-side. Don't paper over with a workaround.
- If `dev:status` shows incorrect state after conversion, the regression is in the lib or the PID file lifecycle. Diff against pre-conversion behaviour and fix surgically.
