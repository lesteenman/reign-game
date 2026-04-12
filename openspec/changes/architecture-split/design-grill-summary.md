# Architecture Split: Design Decisions

The puzzle engine (generator, solver, difficulty rating) lives in Go. The frontend validates solutions locally, renders solver visualizations from API-provided steps, and stores all player state in IndexedDB. Curation is a route in the main frontend app. Free players are anonymous. Premium is a one-time purchase with authenticated identity.

## Decisions

### Engine and Validation

**Generator and solver: Go backend only.** Puzzle generation is CPU-bound. The visual solver sends computed steps to the frontend for rendering. No TypeScript solver implementation.

**Solution validation: frontend.** Checking a filled grid against row/column/region/adjacency constraints is simple. The frontend does this locally for instant feedback and offline play. The backend re-validates when recording competitive completions.

### Data and Storage

**Puzzle delivery: API fetch, cache locally.** The frontend fetches all puzzles the player can access and stores them in IndexedDB. Free players get dozens, premium gets hundreds. Storage cost is negligible (under 1MB for a thousand puzzles).

**Player state: IndexedDB, every move.** Placements, pencil marks, timer, completion status. All persisted on each interaction so the player can resume at any time. Local-first; backend sync deferred until accounts exist.

### Curation

**Curation UI: route in the main frontend app.** Shares grid, theme, and interaction components with the game. The backend generates N candidates, ranks them by an interest heuristic, and returns all with solver steps. The curator plays each puzzle, watches the visual solver, and approves or rejects. Auth protection added later.

### Identity and Monetization

**Free players are fully anonymous.** No device ID, no server-side tracking. They fetch puzzles and submit daily completions to see their percentile, but don't appear on leaderboards.

**Premium is a one-time purchase.** Not a subscription. Unlocks full archive, leaderboard identity, detailed stats, premium themes, cross-device sync. Auth provider TBD (not Cognito).

**Admins are authenticated.** Access to curation UI and puzzle management.

### Infrastructure

**CI/CD from day one.** GitHub Actions builds and deploys on merge to main. The repo contains no AWS account, role, or domain specifics; all injected via GitHub configuration.

**No DynamoDB until Phase 4.** Phase 1-3 puzzles are generated on the fly by the backend. Persistent puzzle storage arrives with the curation tooling.

## Phases

| Phase | Name | Delivers |
|-------|------|----------|
| 0 | Skeleton + Deploy | Monorepo, health check Lambda, React scaffold, Terraform, GitHub Actions CI/CD. App live on AWS. |
| 1 | First Playable | 5x5 Standard Mode. Generator + solver (Go). Grid, markers, pencil marks, conflict highlighting, local validation. IndexedDB state. Theme architecture + minimalist theme. PWA basics. |
| 2 | All Grid Sizes | 7x7, 9x9. Difficulty rating. Difficulty selector in UI. |
| 3 | Double Queens | Generator/solver/UI extended for Double Queens. Mode toggle. |
| 4 | Curation + Puzzle DB | DynamoDB. Admin generation + curation endpoints. Curation route with visual solver. Practice mode serves curated puzzles. IndexedDB caching. Offline practice. |
| 5+ | TBD | Daily challenge, leaderboards, accounts, premium, themes, polish. Scoped later. |

## Deferred

- **Auth provider**: decided when accounts are built. Cognito vetoed.
- **Backend state sync**: after accounts exist.
- **Daily challenge and leaderboards**: Phase 5+.
- **Premium themes**: Phase 5+.
- **Queens Classic theme**: Phase 5+ (theme architecture proven by minimalist theme in Phase 1).

## Assumptions

- The backend is reachable for all play in Phases 1-3 (no offline play until curated puzzles are cached in Phase 4).
- The constraint-check validation in TypeScript and the solver in Go don't need to share code; they solve different problems (verify vs. deduce).
- Free player daily completion submissions are stateless; duplicate submissions are acceptable since free players don't appear on leaderboards.
