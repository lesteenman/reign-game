import { test, expect } from "@playwright/test";

/**
 * Connectivity-probe e2e — guards the contract between
 * `useConnectivity` (frontend) and `/api/health` (backend).
 *
 * Both halves of #116 have passing unit tests in isolation: the
 * frontend hook mocks `fetch` to return `{ ok: true }`, and the
 * backend handler test calls `handler.HealthCheck(rec, req)` directly,
 * bypassing chi's router. The original #179 follow-up shipped a
 * HEAD/405 bug to a merged-but-not-yet-deployed branch because no
 * test exercised the actual wire format — chi was registered for GET
 * only and returned 405 to the frontend's HEAD probe. This spec hits
 * the real Vite → backend path and would catch that class of
 * regression.
 *
 * Test plan:
 *   1. Navigate to / with the e2e stack online → no offline banner.
 *   2. Toggle Playwright's offline simulation → reload → banner
 *      visible + Daily tile disabled.
 *
 * The e2e stack must be up (`task e2e:up`) before this spec runs.
 * baseURL comes from playwright.config.ts: http://localhost:5183.
 */

test.describe("connectivity probe + offline banner", () => {
  test("does not show the offline banner when the backend is reachable (HEAD /api/health → 200)", async ({
    page,
  }) => {
    // Arrange — stub /api/config/modes so the landing page's mode
    // fetch resolves (unrelated to this spec's concern).
    await page.route("**/api/config/modes*", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ modes: [{ size: 5, mode: "standard" }] }),
      }),
    );

    // Act — load the landing page, then wait specifically for the
    // probe response. Starting the wait BEFORE goto ensures we don't
    // miss the response if it lands before the await — Playwright
    // buffers responses on the in-flight waiter.
    const probeResponse = page.waitForResponse(
      (response) =>
        response.url().endsWith("/api/health") &&
        response.request().method() === "HEAD",
      { timeout: 5_000 },
    );
    await page.goto("/");
    await expect(
      page.getByRole("heading", { name: /reign/i }),
    ).toBeVisible();
    const response = await probeResponse;

    // Assert — probe got 200 (this is the wire-format check that
    // would catch a regression like #179's HEAD/405) AND the banner
    // is not visible (so useConnectivity correctly translated the
    // 200 into "online").
    expect(response.status()).toBe(200);
    await expect(page.getByTestId("offline-banner")).not.toBeVisible();
  });

  test("shows the offline banner and disables Daily tile when navigator.onLine flips offline", async ({
    page,
    context,
  }) => {
    // Arrange — load while online so the SPA mounts cleanly.
    await page.route("**/api/config/modes*", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ modes: [{ size: 5, mode: "standard" }] }),
      }),
    );
    await page.goto("/");
    await expect(
      page.getByRole("heading", { name: /reign/i }),
    ).toBeVisible();
    await expect(page.getByTestId("offline-banner")).not.toBeVisible();

    // Act — go offline at the browser-context layer. Playwright
    // dispatches the navigator offline event, which `useOnlineStatus`
    // subscribes to. We don't reload here: in dev (no SW) a reload
    // while offline genuinely fails to fetch the HTML
    // (net::ERR_INTERNET_DISCONNECTED). The reload-while-offline +
    // SW-cached-shell scenario is the one `useConnectivity`'s probe
    // exists for; it's testable only against a production build with
    // a registered SW, which is out of scope here.
    await context.setOffline(true);

    // Assert — banner appears, Daily tile disables.
    await expect(page.getByTestId("offline-banner")).toBeVisible();
    await expect(page.getByTestId("tile-daily")).toBeDisabled();

    // Cleanup — restore connectivity so subsequent tests in the
    // shared serial context aren't affected.
    await context.setOffline(false);
  });
});
