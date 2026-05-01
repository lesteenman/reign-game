import { defineConfig } from "@playwright/test";

/**
 * Two projects:
 *
 *   integration — renders the frontend against mocked /api/* responses
 *                 (see `page.route` in each spec). Runs against the
 *                 normal Vite dev server on :5180. No backend required.
 *
 *   e2e         — drives a separate Vite on :5183 (proxying /api/* to
 *                 the e2e backend on :5182) against a seeded fixture
 *                 pool (puzzle-pool-e2e). Requires `task e2e:up`
 *                 first. Runs serially because the fixture pool holds
 *                 one puzzle per (size, mode).
 *
 * Invocations:
 *
 *   npm run test:integration   -> integration project only
 *   npm run test:e2e           -> e2e project only (assumes task e2e:up ran;
 *                                 the npm script sets PLAYWRIGHT_SKIP_WEBSERVER=1
 *                                 so Playwright does NOT spawn a redundant Vite
 *                                 on :5180 — the e2e Vite is already on :5183)
 *   npm run test:playwright    -> both projects
 *
 * webServer note: Playwright's webServer block is top-level (not per-project),
 * so we gate it on PLAYWRIGHT_SKIP_WEBSERVER. The e2e npm script sets that flag
 * because `task e2e:up` already provides Vite on :5183. Without the gate the
 * e2e job in CI would launch a wasted `npm run dev` on :5180 that no test
 * actually uses (e2e baseURL is :5183). See design-grill-summary Finding 2 #3
 * in openspec/changes/e2e-in-ci/.
 */
const skipWebServer = !!process.env.PLAYWRIGHT_SKIP_WEBSERVER;

export default defineConfig({
  testDir: "./playwright",
  // Clerk session injection scaffolding (slice
  // e2e-coverage-and-clerk-injection, D4). `clerkSetup()` runs once per
  // test process and exchanges the dev-tenant keys for a testing token.
  globalSetup: "./playwright/global-setup.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: "list",
  use: {
    baseURL: "http://localhost:5180",
    trace: "on-first-retry",
  },
  projects: [
    {
      name: "integration",
      testMatch: "integration/**/*.spec.ts",
      use: { browserName: "chromium" },
    },
    {
      name: "e2e",
      testMatch: "e2e/**/*.spec.ts",
      use: {
        browserName: "chromium",
        baseURL: "http://localhost:5183",
      },
      // Fixture pool has one puzzle per (size, mode). Parallel workers
      // would race for the single Standard 7x7 fixture and one would
      // hit the "no puzzles available" path. Serial keeps tests
      // deterministic; the suite is small enough that it isn't slow.
      fullyParallel: false,
      workers: 1,
      // No retries. A retry consumes fixture 2/2 (the StrictMode spare);
      // a second retry hits the same no-puzzles-available path the
      // project-level config is designed to avoid. Failures here need
      // a real look, not a rerun.
      retries: 0,
    },
  ],
  webServer: skipWebServer
    ? undefined
    : {
        command: "npm run dev",
        port: 5180,
        reuseExistingServer: !process.env.CI,
      },
});
