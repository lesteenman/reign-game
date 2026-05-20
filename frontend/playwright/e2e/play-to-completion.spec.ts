import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import { test, expect } from "@playwright/test";

/**
 * End-to-end play-to-completion on a real backend.
 *
 * Serves the committed 7×7 Standard fixture (seed=1) from
 * `puzzle-pool-e2e` via the e2e backend on :5182. The test exercises
 * the full pipeline: /api/config/modes → button list, /api/puzzles/next
 * → a real puzzle with a real region map, cell-click → mark placement,
 * undo/redo → history stack, and the completion overlay triggered when
 * every row/column/region has exactly one marked cell with no
 * adjacency conflicts.
 *
 * The solution positions live alongside the puzzle fixture as a
 * sibling `*.solution.json` file. `task e2e:genfixtures` regenerates
 * both puzzle and solution in lockstep, so the test input is always
 * in sync with the fixture.
 */

const here = dirname(fileURLToPath(import.meta.url));
const solutionPath = resolve(here, "fixtures/puzzles/7_standard_seed1_000001.solution.json");
const SOLUTION = JSON.parse(readFileSync(solutionPath, "utf8")) as Array<[number, number]>;

/** Two pointer taps place a marker: empty → excluded → marked. */
async function placeMarker(page: import("@playwright/test").Page, row: number, col: number) {
  const cell = page.getByTestId(`cell-${row}-${col}`);
  await cell.click();
  await cell.click();
  await expect(cell).toHaveAttribute("data-cell-state", "marked");
}

test.describe("e2e: play to completion (7x7 Standard)", () => {
  test("place seven marks, exercise undo/redo mid-play, reach solved", async ({ page }) => {
    // Arrange — navigate directly to the game route. The post-R-7-02
    // LandingPage no longer surfaces size/mode preset buttons (those
    // moved to admin-gated /curation), but PlayPuzzlePage reads flow/size/
    // mode from the URL and fetches /api/puzzles/next on mount, which
    // hits the e2e backend's seeded fixture pool. The 15s timeout
    // absorbs first-DynamoDB-call cold-start latency (KI-022). This
    // spec's value is the play-through against a real backend, not the
    // landing → curation navigation chain — that's covered separately
    // once Clerk admin session injection lands (see auth.spec.ts).
    await page.goto("/play?flow=curation&size=7&mode=standard");
    await expect(page.getByTestId("game-grid")).toBeVisible({ timeout: 15_000 });

    // Act — place the first mark, then exercise undo (marked → excluded
    // via the history stack) and redo (excluded → marked) to prove the
    // undo/redo wiring survives a real game.
    const [firstRow, firstCol] = SOLUTION[0]!;
    await placeMarker(page, firstRow, firstCol);

    await page.getByTestId("undo-button").click();
    await expect(page.getByTestId(`cell-${firstRow}-${firstCol}`)).toHaveAttribute(
      "data-cell-state",
      "excluded",
    );

    await page.getByTestId("redo-button").click();
    await expect(page.getByTestId(`cell-${firstRow}-${firstCol}`)).toHaveAttribute(
      "data-cell-state",
      "marked",
    );

    // Place the remaining six marks in order.
    for (const [row, col] of SOLUTION.slice(1)) {
      await placeMarker(page, row, col);
    }

    // Assert — completion overlay appears once the seventh mark snaps
    // the board into a solved state.
    await expect(page.getByTestId("completion-overlay")).toBeVisible({ timeout: 5000 });
  });
});
