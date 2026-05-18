# Spec: Re-enable PWA service worker + offline / install UX

**Issue:** [#116](https://github.com/lesteenman/reign-game/issues/116)
**Date:** 2026-05-17
**Status:** Approved (pending final user review of this doc)

## Context

The PWA service worker was disabled when the project upgraded to Vite 8 because `vite-plugin-pwa` lacked Vite 8 support (upstream blocker [vite-pwa/vite-plugin-pwa#923](https://github.com/vite-pwa/vite-plugin-pwa/issues/923)). That blocker closed 2026-05-05 and `vite-plugin-pwa@1.3.0` (same day) now declares Vite ^8 in `peerDependencies`. Our frontend is on Vite ^8.0.13 — compatible.

The original T-113 implementation (commit `23b4079`) included Workbox precaching, Google Fonts runtime caching, and an offline-banner UX on `LandingPage`. The plugin was removed in commit `d0b41a6`; the offline-banner UX was removed in subsequent refactors and is no longer in the codebase.

This spec defines a clean re-introduction at the new tech baseline, plus expanded scope per #116's "verify install prompt + offline shell caching work":
- Workbox precache + Google Fonts runtime caching (as before).
- Service-worker auto-update.
- Global offline banner in `PageShell` (broader than T-113's LandingPage-only banner).
- Custom **Install app** tile on `LandingPage` that wraps the browser's `beforeinstallprompt` event (T-113 left install UX to the browser's built-in UI; this PR adds an in-app CTA).
- CloudFront cache-policy override for `/sw.js` and friends so SW updates propagate (the existing single `CachingOptimized` policy would cache `sw.js` for ~24h, which defeats `registerType: 'autoUpdate'`).

## Out of Scope

- **Tamagui migration.** Tamagui is configured in `frontend/tamagui.config.ts` but not adopted in any production component yet. Migrating LandingPage / PageShell to Tamagui as part of this PR would balloon scope and constitute "Track 3 Tamagui kickoff" — its own piece of work. **Action:** file a separate GitHub issue tracking the Tamagui migration (steps already enumerated in `frontend/tamagui.config.ts` comments) as a follow-up to this PR.
- **`src/pages/LandingPage.tsx` → `src/features/landing/pages/`** folder migration. Defers to the same Tamagui follow-up issue (the Track 3 landing slice covers both).
- **Custom "Update available" toast/prompt.** `registerType: 'autoUpdate'` activates new SW silently on next navigation. No UI work needed. T-113 originally specified `registerType: 'prompt'` but never built the prompt UI, so it never actually applied.
- **Playwright e2e for SW / install prompt.** SW lifecycle is fragile under headless and install behavior is device-dependent. Verification is via vitest units + manual smoke-test per #116.
- **API runtime caching.** Workbox doesn't cache `/api/*` responses (already denylisted from navigate-fallback too). Caching backend reads is the backend's concern via its own headers.

## Functional Requirements

### FR-1 — Service worker active in production builds
- After `vite build`, the SW (`/sw.js`) is generated and registered.
- New SW activates on next navigation after a deploy (auto-update, no user dialog).
- App shell (HTML, CSS, JS, manifest, icons, fonts) loads offline once the SW has primed its cache.

### FR-2 — Offline banner appears globally when navigator.onLine === false
- A banner renders at the top of every route when the browser reports offline.
- Banner uses `role="status"` (which carries implicit `aria-live="polite"`) so it's announced to screen readers without interrupting.
- Banner uses the existing `var(--color-destructive*)` token palette for visual consistency.
- Banner disappears when the browser reports back online.
- Detection combines `navigator.onLine` events with an active probe against `/api/health` on mount. The probe is necessary because the service worker serves the cached shell during a reload while offline — the browser never attempts a real network request, so navigator.onLine stays `true` and the offline event doesn't fire (post-merge user finding 2026-05-18).

### FR-3 — LandingPage tiles that require network are disabled while offline
- Daily and Curation tiles render as `aria-disabled="true"` with reduced opacity and `cursor: not-allowed` when offline.
- Tiles render with a tooltip ("Connect to the internet to start a new puzzle") via `title` attribute.
- When back online, tiles re-enable without page reload.

### FR-4 — Install-app button in PageShell header when installable
- A small "Install" button appears in the PageShell header's right-side cluster (between auth slot and dark-mode toggle) when:
  - The browser fired `beforeinstallprompt` (deferred install available), AND
  - The app is not already running in standalone mode (`matchMedia('(display-mode: standalone)').matches === false`).
- Visually parallel to the dark-mode toggle (compact, transparent background, color-muted text).
- Clicking calls `prompt()` on the deferred event. After the user accepts or dismisses, the button self-hides.
- If the browser never fires `beforeinstallprompt` (iOS Safari, desktop browsers without PWA install support), the button is never rendered.

### FR-5 — CloudFront serves `/sw.js` with `no-cache`
- An `ordered_cache_behavior` block in `infra/modules/frontend/main.tf` uses the AWS-managed `CachingDisabled` policy (`4135ea2d-6df8-44a3-9df3-4b5a84be39ad`) for `/sw.js`, `/workbox-*.js`, and `/registerSW.js`.
- Browser receives `Cache-Control: no-store, no-cache, must-revalidate` (per managed policy), so the SW is revalidated against S3 on every request.

## Architecture

### Files

| Area | File | Type | Notes |
|---|---|---|---|
| **PWA plugin** | `frontend/package.json` + `package-lock.json` | mod | Add `vite-plugin-pwa@^1.3.0` to devDependencies |
| | `frontend/vite.config.ts` | mod | Re-add `VitePWA({...})` block (autoUpdate, manifest:false, workbox precache + Google Fonts runtime caching) |
| | `frontend/src/main.tsx` | mod | Add `import { registerSW } from 'virtual:pwa-register'` + `registerSW({ immediate: true })` |
| **Infra** | `infra/modules/frontend/main.tf` | mod | Add 3 `ordered_cache_behavior` blocks (`/sw.js`, `/workbox-*.js`, `/registerSW.js`) with managed `CachingDisabled` policy |
| **Online status** | `frontend/src/shared/hooks/useOnlineStatus.ts` + test | new | `useSyncExternalStore` against `window.online`/`window.offline` events |
| **Offline UX** | `frontend/src/components/common/OfflineBanner.tsx` + test | new | Consumes `useOnlineStatus()`; renders aria-live banner |
| | `frontend/src/components/common/PageShell.tsx` + `PageShell.test.tsx` | mod | Slot `<OfflineBanner />` at top of shell |
| **Install UX** | `frontend/src/shared/hooks/useInstallPrompt.ts` + test | new | Captures `beforeinstallprompt`; exposes `{ canInstall, isStandalone, promptInstall }` |
| | `frontend/src/components/landing/InstallAppTile.tsx` + test | new | Renders only when `canInstall && !isStandalone` |
| | `frontend/src/pages/LandingPage.tsx` + `LandingPage.test.tsx` | mod | Render `<InstallAppTile />`; disable Daily/Curation when offline |

**Total:** ~15 file changes (10 source + 5 test). One new directory: `frontend/src/shared/hooks/`.

### Hook contracts

```ts
// frontend/src/shared/hooks/useOnlineStatus.ts
export function useOnlineStatus(): boolean;
// Returns navigator.onLine snapshot, re-renders on 'online' / 'offline' events.

// frontend/src/shared/hooks/useInstallPrompt.ts
export interface InstallPromptState {
  canInstall: boolean;       // beforeinstallprompt fired and not yet consumed
  isStandalone: boolean;     // matchMedia('(display-mode: standalone)').matches
  promptInstall: () => Promise<'accepted' | 'dismissed' | 'unavailable'>;
}
export function useInstallPrompt(): InstallPromptState;
```

### Component contracts

```tsx
// frontend/src/components/common/OfflineBanner.tsx
export function OfflineBanner(): JSX.Element | null;
// Returns null when online. When offline, returns a styled <div role="alert"> banner.

// frontend/src/components/landing/InstallAppTile.tsx
export function InstallAppTile(): JSX.Element | null;
// Returns null unless installable + not already standalone.
```

### Data flow

```
                          window.online/offline events
                                       │
                                       ▼
              useOnlineStatus()  ◄── useSyncExternalStore
                  │           └──────────────┐
                  ▼                          ▼
            OfflineBanner             LandingPage
            (PageShell)               (disables tiles)

           window.beforeinstallprompt event
                       │
                       ▼
           useInstallPrompt() ──► InstallAppTile (LandingPage)
                       │
                       ▼
           prompt() resolves → tile self-hides
```

### Styling convention

All new components follow the **existing inline `CSSProperties` + `var(--color-*)` token** pattern (same as `PageShell`, `LandingPage`). Each new file has a one-line comment at the top:

```ts
// Style: inline CSSProperties using theme tokens (no Tamagui).
// Tamagui migration tracked in #176.
```

## Trade-offs / Key Decisions

These go into the PR description's "Key Decisions" section verbatim.

1. **`registerType: 'autoUpdate'`, not `'prompt'`.** Simpler UX (no toast component), zero risk of users getting stuck on stale builds because they ignored the prompt. Trade-off: a navigation after deploy may have a brief reload moment. Acceptable for a casual puzzle game.

2. **`manifest: false` in `VitePWA`.** Our hand-written `public/manifest.json` is the source of truth. The plugin doesn't generate or merge a manifest. Confirmed `manifest: false` still works in `vite-plugin-pwa@1.x`.

3. **Inline `CSSProperties` styling, not Tamagui.** All new components match the existing convention. Tamagui adoption is a separate concern with its own tracking issue.

4. **Custom Install tile, not just the browser's built-in install UI.** Browsers vary wildly in install-prompt surfacing (Chrome omnibox icon, Edge button, Safari "add to home screen" hidden in Share menu). An in-app CTA is consistent across all platforms that fire `beforeinstallprompt`.

5. **Global offline banner in PageShell, not LandingPage-only.** Broader than T-113's original behavior. A user solving a daily puzzle on `/play` who loses connectivity deserves the same signal.

6. **CloudFront `CachingDisabled` policy for `/sw.js`, not just an S3 metadata Cache-Control header.** S3-level headers don't affect what CloudFront serves to repeat visitors at the edge. The CloudFront behavior is the authoritative cache control.

7. **No new Playwright e2e for SW / install prompt.** SW lifecycle is fragile in headless; install prompt is device-dependent. Vitest covers component / hook logic with mocked browser APIs; manual smoke-test on Android Chrome + iOS Safari per #116 covers the device-level behavior.

8. **Workbox runtime caching kept for Google Fonts only.** The existing `index.css` `@import url(fonts.googleapis.com/...)` is the only third-party network resource. Same shape as T-113.

12. **Active connectivity probe on mount.** `navigator.onLine` is event-driven and unreliable when the SW serves the cached shell — no real network request triggers the offline event. A single `fetch('/api/health', { method: 'HEAD', cache: 'no-store' })` on mount gives ground truth; re-probes when navigator dispatches `online`. Hook lives in `shared/hooks/useConnectivity.ts`; `useOnlineStatus` remains as the lower-level navigator-snapshot primitive.

## Risks

| Risk | Mitigation |
|---|---|
| `vite-plugin-pwa@1.x` may have moved `manifest: false` to a different option | Verify against the live plugin docs before committing the config; spec to be updated if option moved |
| `@tamagui/vite-plugin` not needed yet (out of scope) but could conflict with VitePWA plugin order later | Document plugin ordering convention in `vite.config.ts` comment |
| First user load after deploy may see brief flash as SW installs and reloads | `registerSW({ immediate: true })` ensures activation happens ASAP; trade-off documented |
| `beforeinstallprompt` doesn't fire on iOS Safari | Tile self-hides; iOS users still get "Add to home screen" via Share menu (no in-app prompt possible) — documented in code comment |
| Build size increase from `vite-plugin-pwa` + Workbox runtime | Acceptable; PWA is a project goal; pre-push `npm audit` blocks merge on bundle vulns |

## Verification plan

Per CLAUDE.md change workflow:

1. **TDD per task.** Each new hook + component lands with its vitest unit test first.
2. **Build verification:** `task build:frontend` clean, no TS errors, `dist/sw.js` exists and references precached assets.
3. **Local smoke-test:** `task dev:up`; open `http://localhost:5180` in Chrome; DevTools → Application → Service Workers shows registered + active; Application → Manifest shows manifest; toggle Network → Offline and confirm app shell still loads after a refresh.
4. **Manual smoke-test (per #116):**
   - Android Chrome on deployed `reign.acc.steenman.me` — verify install prompt fires, app installs to home screen, offline shell loads after first visit.
   - iOS Safari same URL — verify manifest accepted (no install prompt on iOS by design), app installs via Share → Add to Home Screen, offline shell loads.
5. **CD post-merge:** `Reign CD + Dependabot monitor` routine (twice daily) covers any infra apply failure.

## Tracking follow-ups

Filed during brainstorming:

- **[#176](https://github.com/lesteenman/reign-game/issues/176) — Complete Track 3: Bulletproof React feature-folder migration + Tamagui adoption.** Umbrella with 11 checkboxes covering the remaining moves (features/landing, features/game, features/curation, features/auth, shared/components extraction, src/app extraction, services → hooks via TanStack, services/api.ts consolidation, Tailwind retirement, Tamagui adoption, inline-CSS → Tamagui migration). Each checkbox is a self-contained slice; pick up as standalone PRs. The new files this PR adds (`InstallAppTile`, `useInstallPrompt`, `OfflineBanner`, `useOnlineStatus`) move with their hosts when those slices land.

## References

- GitHub issue: [#116](https://github.com/lesteenman/reign-game/issues/116)
- Original PWA work: commit `23b4079` (T-113 — "Add PWA support with manifest, Workbox service worker, and offline handling")
- Plugin removal: commit `d0b41a6`
- Upstream blocker (closed): [vite-pwa/vite-plugin-pwa#923](https://github.com/vite-pwa/vite-plugin-pwa/issues/923)
- Current Tamagui plan: `frontend/tamagui.config.ts` (file comment)
- Frontend architecture: `frontend/CLAUDE.md`, `.claude/skills/architecture/SKILL.md`
- AWS managed cache policy IDs: [CloudFront managed cache policies](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/using-managed-cache-policies.html)
