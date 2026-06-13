import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import { tamaguiPlugin } from "@tamagui/vite-plugin";
import { VitePWA } from "vite-plugin-pwa";

// Vite runs its config under Node. Node's `process` global isn't
// declared here because we don't depend on @types/node (this file is
// the only consumer). Narrow declaration is enough.
declare const process: { env: Record<string, string | undefined> };

// PWA configuration (re-enabled for #116 — Vite 8 support shipped in
// vite-plugin-pwa@1.3.0 on 2026-05-05). `manifest: false` keeps
// public/manifest.json as the source of truth. `registerType:
// 'autoUpdate'` activates new SWs on next navigation without a UI
// prompt. navigateFallbackDenylist excludes /api/* so the SPA shell
// fallback doesn't swallow backend errors. Runtime caching for
// Google Fonts mirrors the original T-113 setup.

export default defineConfig(({ mode }) => ({
  // Resolves @app/@shared/@features/@theme/@storage aliases from
  // tsconfig.app.json so Vite + Vitest match tsc + IDE. @reign/core
  // resolves via the npm-workspace symlink + its package.json exports.
  resolve: {
    tsconfigPaths: true,
  },
  plugins: [
    react(),
    // Tamagui compiler — hoists styles at build time instead of computing
    // them at runtime. Pin held at @tamagui/vite-plugin@2.0.0-rc.34
    // because rc.35-42 ship a malformed peer dep (`vite: "*8.0.3"`,
    // upstream tamagui/tamagui#4008). See reign-game #207 for tracking.
    tamaguiPlugin({
      config: "./tamagui.config.ts",
      components: ["tamagui"],
    }),
    // The Tamagui plugin's config() hook returns `envPrefix: ["TAMAGUI_"]`,
    // which Vite's mergeConfig treats as REPLACING (not extending) the
    // default "VITE_" prefix. Result: `import.meta.env.VITE_*` becomes
    // undefined in the browser, which crashes ClerkProvider mounting in
    // src/app/providers.tsx and propagates as "useUser must be inside
    // <ClerkProvider />" wherever useUser() is called. Discovered while
    // debugging PR #209's CI integration-test failures — see reign-game
    // #207 + upstream tamagui/tamagui#4008 follow-up. The override below
    // appends VITE_ back to the prefix list so both the SPA's env vars
    // AND Tamagui's TAMAGUI_-prefixed compiler opts still reach the
    // browser.
    {
      name: "reign-restore-vite-env-prefix",
      enforce: "post",
      config() {
        return { envPrefix: ["VITE_", "TAMAGUI_"] };
      },
    },
    VitePWA({
      registerType: 'autoUpdate',
      manifest: false,
      workbox: {
        globPatterns: ['**/*.{js,css,html,ico,png,svg,woff2}'],
        navigateFallback: '/index.html',
        navigateFallbackDenylist: [/^\/api\//],
        runtimeCaching: [
          {
            urlPattern: /^https:\/\/fonts\.googleapis\.com/,
            handler: 'CacheFirst',
            options: {
              cacheName: 'google-fonts-stylesheets',
              expiration: { maxEntries: 10, maxAgeSeconds: 60 * 60 * 24 * 30 },
              cacheableResponse: { statuses: [200] },
            },
          },
          {
            urlPattern: /^https:\/\/fonts\.gstatic\.com/,
            handler: 'CacheFirst',
            options: {
              cacheName: 'google-fonts-webfonts',
              expiration: { maxEntries: 30, maxAgeSeconds: 60 * 60 * 24 * 365 },
              cacheableResponse: { statuses: [0, 200] },
            },
          },
        ],
      },
    }),
  ],
  // Per-library vendor chunks. Splitting the four big vendor
  // groups into their own chunks keeps each chunk under the 150 kB-gzip
  // budget and maximises browser-cache stability across deploys (a bump
  // to one library doesn't invalidate the others). Rolldown (Vite 8's
  // bundler) honours the Rollup-compat `manualChunks` callback.
  build: {
    rollupOptions: {
      output: {
        manualChunks(id: string) {
          if (!id.includes("node_modules")) return undefined;
          if (
            /node_modules\/(react|react-dom|react-router|react-router-dom|scheduler)\//.test(
              id,
            )
          ) {
            return "react";
          }
          if (/node_modules\/(tamagui|@tamagui)\//.test(id)) return "tamagui";
          if (/node_modules\/@clerk\//.test(id)) return "clerk";
          if (/node_modules\/@tanstack\//.test(id)) return "tanstack";
          return undefined;
        },
      },
    },
  },
  define: {
    __TEST_ATTRS__: mode !== "production",
  },
  server: {
    proxy: {
      // REIGN_API_TARGET overrides the /api/* proxy target. The R-06B
      // e2e stack runs a second Vite on :5183 with this set to the
      // e2e backend on :5182, while dev keeps :5180 → :5181. Unset
      // for normal dev.
      "/api": process.env.REIGN_API_TARGET || "http://localhost:5181",
    },
  },
  test: {
    environment: "jsdom",
    globals: false,
    setupFiles: ["./src/test-setup.ts"],
    exclude: ["playwright/**", "node_modules/**"],
    // The default `forks` pool intermittently fails to spawn a
    // child_process under local load ("Failed to start forks worker"),
    // which propagates as 5-second test timeouts in unrelated files.
    // Worker threads have lower startup overhead and don't hit the
    // OS fork limit, so the suite stabilises.
    pool: "threads",
  },
}));
