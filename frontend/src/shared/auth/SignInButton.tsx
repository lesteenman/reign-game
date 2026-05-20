import { SignInButton as ClerkSignInButton } from "@clerk/react";
import { CompactSecondaryButton } from '@shared/components/Button';

interface SignInButtonProps {
  /** Button label. Defaults to "Sign in". */
  label?: string;
}

/**
 * Thin wrapper around Clerk's `<SignInButton mode="modal">`. Clerk's
 * component forwards click events to its children, so we render our
 * branded button as the child to keep visuals consistent with the rest
 * of the app (BRAND_GUIDELINES §5.4 Secondary button).
 *
 * Uses the shared `CompactSecondaryButton` (Tamagui-styled, #208) so
 * the press effect, padding, and tap-target size stay byte-for-byte
 * matched with the admin landing page's home link and any other
 * compact-secondary CTA. The pre-#208 implementation imperatively
 * wired mouseDown/mouseUp/mouseLeave handlers via `press.ts`; that
 * file is gone (Tamagui's `pressStyle` does the same work declaratively).
 */
export function SignInButton({ label = 'Sign in' }: SignInButtonProps) {
  return (
    <ClerkSignInButton mode="modal">
      <CompactSecondaryButton data-testid="sign-in-button">
        {label}
      </CompactSecondaryButton>
    </ClerkSignInButton>
  );
}
