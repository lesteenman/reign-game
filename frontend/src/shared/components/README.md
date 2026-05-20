# `src/shared/components/`

Cross-feature UI primitives. Bulletproof React's shared-component layer — every page and feature consumes from here. Moved from `src/components/common/` in #176 (which kept `Icon` for `lucide-react` defaults).

## Responsibility

The shared chrome and primitive widgets every page reuses: the standard layout wrapper, the chrome button family (Primary / Secondary / Ghost / CompactSecondary), and the router-aware compact link.

## Data flow

- **In:** Imported by every page (`PageShell` is the layout wrapper) and by `shared/auth/` (`SignInButton` wraps `CompactSecondaryButton`).
- **Out:** Nothing; pure presentational primitives. `PageShell` reads `useDarkMode()` and `useClerkAvailable()` to compose the header.

## Files

- **`Icon.tsx`** — Brand-default wrapper around `lucide-react` icons (1.5 stroke, 20px size). Sites import the specific icon and pass via `as` prop: `<Icon as={ArrowLeft} />`. Pre-existed this folder.
- **`PageShell.tsx`** — Top-of-page chrome. Renders the header row (back button | "Reign" wordmark | auth slot + dark-mode toggle), an optional subtitle below the wordmark, and the children below. The auth slot is conditionally rendered via `<Show when="signed-in|signed-out">` only when Clerk is available.
- **`Button.tsx`** — Tamagui-styled chrome buttons (migrated in #208). Exports:
  - `PrimaryButton` (accent background, accentShadow press shadow)
  - `SecondaryButton` (surface background, ink press shadow)
  - `GhostButton` (transparent, muted text → ink on hover)
  - `CompactSecondaryButton` (header-size secondary, 8×16 padding + 44×44 min tap target)
  - `CompactSecondaryLink` (router-aware twin of `CompactSecondaryButton`; renders as a react-router `<Link>` via Tamagui's per-instance `render` override).
  All share a single `styled(View, { render: <button/> })` base. The tactile-ink-shadow press effect (BRAND_GUIDELINES §5.4) is wired via `hoverStyle` + `pressStyle` running the `quickerLessBouncy` animation (100ms ease-out from `@tamagui/config/v4`'s default CSS driver) — no imperative mouse handlers.
- **`OfflineBanner.tsx`** — Global offline indicator slotted into `PageShell`. Renders a `role="status"` banner when `useConnectivity()` returns `false`; returns `null` when online. Added in #116.
- **`InstallButton.tsx`** — Compact install CTA rendered in the PageShell header right-cluster. Calls `useInstallPrompt`; self-hides on iOS Safari / non-Chromium browsers / when already installed. Added in #116 follow-up.

## State management

`PageShell` reads `useDarkMode()` (client state in `theme/useDarkMode.ts`). No other state.

## Rules specific to this directory

- **`PageShell` is the only layout wrapper.** Pages compose `<PageShell>...</PageShell>` at the root of every rendered tree (with one exception: `GameBoard` skips the wrapper when running in delegated mode under `DailyFlow`, because `DailyFlow` already mounts its own `PageShell` with the daily subtitle — see `GameBoard.tsx`'s `isDelegated` branch).
- **Pick the right Button size.** `CompactSecondaryButton` is for headers and forbidden-state cards (8×16 padding, 14px font, 44×44 min tap target). `SecondaryButton` is for body CTAs (32×12 padding, 16px font). Same press feel — only the size differs.
- **Use `CompactSecondaryLink` for router navigation that reads as a button.** It renders as a react-router `<Link>` so client-side routing still works, with the compact-secondary visual style. Don't reach for raw `<Link style={...}>` for chrome links.
- **Don't pass `type="submit"` to the Button wrappers.** The Tamagui-styled base reserves `type` for its own styling system, so the wrapper interfaces don't expose it. Default is `type="button"`. If a form needs to submit on Enter, wire the form's `onSubmit` directly.
