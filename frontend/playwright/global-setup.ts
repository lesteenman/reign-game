import { clerkSetup } from "@clerk/testing/playwright";

/**
 * Playwright globalSetup hook for the e2e Clerk session-injection scaffolding
 * (slice e2e-coverage-and-clerk-injection, D4).
 *
 * `clerkSetup()` exchanges the dev-tenant publishable + secret keys for a
 * long-lived testing token that bypasses Clerk's dev-instance rate limits.
 * It MUST run once per test process, before any spec calls `clerk.signIn(...)`.
 *
 * Required env vars (wired by chunk 4 of devops, commit 9ae6f64):
 *   - VITE_CLERK_PUBLISHABLE_KEY  — Clerk dev tenant publishable key
 *   - CLERK_SECRET_KEY            — Clerk dev tenant secret key
 *
 * If either is missing, `clerkSetup` throws and the entire run aborts. This
 * is intentional: the unskipped admin specs cannot run without it, so failing
 * loud at globalSetup is preferable to per-spec auth failures deep in the run.
 */
async function globalSetup() {
  await clerkSetup({
    publishableKey: process.env.VITE_CLERK_PUBLISHABLE_KEY,
    secretKey: process.env.CLERK_SECRET_KEY,
  });
}

export default globalSetup;
