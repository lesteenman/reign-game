/**
 * Generates PWA icons by rendering a 2x2 Tactile-themed mini grid
 * via Playwright and screenshotting to PNG.
 *
 * Run: npx playwright test scripts/generate-icons.ts
 * Or:  npx tsx scripts/generate-icons.ts
 */
import { chromium } from '@playwright/test';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const SIZES = [192, 512];

// 2x2 grid: region 0 = 3 cells (top-left, top-right, bottom-left), region 1 = 1 cell (bottom-right)
// Marker at top-left (region 0), exclusion dot at bottom-right (region 1)
function buildHTML(size: number): string {
  const padding = Math.round(size * 0.08);
  const gridArea = size - padding * 2;
  const borderWidth = Math.max(2, Math.round(size * 0.012));
  const regionBorderWidth = Math.max(2.5, Math.round(size * 0.015));
  const cellSize = Math.floor((gridArea - borderWidth * 2) / 2);
  const radius = Math.round(size * 0.06);

  // Marker: rounded square
  const markerArea = cellSize * 0.6;
  const markerPad = markerArea * 0.18;
  const markerSide = markerArea - markerPad * 2;
  const markerRx = 3;

  // Exclusion: small dot
  const exArea = cellSize * 0.6;
  const dotR = exArea * 0.08;

  return `<!DOCTYPE html>
<html><head><style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  width: ${size}px; height: ${size}px;
  background: #F8F6F3;
  display: flex; align-items: center; justify-content: center;
}
.grid {
  display: inline-grid;
  grid-template-columns: ${cellSize}px ${cellSize}px;
  grid-template-rows: ${cellSize}px ${cellSize}px;
  border: ${borderWidth}px solid #2D2A26;
  border-radius: ${radius}px;
  box-shadow: 0 ${Math.round(size * 0.012)}px 0 #2D2A26;
  overflow: hidden;
  position: relative;
}
.cell {
  width: ${cellSize}px; height: ${cellSize}px;
  display: flex; align-items: center; justify-content: center;
  border: 0.5px solid rgba(0,0,0,0.07);
}
.r0 { background: #93C5FD; }
.r1 { background: #FCD34D; }
/* Region border overlay */
.overlay {
  position: absolute; top: 0; left: 0; pointer-events: none;
}
</style></head><body>
<div class="grid">
  <div class="cell r0">
    <svg width="${markerArea}" height="${markerArea}" viewBox="0 0 ${markerArea} ${markerArea}">
      <rect x="${markerPad}" y="${markerPad}" width="${markerSide}" height="${markerSide}" rx="${markerRx}" fill="#2D2A26"/>
    </svg>
  </div>
  <div class="cell r0"></div>
  <div class="cell r0"></div>
  <div class="cell r1">
    <svg width="${exArea}" height="${exArea}" viewBox="0 0 ${exArea} ${exArea}">
      <circle cx="${exArea / 2}" cy="${exArea / 2}" r="${dotR}" fill="#2D2A26"/>
    </svg>
  </div>
  <svg class="overlay" width="${cellSize * 2}" height="${cellSize * 2}" viewBox="0 0 ${cellSize * 2} ${cellSize * 2}">
    <!-- Vertical: between col 0 and col 1, only bottom half (region boundary) -->
    <line x1="${cellSize}" y1="${cellSize - regionBorderWidth / 2}" x2="${cellSize}" y2="${cellSize * 2}" stroke="#2D2A26" stroke-width="${regionBorderWidth}"/>
    <!-- Horizontal: between row 0 and row 1, only right half (region boundary) -->
    <line x1="${cellSize - regionBorderWidth / 2}" y1="${cellSize}" x2="${cellSize * 2}" y2="${cellSize}" stroke="#2D2A26" stroke-width="${regionBorderWidth}"/>
  </svg>
</div>
</body></html>`;
}

async function main() {
  const browser = await chromium.launch();

  for (const size of SIZES) {
    const page = await browser.newPage({
      viewport: { width: size, height: size },
      deviceScaleFactor: 1,
    });

    await page.setContent(buildHTML(size));
    await page.waitForTimeout(100);

    const outPath = path.resolve(
      __dirname,
      `../public/icons/icon-${size}.png`,
    );
    await page.screenshot({ path: outPath, omitBackground: false });
    console.log(`Generated ${outPath}`);
    await page.close();
  }

  await browser.close();
}

main().catch(console.error);
