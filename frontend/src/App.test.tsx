import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import App from "./App";

test("renders Reign heading", () => {
  render(<App />);
  const heading = screen.getByRole("heading", { name: /reign/i });
  expect(heading).toBeInTheDocument();
});
