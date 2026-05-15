# `src/pages/`

Route-level components. Eight production files (plus inner `GameBoard` exported from `GamePage.tsx`) and eleven unit tests.

## Responsibility

Top-of-tree React components that the router mounts under each URL path. Each owns its own state machine (load / fetch / error), composes feature-level UI from `components/`, and consumes hooks (`useGame`, `useTimer`, `useGameStorage`) plus services (`puzzleService`, `dailyService`, `landingService`, `adminService`).

## Data flow

- **In:** Mounted by `App.tsx`'s `<Routes>`. Receive route params via `useSearchParams()` and routing primitives via `useNavigate()`.
- **Out:**
  - Render hierarchy: `PageShell` (chrome) → page content. Some daily/curation pages compose lower-level components from `pages/` itself (this is the known page-to-page violation).
  - Service calls: every page that fetches data does so via a `useEffect` + manual `setState<LoadState>` ladder. No TanStack `useQuery` is wired today.

## Files

- **`LandingPage.tsx`** — Public landing. Three tiles (Daily / Packs / Curation). Curation gated on `getClerkUserRole(user.publicMetadata) === 'admin'`. Daily is live; Packs is a disabled placeholder.
- **`GamePage.tsx`** — Curation/practice play route (and daily-flow delegator). Owns the `LoadState` machine (loading / ready / no-state / no-puzzles / error). When `?flow=daily` is in the URL, renders `<DailyFlow />` and skips its own fetcher. Exports `GameBoard` (the inner grid host), which is also reused by `DailyGameBoard`. 786 LOC — the largest production file. **Page-to-page import: `import { DailyFlow } from './DailyFlow'`.**
- **`DailyFlow.tsx`** — Daily-puzzle state machine: loading → playing → submitting → solved → submit-error. Owns the IndexedDB short-circuit (DP-27) that renders `PostCompletionScreen` straight from a persisted solved row without a network round-trip. **Page-to-page imports: `./DailyGameBoard`, `./PostCompletionScreen`.**
- **`DailyGameBoard.tsx`** — Daily-flow grid host. Adapts `DailyPuzzlePayload` to `PuzzleData`, restores in-progress state from IndexedDB (DP-32), passes `onSolveDetected` to delegate the solve event to `DailyFlow`. **Page-to-page import: `import { GameBoard } from './GamePage'`.**
- **`PostCompletionScreen.tsx`** — Terminal "Done for today" card for the daily flow. Pure presentational: solve time + optional leaderboard rank + countdown to next UTC midnight + submitted-at timestamp + back-to-home button.
- **`CurationPage.tsx`** — Admin-gated puzzle selector. Renders `<PuzzleSelector>` with the enabled-modes list and navigates to `/play?flow=curation&size=N&mode=M` on select.
- **`AdminPage.tsx`** — Admin pool-management UI. Lists every `(size, mode)` combo with its config and ready-count; supports Edit, Replenish (per-combo), Replenish All, and Add Combo. 649 LOC with four inline sub-components (`ConfigFields`, `FormShell`, `EditConfigForm`, `CreateConfigForm`).
- **`AdminLandingPage.tsx`** — Two-state landing card for `/admin` when the visitor is not authorized: `anonymous` (sign-in CTA) or `forbidden` (back-to-home link).

## State management

Per page (see `frontend/FINDINGS.md` for the full inventory):

| Page | State |
|---|---|
| `LandingPage` | None (reads `useUser()` from Clerk). |
| `GamePage` | `useState<LoadState>`, `useState(fetchKey)` for retry. Inner `GameBoard` adds timer state, completion overlay state, skip modal state, refs for debounced saves. |
| `DailyFlow` | `useState<FlowState>`, `useState(fetchKey)`, `useRef<FlowState>` for callback stability. |
| `DailyGameBoard` | `useState<RestoredState | null>`, `useState(restoreReady)`. |
| `PostCompletionScreen` | `useState<Date>` for countdown ticking. |
| `CurationPage` | `useState<ModeEntry[] | null>`. |
| `AdminPage` | 8 `useState` calls (combos, loading, error, statusMessage, editingCombo, editConfig, showCreate, newComboConfig, createSize, createMode, saving). |
| `AdminLandingPage` | None. |

## Rules specific to this directory

- **One route, one `PageShell`.** Pages wrap their content in `<PageShell>`. The single exception is `GameBoard` running in delegated mode (the daily flow's `DailyFlow` mounts its own `PageShell` with the daily subtitle, so `GameBoard` returns a fragment instead of double-wrapping).
- **URL params are validated.** `parseFlowType` rejects unknown flow values (lesson 3 — validate URL params before type assertion).
- **`startedAt` is anchored.** `GameBoard` preserves the original `startedAt` across re-renders via `startedAtRef` so a mid-game render can't reset the elapsed-time anchor (lesson 5 / KI-025).
- **`onSolveDetected` is the delegate hook.** When a parent (currently only `DailyFlow`) wants to override the post-solve UX, it passes `onSolveDetected`; `GameBoard` then skips its built-in `addCompletion` / `clearState` / `updatePuzzleStatus` / completion overlay and fires the delegate instead.

## Known architecture violations

- `DailyGameBoard.tsx:3` imports `GameBoard` from `./GamePage` (page-to-page).
- `DailyFlow.tsx:12-13` imports `DailyGameBoard` and `PostCompletionScreen` from `./` (page-to-page).
- `GamePage.tsx:18` imports `DailyFlow` from `./` (page-to-page).

Track 3 fix: extract `GameBoard` to `features/game/components/GameBoard.tsx`; move daily-flow components into `features/daily/`; route `/play?flow=daily` to a dedicated `DailyPage` at the router level instead of branching inside `GamePage`.
