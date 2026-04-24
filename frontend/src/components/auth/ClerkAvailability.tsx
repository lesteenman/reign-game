import { createContext, useContext, type ReactNode } from 'react';

/**
 * Simple context flag telling the UI whether `<ClerkProvider>` was
 * mounted. We need this because `ClerkProvider` throws at construction
 * when the publishable key is missing — if a dev starts the app
 * without `VITE_CLERK_PUBLISHABLE_KEY` we boot without Clerk so the
 * anonymous (public) game still works. Any component that calls a
 * Clerk hook (e.g. `useUser`, `SignedIn`) would crash without a
 * provider, so consumers check this flag before rendering.
 *
 * This is a dev-only escape hatch. In production, the CD workflow
 * always injects the publishable key so `value` is always `true`.
 */
const ClerkAvailabilityContext = createContext<boolean>(false);

interface ClerkAvailabilityProviderProps {
  /** Whether `<ClerkProvider>` is mounted above this subtree. */
  available: boolean;
  children: ReactNode;
}

export function ClerkAvailabilityProvider({
  available,
  children,
}: ClerkAvailabilityProviderProps) {
  return (
    <ClerkAvailabilityContext.Provider value={available}>
      {children}
    </ClerkAvailabilityContext.Provider>
  );
}

/**
 * Returns `true` when a working `<ClerkProvider>` is mounted. Use this
 * in any component that calls Clerk hooks or renders Clerk components,
 * so the UI degrades gracefully in dev environments that haven't
 * configured a publishable key.
 */
export function useClerkAvailable(): boolean {
  return useContext(ClerkAvailabilityContext);
}
