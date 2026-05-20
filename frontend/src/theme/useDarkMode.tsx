import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";

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

interface DarkModeContextValue {
  isDark: boolean;
  toggle: () => void;
}

const DarkModeContext = createContext<DarkModeContextValue | null>(null);

/**
 * Owns the dark-mode state for the entire app. Reads
 * `prefers-color-scheme: dark` as the default with a manual override
 * persisted to localStorage. Toggles the `.dark` class on
 * `document.documentElement` and updates `<meta name="theme-color">`
 * so the device status bar / PWA shell tracks the active theme
 * (otherwise the manifest's static `theme_color` wins and dark-mode
 * users get a bright status bar — see #181).
 *
 * State is owned by THIS provider, not by `useDarkMode` callers, so
 * multiple sites consuming the hook (e.g. `PageShell` for the header
 * toggle UI AND `app/providers.tsx`'s `TamaguiThemedProvider` for
 * feeding Tamagui's active theme) share one source of truth. Without
 * this, each `useDarkMode()` call would own its own `useState`, and
 * the toggle UI's set would not reach Tamagui's theme until a full
 * remount (#208 code-review finding).
 */
export function DarkModeProvider({ children }: { children: ReactNode }) {
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

  return (
    <DarkModeContext.Provider value={{ isDark, toggle }}>
      {children}
    </DarkModeContext.Provider>
  );
}

/**
 * Reads the active dark-mode state + toggle from `DarkModeProvider`.
 * Must be called inside a `<DarkModeProvider>` (mounted by
 * `app/providers.tsx`).
 */
export function useDarkMode(): DarkModeContextValue {
  const ctx = useContext(DarkModeContext);
  if (ctx === null) {
    throw new Error("useDarkMode must be called inside a <DarkModeProvider>");
  }
  return ctx;
}
