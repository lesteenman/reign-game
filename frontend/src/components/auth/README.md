# `src/components/auth/`

Clerk-driven authentication surface and role-gating helpers. Four production files plus two unit tests.

## Responsibility

Bridge the Clerk SDK (`@clerk/react`) into the rest of the app: provide auth-state context, render branded sign-in / user-menu UI, and expose the role extractor consumed by every role-gated UI surface.

## Data flow

- **In:** Clerk hooks (`useUser`, `<UserButton>`, `<SignInButton mode="modal">`, `<Show when="signed-in|signed-out">`) — fed by the `<ClerkProvider>` mounted in `main.tsx`.
- **Out:** Rendered into `PageShell` (via `HeaderAuthSlot`) and consumed by `features/admin/components/ProtectedAdminRoute`.

Auth state never reaches a service module — every backend call is server-side-authoritative; this surface is cosmetic per AS-04.

## Files

- **`ClerkAvailability.tsx`** — A React context flag (`true` / `false`) telling consumers whether `<ClerkProvider>` was mounted. Required because `useUser()` and `<Show>` throw without a provider, and `main.tsx` deliberately skips mounting the provider when `VITE_CLERK_PUBLISHABLE_KEY` is unset (dev-only escape hatch).
- **`SignInButton.tsx`** — Branded wrapper around Clerk's modal `<SignInButton mode="modal">`. Renders the project's secondary-button style.
- **`UserMenu.tsx`** — Branded wrapper around Clerk's `<UserButton>`. Adds an "Admin" menu item for admins (AS-10).
- **`role.ts`** — `getClerkUserRole(publicMetadata)` returns the `role` string field if present (`'admin'` / `'user'`), otherwise `''`. The canonical check is `=== 'admin'`.

Note: `ProtectedAdminRoute` moved to `features/admin/components/` in Track 3.

## State management

No local state. Reads from Clerk hooks; writes nothing.

## Rules specific to this directory

- **Never call `useUser()` (or any Clerk hook) without first checking `useClerkAvailable()`**. Otherwise the component throws on dev machines without a publishable key.
- **`getClerkUserRole` is the single source of truth.** Call sites: `GamePage.tsx` (admin verdict UI), `LandingPage.tsx` (Curation tile), `features/admin/components/ProtectedAdminRoute.tsx` (route gate), `UserMenu.tsx` (admin menu item). New role gates use this helper, not `user.publicMetadata.role` directly.
