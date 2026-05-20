import { render, fireEvent, cleanup } from '@shared/test-utils';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { Cell } from './Cell';
import { ThemeProvider } from '@theme/ThemeContext';
import type { CellProps } from './Cell';

afterEach(() => {
  cleanup();
});

function renderCell(overrides: Partial<CellProps> = {}) {
  const defaultProps: CellProps = {
    row: 0,
    col: 0,
    state: 'empty',
    regionIndex: 0,
    hasConflict: false,
    isDragHighlighted: false,
    cellSize: 60,
    showBorderTop: true,
    showBorderRight: true,
    showBorderBottom: true,
    showBorderLeft: true,
    onPointerDown: vi.fn(),
    onPointerUp: vi.fn(),
    onDragEnter: vi.fn(),
  };

  const props = { ...defaultProps, ...overrides };

  const result = render(
    <ThemeProvider>
      <Cell {...props} />
    </ThemeProvider>,
  );

  const cell = result.container.querySelector(
    '[data-testid="cell-0-0"]',
  ) as HTMLElement;

  return { ...result, cell, props };
}

describe('Cell', () => {
  it('renders with correct region background color', () => {
    // Arrange
    const regionIndex = 2;

    // Act
    const { cell } = renderCell({ regionIndex });

    // Assert
    expect(cell.style.backgroundColor).toBe('var(--region-2-fill)');
  });

  it('empty cell shows no marker or exclusion mark', () => {
    // Arrange & Act
    const { cell } = renderCell({ state: 'empty' });

    // Assert
    expect(cell.querySelector('svg')).toBeNull();
  });

  it('excluded cell shows ExclusionMark (circle dot), not Marker', () => {
    // Arrange & Act
    const { cell } = renderCell({ state: 'excluded' });

    // Assert — ExclusionMark renders a circle, Marker renders a rect
    expect(cell.querySelector('circle')).not.toBeNull();
    expect(cell.querySelector('rect')).toBeNull();
  });

  it('marked cell shows Marker (rounded rect), not ExclusionMark', () => {
    // Arrange & Act
    const { cell } = renderCell({ state: 'marked' });

    // Assert — Marker renders a rect, ExclusionMark renders a circle
    expect(cell.querySelector('rect')).not.toBeNull();
    expect(cell.querySelector('circle')).toBeNull();
  });

  it('conflict cell has destructive background color', () => {
    // Arrange & Act
    const { cell } = renderCell({ state: 'marked', hasConflict: true });

    // Assert
    expect(cell.style.backgroundColor).toBe('var(--color-destructive-bg)');
  });

  it('calls onPointerDown on mouseDown', () => {
    // Arrange
    const onPointerDown = vi.fn();
    const { cell } = renderCell({ onPointerDown });

    // Act
    fireEvent.mouseDown(cell);

    // Assert
    expect(onPointerDown).toHaveBeenCalledTimes(1);
  });

  it('calls onDragEnter on mouseEnter when button is held', () => {
    // Arrange
    const onDragEnter = vi.fn();
    const { cell } = renderCell({ onDragEnter });

    // Act
    fireEvent.mouseEnter(cell, { buttons: 1 });

    // Assert
    expect(onDragEnter).toHaveBeenCalledTimes(1);
  });

  it('does not call onDragEnter on mouseEnter when button is not held', () => {
    // Arrange
    const onDragEnter = vi.fn();
    const { cell } = renderCell({ onDragEnter });

    // Act
    fireEvent.mouseEnter(cell, { buttons: 0 });

    // Assert
    expect(onDragEnter).not.toHaveBeenCalled();
  });

  it('sets data-cell-state attribute', () => {
    // Arrange & Act
    const { cell } = renderCell({ state: 'marked' });

    // Assert
    expect(cell.getAttribute('data-cell-state')).toBe('marked');
  });

  it('sets data-cell-conflict attribute when conflicting', () => {
    // Arrange & Act
    const { cell } = renderCell({ state: 'marked', hasConflict: true });

    // Assert
    expect(cell.getAttribute('data-cell-conflict')).toBe('true');
  });
});
