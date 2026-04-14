import { render, screen, cleanup } from "@testing-library/react";
import { expect, test, vi, beforeEach, afterEach } from "vitest";
import App, { FALLBACK_PUZZLE } from "./App";

const originalFetch = globalThis.fetch;

beforeEach(() => {
  globalThis.fetch = vi.fn().mockResolvedValue({
    ok: true,
    json: () => Promise.resolve(FALLBACK_PUZZLE),
  });
});

afterEach(() => {
  cleanup();
  globalThis.fetch = originalFetch;
});

test("shows loading state initially", () => {
  render(<App />);
  expect(screen.getByTestId("loading-state")).toBeInTheDocument();
});

test("renders Reign heading", async () => {
  render(<App />);
  // Wait for puzzle to load, then verify heading
  await screen.findByTestId("game-grid");
  const heading = screen.getByRole("heading", { name: /reign/i });
  expect(heading).toBeInTheDocument();
});

test("renders the game grid after loading", async () => {
  render(<App />);
  const grid = await screen.findByTestId("game-grid");
  expect(grid).toBeInTheDocument();
});

test("renders reset button after loading", async () => {
  render(<App />);
  await screen.findByTestId("game-grid");
  expect(screen.getByRole("button", { name: /reset/i })).toBeInTheDocument();
});

test("shows error state on fetch failure", async () => {
  globalThis.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

  render(<App />);
  const error = await screen.findByTestId("error-state");
  expect(error).toBeInTheDocument();
  expect(screen.getByText(/network error/i)).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /try again/i })).toBeInTheDocument();
});
