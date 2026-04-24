# Handover — Phase 6 Admin Auth (design complete)

Dropping this note so the next session can pick up cleanly. Delete when implementation starts.

## Where we are

Phase 5 shipped and merged to main (PRs #56, #57 merged; PR #58 retro merged).

Design phase for Phase 6 is complete. The four OpenSpec artifacts + the design-grill summary are committed on branch **`design/phase-6-admin-auth`** (two commits: `180a974` + `84debb0`), pushed to origin. **No PR opened yet.**

## What's on the branch

```
openspec/changes/phase-6-admin-auth/
├── design-grill-summary.md    — decision record from the grill
├── proposal.md                — What/Why, 10 AC, scope, risks, migration
├── design.md                  — architecture, Clerk config, middleware, testing, deployment, security
├── specs/
│   ├── auth-surface.md        — AS-01..AS-10 public auth contract
│   └── backend-middleware.md  — BM-01..BM-10 server contract
└── tasks.md                   — four slices (R-089, R-08A, R-08B, R-08C)
```

ROADMAP.md was restructured in the same commit: Phase 6 = admin auth, Phase 7 = Verdict, Phase 7b = generator deferrals, Phases 8/9/10/11+ renumbered. KI-009 marked in-flight with pointer to this phase. R-075 superseded.

## User's locked decisions (for quick resume without re-reading the grill summary)

- **Provider: Google OAuth via Clerk** (hosted). Not Cognito, not Apple, not magic link, not self-built.
- **Roles: flat enum `USER | ADMIN`** in Clerk `publicMetadata.role`. `isPremium` separate attribute, deferred.
- **Scope: admin-route gating only.** No DynamoDB user records, no IndexedDB sync. Closes KI-009.
- **Cost model:** free tier now; flip new signups to premium-gated at ~few hundred active users; grandfather existing free accounts with `isPremium = true` + `isEarlyAdopter = true`.
- **Session:** Clerk SDK handles it. httpOnly cookies. Backend uses Clerk Go SDK to verify.
- **Middleware:** `RequireAuth` + `RequireAdmin` chi middleware on `/api/admin/*`. All other routes unchanged.
- **Frontend:** sign-in button in header (always visible). Avatar menu when signed in. Admin link in menu only for admins (hidden, not disabled, for non-admins). Admin route has three rendered states: anonymous / non-admin / admin.
- **Glossary:** retire FREE; add USER; redefine ADMIN; PREMIUM deferred.

## Next steps (in order)

1. **Review the design artifacts** on the other laptop. If changes needed, edit and amend/add commits on this branch before opening a PR.
2. **Open PR against `main`** when satisfied. Title suggestion: `design: Phase 6 admin auth — OpenSpec artifacts`. Body should reference the design-grill summary and list the four slices from tasks.md.
3. **Decide merge strategy.** Two patterns in the codebase's history:
   - **Merge design first, then implement.** Phase 5's design landed as a PR before R-062 scaffolding began. This makes the design the stable reference during implementation.
   - **Keep design on the branch; build R-089 on top; merge together.** Less common; makes for a fatter first PR.
   Recommendation: merge design first. Implementation slices then stack cleanly on main with a clean tasks.md tracking status.
4. **Start R-089** (devops-engineer slice). Requires manual setup in:
   - Clerk dashboard (create app, configure Google OAuth)
   - GCP console (OAuth 2.0 Client ID)
   - Then Terraform changes for SSM params + CloudFront cookie forwarding + IAM.
   See `tasks.md` R-089 for the full list.
5. **R-08A + R-08B can ship in parallel** once R-089 merges (SSM env vars available).
6. **R-08C integrates** and closes KI-009.

## Open questions flagged in `design.md` §12

Worth verifying during R-089:

- **Clerk free-tier behavior at 10k MAU.** Is it a hard cap, soft warning, or silent auto-charge? This matters for the "flip trigger" monitoring plan. Check Clerk's billing page before integration.
- **Clerk session cookie name** traversing CloudFront. The Clerk Go SDK abstracts it, but verify it reaches the Lambda in practice. Likely `__session`.
- **CloudFront cookie forwarding** on `/api/*`. Currently forwards only the `Authorization` header (per KI-009's interim mitigation). R-089 adds cookie forwarding; `terraform plan` should show the change explicitly.

## Things NOT done in this phase (recap from design)

Intentionally out of scope so the implementation doesn't sprawl:

- DynamoDB user records (`backend/internal/repository/user.go` should NOT exist after this phase).
- IndexedDB → server state sync.
- Premium purchase flow (R-076).
- Apple Sign-In / magic link.
- Account deletion (separate pre-production slice).
- Leaderboard (separate phase).
- MFA (Clerk supports it; not needed at single-admin scale).

If a review finding suggests adding any of these, push back — they're deferred.

## Environment

- Current branch: `design/phase-6-admin-auth`
- Base: `main` at `fa59089` (Phase 5 merge) + `7a232d8` (retro merge)
- Latest commit on this branch: `84debb0`
- All tests green in pre-push. No implementation code yet; nothing to break.

## Resume on the other laptop

```bash
cd <repo>
git fetch origin
git checkout design/phase-6-admin-auth
git log --oneline -3   # sanity check: two design commits on top of main
```

Read `proposal.md` → `design.md` → `tasks.md` in that order. `specs/*` are the detailed contracts for TDD. `design-grill-summary.md` is the decision record if you want the "why" for a specific choice.

Delete this file once the design PR opens (it's a transient handover, not part of the spec).
