import { test, expect, type APIRequestContext } from "@playwright/test";

/**
 * Pool-empty fallback e2e (slice e2e-coverage-and-clerk-injection, EC-07).
 *
 * Exercises the user-facing UX when a combo is enabled in CONFIG but has no
 * `ready` rows in `puzzle-pool-e2e` — i.e. the pool is drained.
 *
 *   drain 9#double via repeated GET /api/puzzles/next?size=9&mode=double
 *     -> backend now responds 404 {error:"no_puzzles_available"} for that combo
 *   navigate to /play?flow=curation&size=9&mode=double
 *     -> GamePage fetches the next puzzle, hits the 404
 *     -> useGameStorage's NoPuzzlesAvailableError path renders the
 *        `no-puzzles-state` testid with user-readable copy
 *
 * Per design Q-A=(a) lock-in (chunk 4), the assertion is text-only — no
 * screenshot, no specific copy match beyond "user-readable" / non-blank. If
 * the SPA ever regresses to a blank screen on the 404 path, this spec fails
 * loudly with a helpful message and we file a follow-up bug. That's the
 * "surface the bug" strategy the locked decision committed to.
 *
 * The drain runs via the public serve endpoint (loops `/api/puzzles/next`
 * until 404) rather than shelling out to `awslocal dynamodb delete-item`.
 * Two reasons:
 *   1. Pure HTTP client — no `docker compose exec` from inside the spec,
 *      mirrors served-marking's tighter pattern.
 *   2. Robust to replenishment — if pool-replenishment.spec.ts ran first
 *      and pushed 9#double's row count back up to threshold, the loop
 *      drains whatever's there. A fixture-keyed delete-item would only
 *      remove the single seed row and miss replenished rows.
 *
 * Triggering replenish to refill the pool is explicitly NOT in scope —
 * that's `pool-replenishment.spec.ts`. This spec asserts the *blocked* UX
 * state and stops there.
 *
 * This spec is part of the serial pool-mutating group (per design D9 / D11),
 * scheduled in the Playwright `pool-mutating` project alongside
 * `pool-replenishment.spec.ts` and `served-marking.spec.ts` — all three
 * mutate `puzzle-pool-e2e` rows and would race if run in parallel.
 */

const SIZE = 9;
const MODE = "double";
const NEXT_ENDPOINT = `/api/puzzles/next?size=${SIZE}&mode=${MODE}`;

// Hard upper bound on drain calls so a misconfigured replenisher (looping
// fast enough to refill mid-drain) can't spin forever. Threshold * a few
// generations of slack — well above any realistic ready count for one combo.
const DRAIN_MAX_ATTEMPTS = 50;

/**
 * Drain all ready 9#double rows by calling /api/puzzles/next until it 404s.
 * Each successful (200) response marks one more row served=true; eventually
 * the next call sees no ready rows and returns the documented 404 envelope.
 */
async function drainCombo(request: APIRequestContext): Promise<number> {
  for (let i = 0; i < DRAIN_MAX_ATTEMPTS; i++) {
    const res = await request.get(NEXT_ENDPOINT);
    if (res.status() === 404) {
      return i;
    }
    expect(
      res.ok(),
      `drain attempt ${i + 1}: GET ${NEXT_ENDPOINT} expected 200 or 404, got ${res.status()}`,
    ).toBe(true);
  }
  throw new Error(
    `drain did not reach 404 within ${DRAIN_MAX_ATTEMPTS} attempts — ` +
      `replenisher may be refilling faster than drain consumes`,
  );
}

test.describe.serial("Pool-empty fallback", () => {
  test("drained combo surfaces user-readable empty-pool UX on /play", async ({
    page,
    request,
  }) => {
    // Arrange: drain 9#double until the backend returns 404. The drain count
    // is logged in the assertion message below for diagnostic value when
    // the spec fails.
    const drainedCount = await drainCombo(request);

    // Sanity check: a follow-up call must still 404. If this fails, the
    // generator/replenisher refilled between the drain and the navigation
    // and the rest of the spec would be testing the wrong path.
    const confirm = await request.get(NEXT_ENDPOINT);
    expect(
      confirm.status(),
      `drained combo must stay 404 immediately after drain (drained ${drainedCount} rows)`,
    ).toBe(404);

    // Act: navigate to the curation Game page for the drained combo. The
    // `flow=curation` param is mandatory — without it GamePage redirects to
    // `/`. See GamePage.tsx::parseFlowType.
    await page.goto(`/play?flow=curation&size=${SIZE}&mode=${MODE}`);

    // Assert: GamePage renders the documented `no-puzzles-state` empty-state
    // testid. This is what GamePage.tsx renders when fetchNextPuzzle throws
    // NoPuzzlesAvailableError, which is what the puzzleService maps the
    // backend's 404 envelope onto.
    const emptyState = page.getByTestId("no-puzzles-state");
    await expect(
      emptyState,
      "GamePage must render `no-puzzles-state` testid when /api/puzzles/next " +
        "returns 404 — if this fails because the SPA went blank, file a " +
        "follow-up bug per Q-A=(a) lock-in",
    ).toBeVisible({ timeout: 10_000 });

    // Per Q-A=(a): the assertion is text-only, no screenshot. Just confirm
    // the empty-state element has non-blank user-readable text. The exact
    // copy is intentionally not pinned here — that would couple the e2e to
    // microcopy churn that BRAND_GUIDELINES owns.
    const text = (await emptyState.textContent())?.trim() ?? "";
    expect(
      text.length,
      "no-puzzles-state must contain non-blank user-readable text (Q-A=(a))",
    ).toBeGreaterThan(0);
  });
});
