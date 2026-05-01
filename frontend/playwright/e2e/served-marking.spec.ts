import { test, expect, type APIRequestContext } from "@playwright/test";

/**
 * Served-marking e2e (slice e2e-coverage-and-clerk-injection, EC-06 + EC-06.5).
 *
 * Exercises the serve-then-mark-served lifecycle end-to-end against the e2e
 * stack. The spec loops `GET /api/puzzles/next?size=7&mode=standard` until the
 * pool is exhausted, then asserts the documented 404 contract on the next
 * call. The intent is to prove:
 *
 *   1. Each call returns a distinct puzzle ID — the served flag from the
 *      previous call stuck, otherwise NextReady would re-serve the same row.
 *   2. The pool eventually exhausts and the handler surfaces the documented
 *      `{error: "no_puzzles_available"}` 404 (EC-06.5).
 *
 * The loop-until-404 shape is robust to fixture-pool size drift: `task e2e:seed`
 * loads every `*.json` (excluding `*.solution.json`) under
 * `frontend/playwright/e2e/fixtures/puzzles/`, and that set has churned over
 * the slice (currently 3 `7#standard` rows; previously 2). Hardcoding "expect
 * 404 on call N" couples the spec to a count that will break the next time
 * fixtures are added.
 *
 * Per design D14 / chunk-4 lock-in, the Q-A 404 retry assertion (EC-06.5) is
 * folded into this same spec rather than living in its own file: once the
 * fixtures are exhausted, the next call observes the terminal 404 state.
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

// Safety bound on the loop. The fixture pool is small (single-digit rows
// per shape) so 20 calls is comfortably above any realistic drift. If the
// loop ever hits this ceiling, that's a bug — assert against it explicitly.
const MAX_CALLS = 20;

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

test.describe.serial("Served marking", () => {
  test("consecutive calls return distinct puzzles, then 404 when pool exhausted", async ({
    request,
  }) => {
    // Drain the ready pool one call at a time. EC-06.2 / EC-06.3: each
    // 200 must come back with a different puzzleId — the load-bearing
    // assertion that the previous call's served flip stuck. If MarkServed
    // were a no-op, NextReady would return the first ready row every time.
    const seen: string[] = [];
    let exhaustedRes: Awaited<ReturnType<APIRequestContext["get"]>> | undefined;
    for (let i = 0; i < MAX_CALLS; i++) {
      const res = await request.get(ENDPOINT);
      if (res.status() === 404) {
        exhaustedRes = res;
        break;
      }
      expect(
        res.ok(),
        `GET ${ENDPOINT} (call ${i + 1}) expected 200, got ${res.status()}`,
      ).toBe(true);
      const body = (await res.json()) as ServeResponse;
      expect(body.puzzleId, "puzzleId must be non-empty").toBeTruthy();
      expect(body.gridSize).toBe(SIZE);
      expect(body.mode).toBe(MODE);
      seen.push(body.puzzleId);
    }

    // We must have seen at least 2 distinct puzzles served back-to-back —
    // otherwise the served-marking signal isn't observable. The fixture
    // pool always has >= 2 ready rows for `7#standard` (the StrictMode
    // spare alone guarantees that).
    expect(
      seen.length,
      `expected at least 2 ready puzzles before exhaustion, got ${seen.length}`,
    ).toBeGreaterThanOrEqual(2);
    expect(
      new Set(seen).size,
      "every served puzzle ID must be distinct — repeats would mean " +
        "served-marking did not stick on the previous call",
    ).toBe(seen.length);

    // EC-06.5 (Q-A 404 retry, folded per chunk-4 lock-in): once the
    // pool drains, the handler must surface the documented 404 contract —
    // graceful, no 5xx, response body matches
    // `{error: "no_puzzles_available", message: ...}`. The loop must
    // have terminated via 404, not by hitting MAX_CALLS.
    expect(
      exhaustedRes,
      `loop hit MAX_CALLS=${MAX_CALLS} without observing a 404; ` +
        `pool-drain assertion is dead. Either the fixture pool is much ` +
        `larger than expected or the served-marking is leaking rows.`,
    ).toBeDefined();
    expect(
      exhaustedRes!.status(),
      `expected 404 once pool is exhausted, got ${exhaustedRes!.status()}`,
    ).toBe(404);
    const errBody = (await exhaustedRes!.json()) as ErrorEnvelope;
    expect(errBody.error).toBe("no_puzzles_available");
    expect(
      errBody.message,
      "404 must include a non-empty user-facing message",
    ).toBeTruthy();
  });
});
