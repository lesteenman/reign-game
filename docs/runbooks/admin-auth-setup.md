# Admin auth setup (Clerk + GCP OAuth)

End-to-end runbook for the human steps that R-089's infrastructure does **not** automate: creating the Clerk application, registering the GCP OAuth client, supplying the resulting keys to Terraform, and granting an account the admin role. Read this once before the first apply of the R-089 infrastructure PR.

The Terraform side (SSM parameter store entries, Lambda IAM, CloudFront cookie forwarding, CD pipeline wiring) is automated — see `infra/modules/api/{ssm.tf,iam.tf}` and `.github/workflows/cd.yml`. This runbook covers the third-party clicks.

## What you need

- A Google account that will be the project owner (the first admin).
- 30 minutes the first time. ~5 minutes for the secret rotation flow.
- Browser access to the Clerk dashboard, the GCP console, and the AWS Systems Manager (SSM) console.

## 1. Create the Clerk application

1. Sign up at <https://clerk.com> (free tier). Use the project owner's email.
2. **Create application**. Name it `Reign`. Select React as the frontend; the backend SDK choice is informational.
3. **Sign-in methods**. Open the application's `User & Authentication → Email, Phone, Username` settings.
   - Disable email-password, magic link, and any other email-based auth.
   - Disable username sign-in.
4. **Social connections**. Open `User & Authentication → Social Connections`.
   - Disable every connection EXCEPT Google. Leave Google in the "off" state for now — section 2 will fill in its credentials before you flip it on.
5. **User attributes**. Open `User & Authentication → User attributes`.
   - Add a public metadata field named `role` with type `string` and default value `user`. New accounts pick up `"user"` automatically; you'll promote the project owner to `"admin"` in section 6.
6. **Sessions**. Leave session settings at Clerk's defaults (7-day sliding) — see auth-surface spec AS-07.

Don't take the publishable / secret keys yet. Section 4 will copy them with the OAuth flow already wired so you only do the Terraform input once.

> **Heads-up on tenant type vs. domain.** Clerk publishable keys are not interchangeable across tenants — the base64 portion of `pk_test_*` / `pk_live_*` encodes the Frontend API host that the browser uses to fetch `clerk-js`. A `pk_test_*` key always points at `*.clerk.accounts.dev` (Clerk-hosted, works anywhere). A `pk_live_*` key only works if the host it encodes is a custom domain you have configured for the deployment — CloudFront default subdomains (`*.cloudfront.net`) cannot host a Clerk Frontend API and DNS will not resolve. Phase 6 shipped against `dypegk2r2t9nh.cloudfront.net` with a `pk_live_*`, and the prod page failed to load Clerk until the SSM parameters were rotated to a `pk_test_*` dev-tenant pair as a stop-gap. Don't promote a tenant to production keys until R-08D (custom domain) lands.

## 2. Create the GCP OAuth 2.0 client

1. Open <https://console.cloud.google.com>. Create a project named `Reign Auth` (or similar).
2. **APIs & Services → OAuth consent screen**.
   - User type: **External**.
   - App name: `Reign`. Support email: project owner. Developer contact: same.
   - Scopes: leave at the default minimum (`openid`, `email`, `profile`). Clerk negotiates these at runtime.
   - Test users: add the project owner's Google account. Add any other test accounts (e.g., a non-admin Google account you'll use to verify the AS-09 forbidden state).
   - Publishing status: leave as **Testing** until the app is ready for general use. Sub-100 active users do not require Google verification.
3. **APIs & Services → Credentials → Create credentials → OAuth client ID**.
   - Application type: **Web application**.
   - Name: `Reign — Clerk OAuth`.
   - **Authorized JavaScript origins**: paste each of these on its own line:
     - `https://accounts.<YOUR_CLERK_FRONTEND_API>` — Clerk's dashboard shows this under `API Keys → Show frontend API`. Looks like `accounts.<random>.clerk.accountsdev` (dev tier) or your custom domain.
   - **Authorized redirect URIs**: paste this from Clerk's `User & Authentication → Social Connections → Google` panel. Clerk displays the exact URI to use; it is per-application and looks like:
     - `https://<your-clerk-frontend-api>/v1/oauth_callback`
