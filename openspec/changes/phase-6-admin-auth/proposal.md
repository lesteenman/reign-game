# Phase 6: Admin Authentication via Clerk

## What

Gate `/api/admin/*` behind authenticated sessions using **Clerk** as the hosted auth provider with **Google OAuth** as the sole sign-in method. Any user can sign in; only users with `publicMetadata.role = 'admin'` reach admin routes. Backend middleware verifies Clerk sessions on every request; frontend surfaces a sign-in button in the header and renders role-gated UI (admin link in user menu, `/admin` route content) only when the session carries the admin role.

No DynamoDB user records, no IndexedDB→server sync, no premium flow this phase. Clerk stores all user identity (Clerk user ID, email, Google sub, display name, role). Our backend treats Clerk as the source of truth for sessions and role claims.

## Why

- **KI-009 is the last Critical pre-production KI.** `/api/admin/*` has no authentication at any layer. A public URL discovery trivially lets an anonymous caller mutate configs, trigger replenish, and read pool state. We can't expose the admin UI to any non-trusted network until this closes. Interim mitigations (threshold cap, CloudFront `Authorization` forwarding) are defense-in-depth, not a solution.
- **Auth was scoped to Phase 10+ (R-075) but that's too late.** Production deploy is gated on admin auth. Pulling auth forward unblocks that path.
- **Hosted auth removes 1-2 slices of security-critical code.** PKCE, state nonce, JWT signing keys, refresh-token rotation with reuse detection, JWKS caching, `email_verified` checks — all are Clerk's problem. The cost is one vendor dependency with exportable data.
- **Scope is minimum-viable on purpose.** Every adjacent feature (user records in our DB, IndexedDB sync, leaderboards, premium purchase) is a separate phase. This phase does one thing: close KI-009.

## Summary of Locked Decisions

Decisions from the design grill (`design-grill-summary.md`). In one-line form:

1. **Provider:** Google OAuth via Clerk. Not Cognito (pre-existing), not Apple yet, not magic link yet, not self-built.
2. **Role model:** Flat enum `USER | ADMIN` stored in Clerk `publicMetadata.role`. `isPremium` is a separate attribute for later, not a role.
3. **No user records in our DB.** This phase only gates admin routes. Per-user server state is a separate phase when a feature needs it.
4. **No IndexedDB→server sync.** Anonymous play and signed-in play share the same local state; merging is a separate phase.
5. **Session architecture:** Clerk SDK handles sessions (httpOnly cookies, PKCE, refresh-token rotation). Backend uses Clerk's Go server SDK to verify sessions.
6. **Middleware:** `RequireAuth` + `RequireAdmin` chi middleware applied to `/api/admin/*`. No other routes change.
7. **Frontend:** Sign-in button in header always visible; avatar menu when signed in; admin link in menu only if role=admin; role-gated UI hidden from users without the role; backend still enforces.
8. **Cost model:** Start on Clerk free tier. Flip new signups to premium-gated at ~few hundred active users. Grandfather existing accounts with `isPremium = true` + `isEarlyAdopter = true`. Monitor MAU manually at milestones (200 / 400 / 800).
9. **Glossary:** Retire FREE (state, not role). Add USER (signed-in). Redefine ADMIN (signed-in + role claim). PREMIUM deferred.

## Acceptance Criteria

A single cycle of Phase 6 is done when:

- **AC-1. Clerk integrated.** Clerk React SDK installed in `frontend/`; Clerk Go server SDK installed in `backend/`. GCP OAuth app registered; Clerk dashboard configured with Google as the sole OAuth provider.
- **AC-2. Backend middleware in place.** `RequireAuth` verifies Clerk session and rejects with 401 if absent. `RequireAdmin` additionally checks `publicMetadata.role === 'admin'`; rejects with 403 otherwise. Both middleware applied to all five `/api/admin/*` routes via chi router group.
- **AC-3. Frontend sign-in flow works.** Anonymous user clicks sign-in button → Clerk OAuth flow → Google → redirect → signed-in. Avatar + display name replace the sign-in button in the header.
- **AC-4. Admin link gated.** The "Admin" item in the user menu is present only when the signed-in user has `role === 'admin'`. No visual flicker (no "admin link appears for 100ms then disappears").
- **AC-5. Admin route gated.** Navigating to `/admin`:
  - Anonymous → landing page with "Sign in as admin to continue" + Clerk sign-in button.
  - Signed-in non-admin → "This account doesn't have admin access" page + sign-out button.
  - Admin → normal AdminPage UI.
