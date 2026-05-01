import { test, expect, type APIRequestContext } from "@playwright/test";

/**
 * Served-marking e2e (slice e2e-coverage-and-clerk-injection, EC-06 + EC-06.5).
 *
 * Exercises the serve-then-mark-served lifecycle end-to-end against the e2e
 * stack:
 *
 *   GET /api/puzzles/next?size=7&mode=standard   -> 200, returns puzzle A;
 *                                                   marks A's row served=true
 *   GET /api/puzzles/next?size=7&mode=standard   -> 200, returns puzzle B
 *                                                   (different ID, proving
 *                                                   the served flip stuck)
 *   GET /api/puzzles/next?size=7&mode=standard   -> 404 with the documented
 *                                                   {error:"no_puzzles_available"}
 *                                                   shape (EC-06.5: pool
 *                                                   exhausted, graceful 404,
 *                                                   no 5xx, no panic)
 *
 * The two pre-seeded fixtures are committed in devops chunk 3
 * (`7_standard_seed1_000001.json` + `7_standard_seed2_000003.json`) and are
 * inserted into `puzzle-pool-e2e` with `status=ready` by `task e2e:seed`
 * before the suite runs.
 *
 * Per design D14 / chunk-4 lock-in, the Q-A 404 retry assertion (EC-06.5)
 * is folded into this same spec rather than living in its own file: after the
 * 2 fixtures are exhausted, the third call observes the terminal 404 state.
 * Triggering replenish is explicitly out of scope here — that's
 * `pool-replenishment.spec.ts`'s job.
 *
 * This spec is part of the serial pool-mutating group (per design D9 / D11),
 * scheduled in the `pool-mutating` Playwright project alongside
 * `pool-replenishment.spec.ts` and `pool-empty-fallback.spec.ts` (chunk 4) —
 * all three mutate `puzzle-pool-e2e` rows and would race if run in parallel.
 * The `test.describe.serial` wrapper below also enforces sequencing within
 * this file in case more cases get added later.
 *
 * Per the chunk-3 prompt, the simpler public-API-only assertion shape is
 * used: "different puzzle ID returned on consecutive calls" already proves
 * the served-marking worked end-to-end, without needing admin auth or the
 * `/api/admin/pool` DTO. Keeps the spec small and avoids the Clerk session
 * setup that the admin-only DTO inspection would require.
 */

const SIZE = 7;
const MODE = "standard";
const ENDPOINT = `/api/puzzles/next?size=${SIZE}&mode=${MODE}`;

type ServeResponse = {
  puzzleId: string;
  gridSize: number;
  mode: string;
  regionMap: number[][];
  metadata: {
    difficulty: number;
    maxTier: number;
    tierCounts: number[];
    traceLen: number;
    generationDurationMs: number;
    createdAt: string;
    seed?: string;
  };
};

type ErrorEnvelope = {
  error: string;
  message: string;
};

/**
 * Fetch the next ready puzzle. On a 200, asserts the response shape matches
 * the documented serve contract and returns the body. The size/mode echoed
 * in the body are also asserted against the request params — guards against
 * a backend that ever serves a wrong-shape puzzle for the requested combo.
 */
async function fetchNextPuzzle(
  request: APIRequestContext,
): Promise<ServeResponse> {
  const res = await request.get(ENDPOINT);
  expect(
    res.ok(),
    `GET ${ENDPOINT} expected 200, got ${res.status()}`,
  ).toBe(true);
  const body = (await res.json()) as ServeResponse;
  expect(body.puzzleId, "puzzleId must be non-empty").toBeTruthy();
  expect(body.gridSize).toBe(SIZE);
  expect(body.mode).toBe(MODE);
  return body;
}

test.describe.serial("Served marking", () => {
  test("consecutive calls return different puzzles, then 404 when pool exhausted", async ({
    request,
  }) => {
    // EC-06.2: first call returns puzzle A and (atomically) marks its row
    // status=served. The handler is GET /api/puzzles/next, public — no
    // Clerk session required.
    const first = await fetchNextPuzzle(request);

    // EC-06.3: second call returns puzzle B with a different ID. This is
    // the load-bearing assertion that the first call's served-marking
    // actually flipped the row — if MarkServed was a no-op, the same
    // puzzle would come back twice (NextReady picks the first ready row).
    const second = await fetchNextPuzzle(request);
    expect(
      second.puzzleId,
      "second /api/puzzles/next must return a different puzzle ID — " +
        "same ID twice would mean served-marking did not stick",
    ).not.toBe(first.puzzleId);

    // EC-06.5 (Q-A 404 retry, folded per chunk-4 lock-in): with the 2
    // pre-seeded fixtures now both served, the third call must surface
    // the documented 404 contract — graceful, no 5xx, response body
    // matches `{error:"no_puzzles_available", message: ...}`. This spec
    // does NOT trigger replenish; the 404 is the terminal state.
    const exhausted = await request.get(ENDPOINT);
    expect(
      exhausted.status(),
      `expected 404 once pool is exhausted, got ${exhausted.status()}`,
    ).toBe(404);
    const errBody = (await exhausted.json()) as ErrorEnvelope;
    expect(errBody.error).toBe("no_puzzles_available");
    expect(
      errBody.message,
      "404 must include a non-empty user-facing message",
    ).toBeTruthy();
  });
});
