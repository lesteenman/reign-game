# Spec: Game State + Persistence

Covers R-018 (IndexedDB game state) and R-019 (game flow UI).

## Requirements

### GS-01: IndexedDB Schema

- Database name: `reign-game`
- Object store `gameState`: stores the current active puzzle state
  - Key: fixed string `'current'`
  - Value: `{ puzzle, cells[][], timer, status, startedAt }`
- Object store `completions`: stores completion records
  - Key: auto-increment
  - Value: `{ puzzleId, time, completedAt }`
- Schema versioned (version 1) for future migrations

### GS-02: Game State Interface

```typescript
interface GameState {
  id: 'current'
  puzzle: { puzzleId: string, gridSize: number, mode: string, regionMap: number[][] }
  cells: CellState[][]   // 'empty' | 'excluded' | 'marked'
  timer: { elapsedAtLastPause: number, lastResumedAt: number | null }
  status: 'in-progress' | 'solved'
  startedAt: number
}
```

### GS-03: Storage Hook

- `src/hooks/useGameStorage.ts` provides `useGameStorage()` hook
- Methods: `saveState(state)`, `loadState(): GameState | null`, `clearState()`, `addCompletion(record)`
- Wraps IndexedDB async operations behind a clean interface
- Tests: save then load returns same state, clearState removes data, addCompletion appends record

### GS-04: Persistence Triggers

- Save on every cell state change (debounced 200ms to handle drag gestures)
- Save on `visibilitychange` event (page blur/focus — updates timer)
- Save on `beforeunload` event (final timer sync)
- Save on puzzle completion (final state + completion record)
- Tests: mock IndexedDB, verify save called on state change

### GS-05: Timer Logic

- `src/hooks/useTimer.ts` provides `useTimer()` hook
- Timer state: `elapsedAtLastPause` (accumulated seconds) + `lastResumedAt` (timestamp or null)
- Current elapsed = `elapsedAtLastPause + (now - lastResumedAt)` when running
- Timer starts on first cell interaction (not on puzzle load)
- Timer pauses on page blur (`visibilitychange`), resumes on focus
- Timer stops on puzzle completion
- Display: formatted as `MM:SS` using Space Mono font with `font-variant-numeric: tabular-nums`
- Tests: timer accumulates correctly across pause/resume cycles

### GS-06: Landing Page

- `src/pages/LandingPage.tsx` at route `/`
- Checks IndexedDB for active puzzle on mount
- **No active puzzle state:** Shows "Play" button. On click: fetches puzzle from API, saves to IndexedDB, navigates to `/play`.
- **Active puzzle in progress:** Shows "Resume" button (navigates to `/play`) + "New Puzzle" button (fetches fresh puzzle, discards old, navigates to `/play`).
- Loading state while checking IndexedDB / fetching from API
- Error state if API is unreachable (clean "You're offline" or "Could not load puzzle" message)
- Styled per BRAND_GUIDELINES.md (Tactile buttons, Warm Ink palette)
- Tests: renders Play when no state, renders Resume + New when state exists

### GS-07: Game Page

- `src/pages/GamePage.tsx` at route `/play`
- Loads game state from IndexedDB on mount
- If no game state: redirects to `/`
- Renders: Grid component + Timer display
- Timer starts on first cell interaction
- On completion: shows inline overlay with solve time + "Play Again" button
- "Play Again" fetches new puzzle, saves, resets game state
- Tests: redirects when no state, renders grid when state exists

### GS-08: Completion Overlay

- Inline overlay on GamePage (not a separate route)
- Shows: solve time (formatted), brief celebration animation (theme-defined)
- "Play Again" button: fetches new puzzle from API, resets state
- Celebration animation is subtle and non-blocking (per GAME_DESIGN.md UX principles)
- Tests: overlay appears on completion, Play Again resets state

### GS-09: React Router Setup

- `react-router-dom` added as dependency
- Routes defined in `App.tsx`: `/` → LandingPage, `/play` → GamePage
- 404 fallback redirects to `/`

### GS-10: API Client

- `src/services/api.ts` — base API client with configurable base URL
- `src/services/puzzleService.ts` — `generatePuzzle(size, mode): Promise<PuzzleData>`
- Calls `GET /puzzles/generate?size={size}&mode={mode}`
- Handles error responses (400, 500) and network failures
- Base URL configurable via environment variable (Vite `import.meta.env.VITE_API_URL`)
- Tests: mock fetch, verify correct URL construction and error handling

## Acceptance Criteria

All GS-01 through GS-10 requirements pass. Game state persists across page reloads. Timer tracks accurately across pause/resume. Landing page shows correct state. Game page renders the grid and detects completion. API client fetches puzzles.
