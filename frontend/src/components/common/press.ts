import type { MouseEvent } from 'react';

/**
 * Shared press-in / press-out handlers for the "tactile ink shadow"
 * style (BRAND_GUIDELINES §5.4). The shadow shrinks from 3px to 2px
 * while the element translates down 1px, giving a physical press feel.
 *
 * Element-agnostic: applied to `<button>` (Button.tsx, SignInButton)
 * and `<a>` (admin-landing Home link). The handler only touches
 * `e.currentTarget.style`, which is available on every HTMLElement.
 *
 * Triggers differ by call site:
 *   - `Button.tsx` (Primary/Secondary) wires these to
 *     `onMouseEnter` / `onMouseLeave` so the effect plays on hover.
 *   - Header-level buttons / links wire them to `onMouseDown` /
 *     `onMouseUp` / `onMouseLeave` so the effect plays on click.
 *
 * `shadowColor` is a CSS custom-property string, e.g.
 * `var(--color-ink)` or `var(--color-accent-shadow)`.
 */
export function pressIn(
  e: MouseEvent<HTMLElement>,
  shadowColor: string,
): void {
  e.currentTarget.style.transform = 'translateY(1px)';
  e.currentTarget.style.boxShadow = `0 2px 0 ${shadowColor}`;
}

export function pressOut(
  e: MouseEvent<HTMLElement>,
  shadowColor: string,
): void {
  e.currentTarget.style.transform = 'translateY(0)';
  e.currentTarget.style.boxShadow = `0 3px 0 ${shadowColor}`;
}
