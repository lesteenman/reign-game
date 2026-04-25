import { createContext, useContext, type ReactNode } from 'react';

/**
 * Simple context flag telling the UI whether `<ClerkProvider>` was
 * mounted. We need this because:
 *
 *   1. `@clerk/react` v6 types `publishableKey` as a required `string`
 *      on `<ClerkProvider>`. Passing `undefined` is a TypeScript
 *      error.
 *   2. Passing an empty string at runtime makes Clerk fetch its JS
 *      bundle using `""` as the key and fail.
 *   3. Any component that calls a Clerk hook (e.g. `useUser`) or
 *      renders a Clerk gate (`<Show>`) invokes
 *      `useAssertWrappedByClerkProvider`, which throws when no
 *      provider is mounted.
 *
 * So when a dev runs the app without `VITE_CLERK_PUBLISHABLE_KEY`
 * (e.g. before R-089 lands) we deliberately skip mounting
 * `<ClerkProvider>` so anonymous gameplay still works, and set this
 * context flag to `false` so Clerk-dependent subtrees render nothing
 * instead of crashing.
 *
 * Dev-only escape hatch: in production the CD workflow always
 * injects the publishable key, so `value` is always `true`.
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
