import { type ReactNode } from 'react';
import { ClerkProvider } from '@clerk/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TamaguiProvider } from 'tamagui';
import { ThemeProvider } from '@theme/ThemeContext';
import { useDarkMode } from '@theme/useDarkMode';
import { ClerkAvailabilityProvider } from '@shared/auth/ClerkAvailability';
// Tamagui requires the config object at runtime (not just compile-time
// types). It lives at the frontend/ root by Tamagui convention + the
// Vite plugin expects it there, so this single import crosses the
// cross-folder relative-path rule. Documented exemption.
// eslint-disable-next-line no-restricted-imports
import tamaguiConfig from '../../tamagui.config';

// TanStack Query client (Track 2 foundation; first useQuery/useMutation
// adoptions land in Track 3 alongside the LoadState migration). Single
// instance for the whole app — recreating per render breaks caching.
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Conservative defaults; tune in Track 3 once we have real
      // usage to measure against.
      staleTime: 30 * 1000,
      refetchOnWindowFocus: false,
    },
  },
});

// Vite substitutes VITE_-prefixed env vars at build time. When this is
// missing (e.g. nobody created frontend/.env.local) we still want the
// anonymous game to work, so we boot the app without Clerk and log an
// explicit error. Sign-in won't work in that state — public routes
// continue to function. Throwing would bring the SPA down for anonymous
// visitors which breaks the game for every dev who hasn't configured
// keys yet.
const publishableKey = import.meta.env.VITE_CLERK_PUBLISHABLE_KEY;

if (!publishableKey) {
  console.error(
    'auth: VITE_CLERK_PUBLISHABLE_KEY is not set. Sign-in will not work. ' +
      'Copy frontend/.env.local.example to frontend/.env.local and set ' +
      'VITE_CLERK_PUBLISHABLE_KEY. See docs/runbooks/admin-auth-setup.md.',
  );
}

interface ProvidersProps {
  children: ReactNode;
}

/**
 * Tracks dark-mode state via `useDarkMode` and feeds it to Tamagui's
 * `defaultTheme` so Tamagui-styled components pick up the active theme.
 * Lives inside ThemeProvider so the Reign `useTheme()` API can be
 * called from Tamagui-styled components without provider-order surprise.
 */
function TamaguiThemedProvider({ children }: { children: ReactNode }) {
  const { isDark } = useDarkMode();
  return (
    <TamaguiProvider config={tamaguiConfig} defaultTheme={isDark ? 'dark' : 'light'}>
      {children}
    </TamaguiProvider>
  );
}

/**
 * App-level provider composition (Bulletproof React `app/providers.tsx`).
 * Wraps the router and feature tree with TanStack Query, theme,
 * Tamagui, and Clerk auth so feature code stays free of bootstrapping
 * concerns.
 */
export function Providers({ children }: ProvidersProps) {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <TamaguiThemedProvider>
          {publishableKey ? (
            <ClerkProvider publishableKey={publishableKey}>
              <ClerkAvailabilityProvider available={true}>
                {children}
              </ClerkAvailabilityProvider>
            </ClerkProvider>
          ) : (
            <ClerkAvailabilityProvider available={false}>
              {children}
            </ClerkAvailabilityProvider>
          )}
        </TamaguiThemedProvider>
      </ThemeProvider>
    </QueryClientProvider>
  );
}
