// Bundle-size budget guard. Walks frontend/dist, gzips every JS and CSS
// artifact at a fixed level (matching `gzip -c`), and fails when the
// gzipped sizes exceed the ratified budgets in frontend/bundle-budgets.json.
//
//   node scripts/check-bundle-budget.mjs
//
// Budgets (gzip kB): largest single JS chunk, total JS, total CSS. A breach
// exits 1 (which budget + overage), keeping the LCP/TBT win from regressing.
import { readFileSync, readdirSync, statSync, existsSync } from 'node:fs';
import { join, extname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

const scriptDir = fileURLToPath(new URL('.', import.meta.url));
const distDir = join(scriptDir, '..', 'dist');
const budgetsPath = join(scriptDir, '..', 'bundle-budgets.json');

// Recursively collect every file under dir.
function walk(dir) {
  const out = [];
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      out.push(...walk(full));
    } else {
      out.push(full);
    }
  }
  return out;
}

// Fixed level 6 matches `gzip -c` (the audit tool) for determinism
// across local and CI.
function gzipKb(file) {
  return gzipSync(readFileSync(file), { level: 6 }).length / 1024;
}

if (!existsSync(distDir) || walk(distDir).length === 0) {
  console.error('dist is missing or empty — run `npm run build` first.');
  process.exit(1);
}

const files = walk(distDir).filter((f) => {
  if (f.endsWith('.map')) return false;
  const ext = extname(f);
  return ext === '.js' || ext === '.css';
});

const measured = files
  .map((f) => ({ path: f.slice(distDir.length + 1), ext: extname(f), kb: gzipKb(f) }))
  .sort((a, b) => b.kb - a.kb);

const jsFiles = measured.filter((m) => m.ext === '.js');
const cssFiles = measured.filter((m) => m.ext === '.css');

const largestChunkKb = jsFiles.reduce((max, m) => Math.max(max, m.kb), 0);
const totalJsKb = jsFiles.reduce((sum, m) => sum + m.kb, 0);
const totalCssKb = cssFiles.reduce((sum, m) => sum + m.kb, 0);

const budgets = JSON.parse(readFileSync(budgetsPath, 'utf8'));

// Per-file table, sorted largest-first.
console.log('Bundle artifacts (gzip):');
for (const m of measured) {
  console.log(`  ${m.kb.toFixed(2).padStart(8)} kB  ${m.path}`);
}
console.log('');

const checks = [
  { label: 'Largest JS chunk', actual: largestChunkKb, limit: budgets.largestChunkKb },
  { label: 'Total JS', actual: totalJsKb, limit: budgets.totalJsKb },
  { label: 'Total CSS', actual: totalCssKb, limit: budgets.totalCssKb },
];

let failed = false;
for (const c of checks) {
  const pass = c.actual <= c.limit;
  const status = pass ? 'PASS' : 'FAIL';
  let line = `  [${status}] ${c.label}: ${c.actual.toFixed(2)} kB / ${c.limit} kB`;
  if (!pass) {
    failed = true;
    line += ` — over by ${(c.actual - c.limit).toFixed(2)} kB`;
  }
  console.log(line);
}

process.exit(failed ? 1 : 0);
