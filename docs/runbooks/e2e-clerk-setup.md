# E2E Clerk setup (test users + GitHub Actions secrets)

End-to-end runbook for the dashboard-side prep needed to make the Playwright e2e suite work in CI. This covers the steps that the slice's code does **not** automate: provisioning two test users in a Clerk dev tenant, then mirroring six secrets into both the Actions and Dependabot scopes of this repository.

The code side (Playwright `globalSetup` with `clerkSetup()`, per-spec `clerk.signIn`, `task e2e:up:generator`, isolated `puzzle-pool-e2e` table) is wired by the slice — see `openspec/changes/e2e-coverage-and-clerk-injection/` (after archive: `openspec/archive/e2e-coverage-and-clerk-injection/`). This runbook covers the third-party clicks and the GitHub-side configuration.

Per CLAUDE.md lesson 21: dashboard rotation is documented here, not as a tracked code slice.

## What you need

- Access to the **same Clerk dev tenant** already used for dev/sandbox auth work. Do **not** use the production Clerk tenant for e2e — production rotation is a post-launch operational task (D17).
- Repository admin access to GitHub `Settings → Secrets and variables`.
- ~10 minutes the first time. ~5 minutes for the quarterly rotation flow.

## 1. Create the two test users in Clerk

Both users live in the existing dev tenant alongside any human dev sessions. Use a stable email convention so they're easy to identify and revoke.

### 1a. Test admin user

1. Clerk dashboard → **Users → Create user**.
2. Email address: `e2e-admin@reign-test.example` (or, if you want a real inbox for password-reset flows, use `+e2e-admin@<your-real-email>` so Gmail aliasing routes it back to you).
3. Set a strong password. Record it — you'll paste it into the GitHub secret in section 3.
4. Save. Then open the user's detail page → **Metadata → Public metadata** → set:
   ```json
   { "role": "admin" }
   ```
   (`publicMetadata.role === 'admin'` is what the auth middleware checks.)

### 1b. Test regular user

1. Clerk dashboard → **Users → Create user**.
2. Email: `e2e-user@reign-test.example` (or `+e2e-user@<your-real-email>`).
3. Strong password. Record it.
4. **Do not** set `publicMetadata.role`. The default-empty / `'user'` value gives this account the `User` role with no admin access — exactly what AS-09's forbidden-state spec needs.

Rotate both passwords quarterly per general security hygiene. Section 4 covers the rotation flow.

## 2. Required GitHub Actions secrets

Six secrets in total. Two are already wired (PR #89); four are new for this slice.

| Secret | New? | What it is |
|---|---|---|
| `CLERK_PUBLISHABLE_KEY` | existing | Dev tenant `pk_test_*`. The `frontend-e2e` job reads this so `@clerk/clerk-react` can boot. |
| `CLERK_SECRET_KEY` | existing | Dev tenant `sk_test_*`. Backend's auth middleware verifies JWTs against this. |
| `E2E_CLERK_TEST_USER_EMAIL` | NEW | Email of the regular test user from §1b. |
| `E2E_CLERK_TEST_USER_PASSWORD` | NEW | Password for the regular test user. |
| `E2E_CLERK_TEST_ADMIN_EMAIL` | NEW | Email of the admin test user from §1a. |
| `E2E_CLERK_TEST_ADMIN_PASSWORD` | NEW | Password for the admin test user. |

The exact secret names are the **source of truth** that `.github/workflows/ci.yml`, `frontend/playwright.config.ts`, and `frontend/e2e/globalSetup.ts` cross-check against. Renaming a secret requires a sweep of all three files (CLAUDE.md lesson 14).

Distinct admin and user passwords keep the admin credential's blast-radius contained if a non-admin spec ever logs the user password (D3).

## 3. Add the secrets to GitHub (both scopes)

Per slice 1's precedent and design D3, every e2e-related secret must be mirrored into **both** the Actions scope and the Dependabot scope. Without the Dependabot mirror, Dependabot PRs run a degraded e2e job that can't sign in to Clerk.

For each of the six secrets:

1. Open `Settings → Secrets and variables → Actions → New repository secret`. Name it exactly as in the table above. Paste the value. Save.
2. Open `Settings → Secrets and variables → Dependabot → New repository secret`. Name and value identical to step 1. Save.

After all six are added in both scopes you should see 6 entries in the Actions tab and 6 entries in the Dependabot tab.

## 4. Rotation procedure

Run this every quarter, or immediately if a credential leaks.

1. Clerk dashboard → user detail page for each test user → reset password. Record the new password.
2. Update `E2E_CLERK_TEST_USER_PASSWORD` and / or `E2E_CLERK_TEST_ADMIN_PASSWORD` in **both** `Actions` and `Dependabot` scopes (same as §3 — both scopes always change together).
3. If rotating `CLERK_PUBLISHABLE_KEY` / `CLERK_SECRET_KEY` (rare for the dev tenant): same dual-scope update. Note that in production these keys are stored in SSM and the SSM parameter is the source of truth (see `admin-auth-setup.md` §8).
4. Trigger a CI run on a throwaway commit to confirm the e2e job still passes.
5. Append a row to the **History** section at the bottom of this file with the rotation date and which secrets were rotated.

Dev-tenant scoping is what makes the eventual production rotation (R-08D-equivalent) safe — these test users never touch any real-tenant data.

## 5. Verification

After §3, push a trivial commit and observe the `frontend-e2e` job in the Actions tab. The slice does not add a dedicated diagnostic step; verification is by observing the e2e specs pass. If they fail at sign-in, common causes:

- Secret name typo — re-check against the exact strings in the table in §2.
- Secret added to Actions scope but not Dependabot scope — only manifests on Dependabot PRs.
- Test user's `publicMetadata.role` not set to `'admin'` — admin specs will hit the forbidden state.
- Clerk dev-tier rate limit — the dev tenant rate-limits sign-in attempts per IP. If many parallel CI jobs run against the same tenant in quick succession, some will 429. Mitigation: re-run the failed job; long-term, the slice's `workers: 1` for serial pool-mutating specs already reduces concurrent sign-ins.

## Out of scope

- Production Clerk tenant rotation (post-launch operational task, not a tracked slice — D17).
- Custom-domain Clerk configuration (lives in R-08D infra slice).
- Promoting a real human's account to admin (covered in `admin-auth-setup.md`).

## History

<!-- Append rotation events here. Format: YYYY-MM-DD — what was rotated — who. -->
