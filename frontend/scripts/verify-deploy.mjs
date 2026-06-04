// Post-deploy verification for the acc deployment gate (#241).
// Probes the live deployed site — no secrets needed. Run as the final
// step of the CD deploy job; a non-zero exit fails the job, which makes
// the native GitHub Deployment auto-mark `failure`.
//
//   node scripts/verify-deploy.mjs https://reign.acc.steenman.me
//
// Checks (medium depth, per #241 DoR):
//   1. The 6 security headers are present on `/`.
//   2. GET /api/health returns 200.
//   3. The deployed frontend loads with no SAME-ORIGIN console errors or
//      uncaught exceptions (third-party CDN failures — e.g. Google Fonts
//      — are runner-network-dependent, not deploy defects, so they're
//      ignored) AND actually reaches its own API: at least one
//      same-origin /api/* response, all < 400 (a stale bundle with no
//      x-api-key would 403 — the #225-class catch).
import { chromium } from 'playwright';

const BASE = (process.argv[2] || 'https://reign.acc.steenman.me').replace(/\/$/, '');
const REQUIRED_HEADERS = [
  'strict-transport-security',
  'x-content-type-options',
  'x-frame-options',
  'referrer-policy',
  'content-security-policy',
  'permissions-policy',
];

const fail = (msg) => {
  console.error(`✗ ${msg}`);
  process.exitCode = 1;
};

console.log(`=== Post-deploy verification: ${BASE} ===`);

// 1. Security headers on /
const rootRes = await fetch(BASE + '/', { redirect: 'manual' });
const missing = REQUIRED_HEADERS.filter((h) => !rootRes.headers.has(h));
if (missing.length) fail(`missing security headers: ${missing.join(', ')}`);
else console.log(`✓ all 6 security headers present`);

// 2. Backend health
const health = await fetch(BASE + '/api/health');
if (health.status !== 200) fail(`/api/health returned ${health.status}, expected 200`);
else console.log(`✓ /api/health 200`);

// 3. Live frontend -> API wire + no console errors
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
const baseHost = new URL(BASE).host;
const appErrors = []; // same-origin console errors + uncaught exceptions — deploy-relevant
const ignoredErrors = []; // third-party resource failures — runner-network-dependent, not gated
const apiResponses = [];
page.on('pageerror', (e) => appErrors.push(`pageerror: ${e.message.split('\n')[0]}`));
page.on('console', (m) => {
  if (m.type() !== 'error') return;
  let host = '';
  try {
    host = new URL(m.location()?.url || '').host;
  } catch {
    /* ignore unparseable/relative URLs */
  }
  if (host && host === baseHost) appErrors.push(`console: ${m.text().slice(0, 150)} (${m.location().url})`);
  else ignoredErrors.push(`${m.text().slice(0, 80)} (${host || 'no-origin'})`);
});
page.on('response', (r) => {
  try {
    const u = new URL(r.url());
    if (u.host === baseHost && u.pathname.startsWith('/api/')) {
      apiResponses.push({ path: u.pathname, status: r.status() });
    }
  } catch {
    /* ignore unparseable/relative URLs */
  }
});

// '/play?flow=daily' is the real daily entry (router.tsx) -> DailyFlow,
// which makes a keyed /api/daily/* call. A stale bundle without x-api-key
// would 403/401 there — the #225-class catch. Landing ('/') only fires
// the keyless health probe, so it can't catch a missing key on its own.
for (const path of ['/', '/play?flow=daily']) {
  try {
    await page.goto(BASE + path, { waitUntil: 'networkidle', timeout: 45000 });
    await page.waitForTimeout(2500);
  } catch (e) {
    fail(`navigation to ${path} failed: ${e.message.split('\n')[0]}`);
    break;
  }
}
await browser.close();

console.log(`  observed /api/* responses: ${apiResponses.map((r) => `${r.path}=${r.status}`).join(', ') || '(none)'}`);
if (ignoredErrors.length) console.log(`  (ignored ${ignoredErrors.length} third-party/originless error(s): ${ignoredErrors.join('; ')})`);
if (appErrors.length) fail(`same-origin app errors: ${appErrors.length}\n    ${appErrors.join('\n    ')}`);
else console.log(`✓ no same-origin console/page errors`);

if (apiResponses.length === 0) fail(`no same-origin /api/* request observed — the frontend never reached its API`);
else {
  const bad = apiResponses.filter((r) => r.status >= 400);
  if (bad.length) fail(`/api/* responses >= 400: ${bad.map((r) => `${r.path}=${r.status}`).join(', ')}`);
  else console.log(`✓ frontend reached its API (${apiResponses.length} call(s), all < 400)`);
}

console.log(process.exitCode ? '\nFAIL' : '\nPASS');
