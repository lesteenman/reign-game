# Icon Library Adoption — Design

**Issue:** [#180 — Pick an icon library (lucide vs phosphor vs heroicons)](https://github.com/lesteenman/reign-game/issues/180)
**Date:** 2026-05-18
**Status:** Approved (awaiting writing-plans)
**Umbrella:** [#176 — Track 3 migration](https://github.com/lesteenman/reign-game/issues/176)

## Context

Surfaced from #179 (InstallButton ships as plain "Install" text). The original issue lists candidates (lucide-react, @phosphor-icons/react, heroicons, stay-on-Unicode) as if the library choice is open.

It is not. `BRAND_GUIDELINES.md` already specifies:

- Line 437 (Navigation chrome section): *"Tab bar icons: SVG, consistent stroke width (1.5px), from a single icon family (Lucide)."*
- Line 692 (Visual Quality checklist): *"All icons are SVG from Lucide, no emojis"*

So the design accepts the brand mandate (Lucide) and focuses on adoption mechanics rather than re-litigating the choice.

## Current state

Glyph sites in `frontend/src/` (non-test):

| Site | Glyph | Purpose |
|------|-------|---------|
| `components/common/PageShell.tsx:109` | `←` | Back button |
| `components/common/PageShell.tsx:153` | `☀` / `☾` | Dark-mode toggle (sun/moon) |
| `components/common/InstallButton.tsx` | text `Install` | PWA install CTA (no icon yet) |

Total: three sites. The migration is small enough to do in one PR.

## Decisions

### Library

`lucide-react@^1.16.0` (registry-verified 2026-05-18). License ISC (permissive, equivalent to MIT for the issue's licensing constraint). Peer dep `react ^16.5.1 || ^17.0.0 || ^18.0.0 || ^19.0.0` matches our React 19. Tree-shaken per-icon by default via ESM named exports (Vite/Rollup drops unused icons at build).

### `<Icon>` wrapper

Single shared component at `frontend/src/shared/components/Icon.tsx`. Pins brand defaults (size 20, strokeWidth 1.5) via component-level defaults, allowing per-site override through props.

```tsx
import type { LucideIcon, LucideProps } from 'lucide-react';

interface IconProps extends Omit<LucideProps, 'ref'> {
  as: LucideIcon;
}

export function Icon({ as: Component, size = 20, strokeWidth = 1.5, ...rest }: IconProps) {
  return <Component size={size} strokeWidth={strokeWidth} aria-hidden="true" {...rest} />;
}
```

`aria-hidden="true"` by default because every current adoption site is an icon-INSIDE-a-button where the surrounding `<button>` carries `aria-label`. Sites that want the icon to be its own labeled element override via `aria-hidden={false}` + `aria-label="..."`.

Lucide icons are imported per-site (`import { ArrowLeft } from 'lucide-react'`) and passed as the `as` prop. No barrel re-export — tree-shaking happens naturally and a barrel would just add an indirection that the user must keep in sync.

This component is also the first occupant of `shared/components/`, a folder #176's umbrella explicitly plans to create. Creating it here is on-path for that umbrella, not premature scaffolding.

### Adoption sites (all three migrate in this PR)

| Site | Before | After | Icon |
|------|--------|-------|------|
| Back button | `{'←'}` | `<Icon as={ArrowLeft} />` | `ArrowLeft` |
| Dark-mode toggle | `{isDark ? '☀' : '☾'}` | `<Icon as={isDark ? Sun : Moon} />` | `Sun` / `Moon` |
| Install CTA | text `Install` | `<Icon as={Download} />` + visible "Install" label retained | `Download` |

Install button is a labeled action button, not an icon-only chrome control, so the text stays alongside the icon. Sun/Moon and ArrowLeft are icon-only — the surrounding `<button>` keeps its existing `aria-label`.

### Convention update — `frontend/CLAUDE.md`

Add to the "UI Primitives" section:

> **Icons.** Lucide via `lucide-react`. All icon usage goes through `shared/components/Icon` to inherit the brand stroke/size defaults (1.5 stroke, 20px). Per-site override is fine via props. Import the specific Lucide icon directly (`import { ArrowLeft } from 'lucide-react'`) and pass it as `<Icon as={ArrowLeft} />`. No emoji or Unicode glyphs for chrome icons.

### Issue cross-link

Add a checkbox under #176's tooling/styling section: *"Icon library adopted — see #180. Lucide via lucide-react, wrapped by shared/components/Icon."*

## Testing

- **Unit tests.** `frontend/src/components/common/PageShell.test.tsx` and `InstallButton.test.tsx`: if any test asserts on the literal glyph text (`←`, `☀`, `☾`, `Install` text), refactor to assert via `aria-label` / `data-testid` (more robust). The Install button's "Install" text label remains, so any text-based assertion on it stays valid.
- **Playwright e2e.** All e2e specs that interact with these buttons identify them by `aria-label` or `data-testid` — no spec changes expected. To verify: grep `frontend/playwright/` for the three button identifiers and confirm no glyph-string match.
- **No new e2e spec needed.** This is a pure within-frontend change (no service boundary crossed), so the CLAUDE.md cross-boundary integration-test rule does not apply.

## Out of scope (deferred)

- **Tamagui integration.** `<Icon>` today emits a plain Lucide SVG. When `#176`'s Tamagui-adoption slice lands, the wrapper may re-wrap in a Tamagui `<View>` for token-aware color — that's a downstream change with its own PR.
- **Future icons.** Adding more icons (settings, profile, share, etc.) is on-demand as new features need them. No pre-emptive icon set selection.
- **Migrating non-existent glyph sites.** There are no other Unicode chrome glyphs in `frontend/src/` non-test code today.

## Risks / open questions

- **Lucide v1 changelog.** Lucide jumped from `0.x` to `1.x` not long ago. The plan task that installs the dep should confirm there are no breaking API changes vs the docs cited here (`LucideIcon`, `LucideProps`, `strokeWidth`, `size` prop semantics). If any rename happened, the wrapper code adjusts trivially.
- **Bundle impact.** Per-icon imports keep the cost to roughly four SVGs (ArrowLeft + Sun + Moon + Download). Expected delta: a few KB. Verify via `npm run build` before/after.

## Acceptance criteria (from #180)

- [x] Library chosen → Lucide (already mandated; this design ratifies the brand decision).
- [ ] First icon adopted → Install button gets `Download`. (Implementation.)
- [ ] `frontend/CLAUDE.md` updated with conventions. (Implementation.)
- [ ] Add as a checkbox under #176. (Implementation.)
