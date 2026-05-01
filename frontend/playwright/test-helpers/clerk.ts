import { clerk } from "@clerk/testing/playwright";
import type { Page } from "@playwright/test";

/**
 * Sign-in helpers for Playwright e2e specs that need a live Clerk
 * session. Both helpers navigate to "/" first because @clerk/testing
 * requires the Clerk client to be loaded on the page before
 * `clerk.signIn(...)` can attach to it (matches the pattern across
 * auth.spec.ts, admin-config-flow.spec.ts, pool-replenishment.spec.ts).
 *
 * Env vars (validated lazily here — see global-setup.ts header for why
 * the validation moved out of globalSetup):
 *   E2E_CLERK_TEST_ADMIN_EMAIL, E2E_CLERK_TEST_ADMIN_PASSWORD
 *   E2E_CLERK_TEST_USER_EMAIL,  E2E_CLERK_TEST_USER_PASSWORD
 *
 * Each helper validates the vars it needs on first invocation. Missing
 * vars throw a fail-fast error naming exactly which one is absent —
 * preferable to letting `clerk.signIn` reject with an opaque "401
 * unauthorized" deep into the run.
 */

function requireEnv(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(
      `Missing required env var: ${name}. ` +
        `Required for clerk.signIn helpers in e2e tests. ` +
        `See docs/runbooks/e2e-clerk-setup.md.`,
    );
  }
  return value;
}

export async function signInAsAdmin(page: Page): Promise<void> {
  const identifier = requireEnv("E2E_CLERK_TEST_ADMIN_EMAIL");
  const password = requireEnv("E2E_CLERK_TEST_ADMIN_PASSWORD");
  await page.goto("/");
  await clerk.signIn({
    page,
    signInParams: {
      strategy: "password",
      identifier,
      password,
    },
  });
}

export async function signInAsUser(page: Page): Promise<void> {
  const identifier = requireEnv("E2E_CLERK_TEST_USER_EMAIL");
  const password = requireEnv("E2E_CLERK_TEST_USER_PASSWORD");
  await page.goto("/");
  await clerk.signIn({
    page,
    signInParams: {
      strategy: "password",
      identifier,
      password,
    },
  });
}
