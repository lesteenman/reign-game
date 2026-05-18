import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useDarkMode } from "./useDarkMode";

describe("useDarkMode", () => {
  const localStorageStore: Record<string, string> = {};

  beforeEach(() => {
    // Clear localStorage mock
    Object.keys(localStorageStore).forEach(
      (key) => delete localStorageStore[key],
    );

    vi.spyOn(Storage.prototype, "getItem").mockImplementation(
      (key: string) => localStorageStore[key] ?? null,
    );
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(
      (key: string, value: string) => {
        localStorageStore[key] = value;
      },
    );

    // Default: light mode preference
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      configurable: true,
      value: vi.fn().mockReturnValue({
        matches: false,
        media: "(prefers-color-scheme: dark)",
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      }),
    });

    // Start with clean document
    document.documentElement.classList.remove("dark");

    // Reset the theme-color meta tag. Tests that exercise the
    // meta-tag wiring arrange one explicitly via setupMeta().
    document
      .querySelectorAll('meta[name="theme-color"]')
      .forEach((el) => el.remove());
  });

  afterEach(() => {
    vi.restoreAllMocks();
    document
      .querySelectorAll('meta[name="theme-color"]')
      .forEach((el) => el.remove());
  });

  // Helper — installs the <meta name="theme-color" content="..."> tag
  // that index.html ships in production. Tests that don't call this
  // verify the hook is tolerant when the tag is absent.
  function setupMeta(initial = "#F8F6F3"): HTMLMetaElement {
    const meta = document.createElement("meta");
    meta.setAttribute("name", "theme-color");
    meta.setAttribute("content", initial);
    document.head.appendChild(meta);
    return meta;
  }

  it("defaults to light mode when system prefers light", () => {
    const { result } = renderHook(() => useDarkMode());
    expect(result.current.isDark).toBe(false);
    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });

  it("defaults to dark mode when system prefers dark", () => {
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      configurable: true,
      value: vi.fn().mockReturnValue({
        matches: true,
        media: "(prefers-color-scheme: dark)",
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      }),
    });

    const { result } = renderHook(() => useDarkMode());
    expect(result.current.isDark).toBe(true);
    expect(document.documentElement.classList.contains("dark")).toBe(true);
  });

  it("toggles dark mode and updates the document class", () => {
    const { result } = renderHook(() => useDarkMode());
    expect(result.current.isDark).toBe(false);

    act(() => {
      result.current.toggle();
    });

    expect(result.current.isDark).toBe(true);
    expect(document.documentElement.classList.contains("dark")).toBe(true);
  });

  it("persists preference to localStorage", () => {
    const { result } = renderHook(() => useDarkMode());

    act(() => {
      result.current.toggle();
    });

    expect(localStorageStore["reign-dark-mode"]).toBe("true");
  });

  it("restores preference from localStorage", () => {
    localStorageStore["reign-dark-mode"] = "true";

    const { result } = renderHook(() => useDarkMode());
    expect(result.current.isDark).toBe(true);
    expect(document.documentElement.classList.contains("dark")).toBe(true);
  });

  it("updates <meta name=\"theme-color\"> to the light color on mount when light", () => {
    // Arrange
    const meta = setupMeta("#some-stale-value");

    // Act
    renderHook(() => useDarkMode());

    // Assert
    expect(meta.content).toBe("#F8F6F3");
  });

  it("updates <meta name=\"theme-color\"> to the dark color on mount when dark", () => {
    // Arrange
    localStorageStore["reign-dark-mode"] = "true";
    const meta = setupMeta("#some-stale-value");

    // Act
    renderHook(() => useDarkMode());

    // Assert
    expect(meta.content).toBe("#161310");
  });

  it("updates <meta name=\"theme-color\"> when the user toggles dark mode", () => {
    // Arrange
    const meta = setupMeta();
    const { result } = renderHook(() => useDarkMode());
    expect(meta.content).toBe("#F8F6F3");

    // Act
    act(() => {
      result.current.toggle();
    });

    // Assert
    expect(meta.content).toBe("#161310");
  });

  it("does not throw when no theme-color meta tag is present", () => {
    // Arrange — no setupMeta(); the document has no theme-color tag.

    // Act + Assert — render the hook and toggle without crashing.
    const { result } = renderHook(() => useDarkMode());
    expect(() => {
      act(() => {
        result.current.toggle();
      });
    }).not.toThrow();
  });
});
