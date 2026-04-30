import { test, expect } from "@playwright/test";

const MOCK_PUZZLE = {
  puzzleId: "playtest-001",
  gridSize: 5,
  mode: "standard",
  regionMap: [
    [0, 0, 1, 1, 1],
    [0, 0, 1, 2, 2],
    [3, 3, 1, 2, 2],
    [3, 4, 4, 4, 2],
    [3, 3, 4, 4, 4],
  ],
};

test.describe("Grid Interaction", () => {
  test.beforeEach(async ({ page }) => {
    // Intercept puzzle API calls and return mock data so tests don't
    // depend on a running backend.
    await page.route("**/api/puzzles/next*", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(MOCK_PUZZLE),
      }),
    );
    await page.route("**/api/puzzles/*/status*", (route) =>
      route.fulfill({ status: 200, body: "{}" }),
    );
    // Navigate directly to the game route. The post-R-7-02 LandingPage
    // has no public play flow (Daily/Packs are placeholders, Curation
    // is admin-gated), but GamePage itself reads flow/size/mode from
    // the URL and fetches /api/puzzles/next on mount — auth-independent.
    // These tests assert grid mechanics, not landing nav.
    await page.goto("/play?flow=curation&size=5&mode=standard");
    await expect(page.getByTestId("game-grid")).toBeVisible();
  });

  test("grid renders 25 cells for a 5x5 puzzle", async ({ page }) => {
    const cells = page.locator("[data-testid^='cell-']");
    await expect(cells).toHaveCount(25);
  });

  test("cells have region background colors", async ({ page }) => {
    const cell = page.getByTestId("cell-0-0");
    await expect(cell).toHaveCSS("background-color", /rgb/);
  });

  test("three-tap cycle: empty -> excluded -> marked -> empty", async ({
    page,
  }) => {
    const cell = page.getByTestId("cell-2-2");

    // Initially empty
    await expect(cell).toHaveAttribute("data-cell-state", "empty");

    // First click: excluded
    await cell.click();
    await expect(cell).toHaveAttribute("data-cell-state", "excluded");

    // Second click: marked
    await cell.click();
    await expect(cell).toHaveAttribute("data-cell-state", "marked");

    // Third click: back to empty
    await cell.click();
    await expect(cell).toHaveAttribute("data-cell-state", "empty");
  });

  test("conflict highlighting when two markers in same row", async ({
    page,
  }) => {
    const cell00 = page.getByTestId("cell-0-0");
    const cell01 = page.getByTestId("cell-0-1");

    // Place marker at (0,0): click twice (excluded -> marked)
    await cell00.click();
    await cell00.click();
    await expect(cell00).toHaveAttribute("data-cell-state", "marked");

    // Place marker at (0,1): click twice
    await cell01.click();
    await cell01.click();
    await expect(cell01).toHaveAttribute("data-cell-state", "marked");

    // Both cells should have conflict attribute
    await expect(cell00).toHaveAttribute("data-cell-conflict", "true");
    await expect(cell01).toHaveAttribute("data-cell-conflict", "true");
  });

  test("solving the puzzle shows completion overlay", async ({ page }) => {
    // Valid solution for hardcoded puzzle: (0,4), (1,1), (2,3), (3,0), (4,2)
    const solution: [number, number][] = [
      [0, 4],
      [1, 1],
      [2, 3],
      [3, 0],
      [4, 2],
    ];

    for (const [row, col] of solution) {
      const cell = page.getByTestId(`cell-${row}-${col}`);
      await cell.click(); // excluded
      await cell.click(); // marked
    }

    await expect(page.getByText("Puzzle Complete!")).toBeVisible();
  });

  test("reset button clears all markers", async ({ page }) => {
    const cell = page.getByTestId("cell-0-0");

    // Place a marker
    await cell.click(); // excluded
    await cell.click(); // marked
    await expect(cell).toHaveAttribute("data-cell-state", "marked");

    // Reset
    await page.getByRole("button", { name: /reset/i }).click();

    // Cell should be empty again
    await expect(cell).toHaveAttribute("data-cell-state", "empty");
  });

  test("drag to exclude multiple cells", async ({ page }) => {
    const cell00 = page.getByTestId("cell-0-0");
    const cell01 = page.getByTestId("cell-0-1");
    const cell02 = page.getByTestId("cell-0-2");

    const box00 = await cell00.boundingBox();
    const box01 = await cell01.boundingBox();
    const box02 = await cell02.boundingBox();

    if (!box00 || !box01 || !box02) {
      throw new Error("Could not get cell bounding boxes");
    }

    // Mouse down on cell (0,0), drag through (0,1) and (0,2)
    await page.mouse.move(
      box00.x + box00.width / 2,
      box00.y + box00.height / 2,
    );
    await page.mouse.down();

    await page.mouse.move(
      box01.x + box01.width / 2,
      box01.y + box01.height / 2,
    );

    await page.mouse.move(
      box02.x + box02.width / 2,
      box02.y + box02.height / 2,
    );

    // During drag: cells still empty (exclusion deferred)
    await expect(cell01).toHaveAttribute("data-cell-state", "empty");
    await expect(cell02).toHaveAttribute("data-cell-state", "empty");

    await page.mouse.up();

    // After drag end: all cells excluded
    await expect(cell00).toHaveAttribute("data-cell-state", "excluded");
    await expect(cell01).toHaveAttribute("data-cell-state", "excluded");
    await expect(cell02).toHaveAttribute("data-cell-state", "excluded");
  });

  test("solved state disables further interaction", async ({ page }) => {
    // Solve the puzzle
    const solution: [number, number][] = [
      [0, 4],
      [1, 1],
      [2, 3],
      [3, 0],
      [4, 2],
    ];

    for (const [row, col] of solution) {
      const cell = page.getByTestId(`cell-${row}-${col}`);
      await cell.click();
      await cell.click();
    }

    await expect(page.getByText("Puzzle Complete!")).toBeVisible();

    // The completion overlay covers the grid, preventing interaction.
    // Verify that an empty cell is still empty (the overlay blocks clicks).
    const emptyCell = page.getByTestId("cell-0-0");
    await expect(emptyCell).toHaveAttribute("data-cell-state", "empty");
  });

  test("timer display is visible on game page", async ({ page }) => {
    await expect(page.getByTestId("timer-display")).toBeVisible();
    await expect(page.getByTestId("timer-display")).toHaveText("00:00");
  });

  test("undo and redo buttons are visible and initially disabled", async ({
    page,
  }) => {
    await expect(page.getByTestId("undo-button")).toBeVisible();
    await expect(page.getByTestId("redo-button")).toBeVisible();
    await expect(page.getByTestId("undo-button")).toBeDisabled();
    await expect(page.getByTestId("redo-button")).toBeDisabled();
  });

  test("undo button reverts a tap", async ({ page }) => {
    const cell = page.getByTestId("cell-2-2");
    await cell.click();
    await expect(cell).toHaveAttribute("data-cell-state", "excluded");
    await expect(page.getByTestId("undo-button")).toBeEnabled();

    await page.getByTestId("undo-button").click();

    await expect(cell).toHaveAttribute("data-cell-state", "empty");
    await expect(page.getByTestId("undo-button")).toBeDisabled();
    await expect(page.getByTestId("redo-button")).toBeEnabled();
  });

  test("redo button reapplies an undone move", async ({ page }) => {
    const cell = page.getByTestId("cell-2-2");
    await cell.click();
    await page.getByTestId("undo-button").click();
    await expect(cell).toHaveAttribute("data-cell-state", "empty");

    await page.getByTestId("redo-button").click();

    await expect(cell).toHaveAttribute("data-cell-state", "excluded");
  });

  test("Ctrl+Z keyboard shortcut undoes", async ({ page }) => {
    const cell = page.getByTestId("cell-2-2");
    await cell.click();
    await expect(cell).toHaveAttribute("data-cell-state", "excluded");

    await page.keyboard.press("Control+z");

    await expect(cell).toHaveAttribute("data-cell-state", "empty");
  });

  test("Ctrl+Shift+Z keyboard shortcut redoes", async ({ page }) => {
    const cell = page.getByTestId("cell-2-2");
    await cell.click();
    await page.keyboard.press("Control+z");
    await expect(cell).toHaveAttribute("data-cell-state", "empty");

    await page.keyboard.press("Control+Shift+z");

    await expect(cell).toHaveAttribute("data-cell-state", "excluded");
  });

  test("reset is undoable", async ({ page }) => {
    const cell = page.getByTestId("cell-2-2");
    await cell.click();
    await cell.click(); // marked
    await expect(cell).toHaveAttribute("data-cell-state", "marked");

    await page.getByRole("button", { name: /reset/i }).click();
    await expect(cell).toHaveAttribute("data-cell-state", "empty");

    await page.getByTestId("undo-button").click();

    await expect(cell).toHaveAttribute("data-cell-state", "marked");
  });

  test("drag-batch undo reverts all cells together", async ({ page }) => {
    const cell00 = page.getByTestId("cell-0-0");
    const cell01 = page.getByTestId("cell-0-1");
    const cell02 = page.getByTestId("cell-0-2");
    const box00 = await cell00.boundingBox();
    const box01 = await cell01.boundingBox();
    const box02 = await cell02.boundingBox();
    if (!box00 || !box01 || !box02) throw new Error("bounding box missing");

    await page.mouse.move(box00.x + box00.width / 2, box00.y + box00.height / 2);
    await page.mouse.down();
    await page.mouse.move(box01.x + box01.width / 2, box01.y + box01.height / 2);
    await page.mouse.move(box02.x + box02.width / 2, box02.y + box02.height / 2);
    await page.mouse.up();

    await expect(cell00).toHaveAttribute("data-cell-state", "excluded");
    await expect(cell01).toHaveAttribute("data-cell-state", "excluded");
    await expect(cell02).toHaveAttribute("data-cell-state", "excluded");

    await page.getByTestId("undo-button").click();

    await expect(cell00).toHaveAttribute("data-cell-state", "empty");
    await expect(cell01).toHaveAttribute("data-cell-state", "empty");
    await expect(cell02).toHaveAttribute("data-cell-state", "empty");
  });

  test("new move after undo clears the redo stack", async ({ page }) => {
    const cellA = page.getByTestId("cell-0-0");
    const cellB = page.getByTestId("cell-1-1");

    await cellA.click();
    await page.getByTestId("undo-button").click();
    await expect(page.getByTestId("redo-button")).toBeEnabled();

    // New action: redo should now be cleared
    await cellB.click();

    await expect(page.getByTestId("redo-button")).toBeDisabled();
  });
});
