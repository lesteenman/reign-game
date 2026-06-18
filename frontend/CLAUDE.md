# Frontend — React + TypeScript Conventions

This file is auto-loaded by Claude Code when working on files under `frontend/`. The project-wide rules live in `/CLAUDE.md`; this file is additive.

## Architecture (Track 2)

**Feature-folder structure** (Bulletproof React–style). The `architecture` skill enforces these rules at design time and review time.

The engine, theme tokens/types, and storage interface live in
`@reign/core` (`packages/core`, #130); the web-specific remainders stay
under `frontend/src/`:

```
frontend/src/
  app/          app-level composition: router, providers, entry, framework config
  features/     product features (each one self-contained)
    auth/
      pages/        components mounted by the router (one or a few)
      screens/      OPTIONAL — sub-flow components used inside a page, NOT routed
      components/   leaf components specific to this feature
      hooks/        feature-specific hooks (own I/O via useQuery/useMutation)
      services/     OPTIONAL — feature-specific API surface (or skip; wire useQuery directly)
      types/        feature-specific types
    game/  daily/  curation/  admin/  landing/  (same shape)
  shared/       cross-cutting reusables (Tamagui-wrapped chrome, generic hooks, api base, cross-feature types)
  theme/        app-local theme: ThemeContext, useDarkMode, tactile (tokens+types → @reign/core/theme)
  storage/      app-local IndexedDB db.ts implementing @reign/core's GameStorage (types/utils/interface → @reign/core/storage)
```

### Path aliases (#198)

All cross-folder imports use `@layer/...` aliases. Only within-folder
siblings (`./X`) stay relative. Configured in `tsconfig.app.json` +
`tsconfig.json` (for Playwright esbuild) + `vite.config.ts` (via
Vite 8's native `resolve.tsconfigPaths: true` — was the
`vite-tsconfig-paths` plugin pre-#200). Enforced by ESLint rule
`no-restricted-imports` in `eslint.config.js`.

| Alias | Maps to | Status |
|---|---|---|
| `@app/*` | `src/app/*` | BR target home |
| `@shared/*` | `src/shared/*` | BR target home |
| `@features/*` | `src/features/*` | BR target home |
| `@theme/*` | `src/theme/*` | app-local theme remainder (ThemeContext/useDarkMode/tactile) |
| `@storage/*` | `src/storage/*` | app-local storage remainder (db.ts) |

All `@*/*` entries above resolve into `frontend/src`. The pre-#176 transitional
`@services/*` alias is gone — service modules live with their owning
feature (`features/admin/services/adminService`, `features/daily/services/dailyService`,
`shared/game/services/puzzleService`).

The engine, theme tokens/types, and storage interface live in the
`@reign/core` workspace package (`packages/core`, #130), not in `src`.
Import them via subpath exports — `@reign/core/engine`,
`@reign/core/theme`, `@reign/core/storage` — which resolve through the
npm-workspace symlink, not a tsconfig path. `@reign/core` is the leaf of
the dependency graph (no React/DOM/IndexedDB, imports nothing from
`frontend/src`) and is an allowed dependency from every layer.

### Import rules

- **No cross-feature imports.** `features/X` never imports from `features/Y`. Cross-feature dependencies go through `shared/`, `engine/`, or `theme/`. **Enforced by `import/no-restricted-paths` in `eslint.config.js`.**
- **`pages/` are routes only.** `features/X/pages/` holds ONLY components mounted by the router. Sub-flow components (intermediate screens within a flow) live under `features/X/screens/`. Leaf components live under `features/X/components/`.
- **No page-to-page imports.** A page never imports another page, even within the same feature. Shared sub-flow goes under `screens/`.
- **No `services/*` imports below `pages/`.** Leaf components and screens consume hooks; hooks own I/O. `useSubmitVerdict()` in a leaf is fine; `import { submitVerdict } from '...services/...'` in a leaf is a violation.
- **No type imports from `services/*`.** Type definitions belong in `types/` or `engine/`; services may re-export types but consumers must import from the source.
- **`engine/` is pure.** No React, no I/O, no DOM, no `fetch`. Only external libs.
- **`app/` is the top of the import graph.** Nothing imports from `app/`.

Drift detection grep examples (the architecture skill codifies these):
- Cross-feature: `grep -rn 'from .*features/[a-z]*' features/ | grep -v "features/$FEATURE_NAME"` for each feature
- Leaf I/O: `grep -rn 'from .*services/' features/*/components/`

## State Management

- **Server state**: TanStack Query (`@tanstack/react-query`). `useQuery` for reads, `useMutation` for writes. The `QueryClient` is composed in `app/providers.tsx`. No manual `useState<LoadState>` discriminated unions — those are the boilerplate TanStack eliminates.
- **Client state in-component**: `useState` / `useReducer`. Examples: the `useGame` reducer (move history), local form state.
- **Theme**: `theme/ThemeContext.tsx` provides theme tokens; consumed via `useTheme()`.
- **Auth**: Clerk Provider (`@clerk/clerk-react`). `useUser()` for auth state.
- **No Redux, no Zustand** (not currently needed; revisit when cross-feature client state emerges).

## UI Primitives

- **Tamagui 2 RC** for generic chrome (Button, Sheet, Dialog, Select, Tooltip, Tabs, Popover, Input, Toast). Configured in `tamagui.config.ts` (root of `frontend/`); tokens mirror `src/index.css`'s `:root` (light) and `.dark` (dark) CSS custom properties as literal hex values so the Tamagui compiler can statically extract them. Compiler enabled via `@tamagui/vite-plugin` in `vite.config.ts`. `<TamaguiProvider>` lives in `src/app/providers.tsx`, with active theme tied to `useDarkMode`. All Tamagui packages track the same version (`2.1.0`).
- **First Tamagui-migrated component:** `shared/components/Button.tsx` (#208). Pattern for follow-up migrations: `styled(View, { render: <button/>, variants: {variant, size}, hoverStyle, pressStyle, ... })` based on what `@tamagui/button` uses internally. Per-instance `render` override lets a single styled component swap its underlying element (e.g. `CompactSecondaryLink` renders as a react-router `<Link>`). Pseudo-state shadow changes (`hoverStyle: { shadowOffset }`) must re-declare `shadowColor` alongside; Tamagui only emits the composite `box-shadow` when both shadow props appear in the pseudo block. Text children: Tamagui's styled `<View>` derivatives reject raw text nodes; wrappers should auto-wrap strings/numbers in `<Text>` so consumers can keep writing `<PrimaryButton>Click</PrimaryButton>`. Style props: v4 default config sets `onlyAllowShorthands: true` AND has a TS-inference regression that breaks even shorthand keys in `Partial<StackStyle>` — the workaround is a single `as any` cast on `styled` at the file level (the runtime is correct; this matches how `@tamagui/button` itself ships).
- **v2-RC quirks worth knowing when migrating chrome:** *(observed across #210 Button, #216 PageShell, #176 chrome-cards)*
  - **Numeric style props on inline `<View>`/`<Text>` JSX trip the `Partial<...Style>` inference bug** (same bug the `styled as any` cast above sidesteps for `styled(...)` calls). Workaround: extract into a named `styledAny(View, { width: N })` instead of `<View width={N} />`. See PageShell's `HeaderSpacer` (#216) and DailyFlow's `LoadingCaption` (#176 chrome-cards) for examples.
  - **Some style keys leak as lowercase DOM attributes** when set on `View`-derived styled components — `lineHeight` → `lineheight`, `textAlign` → `textalign`. Symptom is React's "does not recognize prop on DOM element" console warning. Fix: move text-shape props (`textAlign`, `lineHeight`, `fontSize`, `fontWeight`, `fontFamily`, `color`) onto `<Text>`-derived children where Tamagui's text-style channel applies them; `View`-derived components should only carry layout + box-shape props (padding, border, background, flex).
  - **Font props don't cascade reliably from a `View`-`<div>` parent to its `<Text>` children** via CSS. Set font family / weight / size on the `<Text>` child directly.
  - **Tamagui's `disabled` prop only sets `aria-disabled`**, not the native HTML `disabled` attribute. Native `disabled` is what actually blocks click dispatch in browsers, so chrome buttons need a per-instance `render={<button disabled={d}/>}` override on top of the styled component's `disabled` prop. See Button.tsx + IconButton.tsx for the `renderEl(disabled)` helper.
  - **Tamagui's `<Text>` defaults to `<span>`**, losing heading role for `getByRole('heading')` and screen readers. For semantic headings, use `render: <h1 />` / `render: <h2 />` etc. on the styled Text component (UA stylesheets add a non-zero block margin to `<h1>`/`<h2>`/`<p>` so `margin: 0` is usually required alongside).
- **Custom game UI** (Grid, Cell, Marker, ExclusionMark, RegionBorderOverlay) is hand-built on Tamagui's low-level primitives (`<View>`, `<Stack>`, `<Text>`) — same tokens, ready for React Native later.
- **Tailwind is gone.** Removed from `package.json`, `vite.config.ts`, and `index.css` in #176. The remaining `className=` usage in `shared/game/components/grid/Cell.tsx` is for a plain-CSS keyframe-animation class hook (`animate-placement` / `animate-conflict`), defined in `index.css` — not Tailwind utility classes. New chrome code uses Tamagui primitives.
- **Icons.** Lucide via `lucide-react`. All icon usage goes through `shared/components/Icon` to inherit brand defaults (1.5 stroke, 20px size). Sites import the specific icon directly (`import { ArrowLeft } from 'lucide-react'`) and pass it via the `as` prop: `<Icon as={ArrowLeft} />`. Per-site overrides for `size` / `strokeWidth` are fine when needed (e.g. compact buttons). No emoji or Unicode glyphs for chrome icons — `BRAND_GUIDELINES.md` lines 437 + 692.

## TypeScript Conventions

- **Functional components only.** No class components.
- **Custom hooks for reusable logic.** Per-feature hooks live in `features/<feature>/hooks/`.
- **Strict TypeScript.** No `any`, no type assertions without justification.
- **Comments describe current state only.** No issue/PR/slice references in code comments (`// added in #327`, `// was X before #N`, `// lesson 13`) — git history + the commit message carry provenance. The issue number belongs in the commit message and PR description, never in the source.
- **File naming.** Components PascalCase (`Grid.tsx`); hooks camelCase (`useGame.ts`).
- **Mobile-first, responsive.** All components must work across mobile, tablet, and desktop viewports.
- **Accessibility: WCAG 2.1 AA minimum.** ARIA attributes, keyboard navigation, contrast ratios, alt text. Tamagui primitives handle most of this via Radix-equivalent internals.

## Security

- Sanitize all user inputs to prevent XSS.
- Never use `innerHTML` or equivalent unsafe DOM injection without explicit sanitization.
- Avoid storing sensitive data in localStorage/sessionStorage — Clerk handles session via httpOnly cookies.
- Escape dynamic content rendered in the DOM.
- Use `rel="noopener noreferrer"` on external links.
- No API keys, secrets, or sensitive configuration in client-side code (except browser-safe Clerk publishable keys baked at build time).

## Testing (TDD — non-negotiable)

Project-wide TDD rule in `/CLAUDE.md`. Frontend-specific:
- Structure every test Arrange-Act-Assert with explicit `// Arrange`, `// Act`, `// Assert` comments.
- **Vitest** for unit tests. Co-located with source: `Foo.tsx` + `Foo.test.tsx`. Run a single file: `npx vitest run <path/to/file.test.tsx>`.
- **Playwright** for e2e in `frontend/playwright/e2e/`; integration specs in `frontend/playwright/integration/`. See `.claude/skills/playwright-cli/` for browser automation patterns.
- Aim for above 90% coverage on hooks and services.
- Use `frontend/src/shared/test-utils.tsx` (wraps RTL `render` with providers); import as `@shared/test-utils`.
- **Mock external dependencies** (network via MSW or `page.route()`; not Vitest unit-mocks of services).

## Lessons (Reign-specific)

1. **Touch/pointer e2e tests first.** For any touch/pointer interaction code, write Playwright e2e tests before unit tests. jsdom does not simulate synthesized mouse events after touch events, so unit tests pass while the actual mobile experience is broken. The Phase 1 touch double-fire bug was only caught by user playtesting.
2. **First-paint correctness for visual components.** Never render a component at a default/placeholder size then resize after measuring. Use CSS-based sizing or defer rendering until the container is measured. Layout flicker is a user-visible bug.
3. **Validate URL params before type assertion.** When the frontend reads URL params and uses them as typed values (enums, numbers), validate against known values before type assertion. URL params are always `string | null` — invalid values passed unchecked will reach the API.
4. **Persisted data shapes live in `storage/`.** If a type is going to be saved to IndexedDB, define it once in the storage module and import from every consumer. Don't redeclare a shape in a hook and storage — they will drift.
5. **Playwright `request` and `page.request` have separate cookie jars.** When a test authenticates via the browser (`clerk.signIn({ page })`, `page.goto('/login')`, etc.), session cookies attach to the `page`'s `BrowserContext`. The standalone `request` fixture is a separate `APIRequestContext` with its own cookie jar — calls through it arrive cookie-less and 401 on auth-gated endpoints. Use `page.request.X(...)` instead, or alias `const request = page.request` at the top of the test. Note in the spec header why the alias exists so future readers don't refactor it back.
6. **Vite reads `.env*` files at dev-server start; HMR doesn't reload them.** Adding or changing a `VITE_*` variable while `task dev:up:frontend` (or `task e2e:up:frontend`) is running has no effect on the served bundle until the Vite process restarts. After editing `.env.local`: `task dev:restart:frontend` (or `task e2e:down:frontend && task e2e:up:frontend`).
7. **Grep for cross-chunk placeholders at chunk close.** When a slice is broken into chunks and an earlier chunk plants a stub for a later chunk (e.g. `'(post-completion screen lands chunk 5)'`, `// TODO chunk 4`, a literal "Coming soon" string), the later chunk MUST grep for the identifying string and confirm it's gone. Don't trust "I'll wire it later" — chunk-handoffs are exactly where stubs survive into shipped code. R-8-02's chunk-3 left a placeholder render branch for the post-completion screen; chunk-5 added the screen but never replaced the branch, so day-2 returning users saw the stub. The pattern: every placeholder string committed in chunk N is a grep target at chunk M's close-out (M > N).
8. **API response shape verification.** For each new API service function, verify the response type field names match the actual backend return type. Read the backend DTO (`backend/internal/handler/*Response`), not just the spec.
9. **Navigate after successful form actions.** After form submission that creates/updates data, navigate the user to the appropriate view.
10. **Null-safe user display names.** Components displaying user names must handle null/missing fields with fallback.
11. **No `!important` overrides.** Use Tamagui props or theme tokens; don't escape into raw CSS hacks.
12. **Extract shared constants/components immediately** for multi-page features. Don't copy-paste between pages.
13. **TanStack-migration test timing.** When migrating from a `useEffect`-driven `useState<LoadState>` cascade to `useQuery`/`useMutation`, audit existing tests for sync `getByTestId` assertions that depended on the synchronous-feeling `useEffect` chain in jsdom. TanStack's idle→pending→settled transitions cross a microtask boundary, so the loading/pending render arrives one tick later than the pre-migration code. Two fixes: (a) swap `getByTestId(loaded-element)` → `await findByTestId(loaded-element)` so the assertion waits past the boundary; (b) for transient pending states (mutation `submitting` rendering), use a deferred-promise mock (`new Promise<R>((resolve) => { resolveSubmit = resolve })`) so the pending render is observable long enough to assert against. Pre-migration tests can be **trivially-passing** on absence assertions (`queryByTestId(X).not.toBeInTheDocument()` returns null while the page is still in loading state) — when the test's intent is "the page rendered AND X isn't there", anchor on a known loaded-state element (`findByTestId('timer-display')`) first, then assert absence. See #212's PlayPuzzlePage metadata-test fixes and #215's daily-submitting fixes for worked examples.
14. **Browser smokes/e2e run against a fresh stack, torn down after — never a stale long-running dev server.** A dev Vite server left running across branch switches or a `node_modules` reinstall goes stale: it serves a 500 on the transform of files the change never touched (a stale module-graph/transform cache), which reads as a phantom test failure. This burned the smoke step twice. End each browser-test cycle with a clean state: bring up a dedicated stack, run, then tear it down (`task e2e:up` → run the spec → `task e2e:down`). When reusing the dev stack for a quick `playwright-cli` check is unavoidable, `task dev:restart:frontend` first so the Vite transform cache is fresh.
15. **Don't assert post-state on an action that triggers a flow transition — assert the transition.** When an interaction advances/unmounts/navigates (e.g. the final mark of a pack puzzle solves it and the flow auto-advances to the next, unmounting the board), an e2e helper that asserts the acting element's state right after will read stale/empty — the element is gone. Place the transition-triggering action without the post-state assertion, then assert the transition itself (the next screen / advanced subtitle / URL). A solve-on-same-board flow (curation shows a completion overlay) hides this; an advancing flow exposes it. Cost a CI cycle on the packs player flow.

## Quality Standards

- **Performance**: Optimize for Core Web Vitals (LCP, FID, CLS). Lazy-load where appropriate. Minimize bundle size. Tamagui's compiler hoists styles at build time — verify the compiler is running, not falling back to runtime mode.
- **Accessibility**: Tamagui ships ARIA via Radix-equivalent internals. Custom game-UI components must do their own work (keyboard nav for the grid, screen-reader announcements for state changes).
- **Responsiveness**: All UI must work across mobile, tablet, and desktop viewports.
