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
 *   npm run test:e2e           -> e2e project only (assumes task e2e:up ran)
 *   npm run test:playwright    -> both projects
 */
export default defineConfig({
  testDir: "./playwright",
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
    },
  ],
  webServer: {
    command: "npm run dev",
    port: 5180,
    reuseExistingServer: !process.env.CI,
  },
});
