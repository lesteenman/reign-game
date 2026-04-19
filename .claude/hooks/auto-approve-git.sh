#!/usr/bin/env bash
#
# PreToolUse hook: auto-approve Bash commands that consist entirely of
#   - `git ...` subcommands, and/or
#   - `cd <project-root>` (without trailing path),
# joined by `&&`, `||`, or `;`.
#
# Anything else — including `cd <subdir> && git ...` — stays silent and
# falls through to Claude Code's normal approval prompt. Rationale: the
# project owner wants the friction preserved when `cd` points at a
# subdirectory, because the correct idiom is `git -C <repo-root> ...`.
#
# Input:  the PreToolUse JSON envelope on stdin (see Claude Code hook docs).
# Output: JSON allow decision on approve; silent otherwise.
# Exit:   always 0 — never block; the decision is conveyed via stdout JSON.

set -euo pipefail

INPUT="$(cat)"
COMMAND="$(printf '%s' "$INPUT" | jq -r '.tool_input.command // empty' 2>/dev/null || true)"

# Nothing to evaluate — defer to the normal flow.
[ -z "$COMMAND" ] && exit 0

# Resolve the project root. Claude Code sets $CLAUDE_PROJECT_DIR for hooks;
# if it's missing (e.g., running the script manually), fall back to the repo
# root relative to this script's location.
ROOT="${CLAUDE_PROJECT_DIR:-}"
if [ -z "$ROOT" ]; then
  ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
fi
ROOT="${ROOT%/}"

# Defense-in-depth: command substitution can smuggle non-git code into a
# string that otherwise looks like `git ...`. If we see `$(` or a backtick,
# bail out and let the normal prompt handle it.
if [[ "$COMMAND" == *'$('* || "$COMMAND" == *'`'* ]]; then
  exit 0
fi

# Split on top-level operators. Order matters: &&/|| must be substituted
# before single & / |, otherwise we'd double-split them. Does NOT understand
# quoting — a literal operator inside a quoted string also splits, which
# just means the hook stays silent (safe fallthrough).
normalized="$COMMAND"
normalized="${normalized//&&/$'\x1e'}"
normalized="${normalized//||/$'\x1e'}"
normalized="${normalized//|/$'\x1e'}"
normalized="${normalized//;/$'\x1e'}"
normalized="${normalized//&/$'\x1e'}"

parts=()
while IFS= read -r line; do
  trimmed="$(printf '%s' "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  [ -z "$trimmed" ] && continue
  parts+=("$trimmed")
done < <(printf '%s\n' "$normalized" | tr $'\x1e' '\n')

# Zero parts (e.g., whitespace-only command) — defer.
[ "${#parts[@]}" -eq 0 ] && exit 0

all_ok=true
for part in "${parts[@]}"; do
  # Any git invocation — `git`, `git status`, `git -C /path log`, etc.
  if [[ "$part" == "git" || "$part" == git\ * ]]; then
    continue
  fi
  # `cd` only to the project root. Reject `cd <subdir>` by design.
  if [[ "$part" == "cd $ROOT" || "$part" == "cd $ROOT/" || "$part" == "cd \"$ROOT\"" ]]; then
    continue
  fi
  all_ok=false
  break
done

if [ "$all_ok" = true ]; then
  jq -n --arg reason "git-only compound (cd restricted to project root) — auto-approved by .claude/hooks/auto-approve-git.sh" \
    '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "allow", permissionDecisionReason: $reason}}'
fi

exit 0
