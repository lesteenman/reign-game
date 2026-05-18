import { useCallback, useEffect, useState } from "react";

const STORAGE_KEY = "reign-dark-mode";

// Keep these in sync with --color-background in frontend/src/index.css
// (`:root` for light, `.dark` for dark). The light value also matches
// frontend/public/manifest.json's `theme_color` and the static default
// in frontend/index.html (#181).
const THEME_COLOR_LIGHT = "#F8F6F3";
const THEME_COLOR_DARK = "#161310";

function getInitialDark(): boolean {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored !== null) {
    return stored === "true";
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

/**
 * Manages dark mode state.
 *
 * Respects `prefers-color-scheme: dark` as the default, with a manual
 * override persisted to localStorage. Adds/removes the `.dark` class
 * on `document.documentElement` and updates `<meta name="theme-color">`
 * so the device status bar / PWA shell tracks the active theme
 * (otherwise the manifest's static `theme_color` wins and dark-mode
 * users get a bright status bar — see #181).
 */
export function useDarkMode(): { isDark: boolean; toggle: () => void } {
  const [isDark, setIsDark] = useState(getInitialDark);

  useEffect(() => {
    if (isDark) {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }

    const meta = document.querySelector<HTMLMetaElement>(
      'meta[name="theme-color"]',
    );
    if (meta) {
      meta.content = isDark ? THEME_COLOR_DARK : THEME_COLOR_LIGHT;
    }
  }, [isDark]);

  const toggle = useCallback(() => {
    setIsDark((prev) => {
      const next = !prev;
      localStorage.setItem(STORAGE_KEY, String(next));
      return next;
    });
  }, []);

  return { isDark, toggle };
}
