import { useCallback, useEffect, useState } from "react";

const STORAGE_KEY = "reign-dark-mode";

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
 * on `document.documentElement`.
 */
export function useDarkMode(): { isDark: boolean; toggle: () => void } {
  const [isDark, setIsDark] = useState(getInitialDark);

  useEffect(() => {
    if (isDark) {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
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
