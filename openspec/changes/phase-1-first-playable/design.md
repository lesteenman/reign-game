# Phase 1: Design Document

Authoritative design reference for Phase 1 implementation. See design-grill-summary.md for the full decision log and rationale.

## Puzzle Engine (R-010, R-011, R-012)

### Data Model

A puzzle is defined by three values:

```go
type Puzzle struct {
    ID        string   `json:"puzzleId"`
    GridSize  int      `json:"gridSize"`
    Mode      string   `json:"mode"`
    RegionMap [][]int  `json:"regionMap"`
    Solution  [][]bool `json:"-"` // never serialized to client
}
```

`RegionMap[row][col]` is the region ID for that cell (0-indexed). For a 5x5 grid there are 5 regions (IDs 0–4), each containing exactly 5 cells. Regions are contiguous.

`Solution[row][col]` is true if a marker belongs at that cell. For Standard Mode, each row, column, and region has exactly one true cell.

### Solver

Constraint-based deduction solver. Checks all four constraints (row, column, region, adjacency) to eliminate candidates. Confirms the puzzle has exactly one valid solution. Used during generation to validate candidates.

The solver is not exposed via API in Phase 1. It runs only inside the generator pipeline.

### Generator

Produces valid 5x5 Standard Mode puzzles with unique solutions:

1. Generate a random valid solution (place 5 markers satisfying all constraints)
2. Generate a region map around the solution (each region contains exactly one solution marker)
3. Run the solver to confirm unique solvability
4. If not uniquely solvable, retry from step 1

The generator is built TDD. A separate benchmark suite measures generation time. Heuristics for "interesting" puzzles (region shape variety, deduction depth) are tunable and tested.

### Generate Endpoint (R-013)

```
GET /puzzles/generate?size=5&mode=standard
```

Response (200):
```json
{
  "puzzleId": "550e8400-e29b-41d4-a716-446655440000",
  "gridSize": 5,
  "mode": "standard",
  "regionMap": [[0,0,1,1,1],[0,0,1,2,2],[3,3,1,2,2],[3,4,4,4,2],[3,3,4,4,4]]
}
```

Error (500):
```json
{
  "error": "generation_failed",
  "message": "Could not generate a valid puzzle. Try again."
}
```

Stateless. No database. PuzzleId is a UUID v4 generated server-side. The solution is never included in the response.

## Theme Architecture (R-014, R-015)

### Hybrid CSS + React Context

Two layers:

**CSS custom properties** — All color, spacing, and shadow tokens. Defined in `:root` (light) and `.dark` (dark mode). Consumed by Tailwind classes and inline styles. Theme switching for colors is a CSS class swap — no React re-render.

**React Context** — `ThemeProvider` wraps the app. Provides non-CSS theme data: marker component, exclusion mark component, animation configuration. Components call `useTheme()` to read these.

```typescript
interface Theme {
  id: string
  name: string
  marker: React.ComponentType<MarkerProps>
  exclusionMark: React.ComponentType<ExclusionMarkProps>
  animations: {
    placement: string   // CSS class name
    conflict: string    // CSS class name
    completion: string  // CSS class name
  }
}
```

### Tactile Theme (default)

The single theme shipping in Phase 1. Visual identity from BRAND_GUIDELINES.md:

- **Marker:** Rounded square SVG (`rect` with rx=3), `--color-ink` fill, tight fit (padding 0.18)
- **Exclusion mark:** Small dot SVG (`circle` with r=0.08), `--color-ink` fill
- **Region colors:** 9 bold-saturation colorblind-safe fills from `--region-N-fill` tokens
- **Region boundaries:** 2px `--color-ink` lines drawn as SVG overlay on top of the grid (not CSS borders). Small filled squares at junction points for clean corners.
- **Cell borders:** 0.5px subtle internal borders between same-region cells only. Hidden on grid edges and under region boundaries.
- **Grid outer border:** 2px `--color-ink`, matching region border width
- **Depth:** Tactile offset shadows (`0 3px 0 var(--color-ink)`) on interactive elements
- **Border radius:** 10px on cards, buttons, and grid container
- **Dark mode:** Toggle button in page header, persists to localStorage, respects system preference

Components never hardcode visual values. They read CSS custom properties or theme context.

## Interactive Grid (R-016)

### Component Structure

```
Grid.tsx                — CSS Grid layout, responsive sizing, touch/mouse handlers
├── Cell.tsx            — Single cell, background color, border visibility
│   ├── Marker.tsx      — Placed piece (from theme, rounded square)
│   └── ExclusionMark.tsx — "Not here" mark (from theme, small dot)
├── RegionBorderOverlay.tsx — SVG overlay drawing region boundary lines
```

Shared layout components: `PageShell` (page wrapper with heading + dark mode toggle), `PrimaryButton`, `SecondaryButton`.

### Interaction Model (deferred-apply)

All state changes are deferred to pointer-up. Nothing happens on pointer-down.

**Single tap** (pointer down + up, no movement):
Three-tap cycle: Empty → Exclusion mark → Marker → Empty

**Drag** (pointer down + movement + up):
Intent determined by starting cell state:
- Start on empty cell → highlighted cells become excluded on release
- Start on excluded cell → highlighted cells become empty on release
- Start on marked cell → no drag effect (treated as tap on release)

