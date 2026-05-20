import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ExclusionMark } from "./ExclusionMark";

describe("ExclusionMark", () => {
  it("renders an SVG element", () => {
    const { container } = render(<ExclusionMark size={24} />);
    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("renders a small circle dot", () => {
    const { container } = render(<ExclusionMark size={24} />);
    const circle = container.querySelector("circle");
    expect(circle).toBeInTheDocument();
  });

  it("uses currentColor for fill", () => {
    const { container } = render(<ExclusionMark size={24} />);
    const circle = container.querySelector("circle");
    expect(circle).toHaveAttribute("fill", "currentColor");
  });

  it("applies the provided size to the SVG dimensions", () => {
    const { container } = render(<ExclusionMark size={16} />);
    const svg = container.querySelector("svg");
    expect(svg).toHaveAttribute("width", "16");
    expect(svg).toHaveAttribute("height", "16");
  });
});
