# Frontend Index (Track 3 Phase 0)

Read-only inventory of the React 19 + Vite + TypeScript SPA at `frontend/`. Snapshot taken on 2026-05-15 against branch `fix/ci-plan-tf-vars`.

## What this is

A single-page Progressive Web App (PWA) that renders the Reign puzzle game. Three top-level surfaces today: a public landing page (Daily / Packs placeholder / admin-only Curation tile), a gameplay route at `/play` (curation and daily flows), and an admin route at `/admin` (pool config CRUD). Auth is Clerk (Google OAuth); persistent client state is IndexedDB; the API is reached at `/api/*` via a Vite proxy in dev and same-origin in prod.

## Current vs target architecture

| | Current (today) | Target (per `frontend/CLAUDE.md` + `architecture` skill) |
|---|---|---|
| Folder shape | Layered: `components/`, `pages/`, `hooks/`, `services/`, `engine/`, `storage/`, `theme/` | Feature-folder: `app/`, `engine/`, `features/{auth,game,daily,curation,admin,landing}/`, `shared/{components,hooks,lib,types}/`, `theme/`, `storage/` |
| UI primitives | Hand-rolled `<div>`, `<button>` with inline `style={}`; one residual `className=` (`Cell.tsx` animation hook) | Tamagui 2 RC primitives + theme tokens |
| Tailwind | Imported once in `index.css` (`@import "tailwindcss"`); no `className=*` consumers other than the animation hook | Retired (gone) |
| Server state | Hand-rolled `useState<LoadState>` / `useState<FlowState>` discriminated unions in `GamePage` + `DailyFlow`; bespoke `useEffect` fetch + cancel | TanStack `useQuery` / `useMutation` |
| `services/*` | Four service modules with three near-identical fetch helpers (`apiFetch` / `apiPost` / `apiPut`) plus a fourth fork (`dailyService.ts`) that bypasses `api.ts` for header injection | Hooks own the I/O; leaf components consume hooks |
| `engine/` | Pure TS (verified: no React, no `fetch`, no DOM) | Same — already conforming |
| `storage/` | Hand-rolled IndexedDB wrapper, single source of truth for persisted shapes | Same — already conforming |

The transition is the entire point of Track 3. `INDEX.md`, the per-directory READMEs, and `FINDINGS.md` capture the gap so the refactor work has a shared map.

## Top-level layout

```
frontend/
  src/                        application source (see per-directory READMEs)
  playwright/                 e2e + integration tests (see playwright/README.md)
  scripts/generate-icons.ts   PWA icon generator (build-time tool, not app code)
  public/                     static assets (icons/, manifest.json, icon.svg)
  dist/                       Vite build output (gitignored)
  test-results/               Playwright artifacts (gitignored)
  node_modules/               (gitignored)

  index.html                  Vite entry HTML
  package.json                deps + scripts (npm)
  tsconfig.json               root project references config
  tsconfig.app.json           app build (browser target)
  tsconfig.node.json          node-side config (vite.config, generate-icons)
  vite.config.ts              Vite + Vitest + Tailwind plugin + /api proxy
  playwright.config.ts        integration + e2e projects
  tamagui.config.ts           Track 2 placeholder; full migration is Track 3
  CLAUDE.md                   frontend conventions (additive to root CLAUDE.md)
```

## `src/` directory tree (with one-line file summaries)

### `src/` root

- `App.tsx` — Router + QueryClientProvider + ThemeProvider composition; exports `FALLBACK_PUZZLE` used by tests.
- `main.tsx` — React DOM bootstrap; mounts `<ClerkProvider>` only when `VITE_CLERK_PUBLISHABLE_KEY` is set (anonymous-safe degradation).
- `App.test.tsx` — smoke test that renders the app and resolves the initial puzzle fetch.
- `index.css` — CSS custom-property tokens (light + dark), animation keyframes, and a `@import "tailwindcss"` line.
- `test-setup.ts` — Vitest setup: jest-dom matchers + `matchMedia` mock for jsdom.
- `test-utils.tsx` — wraps RTL `render` / `renderHook` in `<StrictMode>` to catch impure updaters at unit-test time.
- `vite-env.d.ts` — Vite's ambient type reference.

