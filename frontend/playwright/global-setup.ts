import { clerkSetup } from "@clerk/testing/playwright";

/**
 * Playwright globalSetup hook for the e2e Clerk session-injection scaffolding
 * (slice e2e-coverage-and-clerk-injection, D4).
 *
 * Two responsibilities, in order:
 *
 * 1. Fail-fast env-var gate. The slice's spec suite needs all 6 Clerk-related
 *    env vars to be set before any spec executes. If any are missing, throw
 *    here with a clear list rather than letting per-spec sign-in calls fail
 *    deep into the run with opaque "401 unauthorized" errors. The 4 test-user
 *    creds aren't consumed until per-test `clerk.signIn(...)`, so without this
 *    upfront gate they'd silently pass globalSetup and surface only later.
 *
 * 2. `clerkSetup()` exchanges the dev-tenant publishable + secret keys for a
 *    long-lived testing token that bypasses Clerk's dev-instance rate limits.
 *    It MUST run once per test process, before any spec calls `clerk.signIn(...)`.
 *
 * Required env vars (wired by chunk 4 of devops, commit 9ae6f64):
 *   - VITE_CLERK_PUBLISHABLE_KEY      — Clerk dev tenant publishable key
 *   - CLERK_SECRET_KEY                — Clerk dev tenant secret key
 *   - E2E_CLERK_TEST_ADMIN_EMAIL      — admin test user email
 *   - E2E_CLERK_TEST_ADMIN_PASSWORD   — admin test user password
 *   - E2E_CLERK_TEST_USER_EMAIL       — non-admin test user email
 *   - E2E_CLERK_TEST_USER_PASSWORD    — non-admin test user password
 *
 * If any are missing, the entire run aborts at globalSetup. This is
 * intentional: the unskipped admin specs cannot run without them, so failing
 * loud here is preferable to per-spec auth failures deep in the run.
 *
 * See `docs/runbooks/e2e-clerk-setup.md` for the dashboard prep that creates
 * the dev tenant + the two seeded test users.
 */
async function globalSetup() {
  const required = [
    "VITE_CLERK_PUBLISHABLE_KEY",
    "CLERK_SECRET_KEY",
    "E2E_CLERK_TEST_ADMIN_EMAIL",
    "E2E_CLERK_TEST_ADMIN_PASSWORD",
    "E2E_CLERK_TEST_USER_EMAIL",
    "E2E_CLERK_TEST_USER_PASSWORD",
  ];
  const missing = required.filter((name) => !process.env[name]);
  if (missing.length > 0) {
    throw new Error(
      `global-setup: missing required env vars: ${missing.join(", ")}. See docs/runbooks/e2e-clerk-setup.md.`,
    );
  }

  await clerkSetup({
    publishableKey: process.env.VITE_CLERK_PUBLISHABLE_KEY,
    secretKey: process.env.CLERK_SECRET_KEY,
  });
}

export default globalSetup;
