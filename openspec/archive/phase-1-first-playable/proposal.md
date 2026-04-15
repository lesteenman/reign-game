# Phase 1: First Playable (5x5 Standard)

## What

A playable 5x5 Standard Mode puzzle game. Go backend generates puzzles on demand. React frontend renders an interactive grid with the Tactile theme, three-tap interaction, drag-to-exclude, real-time conflict highlighting, and client-side validation. Game state persists in IndexedDB. PWA basics for installability and offline resume.

## Why

Phase 0 proved the deploy pipeline. Phase 1 proves the game. The core interaction loop (place markers, see conflicts, solve the puzzle) must feel right before adding grid sizes, modes, or backend features.

## Scope

- **R-010** — Puzzle data model: grid, regions, solution representation (Go)
- **R-011** — Puzzle solver: constraint-based deduction, verify uniqueness (Go, 5x5 Standard)
- **R-012** — Puzzle generator: produce valid 5x5 Standard Mode puzzles (Go)
- **R-013** — Generate endpoint: stateless, returns a fresh puzzle on each call (no DB)
- **R-014** — Theme architecture: ThemeContext, theme data structure, component token consumption
- **R-015** — Tactile default theme: piece icons, color palette, grid styling, animations
- **R-016** — Interactive grid component: render regions, place/remove markers, exclusion marks, highlight conflicts (theme-aware)
- **R-017** — Solution validation in TypeScript (constraint check, no solver)
- **R-018** — Game state in IndexedDB: placements, exclusion marks, timer, completion status
- **R-019** — Game flow UI: puzzle loading, timer, solve flow, completion screen
- **R-01A** — PWA basics: service worker (app shell caching), manifest, install prompt

## Not in Scope

- 7x7 and 9x9 grids (Phase 2)
- Double Queens mode (Phase 3)
- DynamoDB / puzzle database (Phase 4)
- Daily puzzles, scoring, leaderboards (Phase 5+)
- Auth, accounts, premium features (Phase 5+)
- Custom install prompt UI
- Offline puzzle caching (beyond current active puzzle)
- Background sync, push notifications

## Implementation Milestones

- **A: Playable grid** — Frontend only, hardcoded puzzle. Full interaction + visuals. Playtest checkpoint.
- **B: Backend + API** — Generator, solver, endpoint. Frontend wired to API.
- **C: Polish + persistence** — IndexedDB, timer, completion overlay, PWA shell.

## References

- ROADMAP.md: R-010 through R-01A
- design-grill-summary.md (this directory)
- GAME_DESIGN.md: Game Modes, Puzzle Design, Theme System sections
- BRAND_GUIDELINES.md: Tactile visual system
- GLOSSARY.md: marker, region, grid, cell, adjacency constraint
