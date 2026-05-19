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

# Private: emit error to stderr.
_dev_lib_die() {
  echo "$1" >&2
}

# start_port_service NAME PORT COMMAND LOG_FILE \
#   [--health-url URL] [--pid-file FILE] [--timeout SECONDS]
#
# Idempotent on PORT having a listener. Launches COMMAND in background,
# overwrites LOG_FILE on each launch, waits up to TIMEOUT seconds (default 60)
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
