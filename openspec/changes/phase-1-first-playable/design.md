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

- **Marker:** Filled circle SVG, `--color-ink` fill
- **Exclusion mark:** Cross SVG, `--color-muted` stroke
- **Region colors:** 9 bold-saturation colorblind-safe fills from `--region-N-fill` tokens
- **Region boundaries:** 2.5px `--color-ink` borders between cells of different regions
- **Grid:** No borders between cells in the same region
- **Depth:** Tactile offset shadows (`0 3px 0 var(--color-ink)`) on interactive elements
- **Border radius:** 10px on cards and buttons, not on cells (grid stays sharp)

Components never hardcode visual values. They read CSS custom properties or theme context.

## Interactive Grid (R-016)

### Component Structure

```
Grid.tsx         — CSS Grid layout, region boundary logic
├── Cell.tsx     — Single cell, tap/drag handler, background color
│   ├── Marker.tsx         — Placed piece (from theme)
│   └── ExclusionMark.tsx  — "Not here" mark (from theme)
```

### Interaction Model

**Three-tap cycle** on a single cell:
1. Empty → Exclusion mark (cross)
2. Exclusion mark → Marker (circle)
3. Marker → Empty

**Drag gesture:**
- Start on empty cell → drag excludes all empty cells in path
- Start on excluded cell → drag clears exclusions in path
- Start on marker → no drag behavior

Drag works on both touch (touchmove) and mouse (mousemove with button held).

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

Two states:
- **No active puzzle:** "Play" button. Fetches from API on click, navigates to `/play`.
- **Active puzzle in progress:** "Resume" button (navigates to `/play` with existing state) + "New Puzzle" button (fetches fresh, discards old).

### Game Page

- Grid component (full width on mobile, constrained on desktop)
- Timer display (Space Mono, tabular-nums)
- On completion: inline overlay with solve time and "Play Again" button

### Completion

1. Timer stops (write final elapsed time to IndexedDB)
2. Brief celebration animation (theme-defined, subtle, non-blocking)
3. Overlay: solve time + "Play Again" button
4. "Play Again" fetches a new puzzle from the API

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

## Doc Updates Required

- **BRAND_GUIDELINES.md** section 8.2: rename "Minimalist" → "Tactile" (id, name)
- **GAME_DESIGN.md** "Default Theme: Minimalist" section: rename to "Default Theme: Tactile"
