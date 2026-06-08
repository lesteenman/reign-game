# `src/theme/`

The web-specific theme wiring and dark-mode hook.

The theme **types** (`MarkerProps`, `ExclusionMarkProps`, `Theme`) live in `@reign/core/theme` (#130). This directory holds the React/DOM-bound remainder: the concrete Tactile theme (which wires app components), the theme context, and the dark-mode hook.

## Responsibility

Two things wrapped in one directory:
1. A pluggable visual theme — a JS object describing which marker / exclusion-mark components and which CSS animation class names to use for the puzzle grid. Today there's exactly one theme (`tactile`); a "Queens" chess theme is contemplated in the project overview.
2. A dark-mode hook that flips the `.dark` class on `<html>` (which then activates the dark-mode CSS custom-property block in `index.css`).

## Data flow

- **In:** `ThemeProvider` is mounted in `App.tsx`. `useDarkMode` is consumed by `PageShell.tsx`.
- **Out:** `useTheme()` returns the active theme object, consumed by `components/grid/Cell.tsx` to pick the marker + animation class names.

## Files

- **`tactile.ts`** — The default "Tactile" theme. Imports `Marker` + `ExclusionMark` from `shared/game/components/grid/` and the `Theme` type from `@reign/core/theme`; declares animation class names that match the keyframes defined in `index.css`. (Stays app-local because it wires React components — not platform-agnostic.)
- **`ThemeContext.tsx`** — `ThemeContext`, `ThemeProvider` (provides `tactileTheme`), `useTheme()` hook (throws if used outside provider).
- **`useDarkMode.ts`** — `prefers-color-scheme: dark` initial state with localStorage override (`reign-dark-mode` key). Adds/removes `.dark` class on `document.documentElement`. Returns `{ isDark, toggle }`.

## State management

- `ThemeContext`: provides a static theme reference (no state today; the theme object is module-level).
- `useDarkMode`: `useState<boolean>` initialized from `getInitialDark()` (matchMedia + localStorage). `useEffect` to sync the class on the html element.

## Rules specific to this directory

- **Tokens live in `index.css`.** This directory carries the JS-side theme abstraction (which components, which animation names). The CSS custom properties (`--color-ink`, `--region-N-fill`, etc.) live in `index.css` and toggle via the `.dark` class.
- **One theme today, abstraction stays.** Don't inline the Tactile theme — keep the `Theme` indirection so the future Queens theme drops in cleanly.
- **Lesson 11: no `!important`.** Theme overrides go through props or tokens, never via raw CSS escape hatches.