4. Click **Create**. Copy the **Client ID** and **Client secret** that appear in the modal — you have one chance to grab the secret without re-generating.

## 3. Wire Google into Clerk

1. Back in Clerk: `User & Authentication → Social Connections → Google → Configure`.
2. Toggle **Use custom credentials** ON.
3. Paste the **Client ID** and **Client secret** from step 2.
4. Save. Clerk will validate the credentials with Google immediately; if it fails, the redirect URI usually doesn't match exactly. Re-copy the URI from Clerk to GCP.
5. Toggle the Google connection ON.
6. Verify in an incognito browser: open Clerk's hosted sign-in URL (Clerk dashboard → Account portal → Sign-in URL). The only option visible should be "Continue with Google."

## 4. Configure Clerk redirect / allowlist URLs

1. Clerk dashboard: `Paths → Allowed origins`. Add:
   - `http://localhost:5180` — frontend dev server.
   - `http://localhost:5183` — e2e dev server (only required if running e2e locally; safe to skip until then).
   - The CloudFront distribution domain once it's known. After the first `terraform apply` lands, run `terraform -chdir=infra output -raw cloudfront_url` and paste it here.
2. `Paths → Sign-in redirect URL` and `Sign-up redirect URL`: leave at Clerk's defaults (`/`) so the SDK returns the user to the page they signed in from.

## 5. Supply the Clerk keys to Terraform / CI

Clerk dashboard → `API Keys`. You will find two values:

- **Publishable key**. Browser-safe. Format: `pk_test_...` (dev) or `pk_live_...` (prod).
- **Secret key**. Server-only. Format: `sk_test_...` or `sk_live_...`.

Both flow into Terraform via the `clerk_publishable_key` and `clerk_secret_key` root variables. There are two paths:

### Path A: GitHub Actions secrets (production CD)

Add the following GitHub repo secrets (`Settings → Secrets and variables → Actions → New repository secret`):

| Secret | Value |
|---|---|
| `CLERK_PUBLISHABLE_KEY` | The publishable key from Clerk. |
| `CLERK_SECRET_KEY` | The secret key from Clerk. |

The CD workflow (`.github/workflows/cd.yml`) reads these as `TF_VAR_clerk_publishable_key` / `TF_VAR_clerk_secret_key` and passes them to the first `terraform apply`. Both SSM parameters use `lifecycle { ignore_changes = [value] }`, so subsequent CD runs do not overwrite values you later rotate in the SSM console.

### Path B: Local apply (rare — only if you bypass CD)

Create `infra/terraform.tfvars` (gitignored):

```hcl
clerk_publishable_key = "pk_live_..."
clerk_secret_key      = "sk_live_..."
```

Run `terraform apply` from `infra/`. Delete the file after — secrets shouldn't sit on disk longer than a few seconds.

### Path C: CI e2e (dev-tenant, both Actions and Dependabot scopes)

The Playwright e2e job (`.github/workflows/ci.yml::frontend-e2e`) needs Clerk dev-tenant keys at workflow time so the e2e backend can verify Clerk JWTs and the Vite bundle can render the sign-in button. Use the **dev tenant**, not the prod tenant — these keys live in CI logs / cache directories indefinitely and the blast radius must be the dev sandbox only.

> **Do NOT also add the Path A prod-tenant `CLERK_PUBLISHABLE_KEY` / `CLERK_SECRET_KEY` to either the Actions or Dependabot secret scope.** Path A keys belong only in Terraform → SSM → Lambda. Path C keys are dev-tenant only and live entirely under repo Secrets and variables. Mixing the two is the failure mode that lesson 21's "dashboard work in runbook" pattern exists to prevent.

Add both keys in **both** secret scopes:

| Scope | Path | Keys |
|---|---|---|
| Actions | `Settings → Secrets and variables → Actions → New repository secret` | `VITE_CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY` |
| Dependabot | `Settings → Secrets and variables → Dependabot → New repository secret` | `VITE_CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY` |

