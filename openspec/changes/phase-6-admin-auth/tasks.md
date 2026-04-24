# Phase 6 Tasks: Admin Authentication via Clerk

## Slice Dependency Graph

```
R-089 (Clerk + infra)               foundational
    |
    ├── R-08A (Backend middleware) ─┐
    │                               │
    └── R-08B (Frontend auth UI)  ──┴── R-08C (Glossary + docs + KI-009 close)
```

R-089 ships first (nothing depends on it landing completely — the two implementation slices can branch off in parallel once Clerk is configured and env vars are available). R-08A and R-08B can ship in parallel; R-08C integrates and closes the loop.

## Status

All Phase 6 slices are `[ ]` until completed. Per CLAUDE.md lesson 17, each slice's PR must flip its row to `[x]` as a required artifact — no post-hoc sweeps.

| ID    | Slice                                           | Layer | Status |
|-------|-------------------------------------------------|-------|--------|
| R-089 | Clerk account + GCP OAuth + Terraform SSM keys  | 0     | [ ]    |
| R-08A | Backend auth middleware + admin route wiring    | 1     | [ ]    |
| R-08B | Frontend sign-in flow + user menu + admin route | 1     | [ ]    |
| R-08C | Glossary + CLAUDE.md + ROADMAP + KI-009 close   | 2     | [ ]    |

## Tasks

### R-089: Clerk setup + GCP OAuth + infra

- **Roadmap:** R-089
- **Agent:** devops-engineer
- **OpenSpec:** `specs/auth-surface.md` (AS-01, AS-02)

**Work**

