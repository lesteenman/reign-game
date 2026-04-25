import { test, expect } from "@playwright/test";

/**
 * Auth-flow e2e scaffolding for Phase 6 (R-08B).
 *
 * These tests are intentionally skipped until the Clerk dev keys land
 * in R-089 and are plumbed through to the e2e Vite server via
 * VITE_CLERK_PUBLISHABLE_KEY. Once the env var is set, the skip
 * predicate goes false and the suite runs.
 *
 * The assertions below reflect AS-09 (three rendered states at
 * /admin) and AS-10 (admin link only for admins).
 *
 * Note: the first test (anonymous header) runs even without keys —
 * when the publishable key is missing we boot the SPA in a
 * Clerk-less mode (see main.tsx) and the sign-in button is not
 * rendered, which is itself a valid observable outcome. We assert
 * the page loads and the heading is visible as a smoke check.
 */

const needsClerk = !process.env.VITE_CLERK_PUBLISHABLE_KEY;

test.describe("Auth: anonymous header smoke", () => {
  test("app boots and renders the Reign heading whether or not Clerk is configured", async ({
    page,
  }) => {
    // Stub the dynamic modes endpoint so the LandingPage mount effect
    // resolves without hitting a real backend.
    await page.route("**/api/config/modes*", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ modes: [{ size: 5, mode: "standard" }] }),
      }),
    );

    await page.goto("/");
    await expect(page.getByRole("heading", { name: /reign/i })).toBeVisible();
  });

  test("sign-in button renders in the header when Clerk is configured", async ({
    page,
  }) => {
    test.skip(needsClerk, "requires VITE_CLERK_PUBLISHABLE_KEY");

    await page.route("**/api/config/modes*", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ modes: [{ size: 5, mode: "standard" }] }),
      }),
    );

    await page.goto("/");
    // Our branded sign-in wrapper exposes data-testid=sign-in-button.
    await expect(page.getByTestId("sign-in-button")).toBeVisible();
  });
});

test.describe("Auth: admin access at /admin", () => {
  test("admin signs in and lands on the admin UI", async ({ page: _page }) => {
    test.skip(needsClerk, "requires Clerk dev keys + admin test account");

    // Intended flow (unblocked once keys + test accounts land):
    // 1. Use `@clerk/testing` Playwright setup to inject an admin session.
    // 2. Navigate to /admin.
    // 3. Assert the pool-management table is visible
    //    (data-testid="pool-table").
    //
    // Left unimplemented on purpose so the first R-089 consumer
    // fills it in with their actual session-injection helper. The
    // skip predicate makes this a no-op until then.
  });

  test("non-admin signs in and sees the forbidden landing at /admin", async ({
    page: _page,
  }) => {
    test.skip(needsClerk, "requires Clerk dev keys + user test account");

    // Intended flow:
    // 1. Inject a Clerk session for a user with publicMetadata.role !== 'admin'.
    // 2. Navigate to /admin.
    // 3. Assert heading "No Admin Access" and sign-out button are visible
    //    (data-testid="admin-landing-forbidden").
  });
});
