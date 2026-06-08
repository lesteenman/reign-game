import { test, expect } from "@playwright/test";

/**
 * Daily-flow e2e (R-8-02 chunk 8).
 *
 * Two cases under DP-23, DP-25, DP-27, DP-31, DP-32:
 *
 *   1. Happy path — anonymous user clicks the daily tile on LandingPage,
 *      lands on `/play?flow=daily`, the GET /api/daily/{date} round-trip
 *      resolves, and DailyGameBoard renders. No solve is performed —
 *      driving the real grid (drag pointer events, marker placement)
 *      via Playwright is too brittle for a meaningful e2e signal; the
 *      submit / post-completion path is covered by Vitest in
 *      DailyFlow.test.tsx and PostCompletionScreen.test.tsx.
 *
 *   2. Already-solved short-circuit (DP-27) — IndexedDB is pre-seeded
 *      with a `'solved'` daily row for today's UTC date BEFORE
 *      navigating to /play?flow=daily. The page must render
 *      <PostCompletionScreen /> directly with NO network call to
 *      /api/daily/* — the negative-network-call assertion is the
 *      observable that proves the short-circuit.
 *
 * Stack assumption: `task e2e:up` (LocalStack + e2e backend on :5182 +
 * e2e frontend on :5183 + e2e generator). Daily reads do NOT mutate
 * `puzzle-pool-e2e` config or replenish state — the schedule row is
 * auto-created on first GET (idempotent), so this spec stays in the
 * default `e2e` Playwright project (NOT the serial `pool-mutating`
 * group). Anonymous identity (DP-28): the SPA mints a `deviceId` in
 * localStorage on first invocation; no pre-test setup needed.
 *
 * Fixture dependency: the e2e fixture pool seeded by `task e2e:seed`
 * must contain at least one approved 9x9 standard puzzle for the
 * sync-fallback (clean LocalStack on first GET of the day) to
 * succeed. Per chunk #90 of R-8-01 / e2e fixtures, this is in place.
 *
 * IndexedDB schema reference (frontend/src/storage/db.ts): DB_NAME
 * `reign-game`, version 2, object store `gameState`, keyPath `id`.
 * Composite key shape: `${flowType}:${flowId}`, e.g. `daily:2026-05-02`.
 * The seeded row's shape mirrors what DailyFlow.handleSolved writes on
 * a successful submit (see DailyFlow.tsx persistedState block).
 */

const DAILY_TILE_TESTID = "tile-daily";
const DAILY_GAME_BOARD_TESTID = "daily-game-board";
const DAILY_POST_COMPLETION_TESTID = "daily-post-completion";
const DAILY_RECYCLE_LINE_TESTID = "daily-recycle-line";

/**
 * Device id the e2e seed pre-seeds a solved PLAY row against, on the
 * recycle-mode DAILY#<today> schedule row (#150). The recycle-line spec
 * pins localStorage `reign.deviceId` to this value so the GET resolves
 * to the seeded solved play, returns isRecycle=true, and renders
 * PostCompletionScreen with the recycle line. Must match the value in
 * Taskfile.yml e2e:seed (recycle_device).
 */
const RECYCLE_SEED_DEVICE_ID = "e2e-recycle-device-150";
/** localStorage key the SPA reads the anonymous deviceId from. */
const DEVICE_ID_STORAGE_KEY = "reign.deviceId";

/** Today's date as YYYY-MM-DD in UTC — matches DailyFlow.todayUtcDate. */
function todayUtcDate(): string {
  return new Date().toISOString().slice(0, 10);
}

/** Read the MM:SS text from the timer-display element and return seconds. */
async function readTimerSeconds(
  locator: import("@playwright/test").Locator,
): Promise<number> {
  const text = (await locator.textContent())?.trim() ?? "";
  const match = text.match(/^(\d+):(\d+)$/);
  if (!match) {
    throw new Error(`Unexpected timer-display text: ${JSON.stringify(text)}`);
  }
  return Number(match[1]) * 60 + Number(match[2]);
}

