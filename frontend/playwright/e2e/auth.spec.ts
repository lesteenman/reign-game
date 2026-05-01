import { test, expect } from "@playwright/test";
import { clerk } from "@clerk/testing/playwright";
import {
  signInAsAdmin,
  signInAsUser,
} from "../test-helpers/clerk";

/**
 * Auth-flow e2e (Phase 6 R-08B + slice e2e-coverage-and-clerk-injection).
 *
 * Clerk session injection is wired via `globalSetup` (see
 * `playwright/global-setup.ts`). Each admin/user-gated test below calls
 * `clerk.signIn(...)` against its own browser context, per design D4
 * (per-spec sign-in beats long-lived storageState because Clerk JWTs expire
 * after ~1 h and we'd rather pay the sign-in cost than debug late-run
 * auth flakes).
 *
 * Assertions reflect AS-09 (three rendered states at /admin), AS-10
 * (admin link only for admins), and EC-01..03 from
 * `openspec/changes/e2e-coverage-and-clerk-injection/specs/e2e-coverage.md`.
 *
 * Note: the first test (anonymous header smoke) runs even without keys —
 * when the publishable key is missing we boot the SPA in a Clerk-less mode
 * (see `main.tsx`) and the sign-in button is not rendered, which is itself
 * a valid observable outcome.
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
  // EC-01: admin route requires admin role. Validates BOTH branches:
  // non-admin user sees the forbidden landing, admin user sees the
  // pool widget.
  test("admin route requires admin role", async ({ browser }) => {
    // Branch 1: non-admin → forbidden landing.
    const userContext = await browser.newContext();
    const userPage = await userContext.newPage();
    await signInAsUser(userPage);
    await userPage.goto("/admin");
    await expect(
      userPage.getByTestId("admin-landing-forbidden"),
    ).toBeVisible();
    await userContext.close();

    // Branch 2: admin → pool widget.
    const adminContext = await browser.newContext();
    const adminPage = await adminContext.newPage();
    await signInAsAdmin(adminPage);
    await adminPage.goto("/admin");
    await expect(adminPage.getByTestId("pool-table")).toBeVisible();
    await adminContext.close();
  });

  // EC-02: sign-out clears the Clerk session and a subsequent /admin
  // navigation falls back to the unauthenticated path.
  test("user can sign out", async ({ page }) => {
    await signInAsAdmin(page);
    await page.goto("/admin");
    await expect(page.getByTestId("pool-table")).toBeVisible();

    await clerk.signOut({ page });

    // After sign-out, /admin must NOT show the pool widget. The exact
    // post-sign-out surface (sign-in CTA vs. forbidden landing) is
    // determined by <ProtectedAdminRoute>'s unauthenticated branch — we
    // assert the negative (pool-table absent) rather than pin a specific
    // alternate testid.
    await page.goto("/admin");
    await expect(page.getByTestId("pool-table")).toHaveCount(0);
  });

  // EC-03: admin session persists across reload (storage-state survives
  // without re-auth).
  test("admin session persists across reload", async ({ page }) => {
    await signInAsAdmin(page);

    await page.goto("/admin");
    await expect(page.getByTestId("pool-table")).toBeVisible();

    await page.reload();
    await expect(page.getByTestId("pool-table")).toBeVisible();
  });
});
