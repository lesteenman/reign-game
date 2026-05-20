import type { ReactNode, CSSProperties, MouseEventHandler, MouseEvent } from 'react';
import { pressIn, pressOut } from './press';

interface ButtonProps {
  children: ReactNode;
  onClick?: MouseEventHandler<HTMLButtonElement>;
  disabled?: boolean;
  type?: 'button' | 'submit' | 'reset';
  'aria-label'?: string;
  'data-testid'?: string;
}

const baseStyle: CSSProperties = {
  padding: '12px 32px',
  border: '2px solid var(--color-ink)',
  borderRadius: 'var(--radius)',
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  fontWeight: 700,
  fontSize: '1rem',
  cursor: 'pointer',
  transition: 'transform 100ms ease-out, box-shadow 100ms ease-out',
};

/**
 * Shared disabled styling for all button variants. Reduced opacity makes
 * the visual weight obviously different from the enabled state, while
 * `not-allowed` confirms the input is gated. BRAND_GUIDELINES does not
 * define an explicit disabled token — these values land within the
 * Tactile style's "muted" semantics (40% opacity preserves enough
 * contrast for legibility but kills the call-to-action read).
 */
const disabledOverrides: CSSProperties = {
  opacity: 0.4,
  cursor: 'not-allowed',
};

/** Accent-colored primary action button. */
export function PrimaryButton({ children, onClick, disabled, type = 'button', ...rest }: ButtonProps) {
  const handleMouseEnter = (e: MouseEvent<HTMLButtonElement>) => {
    if (disabled) return;
    pressIn(e, 'var(--color-accent-shadow)');
  };
  const handleMouseLeave = (e: MouseEvent<HTMLButtonElement>) => {
    if (disabled) return;
    pressOut(e, 'var(--color-accent-shadow)');
  };
  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      {...rest}
      style={{
        ...baseStyle,
        backgroundColor: 'var(--color-accent)',
        color: 'var(--color-on-accent)',
        boxShadow: '0 3px 0 var(--color-accent-shadow)',
        ...(disabled ? disabledOverrides : null),
      }}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    >
      {children}
    </button>
  );
}

/** Surface-colored secondary action button. */
export function SecondaryButton({ children, onClick, disabled, type = 'button', ...rest }: ButtonProps) {
  const handleMouseEnter = (e: MouseEvent<HTMLButtonElement>) => {
    if (disabled) return;
    pressIn(e, 'var(--color-ink)');
  };
  const handleMouseLeave = (e: MouseEvent<HTMLButtonElement>) => {
    if (disabled) return;
    pressOut(e, 'var(--color-ink)');
  };
  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      {...rest}
      style={{
        ...baseStyle,
        backgroundColor: 'var(--color-surface)',
        color: 'var(--color-ink)',
        boxShadow: '0 3px 0 var(--color-ink)',
        ...(disabled ? disabledOverrides : null),
      }}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    >
      {children}
    </button>
  );
}

/**
 * Ghost / tertiary button. Transparent background, muted text, no
 * border or shadow. For deliberate-but-rare actions (Skip, Cancel,
 * dismiss-style controls) where the visual weight of Primary or
 * Secondary would compete with the main CTA.
 *
 * Per BRAND_GUIDELINES §5.4.
 */
export function GhostButton({ children, onClick, disabled, type = 'button', ...rest }: ButtonProps) {
  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      {...rest}
      style={{
        ...baseStyle,
        backgroundColor: 'transparent',
        color: 'var(--color-muted)',
        border: '2px solid transparent',
        boxShadow: 'none',
        ...(disabled ? disabledOverrides : null),
      }}
    >
      {children}
    </button>
  );
}
