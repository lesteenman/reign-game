# OpenSpec — Frozen Archive

This directory is a historical record of the OpenSpec-driven development phases.

**Status (as of Track 1, 2026-05-15): frozen — no new entries.**

Going forward, design discussions and acceptance criteria live in the comments of [GitHub Issues](https://github.com/lesteenman/reign-game/issues). The `openspec/changes/` directory was removed when `auto-replenish-puzzle-pool` (the last in-flight change) was moved here.

## What's preserved here

Each subdirectory holds the design artifacts of one shipped phase or slice:

- `phase-0-skeleton` through `phase-8-daily-puzzle` — phase-level changes that shipped in Phases 0–8
- `architecture-split`, `e2e-in-ci`, `e2e-coverage-and-clerk-injection` — cross-phase slices
- `auto-replenish-puzzle-pool` — the reactive puzzle-pool replenish (shipped in PR #104, 2026-05-08)

Each directory typically contains a `proposal.md`, `design.md`, and `tasks.md`. References from older PRs / commit messages / wiki pages to slice IDs (e.g. `R-08-01`, `R-067a`) resolve back to these files.

## Why frozen

OpenSpec served its purpose during phases 0–8 — it gave structured, file-based artifacts a phase could be designed against. Once the project moved to GitHub-native planning in Track 1, the friction of editing markdown files in PRs (for every plan change) outweighed the benefit. Issues + comments + the project board cover the same use cases with lower edit cost.

The Wiki page **[Roadmap History](https://github.com/lesteenman/reign-game/wiki/Roadmap-History)** has the high-level narrative of each phase; this directory has the durable per-phase technical artifacts.
