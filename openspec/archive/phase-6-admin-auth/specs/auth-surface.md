# Spec: Authentication Surface

The public, stable contract for how authentication appears in Reign.

## AS-01: Sign-in method is Google OAuth via Clerk, single provider

**Rule.** The only sign-in method offered to users is Google OAuth. The dance is delegated to Clerk. Email-password, magic link, Apple Sign-In, other OAuth providers, and SSO are all explicitly disabled in the Clerk dashboard.

**Value.** One provider means one failure mode and one UX flow. Adding providers later is additive work; disabling a shipped provider is subtractive and user-visible.

**Verification.** Clerk dashboard screenshot in a committed doc shows only Google enabled. Frontend renders exactly one option when the sign-in modal opens.

## AS-02: Anyone can create an account; no approval step

**Rule.** Any user who completes the Google OAuth flow receives a Clerk account. Account creation does not require admin approval, email confirmation, or any gate beyond a successful Google sign-in.

**Value.** Matches the design decision that auth is free in this phase. The flip to premium-gated signups is a future phase; until then, sign-up is open.

**Verification.** Integration test: sign in with a fresh Google account → account exists in Clerk; no manual step required.

## AS-03: Roles are `USER` (default) or `ADMIN` (assigned), stored in Clerk `publicMetadata`

**Rule.** Every Clerk account has a `publicMetadata.role` field. New accounts default to `"user"`. Admin role is assigned manually via the Clerk dashboard by someone with Clerk admin access. The role is a string, not an enum, but accepts only two values.

**Value.** Flat model. No hierarchy. Admin is a role, not a tier in a stack. Premium is NOT a role in this phase (see AS-06).

**Verification.** Fresh account has `publicMetadata.role === "user"` (or absent, treated as user by middleware). Manually-promoted account has `"admin"`.

## AS-04: Backend middleware enforces role, frontend never alone

**Rule.** Every role-gated operation has backend middleware (`RequireAuth` + `RequireAdmin` where applicable) as the source of truth. Frontend role-gated UI (e.g., hiding the Admin link from non-admins) is cosmetic convenience, not security.

**Value.** If the frontend code is ever compromised or bypassed (dev tools, direct API call, replay), the backend still rejects. This is standard defense-in-depth but worth calling out because the frontend will hide the Admin link and some future dev might assume the hiding is the security.

**Verification.** For each role-gated route, test: (a) signed-in non-admin makes a direct API call → 403, even though the UI never exposes the action. (b) Signed-in admin can trigger the action.

## AS-05: `USER` role has no features in this phase

**Rule.** A user who signs in with `role=user` sees no additional functionality beyond what an anonymous user sees. They get an avatar, a user menu, and a sign-out option — nothing more. All game-playing routes remain unauthenticated and identical for anonymous and user-role users.

**Value.** This phase scope is narrow — admin gating only. `USER` exists as a role so that (a) the "role missing" case doesn't need special handling, (b) future phases can add user-specific features without re-architecting auth.

**Verification.** E2E test: sign in with a non-admin test account, play a puzzle, complete it. Behavior identical to anonymous play (no leaderboard yet, no stats sync, etc.).

## AS-06: `isPremium` is NOT a role, and NOT in scope

**Rule.** Premium entitlement is a separate attribute (a boolean flag), not a role. This phase does not set, read, or gate on `isPremium`. The attribute is reserved for the flip phase.

**Value.** Premium and admin are orthogonal. A paying admin is possible. A non-paying admin is possible. Treating premium as a role creates a hierarchy problem we deliberately avoid.

**Verification.** Code search: no reference to `isPremium` in the Phase 6 slice diff. Glossary marks PREMIUM as "term reserved for flip phase."

## AS-07: Session lifetime is Clerk's default (7 days sliding)

**Rule.** Session TTL is not customized this phase. Clerk's default is a 7-day session with sliding extension on activity. Our backend reads whatever TTL Clerk enforces.

**Value.** Any custom session policy adds complexity without benefit at this phase's scope (single admin, low-traffic admin use). If we find 7 days too long or too short post-launch, configure in Clerk dashboard.

**Verification.** No code path in our backend or frontend enforces a session TTL. Clerk's dashboard config is the source of truth.

## AS-08: Sign-out clears Clerk session; local game state is unaffected

**Rule.** Signing out via the user menu invokes Clerk's sign-out, which clears the session cookie. The user's IndexedDB game state (history, stats, timer, in-progress puzzle) is not touched. The user returns to an anonymous state seamlessly.

**Value.** Game-state isolation from auth state. A user who signs out doesn't lose their puzzle in progress. Re-signing-in doesn't disturb local state.

**Verification.** E2E test: start a puzzle anonymously, sign in, sign out, confirm puzzle state is intact.

## AS-09: Admin route `/admin` has three rendered states

**Rule.** Navigating to `/admin`:

| Session state | Rendered view |
|---|---|
| Anonymous | Landing page: "Sign in as admin to continue." + sign-in button. |
| Signed-in with `role !== "admin"` | Landing page: "This account doesn't have admin access." + sign-out button. |
| Signed-in with `role === "admin"` | The admin UI (existing AdminPage component). |

**Value.** Clear messaging for each failure mode avoids the "why doesn't this work" UX trap. Non-admin signed-in users get a useful next step (sign out, try different account) rather than a 403 dead-end.

**Verification.** E2E tests for each state. Integration tests in Vitest for the component logic.

## AS-10: Role-gated UI elements are hidden from users without that role

**Rule.** Any UI element that triggers a role-gated operation is hidden, not shown-and-disabled, for users without the required role. Specifically: the "Admin" link in the user menu is absent (not dimmed) for non-admins.

**Value.** Disabled controls invite curiosity ("why can't I click this?"). Hidden controls don't advertise the feature's existence. For admin UI specifically, non-admins don't need to know it exists.

**Verification.** Frontend test: `<UserMenu>` rendered with `role="user"` has no Admin link in its DOM; with `role="admin"` has the link.
