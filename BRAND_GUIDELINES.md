# Brand Guidelines — Reign

Single source of truth for all visual design decisions. Every frontend component references this document. Generated via the ui-ux-pro-max skill, then refined through iterative design comparison.

**Style:** Tactile — layered shadows, thick borders, moderate rounding. Physical, substantial feel.

**Mood:** Warm, approachable, confident. Premium but not cold.

**Identity:** The grid and region colors ARE the brand. UI chrome stays warm and neutral so the puzzle pops.

---

## 1. Color System

Two distinct color layers: warm neutral UI chrome and bold vibrant region fills.

### 1.1 UI Chrome (Light Mode) — "Warm Ink"

Warm brown ink on cream. Indigo accent for CTAs.

| Role | Hex | CSS Variable | Usage |
|------|-----|-------------|-------|
| Ink (foreground) | `#2D2A26` | `--color-ink` | Primary text, borders, shadows |
| Background | `#F8F6F3` | `--color-background` | Page background |
| Surface | `#FFFFFF` | `--color-surface` | Cards, panels, modals |
| Body text | `#6B6358` | `--color-body` | Body copy, descriptions |
| Muted | `#9C9488` | `--color-muted` | Captions, timer, secondary text |
| Border | `#D6CFC5` | `--color-border` | Card borders, dividers |
| Accent | `#4F46E5` | `--color-accent` | CTAs, active states, links |
| Accent Hover | `#4338CA` | `--color-accent-hover` | Hovered accent elements |
| On Accent | `#FFFFFF` | `--color-on-accent` | Text on accent background |
| Accent Shadow | `#312E81` | `--color-accent-shadow` | Layered shadow under accent buttons |
| Destructive | `#DC2626` | `--color-destructive` | Errors, conflict highlights |
| Destructive BG | `#FEE2E2` | `--color-destructive-bg` | Conflict cell background |
| Success | `#16A34A` | `--color-success` | Puzzle completion |
| Success BG | `#F0FDF4` | `--color-success-bg` | Completion background |
| Ring | `#4F46E5` | `--color-ring` | Focus ring color |

### 1.2 UI Chrome (Dark Mode)

| Role | Hex | CSS Variable | Usage |
|------|-----|-------------|-------|
| Ink (foreground) | `#D6CFC5` | `--color-ink` | Primary text, borders, shadows |
| Background | `#161310` | `--color-background` | Page background |
| Surface | `#1F1C18` | `--color-surface` | Cards, panels, modals |
| Body text | `#9C9488` | `--color-body` | Body copy |
| Muted | `#6B6358` | `--color-muted` | Captions, timer |
| Border | `#3D3830` | `--color-border` | Dividers |
| Accent | `#818CF8` | `--color-accent` | CTAs, links (lighter indigo) |
| Accent Hover | `#A5B4FC` | `--color-accent-hover` | Hovered accent |
| On Accent | `#111111` | `--color-on-accent` | Text on accent |
| Accent Shadow | `#6366F1` | `--color-accent-shadow` | Layered shadow under accent buttons |
| Destructive | `#F87171` | `--color-destructive` | Errors |
| Destructive BG | `#450A0A` | `--color-destructive-bg` | Conflict cell background |
| Success | `#4ADE80` | `--color-success` | Completion |
| Success BG | `#052E16` | `--color-success-bg` | Completion background |
| Ring | `#818CF8` | `--color-ring` | Focus ring |

### 1.3 Region Colors

Nine colorblind-safe colors for grid regions at **bold saturation**. Hues are spaced for maximum perceptual distance under protanopia, deuteranopia, and tritanopia.

**Light Mode (bold fills)**

| Region | Name | Fill | On-Fill | Tailwind Class |
|--------|------|------|---------|----------------|
| 0 | Sky | `#93C5FD` | `#1E3A5F` | `region-sky` |
| 1 | Amber | `#FCD34D` | `#78350F` | `region-amber` |
| 2 | Rose | `#FDA4AF` | `#881337` | `region-rose` |
| 3 | Teal | `#5EEAD4` | `#134E4A` | `region-teal` |
| 4 | Violet | `#C4B5FD` | `#4C1D95` | `region-violet` |
| 5 | Orange | `#FDBA74` | `#7C2D12` | `region-orange` |
| 6 | Slate | `#94A3B8` | `#1E293B` | `region-slate` |
| 7 | Lime | `#BEF264` | `#365314` | `region-lime` |
| 8 | Fuchsia | `#E879F9` | `#701A75` | `region-fuchsia` |

