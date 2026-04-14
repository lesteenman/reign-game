import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

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
      "/puzzles": "http://localhost:5181",
    },
  },
  test: {
    environment: "jsdom",
    globals: false,
    setupFiles: ["./src/test-setup.ts"],
    exclude: ["e2e/**", "node_modules/**"],
  },
}));
