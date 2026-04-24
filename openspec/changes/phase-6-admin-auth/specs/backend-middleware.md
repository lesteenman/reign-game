# Spec: Backend Middleware

Server-side auth enforcement contract.

## BM-01: `RequireAuth` middleware gates the handler on session presence + validity

**Rule.** `RequireAuth` reads the Clerk session cookie from the request, verifies it via `clerk.VerifyToken`. On success, attaches the Clerk user to the request context via a package-private key. On failure, writes `401 Unauthorized` with a standard error JSON body and returns without calling the next handler.

**Failure modes mapped to response:**

| Condition | Response |
|---|---|
| No session cookie present | 401 `{"error":"unauthorized","message":"authentication required"}` |
| Session cookie present but invalid signature | 401 `{"error":"unauthorized","message":"invalid session"}` |
| Session cookie valid but expired | 401 `{"error":"unauthorized","message":"session expired"}` |
| Clerk API unreachable (network failure) | 401 (fail closed), log WARN with the underlying error |
| User record not found in Clerk | 401 `{"error":"unauthorized","message":"user not found"}` |

**Value.** Single source of truth for "is this caller authenticated." Every gated route uses the same middleware, so every gated route has the same 401 behavior. No handler-internal auth checks.

**Verification.** Unit test per failure mode: construct a request with the specified condition, run the middleware, assert response status and body. Handler function must not have been invoked (mock `http.Handler` that records calls).

## BM-02: `RequireAdmin` middleware requires `publicMetadata.role === 'admin'`

**Rule.** `RequireAdmin` reads the Clerk user from the request context (panics if `RequireAuth` did not run — programmer error, caught in tests). Reads `user.PublicMetadata["role"]`, type-asserts to string, compares to `"admin"` with exact case-sensitive equality. If match, calls next handler. Otherwise writes `403 Forbidden`.

**Value.** Role check is a single, narrow operation. String equality — no regex, no "admin-ish" fallback, no case normalization. The public metadata field name, key, and value are all fixed in this spec.

**Verification.** Unit test: role=`"admin"` → handler called; role=`"user"` → 403; role missing → 403; role=`"Admin"` (wrong case) → 403; role=`"administrator"` (wrong value) → 403.

## BM-03: Middleware ordering is `RequireAuth` then `RequireAdmin`

**Rule.** Admin routes are registered via chi router group with middleware chain `[RequireAuth, RequireAdmin]` in that order. `RequireAdmin` depends on `RequireAuth` having placed the user in context; running them in the wrong order is a programmer error that `RequireAdmin` panics on.

**Value.** Explicit dependency prevents a future router rewrite from breaking auth silently. The panic surfaces the mistake immediately in testing.

**Verification.** Code review: no admin route registered without the full middleware chain. Integration test for each admin route confirms 401 for anonymous and 403 for non-admin.

## BM-04: `UserFromContext(ctx)` panics if `RequireAuth` did not run

**Rule.** Handler code that calls `auth.UserFromContext(ctx)` retrieves the authenticated user. If the context does not have a user attached (because `RequireAuth` did not run), the function panics with a clear message naming the missing middleware. This is an invariant violation, not a runtime error path.

**Value.** In Go, nil-checking an optional value on every call is noise when the invariant is "this is always set." Panic with a clear message forces the fix at development time. Integration tests that hit the handler without middleware catch this immediately.

**Verification.** Unit test: call handler directly (no middleware) → expect panic. Integration test: real routing path → no panic, handler receives user.

## BM-05: Admin routes are grouped under `/api/admin` with middleware applied once

**Rule.** All admin routes register via `r.Route("/api/admin", func(r chi.Router) { r.Use(RequireAuth, RequireAdmin); ... })`. No admin route is mounted outside this group. Adding a new admin route is a single `r.Method(...)` call inside the group.

**Value.** Prevents a new admin route from being added without auth. A future dev who copies a route pattern gets auth for free by placing it inside the group.

**Verification.** Code search: no `r.Method(...)` with an `/api/admin/` path outside the `r.Route` block. Integration tests covering each of the five current admin routes, plus a check for "handler exists → is behind the middleware."

## BM-06: Public routes are unchanged

**Rule.** Public routes (`/api/health`, `/api/config/modes`, `/api/puzzles/next`, `/api/puzzles/{id}/status`, `/api/puzzles/generate`) continue to work for anonymous callers. No session cookie required. The middleware is not applied to them.

**Value.** The phase is admin-gating only. Game-playing routes must remain open for anonymous users. A future slice can add `RequireAuth` to individual public routes (e.g., for leaderboard submission) without touching this phase's work.

**Verification.** Integration tests for every public route confirm: no cookie → 200 (or route-specific success). Adding a cookie has no effect on authorization (routes succeed either way).

## BM-07: Middleware does not log session tokens, cookies, or full user records

**Rule.** `RequireAuth` and `RequireAdmin` log only: the path, the rejection reason (for failures), and the Clerk user ID / `sub` claim (for successes). Session tokens, cookie values, full `clerk.User` objects, email addresses, and Google OAuth tokens are never logged.

**Value.** Logs are often aggregated to systems with different access controls than the backend. Session tokens in logs = session-hijack risk. Clerk user ID (a random string) is safe to log for correlation.

**Verification.** Code review: `log.Printf` / `log.Println` calls in `auth/middleware.go` only pass the fields listed above. No `%v` / `%+v` on session or user objects.

## BM-08: Middleware responds consistently on CORS preflight (`OPTIONS`)

**Rule.** `OPTIONS` requests skip auth middleware and receive standard CORS headers. Any other method without a valid session receives 401.

**Value.** OAuth flows involve preflight requests in some browsers. Rejecting OPTIONS breaks the flow before it starts.

**Verification.** Integration test: `OPTIONS /api/admin/pool` → 200 with CORS headers, no session required. `GET /api/admin/pool` without session → 401.

## BM-09: Clerk SDK errors fail closed

**Rule.** Any error from the Clerk SDK (verification failure, network error to Clerk, malformed response) results in a 401 response. The middleware logs a `WARN: auth: clerk SDK error ...` line. No 500 responses are returned from auth middleware.

**Value.** 500s leak implementation details (stack traces, error messages). Fail-closed is the correct security default. A working auth layer produces only 401/403 to the client.

**Verification.** Unit test with mocked Clerk SDK returning error → 401, not 500. Log assertion confirms WARN is emitted.

## BM-10: The SSM key `CLERK_SECRET_KEY` is loaded on startup and cached per Lambda container

**Rule.** The Clerk secret key is read from SSM Parameter Store at Lambda cold start. The key is cached in the Clerk SDK's init state for the lifetime of the container. Lambda IAM policy grants `ssm:GetParameter` on the specific parameter ARN only.

**Value.** Per-request SSM fetches are expensive and unnecessary — the key doesn't rotate within a container's lifetime. Scoped IAM prevents broad SSM access if the Lambda role is ever compromised.

**Verification.** `cmd/api/main.go` reads the parameter during init. Terraform-managed IAM policy grants only the specific parameter ARN. Verified by `terraform plan` output.