**Dark Mode (deep fills)**

| Region | Name | Fill | On-Fill |
|--------|------|------|---------|
| 0 | Sky | `#1D4ED8` | `#DBEAFE` |
| 1 | Amber | `#B45309` | `#FEF3C7` |
| 2 | Rose | `#BE123C` | `#FFE4E6` |
| 3 | Teal | `#0F766E` | `#CCFBF1` |
| 4 | Violet | `#6D28D9` | `#EDE9FE` |
| 5 | Orange | `#C2410C` | `#FFEDD5` |
| 6 | Slate | `#475569` | `#E2E8F0` |
| 7 | Lime | `#4D7C0F` | `#ECFCCB` |
| 8 | Fuchsia | `#A21CAF` | `#FAE8FF` |

**Colorblind safety notes:**
- Sky/Teal differentiated by warmth (cool blue vs warm green-blue) and lightness.
- Rose/Orange differentiated by lightness and saturation; not adjacent in common confusion pairs.
- Violet/Fuchsia differentiated by saturation (muted vs vivid) and value.
- Slate provides a neutral anchor distinguishable from all chromatic regions.
- All fill/on-fill pairs meet WCAG AA (4.5:1) contrast minimum.
- Region boundaries (2.5px `--color-ink` borders) provide shape-based identification independent of color.

### 1.4 CSS Custom Properties

```css
:root {
  /* UI Chrome — Light (Warm Ink) */
  --color-ink: #2D2A26;
  --color-background: #F8F6F3;
  --color-surface: #FFFFFF;
  --color-body: #6B6358;
  --color-muted: #9C9488;
  --color-border: #D6CFC5;
  --color-accent: #4F46E5;
  --color-accent-hover: #4338CA;
  --color-on-accent: #FFFFFF;
  --color-accent-shadow: #312E81;
  --color-destructive: #DC2626;
  --color-destructive-bg: #FEE2E2;
  --color-success: #16A34A;
  --color-success-bg: #F0FDF4;
  --color-ring: #4F46E5;

  /* Region Fills — Light (Bold) */
  --region-0-fill: #93C5FD;
  --region-0-on-fill: #1E3A5F;
  --region-1-fill: #FCD34D;
  --region-1-on-fill: #78350F;
  --region-2-fill: #FDA4AF;
  --region-2-on-fill: #881337;
  --region-3-fill: #5EEAD4;
  --region-3-on-fill: #134E4A;
  --region-4-fill: #C4B5FD;
  --region-4-on-fill: #4C1D95;
  --region-5-fill: #FDBA74;
  --region-5-on-fill: #7C2D12;
  --region-6-fill: #94A3B8;
  --region-6-on-fill: #1E293B;
  --region-7-fill: #BEF264;
  --region-7-on-fill: #365314;
  --region-8-fill: #E879F9;
  --region-8-on-fill: #701A75;
}

.dark {
  /* UI Chrome — Dark */
  --color-ink: #D6CFC5;
  --color-background: #161310;
  --color-surface: #1F1C18;
  --color-body: #9C9488;
  --color-muted: #6B6358;
  --color-border: #3D3830;
  --color-accent: #818CF8;
  --color-accent-hover: #A5B4FC;
  --color-on-accent: #111111;
  --color-accent-shadow: #6366F1;
  --color-destructive: #F87171;
  --color-destructive-bg: #450A0A;
  --color-success: #4ADE80;
  --color-success-bg: #052E16;
  --color-ring: #818CF8;

  /* Region Fills — Dark (Deep) */
  --region-0-fill: #1D4ED8;
  --region-0-on-fill: #DBEAFE;
  --region-1-fill: #B45309;
  --region-1-on-fill: #FEF3C7;
  --region-2-fill: #BE123C;
  --region-2-on-fill: #FFE4E6;
  --region-3-fill: #0F766E;
  --region-3-on-fill: #CCFBF1;
  --region-4-fill: #6D28D9;
  --region-4-on-fill: #EDE9FE;
  --region-5-fill: #C2410C;
  --region-5-on-fill: #FFEDD5;
  --region-6-fill: #475569;
  --region-6-on-fill: #E2E8F0;
  --region-7-fill: #4D7C0F;
  --region-7-on-fill: #ECFCCB;
  --region-8-fill: #A21CAF;
  --region-8-on-fill: #FAE8FF;
}
```

