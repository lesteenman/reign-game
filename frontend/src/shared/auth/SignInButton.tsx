import { SignInButton as ClerkSignInButton } from "@clerk/react";
import { compactSecondaryButtonStyle } from '../components/buttonStyles';
import { pressIn, pressOut } from '../components/press';

interface SignInButtonProps {
  /** Button label. Defaults to "Sign in". */
  label?: string;
}

/**
 * Thin wrapper around Clerk's `<SignInButton mode="modal">`. Clerk's
 * component forwards click events to its children, so we render our
 * branded button as the child to keep visuals consistent with the rest
 * of the app (BRAND_GUIDELINES §5.4 Secondary button). Shared style
 * and press handlers live in `shared/components/` so the admin landing
 * page's sign-out button can match byte-for-byte.
 */
export function SignInButton({ label = 'Sign in' }: SignInButtonProps) {
  return (
    <ClerkSignInButton mode="modal">
      <button
        type="button"
        data-testid="sign-in-button"
        style={compactSecondaryButtonStyle}
        onMouseDown={(e) => pressIn(e, 'var(--color-ink)')}
        onMouseUp={(e) => pressOut(e, 'var(--color-ink)')}
        onMouseLeave={(e) => pressOut(e, 'var(--color-ink)')}
      >
        {label}
      </button>
    </ClerkSignInButton>
  );
}
