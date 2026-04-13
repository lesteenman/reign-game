import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ExclusionMark } from "./ExclusionMark";

describe("ExclusionMark", () => {
  it("renders an SVG element", () => {
    const { container } = render(<ExclusionMark size={24} />);
    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("renders two diagonal lines forming a cross", () => {
    const { container } = render(<ExclusionMark size={24} />);
    const lines = container.querySelectorAll("line");
    expect(lines).toHaveLength(2);
  });

  it("uses currentColor for stroke", () => {
    const { container } = render(<ExclusionMark size={24} />);
    const lines = container.querySelectorAll("line");
    for (const line of lines) {
      expect(line).toHaveAttribute("stroke", "currentColor");
    }
  });

  it("applies the provided size to the SVG dimensions", () => {
    const { container } = render(<ExclusionMark size={16} />);
    const svg = container.querySelector("svg");
    expect(svg).toHaveAttribute("width", "16");
    expect(svg).toHaveAttribute("height", "16");
  });
});
