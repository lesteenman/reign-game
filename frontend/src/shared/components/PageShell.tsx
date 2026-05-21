import type { ReactNode } from 'react';
import { ArrowLeft, Sun, Moon } from 'lucide-react';
import { Show } from "@clerk/react";
import { styled, View, Text } from 'tamagui';
import { Icon } from './Icon';
import { IconButton } from './IconButton';
import { useDarkMode } from '@theme/useDarkMode';
import { SignInButton } from '@shared/auth/SignInButton';
import { UserMenu } from '@shared/auth/UserMenu';
import { useClerkAvailable } from '@shared/auth/ClerkAvailability';
import { OfflineBanner } from './OfflineBanner';
import { InstallButton } from './InstallButton';

/**
 * Page chrome — Tamagui-styled wrappers for the column shell, header
 * row, brand wordmark, and subtitle. Migrated from inline `style={}`
 * in #176 (PageShell Tamagui slice). Same `styled(View, ...)` pattern
 * as #210 Button; see `frontend/CLAUDE.md` for the Tamagui-v2-RC
 * workarounds (the `styled as any` cast at the file level).
 */

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const styledAny = styled as any;

const Shell = styledAny(View, {
  name: 'PageShell',
  flexDirection: 'column',
  alignItems: 'center',
  gap: 16,
  paddingHorizontal: 16,
  paddingTop: 16,
  paddingBottom: 24,
  minHeight: '100vh',
  backgroundColor: '$background',
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  color: '$ink',
});

const HeaderRow = styledAny(View, {
  name: 'PageShellHeader',
  flexDirection: 'row',
  alignItems: 'center',
  justifyContent: 'space-between',
  width: '100%',
  maxWidth: 600,
});

const HeaderRightCluster = styledAny(View, {
  name: 'PageShellHeaderRight',
  flexDirection: 'row',
  alignItems: 'center',
  gap: 8,
});

// 44×44 spacer that occupies the back-button slot when no `onBack` is
// passed, keeping the title centered. Extracted as a styled component
// so JSX-side prop typing doesn't trip the v2-RC inference bug — same
// reason we use the `styledAny` cast on `styled(...)` calls.
const HeaderSpacer = styledAny(View, {
  name: 'PageShellHeaderSpacer',
  width: 44,
});

// Render as `<h1>` so screen-readers + Testing Library's
// `getByRole('heading')` find the brand wordmark. Tamagui Text
// defaults to `<span>` which would lose the heading semantics — the
// `render: <h1 />` override forces the element. Same pattern as the
// `render: <button>` overrides in Button.tsx and IconButton.tsx.
const Title = styledAny(Text, {
  name: 'PageShellTitle',
  fontSize: '$8',
  fontWeight: '800',
  letterSpacing: -0.3,
  color: '$ink',
  render: <h1 />,
  // `<h1>` carries a non-zero block margin from the browser UA
  // stylesheet (Chromium: 0.67em top + bottom). Zero it out so the
  // wordmark sits visually centered in the header row alongside the
  // 44×44 icon-button slots. Tamagui's Text doesn't inject margin
  // itself; the override exists for the underlying HTML element.
  margin: 0,
});

const Subtitle = styledAny(Text, {
  name: 'PageShellSubtitle',
  color: '$muted',
  fontWeight: '600',
  fontSize: '$3',
  letterSpacing: 0.1,
  textAlign: 'center',
});

interface PageShellProps {
  children: ReactNode;
  /** When provided, renders a back button that calls this handler. */
  onBack?: () => void;
  /** Accessible label for the back button. Defaults to "Back to home". */
  backLabel?: string;
  /**
   * Optional caption rendered directly under the Reign wordmark.
   * Used by flow-specific pages (e.g. the daily flow renders
   * "Daily Puzzle · May 11"). Omit for generic pages.
   */
  subtitle?: ReactNode;
}

/**
 * Header-right auth slot. Renders Clerk-driven sign-in / user menu
 * when ClerkProvider is mounted; otherwise renders nothing so the
 * anonymous game still works in dev environments without a Clerk
 * key. The backend is still the source of truth for authorisation
 * (auth-surface.md AS-04).
 *
 * `<Show when="signed-in|signed-out">` is the v6 replacement for the
 * removed `<SignedIn>` / `<SignedOut>` components (Clerk core-3
 * upgrade). See @clerk/react 6 upgrade guide.
 */
function HeaderAuthSlot() {
  const clerkAvailable = useClerkAvailable();
  if (!clerkAvailable) {
    return null;
  }
  return (
    <>
      <Show when="signed-out">
        <SignInButton />
      </Show>
      <Show when="signed-in">
        <UserMenu />
      </Show>
    </>
  );
}

/**
 * Standard page layout wrapper. Header pinned to top, body content laid
 * out top-down within the remaining viewport. Keeps the puzzle visible
 * on phone-portrait viewports (e.g. 360x800, 390x844) — vertically
 * centering would push the header down and float the action row below
 * the fold.
 */
export function PageShell({
  children,
  onBack,
  backLabel = 'Back to home',
  subtitle,
}: PageShellProps) {
  const { isDark, toggle } = useDarkMode();

  return (
    <Shell>
      {/* Header row: back button (left) | title (center) | auth + dark-mode toggle (right) */}
      <HeaderRow data-testid="page-header">
        {onBack ? (
          <IconButton
            variant="ink"
            onClick={onBack}
            aria-label={backLabel}
            data-testid="back-button"
          >
            <Icon as={ArrowLeft} />
          </IconButton>
        ) : (
          // Spacer keeps the title centered when no back button is rendered.
          // Matches the 44×44 minimum tap target so the layout doesn't
          // shift between pages that do and don't take an onBack.
          <HeaderSpacer />
        )}

        <Title>Reign</Title>

        {/* Right: auth slot + install button + dark-mode toggle. The admin
            link no longer lives in the header directly — it's inside the
            user menu, role-gated per AS-10. */}
        <HeaderRightCluster>
          <HeaderAuthSlot />
          <InstallButton />
          <IconButton
            variant="muted"
            onClick={toggle}
            aria-label={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
            data-testid="dark-mode-toggle"
          >
            <Icon as={isDark ? Sun : Moon} />
          </IconButton>
        </HeaderRightCluster>
      </HeaderRow>

      <OfflineBanner />

      {subtitle && (
        <Subtitle data-testid="page-subtitle">{subtitle}</Subtitle>
      )}

      {children}
    </Shell>
  );
}