---

## 2. Typography

### 2.1 Font Stack

| Role | Font | Fallback | Weight Range |
|------|------|----------|-------------|
| UI (headings + body) | Nunito Sans | system-ui, sans-serif | 400, 500, 600, 700, 800 |
| Mono (timer, stats) | Space Mono | ui-monospace, monospace | 400, 700 |

**Google Fonts import:**

```css
@import url('https://fonts.googleapis.com/css2?family=Nunito+Sans:wght@400;500;600;700;800&family=Space+Mono:wght@400;700&display=swap');
```

**Tailwind config:**

```js
fontFamily: {
  sans: ['"Nunito Sans"', 'system-ui', 'sans-serif'],
  mono: ['"Space Mono"', 'ui-monospace', 'monospace'],
}
```

### 2.2 Type Scale

Mobile-first. Base: 16px (1rem). All sizes in rem.

| Token | Size | Line Height | Weight | Color | Usage |
|-------|------|-------------|--------|-------|-------|
| `text-xs` | 0.75rem (12px) | 1.33 | 400 | `--color-muted` | Fine print |
| `text-sm` | 0.875rem (14px) | 1.43 | 400 | `--color-muted` | Captions, helper text |
| `text-base` | 1rem (16px) | 1.5 | 400 | `--color-body` | Body text |
| `text-lg` | 1.125rem (18px) | 1.56 | 500 | `--color-body` | Emphasized body |
| `text-xl` | 1.25rem (20px) | 1.4 | 700 | `--color-ink` | Subheadings |
| `text-2xl` | 1.5rem (24px) | 1.33 | 700 | `--color-ink` | Section headings |
| `text-3xl` | 1.875rem (30px) | 1.27 | 800 | `--color-ink` | Page titles |
| `text-4xl` | 2.25rem (36px) | 1.22 | 800 | `--color-ink` | Hero headings (desktop) |
| `text-timer` | 1.5rem (24px) | 1 | 700 | `--color-muted` | Timer display (mono) |

### 2.3 Typography Rules

- Nunito Sans for all text (headings and body). Distinguish hierarchy through weight and color, not font family.
- Timer and numeric displays use Space Mono with `font-variant-numeric: tabular-nums`.
- Headings use `--color-ink`. Body uses `--color-body`. Captions use `--color-muted`.
- Maximum line length: 65 characters for body text.
- No text smaller than 12px (0.75rem) anywhere in the UI.
- Letter-spacing: headings at -0.01em, body at default (0).

---

## 3. Visual Style — Tactile

The Tactile style creates depth through **layered shadows** and **thick borders**. It feels physical and substantial, like stacked paper or pressed buttons.

### 3.1 Borders

| Element | Width | Color | Notes |
|---------|-------|-------|-------|
| Cards, buttons, modals | 2px | `--color-ink` | All interactive containers |
| Grid outer | 2.5px | `--color-ink` | Puzzle perimeter |
| Region boundaries | 2.5px | `--color-ink` | Between cells of different regions |
| Cell internal | 1px | `rgba(0,0,0,0.07)` | Between cells within same region |
| Palette strips, inputs | 2px | `--color-ink` | Form elements |

### 3.2 Shadows (Layered)

Depth is created by offset `box-shadow`, not blur. This gives the "stacked" tactile feel.

| Element | Shadow | Notes |
|---------|--------|-------|
| Cards, stat panels | `0 3px 0 var(--color-ink)` | Subtle lift |
| Puzzle grid | `0 3px 0 var(--color-ink)` | Grid sits on surface |
| Buttons (primary) | `0 3px 0 var(--color-accent-shadow)` | Color-matched shadow |
| Buttons (secondary) | `0 3px 0 var(--color-ink)` | Ink shadow |
| Modals | `0 4px 0 var(--color-ink), 0 12px 32px rgba(0,0,0,0.08)` | Layered: offset + soft ambient |
| Option wrapper | `0 4px 0 var(--color-ink), 0 8px 24px rgba(0,0,0,0.10)` | Prominent containers |

