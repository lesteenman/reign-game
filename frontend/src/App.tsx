import { ThemeProvider } from "./theme/ThemeContext";
import { Grid } from "./components/grid/Grid";
import { useGame } from "./hooks/useGame";
import type { PuzzleData } from "./engine/types";

/**
 * Hardcoded 5x5 puzzle for Milestone A playtesting.
 * Will be replaced by API fetch in Milestone B.
 */
const HARDCODED_PUZZLE: PuzzleData = {
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

function GameBoard() {
  const {
    cells,
    conflicts,
    isSolved,
    handleCellClick,
    handleDragStart,
    handleDragEnter,
    handleDragEnd,
    resetGame,
  } = useGame(HARDCODED_PUZZLE);

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

      {isSolved && (
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

      <Grid
        puzzle={HARDCODED_PUZZLE}
        cells={cells}
        conflicts={conflicts}
        isSolved={isSolved}
        onCellClick={handleCellClick}
        onDragStart={handleDragStart}
        onDragEnter={handleDragEnter}
        onDragEnd={handleDragEnd}
      />

      <button
        type="button"
        onClick={resetGame}
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
    </div>
  );
}

function App() {
  return (
    <ThemeProvider>
      <GameBoard />
    </ThemeProvider>
  );
}

export default App;
