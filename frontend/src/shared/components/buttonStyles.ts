import type { CSSProperties } from 'react';

/**
 * Compact secondary button used by header controls and forbidden-state
 * landing cards. Matches BRAND_GUIDELINES §5.4 secondary-button rules
 * at a smaller footprint than the full-sized `SecondaryButton` in
 * `Button.tsx` (8×16 padding vs. 12×32).
 *
 * Consumers wire the press animation via `pressIn` / `pressOut` from
 * `./press.ts` — typically on `onMouseDown`/`onMouseUp` for click-feel
 * rather than hover-feel, because these buttons live in tight header
 * or card layouts where hover is less informative than a click press.
 *
 * Kept as a style constant (not a component) so Clerk SDK wrappers
 * (`<SignInButton>`, `<SignOutButton>`) and bare `<button>` callers
 * can both apply it without losing their own typing or ref plumbing.
 */
export const compactSecondaryButtonStyle: CSSProperties = {
  padding: '8px 16px',
  border: '2px solid var(--color-ink)',
  borderRadius: 'var(--radius)',
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  fontWeight: 700,
  fontSize: '0.875rem',
  backgroundColor: 'var(--color-surface)',
  color: 'var(--color-ink)',
  cursor: 'pointer',
  boxShadow: '0 3px 0 var(--color-ink)',
  transition: 'transform 100ms ease-out, box-shadow 100ms ease-out',
  minHeight: '44px',
  minWidth: '44px',
};