- **AC-6. KI-009 closes.** All five admin routes (`GET /api/admin/pool`, `PUT /api/admin/config/{size}/{mode}`, `POST /api/admin/config`, `POST /api/admin/replenish`, and any future admin route) return 401 for anonymous callers and 403 for signed-in non-admins. Integration test in `backend/internal/handler/admin_*_test.go` proves it for each route.
- **AC-7. No user records in our DynamoDB.** `backend/internal/repository/` gains no `user.go` file. `puzzle-pool` table schema is unchanged. Clerk stores all identity.
- **AC-8. CLAUDE.md role table rewritten.** Current FREE/PREMIUM/ADMIN table is replaced with: Anonymous (no account) / User (signed-in via Clerk) / Admin (signed-in with role=admin claim). The "(pre-production)" note on KI-009 is removed.
- **AC-9. GLOSSARY.md updated.** FREE entry retired. USER entry added. ADMIN entry redefined. PREMIUM entry marked as "term reserved for flip phase."
- **AC-10. ROADMAP.md updated.** Phase 6 header becomes "Admin Authentication via Clerk" with slices from `tasks.md`. Verdict System moves to Phase 7; downstream phases renumbered. KI-009 strikethrough-closed with reference to this phase.

## Scope

### In Scope

- Clerk account setup (free tier) + Clerk dashboard configuration.
- GCP OAuth app registration.
- Clerk React SDK integration in `frontend/` (sign-in button, `ClerkProvider`, `useAuth` hooks).
- Clerk Go server SDK integration in `backend/` (`RequireAuth`, `RequireAdmin` middleware).
- Backend route wiring under `r.Route("/api/admin", ...)` with middleware applied.
- Frontend header changes (sign-in button + user menu).
- Admin route landing pages for three states (anonymous, non-admin, admin).
- CLAUDE.md role table rewrite.
- GLOSSARY.md updates.
- ROADMAP.md restructuring.
- Integration tests proving KI-009 closure.
- Documentation: how to grant admin role via Clerk dashboard.

### Out of Scope (Deferred)

- **DynamoDB user records.** No `user.go` repository. Add when per-user server state ships.
- **IndexedDB → server sync.** Anonymous and signed-in play share the same local state for now.
- **Apple Sign-In** (add when iOS App Store submission imminent).
- **Magic link** (add if ever needed as fallback).
- **Premium purchase flow** (R-076 — remains deferred).
- **Account deletion** (GDPR — separate pre-production slice).
- **Leaderboard** (separate phase; requires user records + sync).
- **MFA / 2FA** (Clerk supports; not needed for single-admin case).
- **Display name editing** (Clerk shows Google display name; editing deferred).
- **Profile page / settings** (nothing to configure this phase).
- **Rate limiting the sign-in flow** (Clerk handles OAuth throttling; our admin routes throttled by API Gateway already).

### Non-Goals

- Self-owned auth. Decided against; Clerk handles identity, sessions, OAuth, MFA.
- JWT management on our side. Clerk's session model is opaque to us; we call their SDK to verify.
- Multi-tenant admin (organizations, teams). Single admin account (you + maybe 1-2 trusted).

## Dependencies

- **GCP account** for the OAuth app (free tier).
- **Clerk account** (free tier).
- **Clerk Go SDK** (`github.com/clerk/clerk-sdk-go/v2`).
- **Clerk React SDK** (`@clerk/clerk-react`).
- No changes to Terraform (Clerk is external, no AWS resources).

## Risks

| Risk | Mitigation |
|------|------------|
| Clerk changes free-tier terms or pricing | Exportable data; migration is a 1-2 slice job to self-owned or another provider. |
| Clerk outage takes down admin routes | Admin routes are low-traffic; admin tasks can wait. Public routes unaffected. |
| GCP OAuth app verification (Google may flag an un-verified app) | Start with test users in the GCP console; publish the app once ready. Sub-100 user threshold before verification is required. |
| Signed-in-non-admin user tries to call admin API directly | Backend middleware is the source of truth; frontend hiding is cosmetic. 403 returns cleanly. |
| Clerk webhook or session validation fails | Middleware fails closed (401/403, not 500). Health check endpoint stays unauthenticated so monitoring continues. |
| Admin role accidentally revoked mid-session | Clerk's session includes role snapshot. Worst case: admin continues to act as admin until session refresh (typically within minutes). Acceptable for single-admin use. |

## Migration

- **No database schema changes.** DynamoDB is untouched.
- **No frontend state migration.** IndexedDB stays as-is for anonymous play.
- **Clerk dashboard step:** after deploy, manually set `publicMetadata.role = 'admin'` for the initial admin account (the project owner's Google account).
- **Rollback:** if something goes wrong, revert the middleware wiring. Clerk account can be deleted; users reset to anonymous. Puzzle pool unaffected.