**Button press:** On hover/active, `transform: translateY(1px)` and reduce shadow offset by 1px to simulate pressing down.

### 3.3 Border Radius

10px on all containers (cards, buttons, modals, inputs, grid). Consistent across the UI.

```css
--radius: 10px;
```

### 3.4 No Blur Shadows

The Tactile style does NOT use blur-based `box-shadow` for elevation on cards/buttons. Blur is only used for modal backdrop overlay (`backdrop-filter: blur`) and ambient shadows on prominent containers as a secondary layer.

---

## 4. Spacing and Layout

### 4.1 Spacing Scale

4px base unit, following Tailwind defaults.

| Token | Value | Tailwind | Usage |
|-------|-------|----------|-------|
| `--space-1` | 4px | `p-1` | Tight internal padding |
| `--space-2` | 8px | `p-2` | Icon gaps, inline spacing |
| `--space-3` | 12px | `p-3` | Small component padding |
| `--space-4` | 16px | `p-4` | Standard padding |
| `--space-6` | 24px | `p-6` | Card padding |
| `--space-8` | 32px | `p-8` | Section padding |
| `--space-12` | 48px | `p-12` | Large section gaps |
| `--space-16` | 64px | `p-16` | Page-level spacing |

### 4.2 Breakpoints

| Name | Width | Target |
|------|-------|--------|
| `sm` | 375px | Small phones |
| `md` | 768px | Tablets, large phones landscape |
| `lg` | 1024px | Desktop |
| `xl` | 1440px | Wide desktop |

Design mobile-first: default styles target `sm`, progressively enhanced at `md`, `lg`, `xl`.

### 4.3 Grid Component Sizing

| Element | Size | Notes |
|---------|------|-------|
| Grid cell (minimum) | 44x44px | WCAG touch target minimum |
| Grid cell (recommended) | 48-56px | Comfortable tap on mobile |
| Marker (filled dot) | 36% of cell width | Centered within cell |
| Cell internal border | 1px | `rgba(0,0,0,0.07)` |
| Region boundary | 2.5px | `--color-ink` |
| Grid outer border | 2.5px | `--color-ink` |

**Grid sizing formula:**
- Mobile (375px): `(375 - 2 * 16px padding) / gridSize`. For 9x9: ~38px (minimum 40px).
- Tablet (768px): cells scale up to 56-64px.
- Desktop (1024px+): grid capped at max-width, centered. Max cell size: 72px.

### 4.4 Layout Principles

- The grid is always centered horizontally.
- On mobile, the grid takes full available width minus page padding (16px each side).
- On desktop, the grid sits in a centered column with max-width of 600px.
- Timer and mode indicator float above the grid, right-aligned.
- Nothing else on screen during active gameplay. Navigation, stats, and settings are on separate screens or accessible via a single menu icon.

### 4.5 Z-Index Scale

| Token | Value | Usage |
|-------|-------|-------|
| `z-base` | 0 | Default content |
| `z-grid` | 10 | Grid cells (stacking context) |
| `z-marker` | 20 | Placed markers |
| `z-overlay` | 30 | Conflict highlights, hover states |
| `z-sticky` | 40 | Timer, floating UI |
| `z-modal` | 50 | Modals, completion screen |
| `z-toast` | 60 | Toast notifications |

---

## 5. Component Patterns

### 5.1 Grid Cell

States and their visual treatment:

| State | Fill | Border | Marker | Additional |
|-------|------|--------|--------|------------|
| Default | `--region-N-fill` | 1px internal or 2.5px region boundary | None | -- |
| Hovered | Slight brightness shift | Same | None | `cursor: pointer` |
| Focused (keyboard) | Same | 2px `--color-ring` ring inset | None | Visible on all backgrounds |
| Filled | `--region-N-fill` | Same | Marker visible | -- |
| Conflict | `--color-destructive-bg` | Same | Marker in destructive color | Brief pulse animation |
| Completed | `--region-N-fill` | Same | Marker in success color | Part of completion animation |

### 5.2 Marker (Default Theme)

