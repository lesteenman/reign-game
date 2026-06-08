// cellKey is the canonical string identifier for a grid cell, used
// by the game logic (move history, selection sets, render keys).
// Defined here so leaf components and hooks share the same key
// shape without one importing the other.
export function cellKey(row: number, col: number): string {
  return `${row},${col}`;
}
