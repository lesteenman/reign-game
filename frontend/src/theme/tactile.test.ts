import { describe, expect, it } from "vitest";
import { tactileTheme } from "./tactile";

describe("tactileTheme", () => {
  it("has id 'tactile'", () => {
    expect(tactileTheme.id).toBe("tactile");
  });

  it("has name 'Tactile'", () => {
    expect(tactileTheme.name).toBe("Tactile");
  });

  it("has animation class names", () => {
    expect(tactileTheme.animations.placement).toBe("animate-placement");
    expect(tactileTheme.animations.conflict).toBe("animate-conflict");
    expect(tactileTheme.animations.completion).toBe("animate-completion");
  });

  it("has marker and exclusionMark components", () => {
    expect(typeof tactileTheme.marker).toBe("function");
    expect(typeof tactileTheme.exclusionMark).toBe("function");
  });
});
