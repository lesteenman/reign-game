# Phase 2: All Grid Sizes + Double Queens

## What

Reign supports 5x5, 7x7, and 9x9 grids in Standard mode, plus 9x9 Double Queens (2 markers per row/column/region). The backend ships a pluggable generator architecture with multiple solver and region generation strategies, selectable via API parameters. An advanced mode on the landing page exposes all generator knobs for playtesting.

## Why

Phase 1 proved the core game loop at 5x5. Phase 2 proves the engine scales and that the puzzle variety is interesting enough to sustain engagement. The pluggable generator architecture lets us rapidly experiment with different strategies and tune puzzle personality without code changes. Double Queens adds a second mode that reuses the same infrastructure.

## Scope

- **R-020** — Extend generator + solver for 7x7 and 9x9 grids
- **R-022** — UI: grid size selector
- **R-030** — Extend solver for Double Queens constraints (2 per row/column/region)
- **R-031** — Extend generator for Double Queens puzzles
- **R-033** — UI: mode toggle (Standard / Double Queens)

Additional scope beyond original roadmap items:

- Pluggable generator architecture (solver + region generator behind interfaces)
- Constraint propagation solver strategy
- Wave Function Collapse region generation strategy
- Variable region sizes with configurable variance
- Open API accepting arbitrary grid sizes 3-15
- Advanced mode UI for playtesting with all parameters
- Back navigation from game page to landing page
- Dark mode toggle in game page header

## Not in Scope

- Double Queens at sizes other than 9x9 (validate after playtesting)
- Difficulty rating algorithm (Phase 3)
- DynamoDB / puzzle database (Phase 4)
- Daily puzzles, accounts, premium features (Phase 5+)
- Multi-game persistence (save multiple in-progress puzzles)
- Additional generator strategies beyond the initial four combos

## Implementation Milestones

- **A: Larger grids** — Extend current generator/solver for 7x7 and 9x9 Standard. Add benchmarks.
- **B: Generator architecture + Double Queens** — Pluggable interface, constraint propagation solver, WFC region generator, variable region sizes, Double Queens mode. Benchmark all strategy combos.
- **C: Frontend** — Size/mode selectors, advanced mode, adaptive grid, parameterized constraints/validator, back navigation, dark mode in game header.

## References

- ROADMAP.md: R-020, R-022, R-030, R-031, R-033
- design-grill-summary.md (this directory)
- GAME_DESIGN.md: Game Modes, Puzzle Design sections
- GLOSSARY.md: marker, region, grid, Standard Mode, Double Queens Mode
