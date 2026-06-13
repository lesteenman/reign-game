# Frontend Performance Audit — June 2026

**Issue:** #134 · **Date measured:** 2026-06-10 · **Target:** acc (`https://reign.acc.steenman.me`)
**Status:** baseline established. Bundle budgets below are **proposed** (PO ratifies); CWV uses Google's
standard thresholds.

> **Lab, not field.** All Core Web Vitals here are **synthetic (Lighthouse lab)** measurements — they
> approximate but do not equal real-user field data. Real-user/RUM CWV (including true **INP**, which
> needs real interaction) is deferred to **#170**. Lighthouse reports **TBT** as the lab proxy for INP.

---

## Methodology

### Core Web Vitals (lab)

- **Tool:** Lighthouse `13.4.0` (via `npx lighthouse`), driving system **Google Chrome** (headless).
- **Config:** Lighthouse defaults — **mobile** form factor, **simulated Slow 4G** throttling
  (rtt 150 ms, ~1.6 Mbps, **4× CPU slowdown**). This is the intentionally pessimistic mobile profile;
  desktop / fast-network numbers are better.
- **Runs:** each route run **3×**; the **median** of the 3 runs is reported (to tame run-to-run variance).
- **Command (per route, repeated 3×):**

  ```bash
  CHROME_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
  npx lighthouse "<url>" \
    --only-categories=performance \
    --output=json --output=html \
    --chrome-flags="--headless=new --no-sandbox"
  ```

- **Routes measured:**
  - Landing — `https://reign.acc.steenman.me/`
  - Daily play — `https://reign.acc.steenman.me/play?flow=daily`
