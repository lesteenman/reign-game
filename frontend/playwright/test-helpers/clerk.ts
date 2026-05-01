import { clerk } from "@clerk/testing/playwright";
import type { Page } from "@playwright/test";

/**
 * Sign-in helpers for Playwright e2e specs that need a live Clerk
 * session. Both helpers navigate to "/" first because @clerk/testing
 * requires the Clerk client to be loaded on the page before
 * `clerk.signIn(...)` can attach to it (matches the pattern across
 * auth.spec.ts, admin-config-flow.spec.ts, pool-replenishment.spec.ts).
 *
 * Env vars (sourced via globalSetup):
 *   E2E_CLERK_TEST_ADMIN_EMAIL, E2E_CLERK_TEST_ADMIN_PASSWORD
 *   E2E_CLERK_TEST_USER_EMAIL,  E2E_CLERK_TEST_USER_PASSWORD
 */

export async function signInAsAdmin(page: Page): Promise<void> {
  await page.goto("/");
  await clerk.signIn({
    page,
    signInParams: {
      strategy: "password",
      identifier: process.env.E2E_CLERK_TEST_ADMIN_EMAIL!,
      password: process.env.E2E_CLERK_TEST_ADMIN_PASSWORD!,
    },
  });
}

export async function signInAsUser(page: Page): Promise<void> {
  await page.goto("/");
  await clerk.signIn({
    page,
    signInParams: {
      strategy: "password",
      identifier: process.env.E2E_CLERK_TEST_USER_EMAIL!,
      password: process.env.E2E_CLERK_TEST_USER_PASSWORD!,
    },
  });
}
