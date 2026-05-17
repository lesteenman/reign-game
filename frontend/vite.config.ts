import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// Vite runs its config under Node. Node's `process` global isn't
// declared here because we don't depend on @types/node (this file is
// the only consumer). Narrow declaration is enough.
declare const process: { env: Record<string, string | undefined> };

// PWA plugin (vite-plugin-pwa) removed — waiting for Vite 8 support:
// https://github.com/vite-pwa/vite-plugin-pwa/issues/923
// Manifest and icons are in place; service worker will be re-added
// when the plugin ships Vite 8 compatibility.

export default defineConfig(({ mode }) => ({
  plugins: [react(), tailwindcss()],
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