- Shape: filled circle (pip/dot).
- Size: 36% of cell width.
- Color: `--color-ink`.
- Placement animation: fade-in + subtle scale (0.8 to 1.0), 150ms ease-out.
- Removal: fade-out, 100ms ease-in.

### 5.3 Timer Display

```
font-mono text-timer text-muted tabular-nums
```

- Position: above the grid, right-aligned.
- Format: `MM:SS` (or `H:MM:SS` if over an hour).
- No visual chrome — just the numbers.
- Starts counting on first cell interaction.

### 5.4 Buttons

All buttons: `border: 2px solid var(--color-ink)`, `border-radius: 10px`, `font-weight: 700`.

**Primary (accent):**

```
bg-accent text-on-accent border-ink rounded-[10px]
shadow: 0 3px 0 var(--color-accent-shadow)
hover: translateY(1px), shadow shrinks
```

**Secondary (outline):**

```
bg-surface text-ink border-ink rounded-[10px]
shadow: 0 3px 0 var(--color-ink)
hover: translateY(1px), shadow shrinks
```

**Ghost:**

```
bg-transparent text-muted border-transparent rounded-[10px]
no shadow
hover: text-ink, bg-muted-bg
```

### 5.5 Cards

Used for puzzle selection and stats display.

```
bg-surface border-2 border-ink rounded-[10px]
p-6 shadow: 0 3px 0 var(--color-ink)
hover: border-accent (optional)
```

### 5.6 Modal (Completion Screen)

```
/* Overlay */
bg-black/50 backdrop-blur-sm fixed inset-0 z-modal

/* Modal panel */
bg-surface border-2 border-ink rounded-[10px]
p-8 max-w-md w-[90%] mx-auto
shadow: 0 4px 0 var(--color-ink), 0 12px 32px rgba(0,0,0,0.08)
```

Content: completion time, percentile rank (if daily), "Next Puzzle" CTA.

### 5.7 Navigation

Minimal. During gameplay: only a back icon (top-left) and the timer (top-right).

Outside gameplay:
- Simple top bar with the Reign wordmark (Nunito Sans, 800 weight) left-aligned.
- No hamburger menu on desktop. Direct links: Play, Daily, Stats.
- On mobile: bottom tab bar with 3-4 items max (Play, Daily, Stats, Settings).
- Tab bar icons: SVG, consistent stroke width (1.5px), from a single icon family (Lucide).

---

## 6. Animation and Motion

### 6.1 Timing Tokens

| Token | Duration | Easing | Usage |
|-------|----------|--------|-------|
| `--duration-fast` | 100ms | ease-in | Exits, removals |
| `--duration-base` | 150ms | ease-out | Hovers, state changes, button press |
| `--duration-moderate` | 200ms | ease-out | Marker placement, focus transitions |
| `--duration-slow` | 300ms | ease-in-out | Modals, overlays, completion |

### 6.2 Interaction Animations

| Action | Animation | Duration | Easing |
|--------|-----------|----------|--------|
| Marker placement | Fade-in + scale 0.8 to 1.0 | 150ms | ease-out |
| Marker removal | Fade-out + scale 1.0 to 0.8 | 100ms | ease-in |
| Conflict highlight | Background pulse (normal to destructive-bg, 2 cycles) | 300ms per cycle | ease-in-out |
| Conflict shake | translateX(-2px, 2px, 0) | 200ms | ease-out |
| Puzzle completion | Ripple from last-placed marker outward across grid | 400ms total | ease-out |
| Button press | translateY(1px), shadow shrinks | 100ms | ease-out |
| Cell hover | Brightness shift | 150ms | ease-out |
| Modal enter | Fade-in + translateY(8px to 0) | 200ms | ease-out |
| Modal exit | Fade-out + translateY(0 to 8px) | 150ms | ease-in |

### 6.3 Reduced Motion

