import { clerkSetup } from "@clerk/testing/playwright";

/**
 * Playwright globalSetup hook for the e2e Clerk session-injection scaffolding
 * (slice e2e-coverage-and-clerk-injection, D4).
 *
 * globalSetup runs once per Playwright invocation BEFORE any project's tests
 * begin — so it must work for BOTH the `integration` project (mocks /api/*,
 * never calls `clerk.signIn`) AND the `e2e` project (drives a real Clerk
 * dev tenant). The validation surface is split accordingly:
 *
 * 1. Validates the 2 vars `clerkSetup()` needs upfront (per @clerk/testing
 *    API: publishable key always required; secret key required UNLESS
 *    CLERK_TESTING_TOKEN is preset). The 4 test-user creds are validated
 *    at call time inside `playwright/test-helpers/clerk.ts` —
 *    `signInAsAdmin` / `signInAsUser` throw a fail-fast error naming the
 *    missing var on first sign-in attempt. That keeps the integration
 *    project runnable without the test-user vars (CI Frontend Integration
 *    job), while still surfacing missing vars loudly on the e2e job.
 *
 * 2. `clerkSetup()` exchanges the dev-tenant publishable + secret keys for a
 *    long-lived testing token that bypasses Clerk's dev-instance rate
 *    limits. We only invoke it when CLERK_SECRET_KEY is present — the
 *    integration job intentionally omits the secret key (supply-chain
 *    blast-radius minimization, ci.yml comments) and doesn't need a
 *    testing token because no spec contacts the real Clerk API.
 *
 * Required env vars (wired by chunk 4 of devops, commit 9ae6f64):
 *   - VITE_CLERK_PUBLISHABLE_KEY      — Clerk dev tenant publishable key (always required)
 *   - CLERK_SECRET_KEY                — Clerk dev tenant secret key (required when running e2e specs)
 *   - E2E_CLERK_TEST_ADMIN_EMAIL      — admin test user email (validated at sign-in time)
 *   - E2E_CLERK_TEST_ADMIN_PASSWORD   — admin test user password (validated at sign-in time)
 *   - E2E_CLERK_TEST_USER_EMAIL       — non-admin test user email (validated at sign-in time)
 *   - E2E_CLERK_TEST_USER_PASSWORD    — non-admin test user password (validated at sign-in time)
 *
 * See `docs/runbooks/e2e-clerk-setup.md` for the dashboard prep that creates
 * the dev tenant + the two seeded test users.
 */
async function globalSetup() {
  if (!process.env.VITE_CLERK_PUBLISHABLE_KEY) {
    throw new Error(
      "global-setup: missing required env var: VITE_CLERK_PUBLISHABLE_KEY. See docs/runbooks/e2e-clerk-setup.md.",
    );
  }

  // Skip clerkSetup() when the secret key is absent. This is the integration
  // job's expected state (mocked /api/*, no live Clerk calls). The e2e job
  // wires CLERK_SECRET_KEY at the `Run e2e project` step, so this branch is
  // taken on every real e2e run.
  if (!process.env.CLERK_SECRET_KEY) {
    return;
  }

  await clerkSetup({
    publishableKey: process.env.VITE_CLERK_PUBLISHABLE_KEY,
    secretKey: process.env.CLERK_SECRET_KEY,
  });
}

export default globalSetup;
