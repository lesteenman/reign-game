import { useState, useLayoutEffect, useEffect, useCallback } from "react";
import { ThemeProvider } from "./theme/ThemeContext";
import { Grid } from "./components/grid/Grid";
import { useGame } from "./hooks/useGame";
import type { PuzzleData } from "./engine/types";
import { generatePuzzle } from "./services/puzzleService";

/**
 * Hardcoded 5x5 puzzle used as fallback for development/testing
 * when the backend is unavailable.
 */
export const FALLBACK_PUZZLE: PuzzleData = {
  puzzleId: "playtest-001",
  gridSize: 5,
  mode: "standard",
  regionMap: [
    [0, 0, 1, 1, 1],
    [0, 0, 1, 2, 2],
    [3, 3, 1, 2, 2],
    [3, 4, 4, 4, 2],
    [3, 3, 4, 4, 4],
  ],
};

type FetchState =
  | { status: "loading" }
  | { status: "ready"; puzzle: PuzzleData }
  | { status: "error"; message: string };

function GameBoard({ puzzle }: { puzzle: PuzzleData }) {
  const [ready, setReady] = useState(false);
  useLayoutEffect(() => { setReady(true); }, []);

  const {
    cells,
    conflicts,
    isSolved,
    draggedCells,
    handlePointerDown,
    handleDragEnter,
    handlePointerUp,
    resetGame,
  } = useGame(puzzle);

  return (
    <>
      {ready && isSolved && (
        <div
          style={{
            padding: "12px 24px",
            backgroundColor: "var(--color-success-bg)",
            color: "var(--color-success)",
            borderRadius: "var(--radius)",
            border: "2px solid var(--color-success)",
            fontWeight: 700,
          }}
        >
          Puzzle solved!
        </div>
      )}

      <div style={{ width: "100%", maxWidth: 600, display: "flex", justifyContent: "center" }}>
        <Grid
          puzzle={puzzle}
          cells={cells}
          conflicts={conflicts}
          isSolved={isSolved}
          draggedCells={draggedCells}
          onPointerDown={handlePointerDown}
          onPointerUp={handlePointerUp}
          onDragEnter={handleDragEnter}
        />
      </div>

      <button
        type="button"
        onClick={resetGame}
        hidden={!ready}
        style={{
          padding: "12px 32px",
          backgroundColor: "var(--color-surface)",
          color: "var(--color-ink)",
          border: "2px solid var(--color-ink)",
          borderRadius: "var(--radius)",
          boxShadow: "0 3px 0 var(--color-ink)",
          fontFamily: '"Nunito Sans", system-ui, sans-serif',
          fontWeight: 700,
          fontSize: "1rem",
          cursor: "pointer",
        }}
      >
        Reset
      </button>
    </>
  );
}

function AppContent() {
  const [state, setState] = useState<FetchState>({ status: "loading" });

  const fetchPuzzle = useCallback(async () => {
    setState({ status: "loading" });
    try {
      const puzzle = await generatePuzzle(5, "standard");
      setState({ status: "ready", puzzle });
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unknown error";
      setState({ status: "error", message });
    }
  }, []);

  useEffect(() => {
    void fetchPuzzle();
  }, [fetchPuzzle]);

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        gap: "24px",
        padding: "24px 16px",
        minHeight: "100vh",
        backgroundColor: "var(--color-background)",
        fontFamily: '"Nunito Sans", system-ui, sans-serif',
        color: "var(--color-ink)",
      }}
    >
      <h1
        style={{
          fontSize: "1.875rem",
          fontWeight: 800,
          letterSpacing: "-0.01em",
          margin: 0,
        }}
      >
        Reign
      </h1>

      {state.status === "loading" && (
        <div data-testid="loading-state" style={{ padding: "48px 0", fontWeight: 600 }}>
          Loading puzzle...
        </div>
      )}

      {state.status === "error" && (
        <div
          data-testid="error-state"
          style={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            gap: "16px",
            padding: "48px 0",
          }}
        >
          <p style={{ color: "var(--color-error, #c00)", fontWeight: 600 }}>
            Failed to load puzzle: {state.message}
          </p>
          <button
            type="button"
            onClick={() => void fetchPuzzle()}
            style={{
              padding: "12px 32px",
              backgroundColor: "var(--color-surface)",
              color: "var(--color-ink)",
              border: "2px solid var(--color-ink)",
              borderRadius: "var(--radius)",
              boxShadow: "0 3px 0 var(--color-ink)",
              fontFamily: '"Nunito Sans", system-ui, sans-serif',
              fontWeight: 700,
              fontSize: "1rem",
              cursor: "pointer",
            }}
          >
            Try Again
          </button>
        </div>
      )}

      {state.status === "ready" && <GameBoard puzzle={state.puzzle} />}
    </div>
  );
}

function App() {
  return (
    <ThemeProvider>
      <AppContent />
    </ThemeProvider>
  );
}

export default App;
