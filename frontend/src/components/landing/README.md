# `src/components/landing/`

Landing-page-specific UI. One production file plus one unit test.

## Responsibility

A size/mode preset selector that renders one button per enabled `(size, mode)` combo from `/api/config/modes` and emits a selection on Play. **Misleading name:** despite the folder, this component is no longer rendered by `LandingPage.tsx` — it's used by `CurationPage.tsx` only. The Phase 7 redesign moved the public landing page to tiles (Daily / Packs / Curation) and pushed the size/mode preset selector to the admin-gated curation route.

## Data flow

- **In:** Rendered by `CurationPage.tsx`. Receives a `ModeEntry[]` and an `onSelect` callback.
- **Out:** No service calls. The parent (`CurationPage`) owns the `fetchEnabledModes` call.

## Files

- **`PuzzleSelector.tsx`** — Renders one button per `(size, mode)` combo plus a Play button. Selection is local state. Empty-state branch handles `modes.length === 0` ("No puzzles available right now"). Clamps `selectedIndex` to `modes.length - 1` so a shorter list after re-mount still picks a valid entry.
- **`InstallAppTile.tsx`** — Install CTA tile for `LandingPage`. Only rendered when `beforeinstallprompt` has fired (i.e. `canInstall === true`) and the app is not already running in standalone mode. Never shown on iOS Safari (which doesn't fire `beforeinstallprompt`). Added in #116.

## State management

Local: `useState<number>(0)` (selected index).

## Rules specific to this directory

- **No service imports.** The component is presentational; the parent feeds `modes` in. Type-only imports of `Mode` and `ModeEntry` from `services/` exist today — see the architecture findings for the rationale to move those types to `engine/types` or `shared/types/`.

## Track 3 mapping

`PuzzleSelector.tsx` moves to `features/curation/components/PuzzleSelector.tsx`. `InstallAppTile.tsx` stays in `components/landing/` for now and moves to `features/landing/components/` when #176's landing slice lands. The folder is deletable once both moves are complete.
