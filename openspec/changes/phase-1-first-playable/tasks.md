# Phase 1: Implementation Tasks

## Milestones

```
Milestone A (Playable Grid)  → Playtest checkpoint
Milestone B (Backend + API)  → Full integration
Milestone C (Polish + PWA)   → Phase 1 complete
```

## Tasks

### Milestone A: Playable Grid (frontend only, hardcoded puzzle)

Goal: core game loop in the browser with a hardcoded 5x5 puzzle. Playtest before building the backend.

#### T-100: Doc Updates (theme rename)

- **Roadmap:** R-014, R-015
- **Agent:** workflow-orchestrator (direct — doc edits only)
- **Spec:** specs/theme.md (TH-08)
- **Work:**
  - BRAND_GUIDELINES.md: rename Minimalist → Tactile in section 8.2
  - GAME_DESIGN.md: rename "Default Theme: Minimalist" → "Default Theme: Tactile"
- **Acceptance:** TH-08 passes. No references to "Minimalist" as theme name in either file.
- **Commit after completion.**

#### T-101: Theme Architecture + Tactile Theme

- **Roadmap:** R-014, R-015
- **Agent:** frontend-dev
- **Spec:** specs/theme.md (TH-01 through TH-07)
- **Work:**
  - CSS custom properties in `src/index.css` per BRAND_GUIDELINES.md
  - Theme TypeScript interface in `src/theme/types.ts`
  - ThemeContext + ThemeProvider in `src/theme/ThemeContext.tsx`
  - Tactile theme object in `src/theme/tactile.ts`
  - Marker component (filled circle SVG) in `src/components/grid/Marker.tsx`
  - ExclusionMark component (cross SVG) in `src/components/grid/ExclusionMark.tsx`
  - Dark mode toggle (system preference + manual override)
  - TDD: all components and hooks tested
- **Acceptance:** All TH-01 through TH-07 pass.
- **Depends on:** T-100
- **Commit after completion.**

#### T-102: Constraint Engine + Validator

- **Roadmap:** R-017
- **Agent:** frontend-dev
- **Spec:** specs/grid.md (GR-05, GR-07)
- **Work:**
  - `src/engine/types.ts` — puzzle type definitions, CellState, Position, Conflict
  - `src/engine/constraints.ts` — four constraint check functions
  - `src/engine/validator.ts` — solution validation
  - TDD: table-driven tests for each constraint and validation
- **Acceptance:** GR-05 and GR-07 pass.
- **Depends on:** None (can run parallel with T-101)
- **Commit after completion.**

#### T-103: Interactive Grid Component

- **Roadmap:** R-016
- **Agent:** frontend-dev
- **Spec:** specs/grid.md (GR-01 through GR-04, GR-06)
- **Work:**
  - `src/components/grid/Grid.tsx` — CSS Grid layout, region boundaries
  - `src/components/grid/Cell.tsx` — tap handler, drag support, conflict display
  - Three-tap cycle: empty → excluded → marked → empty
  - Drag gesture: intent from starting cell state
  - Conflict highlighting: real-time, both markers pulse
  - Hardcoded 5x5 puzzle for development/testing
  - TDD: interaction, rendering, conflict highlighting tested
- **Acceptance:** GR-01 through GR-04, GR-06 pass.
- **Depends on:** T-101 (theme), T-102 (constraints)
- **Commit after completion.**

#### T-104: Milestone A Integration

- **Roadmap:** R-016
- **Agent:** frontend-dev
- **Spec:** Cross-spec integration
- **Work:**
  - Wire Grid into App.tsx with hardcoded puzzle (no routing yet)
  - Verify full game loop: place markers, see conflicts, solve puzzle
  - Completion detection triggers (no overlay yet — console log or alert is fine)
  - Visual QA against BRAND_GUIDELINES.md
  - Manual test on mobile viewport (Chrome DevTools)
- **Acceptance:** Playable 5x5 puzzle in the browser. Three-tap and drag work. Conflicts highlight. Completion detected.
- **Depends on:** T-103
- **Commit after completion.**

**→ PLAYTEST CHECKPOINT: Review Milestone A before proceeding.**

### Milestone B: Backend + API Integration

Goal: puzzles generated server-side, frontend fetches from API.

#### T-105: Puzzle Data Model + Region Validation

- **Roadmap:** R-010
- **Agent:** backend-dev
- **Spec:** specs/puzzle-engine.md (PE-01, PE-02)
- **Work:**
  - `internal/model/puzzle.go` — Puzzle struct
  - `internal/generator/region.go` — region validation + region map generation
  - TDD: table-driven tests for region validation
- **Acceptance:** PE-01 and PE-02 pass.
- **Commit after completion.**

#### T-106: Constraint Solver

- **Roadmap:** R-011
- **Agent:** backend-dev
- **Spec:** specs/puzzle-engine.md (PE-03)
- **Work:**
  - `internal/generator/solver.go` — constraint-based solver
  - Returns solution count (stops at 2)
  - TDD: puzzles with 0, 1, and multiple solutions
- **Acceptance:** PE-03 passes.
- **Depends on:** T-105
- **Commit after completion.**

#### T-107: Puzzle Generator + Benchmarks

- **Roadmap:** R-012
- **Agent:** backend-dev
- **Spec:** specs/puzzle-engine.md (PE-04, PE-05)
- **Work:**
  - `internal/generator/generator.go` — generation loop
  - Region shape generation around a given solution
  - Uniqueness verification via solver
  - Configurable timeout
  - TDD: generated puzzles pass all constraints
  - `generator_bench_test.go` — benchmark suite
