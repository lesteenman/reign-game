# `src/components/common/`

Cross-feature UI primitives. Four production files plus three unit tests.

## Responsibility

The shared chrome and primitive widgets every page reuses: the standard layout wrapper, the three button variants, the compact-button style for headers, and the press-animation handlers wired into both buttons and links.

## Data flow

- **In:** Imported by every page (`PageShell` is the layout wrapper) and by `components/auth/` (`SignInButton` uses `compactSecondaryButtonStyle` + `press` handlers).
- **Out:** Nothing; pure presentational primitives. `PageShell` reads `useDarkMode()` and `useClerkAvailable()` to compose the header.

## Files

- **`PageShell.tsx`** — Top-of-page chrome. Renders the header row (back button | "Reign" wordmark | auth slot + dark-mode toggle), an optional subtitle below the wordmark, and the children below. The auth slot is conditionally rendered via `<Show when="signed-in|signed-out">` only when Clerk is available.
- **`Button.tsx`** — Three button components: `PrimaryButton` (accent background), `SecondaryButton` (surface background), `GhostButton` (transparent, muted text). All share `baseStyle` and `disabledOverrides`. Hover handlers wire `press.ts` for the tactile-shadow animation.
- **`buttonStyles.ts`** — Exports `compactSecondaryButtonStyle` — a smaller secondary-button style for headers and cards (8×16 padding vs the full-sized 12×32). Kept as a CSS-properties object (not a component) so Clerk SDK wrappers (`<SignInButton>`, `<SignOutButton>`) can apply it as-is.
- **`press.ts`** — `pressIn` / `pressOut` mouse-event handlers that animate the "tactile ink shadow" (shrinks shadow from 3px to 2px while translating 1px down). Element-agnostic: applied to both `<button>` and `<a>`.

## State management

`PageShell` reads `useDarkMode()` (client state in `theme/useDarkMode.ts`). No other state.

## Rules specific to this directory

- **`PageShell` is the only layout wrapper.** Pages compose `<PageShell>...</PageShell>` at the root of every rendered tree (with one exception: `GameBoard` skips the wrapper when running in delegated mode under `DailyFlow`, because `DailyFlow` already mounts its own `PageShell` with the daily subtitle — see `GamePage.tsx:556-558`).
- **`compactSecondaryButtonStyle` and `Button.tsx`'s `SecondaryButton`** are intentionally separate primitives — one is 8×16, the other 12×32. Use the compact style in headers / cards; use the component in body buttons.
- **`Button.tsx`'s buttons wire `press.ts` to `onMouseEnter` / `onMouseLeave`** for hover-feel; header buttons (e.g. `SignInButton`, `AdminLandingPage`'s Home link) wire to `onMouseDown` / `onMouseUp` / `onMouseLeave` for click-feel. The `press` helper handles both — picking the right trigger is the caller's responsibility.

## Track 3 mapping

Everything in this folder moves to `src/shared/components/` (per `frontend/CLAUDE.md` architecture section).
