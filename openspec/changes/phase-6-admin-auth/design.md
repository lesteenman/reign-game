# Phase 6 Design: Admin Authentication via Clerk

## 1. Architecture Overview

```
   Browser                     Clerk             GCP                Our API
  ─────────                   ──────           ──────             ─────────
     │                           │                │                    │
     │  1. Click "Sign in"       │                │                    │
     ├──────────────────────────▶│                │                    │
     │  2. Clerk redirects ▶     │    OAuth w/    │                    │
     │     to Google             ├───────────────▶│                    │
     │                           │  PKCE, state   │                    │
     │     3. Google login UI    │                │                    │
     │◀────────────────────────────────────────────┤                   │
     │  4. Redirect back ▶       │                │                    │
     ├──────────────────────────▶│                │                    │
     │                           │  ID token      │                    │
     │  5. Clerk session cookie  │                │                    │
     │  set (httpOnly, SameSite) │                │                    │
     │◀──────────────────────────┤                │                    │
     │                           │                │                    │
     │  6. API call w/ cookie    │                │                    │
     ├──────────────────────────────────────────────────────────────▶ │
     │                           │   verify session (SDK, cached JWKS) │
     │                           │◀───────────────────────────────────┤
     │                           │   session valid + publicMetadata    │
     │                           │───────────────────────────────────▶│
     │  7. 200 / 401 / 403       │                                     │
     │◀──────────────────────────────────────────────────────────────┤
```

Browser talks to Clerk directly for authentication. Our backend only verifies sessions via Clerk's SDK (cached JWKS). No OAuth code paths in our backend; no key management; no token handling beyond cookie read.

## 2. Package Layout

### Backend

```
backend/
├── internal/
│   ├── auth/                    NEW
│   │   ├── middleware.go        RequireAuth + RequireAdmin chi middleware
│   │   ├── middleware_test.go
│   │   └── doc.go
│   └── handler/
│       ├── admin_config.go      CHANGED — wrapped in RequireAdmin
│       ├── admin_pool.go        CHANGED — wrapped in RequireAdmin
│       ├── replenish.go         CHANGED — wrapped in RequireAdmin
│       └── ...
└── cmd/api/main.go              CHANGED — Clerk SDK init, route group mounts
```

No new `repository/user.go`. No schema changes.

### Frontend

```
frontend/src/
├── components/
│   ├── auth/                    NEW
│   │   ├── SignInButton.tsx     Clerk SDK wrapper
│   │   ├── UserMenu.tsx         Avatar + dropdown with Admin link (role-gated) + Sign out
│   │   └── ProtectedRoute.tsx   Gates /admin based on role
│   └── common/
│       └── PageShell.tsx        CHANGED — sign-in button / user menu in header
├── pages/
│   ├── AdminPage.tsx            CHANGED — wrapped in <ProtectedRoute role="admin">
│   └── AdminLandingPage.tsx     NEW — "Sign in" / "No admin access" landing
├── hooks/
│   └── useAuth.ts               NEW — thin wrapper around Clerk's useUser + useAuth
└── main.tsx                     CHANGED — <ClerkProvider> wraps <App>
```

## 3. Clerk Configuration

### Dashboard setup

