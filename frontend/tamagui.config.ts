import { defaultConfig } from "@tamagui/config/v4";
import { createTamagui } from "tamagui";

/* Animations. We rely on `defaultConfig.animations` (CSS driver from
 * @tamagui/config/v4 — already an `@tamagui/animations-css` instance)
 * rather than registering our own. The default set ships
 * `quickerLessBouncy: 100ms ease-out` which matches BRAND_GUIDELINES
 * §5.4's button-press timing (current `transition: transform 100ms
 * ease-out`). Adding our own driver here would override defaultConfig's
 * full animation menu (slow/medium/lazy/bouncy/etc.) which we may want
 * for sheets + modals later. Reign components should reference keys by
 * intent, not duration; canonical mapping:
 *
 *   button hover/press         → "quickerLessBouncy" (100ms ease-out)
 *   (future) sheet open/close  → "medium"           (300ms ease-out)
 *   (future) toast slide       → "quick"            (150ms ease-out)
 */

/* =====================================================================
 * SYNC DISCIPLINE: the token values below mirror `frontend/src/index.css`.
 *
 * CSS is the source of truth for theme tokens — they're declared as
 * custom properties under `:root` (light) and `.dark` (dark) in
 * `src/index.css`. Tamagui needs literal hex values for its compiler
 * to statically extract styles (it can't resolve `var(--color-X)` at
 * build time), so we mirror them here.
 *
 * When a CSS custom property changes in `index.css`, update the mirror
 * here. The dual source of truth is intentional — the hybrid trade-off
 * captured in the #176 Tamagui-kickoff design fork: literal values
 * for compiler-readability + a single mental model (always check
 * index.css first, then propagate).
 *
 * Last full sync: 2026-05-20 (#208 Button migration — added `mutedBg`
 * for Ghost-button hover per BRAND_GUIDELINES §5.4).
 * ===================================================================== */

const lightColors = {
  // Chrome
  ink: '#2D2A26',
  background: '#F8F6F3',
  surface: '#FFFFFF',
  body: '#6B6358',
  muted: '#9C9488',
  // Soft surface tint used for muted-context hover states (e.g. GhostButton hover).
  // Sits between `background` (#F8F6F3) and `border` (#D6CFC5) so the
  // hover affordance reads as a subtle "page surface darkening" rather
  // than a hard outlined block.
  mutedBg: '#EDE7DA',
  border: '#D6CFC5',
  // Accent
  accent: '#4F46E5',
  accentHover: '#4338CA',
  onAccent: '#FFFFFF',
  accentShadow: '#312E81',
  // Status
  destructive: '#DC2626',
  destructiveBg: '#FEE2E2',
  success: '#16A34A',
  successBg: '#F0FDF4',
  ring: '#4F46E5',
  cellBorder: 'rgba(0, 0, 0, 0.15)',
};

const darkColors = {
  ink: '#D6CFC5',
  background: '#161310',
  surface: '#1F1C18',
  body: '#9C9488',
  muted: '#6B6358',
  mutedBg: '#2A2620',
  border: '#3D3830',
  accent: '#818CF8',
  accentHover: '#A5B4FC',
  onAccent: '#111111',
  accentShadow: '#6366F1',
  destructive: '#F87171',
  destructiveBg: '#450A0A',
  success: '#4ADE80',
  successBg: '#052E16',
  ring: '#818CF8',
  cellBorder: 'rgba(255, 255, 255, 0.15)',
};

/**
 * Light theme. Extends `@tamagui/config/v4`'s default light so Tamagui's
 * stock primitives (Sheet, Dialog, Input, etc.) get sensible defaults;
 * overrides the semantic keys (background, color, borderColor) with
 * Reign's tokens and adds Reign-specific aliases (`accent`,
 * `accentShadow`, etc.) for use via the `$tokenName` syntax in component
 * props (e.g. `<Button backgroundColor="$accent" />`).
 */
const light = {
  ...defaultConfig.themes.light,
  ...lightColors,
  // `background` is already in `lightColors`; the explicit assignments
  // below are the ones doing real work — Tamagui's defaults have
  // `color` + `borderColor` keys but Reign's tokens carry the same
  // values under `ink` and `border`, so we re-bind for Tamagui's
  // semantic-key consumers.
  color: lightColors.ink,
  borderColor: lightColors.border,
};

const dark = {
  ...defaultConfig.themes.dark,
  ...darkColors,
  color: darkColors.ink,
  borderColor: darkColors.border,
};

const config = createTamagui({
  ...defaultConfig,
  themes: {
    ...defaultConfig.themes,
    light,
    dark,
  },
});

export default config;

export type AppConfig = typeof config;

declare module "tamagui" {
  /**
   * Inform Tamagui's type system about our config so component props
   * (e.g. `<Button backgroundColor="$accent" />`) typecheck against
   * our tokens.
   */
  interface TamaguiCustomConfig extends AppConfig {}
}
