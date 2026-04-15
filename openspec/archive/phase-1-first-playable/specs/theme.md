# Spec: Theme Architecture + Tactile Theme

Covers R-014 (theme architecture) and R-015 (Tactile default theme).

## Requirements

### TH-01: CSS Custom Properties

- All color, spacing, and shadow tokens defined as CSS custom properties in `src/index.css`
- Light mode tokens on `:root`, dark mode tokens on `.dark`
- Tokens match BRAND_GUIDELINES.md sections 1 (Color System) and 3 (Visual Style)
- Region colors: `--region-0-fill` through `--region-8-fill` and corresponding `--region-N-on-fill`
- Chrome colors: `--color-ink`, `--color-background`, `--color-surface`, `--color-body`, `--color-muted`, `--color-accent`, `--color-accent-shadow`

### TH-02: Theme TypeScript Interface

- `src/theme/types.ts` defines the `Theme` interface:
  - `id: string`
  - `name: string`
  - `marker: React.ComponentType<MarkerProps>` — the placed piece component
  - `exclusionMark: React.ComponentType<ExclusionMarkProps>` — the "not here" component
  - `animations: { placement: string, conflict: string, completion: string }` — CSS class names
- `MarkerProps`: `{ size: number, regionIndex: number }`
- `ExclusionMarkProps`: `{ size: number }`

### TH-03: ThemeContext + Provider

- `src/theme/ThemeContext.tsx` provides `ThemeProvider` and `useTheme()` hook
- `ThemeProvider` wraps the app in `App.tsx`
- `useTheme()` returns the current `Theme` object
- Default theme: Tactile
- Tests: `useTheme()` returns Tactile theme when no override provided

### TH-04: Tactile Theme Implementation

- `src/theme/tactile.ts` exports the Tactile theme object
- `id: 'tactile'`, `name: 'Tactile'`
- Marker component: filled circle SVG using `--color-ink`
- Exclusion mark component: cross SVG using `--color-muted`
- Animation classes defined in CSS (placement fade-in, conflict pulse, completion ripple)

### TH-05: Tactile Marker Component

- `src/components/grid/Marker.tsx` — renders a filled circle
- SVG circle, fill color from `--color-ink`
- Accepts `size` and `regionIndex` props
- Renders at the center of the cell
- Tests: renders without crashing, applies correct SVG attributes

### TH-06: Tactile Exclusion Mark Component

- `src/components/grid/ExclusionMark.tsx` — renders a cross (×)
- SVG with two crossing lines, stroke from `--color-muted`
- Accepts `size` prop
- Lighter visual weight than the marker (thinner stroke, muted color)
- Tests: renders without crashing, applies correct SVG attributes

### TH-07: Dark Mode Toggle

- Dark mode class (`.dark` on `<html>`) toggles all CSS custom properties
- Toggle mechanism: respect system preference via `prefers-color-scheme`, allow manual override
- Preference persisted to localStorage
- Tests: toggling dark mode switches CSS class on document element

### TH-08: Doc Updates

- BRAND_GUIDELINES.md section 8.2: change `id: 'minimalist'` → `id: 'tactile'`, `name: 'Minimalist'` → `name: 'Tactile'`
- GAME_DESIGN.md: rename "Default Theme: Minimalist" → "Default Theme: Tactile"

## Acceptance Criteria

All TH-01 through TH-08 requirements pass. Theme context provides the Tactile theme. Marker and exclusion mark render correctly. CSS custom properties match BRAND_GUIDELINES.md. Dark mode works. Docs updated.