- Create Clerk account (free tier). Create "Reign" application.
- Configure Clerk: enable Google OAuth only; disable all other sign-in methods.
- Configure Clerk user attribute schema: `publicMetadata.role` (string, `"user"` default).
- Create GCP project "Reign Auth". Create OAuth 2.0 Client ID for web. Add authorized redirect URIs from Clerk dashboard.
- Copy Google Client ID + Client Secret into Clerk's Google OAuth config.
- Configure Clerk redirect URLs: `http://localhost:5180`, `http://localhost:5183`, production CloudFront URL (if known; add later if not).
- Get Clerk publishable key + secret key from Clerk dashboard.
- Terraform: add two SSM Parameter Store entries:
  - `/reign/prod/clerk-publishable-key` — `String` type, public (it's in browser bundles anyway).
  - `/reign/prod/clerk-secret-key` — `SecureString` type.
- Update Lambda IAM policy (Terraform `infra/modules/api/iam.tf`) to grant `ssm:GetParameter` on both parameter ARNs.
- Update CloudFront behavior for `/api/*` (Terraform `infra/modules/frontend/cloudfront.tf`) to forward cookies (in addition to the existing `Authorization` header forwarding). Cookie forwarding is needed so Clerk's session cookie reaches the Lambda. Verify this doesn't break existing anonymous routes.
- Frontend build: add `VITE_CLERK_PUBLISHABLE_KEY` to GitHub Actions CD workflow (pulled from SSM or set as a workflow variable).
- Document in a new file: `docs/runbooks/admin-auth-setup.md` — step-by-step for granting admin role to a new Clerk account via the Clerk dashboard.

**Gate**

- Clerk dashboard reflects Google-only sign-in.
- GCP OAuth app created and tested (manual browser sign-in completes).
- `terraform plan` shows the two new SSM params + IAM changes + CloudFront behavior update.
- `terraform apply` succeeds in dev.
- Sample Lambda invocation reads both SSM params without error (can be a one-liner test handler).

**Files touched**

- `infra/modules/api/iam.tf` (update)
- `infra/modules/api/main.tf` (SSM params)
- `infra/modules/frontend/cloudfront.tf` (cookie forwarding)
- `.github/workflows/cd.yml` (VITE_CLERK_PUBLISHABLE_KEY)
- `docs/runbooks/admin-auth-setup.md` (new)

**Dependencies:** none.

**Commit after completion.**

---

### R-08A: Backend auth middleware + admin route wiring

- **Roadmap:** R-08A
- **Agent:** backend-dev
- **OpenSpec:** `specs/backend-middleware.md` (BM-01 through BM-10)

**Work**

- Install Clerk Go SDK: `go get github.com/clerk/clerk-sdk-go/v2`.
- Create `backend/internal/auth/` package:
  - `doc.go` — package GoDoc explaining the two middleware and the context key.
  - `middleware.go` — `RequireAuth`, `RequireAdmin`, `UserFromContext`, `writeUnauth`, `writeForbidden`.
  - `middleware_test.go` — unit tests per BM-01..BM-09.
- `backend/cmd/api/main.go`:
  - Read `CLERK_SECRET_KEY` from SSM on startup (cache for container lifetime per BM-10).
  - Initialize Clerk SDK with the secret key.
  - Rewrite router: admin routes move into `r.Route("/api/admin", func(r chi.Router) { r.Use(auth.RequireAuth, auth.RequireAdmin); ... })`. Non-admin routes untouched.
- Update existing admin handler tests to supply a mock Clerk user in context (or use Clerk SDK test helpers). Follow TDD: write the middleware integration test first (expects 401 for anonymous, 403 for non-admin, 200 for admin), then implement the wiring.

**Gate**

- `go build ./...` passes.
- `go test -short ./...` green.
- `golangci-lint run` green.
- Integration test proves all four current `/api/admin/*` routes return 401 / 403 / 200 for the three session states.
- CURL test from a dev-stack backend: `curl /api/admin/pool` → 401.

**Files touched**

- `backend/internal/auth/doc.go` (new)
- `backend/internal/auth/middleware.go` (new)
- `backend/internal/auth/middleware_test.go` (new)
- `backend/cmd/api/main.go` (update — router restructure + SDK init)
- `backend/internal/handler/admin_config_test.go` (update — middleware-aware)
- `backend/internal/handler/admin_pool_test.go` (update — middleware-aware)
- `backend/internal/handler/replenish_test.go` (update — middleware-aware)
- `backend/go.mod` / `go.sum` (Clerk SDK)

**Dependencies:** R-089 (SSM params + Clerk account).

**Commit after completion.**

---

### R-08B: Frontend sign-in flow + user menu + admin route

- **Roadmap:** R-08B
- **Agent:** frontend-dev
- **OpenSpec:** `specs/auth-surface.md` (AS-03 through AS-10)

**Work**

- Install Clerk React SDK: `npm install @clerk/clerk-react`.
- `frontend/src/main.tsx`: wrap `<App>` in `<ClerkProvider publishableKey={...}>`. Publishable key from `import.meta.env.VITE_CLERK_PUBLISHABLE_KEY`.
- Create `frontend/src/components/auth/` components:
  - `SignInButton.tsx` — thin wrapper around Clerk's `<SignInButton mode="modal">`.
  - `UserMenu.tsx` — Clerk's `<UserButton>` with a conditional `<Link to="/admin">` child that renders only when `user.publicMetadata.role === 'admin'`.
  - `ProtectedAdminRoute.tsx` — routes `/admin` to the AdminPage or the landing page based on auth/role state.
- Create `frontend/src/pages/AdminLandingPage.tsx` — two-state component (anonymous vs forbidden).
- Update `frontend/src/components/common/PageShell.tsx` header:
  - `<SignedOut><SignInButton /></SignedOut>`
  - `<SignedIn><UserMenu /></SignedIn>`
- Update `frontend/src/App.tsx` routes: wrap `/admin` route in `<ProtectedAdminRoute>`.
- Remove any existing frontend references to the old admin-link-in-header pattern (if any) — admin link lives inside the user menu now.
- Tests:
  - `UserMenu.test.tsx` — three states (signed out nothing rendered, user no admin link, admin yes admin link).
  - `ProtectedAdminRoute.test.tsx` — four states (loading, anonymous, non-admin, admin).
  - `AdminLandingPage.test.tsx` — anonymous state + forbidden state.
- Mock `@clerk/clerk-react` using its provided test helpers.

**Gate**

- `npx tsc -b` passes.
- `npm test` green (new tests + existing tests still pass).
- Manual: `task dev:up` → sign in via the header button → avatar appears → clicking admin link (as non-admin) shows forbidden landing.

**Files touched**

- `frontend/package.json` / `package-lock.json`
- `frontend/src/main.tsx`
- `frontend/src/components/auth/SignInButton.tsx` (new)
- `frontend/src/components/auth/UserMenu.tsx` (new)
- `frontend/src/components/auth/ProtectedAdminRoute.tsx` (new)
- `frontend/src/components/auth/SignInButton.test.tsx` (new)
- `frontend/src/components/auth/UserMenu.test.tsx` (new)
- `frontend/src/components/auth/ProtectedAdminRoute.test.tsx` (new)
- `frontend/src/components/common/PageShell.tsx` (update)
- `frontend/src/pages/AdminLandingPage.tsx` (new)
- `frontend/src/pages/AdminLandingPage.test.tsx` (new)
- `frontend/src/App.tsx` (update)

**Dependencies:** R-089 (`VITE_CLERK_PUBLISHABLE_KEY` available).

**Commit after completion.**

---

### R-08C: Glossary + CLAUDE.md + ROADMAP + KI-009 close

- **Roadmap:** R-08C
- **Agent:** general-purpose (docs work)
- **OpenSpec:** whole spec (integration)

**Work**

- `GLOSSARY.md`:
  - Remove FREE entry.
  - Add USER entry: "User. Signed-in via Clerk (Google OAuth). Default role; no additional features beyond anonymous play until later phases add leaderboard, stats sync, etc."
  - Redefine ADMIN: "Admin. Signed-in with `publicMetadata.role === 'admin'` claim in Clerk. Access to `/admin` UI and `/api/admin/*` routes. Role assigned manually via Clerk dashboard."
  - Add PREMIUM placeholder: "Premium. Term reserved for flip phase (future). Not used this phase."
- `CLAUDE.md` — rewrite the Roles section table:
  - Remove FREE / PREMIUM / ADMIN table.
  - Add Anonymous / User / Admin table.
- `ROADMAP.md`: the phase restructure (Phase 6 = Admin Auth, Phase 7 = Verdict, Phase 7b = Generator deferrals, Phase 8 = Replay, Phase 9 = Analysis, Phase 10 = Difficulty, Phase 11+ = Future) already landed in the design commits. This slice only needs to:
  - Strikethrough-close KI-009: swap the "in flight" row for `~~Critical~~ Fixed by Phase 6 admin auth (R-089..R-08C).`
  - Flip the four R-089..R-08C checkboxes in the Phase 6 block from `[ ]` to `[x]`.
  - Sanity-grep for leftover "pre-production" or "in flight" notes on KI-009 and clear them.
- `PROJECT_STRUCTURE.md`:
  - Add `backend/internal/auth/` to the backend tree.
  - Add `frontend/src/components/auth/` to the frontend tree.
  - Add `docs/runbooks/admin-auth-setup.md` to the docs tree.
- E2E smoke: manual check that sign-in works end-to-end on a deployed environment, and that a non-admin sign-in sees the forbidden landing on `/admin`.
- Flip all four status rows in this `tasks.md` to `[x]` (per CLAUDE.md lesson 17). Prior slices R-089, R-08A, R-08B should have already flipped their own rows in their PRs; this slice flips R-08C's row and confirms the other three are already `[x]`.

**Gate**

- Grep sweep: no reference to "FREE role" remains in the repo (other than historical archive docs in `openspec/changes/phase-5-*/`).
- Grep sweep: KI-009 in `ROADMAP.md` is strikethrough.
- `tasks.md` status table all `[x]`.
- E2E happy path works on deployed stack.

**Files touched**

- `GLOSSARY.md` (update)
- `CLAUDE.md` (update)
- `ROADMAP.md` (update — KI-009 strikethrough-close + flip R-089..R-08C checkboxes; phase renumbering already landed in the design commits)
- `PROJECT_STRUCTURE.md` (update)
- `openspec/changes/phase-6-admin-auth/tasks.md` (update — flip rows)

**Dependencies:** R-089, R-08A, R-08B all merged.

**Commit after completion. Then archive this OpenSpec change via `/opsx:archive`.**

## Verification Checklist (Phase Close)

Before promoting this epic to main:

- [ ] All four current `/api/admin/*` routes return 401 for anonymous, 403 for user-role, 200 for admin. Integration test proves it.
- [ ] Sign-in button visible to anonymous users in header; avatar+menu for signed-in; admin link only for admin role.
- [ ] `/admin` route renders correct landing for each of the three session states.
- [ ] `tasks.md` status table all `[x]`.
- [ ] KI-009 strikethrough-closed in `ROADMAP.md`.
- [ ] CLAUDE.md roles table matches new model (Anonymous/User/Admin).
- [ ] `GLOSSARY.md` retires FREE, adds USER, redefines ADMIN.
- [ ] No `backend/internal/repository/user.go`. No user records in DynamoDB. (This phase is admin-gating only.)
- [ ] Clerk free tier MAU alert: document the point at which flip monitoring begins (200 MAU).
- [ ] Follow 4-axis review-local + security-review before epic→main merge (per CLAUDE.md lesson 13).
