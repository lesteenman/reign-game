import { test, expect } from "@playwright/test";
import { signInAsAdmin } from "../test-helpers/clerk";
import { readCombo } from "../test-helpers/admin-pool";

/**
 * Pool replenishment e2e (slice e2e-coverage-and-clerk-injection, EC-05).
 *
 * Exercises the full admin-replenish path end-to-end against the e2e stack:
 *
 *   admin signs in
 *     -> POST /api/admin/replenish?size=9&mode=double
 *     -> SQS puzzle-generation-e2e enqueue
 *     -> e2e generator dequeues + generates a 9x9 Double puzzle
 *     -> puzzle-pool-e2e gains a new ready row
 *
 * Polling assertion shape (design D9): expect.poll over GET /api/admin/pool,
 * 30 s flat timeout, intervals [500, 1000, 2000] ms — front-loads fast
 * checks while keeping a generous ceiling for CI cold-cache cases. Generator
 * latency for 9x9 Double sits well under that budget under normal dev-stack
 * timings; the 30 s ceiling is the slow-path guardrail, not the expected
 * green-path duration.
 *
 * This spec is part of the serial pool-mutating group (per design D11 and
 * EC-05 / EC-06 / EC-07). It runs in the Playwright `pool-mutating` project
 * with `fullyParallel: false, workers: 1`, alongside `served-marking.spec.ts`
 * and `pool-empty-fallback.spec.ts` (chunks 3+) — all three mutate
 * `puzzle-pool-e2e` rows and would race if run in parallel. The
 * `test.describe.serial` wrapper below also enforces sequencing within this
 * file in case more cases get added later.
 *
 * Env vars required (per design D11):
 *   E2E_CLERK_TEST_ADMIN_EMAIL — admin email wired into the e2e Clerk dev
 *   tenant. (E2E_CLERK_TEST_ADMIN_PASSWORD is stored in CI secrets for
 *   future flexibility but not consumed by the email-ticket sign-in helper.)
 */

const COMBO_SIZE = 9;
const COMBO_MODE = "double";

test.describe.serial("Pool replenishment", () => {
  test("admin replenish triggers fan-out and pool ready count rises", async ({
    page,
  }) => {
    // Use `page.request` (NOT the standalone `request` fixture) so admin
    // API calls share the page's cookie jar — clerk.signIn sets the session
    // cookie on the page's BrowserContext; the standalone `request` has its
    // own cookie jar and would 401 on /api/admin/*.
    const request = page.request;
    // Sign in as admin so /api/admin/* requests carry a Clerk JWT with
    // role=admin. Pattern mirrors auth.spec.ts.
    await signInAsAdmin(page);

    // Snapshot the pre-replenish state for the 9#double combo.
    // EC-05.1 puts the seed fixture at count=1 below threshold; we read
    // the actual baseline rather than hard-coding so the spec is robust
    // to fixture tweaks.
    const before = await readCombo(request, COMBO_SIZE, COMBO_MODE);
    const targetCount = before.config.threshold;
    expect(
      before.readyCount,
      "fixture must seed the combo below threshold so replenish has work to do",
    ).toBeLessThan(targetCount);

    // EC-05.2: trigger the replenish. The handler reads CONFIG, sees the
    // 9#double combo is below threshold, and publishes (threshold - count)
    // GenerationRequest messages onto puzzle-generation-e2e.
    const replenishRes = await request.post(
      `/api/admin/replenish?size=${COMBO_SIZE}&mode=${COMBO_MODE}`,
    );
    expect(
      replenishRes.ok(),
      `POST /api/admin/replenish failed: ${replenishRes.status()}`,
    ).toBe(true);
    const replenishBody = (await replenishRes.json()) as {
      triggered: { size: number; mode: string; count: number }[];
      skipped: { size: number; mode: string; ready: number }[];
    };
    const triggered = replenishBody.triggered.find(
      (t) => t.size === COMBO_SIZE && t.mode === COMBO_MODE,
    );
    expect(
      triggered,
      "replenish should report 9#double as triggered (was below threshold)",
    ).toBeDefined();
    expect(triggered!.count).toBeGreaterThan(0);

    // EC-05.3: poll the admin pool endpoint until readyCount catches up
    // to the configured threshold (or, more weakly, exceeds the
    // pre-replenish baseline by at least one — the threshold is the
    // strict assertion the spec calls for).
    await expect
      .poll(
        async () => {
          const cur = await readCombo(request, COMBO_SIZE, COMBO_MODE);
          return cur.readyCount;
        },
        {
          timeout: 30_000,
          intervals: [500, 1000, 2000],
          message: `9#double readyCount did not reach threshold ${targetCount} within 30s`,
        },
      )
      .toBeGreaterThanOrEqual(targetCount);
  });
});
