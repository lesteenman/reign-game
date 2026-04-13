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
    cellSize: 60,
    borderTop: false,
    borderRight: false,
    borderBottom: false,
    borderLeft: false,
    onClick: vi.fn(),
    onDragStart: vi.fn(),
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
    // ExclusionMark renders lines (cross)
    expect(cell.querySelectorAll('line').length).toBe(2);
  });

  it('marked cell shows Marker', () => {
    const { cell } = renderCell({ state: 'marked' });
    const svg = cell.querySelector('svg');
    expect(svg).not.toBeNull();
    expect(cell.querySelector('circle')).not.toBeNull();
  });

  it('conflict cell has destructive background color', () => {
    const { cell } = renderCell({ state: 'marked', hasConflict: true });
    expect(cell.style.backgroundColor).toBe('var(--color-destructive-bg)');
  });

  it('calls onClick on mouseDown', () => {
    const onClick = vi.fn();
    const { cell } = renderCell({ onClick });
    fireEvent.mouseDown(cell);
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('calls onDragStart on mouseDown', () => {
    const onDragStart = vi.fn();
    const { cell } = renderCell({ onDragStart });
    fireEvent.mouseDown(cell);
    expect(onDragStart).toHaveBeenCalledTimes(1);
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

  it('applies region boundary borders correctly', () => {
    const { cell } = renderCell({
      borderTop: true,
      borderRight: false,
      borderBottom: true,
      borderLeft: false,
    });
    expect(cell.style.borderTopWidth).toBe('2.5px');
    expect(cell.style.borderBottomWidth).toBe('2.5px');
    expect(cell.style.borderRightWidth).toBe('1px');
    expect(cell.style.borderLeftWidth).toBe('1px');
  });
});