test.describe("Daily flow", () => {
  test("happy path — anonymous user navigates from landing tile to playing state", async ({
    page,
  }) => {
    // Arrange: clear any leftover daily state so the GET path runs.
    // The e2e Vite serves a fresh origin per test (Playwright contexts
    // are isolated), but a pre-existing solved row in IndexedDB from a
    // prior run on the same browser profile would short-circuit DP-27
    // and skip the load path we want to assert. We start from "/" so
    // the storage origin is bound before deleteDatabase fires.
    await page.goto("/");
    await page.evaluate(
      () =>
        new Promise<void>((resolve, reject) => {
          const req = indexedDB.deleteDatabase("reign-game");
          req.onsuccess = () => resolve();
          req.onerror = () => reject(req.error);
          req.onblocked = () => resolve();
        }),
    );
    await page.goto("/");

    // Act: click the daily tile.
    const tile = page.getByTestId(DAILY_TILE_TESTID);
    await expect(tile).toBeVisible();
    await tile.click();

    // Assert: URL reflects the daily flow and the GameBoard mounts after
    // the GET /api/daily/{date} round-trip resolves to a playing state.
    await expect(page).toHaveURL(/\/play\?flow=daily/);
    await expect(page.getByTestId(DAILY_GAME_BOARD_TESTID)).toBeVisible({
      timeout: 15_000,
    });
  });

  test("KI-025 — displayed timer does NOT reset when the player navigates back and re-enters", async ({
    page,
  }) => {
    // Arrange: start clean so the daily PLAY row is materialised on
    // first GET (sets assignedAt server-side, never overwritten on
    // re-GET per DP-19 — the invariant this fix relies on).
    await page.goto("/");
    await page.evaluate(
      () =>
        new Promise<void>((resolve, reject) => {
          const req = indexedDB.deleteDatabase("reign-game");
          req.onsuccess = () => resolve();
          req.onerror = () => reject(req.error);
          req.onblocked = () => resolve();
        }),
    );
    await page.goto("/");

    // Act 1: enter the daily flow and capture the initial timer
    // reading. Timer-display text is MM:SS (e.g. "00:03"); convert to
    // seconds for arithmetic.
    await page.getByTestId(DAILY_TILE_TESTID).click();
    const board = page.getByTestId(DAILY_GAME_BOARD_TESTID);
    await expect(board).toBeVisible({ timeout: 15_000 });
    const timer = page.getByTestId("timer-display");
    await expect(timer).toBeVisible();
    const firstReading = await readTimerSeconds(timer);

    // Act 2: linger on the puzzle, then navigate back to landing.
    await page.waitForTimeout(3_000);
    await page.goto("/");
    await expect(page.getByTestId(DAILY_TILE_TESTID)).toBeVisible();
    await page.waitForTimeout(3_000); // simulated "in the menu" away time

    // Act 3: re-enter daily. With the wall-clock anchor on `assignedAt`,
    // the displayed elapsed must reflect the total wall time since the
    // first GET — not restart from 0 (KI-025 symptom).
    await page.getByTestId(DAILY_TILE_TESTID).click();
    await expect(board).toBeVisible({ timeout: 15_000 });
    await expect(timer).toBeVisible();
    const secondReading = await readTimerSeconds(timer);

    // Assert: ~6 seconds passed between readings (3s linger + 3s away).
    // Allow a 2s tolerance for navigation + network round-trips.
    const elapsedBetweenReadings = secondReading - firstReading;
    expect(
      elapsedBetweenReadings,
      `timer should reflect wall-clock progress across navigate-back ` +
        `(got first=${firstReading}s second=${secondReading}s, delta=${elapsedBetweenReadings}s)`,
    ).toBeGreaterThanOrEqual(4);
    // Guard against a regression where the second reading drops to
    // zero entirely (the original symptom).
    expect(
      secondReading,
      `second reading must NOT reset to 0 (KI-025 regression)`,
    ).toBeGreaterThan(0);
  });

  test("already-solved short-circuit (DP-27) renders PostCompletionScreen with no network call", async ({
    page,
  }) => {
    // Arrange: seed IndexedDB with a solved daily row for today BEFORE
    // navigating to /play?flow=daily. The seed must include
    // `serverElapsedMs > 0` so DailyFlow's short-circuit predicate
    // (DailyFlow.tsx ~L131) accepts the row.
    //
    // Origin pinning: openDB needs a real document origin. We hop to
    // "/" first (LandingPage), drop any stale DB, then write the seed
    // row, then navigate to /play?flow=daily.
    await page.goto("/");

    // Wipe and re-create the DB at v2 with a single solved row.
    const flowId = todayUtcDate();
    await page.evaluate(
      ({ flowId }) =>
        new Promise<void>((resolve, reject) => {
          const del = indexedDB.deleteDatabase("reign-game");
          del.onerror = () => reject(del.error);
          del.onblocked = () => resolve();
          del.onsuccess = () => {
            const open = indexedDB.open("reign-game", 2);
            open.onupgradeneeded = () => {
              const db = open.result;
              if (!db.objectStoreNames.contains("gameState")) {
                db.createObjectStore("gameState", { keyPath: "id" });
              }
              if (!db.objectStoreNames.contains("completions")) {
                db.createObjectStore("completions", { autoIncrement: true });
              }
            };
            open.onerror = () => reject(open.error);
            open.onsuccess = () => {
              const db = open.result;
              const tx = db.transaction("gameState", "readwrite");
              tx.oncomplete = () => {
                db.close();
                resolve();
              };
              tx.onerror = () => reject(tx.error);
              const seed = {
                id: `daily:${flowId}`,
                flowType: "daily",
                flowId,
                puzzle: {
                  puzzleId: "seed-fixture-puzzle",
                  gridSize: 9,
                  mode: "standard",
                  regionMap: Array.from({ length: 9 }, () =>
                    Array.from({ length: 9 }, () => 0),
                  ),
                },
                cells: Array.from({ length: 9 }, () =>
                  Array.from({ length: 9 }, () => "empty"),
                ),
                timer: {
                  elapsedAtLastPause: 60_000,
                  lastResumedAt: null,
                },
                status: "solved",
                startedAt: Date.now() - 60_000,
                serverElapsedMs: 60_000,
                submittedAt: `${flowId}T12:00:00.000Z`,
                leaderboardRank: undefined,
              };
              tx.objectStore("gameState").put(seed);
            };
          };
        }),
      { flowId },
    );

    // Track network calls to /api/daily/*. The DP-27 short-circuit
    // contract is "no GET to /api/daily/{date}" — assert by negative
    // observation. We attach the listener BEFORE navigation so any
    // request fired during the load path is captured.
    const dailyApiCalls: string[] = [];
    page.on("request", (req) => {
      const url = req.url();
      if (url.includes("/api/daily/")) {
        dailyApiCalls.push(`${req.method()} ${url}`);
      }
    });

    // Act: navigate directly to the daily flow.
    await page.goto("/play?flow=daily");

    // Assert: PostCompletionScreen renders straight from the persisted
    // row. No /api/daily/* request should have fired in the process.
    await expect(
      page.getByTestId(DAILY_POST_COMPLETION_TESTID),
    ).toBeVisible({ timeout: 5_000 });
    expect(
      dailyApiCalls,
      `DP-27 short-circuit must skip the network round-trip; got: ${dailyApiCalls.join(", ")}`,
    ).toEqual([]);
  });

  test("recycle day (#150) — recycle line renders on the network-backed PostCompletionScreen", async ({
    page,
  }) => {
    // Real-wire proof for #150: the backend derives `isRecycle` on
    // GET /api/daily/{date} from the schedule row's `mode` attribute,
    // and the frontend renders the recycle line only on the *network*
    // path (the IDB short-circuit hardcodes isRecycle=false — see
    // useDailyPuzzle.ts). So this MUST drive the GET, not the cache.
    //
    // The e2e seed (Taskfile.yml e2e:seed) pre-seeds, for today (UTC):
    //   - a DAILY#<today> schedule row with mode=recycle
    //   - a solved PLAY#<RECYCLE_SEED_DEVICE_ID>/DAILY#<today> row
    // We pin localStorage `reign.deviceId` to that device so the GET
    // resolves to the seeded solved play (outcome=solved, with
    // serverElapsedMs + submittedAt), which makes useDailyPuzzle return
    // kind:'solved' from the network payload carrying isRecycle=true,
    // and DailyFlow renders PostCompletionScreen with the recycle line.

    // Arrange: bind the storage origin, pin the seeded device id, and
    // wipe IndexedDB so the GET path runs (no DP-27 short-circuit).
    await page.goto("/");
    await page.evaluate(
      ({ deviceId, deviceKey }) => {
        localStorage.setItem(deviceKey, deviceId);
        return new Promise<void>((resolve, reject) => {
          const req = indexedDB.deleteDatabase("reign-game");
          req.onsuccess = () => resolve();
          req.onerror = () => reject(req.error);
          req.onblocked = () => resolve();
        });
      },
      {
        deviceId: RECYCLE_SEED_DEVICE_ID,
        deviceKey: DEVICE_ID_STORAGE_KEY,
      },
    );

    // Track the GET so we can prove the recycle line came from the wire,
    // not a cached short-circuit.
    const dailyGets: string[] = [];
    page.on("request", (req) => {
      if (
        req.method() === "GET" &&
        req.url().includes("/api/daily/") &&
        !req.url().includes("/result")
      ) {
        dailyGets.push(req.url());
      }
    });

    // Act: navigate into the daily flow.
    await page.goto("/play?flow=daily");

    // Assert: PostCompletionScreen renders (the seeded play is solved)
    // and the recycle line is present with the exact copy.
    await expect(
      page.getByTestId(DAILY_POST_COMPLETION_TESTID),
    ).toBeVisible({ timeout: 15_000 });
    const recycleLine = page.getByTestId(DAILY_RECYCLE_LINE_TESTID);
    await expect(recycleLine).toBeVisible();
    await expect(recycleLine).toHaveText(
      "Today's puzzle is a recycle of a recent day.",
    );

    // The recycle flag must have arrived over the wire — at least one
    // GET /api/daily/{date} fired (no cache short-circuit).
    expect(
      dailyGets.length,
      `recycle line must come from the network GET, not the IDB short-circuit`,
    ).toBeGreaterThan(0);
  });
});