All animations wrapped in a `prefers-reduced-motion` check:

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}
```

When reduced motion is active:
- Marker placement: instant appear (no scale/fade).
- Conflict: static color change only (no pulse, no shake).
- Completion: static success state (no ripple).
- Modals: instant appear/disappear.
- Button press: no translateY, instant shadow change.

---

## 7. Accessibility Standards

### 7.1 Contrast Requirements

| Element | Minimum Ratio | Standard |
|---------|---------------|----------|
| Body text on background | 4.5:1 | WCAG AA |
| Heading text (ink) on background | 7:1+ | WCAG AAA |
| UI components (borders, icons) | 3:1 | WCAG AA |
| Region on-fill text | 4.5:1 | WCAG AA |
| Marker on region fill | 3:1 | WCAG AA |

### 7.2 Touch Targets

- Minimum interactive size: 44x44px (WCAG 2.1 AA).
- Grid cells meet this by default through sizing (see Section 4.3).
- Buttons: minimum height 44px, minimum width 44px.
- Icon-only buttons: use padding to extend tap area to 44px if the icon is smaller.
- Spacing between adjacent touch targets: minimum 8px.

### 7.3 Keyboard Navigation

- Grid cells are focusable via Tab (enters grid) then Arrow keys (navigate within grid).
- Enter/Space toggles marker placement on focused cell.
- Escape exits the grid focus context.
- Focus ring: 2px `--color-ring`, inset, visible on all backgrounds.
- Tab order matches visual reading order (left to right, top to bottom).

### 7.4 Screen Reader Support

- Grid announced as a table/grid role with row/column headers.
- Each cell announces: row, column, region name, current state (empty/filled/conflict).
- Marker placement/removal announced via `aria-live` region.
- Conflict announced immediately: "Conflict: row 3 column 5 violates adjacency constraint."
- Completion announced: "Puzzle complete. Time: 2 minutes 34 seconds."

### 7.5 Colorblind Accessibility

Region colors are chosen for colorblind safety, but color is never the sole differentiator:

- Region boundaries are always drawn with a 2.5px `--color-ink` border, providing shape-based identification.
- Themes may add pattern overlays (dots, stripes, crosshatch) as additional cues.
- Conflict state uses both color (red) and animation (pulse/shake) plus an icon indicator.
- Success state uses both color (green) and an icon (checkmark).

---

## 8. Theme Architecture

Themes are data objects consumed via React Context. Components never hard-code visual values.

### 8.1 Theme Token Interface

Each theme provides these tokens:

```
{
  id: string
  name: string
  marker: {
    component: React.ComponentType  // SVG or component for the marker
    size: number                     // as percentage of cell width (0-1)
  }
  grid: {
    outerBorderWidth: number
    outerBorderColor: string
    cellBorderWidth: number
    regionBorderWidth: number
    regionBorderColor: string
  }
  regions: {
    colors: RegionColor[]           // array of { fill, onFill } per region
    pattern?: PatternType           // optional pattern overlay for colorblind support
  }
  background: {
    light: string                   // CSS background value
    dark: string
  }
  animations: {
    placement: AnimationConfig
    removal: AnimationConfig
    completion: AnimationConfig
    conflict: AnimationConfig
  }
}
```

### 8.2 Default Theme Tokens (Tactile)

```
{
  id: 'tactile'
  name: 'Tactile'
  marker: { component: FilledCircle, size: 0.36 }
  grid: {
    outerBorderWidth: 2.5,
    outerBorderColor: 'var(--color-ink)',
    cellBorderWidth: 1,
    regionBorderWidth: 2.5,
    regionBorderColor: 'var(--color-ink)'
  }
  regions: {
    colors: [/* region palette from Section 1.3 */],
    pattern: undefined
  }
  background: {
    light: 'var(--color-background)',
    dark: 'var(--color-background)'
  }
  animations: {
    placement: { type: 'fade-scale', duration: 150, easing: 'ease-out' },
    removal: { type: 'fade-scale', duration: 100, easing: 'ease-in' },
    completion: { type: 'ripple', duration: 400, easing: 'ease-out' },
    conflict: { type: 'pulse-shake', duration: 300, easing: 'ease-in-out' }
  }
}
```

### 8.3 Theme Overrides

Alternative themes (Queens Classic, Gems, etc.) override any subset of these tokens. Unspecified tokens fall back to the Tactile defaults. The ThemeContext provider handles this merge.

Theme selection is persisted to localStorage. For authenticated users, it syncs to the server.

---

## 9. Tailwind Configuration

Extend the default Tailwind config with Reign's design tokens.

```js
// tailwind.config.js (relevant extensions)
module.exports = {
  theme: {
    extend: {
      colors: {
        ink: 'var(--color-ink)',
        background: 'var(--color-background)',
        surface: 'var(--color-surface)',
        body: 'var(--color-body)',
        muted: 'var(--color-muted)',
        border: 'var(--color-border)',
        accent: {
          DEFAULT: 'var(--color-accent)',
          hover: 'var(--color-accent-hover)',
          shadow: 'var(--color-accent-shadow)',
        },
        'on-accent': 'var(--color-on-accent)',
        destructive: {
          DEFAULT: 'var(--color-destructive)',
          bg: 'var(--color-destructive-bg)',
        },
        success: {
          DEFAULT: 'var(--color-success)',
          bg: 'var(--color-success-bg)',
        },
        ring: 'var(--color-ring)',
      },
      fontFamily: {
        sans: ['"Nunito Sans"', 'system-ui', 'sans-serif'],
        mono: ['"Space Mono"', 'ui-monospace', 'monospace'],
      },
      borderRadius: {
        DEFAULT: '10px',
      },
      boxShadow: {
        tactile: '0 3px 0 var(--color-ink)',
        'tactile-accent': '0 3px 0 var(--color-accent-shadow)',
        'tactile-lg': '0 4px 0 var(--color-ink), 0 12px 32px rgba(0,0,0,0.08)',
      },
      transitionDuration: {
        fast: '100ms',
        base: '150ms',
        moderate: '200ms',
        slow: '300ms',
      },
      zIndex: {
        base: '0',
        grid: '10',
        marker: '20',
        overlay: '30',
        sticky: '40',
        modal: '50',
        toast: '60',
      },
    },
  },
}
```

---

## 10. Pre-Delivery Checklist

Before shipping any frontend visual code, verify every item.

### Visual Quality
- [ ] All colors reference CSS custom properties, not raw hex values
- [ ] Light and dark mode tested independently
- [ ] Region colors match the bold palette from Section 1.3
- [ ] Cards/buttons/modals use 2px `--color-ink` border
- [ ] Layered shadows (offset, no blur) on all elevated elements
- [ ] Border-radius is 10px on all containers
- [ ] All icons are SVG from Lucide, no emojis
- [ ] Typography uses Nunito Sans (UI) and Space Mono (numbers)
- [ ] Headings use `--color-ink`, body uses `--color-body`, captions use `--color-muted`

### Tactile Style
- [ ] Buttons have offset shadow and press-down animation
- [ ] Cards have offset shadow
- [ ] Grid has 2.5px ink border with offset shadow
- [ ] Region boundaries are 2.5px ink, not just color changes
- [ ] No blur-based elevation shadows on cards/buttons (only on modal backdrop)

### Interaction
- [ ] All interactive elements have `cursor-pointer`
- [ ] All state transitions use `transition duration-150` minimum
- [ ] Hover states defined and smooth
- [ ] Focus states visible with 2px ring
- [ ] Touch targets meet 44x44px minimum
- [ ] No reliance on hover-only interactions (mobile has no hover)

### Accessibility
- [ ] Text contrast meets 4.5:1 (body) / 7:1 (headings on bg)
- [ ] UI component contrast meets 3:1
- [ ] `prefers-reduced-motion` respected
- [ ] `aria-label` on all icon-only buttons
- [ ] Keyboard navigation works: Tab, Arrow keys, Enter, Escape
- [ ] Screen reader announces game state changes via `aria-live`
- [ ] Region identification does not rely solely on color

### Layout
- [ ] Mobile-first: default styles work at 375px
- [ ] No horizontal scroll at any breakpoint
- [ ] Grid centered with correct padding at all breakpoints
- [ ] Content does not collide with safe areas (mobile notch, gesture bar)
- [ ] Timer and minimal chrome only during gameplay

### Performance
- [ ] Font loading uses `display: swap`
- [ ] No layout shift on font load (dimensions reserved)
- [ ] Grid renders without visible flash or reflow
- [ ] Animations use `transform` and `opacity` only (GPU-composited)

---

*This document is the single source of truth for Reign's visual design. All frontend agents, reviewers, and theme authors reference it. Changes require design review.*
