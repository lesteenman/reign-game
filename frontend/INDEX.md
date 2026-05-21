# Frontend Index (Track 3 Phase 0)

Read-only inventory of the React 19 + Vite + TypeScript SPA at `frontend/`. Snapshot taken on 2026-05-15 against branch `fix/ci-plan-tf-vars`.

## What this is

A single-page Progressive Web App (PWA) that renders the Reign puzzle game. Three top-level surfaces today: a public landing page (Daily / Packs placeholder / admin-only Curation tile), a gameplay route at `/play` (curation and daily flows), and an admin route at `/admin` (pool config CRUD). Auth is Clerk (Google OAuth); persistent client state is IndexedDB; the API is reached at `/api/*` via a Vite proxy in dev and same-origin in prod.

## Current vs target architecture

| | Current (today) | Target (per `frontend/CLAUDE.md` + `architecture` skill) |
|---|---|---|
| Folder shape | Mostly BR feature-folder. Remaining legacy: `services/` (four cross-feature service files). Target: pure feature-folder with no legacy layered dirs. | Feature-folder: `app/`, `engine/`, `features/{daily,curation,admin,landing}/`, `shared/{auth,components,game,hooks,types}/`, `theme/`, `storage/` (note: `auth` and `game` live under `shared/` because they are genuinely cross-feature, not single-feature concerns — see #196 + #204's BR-incorrect-framing analysis) |
| UI primitives | Tamagui 2 RC config + provider + Vite compiler plugin all wired (#176 kickoff). Token/theme system mirrors `index.css`'s CSS custom properties as literal hex for compiler extraction. **First Tamagui-migrated component: `Button.tsx` (#208).** Per-component migration is incremental — most chrome still uses inline `style={}` until each component lands its own Tamagui slice. One residual `className=` in `Cell.tsx` (keyframe-animation hook, not Tailwind). | Tamagui 2 RC primitives + theme tokens (per-component) |
| Tailwind | Gone (`tailwindcss` + `@tailwindcss/vite` removed in #176; `@import "tailwindcss"` removed from `index.css`; build precache -9 KB). The remaining `className=` in `Cell.tsx` is for a plain-CSS keyframe-animation hook, not Tailwind. | Gone |
| Server state | `PlayPuzzlePage` migrated to TanStack (#176 curation half — `useCurationPuzzle` wraps the load-saved-state → fetch-fresh-puzzle → save cascade in one `useQuery`). `DailyFlow` still on hand-rolled `useState<FlowState>` pending its own sub-slice. | TanStack `useQuery` / `useMutation` everywhere |
| `services/*` | Three legacy service modules (`adminService`, `dailyService`, `puzzleService`). `api.ts` was consolidated into `shared/api/` and the trio of near-identical helpers (`apiFetch` / `apiPost` / `apiPut`) collapsed to one `apiRequest` + thin `apiGet` / `apiPut` / `apiPost` wrappers (#120). | Hooks own the I/O; leaf components consume hooks |
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
  vite.config.ts              Vite + Vitest + PWA + tsconfig-paths + /api proxy
  playwright.config.ts        integration + e2e projects
  tamagui.config.ts           Track 2 placeholder; full migration is Track 3
  CLAUDE.md                   frontend conventions (additive to root CLAUDE.md)
```

## `src/` directory tree (with one-line file summaries)

### `src/` root

- `App.tsx` — Router composition (BrowserRouter + Routes); exports `FALLBACK_PUZZLE` used by tests. Global providers moved to `src/app/providers.tsx` in #176.
- `main.tsx` — React DOM bootstrap; mounts `<Providers>` from `src/app/providers.tsx`. Service-worker registration lives here (production-only).
- `App.test.tsx` — smoke test that renders the app and resolves the initial puzzle fetch.
- `index.css` — CSS custom-property tokens (light + dark) + animation keyframes. (`@import "tailwindcss"` removed in #176.)
- `test-setup.ts` — Vitest setup: jest-dom matchers + `matchMedia` mock for jsdom.
- `test-utils.tsx` — wraps RTL `render` / `renderHook` in `<StrictMode>` to catch impure updaters at unit-test time.
- `vite-env.d.ts` — Vite's ambient type reference.

### `src/app/` *(2026-05-20: introduced in #176)*

Bulletproof React app-composition layer.

- `providers.tsx` — `<Providers>`: composes `QueryClientProvider` (TanStack), `ThemeProvider`, `ClerkProvider` (conditional on `VITE_CLERK_PUBLISHABLE_KEY`), and `ClerkAvailabilityProvider`. Mounted by `main.tsx`.
- `router.tsx` — `<Router>`: BrowserRouter + Routes definitions for `/`, `/play`, `/curation`, `/admin`. `/play` uses an inline `<PlayRoute>` dispatcher that branches on `?flow=` to either `features/daily/screens/DailyFlow` or `features/curation/pages/PlayPuzzlePage` — the dispatch lives at the router level so features/curation/ doesn't import features/daily/.

### `src/engine/`

Pure-TS puzzle domain. No React, no fetch, no DOM. See `src/engine/README.md`.

- `types.ts` — `CellState`, `Mode`, `MODES`, `isMode`, `PuzzleData`, `Conflict`, `PuzzleMetadata`.
- `constraints.ts` — row / column / region / adjacency conflict detectors + `getAllConflicts` deduplicator.
- `validator.ts` — solution validator.
- Tests: `constraints.test.ts`, `validator.test.ts`.

### `src/shared/api/` *(2026-05-21: moved from `src/services/api.ts` in #120)*

Cross-feature backend fetch client. Replaces the legacy `services/api.ts` and collapses its `apiFetch` / `apiPut` / `apiPost` trio into one internal `apiRequest(method, path, body, opts)` with thin method-specific wrappers (`apiGet`, `apiPut`, `apiPost`). Single unified empty-body handling — every method resolves to `{}` on empty 200/204, where the old `apiFetch` would throw `SyntaxError: Unexpected end of JSON input`.

- `client.ts` — `apiRequest` + `apiGet` / `apiPut` / `apiPost` + `ApiError`.
- `index.ts` — barrel re-export (consumers `import { apiGet } from '@shared/api'`).
- Tests: `client.test.ts` (URL composition, ApiError shape, empty-body parity across all three methods, network-error propagation).

### `src/shared/hooks/` *(2026-05-18: PWA-related additions from #116)*

Cross-feature reusable hooks.

- `useOnlineStatus.ts` — `useSyncExternalStore` against `window.online`/`window.offline` events; returns `navigator.onLine` snapshot. Lower-level primitive; prefer `useConnectivity` for UI decisions.
- `useInstallPrompt.ts` — captures `beforeinstallprompt`; exposes `{ canInstall, isStandalone, promptInstall }`.
- `useConnectivity.ts` — *(2026-05-18: #116 follow-up)* authoritative connectivity hook; combines `useOnlineStatus` with an active HEAD probe of `/api/health` on mount. Re-probes on browser `online` events. Used by `OfflineBanner` and `LandingPage`.
- Tests: `useOnlineStatus.test.ts`, `useInstallPrompt.test.ts`, `useConnectivity.test.ts`.

### `src/shared/auth/` *(2026-05-20: moved from `src/components/auth/` in #176)*

Clerk-integration surface. Cross-feature: every role-gated UI consumes `getClerkUserRole`, and the chrome (`PageShell`) renders `SignInButton`/`UserMenu` conditionally on `useClerkAvailable()`.

- `ClerkAvailability.tsx` — context flag (provider + `useClerkAvailable` hook) telling consumers whether `<ClerkProvider>` was mounted. Provider is composed in `src/app/providers.tsx`.
- `SignInButton.tsx` — wraps Clerk's `<SignInButton mode="modal">` with branded styling. Used by `PageShell` and `AdminLandingPage`.
- `UserMenu.tsx` — wraps Clerk's `<UserButton>`; adds an "Admin" menu item for admins (AS-10).
- `role.ts` — `getClerkUserRole(publicMetadata)`. Single source of truth for the admin gate; consumed by `ProtectedAdminRoute`, `GameBoard`, `LandingPage`, and `UserMenu`.
- Tests: `SignInButton.test.tsx`, `UserMenu.test.tsx`.

### `src/shared/components/` *(2026-05-20: extended from `Icon` to full chrome in #176)*

Bulletproof React shared-component layer. Every page and feature consumes from here.

- `Icon.tsx` — brand-default wrapper around `lucide-react` icons (1.5 stroke, 20px size). Sites pass the icon via `as` prop: `<Icon as={ArrowLeft} />`.
- `PageShell.tsx` — top-of-page chrome (header, back button, subtitle, dark-mode toggle, auth slot, install button, offline banner).
- `Button.tsx` — *(2026-05-20: #208)* Tamagui-styled `PrimaryButton` / `SecondaryButton` / `GhostButton` + new `CompactSecondaryButton` (header/card size) + `CompactSecondaryLink` (router-aware `<Link>`-rendering twin of CompactSecondaryButton). All four share a single `styled(View, { render: <button/> })` base; hoverStyle + pressStyle drive the tactile-ink-shadow press effect via the `quickerLessBouncy` animation key (100ms ease-out). First Tamagui-styled component in the chrome.
- *(gone — #208)* `buttonStyles.ts` + `press.ts`. The pre-Tamagui `compactSecondaryButtonStyle` const moved into `Button.tsx`'s `compact` size variant; imperative `pressIn`/`pressOut` mouse handlers are replaced by Tamagui's declarative `hoverStyle` + `pressStyle`.
- `OfflineBanner.tsx` — *(2026-05-18: #116)* global offline indicator slotted into `PageShell`; renders when `useConnectivity()` returns `false`.
- `InstallButton.tsx` — *(2026-05-18: #116 follow-up)* compact install CTA in the PageShell header right-cluster.
- Tests: `Icon.test.tsx`, `PageShell.test.tsx`, `Button.test.tsx`, `OfflineBanner.test.tsx`, `InstallButton.test.tsx`.

### `src/services/`

Legacy backend client modules. New code uses `features/<feature>/services/` or wraps services in shared hooks (`shared/game/hooks/useUpdatePuzzleStatus`). The shared fetch base moved to `shared/api/` in #120; only three feature-specific services remain here.

- `puzzleService.ts` — just `updatePuzzleStatus` after #176's split (cross-feature; used by both curation + daily via `shared/game/hooks/useUpdatePuzzleStatus`).
- `adminService.ts` — pool / config CRUD (`fetchPoolStatus`, `updateConfig`, `createConfig`, `triggerReplenish`).
- `dailyService.ts` — daily flow (`getDaily`, `submitDailyResult`); injects `X-Device-Id` via `apiGet` / `apiPost`'s `options.headers`.

Already migrated out of this folder:

- `api.ts` → `shared/api/` (#120 — collapsed `apiFetch` / `apiPut` / `apiPost` into one `apiRequest` + thin wrappers; renamed `apiFetch` to `apiGet`).
- `verdictService.ts` → `features/curation/services/` (#176, PR #202).
- `landingService.ts` → `features/curation/services/enabled-modes-service.ts` (#176, PR #203 — renamed; only CurationPage ever consumed it).
- `puzzleService.ts::fetchNextPuzzle` → `features/curation/services/fetch-next-puzzle-service.ts` (#176, PR #205 — single-feature consumer once GamePage moved to features/curation/).

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

### `src/shared/game/` *(2026-05-20: consolidated grid + hooks here in #176)*

Cross-feature puzzle-rendering layer. Used by both the curation flow (via `features/curation/pages/PlayPuzzlePage`) and the daily flow (via `features/daily/screens/DailyGameBoard`). Lives in `shared/` because no single feature owns it.

**`components/`**
- `GameBoard.tsx` — the puzzle play surface (550+ LOC). Renders grid + completion overlay + skip modal. Accepts an `AdminVerdictSurface?` slot (curation flow's `VerdictSurface`) via DI — see `src/features/curation/` and `shared/game/types/admin-verdict-surface.ts`.
- `grid/` — custom hand-built grid UI (moved from `src/components/grid/` in #176):
  - `Grid.tsx` — measures the container, lays out cells, renders the region-border overlay. Uses `cellKey` from `@engine/cellKey`.
  - `Cell.tsx` — single cell; renders marker / exclusion mark; handles touch + mouse pointer-down with synthesized-mouse-event suppression.
  - `Marker.tsx` — rounded-square marker SVG for the Tactile theme.
  - `ExclusionMark.tsx` — small dot SVG for excluded cells.
  - `RegionBorderOverlay.tsx` — SVG overlay drawing region boundary lines + corner junctions.

**`hooks/`** (moved from `src/hooks/` in #176, except `useUpdatePuzzleStatus` which was already here)
- `useGame.ts` — gameplay reducer (history stack, drag intent, conflicts, isSolved).
- `useTimer.ts` — pause/resume timer with `restore()` for persistence and `stop()` for solved-state.
- `useGameStorage.ts` — IndexedDB CRUD wrapper (saveState / loadState / clearState / addCompletion). Used by `PlayPuzzlePage` (curation flow) AND `features/daily/screens/DailyFlow` + `DailyGameBoard`.
- `useUpdatePuzzleStatus.ts` — TanStack `useMutation` wrapper around `puzzleService.updatePuzzleStatus`.

**`types/`**
- `admin-verdict-surface.ts` — slot contract `AdminVerdictSurfaceComponent` consumed by `GameBoard`. Implementation lives in `features/curation/components/VerdictSurface.tsx`. Prevents the shared → features dependency that would otherwise be a BR violation.

### `src/features/curation/` *(2026-05-20: introduced in #176)*

Curation feature: admin-only puzzle review surface. The `/curation` route lands here; admin verdict submission inside any curation-flow `GameBoard` also threads through this feature.

- `pages/CurationPage.tsx` — admin-gated landing for the curation flow; presents `PuzzleSelector` for size/mode pick.
- `pages/PlayPuzzlePage.tsx` — curation/practice play route. Was `src/pages/GamePage.tsx` before #176; moved here when the GamePage was split (the `?flow=daily` branch collapsed into `src/app/router.tsx`).
- `components/PuzzleSelector.tsx` — size/mode preset selector + Play button. (Was misnamed `components/landing/PuzzleSelector` in legacy layout; moved here in #176.)
- `components/VerdictSurface.tsx` — admin verdict UI (completion + skip variants). Mounted by `GameBoard` via the `AdminVerdictSurface` prop (slot contract in `shared/game/types/admin-verdict-surface.ts`). Was previously in `shared/game/components/`; moved here in #176 because the verdict surface is curation-specific.
- `services/verdictService.ts` — `submitVerdict` (admin PUT to `/api/admin/puzzles/{id}/verdict`). Was previously in `src/services/`.
- `services/enabled-modes-service.ts` — `fetchEnabledModes` (GET `/api/config/modes`). Used by `CurationPage` to populate the size/mode picker. Was previously `src/services/landingService.ts`; renamed + relocated in #176 because the "landing" label was vestigial — only CurationPage ever consumed it.
- `services/fetch-next-puzzle-service.ts` — `fetchNextPuzzle` (GET `/api/puzzles/next`) + `NoPuzzlesAvailableError`. Was previously `src/services/puzzleService.ts::fetchNextPuzzle`; moved here in #176 when the GamePage split made the curation flow the sole consumer.
- `hooks/useSubmitVerdict.ts` — TanStack `useMutation` wrapper around `verdictService.submitVerdict`. Was previously in `shared/game/hooks/`.
- `hooks/useCurationPuzzle.ts` *(2026-05-21: #176 Services→TanStack curation half)* — TanStack `useQuery` wrapping the load-saved-state → fetch-fresh-puzzle → save cascade for `PlayPuzzlePage`. Replaces the page's pre-#176 hand-rolled `useState<LoadState>` discriminated union + `useEffect` + `fetchKey` retry counter. Surfaces `NoPuzzlesAvailableError` as a typed `query.error` (`error instanceof NoPuzzlesAvailableError` discriminates in the render).
- Tests: one per source file.

### `src/features/landing/` *(2026-05-20: introduced in #176)*

Landing feature: the public landing page at `/` with three top-level tiles (Daily / Packs placeholder / admin-only Curation).

- `pages/LandingPage.tsx` — tile-based landing. Reads `useUser` (Clerk) + `useConnectivity` (shared/hooks) to decide which tiles are enabled / visible. No I/O of its own — entirely self-contained.
- Tests: `pages/LandingPage.test.tsx`.

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
