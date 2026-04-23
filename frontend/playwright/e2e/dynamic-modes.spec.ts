import { test, expect } from "@playwright/test";

/**
 * Asserts the landing page's mode-button list matches CONFIG rows
 * with enabled=true. The e2e fixture pool is seeded (by .localstack/
 * init-aws.sh) with:
 *
 *   7#standard enabled
 *   9#standard enabled
 *   9#double   DISABLED  ← the interesting case
 *
 * The disabled combo lets us assert the filter works end-to-end:
 * it is visible in admin pool listings but absent from the public
 * /api/config/modes response and therefore absent from the UI.
 */

test.describe("e2e: dynamic mode buttons", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: /reign/i })).toBeVisible();
  });

  test("renders only enabled combos; disabled 9×9 Double is absent", async ({ page }) => {
    // Arrange / Act — wait for the buttons to render (the list is
    // fetched on mount, so the initial paint shows the loading path
    // briefly; KI-023 tracks the UX improvement). Timeout is
    // generous because the backend's first DynamoDB call can be
    // slow on a cold LocalStack (KI-022).
    const presetButtons = page.locator("[data-testid^='preset-']");
    await expect(presetButtons).toHaveCount(2, { timeout: 15_000 });

    // Assert — the two enabled combos are present; the disabled one
    // is not.
    await expect(page.getByRole("button", { name: /7.7 standard/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /9.9 standard/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /double queens/i })).toHaveCount(0);
  });
});
