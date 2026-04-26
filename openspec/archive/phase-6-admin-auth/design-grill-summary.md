# Design Grill Summary: Phase 6 — Admin Authentication via Clerk

## Final Design

Pull authentication forward from Phase 10+ (R-075) to close KI-009 now. Ship admin-route gating via Clerk (Google OAuth only). Anyone can sign in; admin role lives in Clerk `publicMetadata.role`; backend middleware verifies Clerk sessions on `/api/admin/*` and checks the role claim. No DynamoDB user records, no local-to-server sync, no premium flow. A later phase flips new signups behind premium when active users approach Clerk's free-tier limit; early adopters are grandfathered with free premium.

## Decisions

### Authentication approach

- **Provider: Google OAuth via Clerk** (hosted auth service). Self-built OAuth was viable but adds 1–2 slices of security-critical code (PKCE, state, refresh-token rotation with reuse detection, JWKS caching). Clerk's free tier (~10k MAU) buys multi-year runway at realistic growth.
- **Not Cognito** (pre-existing project decision). Clerk fits the "self-owned enough" spirit because user data and metadata are exportable.
- **Not Apple Sign-In yet.** Apple requires a $99/yr developer account we're not ready to commit to. Defer until iOS App Store submission is imminent.
- **Not magic link yet.** Adds a third auth flow to maintain. Google-only covers the PWA case.

### Role model

- **Two roles: USER, ADMIN.** Flat enum, mutually exclusive. Not a hierarchy.
- **`isPremium` is a separate attribute**, not a role. Premium is an entitlement layered on top, addable later without breaking the role model.
- **Role stored in Clerk `publicMetadata.role`.** Assigned via Clerk dashboard. No admin UI to build.
- **Glossary updates:**
  - Retire **FREE** (it described an anonymous *state*, not a role).
  - Define **USER** (signed-in via Clerk).
  - Redefine **ADMIN** (signed-in with `role=admin` claim in Clerk metadata).
  - **PREMIUM** as a glossary term — deferred until the flip phase.

### Session architecture

- **Clerk SDK handles sessions.** httpOnly cookies by default; refresh token rotation handled by Clerk; PKCE and state parameter handled by Clerk.
- **Backend middleware:**
  - `RequireAuth` — verifies Clerk session, rejects with 401 if absent or invalid.
  - `RequireAdmin` — `RequireAuth` plus `publicMetadata.role === 'admin'`, rejects with 403 otherwise.
- **Applied to `/api/admin/*`.** Other routes (`/api/health`, `/api/config/modes`, `/api/puzzles/*`) stay public.

### Cost model

- **Clerk free tier from day 1.**
- **Flip trigger: ~few hundred active users** ("success" signal).
  - At flip: new signups require premium purchase (blocked in the sign-up flow).
  - Existing free users: `publicMetadata.isPremium = true` + `isEarlyAdopter = true`. When premium features ship, existing accounts have them automatically.
- **Why this eliminates cost risk:** post-flip, MAU is bounded by paying users. At a few thousand paying users with an established premium flow, revenue dwarfs the Clerk subscription; at that point free-tier cost protection is irrelevant.
- **Monitoring:** Clerk dashboard shows MAU. Milestones: surface awareness at 200 / 400 / 800 MAU. No automation.

### Frontend surface

- **Sign-in button** in PageShell header, visible to all anonymous users.
- **Signed-in state:** avatar + display name replace the sign-in button. Opens a dropdown menu.
- **Menu items:**
  - "Admin" — only if `role === 'admin'`.
  - "Sign out" — always.
- **Admin route `/admin`:**
  - Anonymous → "Sign in as admin" landing.
  - Signed-in non-admin → "This account doesn't have admin access."
  - Admin → normal admin UI.
- **Rule: UI elements gated by role are hidden from users without that role.** Backend still enforces — frontend hiding is cosmetic.

### Backend middleware surface

- Middleware applies via chi router groups: `r.Route("/admin", func(r chi.Router) { r.Use(RequireAuth, RequireAdmin); ... })`.
- Clerk's Go server SDK verifies session on each request against Clerk's JWKS (cached per Lambda container).
- Role is read from session's `publicMetadata.role` claim — no database lookup on the hot path.

## Deferred

- **DynamoDB user records.** Not needed to gate admin routes. Add when the first per-user server state ships (leaderboard, stats sync, saved preferences).
- **Local IndexedDB → server sync on first sign-in.** Not needed this phase. Pairs with the first feature that requires server-side user state.
- **Premium purchase flow (R-076).** Remains deferred until the flip trigger fires.
- **Apple Sign-In.** Add when iOS App Store submission becomes imminent.
- **Magic link.** Add only if we ever need a fallback for users without Google accounts.
- **Account deletion.** Separate slice before production launch; GDPR requires it.
- **Leaderboard (previously tied to auth).** Separate phase; depends on user records + sync.

## Constraints & Assumptions

- **Clerk's free tier is 10k MAU with no silent auto-charge on overage.** Verify before integration; if the free-tier behavior is a hard cap or requires explicit upgrade, the cost model holds. If Clerk auto-charges, the flip-trigger monitoring becomes load-bearing.
- **Clerk's `publicMetadata` is exportable.** Migration off Clerk in the future is a JSON export + replay, not a re-registration of users.
- **Google OAuth registration is straightforward.** GCP project creation is free; OAuth app config is a one-time setup.
- **Our Go Lambda can use Clerk's server SDK.** Adds one Go dependency; session-verification cold-start cost is JWKS fetch (cached per container).
- **Admin users are few (you + maybe 1-2 trusted people).** Role assignment via Clerk dashboard is operationally fine at this scale; doesn't warrant building our own admin UI.
- **User growth to the "flip trigger" point will have visible leading indicators** (organic reach, press, shared links). No silent ramp that bypasses MAU awareness.

## Known Issues to Track

- **KI-009 closes with this phase.** The interim mitigation (threshold cap + CloudFront `Authorization` forwarding) stays in place as defense-in-depth.
- **No new KIs expected from this phase.** Clerk removes most of the self-built auth risk surface.

## Roadmap Effects

- **Phase 6 becomes Admin Authentication via Clerk.** Previously Phase 6 was Verdict System (R-081, R-082).
- **Verdict System slides to Phase 7** (was Phase 6). All downstream phases shift: Phase 6b (generator deferrals) stays in that slot; Phase 7 (Puzzle Replay) becomes Phase 8; Phase 8 (Analysis Agent) becomes Phase 9. Existing numeric slice IDs (R-081 through R-088) keep their numbers; only the phase they sit under changes.
- **Alternative:** insert as Phase 5.5 instead of claiming the Phase 6 slot. User's call on which ordering they prefer.

## Next Steps

1. Commit this summary to `openspec/changes/phase-6-admin-auth/design-grill-summary.md`.
2. User confirms phase number (6 vs 5.5) and whether Verdict shifts to Phase 7.
3. Optional: hand off to OpenSpec to generate `proposal.md`, `design.md`, `specs/`, `tasks.md` — either via `/opsx:new phase-6-admin-auth` (green-field) or by starting from this summary.
4. Glossary update: remove FREE, add USER, redefine ADMIN.
5. CLAUDE.md role table rewrite to match new model.
