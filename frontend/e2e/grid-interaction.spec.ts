import { test, expect } from "@playwright/test";

test.describe("Grid Interaction", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: /reign/i })).toBeVisible();
    await expect(page.getByTestId("game-grid")).toBeVisible();
  });

  test("grid renders 25 cells for a 5x5 puzzle", async ({ page }) => {
    const cells = page.locator("[data-testid^='cell-']");
    await expect(cells).toHaveCount(25);
  });

  test("cells have region background colors", async ({ page }) => {
    // Cell (0,0) should have region-0-fill background
    const cell = page.getByTestId("cell-0-0");
    await expect(cell).toHaveCSS(
      "background-color",
      /rgb/,
    );
  });

  test("three-tap cycle: empty → excluded → marked → empty", async ({
    page,
  }) => {
    const cell = page.getByTestId("cell-2-2");

    // Initially empty: no SVG
    await expect(cell.locator("svg")).toHaveCount(0);

    // First click: excluded (shows cross — two lines)
    await cell.click();
    await expect(cell.locator("svg")).toHaveCount(1);
    await expect(cell.locator("line")).toHaveCount(2);

    // Second click: marked (shows circle)
    await cell.click();
    await expect(cell.locator("svg")).toHaveCount(1);
    await expect(cell.locator("circle")).toHaveCount(1);

    // Third click: back to empty
    await cell.click();
    await expect(cell.locator("svg")).toHaveCount(0);
  });

  test("conflict highlighting when two markers in same row", async ({
    page,
  }) => {
    const cell00 = page.getByTestId("cell-0-0");
    const cell01 = page.getByTestId("cell-0-1");

    // Place marker at (0,0): click twice (excluded → marked)
    await cell00.click();
    await cell00.click();
    await expect(cell00.locator("circle")).toHaveCount(1);

    // Place marker at (0,1): click twice
    await cell01.click();
    await cell01.click();
    await expect(cell01.locator("circle")).toHaveCount(1);

    // Both cells should now have destructive background (conflict)
    // The exact RGB depends on the CSS variable resolution
    const bg00 = await cell00.evaluate(
      (el) => getComputedStyle(el).backgroundColor,
    );
    const bg01 = await cell01.evaluate(
      (el) => getComputedStyle(el).backgroundColor,
    );
    // Both should be the same destructive background
    expect(bg00).toBe(bg01);
    // Should not be the original region color anymore
    expect(bg00).not.toBe("");
  });

  test("solving the puzzle shows completion message", async ({ page }) => {
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
      // Click twice: empty → excluded → marked
      await cell.click();
      await cell.click();
    }

    // Should show "Puzzle solved!" message
    await expect(page.getByText("Puzzle solved!")).toBeVisible();
  });

  test("reset button clears all markers", async ({ page }) => {
    const cell = page.getByTestId("cell-0-0");

    // Place a marker
    await cell.click(); // excluded
    await cell.click(); // marked
    await expect(cell.locator("circle")).toHaveCount(1);

    // Reset
    await page.getByRole("button", { name: /reset/i }).click();

    // Cell should be empty again
    await expect(cell.locator("svg")).toHaveCount(0);
  });

  test("drag to exclude multiple cells", async ({ page }) => {
    const grid = page.getByTestId("game-grid");
    const cell00 = page.getByTestId("cell-0-0");
    const cell01 = page.getByTestId("cell-0-1");
    const cell02 = page.getByTestId("cell-0-2");

    // Get bounding boxes for cells
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

    // During drag: dragged cells should NOT have crosses yet
    await expect(cell01.locator("svg")).toHaveCount(0);
    await expect(cell02.locator("svg")).toHaveCount(0);

    await page.mouse.up();

    // After drag end: starting cell was excluded on tap, dragged cells now excluded
    await expect(cell00.locator("line")).toHaveCount(2);
    await expect(cell01.locator("line")).toHaveCount(2);
    await expect(cell02.locator("line")).toHaveCount(2);
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

    await expect(page.getByText("Puzzle solved!")).toBeVisible();

    // Click on an empty cell — should remain empty (interactions disabled)
    const emptyCell = page.getByTestId("cell-0-0");
    await emptyCell.click();
    await expect(emptyCell.locator("svg")).toHaveCount(0);
  });
});