Both scopes are required. Workflows triggered by Dependabot PRs only see Dependabot-scoped secrets — Actions secrets are not available in that context. Mirroring the same secret names into both scopes lets the workflow YAML reference `${{ secrets.CLERK_SECRET_KEY }}` once and resolve in either context.

When rotating: rotate **both** scopes together. The values must match — if Actions has key A and Dependabot has key B, only one set of PRs (Dependabot vs. human) will pass CI.

> Note the `VITE_` prefix on the publishable key here, distinct from `CLERK_PUBLISHABLE_KEY` in Path A. The CD path passes the publishable key into Terraform/SSM/Lambda; the CI e2e path needs the same value but exposed under the `VITE_*` prefix so Vite injects it into the browser bundle. The two are conceptually the same key (both publishable), but live under different secret names because they enter different build pipelines.

## 6. Grant an account the admin role

Once the application is deployed and at least one user has signed in via Google:

1. Clerk dashboard → `Users`.
2. Click the user (search by email if needed).
3. Scroll to **Public metadata**.
4. Click **Edit**. Set the JSON to:

```json
{
  "role": "admin"
}
```

5. Save. The change propagates to the user's session within minutes (next session refresh) — for instant effect, the user can sign out and back in.

The user can now navigate to `/admin` and reach the AdminPage. The admin link in the user menu becomes visible the moment their session reflects the new role.

### Verifying the elevation worked

- The user opens `/admin` while signed in.
- Before elevation: the page shows "This account doesn't have admin access" with a sign-out button.
- After elevation (and a session refresh): the AdminPage UI loads with the puzzle pool table, mode list, and replenish controls.

## 7. Rollback / revoke admin

To remove the admin role from an account:

1. Clerk dashboard → `Users` → select the user.
2. Public metadata: change `"role"` from `"admin"` back to `"user"` (or remove the field — middleware treats absent as `"user"`).
3. The user's next session refresh applies the change. For instant effect, click `Sessions → Revoke all sessions` for that user.

## 8. Rotate the Clerk secret key

Clerk supports rotating both keys. The infrastructure is set up so rotation does not require a Terraform change (`lifecycle { ignore_changes = [value] }`):

1. Clerk dashboard → `API Keys → Secret keys → Generate new secret key`.
2. Copy the new value.
3. AWS Systems Manager Console → `Parameter Store` → `/reign/prod/clerk-secret-key`.
4. **Edit** → paste the new value → **Save changes**.
5. The Lambda caches the secret for the lifetime of its container. Either trigger a deploy to bounce the function, or wait for natural cold starts (a few minutes of idle, then the next request rebuilds the cache).
6. Once the new key is live and you've verified `/api/admin/*` returns 200 to the admin account, **revoke the old secret** in the Clerk dashboard.

For the publishable key (rare — keys regenerate when you re-create the Clerk application):

1. Clerk dashboard → `API Keys → Publishable keys → Show / Copy new`.
2. AWS SSM Console → `/reign/prod/clerk-publishable-key` → Edit → paste new value.
3. Re-run the CD workflow (push an empty commit to main, or rerun the latest run from the Actions tab). The next frontend build picks up the new value via the SSM fetch step.

## 9. Decommission

If the project shuts down or moves off Clerk:

1. Disable the production CD workflow.
2. Clerk dashboard → `Settings → Delete application`.
3. GCP console → `Credentials` → revoke the OAuth client.
4. AWS SSM Console → delete `/reign/prod/clerk-publishable-key` and `/reign/prod/clerk-secret-key` (or `terraform destroy` the api module — the parameters are managed there).

## Reference

- Auth surface spec: `openspec/archive/phase-6-admin-auth/specs/auth-surface.md`
- Backend secret loader contract: `backend/internal/auth/secret.go` (created in slice R-08A)
- Terraform: `infra/modules/api/ssm.tf`, `infra/modules/api/iam.tf`
- CD workflow Clerk steps: `.github/workflows/cd.yml`
