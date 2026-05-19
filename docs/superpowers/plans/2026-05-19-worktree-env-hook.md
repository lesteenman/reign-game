# Worktree `post-checkout` Hook for `.env.local` Propagation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `.githooks/post-checkout` script that symlinks `.env.local` files from the main worktree into newly created linked worktrees, eliminating the manual `ln -s` step hit twice during PR #190 work.

**Architecture:** New shell script in `.githooks/` (auto-installed by the existing `scripts/install-hooks.sh` with zero changes). Fires only on first checkout (`prev_HEAD = 0000…`). Self-skips in the main worktree. Symlinks `backend/.env.local` and `frontend/.env.local` from main → new linked worktree, no-clobber.

**Tech Stack:** bash, git hooks.

**Spec:** `docs/superpowers/specs/2026-05-19-worktree-env-hook-design.md`
**Issue:** [#191](https://github.com/lesteenman/reign-game/issues/191)
**Branch:** `feat/191-worktree-env-hook` (worktree at `.claude/worktrees/feat+191-worktree-env-hook`).

---

## File Structure

**Created:**
- `.githooks/post-checkout` — the hook script (~40 lines including comments).

**Modified:**
- `CLAUDE.md` — one-line tweak to the Setup step 1 comment so it lists all three hooks now installed.
- `README.md` — same one-line tweak.

**Untouched (intentional):**
- `scripts/install-hooks.sh` — already iterates `.githooks/*`; picks up the new file automatically.

---

## Task 1 — Create `.githooks/post-checkout`

**Files:**
- Create: `.githooks/post-checkout`

- [ ] **Step 1.1: Create the script**

Create `.githooks/post-checkout` with EXACTLY this content:

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
  [ -e "$src" ] || continue                          # main doesn't have it — nothing to share
  { [ -e "$dst" ] || [ -L "$dst" ]; } && continue    # don't clobber an existing file/link
  ln -s "$src" "$dst" 2>/dev/null && linked+=("$rel")
done

[ "${#linked[@]}" -gt 0 ] && echo "post-checkout: symlinked from main worktree: ${linked[*]}"
exit 0
```

- [ ] **Step 1.2: Make executable**

```bash
chmod +x .githooks/post-checkout
```

- [ ] **Step 1.3: Parse-check**

```bash
bash -n .githooks/post-checkout
```

Expected: no output, exit 0.

- [ ] **Step 1.4: Smoke-test the no-op paths**

The hook should exit silently when its preconditions don't hold.

Run with a non-zero prev_HEAD (simulates ordinary `git checkout BRANCH`):

```bash
.githooks/post-checkout abc123 def456 1 2>&1; echo "exit=$?"
```

Expected: no output, `exit=0`.

Run with `prev_HEAD = 0000…` but `branch_flag = 0` (file checkout, not branch):

```bash
.githooks/post-checkout 0000000000000000000000000000000000000000 abc123 0 2>&1; echo "exit=$?"
```

Expected: no output, `exit=0`.

Run from inside the current worktree with full args (`prev_HEAD=0…`, `branch_flag=1`). This SHOULD attempt to link — but the destination files already exist (or we're not in a fresh worktree state). Expected: either silent exit 0 (files already present) or one line `post-checkout: symlinked from main worktree: …` if both destinations were missing. Either is correct.

```bash
.githooks/post-checkout 0000000000000000000000000000000000000000 HEAD 1 2>&1; echo "exit=$?"
```

- [ ] **Step 1.5: Install the new shim via the existing install script**

```bash
scripts/install-hooks.sh
```

Expected output ends with: `Installed delegate hooks in /Users/.../reign-game/.git/hooks: post-checkout pre-commit pre-push` (order may vary).

Verify the shim exists and is executable:

```bash
ls -la "$(cd "$(git rev-parse --git-common-dir)" && pwd -P)/hooks/post-checkout"
```

Expected: file present, mode `100755`.

- [ ] **Step 1.6: Commit**

```bash
git add .githooks/post-checkout
git commit -m "$(cat <<'EOF'
feat(infra): post-checkout hook symlinks .env.local into new worktrees (#191)

Fires only on the first checkout (prev_HEAD=0000…), covering
`git worktree add` and `git clone`. Self-skips in the main worktree.
Symlinks backend/.env.local and frontend/.env.local from the main
worktree into the new linked worktree, no-clobber. Idempotent.

The existing scripts/install-hooks.sh picks the new hook up
automatically — no changes to the install script. Existing clones
re-run scripts/install-hooks.sh once to install the new delegate
shim; fresh clones are auto-covered.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

Pre-commit hook fires on Go/TS only — this commit has neither, so it short-circuits.

---

## Task 2 — Update CLAUDE.md + README comment

**Files:**
- Modify: `CLAUDE.md` (the Setup section, around the `# 1. Git hooks (pre-commit + pre-push gates)` comment).
- Modify: `README.md` (the Getting Started section, parallel `(pre-commit + pre-push gates)` comment).

- [ ] **Step 2.1: Locate the CLAUDE.md line**

```bash
grep -n "pre-commit + pre-push" CLAUDE.md
```

Expected: one match (the inline comment above the `scripts/install-hooks.sh` line in the Setup section).

- [ ] **Step 2.2: Update CLAUDE.md**

Replace `(pre-commit + pre-push gates)` with `(pre-commit + pre-push + post-checkout)` in the Setup block. Use the Edit tool with the surrounding context to keep the edit unambiguous. Exact old/new strings:

OLD:
```
# 1. Git hooks (pre-commit + pre-push gates). Installs delegate shims in
```

NEW:
```
# 1. Git hooks (pre-commit + pre-push + post-checkout). Installs delegate shims in
```

- [ ] **Step 2.3: Locate the README.md line**

```bash
grep -n "pre-commit + pre-push" README.md
```

Expected: one match (parallel comment in the Getting Started section).

- [ ] **Step 2.4: Update README.md**

Same replacement.

OLD:
```
# Install the project's git hooks (required — pre-commit + pre-push gates).
```

NEW:
```
# Install the project's git hooks (required — pre-commit + pre-push + post-checkout).
```

- [ ] **Step 2.5: Verify both edits landed**

```bash
grep -n "post-checkout" CLAUDE.md README.md
```

Expected: one match in each file.

- [ ] **Step 2.6: Commit**

```bash
git add CLAUDE.md README.md
git commit -m "$(cat <<'EOF'
docs: mention post-checkout in setup-hook comments (#191)

CLAUDE.md and README.md setup blocks both listed only pre-commit and
pre-push. Add post-checkout to the parenthetical so future readers
see the full set of hooks installed by scripts/install-hooks.sh.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3 — End-to-end smoke test on a fresh worktree

**Files:** none (verification only).

This is the only meaningful behavioural verification — the hook has to fire from `git worktree add`, not just from manual invocation.

- [ ] **Step 3.1: Confirm install ran in the main checkout**

From this branch's worktree:

```bash
ls -la "$(cd "$(git rev-parse --git-common-dir)" && pwd -P)/hooks/" | grep post-checkout
```

Expected: shim is installed. If not, run `scripts/install-hooks.sh` first.

- [ ] **Step 3.2: Create a throwaway worktree to verify the hook fires**

```bash
git worktree add ../.test-191-smoke main 2>&1 | tail -10
```

Expected output ends with the hook's success line: `post-checkout: symlinked from main worktree: backend/.env.local frontend/.env.local` (or whichever of the two files exist in the main worktree).

If you only see the `Preparing worktree (checking out 'main')` line and no `post-checkout: symlinked…` line, the hook didn't run — investigate: was `scripts/install-hooks.sh` re-run after committing `.githooks/post-checkout`?

Note: the test worktree is created OUTSIDE `.claude/worktrees/` so the harness's tracking isn't involved.

- [ ] **Step 3.3: Inspect the symlinks**

```bash
ls -la /Users/eriksteenman/Projects/reign-game/.claude/worktrees/feat+191-worktree-env-hook/../.test-191-smoke/backend/.env.local \
       /Users/eriksteenman/Projects/reign-game/.claude/worktrees/feat+191-worktree-env-hook/../.test-191-smoke/frontend/.env.local 2>&1
```

Expected: both shown as `lrwxr-xr-x` symlinks. Each `->` target should be an absolute path into the MAIN worktree (`/Users/eriksteenman/Projects/reign-game/{backend,frontend}/.env.local`), not into another linked worktree.

- [ ] **Step 3.4: Test backend can start**

```bash
cd /Users/eriksteenman/Projects/reign-game/.claude/worktrees/feat+191-worktree-env-hook/../.test-191-smoke
task dev:down:backend 2>&1 | tail -3   # cleanup if anything from a previous run
task dev:up:backend 2>&1 | tail -5
```

Expected: ends with `backend ready on :5181 (PID …)` — no `log.Fatal` about missing Clerk secret.

Tear down:

```bash
task dev:down:backend 2>&1 | tail -3
cd /Users/eriksteenman/Projects/reign-game/.claude/worktrees/feat+191-worktree-env-hook
```

- [ ] **Step 3.5: Test Playwright global-setup loads env**

From the test worktree:

```bash
cd /Users/eriksteenman/Projects/reign-game/.claude/worktrees/feat+191-worktree-env-hook/../.test-191-smoke
cd frontend && npm ci 2>&1 | tail -3 && cd ..
task e2e:up:backend 2>&1 | tail -3
task e2e:up:generator 2>&1 | tail -3
task e2e:up:frontend 2>&1 | tail -3
task e2e:seed 2>&1 | tail -3
task test:e2e -- playwright/e2e/auth.spec.ts 2>&1 | tail -20
```

Expected: test:e2e passes (no `missing required env var: VITE_CLERK_PUBLISHABLE_KEY` error). Even just getting past `global-setup.ts:41` is the win — actual test pass/fail is secondary; the env-loading is what we're verifying.

Tear down:

```bash
task e2e:down:frontend && task e2e:down:generator && task e2e:down:backend
cd /Users/eriksteenman/Projects/reign-game/.claude/worktrees/feat+191-worktree-env-hook
```

- [ ] **Step 3.6: Idempotence — re-run the hook**

Inside the test worktree, manually re-trigger the hook:

```bash
cd /Users/eriksteenman/Projects/reign-game/.claude/worktrees/feat+191-worktree-env-hook/../.test-191-smoke
.githooks/post-checkout 0000000000000000000000000000000000000000 HEAD 1 2>&1
cd /Users/eriksteenman/Projects/reign-game/.claude/worktrees/feat+191-worktree-env-hook
```

Expected: no output (both destinations already exist; nothing to link).

- [ ] **Step 3.7: Cleanup**

```bash
git worktree remove /Users/eriksteenman/Projects/reign-game/.claude/worktrees/feat+191-worktree-env-hook/../.test-191-smoke 2>&1 | tail -3
git worktree prune
```

If `git worktree remove` complains about uncommitted changes (it shouldn't — the test worktree didn't touch tracked files), add `--force` to clean up. Verify removal:

```bash
git worktree list
```

Expected: only the main worktree and `feat+191-worktree-env-hook` listed.

- [ ] **Step 3.8: No commit needed**

Task 3 is verification only. The branch has 2 commits at this point (the hook + the doc tweaks).

---

## Task 4 — Push + PR

**Files:** none.

- [ ] **Step 4.1: Rename branch to canonical name**

The harness created the branch as `worktree-feat+191-worktree-env-hook`. Rename so the PR ref is cleaner:

```bash
git branch -m worktree-feat+191-worktree-env-hook feat/191-worktree-env-hook
git branch --show-current
```

Expected: `feat/191-worktree-env-hook`.

- [ ] **Step 4.2: Push**

```bash
git push -u origin feat/191-worktree-env-hook 2>&1 | tail -10
```

Expected: pre-push hook fires (gitleaks), reports no leaks, push completes.

- [ ] **Step 4.3: Open PR**

```bash
gh pr create --repo lesteenman/reign-game \
  --title "feat(infra): post-checkout hook symlinks .env.local into new worktrees (#191)" \
  --body "$(cat <<'EOF'
## Summary

Closes #191. New `.githooks/post-checkout` script symlinks `backend/.env.local` and `frontend/.env.local` from the main worktree into a freshly created linked worktree — no more manual `ln -s` after each `git worktree add` / `EnterWorktree`. The existing `scripts/install-hooks.sh` picks the new hook up automatically.

## Changes

- **`.githooks/post-checkout`** — new (~40 lines). Triggers only on first checkout (`prev_HEAD = 0000…`), self-skips in the main worktree, no-clobber on existing files, permissive on individual `ln -s` failures.
- **`CLAUDE.md` + `README.md`** — one-line tweak each to mention `post-checkout` in the parenthetical alongside `pre-commit + pre-push`.

## Key Decisions

- **Symlink, not copy.** Rotating a secret in main propagates instantly to every worktree. Discussed in the spec; user explicitly chose this over `cp -n`.
- **Absolute symlinks.** Robust to CWD inside the worktree. Breaks loudly if the main worktree moves — correct semantic.
- **First-checkout trigger only (`prev_HEAD = 0000…`).** Never auto-restores a file the user deliberately deleted on subsequent branch checkouts.
- **Self-skip in main worktree.** `prev_HEAD = 0000…` also fires for `git clone`, where the new clone IS the main; the hook detects this via `git-dir == git-common-dir` and exits.
- **Explicit two-file list.** No glob. Predictable; extend by adding to `FILES=` when a new gitignored env file shows up.
- **Permissive failure.** `set -u` only, no `-e`. A single failed symlink shouldn't fail the whole checkout.

## Migration

**Existing clones must re-run `scripts/install-hooks.sh` once** after pulling this branch's merge commit to install the new `post-checkout` delegate shim. The install script is idempotent and one command.

Fresh clones (and any worktree created from a clone that's already on this commit) are auto-covered.

## Spec / plan

- Spec: `docs/superpowers/specs/2026-05-19-worktree-env-hook-design.md`
- Plan: `docs/superpowers/plans/2026-05-19-worktree-env-hook.md`

## Test plan

- [x] `bash -n .githooks/post-checkout` parses clean.
- [x] No-op paths (non-zero `prev_HEAD`, `branch_flag = 0`) exit silently.
- [x] `scripts/install-hooks.sh` picks up the new hook (`Installed delegate hooks … post-checkout pre-commit pre-push`).
- [x] `git worktree add ../.test-191-smoke main` produces a worktree with the two expected symlinks; both `->` absolute paths into main.
- [x] `task dev:up:backend` works in the new worktree (no Clerk-missing `log.Fatal`).
- [x] `task test:e2e` works in the new worktree (no `VITE_CLERK_PUBLISHABLE_KEY` error in Playwright global-setup).
- [x] Idempotence — re-running the hook manually inside the same worktree produces no output (skip-existing).

## Out of scope

- Backporting symlinks into pre-existing worktrees (run `scripts/install-hooks.sh` after pulling main; then re-create the worktree, or `ln -s` once manually).
- A generic "copy gitignored files" mechanism. Explicit two-file list keeps surprises out.
- shellcheck in CI for hook scripts.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)" 2>&1 | tail -3
```

- [ ] **Step 4.4: Wait for CI**

```bash
gh pr checks <PR_NUMBER> --repo lesteenman/reign-game 2>&1 | tail -10
```

Expected: all checks pass. The diff is hook + docs — CI runs the full test suites against an unchanged code path; no new failure surface.

---

## Out-of-plan housekeeping (only if hit)

- If gitleaks finds a false positive in the hook script (e.g. the `0000…` SHA-shaped string triggers a secret detector — unlikely), update `.gitleaks.toml` or add a comment that breaks the pattern. Do NOT bypass with `--no-verify`.
- If the hook fails in Step 3.2 (no symlink output line), the most likely cause is that `scripts/install-hooks.sh` was last run before `.githooks/post-checkout` existed. Re-run it once and retry.
- If `git worktree remove` refuses cleanup in Step 3.7 because the test worktree has untracked files, add `--force` (it's a smoke-test worktree we created seconds ago).