- **Routes NOT lab-measurable:** **post-completion** is interaction-gated (only reachable after a daily
  puzzle is solved). Lighthouse cold-loads a URL and cannot drive that interaction, so it is **not
  lab-measurable here** — deferred to RUM/**#170**. No numbers are faked for it.
- Raw Lighthouse JSON/HTML were written to a tmp dir outside the repo (`/tmp/lh-134`); only the
  summarized medians are committed.

### Bundle size

- **Build:** `cd frontend && npm ci && npm run build` (Vite 8, `tsc -b && vite build`).
- **Sizes:** raw = `wc -c` on each `frontend/dist` artifact; gzip = `gzip -c <file> | wc -c`.
- Vite's built-in reporter already warns on any chunk **> 500 kB** — that warning fires today.

### Delivery headers (acc)

Checked with `curl -I -H "Accept-Encoding: gzip, br"` against the deployed hashed JS asset to confirm
what bytes actually cross the wire (compression + cache headers), since that drives LCP independently of
the build output.

---

## Core Web Vitals — measured (median of 3) vs Google thresholds

Google thresholds: **LCP** good < 2.5 s / poor > 4.0 s · **CLS** good < 0.1 / poor > 0.25 ·
**INP** good < 200 ms / poor > 500 ms (field-only; **TBT** is the lab proxy, good < 200 ms).

### Landing (`/`)

| Metric | Median | Threshold | Verdict |
|---|---|---|---|
| Performance score | **44** / 100 | ≥ 90 good | 🔴 poor |
| LCP | **8.04 s** | < 2.5 s | 🔴 poor |
| FCP | **4.73 s** | < 1.8 s | 🔴 poor |
| TBT (INP lab proxy) | **812 ms** | < 200 ms | 🔴 poor |
| CLS | **0.00** | < 0.1 | 🟢 good |
| Speed Index | **5.15 s** | < 3.4 s | 🔴 poor |
| TTI | **8.23 s** | < 3.8 s | 🔴 poor |
| TTFB (server-response) | **21 ms** | < 800 ms | 🟢 good |
| INP (field) | not lab-measurable | < 200 ms | → #170 |

### Daily play (`/play?flow=daily`)

| Metric | Median | Threshold | Verdict |
|---|---|---|---|
| Performance score | **43** / 100 | ≥ 90 good | 🔴 poor |
| LCP | **8.19 s** | < 2.5 s | 🔴 poor |
| FCP | **4.74 s** | < 1.8 s | 🔴 poor |
| TBT (INP lab proxy) | **819 ms** | < 200 ms | 🔴 poor |
| CLS | **0.00** | < 0.1 | 🟢 good |
| Speed Index | **5.26 s** | < 3.4 s | 🔴 poor |
| TTI | **8.35 s** | < 3.8 s | 🔴 poor |
| TTFB (server-response) | **15 ms** | < 800 ms | 🟢 good |
| INP (field) | not lab-measurable | < 200 ms | → #170 |

**Reading the numbers.** Both routes are essentially identical, which is expected — the app ships one
monolithic JS bundle, so every route pays the same load+parse cost. The shape is consistent:

- **TTFB is excellent** (15–21 ms): CloudFront edge serves `index.html` from cache. This is **not** an
  infra-latency problem (that is #305).
- **CLS is perfect** (0.00): no layout shift.
- **Everything paint/interactivity is poor** and traces to one root cause: a single large,
  render-blocking JS bundle that is **not compressed over the wire** and ships ~498 KiB of unused JS for
  the first paint. Lighthouse diagnostics on the landing run confirm it:
  - `unused-javascript`: ~498 KiB unused (est. 2.08 s savings)
  - `render-blocking-insight`: ~850 ms
  - `cache-insight`: ~552 KiB (no long-cache header on the hashed asset)
  - `mainthread-work-breakdown`: 2.5 s · `bootup-time`: 1.6 s (JS parse/exec)

---

## Bundle size — production build (`frontend/dist`)

| Artifact | Raw | Gzip | Note |
|---|---|---|---|
| `assets/index-*.js` (app) | **553.3 kB** | **168.6 kB** | single app chunk — over Vite's 500 kB warn |
| `workbox-*.js` (SW runtime) | 21.9 kB | 7.4 kB | |
| `assets/workbox-window.prod.es5-*.js` | 5.7 kB | 2.3 kB | |
| `sw.js` | 1.7 kB | 0.9 kB | |
| `assets/index-*.css` | 2.4 kB | 0.9 kB | |
| `index.html` | 0.8 kB | 0.5 kB | |
| **Total JS** | **582.6 kB** | **~179.3 kB** | |
| **Total CSS** | **2.4 kB** | **0.9 kB** | |

**Observations:**

- There is effectively **one application chunk** — no route-level or vendor code-splitting. The whole app
  (engine, all features, Tamagui, Clerk, TanStack, lucide icons) loads before first paint on every route.
- CSS is tiny (Tamagui compiler is hoisting/extracting correctly — 2.4 kB is healthy).

### Proposed bundle budgets (PO ratifies)

Set just above the current baseline so they act as a regression guard now and tighten as remediation
lands. These are **proposals, not failures** — only the app chunk is flagged below because it already
trips Vite's own 500 kB warning.

| Budget | Proposed limit (gzip) | Proposed limit (raw) | Current | Status |
|---|---|---|---|---|
| **Total JS** (all chunks) | **190 kB** | 600 kB | 179.3 kB / 582.6 kB | within proposed budget |
| **Largest single JS chunk** | **150 kB** | 500 kB | **168.6 kB / 553.3 kB** | 🔴 over (the app chunk) |
| **Total CSS** | **15 kB** | 40 kB | 0.9 kB / 2.4 kB | comfortably within |

Rationale for the per-chunk budget being *below* current: the single app chunk is the headline problem.
A 150 kB-gzip per-chunk ceiling forces the code-split that fixes LCP/TBT, rather than blessing the
monolith. The total-JS budget (190 kB) is set just above today's number so splitting work doesn't
accidentally *grow* total shipped bytes.

---

## Reds / remediation (filed)

Each red below is filed as a `type:perf` remediation ticket. Items 1 + 2 (CloudFront delivery config)
were filed together as **#307**; item 3 → **#308**; item 4 → **#309**; the CI guard → **#310**.

1. **Enable gzip/brotli compression for static assets on acc/CloudFront** — filed as **#307**
   - *Metric:* LCP, FCP, Speed Index (all routes). *Rationale:* the deployed hashed JS asset is served
     **uncompressed** — 553 kB over the wire despite `Accept-Encoding: gzip, br` (`content-encoding` is
     absent). Compressing drops it to ~169 kB (~3.3×), the single highest-leverage LCP fix. This is
     content-delivery config (CloudFront/S3 frontend serving), not infra-latency (#305).

2. **Set long-lived `Cache-Control` on hashed (immutable) frontend assets** — filed as **#307** (with item 1)
   - *Metric:* repeat-visit LCP/load; `cache-insight` ≈ 552 KiB. *Rationale:* the hashed JS asset returns
     **no `Cache-Control` header**, so the browser can't cache the immutable bundle across visits.
     Hashed assets should be `Cache-Control: public, max-age=31536000, immutable`; `index.html` stays
     short/no-cache. (Pairs naturally with ticket 1.)

3. **Code-split the monolithic app bundle (route-level + vendor split)** — filed as **#308**
   - *Metric:* LCP, TBT, TTI, bundle per-chunk budget. *Rationale:* one ~553 kB chunk loads for every
     route; Lighthouse reports ~498 KiB **unused JS** on first paint and ~850 ms render-blocking. Split by
     route (lazy-load play/daily/admin) and isolate heavy vendors (Clerk, Tamagui, lucide) so the landing
     first-paint ships far less JS. This is what brings the largest chunk under the proposed 150 kB-gzip
     budget.

4. **Reduce main-thread blocking / JS bootup time** — filed as **#309**
   - *Metric:* TBT (~812 ms, threshold 200 ms), TTI (~8.2 s). *Rationale:* 2.5 s main-thread work and
     1.6 s bootup come from parsing/executing the full bundle up front. Largely resolved by ticket 3
     (code-splitting), but track separately to verify TBT lands under 200 ms after the split; may also need
     deferring non-critical init (e.g. Clerk, service-worker registration) off the critical path.

> #307 is the fastest, highest-leverage fix (delivery config, no app code change). #308 + #309 are the
> structural follow-ups. A **CWV/bundle-size CI guard** to enforce the budgets above was filed as **#310**.

---

## Deferred (out of scope for #134)

- **Infra-latency** — Lambda cold-start budgets, DynamoDB p99, CloudFront cache-hit ratio → **#305**
  (needs real traffic; builds on #133's CloudWatch dashboards). Note: TTFB measured here is excellent, so
  the frontend reds above are **not** infra-latency.
- **Field / RUM Core Web Vitals** — real-user LCP/CLS and true **INP** (needs `web-vitals` instrumentation
  + real users) → **#170**. The post-completion route, not lab-measurable, falls here too.

---

## Post-remediation re-measure — 2026-06-13 (#307 + #308 + #309 shipped)

Re-ran the landing-route Lighthouse pass (same tool/config: Lighthouse 13.4.0, mobile / Slow-4G /
4× CPU, median of 3) against **acc** after #307 (CloudFront compression + immutable Cache-Control) and
#308 (route + vendor code-split) deployed.

| Metric | Baseline (2026-06-10) | After #307+#308 | Target |
|---|---|---|---|
| Performance score | 44 | **75–91** (median 83) | ≥ 90 |
| TBT | 812 ms | **0 ms** | < 200 ms ✓ |
| LCP | 8.04 s | **3.8 s** | < 2.5 s |

**TBT is at the lab floor (0 ms)** — the code-split (#308) removed the monolithic up-front parse/exec
that drove the 812 ms, so main-thread blocking is now well under the 200 ms target. **#309** then defers
service-worker registration off the critical render path (`registerSW({ immediate: false })`, registers
on `load`) — a correctness/best-practice change verified to still register the SW; its marginal TBT
effect is below lab resolution because #308 already floored TBT. No Clerk-deferral follow-up is needed
(TBT is under target). LCP improved markedly (8.0 → 3.8 s) but is still above the 2.5 s "good" line —
remaining LCP headroom is render/network, not main-thread, and is field-measured via #170.
