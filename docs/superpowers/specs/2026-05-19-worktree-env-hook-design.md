# Worktree `post-checkout` Hook for `.env.local` Propagation — Design

**Issue:** [#191 — Worktree post-checkout hook to copy .env.local files from main worktree](https://github.com/lesteenman/reign-game/issues/191)
**Date:** 2026-05-19
**Status:** Approved (awaiting writing-plans)
**Type:** Tech-debt / dev-stack ergonomics.

## Context

PR #189 installed delegate shims in `.git/hooks/` (forwarding to `.githooks/pre-commit` and `.githooks/pre-push`) so every worktree of a clone runs the project's hooks without per-worktree setup. The shims auto-install whenever someone runs `scripts/install-hooks.sh`.

That fixed the hooks half of the problem. The other half — gitignored `.env.local` files — is still manual. New worktrees are branched from `origin/main`, so they don't include:

- `backend/.env.local` (Clerk dev secret → backend `log.Fatal` on startup without it)
- `frontend/.env.local` (`VITE_CLERK_PUBLISHABLE_KEY` etc. → Playwright `global-setup.ts` throws)

Hit twice during PR #190 work (#121 implementation): the Task 4 implementer symlinked `backend/.env.local` manually to recover; the lead later symlinked `frontend/.env.local` to get Playwright running. Both were workarounds for a missing automation step.

## Decisions

### Mechanism: symlink, not copy

Each new worktree gets symlinks pointing at the **main worktree's** `.env.local` files. Rotating a secret once (e.g. Clerk dev tenant key) propagates instantly to all worktrees.

Alternative considered: `cp -n` per-file copy. Gives per-worktree independence (an e2e worktree could point at a different Clerk tenant). Rejected: not needed in current workflow, and rotating a secret in one place is the bigger ergonomic win.

### Symlinks are absolute, not relative

`ln -s "$MAIN_WORKTREE/backend/.env.local" "$NEW_WORKTREE/backend/.env.local"`. Robust to CWD changes within the worktree. Breaks if the user later moves the main worktree, which is the correct semantic — that should be a noisy failure, not silently routed elsewhere.

### Hook fires only on first checkout

`post-checkout` is called with `prev_HEAD = 0000…` only on first checkout (covers `git worktree add` and `git clone`). Ordinary `git checkout BRANCH` in an existing worktree passes a real prev_HEAD; the hook short-circuits and exits 0 without touching anything.

This means: if a user deliberately deletes a worktree's `.env.local`, the hook does NOT auto-restore it on the next branch checkout. To restore, the user re-runs the hook explicitly (or just `ln -s` themselves).

### Hook self-skips in the main worktree

In a freshly cloned repository, `prev_HEAD = 0000…` fires too — but the new clone IS the main worktree, so source == dest. The hook detects this via `git rev-parse --git-dir == --git-common-dir` (equal in main worktree, divergent in linked worktrees) and exits 0.

### Explicit file list, not a glob

```bash
FILES=(
  "backend/.env.local"
  "frontend/.env.local"
)
```

Two files known to be required. Glob `**/.env.local` is more future-proof but riskier (could pick up unexpected `node_modules/**/.env.local` or test fixtures). Add to the list when a new gitignored env file appears.

### Idempotent

If destination already has a file or symlink at the expected path, the hook skips it silently. Re-running on the same worktree is a no-op. No risk of clobbering manual edits.

### Failure mode: permissive

`set -u` only (no `-e`). A single `ln -s` failure (e.g. permission, broken intermediate dir) shouldn't fail the entire checkout. The hook prints what it linked and exits 0.

## File structure

**Created:**
- `.githooks/post-checkout` — the hook script (~50 lines including comments).

**Untouched:**
- `scripts/install-hooks.sh` — already iterates every file in `.githooks/`, so it picks up the new hook automatically with zero changes.
- All Taskfile entries.

**Modified:**
- `CLAUDE.md` Setup section comment — tweak `(pre-commit + pre-push gates)` to mention `post-checkout` so future readers see the full picture.
- `README.md` — same tweak.

## Implementation outline

### `.githooks/post-checkout` (full text)

```bash
#!/usr/bin/env bash
# Symlink .env.local files from the main worktree into a freshly created
# linked worktree. Triggers ONLY on the first checkout
# (prev_HEAD = 0000…). Covers `git worktree add` and `git clone`; the
# latter is self-skipped because the new clone IS the main worktree
# (source == dest).
set -u

PREV_HEAD=$1
NEW_HEAD=$2
BRANCH_FLAG=$3

# First-checkout signal. Skip ordinary `git checkout BRANCH` so the hook
# never auto-restores a file the user deliberately deleted.
[ "$PREV_HEAD" = "0000000000000000000000000000000000000000" ] || exit 0
[ "$BRANCH_FLAG" = "1" ] || exit 0

GIT_DIR="$(cd "$(git rev-parse --git-dir)" && pwd -P)"
COMMON_DIR="$(cd "$(git rev-parse --git-common-dir)" && pwd -P)"
NEW_WORKTREE="$(git rev-parse --show-toplevel)"

# Self-skip in main worktree (linked worktrees have GIT_DIR != COMMON_DIR).
[ "$GIT_DIR" != "$COMMON_DIR" ] || exit 0

MAIN_WORKTREE="$(cd "$COMMON_DIR/.." && pwd -P)"

FILES=(
  "backend/.env.local"
  "frontend/.env.local"
)

linked=()
for rel in "${FILES[@]}"; do
  src="$MAIN_WORKTREE/$rel"
  dst="$NEW_WORKTREE/$rel"
  [ -e "$src" ] || continue                  # main doesn't have it — nothing to share
  { [ -e "$dst" ] || [ -L "$dst" ]; } && continue   # don't clobber an existing file/link
  ln -s "$src" "$dst" 2>/dev/null && linked+=("$rel")
done

[ "${#linked[@]}" -gt 0 ] && echo "post-checkout: symlinked from main worktree: ${linked[*]}"
exit 0
```

### CLAUDE.md tweak

Setup section currently says:

```
# 1. Git hooks (pre-commit + pre-push gates). Installs delegate shims in
#    .git/hooks/ that forward to .githooks/.
```

Becomes:

```
# 1. Git hooks (pre-commit + pre-push + post-checkout). Installs delegate
#    shims in .git/hooks/ that forward to .githooks/.
```

README gets the same line change (it carries a parallel description per #189).

## Migration

Existing clones must re-run `scripts/install-hooks.sh` once to install the new `post-checkout` delegate shim. The install script is idempotent — re-running is safe and cheap. Documented in the PR body.

Fresh clones (and any worktree created from a clone that's already run the updated install script) are auto-covered.

## Testing

No unit tests. Manual smoke is the only meaningful verification:

1. `bash -n .githooks/post-checkout` — parse-check.
2. In main checkout: pull the merged branch + `scripts/install-hooks.sh` — confirm `Installed delegate hooks in .../hooks: post-checkout pre-commit pre-push`.
3. Create a fresh worktree via `EnterWorktree`. Verify:
   - `ls -la backend/.env.local frontend/.env.local` — both are symlinks pointing into the main worktree.
   - `task dev:up:backend 2>&1 | tail -3` — succeeds without `log.Fatal: no Clerk secret`.
   - `task e2e:up:backend && task test:e2e -- playwright/e2e/auth.spec.ts` — succeeds without `missing required env var: VITE_CLERK_PUBLISHABLE_KEY`.
4. From the new worktree, manually rerun the hook (`bash .githooks/post-checkout 0000000000000000000000000000000000000000 HEAD 1`) and confirm no clobber (it reports nothing new).

## Out of scope

- Backporting symlinks into pre-existing worktrees (run `scripts/install-hooks.sh` then re-create the worktree, or `ln -s` once manually).
- Detecting "main worktree moved" and re-linking. Broken symlinks fail loudly when accessed; that's acceptable.
- A general "copy any gitignored file" mechanism — explicit two-file list keeps the hook surprise-free.
- shellcheck in CI (separate decision; the hook is small enough to eyeball).

## PR scope

Single branch `feat/191-worktree-env-hook`, single PR. Diff is one new file (`.githooks/post-checkout`) + two doc-line tweaks (`CLAUDE.md`, `README.md`). Key Decisions section in the PR body reproduces: symlink-not-copy, absolute paths, prev_HEAD=0000 trigger, explicit file list, idempotent, permissive failure. No security-review trigger (no auth/middleware/handler/IAM/dep changes).