### `src/components/`

Cross-feature React components. See `src/components/README.md` for the directory breakdown — each immediate subfolder (`auth/`, `common/`, `game/`, `grid/`, `landing/`) has its own README.

### `src/engine/`

Pure-TS puzzle domain. No React, no fetch, no DOM. See `src/engine/README.md`.

- `types.ts` — `CellState`, `Mode`, `MODES`, `isMode`, `PuzzleData`, `Conflict`, `PuzzleMetadata`.
- `constraints.ts` — row / column / region / adjacency conflict detectors + `getAllConflicts` deduplicator.
- `validator.ts` — solution validator.
- Tests: `constraints.test.ts`, `validator.test.ts`.

### `src/hooks/`

Reusable hooks. See `src/hooks/README.md`.

- `useGame.ts` — gameplay reducer (history stack, drag intent, conflicts, isSolved). Exports `cellKey` (consumed by `grid/Grid.tsx`).
- `useTimer.ts` — pause/resume timer with `restore()` for persistence and `stop()` for solved-state.
- `useGameStorage.ts` — IndexedDB CRUD wrapper (saveState / loadState / clearState / addCompletion).
- Tests: `useGame.test.ts`, `useTimer.test.ts`, `useGameStorage.test.ts`.

### `src/shared/hooks/` *(2026-05-18: PWA-related additions from #116)*

Cross-feature reusable hooks.

- `useOnlineStatus.ts` — `useSyncExternalStore` against `window.online`/`window.offline` events; returns `navigator.onLine` snapshot.
- `useInstallPrompt.ts` — captures `beforeinstallprompt`; exposes `{ canInstall, isStandalone, promptInstall }`.
- Tests: `useOnlineStatus.test.ts`, `useInstallPrompt.test.ts`.

### `src/pages/`

Route-level components. See `src/pages/README.md`.

- `LandingPage.tsx` — public landing with Daily / Packs / Curation tiles (Curation gated on admin role).
- `GamePage.tsx` — gameplay host (786 LOC). Manages `LoadState` machine + the inner `GameBoard` (also exported here).
- `DailyFlow.tsx` — daily-puzzle state machine (loading → playing → submitting → solved). Internally uses `DailyGameBoard`.
- `DailyGameBoard.tsx` — daily flow's grid host; adapts the daily payload to the `GameBoard` contract.
- `PostCompletionScreen.tsx` — terminal "Done for today" screen with countdown.
- `CurationPage.tsx` — admin-gated puzzle-selector for curation play.
- `AdminPage.tsx` — admin pool-management UI (649 LOC).
- `AdminLandingPage.tsx` — unauthenticated / forbidden landing for `/admin`.
- Tests: one `.test.tsx` per page plus `GameBoard.test.tsx` and `GameBoardWallClock.test.tsx`, both targeting the `GameBoard` function exported from `GamePage.tsx`.

### `src/services/`

Backend client modules. See `src/services/README.md`.

- `api.ts` — shared fetch base (`apiFetch` / `apiPut` / `apiPost`) + `ApiError`.
- `puzzleService.ts` — `fetchNextPuzzle`, `updatePuzzleStatus`, `NoPuzzlesAvailableError`.
- `verdictService.ts` — `submitVerdict` (admin verdict POST with `clientVersion`).
- `adminService.ts` — pool / config CRUD (`fetchPoolStatus`, `updateConfig`, `createConfig`, `triggerReplenish`) plus type re-exports.
- `landingService.ts` — `fetchEnabledModes` (public `/api/config/modes`).
- `dailyService.ts` — daily flow (`getDaily`, `submitDailyResult`); intentionally bypasses `api.ts` to inject `X-Device-Id`.

### `src/storage/`

IndexedDB wrapper + persisted shapes. See `src/storage/README.md`.

- `db.ts` — `openDB()`, `idFor(flowType, flowId)`, schema upgrades.
- `types.ts` — `GameState`, `FlowType`, `GameHistory`, `CompletionRecord`, `EMPTY_HISTORY`, `parseFlowType`, `buildCurationFlowId`.
- `utils.ts` — `createFreshGameState`.
- Tests: `db.test.ts`, `types.test.ts`, `utils.test.ts`.

