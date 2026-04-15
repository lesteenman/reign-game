# Spec: PWA Basics

Covers R-01A (service worker, manifest, install prompt).

## Requirements

### PW-01: Web App Manifest

- `public/manifest.json` updated from Phase 0 stub
- Fields: `name: "Reign"`, `short_name: "Reign"`, `display: "standalone"`, `start_url: "/"`
- `theme_color` and `background_color` from BRAND_GUIDELINES.md (`--color-background`)
- Icons: at minimum 192x192 and 512x512 PNG (placeholder icons acceptable for Phase 1)
- `categories: ["games", "puzzle"]`

### PW-02: Service Worker (Workbox)

- Workbox configured via Vite plugin (`vite-plugin-pwa` or equivalent)
- Precaches all build output: HTML, JS, CSS bundles, fonts
- Runtime caching strategy for Google Fonts (if used): CacheFirst with 30-day expiry
- No puzzle data caching (API calls are not cached by the service worker)
- Service worker updates: prompt user to reload when new version available

### PW-03: Offline App Shell

- When offline, the app shell loads from service worker cache
- Navigation to `/` and `/play` works offline (HTML served from cache)
- Active puzzle resumes from IndexedDB — full gameplay including completion works offline
- CSS, JS, and fonts load from cache

### PW-04: Offline Connectivity Handling

- "Play" and "New Puzzle" buttons detect offline state before attempting API call
- If offline: show a non-blocking message ("You're offline — resume your current puzzle or connect to start a new one")
- "Resume" button works offline (no API call needed)
- Offline detection via `navigator.onLine` + `online`/`offline` events
- Tests: offline state shows appropriate message, resume remains available

### PW-05: No Custom Install Prompt

- No custom install prompt UI in Phase 1
- Browser's native install banner / "Add to Home Screen" is sufficient
- Manifest correctness ensures the browser can offer installation

## Acceptance Criteria

All PW-01 through PW-05 requirements pass. App is installable via browser prompt. App shell loads offline. Active puzzle is playable offline. New puzzle requests show offline message when disconnected.
