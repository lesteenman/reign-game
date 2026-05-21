import { type ReactNode, type MouseEventHandler } from 'react';
import { styled, View } from 'tamagui';

/**
 * Icon-only chrome button (back button, dark-mode toggle, install
 * button, etc.). 44×44 minimum tap target per WCAG 2.5.5 / Apple HIG,
 * 8px padding, transparent background, no border or shadow. Distinct
 * from the action `<Button>` family (#208) which carries weight via
 * accent/secondary colors + the tactile-ink-shadow press effect —
 * those affordances would be visually overpowering for chrome.
 *
 * Variants:
 *   - `ink`   — full-opacity ink color (back button, primary affordance)
 *   - `muted` — muted color (dark-mode toggle, secondary affordance)
 *
 * Children are typically an `<Icon as={...} />` wrapping a Lucide
 * icon, but any ReactNode renders.
 *
 * **Implementation note.** Same `styled(View, { render: <button/> })`
 * pattern as #210 Button — see `frontend/CLAUDE.md` for the
 * Tamagui-v2-RC workarounds (`styled as any` cast, native HTML
 * `disabled` attribute forwarding, etc.). Reuses the same
 * `quickerLessBouncy` (100ms ease-out) animation key so hover/press
 * timing stays consistent with Button.
 */

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const styledAny = styled as any;

const IconButtonBase = styledAny(View, {
  name: 'IconButton',
  padding: 8,
  borderWidth: 0,
  backgroundColor: 'transparent',
  cursor: 'pointer',
  minWidth: 44,
  minHeight: 44,
  alignItems: 'center',
  justifyContent: 'center',
  animation: 'quickerLessBouncy',

  disabledStyle: {
    opacity: 0.4,
    cursor: 'not-allowed',
  },

  variants: {
    variant: {
      ink: {
        color: '$ink',
        hoverStyle: { color: '$accent' },
      },
      muted: {
        color: '$muted',
        hoverStyle: { color: '$ink' },
      },
    },
  },

  defaultVariants: {
    variant: 'ink',
  },
});

interface IconButtonProps {
  children: ReactNode;
  onClick?: MouseEventHandler<HTMLButtonElement>;
  disabled?: boolean;
  variant?: 'ink' | 'muted';
  'aria-label': string;
  'data-testid'?: string;
}

// See `Button.tsx`'s `renderEl` — Tamagui's `disabled` prop only sets
// `aria-disabled`; we override `render` per-instance to also forward
// the native HTML `disabled` attribute so the browser blocks click
// dispatch.
function renderEl(disabled: boolean | undefined) {
  return <button type="button" disabled={disabled} />;
}

/**
 * Icon-only chrome button.
 *
 * `aria-label` is required — icon-only buttons must announce their
 * purpose to screen readers (WCAG 2.1 SC 4.1.2). TypeScript enforces
 * non-omission via the required prop; empty-string is a hole we
 * accept for now (no `jsx-a11y/aria-label-prevent-empty` rule
 * configured yet — tracked alongside the broader a11y audit in #143).
 */
export function IconButton({
  children,
  disabled,
  variant = 'ink',
  ...rest
}: IconButtonProps) {
  return (
    <IconButtonBase
      variant={variant}
      disabled={disabled}
      render={renderEl(disabled)}
      {...rest}
    >
      {children}
    </IconButtonBase>
  );
}