- **Acceptance:** PE-04 and PE-05 pass. Benchmark suite runs.
- **Depends on:** T-106
- **Commit after completion.**

#### T-108: Generate Endpoint

- **Roadmap:** R-013
- **Agent:** backend-dev
- **Spec:** specs/puzzle-engine.md (PE-06, PE-07)
- **Work:**
  - `internal/handler/generate.go` — HTTP handler
  - Register on chi router in `cmd/api/main.go`
  - Validate params (only size=5, mode=standard in Phase 1)
  - UUID generation for puzzleId
  - TDD: success, invalid params, generation failure
- **Acceptance:** PE-06 and PE-07 pass.
- **Depends on:** T-107
- **Commit after completion.**

#### T-109: Frontend API Integration

- **Roadmap:** R-013, R-016
- **Agent:** frontend-dev
- **Spec:** specs/game-state.md (GS-10)
- **Work:**
  - `src/services/api.ts` — base API client
  - `src/services/puzzleService.ts` — generatePuzzle function
  - Wire into Grid: replace hardcoded puzzle with API fetch
  - Loading state while puzzle generates
  - Error handling for API failures
  - TDD: mock fetch, verify URL construction and error handling
- **Acceptance:** GS-10 passes. Grid loads puzzles from API.
- **Depends on:** T-108 (backend), T-104 (frontend)
- **Commit after completion.**

### Milestone C: Polish + Persistence + PWA

Goal: full game flow with state persistence and PWA installability.

#### T-110: IndexedDB Game State

- **Roadmap:** R-018
- **Agent:** frontend-dev
- **Spec:** specs/game-state.md (GS-01 through GS-04)
- **Work:**
  - IndexedDB setup (database, object stores, versioning)
  - `useGameStorage` hook
  - Persistence triggers (cell change debounced, visibility, beforeunload, completion)
  - TDD: save/load/clear/addCompletion
- **Acceptance:** GS-01 through GS-04 pass.
- **Commit after completion.**

#### T-111: Timer

- **Roadmap:** R-018, R-019
- **Agent:** frontend-dev
- **Spec:** specs/game-state.md (GS-05)
- **Work:**
  - `useTimer` hook
  - Elapsed + resumed timestamp model
  - Pause on blur, resume on focus
  - Start on first cell interaction
  - Display: MM:SS, Space Mono, tabular-nums
  - TDD: accumulation across pause/resume
- **Acceptance:** GS-05 passes.
- **Depends on:** T-110
- **Commit after completion.**

#### T-112: Game Flow (Routing + Pages)

- **Roadmap:** R-019
- **Agent:** frontend-dev
- **Spec:** specs/game-state.md (GS-06 through GS-09)
- **Work:**
  - Add react-router-dom
  - LandingPage: check IndexedDB, show Play/Resume/New Puzzle
  - GamePage: load state, render grid + timer, completion overlay
  - Completion overlay: solve time, celebration animation, Play Again
  - Route setup in App.tsx
  - TDD: landing page states, game page redirect, completion overlay
- **Acceptance:** GS-06 through GS-09 pass.
- **Depends on:** T-110 (storage), T-111 (timer), T-109 (API)
- **Commit after completion.**

#### T-113: PWA Setup

- **Roadmap:** R-01A
- **Agent:** frontend-dev
- **Spec:** specs/pwa.md (PW-01 through PW-05)
- **Work:**
  - Update manifest.json with final values
  - Workbox via vite-plugin-pwa: precache build output
  - Offline connectivity handling for Play/New Puzzle buttons
  - Placeholder icons (192x192, 512x512)
  - TDD: offline state messaging
- **Acceptance:** PW-01 through PW-05 pass.
- **Depends on:** T-112
- **Commit after completion.**

#### T-114: Taskfile Updates

- **Roadmap:** R-007
- **Agent:** devops-engineer
- **Spec:** N/A (extends Phase 0 Taskfile)
- **Work:**
  - Add `task bench:backend` target for generator benchmarks
  - Verify all existing targets still work with new code
- **Acceptance:** `task bench:backend` runs generator benchmarks. All other targets pass.
- **Depends on:** T-107
- **Commit after completion.**

## Execution Summary

| Milestone | Tasks | Agent(s) | Notes |
|-----------|-------|----------|-------|
| A | T-100, T-101, T-102, T-103, T-104 | orchestrator, frontend-dev | T-101 and T-102 can run in parallel |
| B | T-105, T-106, T-107, T-108, T-109 | backend-dev, frontend-dev | T-105–T-108 are sequential (backend). T-109 bridges backend and frontend |
| C | T-110, T-111, T-112, T-113, T-114 | frontend-dev, devops-engineer | T-114 can run parallel with T-110–T-113 |

## Dependency Graph

```
T-100 (docs)
  └→ T-101 (theme) ──┐
                      ├→ T-103 (grid) → T-104 (milestone A integration) ──→ T-109 (API integration) → T-112 (game flow)
T-102 (constraints) ──┘                                                      ↑                          ↑
                                                                             │                     T-110 (IndexedDB) → T-111 (timer)
T-105 (model) → T-106 (solver) → T-107 (generator) → T-108 (endpoint) ──────┘                          │
                                          │                                                         T-113 (PWA)
                                          └→ T-114 (taskfile)
```

## Notes

- Milestone A and B backend work (T-105–T-108) can overlap if different agents work in parallel.
- Playtest checkpoint after Milestone A is mandatory — interaction and visual issues must be caught before backend work.
- All tasks follow TDD: failing test first, then implementation, then refactor.
