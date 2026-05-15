# internal/auth/

Clerk-backed HTTP middleware for admin authentication, plus the one-shot Clerk-secret loader. See `doc.go` for the package-level documentation; this README is the additive overview.

## Data flow

- **In** — `__session` Clerk cookie on every non-OPTIONS request. The Clerk JS SDK on the frontend sets it; CloudFront forwards it to API Gateway.
- **Calls** — `github.com/clerk/clerk-sdk-go/v2` (the SDK), `github.com/clerk/clerk-sdk-go/v2/jwt::Verify`, `.../user::Get`. Plus `aws-sdk-go-v2/service/ssm` for the secret loader.
- **Out** — Either `next.ServeHTTP` with the `*clerk.User` attached to `r.Context()` under a package-private key, or a 401 / 403 JSON error via `httperr.WriteError`.

## Middleware suite

| Function | Purpose | Failure mode |
|---|---|---|
| `RequireAuth(verifier)` | Hard gate. Verifies the JWT, fetches the user, attaches to context. | 401 on any failure (fail-closed per BM-09). |
| `OptionalAuth(verifier)` | Soft gate. Same flow on success; passes the request through unchanged on any failure so the handler can fall back to a device-ID header. | Silent. No logging on miss. |
| `RequireAdmin` | Reads the user from context (`UserFromContext` panics on missing user) and admits only `publicMetadata.role == "admin"`. | 403. Panics if `RequireAuth` didn't run first — surfaces a programmer error at request time. |

Middleware order is always `(RequireAuth, RequireAdmin)`. Reversing them or omitting `RequireAuth` is a panic, not a silent 403.

## Key types and exported symbols

- `RequireAuth`, `OptionalAuth`, `RequireAdmin` — the three middleware functions.
- `NewClerkSessionVerifier()` — production verifier with a 60s in-memory user cache.
- `UserFromContext`, `UserFromContextOK`, `WithUserForTest` — context accessors.
- `LoadClerkSecret(ctx)` — `sync.Once`-cached secret loader. Resolves `CLERK_SECRET_KEY` env var first, falls back to `CLERK_SECRET_PARAM_NAME` SSM parameter.
- Unexported but important: `sessionVerifier` (interface), `clerkSessionVerifier` (production impl), `cachedSessionVerifier` (TTL cache wrapper), `userContextKey` (zero-sized struct used as context key).

## Caching strategy

The session JWT does not carry `publicMetadata`, so every admin request would otherwise round-trip Clerk's `/v1/users/{id}` endpoint (~200 ms over the public internet). `cachedSessionVerifier` keys on the subject ID in a `sync.Map` with a 60s TTL — second-and-later hits become ~1 µs. The race where two goroutines miss simultaneously is benign (both writes set the same value). Role-change propagation is bounded by the TTL.

## Rules specific to this directory

- **Fail closed.** A Clerk SDK error is logged at `WARN` and answered with 401 — never with 5xx (BM-09).
- **No cookie value in logs.** Logging `err` from `jwt.Verify` is OK because the SDK formats its own message; logging the raw error object with `%v` could leak the token, so don't (BM-07 in the source).
- **Distinguish expired tokens.** `errors.Is(err, gojwt.ErrExpired)` produces a `"session expired"` body; everything else is `"invalid session"`. The frontend uses this to render a different prompt.
- **Per-step timing.** `RequireAuth` logs `verify_ms=N get_user_ms=N` on every success and every `WARN` so the dev loop surfaces SDK latency outliers.
- **`WithUserForTest` is for test packages only.** The name signals the contract; nothing outside `*_test.go` should import it.

## Secret bootstrap

- **Local dev** — `CLERK_SECRET_KEY` from `backend/.env.local` (gitignored). The SSM client is never constructed.
- **Production (Lambda)** — `CLERK_SECRET_PARAM_NAME` holds the SSM path (e.g. `/reign/prod/clerk-secret-key`). The actual secret is fetched once at cold start and cached for the container lifetime. Lambda env vars never carry the secret itself because `lambda:GetFunctionConfiguration` would expose it.
- **Compile-time guard** — `var _ ssmGetter = (*ssm.Client)(nil)` at the bottom of `secret.go` fails the build if AWS ever changes the SDK surface in a breaking way.
