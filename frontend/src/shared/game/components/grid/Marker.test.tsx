import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Marker } from "./Marker";

describe("Marker", () => {
  it("renders an SVG element", () => {
    const { container } = render(<Marker size={24} regionIndex={0} />);
    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("renders a rounded rect", () => {
    const { container } = render(<Marker size={24} regionIndex={0} />);
    const rect = container.querySelector("rect");
    expect(rect).toBeInTheDocument();
  });

  it("uses currentColor for fill", () => {
    const { container } = render(<Marker size={24} regionIndex={0} />);
    const rect = container.querySelector("rect");
    expect(rect).toHaveAttribute("fill", "currentColor");
  });

  it("applies the provided size to the SVG dimensions", () => {
    const { container } = render(<Marker size={32} regionIndex={2} />);
    const svg = container.querySelector("svg");
    expect(svg).toHaveAttribute("width", "32");
    expect(svg).toHaveAttribute("height", "32");
  });
});
