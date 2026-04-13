import { render, fireEvent, cleanup } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { Grid } from './Grid';
import { ThemeProvider } from '../../theme/ThemeContext';
import type { PuzzleData, CellState } from '../../engine/types';
import type { GridProps } from './Grid';

afterEach(() => {
  cleanup();
});

const testPuzzle: PuzzleData = {
  puzzleId: 'test-1',
  gridSize: 5,
  mode: 'standard',
  regionMap: [
    [0, 0, 1, 1, 1],
    [0, 0, 1, 2, 2],
    [3, 3, 1, 2, 2],
    [3, 4, 4, 4, 2],
    [3, 3, 4, 4, 4],
  ],
};

function makeEmptyCells(size: number): CellState[][] {
  return Array.from({ length: size }, () =>
    Array.from({ length: size }, () => 'empty' as CellState),
  );
}

function renderGrid(overrides: Partial<GridProps> = {}) {
  const defaultProps: GridProps = {
    puzzle: testPuzzle,
    cells: makeEmptyCells(5),
    conflicts: [],
    isSolved: false,
    draggedCells: new Set<string>(),
    onPointerDown: vi.fn(),
    onPointerUp: vi.fn(),
    onDragEnter: vi.fn(),
  };

  const props = { ...defaultProps, ...overrides };

  const result = render(
    <ThemeProvider>
      <Grid {...props} />
    </ThemeProvider>,
  );

  return { ...result, props };
}

describe('Grid', () => {
  it('renders correct number of cells', () => {
    const { container } = renderGrid();
    const cells = container.querySelectorAll('[data-testid^="cell-"]');
    expect(cells).toHaveLength(25);
  });

  it('cells have correct region background colors', () => {
    const { container } = renderGrid();
    const cell00 = container.querySelector('[data-testid="cell-0-0"]') as HTMLElement;
    expect(cell00.style.backgroundColor).toBe('var(--region-0-fill)');

    const cell02 = container.querySelector('[data-testid="cell-0-2"]') as HTMLElement;
    expect(cell02.style.backgroundColor).toBe('var(--region-1-fill)');
  });

  it('renders region border overlay SVG', () => {
    const { container } = renderGrid();
    const grid = container.querySelector('[data-testid="game-grid"]') as HTMLElement;
    const svg = grid.querySelector('svg');
    expect(svg).not.toBeNull();
    // Should have at least some boundary lines
    const lines = svg!.querySelectorAll('line');
    expect(lines.length).toBeGreaterThan(0);
  });

  it('click on cell triggers onPointerDown with correct row/col', () => {
    const onPointerDown = vi.fn();
    const { container } = renderGrid({ onPointerDown });

    const cell23 = container.querySelector('[data-testid="cell-2-3"]') as HTMLElement;
    fireEvent.mouseDown(cell23);
    expect(onPointerDown).toHaveBeenCalledWith(2, 3);
  });

  it('solved state disables interaction', () => {
    const onPointerDown = vi.fn();
    const { container } = renderGrid({ isSolved: true, onPointerDown });

    const cell00 = container.querySelector('[data-testid="cell-0-0"]') as HTMLElement;
    fireEvent.mouseDown(cell00);
    expect(onPointerDown).not.toHaveBeenCalled();
  });

  it('onPointerUp called on mouseUp on a cell', () => {
    const onPointerUp = vi.fn();
    const { container } = renderGrid({ onPointerUp });

    const cell = container.querySelector('[data-testid="cell-0-0"]') as HTMLElement;
    fireEvent.mouseUp(cell);
    expect(onPointerUp).toHaveBeenCalledTimes(1);
  });

  it('onPointerUp called on mouseLeave on the grid', () => {
    const onPointerUp = vi.fn();
    const { container } = renderGrid({ onPointerUp });

    const grid = container.querySelector('[data-testid="game-grid"]') as HTMLElement;
    fireEvent.mouseLeave(grid);
    expect(onPointerUp).toHaveBeenCalledTimes(1);
  });
});
