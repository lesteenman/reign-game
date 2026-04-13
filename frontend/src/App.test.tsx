import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import App from "./App";

test("renders Reign heading", () => {
  render(<App />);
  const heading = screen.getByRole("heading", { name: /reign/i });
  expect(heading).toBeInTheDocument();
});

test("renders the game grid", () => {
  render(<App />);
  const grids = screen.getAllByTestId("game-grid");
  expect(grids.length).toBeGreaterThanOrEqual(1);
});

test("renders reset button", () => {
  render(<App />);
  const buttons = screen.getAllByText(/reset/i);
  expect(buttons.length).toBeGreaterThanOrEqual(1);
});
