import { renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ThemeProvider, useTheme } from "./ThemeContext";
import { tactileTheme } from "./tactile";
import type { ReactNode } from "react";

function wrapper({ children }: { children: ReactNode }) {
  return <ThemeProvider>{children}</ThemeProvider>;
}

describe("ThemeContext", () => {
  it("useTheme returns the Tactile theme by default", () => {
    const { result } = renderHook(() => useTheme(), { wrapper });
    expect(result.current.id).toBe("tactile");
    expect(result.current.name).toBe("Tactile");
  });

  it("provides the same theme object as tactileTheme", () => {
    const { result } = renderHook(() => useTheme(), { wrapper });
    expect(result.current).toBe(tactileTheme);
  });

  it("throws when useTheme is called outside ThemeProvider", () => {
    expect(() => {
      renderHook(() => useTheme());
    }).toThrow();
  });
});
