import { SignInButton as ClerkSignInButton } from '@clerk/clerk-react';
import type { CSSProperties } from 'react';

interface SignInButtonProps {
  /** Button label. Defaults to "Sign in". */
  label?: string;
}

// Same visual tokens as common/Button.tsx's SecondaryButton — this button
// sits in the top-right of the app shell next to the dark-mode toggle,
// so we keep the surface/ink treatment rather than the accent fill.
const buttonStyle: CSSProperties = {
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

function handlePressDown(e: React.MouseEvent<HTMLButtonElement>) {
  e.currentTarget.style.transform = 'translateY(1px)';
  e.currentTarget.style.boxShadow = '0 2px 0 var(--color-ink)';
}

function handlePressUp(e: React.MouseEvent<HTMLButtonElement>) {
  e.currentTarget.style.transform = 'translateY(0)';
  e.currentTarget.style.boxShadow = '0 3px 0 var(--color-ink)';
}

/**
 * Thin wrapper around Clerk's `<SignInButton mode="modal">`. When Clerk's
 * SignInButton receives children it forwards click events to them, so we
 * render our branded button as the child to keep consistent visuals with
 * the rest of the app (BRAND_GUIDELINES §5.4 Secondary button).
 */
export function SignInButton({ label = 'Sign in' }: SignInButtonProps) {
  return (
    <ClerkSignInButton mode="modal">
      <button
        type="button"
        data-testid="sign-in-button"
        style={buttonStyle}
        onMouseDown={handlePressDown}
        onMouseUp={handlePressUp}
        onMouseLeave={handlePressUp}
      >
        {label}
      </button>
    </ClerkSignInButton>
  );
}
