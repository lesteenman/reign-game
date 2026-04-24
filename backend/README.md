# Backend

Go backend for Reign: chi HTTP router, Lambda-compatible (via `aws-lambda-go-api-proxy`),
DynamoDB via `aws-sdk-go-v2`, Clerk for admin auth.

See the project root `CLAUDE.md` and `PROJECT_STRUCTURE.md` for orientation; this
file only covers backend-local dev setup.

## Local dev environment

Secrets and runtime settings for local dev live in `backend/.env.local`
(gitignored). Copy the template and fill in:

```bash
cp backend/.env.local.example backend/.env.local
# edit backend/.env.local and paste your Clerk dev secret key
```

Currently the file carries the Clerk secret used by `RequireAuth`
middleware. See `docs/runbooks/admin-auth-setup.md` (delivered with
R-089) for how to get a development key pair from the Clerk dashboard.

## Running the stack

Don't `go run ./cmd/api` by hand — go through `task dev:up:backend`
which loads `.env.local`, sets the LocalStack AWS endpoints, and
streams logs to `logs/backend.log`. See `CLAUDE.md` §"Running the
Dev Stack" for the full contract.

## Admin auth layer

Admin routes live under `/api/admin/*` and are wrapped in
`auth.RequireAuth` + `auth.RequireAdmin`. Package `internal/auth` has
the middleware, the Clerk secret bootstrapper, and the context key
used to thread the authenticated user through handlers. Unit tests
use a fake `sessionVerifier` — no Clerk credentials are required to
run the backend test suite. See `internal/auth/doc.go` for the
package overview.
