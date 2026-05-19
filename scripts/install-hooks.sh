#!/usr/bin/env bash
# Install delegate hooks in $GIT_COMMON_DIR/hooks/ that forward to .githooks/.
#
# Why this and not `git config core.hooksPath .githooks`:
#   - core.hooksPath bypasses .git/hooks/ entirely, and we found in practice
#     that the setting didn't reliably stick across machines / worktrees.
#   - .git/hooks/ is per-clone, NOT per-worktree (see
#     https://www.gitworktree.org/guides/hooks). Delegate shims installed
#     here run from EVERY worktree of the same clone automatically.
#   - Run this script ONCE per fresh clone. Worktrees inherit it.
#
# Idempotent: re-running rewrites the delegate shims (cheap) and unsets
# core.hooksPath if present.
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
COMMON_DIR="$(cd "$(git rev-parse --git-common-dir)" && pwd -P)"
HOOK_DIR="$COMMON_DIR/hooks"
SRC_DIR="$REPO_ROOT/.githooks"

mkdir -p "$HOOK_DIR"

installed=()
for hook_path in "$SRC_DIR"/*; do
  [ -f "$hook_path" ] || continue
  hook="$(basename "$hook_path")"
  # Skip dotfiles and editor scratch files.
  case "$hook" in
    .*|*~|*.bak) continue ;;
  esac

  dest="$HOOK_DIR/$hook"
  cat > "$dest" <<EOF
#!/usr/bin/env bash
# Auto-installed by scripts/install-hooks.sh — DO NOT EDIT.
# Delegates to .githooks/$hook. Tries the current worktree first; falls back
# to the main worktree. The fallback matters for post-checkout, which fires in
# the NEW worktree during \`git worktree add\` before its checkout is complete,
# so the new worktree's .githooks/ may not yet contain the hook.
# .git/hooks/ is shared across worktrees of the same clone (see
# https://www.gitworktree.org/guides/hooks), so these delegate shims run from
# EVERY worktree of the same clone automatically.
HOOK="\$(git rev-parse --show-toplevel)/.githooks/$hook"
if [ ! -x "\$HOOK" ]; then
  MAIN_WORKTREE="\$(cd "\$(git rev-parse --git-common-dir)" && cd .. && pwd -P)"
  HOOK="\$MAIN_WORKTREE/.githooks/$hook"
fi
[ -x "\$HOOK" ] || exit 0
exec "\$HOOK" "\$@"
EOF
  chmod +x "$dest"
  installed+=("$hook")
done

if [ "${#installed[@]}" -eq 0 ]; then
  echo "No hooks found in $SRC_DIR — nothing installed."
  exit 0
fi

echo "Installed delegate hooks in $HOOK_DIR: ${installed[*]}"

# core.hooksPath would point git at .githooks directly, bypassing the
# delegate shims we just installed. Unset it so git uses the default
# .git/hooks/ location (where our shims live).
if git config --get core.hooksPath >/dev/null 2>&1; then
  git config --unset core.hooksPath
  echo "Unset core.hooksPath (using default .git/hooks/)"
fi