### `src/theme/`

Theme abstraction + dark mode hook. See `src/theme/README.md`.

- `types.ts` — `Theme`, `MarkerProps`, `ExclusionMarkProps` contracts.
- `tactile.ts` — the default "Tactile" theme (the only theme implemented today).
- `ThemeContext.tsx` — provider + `useTheme()` hook.
- `useDarkMode.ts` — `prefers-color-scheme` initial + localStorage override; toggles `.dark` class on `<html>`.
- Tests: `tactile.test.ts`, `useDarkMode.test.ts`, `ThemeContext.test.tsx`.

### `src/components/auth/`

Auth-related components. See `src/components/auth/README.md`.

- `ClerkAvailability.tsx` — context flag indicating whether `<ClerkProvider>` was mounted (anonymous-safe degradation).
- `ProtectedAdminRoute.tsx` — wraps `/admin` and `/curation`; renders three states per AS-09. **Known violation:** imports `AdminPage` + `AdminLandingPage` from `pages/`.
- `SignInButton.tsx` — wraps Clerk's `<SignInButton mode="modal">` with branded styling.
- `UserMenu.tsx` — wraps Clerk's `<UserButton>`; adds an "Admin" menu item for admins.
- `role.ts` — `getClerkUserRole(publicMetadata)`.

### `src/components/common/`

Shared UI primitives. See `src/components/common/README.md`.

- `Button.tsx` — `PrimaryButton`, `SecondaryButton`, `GhostButton`.
- `PageShell.tsx` — top-of-page chrome (header, back button, subtitle, dark-mode toggle, auth slot).
- `buttonStyles.ts` — `compactSecondaryButtonStyle` (smaller variant for headers / cards).
- `press.ts` — `pressIn` / `pressOut` mouse-event handlers for the "tactile ink shadow" press.
- `OfflineBanner.tsx` — *(2026-05-18: #116)* global offline indicator slotted into `PageShell`; renders when `useOnlineStatus()` returns `false`.
- Tests: `OfflineBanner.test.tsx`.

### `src/components/game/`

Game-specific (non-grid) UI. See `src/components/game/README.md`.

- `VerdictSurface.tsx` — admin curation verdict surface (completion and skip variants). **Known violation:** imports `submitVerdict` + `updatePuzzleStatus` directly.

### `src/components/grid/`

Custom hand-built grid UI. See `src/components/grid/README.md`.

- `Grid.tsx` — measures the container, lays out cells, renders the region-border overlay. **Imports `cellKey` from `hooks/useGame`.**
- `Cell.tsx` — single cell; renders marker / exclusion mark; handles touch + mouse pointer-down with synthesized-mouse-event suppression.
- `Marker.tsx` — rounded-square marker SVG for the Tactile theme.
- `ExclusionMark.tsx` — small dot SVG for excluded cells.
- `RegionBorderOverlay.tsx` — SVG overlay drawing region boundary lines + corner junctions.

### `src/components/landing/`

Landing-page-specific UI. See `src/components/landing/README.md`.

- `PuzzleSelector.tsx` — size/mode preset selector + Play button. Used by `CurationPage`.
- `InstallAppTile.tsx` — *(2026-05-18: #116)* install CTA tile; only rendered when `beforeinstallprompt` has fired and the app is not already running in standalone mode.
- Tests: `InstallAppTile.test.tsx`.

## Playwright tests

Two Playwright projects: `integration` (mocked `/api/*`, runs against the dev server on `:5180`) and `e2e` (real backend + real LocalStack, fixture-driven, on `:5183`). See `playwright/README.md` (existing) and the newer `playwright/README.md` content extended for Track 3 details.

## Build commands (frontend-local)

```
npm run dev                  vite on :5180
npm run build                tsc -b && vite build
npm run preview              vite preview (build artifacts)
npm run test                 vitest run (unit)
npm run test:integration     playwright --project=integration
npm run test:e2e             playwright --project=e2e (assumes `task e2e:up`)
npm run test:playwright      both Playwright projects
```

For the standard dev workflow use `task dev:up:frontend` from the repo root (see root CLAUDE.md "Running the Dev Stack").
