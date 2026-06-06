# Setup

Run these once after cloning. They wire up the git hooks, install dependencies, and make the
agent workflow skills available.

```bash
# 1. Git hooks (pre-commit + pre-push + post-checkout). Installs delegate shims in
#    .git/hooks/ that forward to .githooks/. .git/hooks/ is per-clone and shared
#    across worktrees, so this runs ONCE per fresh clone — every worktree picks up
#    the hooks automatically.
scripts/install-hooks.sh

# 2. Frontend dependencies (React, Vite, Tamagui, TanStack, Clerk, etc.)
#    Re-run inside any new `git worktree add` / `EnterWorktree` worktree —
#    node_modules is gitignored and per-worktree. The pre-push hook's `tsc` step
#    fails with "command not found" otherwise.
cd frontend && npm ci && cd ..

# 3. Playwright CLI for agent-driven browser testing
npm install -g @playwright/cli@latest
playwright-cli install --skills   # writes/updates .claude/skills/playwright-cli/

# 4. Superpowers plugin (per-machine; the repo only commits the `enabledPlugins`
#    flag in .claude/settings.json — the skill files live in ~/.claude/plugins/,
#    a per-machine cache). Run from inside a Claude Code session:
#        /plugin install superpowers@claude-plugins-official
```

## Why these matter

**Step 1 — hooks.** Without it the pre-commit and pre-push gates silently don't run, and CI catches
what your local shell should have. The install script replaces the old `git config core.hooksPath`
approach because `core.hooksPath` didn't propagate across machines and worktrees, while delegate
shims in `.git/hooks/` do. See https://www.gitworktree.org/guides/hooks.

**Step 4 — Superpowers.** Without it the skills (`brainstorming`, `writing-plans`,
`subagent-driven-development`, and the rest) are referenced by `.claude/settings.json` but the files
aren't on your machine, and the workflow falls back to ad-hoc behavior.

Dev server ports and the dev-stack tasks live in CLAUDE.md.
