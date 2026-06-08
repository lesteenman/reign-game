import { createContext, useContext } from "react";
import type { ReactNode } from "react";
import type { Theme } from "@reign/core/theme";
import { tactileTheme } from "./tactile";

const ThemeContext = createContext<Theme | null>(null);

/** Provides the active theme to all descendants. */
export function ThemeProvider({ children }: { children: ReactNode }) {
  return (
    <ThemeContext.Provider value={tactileTheme}>
      {children}
    </ThemeContext.Provider>
  );
}

/** Returns the current theme. Must be called within a ThemeProvider. */
export function useTheme(): Theme {
  const theme = useContext(ThemeContext);
  if (theme === null) {
    throw new Error("useTheme must be used within a ThemeProvider");
  }
  return theme;
}