- **Application name:** Reign
- **Sign-in methods:** Google OAuth only. All others (email, password, magic link, Apple, GitHub, etc.) disabled.
- **User attributes:**
  - `publicMetadata.role` — `"user"` (default) or `"admin"` (set manually for the project owner's account).
  - `publicMetadata.isPremium` — not set this phase; reserved for flip.
  - `publicMetadata.isEarlyAdopter` — not set this phase.
- **Redirect URLs:**
  - Dev: `http://localhost:5180`, `http://localhost:5183` (for e2e stack)
  - Prod: the CloudFront distribution URL (once registered)
- **Webhook endpoints:** none this phase.

### Environment variables

| Name | Frontend / Backend | Purpose |
|------|--------------------|---------|
| `VITE_CLERK_PUBLISHABLE_KEY` | Frontend | Clerk SDK init |
| `CLERK_SECRET_KEY` | Backend | Clerk SDK server-side verification |

Secrets stored in AWS SSM Parameter Store (Terraform adds them in the `devops-engineer` slice). Dev values in `.env.local` (gitignored).

## 4. Backend Middleware

### `RequireAuth`

Reads the Clerk session cookie from the request, verifies it via `clerk.VerifyToken`. On success, attaches a `*clerk.User` to the request context. On failure, returns 401 with the standard error JSON.

The snippet below is **illustrative** — the real clerk-sdk-go v2 surface (clients, option structs, context-plumbing helpers) shifts between minor versions. The R-08A implementer reads the current SDK docs and adapts. The invariants that must hold regardless of SDK shape are the ones spelled out in `specs/backend-middleware.md` BM-01..BM-09.

Also: `OPTIONS` requests must bypass this middleware (per BM-08). Either wire CORS middleware before auth on the admin group, or add an early `if r.Method == http.MethodOptions` check at the top of `RequireAuth`. The snippet below shows the happy path only.

```go
// backend/internal/auth/middleware.go
package auth

import (
    "context"
    "net/http"

    "github.com/clerk/clerk-sdk-go/v2"
)

type userContextKey struct{}

// UserFromContext retrieves the authenticated Clerk user from the request
// context. Panics if RequireAuth did not run on this request — that's a
// programmer error, never a runtime concern, because middleware order is
// deterministic.
func UserFromContext(ctx context.Context) *clerk.User {
    user, ok := ctx.Value(userContextKey{}).(*clerk.User)
    if !ok {
        panic("auth.UserFromContext: RequireAuth middleware did not run")
    }
    return user
}

// RequireAuth verifies the Clerk session and attaches the user to the
// request context. Rejects with 401 if the session is absent, expired, or
// invalid.
func RequireAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sessionToken := clerk.SessionTokenFromRequest(r)
        if sessionToken == "" {
            writeUnauth(w, "authentication required")
            return
        }
        claims, err := clerk.VerifyToken(r.Context(), sessionToken)
        if err != nil {
            writeUnauth(w, "invalid session")
            return
        }
        user, err := clerk.UserGet(r.Context(), claims.Subject)
        if err != nil {
            writeUnauth(w, "user not found")
            return
        }
        ctx := context.WithValue(r.Context(), userContextKey{}, user)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### `RequireAdmin`

Runs after `RequireAuth`. Reads `publicMetadata.role` from the user attached to context. Returns 403 if not `"admin"`.

```go
// RequireAdmin requires the authenticated user to have role=admin in
// their Clerk publicMetadata. Must run after RequireAuth.
func RequireAdmin(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        user := UserFromContext(r.Context())
        role, _ := user.PublicMetadata["role"].(string)
        if role != "admin" {
            writeForbidden(w, "admin role required")
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### Route wiring

```go
// backend/cmd/api/main.go (excerpt)
r := chi.NewRouter()

// Public routes
r.Get("/api/health", handler.HealthHandler())
r.Get("/api/config/modes", handler.ConfigModesHandler(repo))
r.Get("/api/puzzles/next", handler.ServeHandler(repo))
r.Put("/api/puzzles/{id}/status", handler.StatusHandler(repo))
r.Get("/api/puzzles/generate", handler.GenerateHandler(...))

// Admin routes
r.Route("/api/admin", func(r chi.Router) {
    r.Use(auth.RequireAuth, auth.RequireAdmin)
    r.Get("/pool", handler.AdminPoolHandler(repo))
    r.Put("/config/{size}/{mode}", handler.UpdateConfigHandler(repo))
    r.Post("/config", handler.CreateConfigHandler(repo))
    r.Post("/replenish", handler.ReplenishHandler(repo, publisher))
})
```

Before: all admin routes registered flat at root level, no middleware. After: grouped under `/api/admin` with `RequireAuth, RequireAdmin` applied by chi's middleware chain. Handler functions themselves are unchanged.

## 5. Frontend Integration

### ClerkProvider

```tsx
// frontend/src/main.tsx
import { ClerkProvider } from '@clerk/clerk-react'

const publishableKey = import.meta.env.VITE_CLERK_PUBLISHABLE_KEY

root.render(
  <ClerkProvider publishableKey={publishableKey}>
    <App />
  </ClerkProvider>,
)
```

### Sign-in button + user menu

```tsx
// frontend/src/components/common/PageShell.tsx (excerpt)
import { SignedIn, SignedOut, SignInButton, UserButton } from '@clerk/clerk-react'

<header>
  <h1>Reign</h1>
  <nav>
    <SignedOut>
      <SignInButton mode="modal" />
    </SignedOut>
    <SignedIn>
      <UserMenu />  {/* our component: UserButton + conditional Admin link */}
    </SignedIn>
  </nav>
</header>
```

`<UserMenu>` wraps Clerk's `<UserButton>` and conditionally renders the Admin link based on `user.publicMetadata.role === 'admin'`.

### Protected admin route

```tsx
// frontend/src/App.tsx
<Route path="/admin" element={<ProtectedAdminRoute />} />
```

`<ProtectedAdminRoute>` reads `useUser()` from Clerk:
- `!isLoaded` → loading spinner.
- `!isSignedIn` → `<AdminLandingPage state="anonymous" />`.
- `user.publicMetadata.role !== 'admin'` → `<AdminLandingPage state="forbidden" />`.
- Otherwise → `<AdminPage />`.

### Admin landing page

Single component with two states, both simple:

```tsx
// frontend/src/pages/AdminLandingPage.tsx
type State = 'anonymous' | 'forbidden'

export function AdminLandingPage({ state }: { state: State }) {
  if (state === 'anonymous') {
    return (
      <PageShell>
        <h1>Admin Access</h1>
        <p>Sign in to access the admin panel.</p>
        <SignInButton mode="modal" />
      </PageShell>
    )
  }
  return (
    <PageShell>
      <h1>No Admin Access</h1>
      <p>This account doesn't have admin access.</p>
      <SignOutButton />
    </PageShell>
  )
}
```

## 6. Data Flow Examples

### Anonymous user hits `/api/admin/pool`

1. Request arrives at Lambda with no Clerk session cookie.
2. `RequireAuth` middleware: `clerk.SessionTokenFromRequest(r)` returns empty.
3. Middleware writes 401 `authentication required`.
4. Handler never runs.

### Signed-in non-admin hits `/api/admin/pool`

1. Request arrives with valid Clerk session cookie.
2. `RequireAuth`: session valid. Clerk SDK returns `user` with `publicMetadata.role = "user"`. Attached to context.
3. `RequireAdmin`: reads role from context. Not `"admin"`. Writes 403 `admin role required`.
4. Handler never runs.

### Admin hits `/api/admin/pool`

1. Request arrives with valid session cookie.
2. `RequireAuth`: session valid, user attached. Role = `"admin"`.
3. `RequireAdmin`: role matches. Passes through.
4. Handler runs normally; returns pool state.

### Admin demoted mid-session

- Clerk session lasts up to its configured TTL (Clerk default: 7 days sliding, configurable).
- At the next session refresh (triggered by Clerk SDK heartbeat or page reload), the session claims update. Role demotion propagates within minutes.
- For instant revoke: Clerk dashboard "Revoke session" button kills the session immediately. Acceptable for single-admin case.

## 7. Testing Strategy

### Unit tests (backend)

- `backend/internal/auth/middleware_test.go`:
  - `RequireAuth`: empty cookie → 401; valid cookie → next called with user in context; expired session → 401; Clerk SDK error → 401.
  - `RequireAdmin`: role=admin → next called; role=user → 403; no role → 403; called without RequireAuth → panic (programmer error).
  - Mock Clerk SDK via an interface adapter so tests don't hit real Clerk.

### Integration tests (backend)

- `backend/internal/handler/admin_config_test.go`: rewrite existing test setup to include middleware chain. Test each admin route with three fixture sessions: anonymous (no cookie), user (role=user), admin (role=admin). Assert status codes per AC-6.
- `backend/internal/handler/admin_pool_test.go`: same.
- `backend/internal/handler/replenish_test.go`: same.
- Use a Clerk test mode fixture (their SDK supports test JWT tokens that bypass signature verification).

### Frontend tests (Vitest)

- `frontend/src/components/auth/UserMenu.test.tsx`: signed-out → shows nothing / sign-in; signed-in + user role → menu without Admin; signed-in + admin role → menu with Admin.
- `frontend/src/components/auth/ProtectedRoute.test.tsx`: each state (loading, anonymous, non-admin, admin) renders the expected branch.
- Mock `@clerk/clerk-react` in tests (vendor provides test helpers).

### E2E tests (Playwright)

- Integration suite (mocked): sign-in button visible when signed out; user menu when signed in; admin link only for admin role.
- E2E suite (real backend on :5182 + e2e Clerk tenant): sign in as admin test account, confirm `/admin` loads; sign in as user test account, confirm `/admin` shows "no admin access."

## 8. Deployment

### Terraform

Two new SSM parameters under `/reign/prod/`:
- `clerk-publishable-key` (not sensitive, but co-located).
- `clerk-secret-key` (sensitive, `SecureString`).

Lambda IAM policy updated to allow `ssm:GetParameter` on both. Frontend build pulls `VITE_CLERK_PUBLISHABLE_KEY` from GitHub Actions secrets at CI time.

### Rollout

1. Set up Clerk account + GCP OAuth app (manual, ~15 min).
2. Deploy backend first: middleware wired, routes return 401 to anonymous. Admin routes become unusable from the current admin UI until the frontend ships.
3. Deploy frontend: sign-in flow works; admin UI accessible after admin role granted via Clerk dashboard.
4. Grant admin role to project owner's Google account in Clerk dashboard.
5. Verify: `curl /api/admin/pool` → 401; sign in via frontend → 200.

### Backward compatibility

Not applicable. Admin UI was previously unauthenticated; the change is purely additive from a user-experience perspective (new sign-in button for previously-anonymous admins).

### CloudFront interaction

CloudFront already forwards the `Authorization` header (per KI-009's interim mitigation). Cookie-based auth requires CloudFront to forward the `Cookie` header on `/api/*`. Verify in Terraform; add to the CloudFront behavior config for `/api/*` if missing.

## 9. Observability

- Backend logs: `auth: RequireAuth rejected (reason=<reason>) <path>` — no PII, no session tokens.
- Backend logs: `auth: RequireAdmin rejected (userId=<clerk-sub>, path=<path>)` — Clerk user ID is a random string, not PII.
- Frontend: no logging beyond Clerk SDK's own.
- No new metrics this phase. If auth-failure rate becomes a concern, add a CloudWatch metric filter on the `auth: Require*` log pattern.

## 10. Security

Clerk handles the heavy lifting. Our responsibilities are narrow:

- **Never log session tokens or full Clerk user objects.** Log only the `userId` (Clerk sub) and the path.
- **Middleware fails closed.** Every error path returns 401 or 403, never 500 with details.
- **Role check is by string equality.** `user.PublicMetadata["role"].(string) == "admin"`. No regex, no case-insensitive compare, no "admin-ish" fallback.
- **Frontend hiding is cosmetic.** Backend is the source of truth for authorization.

### Out of Scope

- CSRF protection. Clerk's session cookie uses `SameSite=Lax`, which blocks most CSRF. Our admin API only accepts JSON bodies (not form-encoded), blunting the rest. Explicit CSRF tokens are not needed at our surface.
- Rate limiting admin routes. Admin is low-traffic; Clerk throttles the OAuth flow itself.

## 11. Migration to Self-Owned Auth (if ever needed)

If we later decide to leave Clerk:

1. Export users + `publicMetadata` via Clerk's API (JSON).
2. Stand up self-owned auth per the design grill's self-build path (OAuth callback + JWT + refresh).
3. Import user data; assign new internal user IDs; map old Clerk subs to new IDs.
4. Issue migration tokens that accept old Clerk sessions for a grace period.
5. Deprecate Clerk.

This is a several-slice project. Not something we'll do casually. Capturing the path here so we know it exists.

## 12. Open Questions

None that block implementation. Things worth confirming during slice execution:

- **Clerk free tier behavior above 10k MAU.** Document whether it's a hard cap, soft cap with warning, or auto-charge. Must be verified before any production launch.
- **Clerk session cookie name.** Documented in Clerk's SDK; the Go SDK's `SessionTokenFromRequest` abstracts this, but verify during integration that cookies traverse CloudFront correctly.
- **CloudFront cookie forwarding.** Verify the `/api/*` behavior forwards the Clerk session cookie (likely `__session` or similar). Add to Terraform if missing.

## 13. Roadmap Effects

Already landed in the design commit (`ROADMAP.md` restructured alongside these artifacts):

- **Phase 6** header becomes "Admin Authentication via Clerk" (was "Verdict System"). Slices: see `tasks.md`.
- **Phase 7** becomes "Verdict System" (was Phase 6). Slice IDs `R-081`, `R-082` retain their numbers; only the phase header moves.
- **Phase 7b** becomes "Generator Quality Deferrals" (was Phase 6b — kept as a sub-phase slot so it stays attached to the Verdict phase that unblocks it).
- **Phase 8** becomes "Puzzle Replay" (was Phase 7). Slices `R-085`, `R-086` unchanged.
- **Phase 9** becomes "Puzzle Analysis Agent" (was Phase 8). Slices `R-087`, `R-088` unchanged.
- **Phase 10** becomes "Difficulty Rating" (was Phase 9).
- **Phase 11+** becomes "Future" (was Phase 10+).

New slice IDs for Phase 6 Auth: `R-089`, `R-08A`, `R-08B`, `R-08C` (four slices in `tasks.md`; IDs allocated to avoid collisions with R-080..R-088 already claimed).
