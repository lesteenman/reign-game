# `src/pages/`

Route-level components. Three production files and three unit tests.

## Responsibility

Top-of-tree React components that the router mounts under each URL path. Each owns its own state machine (load / fetch / error), composes UI from `components/` and `shared/`, and consumes hooks plus services.

## Data flow

- **In:** Mounted by `App.tsx`'s `<Routes>`. Receive route params via `useSearchParams()` and routing primitives via `useNavigate()`.
- **Out:** Render hierarchy: `PageShell` (chrome) → page content.

## Files

- **`LandingPage.tsx`** — Public landing. Three tiles (Daily / Packs / Curation). Curation gated on `getClerkUserRole(user.publicMetadata) === 'admin'`. Daily is live; Packs is a disabled placeholder.
- **`GamePage.tsx`** — Curation/practice play route (and daily-flow delegator). Owns the `LoadState` machine (loading / ready / no-state / no-puzzles / error). When `?flow=daily` is in the URL, renders `<DailyFlow />` from `features/daily/` and skips its own fetcher. Composes `<GameBoard>` from `shared/game/components/`.
- **`CurationPage.tsx`** — Admin-gated puzzle selector. Renders `<PuzzleSelector>` with the enabled-modes list and navigates to `/play?flow=curation&size=N&mode=M` on select.

Note: `AdminPage`, `AdminLandingPage` moved to `features/admin/pages/`. `DailyFlow`, `DailyGameBoard`, `PostCompletionScreen` moved to `features/daily/screens/`.

## Rules specific to this directory

- **One route, one `PageShell`.** Pages wrap their content in `<PageShell>`. The single exception is `GameBoard` running in delegated mode (the daily flow's `DailyFlow` mounts its own `PageShell` with the daily subtitle, so `GameBoard` returns a fragment instead of double-wrapping).
- **URL params are validated.** `parseFlowType` rejects unknown flow values (lesson 3 — validate URL params before type assertion).
