import { render, fireEvent, cleanup } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { Cell } from './Cell';
import { ThemeProvider } from '../../theme/ThemeContext';
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
    const { cell } = renderCell({ regionIndex: 2 });
    expect(cell.style.backgroundColor).toBe('var(--region-2-fill)');
  });

  it('empty cell shows no marker or exclusion mark', () => {
    const { cell } = renderCell({ state: 'empty' });
    expect(cell.querySelector('svg')).toBeNull();
  });

  it('excluded cell shows ExclusionMark', () => {
    const { cell } = renderCell({ state: 'excluded' });
    const svg = cell.querySelector('svg');
    expect(svg).not.toBeNull();
  });

  it('marked cell shows Marker', () => {
    const { cell } = renderCell({ state: 'marked' });
    const svg = cell.querySelector('svg');
    expect(svg).not.toBeNull();
    expect(cell.querySelector('rect')).not.toBeNull();
  });

  it('conflict cell has destructive background color', () => {
    const { cell } = renderCell({ state: 'marked', hasConflict: true });
    expect(cell.style.backgroundColor).toBe('var(--color-destructive-bg)');
  });

  it('calls onPointerDown on mouseDown', () => {
    const onPointerDown = vi.fn();
    const { cell } = renderCell({ onPointerDown });
    fireEvent.mouseDown(cell);
    expect(onPointerDown).toHaveBeenCalledTimes(1);
  });

  it('calls onDragEnter on mouseEnter when button is held', () => {
    const onDragEnter = vi.fn();
    const { cell } = renderCell({ onDragEnter });
    fireEvent.mouseEnter(cell, { buttons: 1 });
    expect(onDragEnter).toHaveBeenCalledTimes(1);
  });

  it('does not call onDragEnter on mouseEnter when button is not held', () => {
    const onDragEnter = vi.fn();
    const { cell } = renderCell({ onDragEnter });
    fireEvent.mouseEnter(cell, { buttons: 0 });
    expect(onDragEnter).not.toHaveBeenCalled();
  });

  it('sets data-cell-state attribute', () => {
    const { cell } = renderCell({ state: 'marked' });
    expect(cell.getAttribute('data-cell-state')).toBe('marked');
  });

  it('sets data-cell-conflict attribute when conflicting', () => {
    const { cell } = renderCell({ state: 'marked', hasConflict: true });
    expect(cell.getAttribute('data-cell-conflict')).toBe('true');
  });
});
