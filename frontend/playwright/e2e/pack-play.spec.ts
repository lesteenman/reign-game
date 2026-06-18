import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import { test, expect } from "@playwright/test";

/**
 * Player pack-play vertical e2e — browse → open → solve → leave →
 * re-open → resume at the next unsolved puzzle. Drives the real
 * `GET /api/packs` + `GET /api/packs/{slug}` wire and the IndexedDB
 * `pack:{slug}` progress record across a real reload (a mocked fetch +
 * jsdom can't prove resume across a real reload — frontend/CLAUDE.md
 * integration-verification rule).
 *
 * Stack assumption: `task e2e:up` (LocalStack + e2e backend on :5182 +
 * e2e frontend on :5183).
 *
 * SEED CONTRACT (lead runs this gate). The spec needs ONE published
 * pack in the e2e pack table whose first two ordered puzzles are the
 * two committed 7×7 Standard seed1 fixtures (already seeded into
 * `puzzle-pool-e2e` by `task e2e:seed`):
 *
 *   slug:        `e2e-pack`
 *   name:        `E2E Pack`
 *   size:        7
 *   mode:        `standard`
 *   published:   true
 *   puzzleIds:   [
 *                  e2e0000-0000-4000-a000-000000000001,  (seed1_000001)
 *                  e2e0000-0000-4000-a000-000000000002,  (seed1_000002)
 *                ]
 *
 * The two puzzle ids are the SKs of the committed 7×7 seed1 fixtures;
 * their solutions live alongside as `*.solution.json`. Adjust PACK_SLUG
 * / the fixture filenames here if the lead seeds a different pack.
 */

const PACK_SLUG = "e2e-pack";

const here = dirname(fileURLToPath(import.meta.url));
function loadSolution(name: string): Array<[number, number]> {
  const p = resolve(here, `fixtures/puzzles/${name}`);
  return JSON.parse(readFileSync(p, "utf8")) as Array<[number, number]>;
}
// First two ordered puzzles in the seeded pack.
const PUZZLE_1_SOLUTION = loadSolution("7_standard_seed1_000001.solution.json");

/**
 * Two pointer taps place a marker: empty → excluded → marked. When
 * `verify` is true, assert the cell settled on "marked". The final mark
 * of a pack puzzle is the exception: completing the puzzle advances the
 * pack flow to the next puzzle, unmounting this board, so the cell no
 * longer exists to assert against.
 */
async function placeMarker(
  page: import("@playwright/test").Page,
  row: number,
  col: number,
  verify = true,
) {
  const cell = page.getByTestId(`cell-${row}-${col}`);
  await cell.click();
  await cell.click();
  if (verify) {
    await expect(cell).toHaveAttribute("data-cell-state", "marked");
  }
}

/**
 * Solve the currently-rendered pack puzzle by placing its solution marks.
 * The last mark completes the puzzle and the pack flow advances (this
 * board unmounts), so it is placed without the post-mark assertion — the
 * caller asserts the advance instead.
 */
async function solveCurrent(
  page: import("@playwright/test").Page,
  solution: Array<[number, number]>,
) {
  for (let i = 0; i < solution.length; i++) {
    const [row, col] = solution[i];
    await placeMarker(page, row, col, i < solution.length - 1);
  }
}

test.describe("e2e: player pack-play (browse → solve → resume)", () => {
  test("browse a pack, solve the first puzzle, leave, re-open, resume at the next unsolved", async ({
    page,
  }) => {
    // Arrange — land on the browse surface from the landing Packs tile.
    await page.goto("/");
    await page.getByTestId("tile-packs").click();
    await expect(page.getByTestId("packs-list")).toBeVisible({ timeout: 15_000 });

    // The seeded pack appears with its 0/2 local progress.
    const tile = page.getByTestId(`pack-tile-${PACK_SLUG}`);
    await expect(tile).toBeVisible();
    await expect(tile).toContainText("0/2");

    // Act 1 — open the pack; the first puzzle's board renders.
    await tile.click();
    await expect(page).toHaveURL(new RegExp(`flow=pack&pack=${PACK_SLUG}`));
    await expect(page.getByTestId("pack-game-board")).toBeVisible({ timeout: 15_000 });
    await expect(page.getByTestId("game-grid")).toBeVisible();
    // Subtitle reflects position 1 of 2.
    await expect(page.getByText(/· 1\/2/)).toBeVisible();

    // Act 2 — solve the first puzzle. The pack flow records progress
    // (IndexedDB pack:{slug}) and advances to puzzle 2.
    await solveCurrent(page, PUZZLE_1_SOLUTION);
    await expect(page.getByText(/· 2\/2/)).toBeVisible({ timeout: 5000 });

    // Act 3 — leave to the browse surface, confirm 1/2 progress shows.
    await page.goto("/packs");
    await expect(page.getByTestId(`pack-tile-${PACK_SLUG}`)).toContainText("1/2");

    // Act 4 — re-open the pack (a fresh navigation/reload). Progress is
    // read back from IndexedDB and the flow resumes at the FIRST
    // UNSOLVED puzzle (position 2), not back at puzzle 1.
    await page.goto(`/play?flow=pack&pack=${PACK_SLUG}`);
    await expect(page.getByTestId("pack-game-board")).toBeVisible({ timeout: 15_000 });

    // Assert — resumed at position 2 of 2 (the first unsolved puzzle).
    await expect(page.getByText(/· 2\/2/)).toBeVisible();
  });
});
