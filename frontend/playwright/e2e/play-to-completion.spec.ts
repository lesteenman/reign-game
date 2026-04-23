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
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: /reign/i })).toBeVisible();
  });

  test("place seven marks, exercise undo/redo mid-play, reach solved", async ({ page }) => {
    // Arrange — wait for the dynamic button list to render (cold-
    // start latency on the first DynamoDB call — KI-022), pick 7×7
    // Standard, and start the game.
    const sevenByFive = page.getByRole("button", { name: /7.7 standard/i });
    await expect(sevenByFive).toBeVisible({ timeout: 15_000 });
    await sevenByFive.click();
    await page.getByTestId("play-button").click();
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