During drag, all cells in the path are highlighted (15% darken) including marked cells. Exclusions/clears only applied on pointer-up. Touch double-fire prevented via `touchedRef` flag.

### Grid Sizing

Grid measures its container on mount and on (debounced) resize. Cells are clamped 44–72px. Grid defers rendering until measured (no flicker).

### Conflict Highlighting

Real-time. On every cell state change, check all constraints. If a marker conflicts with another marker (same row, column, region, or adjacent), both markers receive a conflict highlight: error color + subtle pulse animation. Conflicts clear when the violation is resolved.

## Client-Side Validation (R-017)

TypeScript constraint checker. No solver logic — just verifies whether the current board state satisfies all constraints.

Four checks:
1. **Row constraint:** Each row has exactly 1 marker (for a complete solution)
2. **Column constraint:** Each column has exactly 1 marker
3. **Region constraint:** Each region has exactly 1 marker
4. **Adjacency constraint:** No two markers are horizontally, vertically, or diagonally adjacent

A puzzle is solved when all four pass and the board has exactly N markers on an N×N grid. Validation runs on every cell state change to update conflict highlighting. Completion is detected when validation passes with N markers placed.

## Game State Persistence (R-018)

### IndexedDB Schema

Single object store: `gameState`

```typescript
interface GameState {
  id: 'current'                      // fixed key, one active puzzle
  puzzle: {
    puzzleId: string
    gridSize: number
    mode: string
    regionMap: number[][]
  }
  cells: CellState[][]               // 'empty' | 'excluded' | 'marked'
  timer: {
    elapsedAtLastPause: number        // accumulated seconds
    lastResumedAt: number | null      // timestamp, null when paused
  }
  status: 'in-progress' | 'solved'
  startedAt: number                   // timestamp
}
```

Completion records stored in a separate object store: `completions`

```typescript
interface CompletionRecord {
  puzzleId: string
  time: number          // seconds
  completedAt: number   // timestamp
}
```

### Persistence Triggers

- Cell state change (debounced 200ms for drag gestures)
- Page visibility change (blur/focus) — updates timer
- `beforeunload` event — final timer sync
- Puzzle completion

A thin wrapper (custom hook `useGameStorage`) hides IndexedDB async complexity from components.

## Game Flow (R-019)

### Routes

| Route | Component | Description |
|-------|-----------|-------------|
| `/` | LandingPage | Branding, play/resume CTA |
| `/play` | GamePage | Grid, timer, controls |

### Landing Page

Navigation only — does not fetch puzzles or construct game state.

Two states:
- **No active puzzle:** "Play" button → navigates to `/play?new=true`
- **Active puzzle in progress:** "Resume" button → `/play`, "New Puzzle" button → `/play?new=true`
- Offline detection: shows message when offline, Resume still works

### Game Page

Owns puzzle fetching and game state construction:
- `?new=true` param: fetches puzzle from API, creates fresh GameState, saves to IndexedDB
- No param: loads existing state from IndexedDB, redirects to `/` if none
- Grid component (full width on mobile, max 600px on desktop)
- Timer display (Space Mono, tabular-nums, right-aligned above grid)
- Timer starts on first cell interaction, pauses on page blur, resumes on focus
- `startedAt` captured once on game creation, never overwritten
- Reset clears the grid but keeps the timer running (not a way to get a lower time)

### Completion

1. Timer stops
2. Inline overlay: "Puzzle Complete!" + solve time + "Play Again" + "Home"
3. "Play Again" navigates to `/play?new=true` (fetches new puzzle)
4. Completion record saved to IndexedDB `completions` store

## PWA (R-01A)

### Manifest

`public/manifest.json` with app name, icons, `display: standalone`, theme color from BRAND_GUIDELINES.md.

### Service Worker

Workbox precache of build output (HTML, JS, CSS, fonts). App shell loads offline. No puzzle data caching.

### Offline Behavior

- App shell loads from cache
- Active puzzle resumes from IndexedDB (full gameplay works offline)
- "Play" / "New Puzzle" buttons show connectivity error when offline
- No custom install prompt — browser native is fine for Phase 1

## Implementation Notes (post-archive)

Changes from original design that emerged during implementation:

- **Interaction model**: Deferred-apply replaced immediate click+drag. All state changes on pointer-up. Prevents touch double-fire bug.
- **Region borders**: SVG overlay replaced CSS cell borders. Eliminates doubling, offset, and corner artifacts. 2px width (not 2.5px) for pixel-aligned rendering.
- **Markers**: Rounded square (`rect` rx=3) replaced filled circle. Exclusion dot replaced cross. Chosen through iterative visual comparison.
- **Puzzle fetch ownership**: GamePage owns fetching (via `?new=true` param), not LandingPage. Cleaner separation — landing page is navigation only.
- **Reset behavior**: Clears grid only, timer keeps running. It's a "I'm stuck" helper, not a score reset.
- **Dev ports**: Frontend 5180, backend 5181 (avoids conflicts with other local services).
- **Dark mode toggle**: Added to PageShell header, not in original spec. Uses existing useDarkMode hook.
- **Shared components**: PageShell, PrimaryButton, SecondaryButton extracted during review to eliminate duplication.
- **Test conventions**: Arrange-Act-Assert with explicit comments. Cell tests verify specific SVG elements (circle vs rect).
