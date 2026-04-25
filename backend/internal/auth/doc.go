// Package auth provides HTTP middleware for admin authentication via Clerk.
//
// # Overview
//
// Two pieces of middleware compose the admin gate:
//
//   - [RequireAuth] reads the Clerk session cookie (__session) from the
//     request, verifies it with the Clerk SDK, fetches the full user
//     record, and attaches the user to the request context. Any failure
//     along this path produces a 401 response. The middleware fails
//     closed — a Clerk SDK error is surfaced as 401, never as 500.
//   - [RequireAdmin] reads the user from the request context (placed
//     there by RequireAuth) and allows the request through only if
//     publicMetadata.role == "admin". Everything else produces 403.
//
// Middleware order is always [RequireAuth, RequireAdmin]. Running
// RequireAdmin without RequireAuth first is a programmer error and
// panics at request time — the panic makes the mistake obvious in
// tests and development, and no such request ever reaches production
// because the route registration wires both in the correct order.
//
// CORS preflight (OPTIONS) requests bypass RequireAuth. The browser
// sends them without cookies, and rejecting them would break the
// subsequent real request.
//
// # Context key
//
// The authenticated user is stored on the request context under a
// package-private key (a zero-sized struct type). Retrieve it with
// [UserFromContext]. The key is intentionally unexported: call sites
// outside this package cannot accidentally stash unrelated values
// under the same key, and the context cannot be tampered with by
// handler code.
//
// # Secret bootstrapping
//
// The Clerk secret key is loaded once per process lifetime. Production
// (AWS Lambda) holds only the SSM parameter *name* in the
// CLERK_SECRET_PARAM_NAME environment variable — never the secret
// itself, because Lambda env vars are visible through
// lambda:GetFunctionConfiguration. The secret value is fetched from
// SSM Parameter Store at startup and cached in memory for the
// container lifetime.
//
// Local development sets CLERK_SECRET_KEY directly in
// backend/.env.local (gitignored). The bootstrapper prefers the
// env-var value when present, skipping the SSM call entirely.
//
// See [LoadClerkSecret] for the bootstrap entry point.
package auth
